//go:build linux

package incus

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/persistentresource"
	"github.com/SLktEx/Hacocoon/internal/state"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
	"github.com/lxc/incus/v6/shared/cliconfig"
)

type importAcceptanceBackend struct {
	*PersistentResourceBackend
	pool string
}

func (b importAcceptanceBackend) Plan(_ context.Context, kind, owner string) (string, error) {
	if kind != OCIStoreKind {
		return "", core.ErrInvalidArgument
	}
	return b.pool + "/haco-persistent-" + owner, nil
}

type workspaceImportAcceptanceBackend struct {
	*RepositoryBackend
	pool string
}

func (b workspaceImportAcceptanceBackend) Plan(_ context.Context, kind, id string) (string, error) {
	if kind != "work" || !gitrepo.ValidID(id) {
		return "", core.ErrInvalidArgument
	}
	return b.pool + "/haco-work-" + id, nil
}

func TestRealIncusOwnedVolumeImportE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_INCUS_VOLUME_IMPORT") != "1" {
		t.Skip("requires explicit dedicated root Incus/Btrfs import acceptance")
	}
	if os.Geteuid() != 0 {
		t.Fatal("root required")
	}
	config, configErr := cliconfig.LoadConfig("")
	if configErr != nil {
		t.Fatal(configErr)
	}
	remote, ok := config.Remotes[config.DefaultRemote]
	if !ok || remote.Public || remote.Protocol != "incus" || !strings.HasPrefix(remote.Addr, "unix:") {
		t.Fatal("requires private local Unix Incus")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	runner := host.ExecRunner{}
	run := func(args ...string) string {
		t.Helper()
		out, err := runner.Run(ctx, "incus", args...)
		if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
			t.Fatalf("Incus fixture failed: %v", err)
		}
		return strings.TrimSpace(out.Stdout)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var nonce [8]byte
	_, err := rand.Read(nonce[:])
	must(err)
	owner := hex.EncodeToString(nonce[:])
	sourcePool, targetPool := "haco-import-source-"+owner, "haco-import-target-"+owner
	root, err := os.MkdirTemp("/var/lib", "haco-owned-import-")
	must(err)
	plan, err := os.OpenFile(filepath.Join(root, "plan.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	must(err)
	must(json.NewEncoder(plan).Encode(map[string]any{"owner": owner, "source_pool": sourcePool, "target_pool": targetPool, "source_volume": "source", "project": "default"}))
	must(plan.Sync())
	must(plan.Close())
	directory, err := os.Open(root)
	must(err)
	must(directory.Sync())
	must(directory.Close())
	t.Log("isolated native import fixture:", root)
	const ownerKey = "user.hacocoon.transfer-test"
	for _, pool := range []string{sourcePool, targetPool} {
		run("storage", "create", pool, "btrfs", "size=1GiB", ownerKey+"="+owner)
	}
	run("storage", "volume", "create", sourcePool, "source", ownerKey+"="+owner, "user.hacocoon.owner=old-import-owner", "--project", "default")
	volumePath := func(pool, name string) string {
		p := filepath.Join("/var/lib/incus/storage-pools", pool, "custom", "default_"+name)
		info, e := os.Lstat(p)
		if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			t.Fatal("owned volume mount unavailable", e)
		}
		return p
	}
	src := volumePath(sourcePool, "source")
	must(os.WriteFile(filepath.Join(src, "data"), []byte("persistent data"), 0600))
	must(os.Link(filepath.Join(src, "data"), filepath.Join(src, "hardlink")))
	must(os.Symlink("data", filepath.Join(src, "symlink")))
	must(os.WriteFile(filepath.Join(src, "numeric-owner"), []byte("numeric IDs"), 0600))
	must(os.Chown(filepath.Join(src, "numeric-owner"), 1000123, 1000456))
	mapping := `[{"Isuid":true,"Isgid":false,"Hostid":1000000,"Nsid":0,"Maprange":65536},{"Isuid":false,"Isgid":true,"Hostid":1000000,"Nsid":0,"Maprange":65536}]`
	for _, key := range []string{"volatile.idmap.last", "volatile.idmap.next"} {
		run("storage", "volume", "set", sourcePool, "source", key, mapping, "--project", "default")
	}
	git := func(path string, args ...string) string {
		t.Helper()
		commandArgs := append([]string{"-C", path}, args...)
		out, e := runner.Run(ctx, "git", commandArgs...)
		must(e)
		if out.ExitCode != 0 {
			t.Fatal("fixture Git command failed", out.ExitCode)
		}
		return strings.TrimSpace(out.Stdout)
	}
	git(src, "init", "--quiet")
	must(os.WriteFile(filepath.Join(src, "tracked-change"), []byte("committed"), 0600))
	git(src, "add", "data", "tracked-change")
	git(src, "-c", "user.name=Import Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--quiet", "-m", "retained commit")
	savedCommit := git(src, "rev-parse", "HEAD")
	git(src, "config", "remote.origin.url", "file:///source-only/untrusted.git")
	must(os.WriteFile(filepath.Join(src, "data"), []byte("persistent data"), 0600))
	must(os.WriteFile(filepath.Join(src, "untracked"), []byte("untracked data"), 0600))
	must(os.WriteFile(filepath.Join(src, "tracked-change"), []byte("uncommitted"), 0600))
	archivePath := filepath.Join(root, "source.tar")
	run("storage", "volume", "export", sourcePool, "source", archivePath, "--volume-only", "--compression=none", "--project", "default")
	original, err := os.ReadFile(archivePath)
	must(err)
	digest := sha256.Sum256(original)
	input, err := os.Open(archivePath)
	must(err)
	defer input.Close()
	runtime := New(runner)
	runtime.project = "default"
	backend := &PersistentResourceBackend{Runtime: runtime, ImportRoot: root, ImportLimit: 16 << 20}
	catalog := state.NewEnvironmentJSONStore(filepath.Join(root, "state.json"))
	service := persistentresource.Service{Store: catalog, Backend: importAcceptanceBackend{backend, targetPool}}
	resource, err := service.Import(ctx, "oci:imported-"+owner, OCIStoreKind, input)
	must(err)
	if resource.State != "ready" || resource.Owner == owner || resource.Owner == "old-import-owner" {
		t.Fatal("fresh ownership absent")
	}
	must(backend.Verify(ctx, resource))
	if _, err := service.Import(ctx, resource.ID, OCIStoreKind, input); !errors.Is(err, core.ErrAlreadyExists) {
		t.Fatal("duplicate import accepted", err)
	}
	_, name, err := persistentVolume(resource)
	must(err)
	dst := volumePath(targetPool, name)
	data, err := os.ReadFile(filepath.Join(dst, "data"))
	must(err)
	if string(data) != "persistent data" {
		t.Fatal("data lost")
	}
	a, err := os.Stat(filepath.Join(dst, "data"))
	must(err)
	b, err := os.Stat(filepath.Join(dst, "hardlink"))
	must(err)
	if !os.SameFile(a, b) || a.Mode().Perm() != 0600 {
		t.Fatal("hardlink or mode lost")
	}
	link, err := os.Readlink(filepath.Join(dst, "symlink"))
	must(err)
	if link != "data" {
		t.Fatal("symlink lost")
	}
	numeric, err := runner.Run(ctx, "stat", "-c", "%u:%g", filepath.Join(dst, "numeric-owner"))
	must(err)
	if strings.TrimSpace(numeric.Stdout) != "1000123:1000456" {
		t.Fatal("numeric IDs changed")
	}
	for _, key := range []string{"volatile.idmap.last", "volatile.idmap.next"} {
		if run("storage", "volume", "get", targetPool, name, key, "--project", "default") != mapping {
			t.Fatal("data idmap lost")
		}
	}
	if run("storage", "volume", "get", targetPool, name, ownerKey, "--project", "default") != "" {
		t.Fatal("old config adopted")
	}
	must(os.WriteFile(filepath.Join(dst, "data"), []byte("independent import"), 0600))
	unchanged, err := os.ReadFile(filepath.Join(src, "data"))
	must(err)
	if string(unchanged) != "persistent data" {
		t.Fatal("source mutated")
	}
	if run("storage", "get", sourcePool, ownerKey) != owner || run("storage", "volume", "get", sourcePool, "source", ownerKey, "--project", "default") != owner {
		t.Fatal("foreign source cleanup")
	}
	run("storage", "volume", "delete", sourcePool, "source", "--project", "default")
	retained, err := os.ReadFile(filepath.Join(dst, "data"))
	must(err)
	if string(retained) != "independent import" {
		t.Fatal("source deletion affected import")
	}
	must(service.Delete(ctx, resource.ID))
	if _, err := catalog.GetPersistentResource(ctx, resource.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatal("catalog retained deleted volume")
	}
	workBackend := &RepositoryBackend{Runtime: runtime, ImportRoot: root, ImportLimit: 16 << 20}
	works := gitrepo.NewRepositoryService(filepath.Join(root, "workspaces"), workspaceImportAcceptanceBackend{workBackend, targetPool})
	object, err := works.ImportWorkspace(ctx, "imported-"+owner, "repo", "https://github.com/SLktEx/Hacocoon-test.git", "main", input)
	must(err)
	if object.State != "ready" || object.Owner == resource.Owner || object.Owner == owner {
		t.Fatal("Workspace ownership was reused")
	}
	must(workBackend.InspectVolume(ctx, object))
	workPath := volumePath(targetPool, "haco-work-"+object.ID)
	if git(workPath, "rev-parse", "HEAD") != savedCommit || git(workPath, "config", "remote.origin.url") != "file:///source-only/untrusted.git" {
		t.Fatal("saved Git data rewritten")
	}
	workData, err := os.ReadFile(filepath.Join(workPath, "untracked"))
	must(err)
	if string(workData) != "untracked data" {
		t.Fatal("untracked data lost")
	}
	dirty, err := os.ReadFile(filepath.Join(workPath, "tracked-change"))
	must(err)
	if string(dirty) != "uncommitted" || git(workPath, "status", "--porcelain", "--", "tracked-change") == "" {
		t.Fatal("uncommitted data lost")
	}
	if object.Remote != "https://github.com/SLktEx/Hacocoon-test.git" {
		t.Fatal("guest Git config adopted as routing")
	}
	if _, err := works.ImportWorkspace(ctx, object.ID, "repo", object.Remote, object.Branch, input); !errors.Is(err, core.ErrAlreadyExists) {
		t.Fatal("duplicate Workspace import", err)
	}
	// This isolated fixture has no Env or lease. Use the existing owned native
	// deletion path and retain the registry if any absence check fails.
	must(works.DeleteWorkspace(ctx, object.ID, object.Owner))
	if _, err := works.Get("work", object.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatal("deleted Workspace still registered", err)
	}
	for _, pool := range []string{sourcePool, targetPool} {
		if run("storage", "get", pool, ownerKey) != owner {
			t.Fatal("foreign pool cleanup")
		}
		var volumes []any
		must(json.Unmarshal([]byte(run("storage", "volume", "list", pool, "--all-projects", "--format=json")), &volumes))
		if volumes == nil || len(volumes) != 0 {
			t.Fatal("pool not positively empty")
		}
		run("storage", "delete", pool)
	}
	after, err := os.ReadFile(archivePath)
	must(err)
	if sha256.Sum256(after) != digest {
		t.Fatal("saved input changed")
	}
	t.Log("PASS native Btrfs volume import through canonical Store creation: fresh owner/config, data/links/mode/numeric IDs/idmap, duplicate refusal, source deletion independence, owned cleanup; no public bundle/Env import or live OCI daemon asserted")
}
