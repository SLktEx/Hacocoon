package localqcow2

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/modules/storage/btrfs/internal/block"
)

type fakeNBDFixture struct {
	mu             sync.Mutex
	devRoot        string
	sysBlockRoot   string
	procRoot       string
	nextPID        int
	calls          []string
	failDisconnect bool
}

func newFakeNBDFixture(t *testing.T, devices int) *fakeNBDFixture {
	t.Helper()
	root := t.TempDir()
	fixture := &fakeNBDFixture{
		devRoot:      filepath.Join(root, "dev"),
		sysBlockRoot: filepath.Join(root, "sys", "block"),
		procRoot:     filepath.Join(root, "proc"),
		nextPID:      4100,
	}
	for _, dir := range []string{fixture.devRoot, fixture.sysBlockRoot, fixture.procRoot} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < devices; i++ {
		name := fmt.Sprintf("nbd%d", i)
		if err := os.WriteFile(filepath.Join(fixture.devRoot, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(fixture.sysBlockRoot, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return fixture
}

func (f *fakeNBDFixture) store() *Store {
	store := New(f)
	store.devRoot = f.devRoot
	store.sysBlockRoot = f.sysBlockRoot
	store.procRoot = f.procRoot
	return store
}

func (f *fakeNBDFixture) Run(_ context.Context, name string, args ...string) (host.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	call := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, call)
	if name != "qemu-nbd" {
		return host.Result{}, nil
	}
	if len(args) == 2 && strings.HasPrefix(args[0], "--connect=") {
		device := strings.TrimPrefix(args[0], "--connect=")
		if err := f.addMappingLocked(device, args[1]); err != nil {
			return host.Result{ExitCode: 1}, err
		}
		return host.Result{}, nil
	}
	if len(args) == 2 && args[0] == "--disconnect" {
		if f.failDisconnect {
			return host.Result{ExitCode: 1}, errors.New("forced disconnect failure")
		}
		if err := f.removeMappingLocked(args[1]); err != nil {
			return host.Result{ExitCode: 1}, err
		}
		return host.Result{}, nil
	}
	return host.Result{}, nil
}

func (f *fakeNBDFixture) addMapping(t *testing.T, device, backing string) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.addMappingLocked(device, backing); err != nil {
		t.Fatal(err)
	}
}

func (f *fakeNBDFixture) addMappingLocked(device, backing string) error {
	name := filepath.Base(device)
	pidPath := filepath.Join(f.sysBlockRoot, name, "pid")
	if _, err := os.Stat(pidPath); err == nil {
		return fmt.Errorf("device already busy: %s", device)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	pid := f.nextPID
	f.nextPID++
	if err := os.WriteFile(pidPath, []byte(fmt.Sprintf("%d\n", pid)), 0o600); err != nil {
		return err
	}
	procDir := filepath.Join(f.procRoot, fmt.Sprintf("%d", pid))
	fdDir := filepath.Join(procDir, "fd")
	if err := os.MkdirAll(fdDir, 0o700); err != nil {
		return err
	}
	cmdline := []byte("/usr/bin/qemu-nbd\x00--connect=" + device + "\x00" + backing + "\x00")
	if err := os.WriteFile(filepath.Join(procDir, "cmdline"), cmdline, 0o600); err != nil {
		return err
	}
	return os.Symlink(backing, filepath.Join(fdDir, "3"))
}

func (f *fakeNBDFixture) removeMappingLocked(device string) error {
	name := filepath.Base(device)
	pidPath := filepath.Join(f.sysBlockRoot, name, "pid")
	rawPID, err := os.ReadFile(pidPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	pid := strings.TrimSpace(string(rawPID))
	if err := os.Remove(pidPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.RemoveAll(filepath.Join(f.procRoot, pid))
}

func (f *fakeNBDFixture) replaceMapping(t *testing.T, device, backing string) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.removeMappingLocked(device); err != nil {
		t.Fatal(err)
	}
	if err := f.addMappingLocked(device, backing); err != nil {
		t.Fatal(err)
	}
}

func (f *fakeNBDFixture) countCalls(needle string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	count := 0
	for _, call := range f.calls {
		if strings.Contains(call, needle) {
			count++
		}
	}
	return count
}

func createQCOW2Fixture(t *testing.T, root, name string) string {
	t.Helper()
	images := filepath.Join(root, "images")
	if err := os.MkdirAll(images, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(images, name+".qcow2")
	if err := os.WriteFile(path, []byte("qcow2-"+name), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRestartRecoversPersistedNBDIdentityBeforeDetach(t *testing.T) {
	fixture := newFakeNBDFixture(t, 2)
	storageRoot := filepath.Join(t.TempDir(), "storage")
	backing := createQCOW2Fixture(t, storageRoot, "demo")
	first := fixture.store()
	attached, err := first.Attach(context.Background(), block.Handle{ID: "demo", Path: backing})
	if err != nil {
		t.Fatal(err)
	}
	if attached.Device == "" {
		t.Fatal("attach returned no device")
	}
	if _, err := os.Stat(nbdStatePath(backing)); err != nil {
		t.Fatalf("durable NBD identity missing: %v", err)
	}

	restarted := fixture.store()
	if err := restarted.Detach(context.Background(), block.Handle{ID: "demo", Path: backing}); err != nil {
		t.Fatal(err)
	}
	if fixture.countCalls("qemu-nbd --disconnect "+attached.Device) != 1 {
		t.Fatalf("restart did not disconnect reconstructed device: %v", fixture.calls)
	}
	if _, err := os.Stat(nbdStatePath(backing)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("identity sidecar survived verified detach: %v", err)
	}
}

func TestDeleteRejectsStaleDeviceReusedForDifferentBacking(t *testing.T) {
	fixture := newFakeNBDFixture(t, 2)
	storageRoot := filepath.Join(t.TempDir(), "storage")
	victim := createQCOW2Fixture(t, storageRoot, "victim")
	other := createQCOW2Fixture(t, storageRoot, "other")
	store := fixture.store()
	attached, err := store.Attach(context.Background(), block.Handle{ID: "victim", Path: victim})
	if err != nil {
		t.Fatal(err)
	}
	fixture.replaceMapping(t, attached.Device, other)
	beforeDisconnect := fixture.countCalls("qemu-nbd --disconnect")

	err = fixture.store().Delete(context.Background(), block.Handle{ID: "victim", Path: victim})
	if !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("stale device reuse err=%v, want recovery-required", err)
	}
	if fixture.countCalls("qemu-nbd --disconnect") != beforeDisconnect {
		t.Fatalf("stale device reuse disconnected somebody else's NBD: %v", fixture.calls)
	}
	if _, err := os.Stat(victim); err != nil {
		t.Fatalf("victim backing was removed: %v", err)
	}
}

func TestMissingSidecarReconstructsUniqueLiveMapping(t *testing.T) {
	fixture := newFakeNBDFixture(t, 2)
	storageRoot := filepath.Join(t.TempDir(), "storage")
	backing := createQCOW2Fixture(t, storageRoot, "demo")
	device := filepath.Join(fixture.devRoot, "nbd1")
	fixture.addMapping(t, device, backing)

	if err := fixture.store().Detach(context.Background(), block.Handle{ID: "demo", Path: backing}); err != nil {
		t.Fatal(err)
	}
	if fixture.countCalls("qemu-nbd --disconnect "+device) != 1 {
		t.Fatalf("unique live mapping was not reconstructed: %v", fixture.calls)
	}
}

func TestAmbiguousLiveMappingsFailClosedBeforeDelete(t *testing.T) {
	fixture := newFakeNBDFixture(t, 2)
	storageRoot := filepath.Join(t.TempDir(), "storage")
	backing := createQCOW2Fixture(t, storageRoot, "demo")
	fixture.addMapping(t, filepath.Join(fixture.devRoot, "nbd0"), backing)
	fixture.addMapping(t, filepath.Join(fixture.devRoot, "nbd1"), backing)

	err := fixture.store().Delete(context.Background(), block.Handle{ID: "demo", Path: backing})
	if !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("ambiguous mapping err=%v, want recovery-required", err)
	}
	if fixture.countCalls("qemu-nbd --disconnect") != 0 {
		t.Fatalf("ambiguous mapping caused disconnect: %v", fixture.calls)
	}
	if _, err := os.Stat(backing); err != nil {
		t.Fatalf("ambiguous mapping removed backing: %v", err)
	}
}

func TestConcurrentAttachUsesDistinctNBDDevices(t *testing.T) {
	fixture := newFakeNBDFixture(t, 2)
	storageRoot := filepath.Join(t.TempDir(), "storage")
	firstBacking := createQCOW2Fixture(t, storageRoot, "first")
	secondBacking := createQCOW2Fixture(t, storageRoot, "second")

	var first, second block.Handle
	var firstErr, secondErr error
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		first, firstErr = fixture.store().Attach(context.Background(), block.Handle{ID: "first", Path: firstBacking})
	}()
	go func() {
		defer wg.Done()
		<-start
		second, secondErr = fixture.store().Attach(context.Background(), block.Handle{ID: "second", Path: secondBacking})
	}()
	close(start)
	wg.Wait()
	if firstErr != nil || secondErr != nil {
		t.Fatalf("concurrent attach errors: first=%v second=%v", firstErr, secondErr)
	}
	if first.Device == "" || second.Device == "" || first.Device == second.Device {
		t.Fatalf("allocator reused one NBD: first=%#v second=%#v", first, second)
	}
}

func TestDisconnectFailureRetainsBackingAndIdentity(t *testing.T) {
	fixture := newFakeNBDFixture(t, 1)
	storageRoot := filepath.Join(t.TempDir(), "storage")
	backing := createQCOW2Fixture(t, storageRoot, "demo")
	store := fixture.store()
	if _, err := store.Attach(context.Background(), block.Handle{ID: "demo", Path: backing}); err != nil {
		t.Fatal(err)
	}
	fixture.failDisconnect = true

	err := fixture.store().Delete(context.Background(), block.Handle{ID: "demo", Path: backing})
	if err == nil {
		t.Fatal("delete succeeded despite disconnect failure")
	}
	if _, err := os.Stat(backing); err != nil {
		t.Fatalf("backing removed after disconnect failure: %v", err)
	}
	if _, err := os.Stat(nbdStatePath(backing)); err != nil {
		t.Fatalf("identity state removed after disconnect failure: %v", err)
	}
}

func TestBackingPathReplacementCannotReusePersistedNBDIdentity(t *testing.T) {
	fixture := newFakeNBDFixture(t, 1)
	storageRoot := filepath.Join(t.TempDir(), "storage")
	backing := createQCOW2Fixture(t, storageRoot, "demo")
	if _, err := fixture.store().Attach(context.Background(), block.Handle{ID: "demo", Path: backing}); err != nil {
		t.Fatal(err)
	}
	old := backing + ".old"
	if err := os.Rename(backing, old); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backing, []byte("replacement"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := fixture.store().Delete(context.Background(), block.Handle{ID: "demo", Path: backing})
	if !errors.Is(err, core.ErrRecoveryRequired) || !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("replaced backing err=%v, want incompatible + recovery-required", err)
	}
	if _, err := os.Stat(backing); err != nil {
		t.Fatalf("replacement backing was removed: %v", err)
	}
	if fixture.countCalls("qemu-nbd --disconnect") != 0 {
		t.Fatalf("replaced backing caused disconnect of old inode: %v", fixture.calls)
	}
}

func TestCompactReconstructsLiveAttachmentAndRefusesOfflineCheck(t *testing.T) {
	fixture := newFakeNBDFixture(t, 1)
	storageRoot := filepath.Join(t.TempDir(), "storage")
	backing := createQCOW2Fixture(t, storageRoot, "demo")
	fixture.addMapping(t, filepath.Join(fixture.devRoot, "nbd0"), backing)

	err := fixture.store().Compact(context.Background(), block.Handle{ID: "demo", Path: backing})
	if err == nil || !strings.Contains(err.Error(), "requires detached image") {
		t.Fatalf("compact err=%v", err)
	}
	if fixture.countCalls("qemu-img check") != 0 {
		t.Fatalf("compact ran qemu-img on a live attachment: %v", fixture.calls)
	}
}
