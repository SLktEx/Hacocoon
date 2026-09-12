//go:build linux

package environmenttransfer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
	"io"
	"os"
	"strings"
	"testing"
)

type importFlow struct {
	t                         *testing.T
	mode                      string
	work                      gitrepo.Object
	resource                  core.PersistentResource
	inputs                    []gitrepo.WorkspaceImport
	deleted                   []string
	created, started, cleaned bool
	cancel                    context.CancelFunc
	original                  []byte
	checks                    int
}

func (f *importFlow) GetEnvironment(context.Context, string) (core.Environment, error) {
	f.checks++
	if f.mode == "exists" {
		return core.Environment{}, nil
	}
	return core.Environment{}, core.ErrNotFound
}
func (f *importFlow) GetWorkspaceLease(context.Context, string) (core.WorkspaceLease, error) {
	if f.mode == "lease" {
		return core.WorkspaceLease{}, nil
	}
	return core.WorkspaceLease{}, core.ErrNotFound
}
func (f *importFlow) ImportWorkspace(ctx context.Context, id, repository, remote, branch string, reader io.ReadSeeker) (gitrepo.Object, error) {
	return f.ImportWorkspaceSet(ctx, id, []gitrepo.WorkspaceImport{{Repository: repository, Remote: remote, Branch: branch, Archive: reader}})
}
func (f *importFlow) ImportWorkspaceSet(ctx context.Context, id string, in []gitrepo.WorkspaceImport) (gitrepo.Object, error) {
	f.inputs = in
	for i, input := range in {
		data, err := io.ReadAll(input.Archive)
		if err != nil || string(data) != "native "+workspaceRole(i)+" archive" {
			f.t.Fatal("incorrect data order", err)
		}
	}
	f.work = gitrepo.Object{ID: id, Kind: "work", Owner: strings.Repeat("a", 32), State: "ready"}
	if f.mode == "work" {
		f.work.State = "creating"
		return f.work, core.ErrRecoveryRequired
	}
	if f.mode == "mutate" {
		for i := range f.original {
			f.original[i] = 0
		}
	}
	return f.work, nil
}
func (f *importFlow) ImportForWorkspace(ctx context.Context, id, kind string, source io.ReadSeeker, work core.WorkspaceID) (core.PersistentResource, error) {
	data, err := io.ReadAll(source)
	if err != nil || string(data) != "native oci archive" || work != core.WorkspaceID("workspace:managed:"+f.work.Owner) {
		f.t.Fatal("OCI source/association", err)
	}
	if f.mode == "store-before" {
		return core.PersistentResource{}, core.ErrRuntimeUnavailable
	}
	f.resource = core.PersistentResource{ID: id, Owner: strings.Repeat("b", 32), NativeRef: "pool/owned", Kind: kind, WorkspaceID: work, State: "ready"}
	if f.mode == "store" {
		f.resource.State = "creating"
		return f.resource, core.ErrRecoveryRequired
	}
	return f.resource, nil
}
func (f *importFlow) CreateFromArchive(ctx context.Context, spec core.EnvironmentSpec, source io.ReadSeeker, root string, limit int64) (core.Environment, error) {
	f.created = true
	data, err := io.ReadAll(source)
	if err != nil || string(data) != "native rootfs archive" || spec.Base != "" || spec.PersistentResource != f.resource.ID || spec.ExpectedResource != f.resource.Ref() {
		f.t.Fatal("rootfs/binding mismatch", spec, err)
	}
	if spec.SkipDefaultResource != (f.resource.ID == "") {
		f.t.Fatal("implicit Host Store")
	}
	if f.mode == "cancel" {
		f.cancel()
		return core.Environment{}, context.Canceled
	}
	if f.mode == "env" || f.mode == "held" || f.mode == "cleanup" {
		return core.Environment{}, core.ErrRuntimeUnavailable
	}
	return core.Environment{Name: spec.Name, Workspace: core.Workspace{ID: core.WorkspaceID("workspace:managed:" + f.work.Owner), Path: "managed:" + f.work.ID}, PersistentResource: f.resource.Ref()}, nil
}
func (f *importFlow) StartForWorkspace(ctx context.Context, name string, work core.WorkspaceID) error {
	f.started = true
	if f.mode == "start" {
		return core.ErrRuntimeUnavailable
	}
	return nil
}
func (f *importFlow) CleanupRestoredData(ctx context.Context, work core.Workspace, remove func(context.Context) error) error {
	f.cleaned = true
	if ctx.Err() != nil {
		f.t.Fatal("cleanup inherited cancellation")
	}
	if f.mode == "held" {
		return core.ErrStorageBusy
	}
	return remove(ctx)
}
func (f *importFlow) DeleteRestoredCopy(ctx context.Context, r core.PersistentResource) error {
	if r != f.resource {
		f.t.Fatal("foreign Store deletion")
	}
	if f.mode == "cleanup" {
		return core.ErrRuntimeUnavailable
	}
	f.deleted = append(f.deleted, "oci")
	return nil
}
func (f *importFlow) DeleteWorkspace(ctx context.Context, id, owner string) error {
	if id != f.work.ID || owner != f.work.Owner {
		f.t.Fatal("foreign Workspace deletion")
	}
	f.deleted = append(f.deleted, "work")
	return nil
}

func importBundle(t *testing.T, version, count int, oci bool) []byte {
	t.Helper()
	m := Manifest{Version: version, Source: "dev", HasOCI: oci}
	roles := []string{"rootfs"}
	for i := 0; i < count; i++ {
		role := workspaceRole(i)
		roles = append(roles, role)
		if version == 2 {
			remote, branch := "https://github.com/example/repo.git", "main"
			if i == 1 {
				remote = "file:///source/host"
			}
			m.Workspaces = append(m.Workspaces, Workspace{Role: role, Name: role, Remote: remote, Branch: branch})
		}
	}
	if oci {
		roles = append(roles, "oci")
	}
	parts := []io.Reader{}
	for _, role := range roles {
		payload := []byte("native " + role + " archive")
		sum := sha256.Sum256(payload)
		m.Components = append(m.Components, Component{Role: role, Bytes: int64(len(payload)), SHA256: hex.EncodeToString(sum[:])})
		parts = append(parts, bytes.NewReader(payload))
	}
	var output bytes.Buffer
	if err := Write(&output, m, parts, 1<<20); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
func TestBundleImportOwnsDataAndFailureCleanup(t *testing.T) {
	for _, mode := range []string{"ok", "mutate", "work", "store-before", "store", "env", "held", "cleanup", "cancel", "start", "exists", "lease"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			root := t.TempDir()
			if err := os.Chmod(root, 0700); err != nil {
				t.Fatal(err)
			}
			source := importBundle(t, 2, 2, true)
			f := &importFlow{t: t, mode: mode, cancel: cancel, original: source}
			s := Importer{Catalog: f, Environments: f, Workspaces: f, Stores: f, Root: root, StoreKind: "oci-containerd"}
			result, err := s.Import(ctx, bytes.NewReader(source), "", 1<<20)
			if mode == "ok" || mode == "mutate" {
				if err != nil || result.State != "running" || result.Environment != "dev-imported" || !f.started || len(result.Offline) != 1 || f.inputs[1].Remote != "" || f.inputs[1].Branch != "" {
					t.Fatal(result, err)
				}
				return
			}
			if err == nil {
				t.Fatal("failure accepted")
			}
			switch mode {
			case "exists", "lease":
				if len(f.inputs) != 0 {
					t.Fatal("existing Env touched")
				}
			case "env", "cancel", "store-before":
				if result.Workspace != "" || result.OCI != "" || !f.cleaned || f.deleted[len(f.deleted)-1] != "work" {
					t.Fatal("owned cleanup failed", result, f.deleted)
				}
			case "start":
				if result.State != "start-failed" || f.cleaned || !f.started {
					t.Fatal("startup failure destroyed data", result)
				}
			default:
				if result.State != "cleanup-required" || result.Workspace == "" || len(f.deleted) != 0 {
					t.Fatal("uncertainty lost data", result, f.deleted)
				}
			}
		})
	}
}
func TestBundleImportPreflightAndLegacyOffline(t *testing.T) {
	for _, mode := range []string{"legacy", "single", "corrupt", "too-many"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Chmod(root, 0700); err != nil {
				t.Fatal(err)
			}
			version, count := 2, 1
			if mode == "legacy" {
				version = 1
			}
			if mode == "too-many" {
				count = 9
			}
			data := importBundle(t, version, count, false)
			if mode == "corrupt" {
				data[len(data)-1] = 1
			}
			f := &importFlow{t: t}
			s := Importer{Catalog: f, Environments: f, Workspaces: f, Root: root}
			result, err := s.Import(context.Background(), bytes.NewReader(data), "chosen", 1<<20)
			if mode == "corrupt" || mode == "too-many" {
				if err == nil || f.checks != 0 || len(f.inputs) != 0 {
					t.Fatal("invalid bundle mutated state", err)
				}
				return
			}
			if err != nil || result.Environment != "chosen" || result.OCI != "" || result.State != "running" {
				t.Fatal(result, err)
			}
			if mode == "legacy" && (len(result.Offline) != 1 || f.inputs[0].Remote != "") {
				t.Fatal("legacy authority guessed")
			}
		})
	}
}
