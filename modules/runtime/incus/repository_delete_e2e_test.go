package incus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
)

// This bounded native check needs no image, running Env, credentials or network.
func TestRealIncusWorkspaceDeletionE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_WORKSPACE_DELETE") != "1" {
		t.Skip("set HACO_E2E_WORKSPACE_DELETE=1 on a dedicated Incus/Btrfs host")
	}
	pool := os.Getenv("HACO_E2E_INCUS_RESUME_POOL")
	if os.Geteuid() != 0 || !safeIncusRef(pool) {
		t.Fatal("explicit root/pool required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	runner := host.ExecRunner{}
	query := func(method, path string, data any, result any) {
		t.Helper()
		args := []string{"query", "-X", method, path}
		if data != nil {
			raw, err := json.Marshal(data)
			must(err)
			args = append(args, "--data", string(raw))
		}
		if method != "GET" {
			args = append(args, "--wait")
		}
		out, err := runner.Run(ctx, "incus", args...)
		must(err)
		if out.ExitCode != 0 || out.StdoutTruncated {
			t.Fatal("native query failed", method, path)
		}
		if result != nil {
			must(json.Unmarshal([]byte(out.Stdout), result))
		}
	}
	var storage struct{ Name, Driver string }
	query("GET", "/1.0/storage-pools/"+pool, nil, &storage)
	if storage.Name != pool || storage.Driver != "btrfs" {
		t.Fatal("dedicated Btrfs pool required")
	}
	var nonce [16]byte
	_, err := rand.Read(nonce[:])
	must(err)
	owner := hex.EncodeToString(nonce[:])
	id := "delete-" + owner
	object := gitrepo.Object{Kind: "work", ID: id, Repository: "fixture", Remote: "https://github.com/example/fixture.git", Branch: "main", Owner: owner, State: "ready", NativeRef: pool + "/haco-work-" + id}
	dir, err := os.MkdirTemp("/var/lib", "haco-workspace-delete-")
	must(err)
	receipt, err := json.Marshal(object)
	must(err)
	must(os.WriteFile(filepath.Join(dir, "ownership.json"), receipt, 0600))
	t.Log("exact fixture ownership receipt", dir)
	r := New(runner)
	backend := &RepositoryBackend{Runtime: r}
	volume := "haco-work-" + id
	query("POST", "/1.0/storage-pools/"+pool+"/volumes/custom?project="+r.project, map[string]any{"name": volume, "type": "custom", "content_type": "filesystem", "config": volumeConfig(object)}, nil)
	must(backend.CheckWorkspaceVolumeDeletion(ctx, object))
	path := "/1.0/storage-pools/" + pool + "/volumes/custom/" + volume
	for _, kind := range []string{"snapshots", "backups"} {
		query("POST", path+"/"+kind+"?project="+r.project, map[string]any{"name": "keep"}, nil)
		if err := backend.DeleteWorkspaceVolume(ctx, object); !errors.Is(err, core.ErrStorageBusy) {
			t.Fatal("native saved data not protected", kind, err)
		}
		var child map[string]any
		query("GET", path+"/"+kind+"/keep?project="+r.project, nil, &child)
		if child == nil {
			t.Fatal("native saved child missing")
		}
		if present, err := backend.workspaceVolumeForDeletion(ctx, object); err != nil || present == nil {
			t.Fatal("parent lost", err)
		}
		query("DELETE", path+"/"+kind+"/keep?project="+r.project, nil, nil)
	}
	must(backend.DeleteWorkspaceVolume(ctx, object))
	if present, err := backend.workspaceVolumeForDeletion(ctx, object); err != nil || present != nil {
		t.Fatal("owned volume remains", err)
	}
	t.Log("PASS real Incus/Btrfs snapshot and backup refusal, child/parent retention, exact child deletion, owned volume deletion and positive absence; metadata receipt retained")
}
