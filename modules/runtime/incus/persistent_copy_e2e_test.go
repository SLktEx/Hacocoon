package incus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/persistentresource"
	"github.com/SLktEx/Hacocoon/internal/state"
)

// A dedicated test pool/project contains synthetic data only. This is provider
// COW acceptance, not packaged CLI, OCI runtime or credential acceptance.
func TestRealIncusPersistentCopyE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_INCUS_PERSISTENT_COPY") != "1" {
		t.Skip("set HACO_E2E_INCUS_PERSISTENT_COPY=1 on a root Linux/WSL Incus host with Btrfs")
	}
	if os.Geteuid() != 0 {
		t.Fatal("root required to independently inspect Btrfs UUIDs")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	runner := host.ExecRunner{}
	command := func(name string, args ...string) string {
		t.Helper()
		result, err := runner.Run(ctx, name, args...)
		if err != nil {
			t.Fatalf("%s %v: %v %s", name, args, err, result.Stderr)
		}
		return result.Stdout
	}
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	pool := "haco-copy-" + hex.EncodeToString(nonce[:])
	project := pool
	command("incus", "storage", "create", pool, "btrfs", "size=1GiB")
	runtime := New(runner)
	runtime.project = project
	runtime.setRootPool(pool)
	if err := runtime.ensureProject(ctx); err != nil {
		t.Fatal(err)
	}
	svc := &persistentresource.Service{Store: state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json")), Backend: &PersistentResourceBackend{Runtime: runtime}}
	t.Logf("test-owned pool/project: %s; failure retains exact resources for inspection", pool)
	source, err := svc.Create(ctx, "oci:source", OCIStoreKind)
	if err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join("/var/lib/incus/storage-pools", pool, "custom", project+"_haco-persistent-"+source.Owner)
	// The path comes only from fresh test ownership, never a guest/backend path.
	if info, err := os.Lstat(sourcePath); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("test volume path unavailable: %v", err)
	}
	marker := []byte("independent OCI Store copy test\n")
	if err := os.WriteFile(filepath.Join(sourcePath, "marker"), marker, 0600); err != nil {
		t.Fatal(err)
	}
	target, err := svc.Copy(ctx, "oci:target", OCIStoreKind, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join("/var/lib/incus/storage-pools", pool, "custom", project+"_haco-persistent-"+target.Owner)
	read := func(path string) string {
		t.Helper()
		value, err := os.ReadFile(filepath.Join(path, "marker"))
		if err != nil {
			t.Fatal(err)
		}
		return string(value)
	}
	if read(targetPath) != string(marker) {
		t.Fatal("source contents not copied")
	}
	field := func(output, key string) string {
		for _, line := range strings.Split(output, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), key+":") {
				return strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			}
		}
		return ""
	}
	originalUUID := field(command("btrfs", "subvolume", "show", sourcePath), "UUID")
	copied := command("btrfs", "subvolume", "show", targetPath)
	if originalUUID == "" || originalUUID == "-" || field(copied, "Parent UUID") != originalUUID || field(copied, "UUID") == originalUUID {
		t.Fatalf("COW ancestry not proven: source %s target %s", originalUUID, copied)
	}
	if err := os.WriteFile(filepath.Join(targetPath, "marker"), []byte("copy edited\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if read(sourcePath) != string(marker) {
		t.Fatal("copy writes changed source")
	}
	if err := os.WriteFile(filepath.Join(sourcePath, "marker"), []byte("source edited\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if read(targetPath) != "copy edited\n" {
		t.Fatal("source writes changed copy")
	}
	if err := svc.Delete(ctx, source.ID); err != nil {
		t.Fatal(err)
	}
	if read(targetPath) != "copy edited\n" {
		t.Fatal("source deletion lost copy")
	}
	if err := svc.Delete(ctx, target.ID); err != nil {
		t.Fatal(err)
	}
	command("incus", "project", "delete", project)
	command("incus", "storage", "delete", pool)
	t.Log(fmt.Sprintf("PASS independent contents, bidirectional mutation isolation, source deletion and Btrfs parent UUID %s", originalUUID))
}
