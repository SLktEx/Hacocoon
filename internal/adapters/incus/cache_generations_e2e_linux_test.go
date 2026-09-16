package incus

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/state"
	"github.com/SLktEx/Hacocoon/internal/storage/resource"
)

// Synthetic provider acceptance only: this does not collect paths from an Env,
// establish large-repository performance or use existing installation data.
func TestRealIncusCacheGenerationsE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_INCUS_CACHE_GENERATIONS") != "1" {
		t.Skip("set HACO_E2E_INCUS_CACHE_GENERATIONS=1 on a root Incus/Btrfs host")
	}
	if os.Geteuid() != 0 {
		t.Fatal("root required for independent extent measurement")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	runner := host.ExecRunner{}
	command := func(name string, args ...string) string {
		t.Helper()
		result, err := runner.Run(ctx, name, args...)
		if err != nil || result.ExitCode != 0 || result.StdoutTruncated {
			t.Fatalf("fixture command failed: %s %v: %v exit=%d", name, args, err, result.ExitCode)
		}
		return result.Stdout
	}
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	pool := "haco-cache-" + hex.EncodeToString(nonce[:])
	project := pool
	receipt, err := os.MkdirTemp("/var/lib", "haco-cache-generation-")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("test-owned pool/project=%s catalog=%s; failure retains exact fixture resources", pool, receipt)
	command("incus", "storage", "create", pool, "btrfs", "size=1GiB")
	runtime := New(runner)
	runtime.project = project
	runtime.setRootPool(pool)
	if err := runtime.ensureProject(ctx); err != nil {
		t.Fatal(err)
	}
	store := state.NewEnvironmentJSONStore(filepath.Join(receipt, "state.json"))
	svc := &persistentresource.Service{Store: store, Backend: &PersistentResourceBackend{Runtime: runtime}}
	initial, err := store.EnsureResourceGeneration(ctx, "fixture", "build-cache", strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	ownedPath := func(r core.PersistentResource) string {
		t.Helper()
		actualPool, volume, err := managedResourceVolume(r)
		if err != nil || actualPool != pool {
			t.Fatal("fixture ownership mismatch", err)
		}
		path := filepath.Join("/var/lib/incus/storage-pools", pool, "custom", project+"_"+volume)
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			t.Fatal("fixture path unavailable", err)
		}
		return path
	}
	payload := make([]byte, 8*1024*1024)
	if _, err := rand.Read(payload); err != nil {
		t.Fatal(err)
	}
	publication, err := svc.PublishGeneration(ctx, initial, func(_ context.Context, r core.PersistentResource) error {
		f, err := os.OpenFile(filepath.Join(ownedPath(r), "payload"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		_, writeErr := f.Write(payload)
		syncErr := f.Sync()
		closeErr := f.Close()
		return errors.Join(writeErr, syncErr, closeErr)
	})
	if err != nil || publication.State != "published" {
		t.Fatal(publication, err)
	}
	source := publication.Candidate
	copies := make([]core.PersistentResource, 2)
	started := time.Now()
	for i, id := range []string{"cache:first", "cache:second"} {
		copies[i], err = svc.Copy(ctx, id, CacheResourceKind, source.ID)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("two 8 MiB provider copies elapsed=%s", time.Since(started))
	sourcePath := ownedPath(source)
	paths := []string{sourcePath, ownedPath(copies[0]), ownedPath(copies[1])}
	command("btrfs", "filesystem", "sync", sourcePath)
	du := command("btrfs", append([]string{"filesystem", "du", "--raw", "-s"}, paths...)...)
	measured := 0
	for _, line := range strings.Split(du, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		total, e1 := strconv.ParseUint(fields[0], 10, 64)
		exclusive, e2 := strconv.ParseUint(fields[1], 10, 64)
		shared, e3 := strconv.ParseUint(fields[2], 10, 64)
		if e1 != nil || e2 != nil || e3 != nil {
			continue
		}
		if total < uint64(len(payload)) || shared == 0 || exclusive >= total {
			t.Fatalf("shared extents not proven: %s", line)
		}
		measured++
		t.Logf("extent bytes total=%d exclusive=%d set_shared=%d path=%s", total, exclusive, shared, fields[3])
	}
	if measured != 3 {
		t.Fatalf("expected three measured volumes: %s", du)
	}
	read := func(path string) []byte {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(path, "payload"))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	for _, path := range paths {
		if !bytes.Equal(read(path), payload) {
			t.Fatal("copy content mismatch")
		}
	}
	if err := os.WriteFile(filepath.Join(paths[1], "payload"), []byte("changed copy"), 0600); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(read(paths[0]), payload) || !bytes.Equal(read(paths[2]), payload) {
		t.Fatal("copy mutation escaped")
	}
	if err := svc.Delete(ctx, source.ID); !errors.Is(err, core.ErrStorageBusy) {
		t.Fatal("current source deleted", err)
	}
	if _, err := store.ResetResourceGeneration(ctx, publication.Generation, initial.Compatibility); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, source.ID); err != nil {
		t.Fatal(err)
	}
	if string(read(paths[1])) != "changed copy" || !bytes.Equal(read(paths[2]), payload) {
		t.Fatal("source deletion lost copies")
	}
	volume := "haco-persistent-" + copies[1].Owner
	command("incus", "storage", "volume", "snapshot", "create", pool, volume, "keep", "--project", project)
	if err := svc.DeleteReviewed(ctx, copies[1].Ref()); !errors.Is(err, core.ErrStorageBusy) {
		t.Fatal("snapshot reference was not protected", err)
	}
	command("incus", "storage", "volume", "snapshot", "delete", pool, volume, "keep", "--project", project)
	for _, copy := range copies {
		if err := svc.DeleteReviewed(ctx, copy.Ref()); err != nil {
			t.Fatal(err)
		}
	}
	retained, err := store.ListPersistentResources(ctx)
	if err != nil || len(retained) != 0 {
		t.Fatal("fixture catalog not empty", retained, err)
	}
	command("incus", "project", "delete", project)
	command("incus", "storage", "delete", pool)
	t.Log("PASS complete-source selection, measured shared extents, independent mutation/source deletion, snapshot refusal and owned fixture cleanup; ordinary-Env collection remains unverified")
}
