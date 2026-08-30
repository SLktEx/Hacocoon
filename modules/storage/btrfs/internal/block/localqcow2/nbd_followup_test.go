package localqcow2

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestNBDAllocatorLockIsHostGlobalAcrossStorageRoots(t *testing.T) {
	first := New(&recordingRunner{})
	second := New(&recordingRunner{})
	if got := first.nbdAllocatorLockPath(); got != defaultNBDAllocatorLockPath {
		t.Fatalf("default lock path=%q want=%q", got, defaultNBDAllocatorLockPath)
	}

	lockPath := filepath.Join(t.TempDir(), "hacocoon-nbd.lock")
	first.nbdLockPath = lockPath
	second.nbdLockPath = lockPath
	firstBacking := filepath.Join(t.TempDir(), "storage-a", "images", "a.qcow2")
	secondBacking := filepath.Join(t.TempDir(), "storage-b", "images", "b.qcow2")

	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondEntered := make(chan struct{})
	errCh := make(chan error, 2)

	go func() {
		errCh <- first.withNBDAllocatorLock(firstBacking, func() error {
			close(firstEntered)
			<-releaseFirst
			return nil
		})
	}()
	select {
	case <-firstEntered:
	case err := <-errCh:
		t.Fatalf("first allocator failed before entering critical section: %v", err)
	case <-time.After(time.Second):
		t.Fatal("first allocator did not enter critical section")
	}

	go func() {
		errCh <- second.withNBDAllocatorLock(secondBacking, func() error {
			close(secondEntered)
			return nil
		})
	}()

	select {
	case <-secondEntered:
		t.Fatal("second storage root entered the host-global NBD critical section concurrently")
	case <-time.After(50 * time.Millisecond):
	}
	close(releaseFirst)
	select {
	case <-secondEntered:
	case err := <-errCh:
		t.Fatalf("allocator failed while handing off host-global lock: %v", err)
	case <-time.After(time.Second):
		t.Fatal("second storage root never acquired host-global NBD allocator lock")
	}
	for i := 0; i < 2; i++ {
		select {
		case err := <-errCh:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("allocator goroutine did not finish")
		}
	}
}

func TestNBDAllocatorLockRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	victim := filepath.Join(root, "victim")
	if err := os.WriteFile(victim, []byte("sentinel\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(root, "hacocoon-nbd.lock")
	if err := os.Symlink(victim, lockPath); err != nil {
		t.Fatal(err)
	}
	store := New(&recordingRunner{})
	store.nbdLockPath = lockPath
	if err := store.withNBDAllocatorLock("/ignored", func() error { return nil }); err == nil {
		t.Fatal("host-global NBD allocator accepted symlink lock")
	}
	got, err := os.ReadFile(victim)
	if err != nil || string(got) != "sentinel\n" {
		t.Fatalf("lock symlink victim changed: data=%q err=%v", got, err)
	}
}

func TestWaitForNBDMatchToleratesTransientProcObservation(t *testing.T) {
	fixture := newFakeNBDFixture(t, 1)
	storageRoot := filepath.Join(t.TempDir(), "storage")
	backing := createQCOW2Fixture(t, storageRoot, "demo")
	store := fixture.store()
	device := filepath.Join(fixture.devRoot, "nbd0")
	pid := 9001
	if err := os.WriteFile(filepath.Join(fixture.sysBlockRoot, "nbd0", "pid"), []byte(fmt.Sprintf("%d\n", pid)), 0o600); err != nil {
		t.Fatal(err)
	}

	ready := make(chan error, 1)
	go func() {
		time.Sleep(2 * nbdVerifyDelay)
		procDir := filepath.Join(fixture.procRoot, fmt.Sprintf("%d", pid))
		fdDir := filepath.Join(procDir, "fd")
		if err := os.MkdirAll(fdDir, 0o700); err != nil {
			ready <- err
			return
		}
		cmdline := []byte("/usr/bin/qemu-nbd\x00--connect=" + device + "\x00" + backing + "\x00")
		if err := os.WriteFile(filepath.Join(procDir, "cmdline"), cmdline, 0o600); err != nil {
			ready <- err
			return
		}
		ready <- os.Symlink(backing, filepath.Join(fdDir, "3"))
	}()

	if err := store.waitForNBDMatch(context.Background(), device, backing); err != nil {
		t.Fatalf("transient pid-before-proc observation failed: %v", err)
	}
	if err := <-ready; err != nil {
		t.Fatal(err)
	}
}

func TestWaitForNBDMatchPersistentUncertaintyFailsClosed(t *testing.T) {
	fixture := newFakeNBDFixture(t, 1)
	storageRoot := filepath.Join(t.TempDir(), "storage")
	backing := createQCOW2Fixture(t, storageRoot, "demo")
	device := filepath.Join(fixture.devRoot, "nbd0")
	if err := os.WriteFile(filepath.Join(fixture.sysBlockRoot, "nbd0", "pid"), []byte("9002\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := fixture.store().waitForNBDMatch(context.Background(), device, backing)
	if !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("persistent uncertainty err=%v, want recovery-required", err)
	}
	if !strings.Contains(err.Error(), "cannot inspect qemu-nbd command line") {
		t.Fatalf("persistent uncertainty lost final observation reason: %v", err)
	}
}

func TestWaitForNBDMatchDifferentBackingFailsImmediately(t *testing.T) {
	fixture := newFakeNBDFixture(t, 1)
	storageRoot := filepath.Join(t.TempDir(), "storage")
	backing := createQCOW2Fixture(t, storageRoot, "wanted")
	other := createQCOW2Fixture(t, storageRoot, "other")
	device := filepath.Join(fixture.devRoot, "nbd0")
	fixture.addMapping(t, device, other)

	err := fixture.store().waitForNBDMatch(context.Background(), device, backing)
	if !errors.Is(err, core.ErrRecoveryRequired) || !strings.Contains(err.Error(), "different backing") {
		t.Fatalf("different backing err=%v", err)
	}
}
