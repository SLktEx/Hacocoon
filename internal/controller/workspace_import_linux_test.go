//go:build linux

package controller

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	gitadapter "github.com/SLktEx/Hacocoon/internal/adapters/git"
	"github.com/SLktEx/Hacocoon/internal/adapters/incus"
	"github.com/SLktEx/Hacocoon/internal/composition"
	controlapi "github.com/SLktEx/Hacocoon/internal/controller/api"
	control "github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	gitrepo "github.com/SLktEx/Hacocoon/internal/git"
	"github.com/SLktEx/Hacocoon/internal/host"
)

// Keep the actual controller receiver, portable-tree conversion and native
// archive staging. Only source routing/observation and Incus commands are fake.
type workspaceInputBackend struct{ *incus.RepositoryBackend }

func (*workspaceInputBackend) Plan(_ context.Context, kind, id string) (string, error) {
	return "pool/haco-" + kind + "-" + id, nil
}
func (*workspaceInputBackend) InspectVolume(context.Context, gitrepo.Object) error { return nil }
func (*workspaceInputBackend) RunGit(context.Context, gitadapter.AgentRequest) (gitadapter.Response, error) {
	return gitadapter.Response{Ref: "refs/heads/main", OID: strings.Repeat("a", 40)}, nil
}

type workspaceInputRunner struct{ imports atomic.Int32 }

func (r *workspaceInputRunner) Run(_ context.Context, command string, args ...string) (host.Result, error) {
	if command == "incus" && len(args) == 2 && args[0] == "query" {
		return host.Result{Stdout: "[]"}, nil
	}
	if command != "incus" || len(args) != 8 || args[0] != "storage" || args[2] != "import" || args[3] != "pool" || args[5] != "haco-work-input" {
		return host.Result{}, errors.New("unexpected native import command")
	}
	file, err := os.Open(args[4])
	if err != nil {
		return host.Result{}, err
	}
	defer func() { _ = file.Close() }()
	entry, err := tar.NewReader(file).Next()
	if err != nil || entry.Name != "backup/index.yaml" {
		return host.Result{}, errors.New("actual native archive staging missing")
	}
	r.imports.Add(1)
	return host.Result{}, nil
}

type workspaceInputListener struct {
	connections chan net.Conn
	done        chan struct{}
	once        sync.Once
}

func (l *workspaceInputListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.connections:
		return conn, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}
func (l *workspaceInputListener) Close() error { l.once.Do(func() { close(l.done) }); return nil }
func (*workspaceInputListener) Addr() net.Addr {
	return &net.UnixAddr{Name: "workspace-input-memory", Net: "unix"}
}

func workspaceInputBytes(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	for _, entry := range []struct{ name, content string }{
		{"tree", ""}, {"tree/.git", ""},
		{"tree/.git/HEAD", "ref: refs/heads/main\n"},
		{"tree/.git/config", "[core]\nrepositoryformatversion = 0\n"},
	} {
		kind := byte(tar.TypeReg)
		if entry.content == "" {
			kind = tar.TypeDir
		}
		if err := writer.WriteHeader(&tar.Header{Name: entry.name, Typeflag: kind, Mode: 0700, Size: int64(len(entry.content))}); err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(writer, entry.content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestWorkspaceImportRegistrationDoesNotPrepareStaging(t *testing.T) {
	for _, configured := range []bool{false, true} {
		name := "default root"
		if configured {
			name = "custom root"
		}
		t.Run(name, func(t *testing.T) {
			root := ""
			if configured {
				root = filepath.Join(t.TempDir(), "not-created")
			}
			t.Setenv("HACO_ROOT", root)
			server := control.NewServer()
			// Registration must install the endpoint without invoking the service
			// or requiring filesystem access to the default or custom root.
			if err := registerWorkspaceImport(server, nil); err != nil {
				t.Fatal(err)
			}
			if err := registerWorkspaceImport(server, nil); !errors.Is(err, control.ErrInvalidArgument) {
				t.Fatal("Workspace import endpoint was not registered", err)
			}
			if configured {
				if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("registration prepared staging before an import", err)
				}
			}
		})
	}
}

func TestWorkspaceImportInitializesInstalledStaging(t *testing.T) {
	for _, mode := range []string{"missing", "existing", "file", "broad", "symlink"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("HACO_ROOT", root)
			staging := filepath.Join(root, "transfers")
			switch mode {
			case "existing", "broad":
				perm := os.FileMode(0700)
				if mode == "broad" {
					perm = 0755
				}
				if err := os.Mkdir(staging, perm); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(staging, "keep"), []byte("retained"), 0600); err != nil {
					t.Fatal(err)
				}
			case "file":
				if err := os.WriteFile(staging, []byte("retained"), 0600); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Symlink(t.TempDir(), staging); err != nil {
					t.Fatal(err)
				}
			}
			runner := &workspaceInputRunner{}
			backend := &workspaceInputBackend{&incus.RepositoryBackend{Runtime: incus.New(runner), ImportRoot: staging, ImportLimit: 1 << 20}}
			repositories := gitrepo.NewRepositoryService(filepath.Join(root, "repositories"), backend)
			if err := os.Mkdir(repositories.Root, 0700); err != nil {
				t.Fatal(err)
			}
			source := gitrepo.Object{Kind: "repo", ID: "source", Repository: "source", Owner: strings.Repeat("a", 32), Remote: "https://github.com/example/source.git", NativeRef: "pool/haco-repo-source", State: "ready"}
			raw, err := json.Marshal(source)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(repositories.Root, "repo-source.json"), raw, 0600); err != nil {
				t.Fatal(err)
			}
			server := control.NewServer()
			if err := registerWorkspaceImport(server, &composition.App{Repositories: repositories}); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			listener := &workspaceInputListener{connections: make(chan net.Conn), done: make(chan struct{})}
			defer func() { _ = listener.Close() }()
			serveDone := make(chan error, 1)
			go func() { serveDone <- server.Serve(ctx, listener) }()
			defer func() { cancel(); _ = listener.Close(); <-serveDone }()
			client, err := controlapi.NewClientWithDialer(func(ctx context.Context) (net.Conn, error) {
				local, peer := net.Pipe()
				select {
				case listener.connections <- peer:
					return local, nil
				case <-ctx.Done():
					_ = local.Close()
					_ = peer.Close()
					return nil, ctx.Err()
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			result, err := client.ImportWorkspace(ctx, bytes.NewReader(workspaceInputBytes(t)), controlapi.WorkspaceImportRequest{Name: "input", Repository: "source"})
			if mode == "missing" || mode == "existing" {
				if err != nil || result.State != "ready" || result.Workspace == "" || runner.imports.Load() != 1 {
					t.Fatalf("installed Workspace import: result=%+v imports=%d err=%v", result, runner.imports.Load(), err)
				}
				info, err := os.Stat(staging)
				if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
					t.Fatal("staging is not private and controller-owned", info, err)
				}
			} else if err == nil || runner.imports.Load() != 0 {
				t.Fatal("unsafe staging accepted", result, err)
			}
			if mode == "broad" || mode == "symlink" {
				_, retainedErr := repositories.Get("work", "input")
				if result.State != "recovery-required" || result.Workspace == "" || !errors.Is(retainedErr, core.ErrRecoveryRequired) {
					t.Fatal("backend refusal lost its reserved recovery identity", result, retainedErr)
				}
			}
			if mode == "file" {
				_, lookupErr := repositories.Get("work", "input")
				if result.Workspace != "" || !errors.Is(lookupErr, core.ErrNotFound) {
					t.Fatal("staging initialization failure reserved a Workspace", result, lookupErr)
				}
			}
			if mode == "existing" || mode == "broad" {
				data, err := os.ReadFile(filepath.Join(staging, "keep"))
				if err != nil || string(data) != "retained" {
					t.Fatal("existing staging data changed", err)
				}
			}
			if mode == "broad" {
				info, err := os.Stat(staging)
				if err != nil || info.Mode().Perm() != 0755 {
					t.Fatal("existing staging permissions silently changed", err)
				}
			}
		})
	}
}
