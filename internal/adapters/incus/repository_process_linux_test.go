//go:build linux

package incus

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	gitadapter "github.com/SLktEx/Hacocoon/internal/adapters/git"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func repositoryGitProcess(t *testing.T, mode string) string {
	t.Helper()
	dir := t.TempDir()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nexec '" + strings.ReplaceAll(binary, "'", "'\"'\"'") + "' -test.run=^TestRepositoryGitProcessHelper$ -- \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "incus"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("HACOTEST_GIT_PROCESS", mode)
	started := filepath.Join(dir, "started")
	t.Setenv("HACOTEST_GIT_STARTED", started)
	return started
}

// This subprocess substitutes only Incus exec. It exercises the real controller
// pipes with independently encoded bounded frames and a mandatory final receipt.
func TestRepositoryGitProcessHelper(t *testing.T) {
	mode := os.Getenv("HACOTEST_GIT_PROCESS")
	if mode == "" {
		return
	}
	var args []string
	for i, arg := range os.Args {
		if arg == "--" {
			args = os.Args[i+1:]
			break
		}
	}
	if !reflect.DeepEqual(args, []string{"exec", "haco-host", "--project", "hacocoon", "--", "/usr/local/bin/haco", "_git-agent"}) {
		os.Exit(70)
	}
	if err := os.WriteFile(os.Getenv("HACOTEST_GIT_STARTED"), []byte("started"), 0600); err != nil {
		os.Exit(71)
	}
	request, err := gitadapter.ReadAgentRequest(os.Stdin)
	if err != nil || request.Repository != "demo" || request.Remote != "https://github.com/example/demo.git" || request.Branch != "main" {
		os.Exit(72)
	}
	var pack []byte
	if request.Pack != nil {
		pack, err = io.ReadAll(request.Pack)
		if err != nil {
			os.Exit(73)
		}
	}
	if err := os.WriteFile(os.Getenv("HACOTEST_GIT_STARTED"), []byte(request.Operation+":"+request.Workspace), 0600); err != nil {
		os.Exit(81)
	}
	if mode == "wait" {
		time.Sleep(time.Hour)
		os.Exit(74)
	}
	if _, err := io.WriteString(os.Stderr, "private-agent-diagnostic"); err != nil {
		os.Exit(75)
	}
	if mode == "malformed" {
		if _, err := os.Stdout.Write([]byte{0, 0, 0, 1}); err != nil {
			os.Exit(76)
		}
		os.Exit(0)
	}
	response := gitadapter.Response{Summary: request.Operation + ":" + request.Workspace, PackBytes: int64(len(pack))}
	if mode == "receipt-error" {
		response.Error = "private-agent-diagnostic"
	}
	if mode == "wrong-count" {
		response.PackBytes++
	}
	var encoded bytes.Buffer
	for len(pack) != 0 {
		size := min(len(pack), 64<<10)
		if err := binary.Write(&encoded, binary.BigEndian, uint32(size)); err != nil {
			os.Exit(77)
		}
		_, _ = encoded.Write(pack[:size])
		pack = pack[size:]
	}
	_, _ = encoded.Write([]byte{0, 0, 0, 0})
	metadata, err := json.Marshal(response)
	if err != nil {
		os.Exit(78)
	}
	if err := binary.Write(&encoded, binary.BigEndian, uint32(len(metadata))); err != nil {
		os.Exit(79)
	}
	_, _ = encoded.Write(metadata)
	if _, err := os.Stdout.Write(encoded.Bytes()); err != nil {
		os.Exit(80)
	}
	if mode == "exit" {
		os.Exit(17)
	}
	os.Exit(0)
}

type repositoryFailedOutput struct{ err error }

func (w repositoryFailedOutput) Write([]byte) (int, error) { return 0, w.err }

func TestRepositoryRunGitRequiresCompleteProcessAndReceipt(t *testing.T) {
	for _, mode := range []string{"ok", "malformed", "exit", "wrong-count", "receipt-error", "output-fails", "foreign-host", "oversized-metadata", "start-fails"} {
		t.Run(mode, func(t *testing.T) {
			started := repositoryGitProcess(t, mode)
			if mode == "start-fails" {
				t.Setenv("PATH", t.TempDir())
			}
			runner := &programRunner{owned: mode != "foreign-host"}
			backend := &RepositoryBackend{Runtime: New(runner)}
			payload := bytes.Repeat([]byte{0, 255, '\n', '{', '}'}, 40000)
			var output bytes.Buffer
			request := gitadapter.AgentRequest{Operation: "prepare", Repository: "demo", Workspace: "task", Remote: "https://github.com/example/demo.git", Branch: "main", Pack: bytes.NewReader(payload), PackOutput: &output}
			failure := errors.New("output closed")
			if mode == "output-fails" {
				request.PackOutput = repositoryFailedOutput{failure}
			}
			if mode == "oversized-metadata" {
				request.Remote = strings.Repeat("x", 2<<20)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			response, err := backend.RunGit(ctx, request)
			if mode == "ok" {
				if err != nil || response.Summary != "prepare:task" || response.PackBytes != int64(len(payload)) || !bytes.Equal(output.Bytes(), payload) {
					t.Fatal("streamed bytes or receipt changed", response, err)
				}
				return
			}
			if err == nil || !reflect.DeepEqual(response, gitadapter.Response{}) || strings.Contains(err.Error(), "private-agent-diagnostic") {
				t.Fatal("failed agent reported success or leaked diagnostics", response, err)
			}
			if mode == "output-fails" && !errors.Is(err, failure) {
				t.Fatal("output failure lost", err)
			}
			if mode == "oversized-metadata" && !errors.Is(err, core.ErrInvalidArgument) {
				t.Fatal("invalid metadata outcome", err)
			}
			if mode == "foreign-host" || mode == "oversized-metadata" || mode == "start-fails" {
				if _, err := os.Stat(started); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("agent started before admission", err)
				}
			}
		})
	}
}

func TestRepositoryRunGitCancellationWaitsForOwnedChild(t *testing.T) {
	started := repositoryGitProcess(t, "wait")
	backend := &RepositoryBackend{Runtime: New(&programRunner{owned: true})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := backend.RunGit(ctx, gitadapter.AgentRequest{Operation: "clone", Repository: "demo", Remote: "https://github.com/example/demo.git", Branch: "main"})
		done <- err
	}()
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		if _, err := os.Stat(started); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		select {
		case err := <-done:
			t.Fatal("agent did not wait for cancellation", err)
		case <-deadline.C:
			t.Fatal("agent did not start")
		case <-tick.C:
		}
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled Git process succeeded")
		}
	case <-deadline.C:
		t.Fatal("canceled Git process did not return")
	}
}

func TestRepositoryPopulateDetachesWorkspaceBeforeReturningSuccess(t *testing.T) {
	for _, mode := range []string{"repo", "work", "foreign-host", "foreign-volume", "attach-fails", "agent-fails", "detach-fails"} {
		t.Run(mode, func(t *testing.T) {
			agentMode := "ok"
			if mode == "agent-fails" {
				agentMode = "exit"
			}
			started := repositoryGitProcess(t, agentMode)
			kind := "work"
			if mode == "repo" {
				kind = "repo"
			}
			object := repositoryObject(kind, "task", "demo")
			mounted, removed := false, false
			failure := errors.New("device operation failed")
			runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
				if name != "incus" {
					t.Fatal(name)
				}
				if reflect.DeepEqual(args, []string{"config", "get", "haco-host", "user.hacocoon.role", "--project", "hacocoon"}) {
					role := "trusted-host"
					if mode == "foreign-host" {
						role = "other"
					}
					return host.Result{Stdout: role}, nil
				}
				if reflect.DeepEqual(args, []string{"query", "/1.0/storage-pools/haco-local-default/volumes/custom/haco-" + kind + "-task?project=hacocoon"}) {
					observed := repositoryVolume(object)
					if mode == "foreign-volume" {
						observed["config"].(map[string]string)["user.hacocoon.owner"] = "other"
					}
					return repositoryJSON(t, observed), nil
				}
				root := "/var/lib/hacocoon-workspaces"
				if kind == "repo" {
					root = "/var/lib/hacocoon-repos"
				}
				if reflect.DeepEqual(args, []string{"config", "device", "add", "haco-host", "haco-" + kind + "-task", "disk", "pool=haco-local-default", "source=haco-" + kind + "-task", "path=" + root + "/task", "--project", "hacocoon"}) {
					if mode == "attach-fails" {
						return host.Result{}, failure
					}
					mounted = true
					return host.Result{}, nil
				}
				if reflect.DeepEqual(args, []string{"config", "device", "remove", "haco-host", "haco-work-task", "--project", "hacocoon"}) {
					if !mounted {
						t.Fatal("removed an unowned device")
					}
					if mode == "detach-fails" {
						return host.Result{}, failure
					}
					mounted, removed = false, true
					return host.Result{}, nil
				}
				t.Fatal("unexpected provider mutation", args)
				return host.Result{}, failure
			}}
			err := (&RepositoryBackend{Runtime: New(runner)}).Populate(context.Background(), object)
			if mode == "repo" || mode == "work" {
				if err != nil || mounted != (mode == "repo") || removed != (mode == "work") {
					t.Fatal("wrong Host mount lifetime", err, mounted, removed)
				}
				operation := "workspace:task"
				if mode == "repo" {
					operation = "clone:task"
				}
				observed, err := os.ReadFile(started)
				if err != nil || string(observed) != operation {
					t.Fatal("wrong Git preparation operation", string(observed), err)
				}
				return
			}
			if err == nil {
				t.Fatal("failed preparation reported success")
			}
			if mode == "foreign-host" || mode == "foreign-volume" || mode == "attach-fails" {
				if _, err := os.Stat(started); !errors.Is(err, os.ErrNotExist) || mounted || removed {
					t.Fatal("unverified Workspace reached Git or Host mount", err)
				}
			}
		})
	}
}
