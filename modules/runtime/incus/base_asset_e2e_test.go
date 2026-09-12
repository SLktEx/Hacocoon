package incus

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/baseasset"
	"github.com/SLktEx/Hacocoon/internal/core"
	environmentapp "github.com/SLktEx/Hacocoon/internal/environment"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/state"
)

func TestRealIncusBaseAssetE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_BASE_ASSET") != "1" {
		t.Skip("set HACO_E2E_BASE_ASSET=1 on a dedicated root Incus/Btrfs host")
	}
	pool, image := os.Getenv("HACO_E2E_INCUS_RESUME_POOL"), os.Getenv("HACO_E2E_INCUS_RESUME_IMAGE")
	if os.Geteuid() != 0 || !safeIncusRef(pool) || !baseFingerprintPattern.MatchString(image) {
		t.Fatal("root and explicit pool/full image required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	r := New(host.ExecRunner{})
	command := func(args ...string) string {
		t.Helper()
		out, err := r.runner.Run(ctx, "incus", args...)
		if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
			t.Fatal("Incus fixture operation failed", err)
		}
		return out.Stdout
	}
	var cached struct{ Fingerprint, Type string }
	must(json.Unmarshal([]byte(command("query", "/1.0/images/"+image+"?project="+r.project)), &cached))
	if cached.Fingerprint != image || cached.Type != "container" {
		t.Fatal("specified cached Base unavailable")
	}
	dir, err := os.MkdirTemp("/var/lib", "haco-base-asset-")
	must(err)
	t.Logf("durable ownership catalog %s", dir)
	path := filepath.Join(dir, "state.json")
	catalog := state.NewEnvironmentJSONStore(path)
	provider, err := NewBaseProvider(r)
	must(err)
	base := core.BaseRef{Name: "fixture/base", Revision: core.BaseRevision("sha256:" + image)}
	provider.sources[base.Name] = "local:" + image
	backend := &BaseAssetBackend{Provider: provider}
	service := baseasset.Service{Store: catalog, Backend: backend, Provider: environmentapp.ProviderIncus}
	asset, err := service.Ensure(ctx, base, r.project+"/"+pool)
	must(err)
	if asset.State != "ready" {
		t.Fatal("incomplete asset")
	}
	p, _, err := backend.decode(asset)
	must(err)
	t.Logf("ready asset %s; native %s", asset.ID, p.target())
	if os.Getenv("HACO_E2E_BASE_ASSET_DELETE_IMAGE") == "1" {
		// Only a dedicated fixture may consume its explicitly selected cache entry.
		// Refuse shared project images and any other instance depending on this Base.
		var project struct {
			Name   string
			Config map[string]string
		}
		must(json.Unmarshal([]byte(command("query", "/1.0/projects/"+r.project)), &project))
		if project.Name != r.project || project.Config["features.images"] != "true" {
			t.Fatal("image project isolation unproven; retained asset preserved")
		}
		var instances []snapshotInstanceObservation
		must(json.Unmarshal([]byte(command("query", "/1.0/instances?project="+r.project+"&recursion=1")), &instances))
		if instances == nil {
			t.Fatal("instance inventory unproven")
		}
		seen := map[string]bool{}
		for _, instance := range instances {
			if instance.Name == "" || instance.Config == nil || instance.ExpandedConfig == nil || seen[instance.Name] {
				t.Fatal("instance inventory incomplete or duplicated")
			}
			seen[instance.Name] = true
			if instance.Name != p.target() && (instance.Config["volatile.base_image"] == image || instance.ExpandedConfig["volatile.base_image"] == image) {
				t.Fatal("selected Base still used by another instance; retained asset preserved")
			}
		}
		must(backend.Verify(ctx, asset))
		command("image", "delete", image, "--project", r.project)
		var images []struct{ Fingerprint string }
		must(json.Unmarshal([]byte(command("query", "/1.0/images?project="+r.project+"&recursion=1")), &images))
		if images == nil {
			t.Fatal("image absence unproven")
		}
		for _, i := range images {
			if i.Fingerprint == image {
				t.Fatal("source image remains")
			}
		}
		t.Log("source Base image deleted and absence verified")
	} else {
		t.Log("SKIP source image deletion: explicit dedicated-image permission not enabled")
	}
	service.Store = state.NewEnvironmentJSONStore(path)
	reused, err := service.Ensure(ctx, base, r.project+"/"+pool)
	must(err)
	if reused != asset {
		t.Fatal("recreated or substituted Base after reload")
	}
	root := filepath.Join("/var/lib/incus/storage-pools", pool, "containers", r.project+"_"+p.target(), "rootfs")
	scoped, err := os.OpenRoot(root)
	must(err)
	file, err := scoped.Open("usr/lib/os-release")
	must(err)
	data, err := io.ReadAll(io.LimitReader(file, 65536))
	must(err)
	must(file.Close())
	must(scoped.Close())
	if !strings.Contains(string(data), "ID=ubuntu") {
		t.Fatal("retained Base rootfs unavailable")
	}
	// This private fixture owns the entire catalog, and creates no Environments
	// or snapshots referencing it. Keep its receipt until exact provider absence.
	must(r.deleteBaseStorage(ctx, p))
	entries, err := os.ReadDir(dir)
	must(err)
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			t.Fatal("unexpected recovery entry")
		}
		must(os.Remove(filepath.Join(dir, entry.Name())))
	}
	must(os.Remove(dir))
	t.Log("PASS durable Base creation/verification, catalog restart and exact reuse, retained rootfs read, positive-absence cleanup")
}
