package incus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestRealIncusSnapshotVolumesE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_SNAPSHOT_VOLUME") != "1" {
		t.Skip("set HACO_E2E_SNAPSHOT_VOLUME=1 with a cached image and dedicated root Incus/Btrfs host")
	}
	pool, image := os.Getenv("HACO_E2E_INCUS_RESUME_POOL"), os.Getenv("HACO_E2E_INCUS_RESUME_IMAGE")
	if os.Geteuid() != 0 || !safeIncusRef(pool) || !baseFingerprintPattern.MatchString(image) {
		t.Fatal("explicit pool, full cached image fingerprint and root required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	r := New(host.ExecRunner{})
	command := func(name string, args ...string) string {
		t.Helper()
		out, err := r.runner.Run(ctx, name, args...)
		if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
			t.Fatalf("%s %v failed: %v", name, args, err)
		}
		return out.Stdout
	}
	random := func() string {
		var nonce [16]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			t.Fatal(err)
		}
		return hex.EncodeToString(nonce[:])
	}
	instance := "haco-snap-probe-" + random()[:16]
	id, err := core.NewEnvironmentInstanceID()
	if err != nil {
		t.Fatal(err)
	}
	plans := []snapshotVolumePlan{}
	for _, kind := range []string{"work", "oci"} {
		p := snapshotVolumeFixture(kind)
		p.Pool = pool
		p.SourceInstance = instance
		p.SourceInstanceID = id
		p.SourceOwner = random()
		p.Owner = random()
		if kind == "work" {
			p.Source = "haco-work-probe-" + random()[:16]
		} else {
			p.Source = "haco-persistent-" + p.SourceOwner
		}
		plans = append(plans, p)
	}
	// Keep exact intended identities on disk before any creation. A failed test
	// deliberately retains these owned resources and this recovery record.
	stateDir, err := os.MkdirTemp("", "haco-snapshot-volume-")
	if err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(stateDir, "plan.json")
	data, err := json.Marshal(plans)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(planPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := file.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	t.Logf("owned fixture %s; failure recovery identities: %s", instance, planPath)
	command("incus", "init", "local:"+image, instance, "--project", r.project, "--no-profiles", "--storage", pool, "--config", environmentInstanceKey+"="+id)
	for _, p := range plans {
		source := snapshotSourceObservation(p)
		source.UsedBy = nil
		delete(source.Config, "security.unmapped")
		delete(source.Config, "volatile.idmap.last")
		data, err := json.Marshal(map[string]any{"name": p.Source, "type": "custom", "content_type": "filesystem", "config": source.Config})
		if err != nil {
			t.Fatal(err)
		}
		command("incus", "query", "-X", "POST", "--wait", "/1.0/storage-pools/"+pool+"/volumes/custom?project="+r.project, "--data", string(data))
		path := func(name string) string {
			return filepath.Join("/var/lib/incus/storage-pools", pool, "custom", r.project+"_"+name)
		}
		original, saved := path(p.Source), path(p.target())
		info, err := os.Lstat(original)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			t.Fatal("new owned source path unavailable", err)
		}
		files := map[string]string{"untracked": "uncommitted bytes", ".git/objects/local": "unpushed object", "containerd/content/blob": "containerd data", "docker/volumes/data": "docker data"}
		for name, value := range files {
			full := filepath.Join(original, name)
			if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, []byte(value), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Link(filepath.Join(original, "untracked"), filepath.Join(original, "hardlink")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("untracked", filepath.Join(original, "symlink")); err != nil {
			t.Fatal(err)
		}
		command("sync")
		if err := r.createSnapshotVolume(ctx, p); err != nil {
			t.Fatal(err)
		}
		if err := r.verifySnapshotVolume(ctx, p); err != nil {
			t.Fatal(err)
		}
		for name, want := range files {
			got, err := os.ReadFile(filepath.Join(saved, name))
			if err != nil || string(got) != want {
				t.Fatalf("lost %s: %v", name, err)
			}
		}
		a, err := os.Stat(filepath.Join(saved, "untracked"))
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.Stat(filepath.Join(saved, "hardlink"))
		if err != nil || !os.SameFile(a, b) {
			t.Fatal("hardlink lost", err)
		}
		if value, err := os.Readlink(filepath.Join(saved, "symlink")); err != nil || value != "untracked" {
			t.Fatal("symlink lost", err)
		}
		field := func(output, key string) string {
			for _, line := range strings.Split(output, "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), key+":") {
					return strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
				}
			}
			return ""
		}
		uuid := field(command("btrfs", "subvolume", "show", original), "UUID")
		copied := command("btrfs", "subvolume", "show", saved)
		if uuid == "" || uuid == "-" || field(copied, "Parent UUID") != uuid || field(copied, "UUID") == uuid {
			t.Fatal("COW ancestry not proven")
		}
		if err := os.WriteFile(filepath.Join(saved, "untracked"), []byte("saved changed"), 0600); err != nil {
			t.Fatal(err)
		}
		if got, err := os.ReadFile(filepath.Join(original, "untracked")); err != nil || string(got) != "uncommitted bytes" {
			t.Fatal("copy changed source", err)
		}
		if err := os.WriteFile(filepath.Join(original, "untracked"), []byte("source changed"), 0600); err != nil {
			t.Fatal(err)
		}
		if got, err := os.ReadFile(filepath.Join(saved, "untracked")); err != nil || string(got) != "saved changed" {
			t.Fatal("source changed copy", err)
		}
		if v, err := r.snapshotVolumeObservation(ctx, p, false); err != nil || v == nil || len(v.UsedBy) != 0 {
			t.Fatal("source cleanup identity uncertain", err)
		}
		command("incus", "storage", "volume", "delete", pool, p.Source, "--project", r.project)
		if v, err := r.snapshotVolumeObservation(ctx, p, false); err != nil || v != nil {
			t.Fatal("source absence unproven", err)
		}
		if got, err := os.ReadFile(filepath.Join(saved, "untracked")); err != nil || string(got) != "saved changed" {
			t.Fatal("source deletion lost saved data", err)
		}
		if err := r.deleteSnapshotVolume(ctx, p); err != nil {
			t.Fatal(err)
		}
		t.Logf("PASS %s COW parent UUID %s, all file data/links, mutation isolation, source deletion, owned cleanup", p.Role, uuid)
	}
	if err := r.VerifyEnvironmentIdentity(ctx, instance, id); err != nil {
		t.Fatal(err)
	}
	command("incus", "delete", instance, "--project", r.project)
	if exists, err := r.environmentExists(ctx, instance); err != nil || exists {
		t.Fatal("instance cleanup unproven", err)
	}
	if err := os.Remove(planPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(stateDir); err != nil {
		t.Fatal(err)
	}
}
