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

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/persistentresource"
	"github.com/SLktEx/Hacocoon/internal/state"
	ociplugin "github.com/SLktEx/Hacocoon/modules/plugin/oci"
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
	var sourcePath string
	marker := []byte("independent OCI Store copy test\n")
	source, err := svc.PublishSource(ctx, ociplugin.HostStoreID, OCIStoreKind, func(_ context.Context, r core.PersistentResource) error {
		sourcePath = filepath.Join("/var/lib/incus/storage-pools", pool, "custom", project+"_haco-persistent-"+r.Owner)
		// Derived only from fresh test ownership, never guest/backend paths.
		info, err := os.Lstat(sourcePath)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return core.ErrIncompatibleState
		}
		return os.WriteFile(filepath.Join(sourcePath, "marker"), marker, 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
	target, err := (ociplugin.WorkspaceStores{Resources: svc}).Resolve(ctx, core.Workspace{ID: "automatic-copy-work"})
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

// This uses a real running owned Host and its attached data area. It proves the
// pause/COW/resume provider mechanism, not Docker/containerd crash recovery.
func TestRealIncusHostAreaCopyE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_INCUS_HOST_AREA_COPY") != "1" {
		t.Skip("set HACO_E2E_INCUS_HOST_AREA_COPY=1 on a root Incus/Btrfs host")
	}
	if os.Geteuid() != 0 {
		t.Fatal("root required for independent Btrfs inspection")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Minute)
	defer cancel()
	runner := host.ExecRunner{}
	command := func(args ...string) string {
		t.Helper()
		result, err := runner.Run(ctx, "incus", args...)
		if err != nil || result.ExitCode != 0 || result.StdoutTruncated {
			t.Fatalf("fixture Incus command failed: %v", args)
		}
		return result.Stdout
	}
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	project := "haco-area-" + hex.EncodeToString(nonce[:])
	pool := project
	t.Logf("test-owned pool/project %s; failure retains ownership and paused Host for recovery", project)
	command("storage", "create", pool, "btrfs", "size=4GiB")
	runtime := New(runner)
	runtime.project = project
	runtime.setRootPool(pool)
	command("project", "create", project, "--config", "features.profiles=false")
	stateRoot, err := os.MkdirTemp("", "haco-host-area-state-")
	if err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(stateRoot, "state.json")
	t.Logf("failure retains recovery catalog at %s", statePath)
	service := &persistentresource.Service{Store: state.NewEnvironmentJSONStore(statePath), Backend: &PersistentResourceBackend{Runtime: runtime}}
	source, err := service.PublishSource(ctx, ociplugin.HostStoreID, OCIStoreKind, func(context.Context, core.PersistentResource) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	command("launch", defaultImage, trustedHostName, "--project", project, "--storage", pool, "--no-profiles", "--config", trustedHostRoleKey+"="+trustedHostRoleValue, "--config", "boot.autostart=true")
	imageFingerprint := strings.TrimSpace(command("config", "get", trustedHostName, "volatile.base_image", "--project", project))
	if decoded, err := hex.DecodeString(imageFingerprint); err != nil || len(decoded) != 32 {
		t.Fatal("fixture Base identity unavailable")
	}
	command("config", "device", "add", trustedHostName, "oci-source", "disk", "pool="+pool, "source=haco-persistent-"+source.Owner, "path="+OCIStorePath, "--project", project)
	command("exec", trustedHostName, "--project", project, "--", "/bin/sh", "-ec", "printf 'Host area content\\n' > /var/lib/hacocoon-oci/marker; sync")
	target, err := (ociplugin.WorkspaceStores{Resources: service}).Resolve(ctx, core.Workspace{ID: "area-copy-work"})
	if err != nil {
		t.Fatal(err)
	}
	backend := &PersistentResourceBackend{Runtime: runtime}
	instance, err := backend.hostCopyInstance(ctx, source)
	if err != nil || instance.StatusCode != 103 || instance.Config[hostOCICopyKey] != "" || instance.Config["boot.autostart"] != "true" {
		t.Fatalf("Host not safely resumed: %v", err)
	}
	pathFor := func(resource core.PersistentResource) string {
		return filepath.Join("/var/lib/incus/storage-pools", pool, "custom", project+"_haco-persistent-"+resource.Owner)
	}
	sourcePath, targetPath := pathFor(source), pathFor(target)
	read := func(path string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(path, "marker"))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	original := read(sourcePath)
	if original == "" || read(targetPath) != original {
		t.Fatal("Host area not copied")
	}
	btrfs := func(path string) string {
		t.Helper()
		result, err := runner.Run(ctx, "btrfs", "subvolume", "show", path)
		if err != nil {
			t.Fatal(err)
		}
		return result.Stdout
	}
	field := func(text, key string) string {
		for _, line := range strings.Split(text, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), key+":") {
				return strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			}
		}
		return ""
	}
	uuid := field(btrfs(sourcePath), "UUID")
	copied := btrfs(targetPath)
	if uuid == "" || uuid == "-" || field(copied, "Parent UUID") != uuid || field(copied, "UUID") == uuid {
		t.Fatal("Btrfs COW ancestry missing")
	}
	if err := os.WriteFile(filepath.Join(targetPath, "marker"), []byte("copy changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if read(sourcePath) != original {
		t.Fatal("copy changed Host area")
	}
	command("exec", trustedHostName, "--project", project, "--", "/bin/sh", "-ec", "printf 'Host changed' > /var/lib/hacocoon-oci/marker; sync")
	if read(targetPath) != "copy changed" {
		t.Fatal("Host changed copy")
	}
	// Only delete this freshly owned fixture Host, then the positively detached area.
	if _, err := backend.hostCopyInstance(ctx, source); err != nil {
		t.Fatal(err)
	}
	command("delete", trustedHostName, "--project", project, "--force")
	if err := service.Delete(ctx, source.ID); err != nil {
		t.Fatal(err)
	}
	if read(targetPath) != "copy changed" {
		t.Fatal("source deletion affected copy")
	}
	if err := service.Delete(ctx, target.ID); err != nil {
		t.Fatal(err)
	}
	t.Log("PASS Host pause/COW/resume and bidirectional write/deletion independence; cleaning fixture Base")
	// This exact image was introduced by launch into the freshly created project.
	command("image", "delete", imageFingerprint, "--project", project)
	command("project", "delete", project)
	command("storage", "delete", pool)
	for _, path := range []string{statePath, statePath + ".lock"} {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(stateRoot); err != nil {
		t.Fatal(err)
	}

	t.Log("PASS running Host pause, attached-area Btrfs COW, resume, independent writes/deletion and owned cleanup")
}
