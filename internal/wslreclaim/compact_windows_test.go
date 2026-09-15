//go:build windows && (amd64 || arm64)

package wslreclaim

import (
	"context"
	"encoding/binary"
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestCompactionRefusesCancellationAndNonVirtualFile(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var p *pinnedDisk
	result, err := p.compact(ctx)
	if !errors.Is(err, context.Canceled) || result.Attempted {
		t.Fatal(result, err)
	}
	path := filepath.Join(t.TempDir(), "invalid.vhdx")
	if err := os.WriteFile(path, []byte("retained ordinary file"), 0600); err != nil {
		t.Fatal(err)
	}
	p, err = pinDisk(path)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	result, err = p.compact(context.Background())
	if err == nil || result.Attempted {
		t.Fatal("non-virtual file accepted", result, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "retained ordinary file" {
		t.Fatal("invalid target changed", err)
	}
}

func TestDedicatedWSLVHDCompaction(t *testing.T) {
	if os.Getenv("HACO_E2E_RECLAIM_COMPACT") != "1" {
		t.Skip("requires separate exact offline managed-WSL disk authorization")
	}
	path := os.Getenv("HACO_E2E_RECLAIM_VHD")
	p, err := pinDisk(path)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	result, err := p.compact(context.Background())
	if err != nil {
		t.Fatalf("compaction failed; observation=%+v error=%v", result, err)
	}
	if !result.Attempted || !result.Completed {
		t.Fatal("compaction completion unproven", result)
	}
	t.Logf("PASS native compact; observation=%+v; filesystem bytes/resume checked separately", result)
}

// Create an isolated real dynamic VHDX without a parent, source or attachment.
func nativeEmptyVHD(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "isolated.vhdx")
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	var storage struct {
		Device uint32
		Vendor windows.GUID
	}
	storage.Device = 3
	storage.Vendor = windows.GUID{Data1: 0xec984aec, Data2: 0xa0f9, Data3: 0x47e9, Data4: [8]byte{0x90, 0x1f, 0x71, 0x41, 0x5a, 0x66, 0x34, 0x5b}}
	// CREATE_VIRTUAL_DISK_PARAMETERS V2, union aligned to eight bytes.
	var params [128]byte
	binary.LittleEndian.PutUint32(params[:], 2)
	binary.LittleEndian.PutUint64(params[24:], 256<<20)
	binary.LittleEndian.PutUint32(params[36:], 512)
	binary.LittleEndian.PutUint32(params[40:], 4096)
	var h windows.Handle
	code, _, _ := virtualDiskDLL.NewProc("CreateVirtualDisk").Call(uintptr(unsafe.Pointer(&storage)), uintptr(unsafe.Pointer(name)), 0, 0, 0, 0, uintptr(unsafe.Pointer(&params[0])), 0, uintptr(unsafe.Pointer(&h)))
	if code != 0 {
		t.Fatalf("create isolated VHDX: %v", syscall.Errno(code))
	}
	if err := windows.CloseHandle(h); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNativeVirtualDiskCompactionPreservesPinnedIdentity(t *testing.T) {
	path := nativeEmptyVHD(t)
	pin, err := pinDisk(path)
	if err != nil {
		t.Fatal(err)
	}
	defer pin.Close()
	compacted, err := pin.compact(context.Background())
	if err != nil || !compacted.Attempted || !compacted.Completed {
		t.Fatalf("owned empty VHD native compaction: %+v %v", compacted, err)
	}
	if compacted.Virtual.Capacity != 256<<20 || compacted.OpenAttempts != 1 {
		t.Fatal("unexpected native disk observation", compacted)
	}
	if err := os.Rename(path, path+".moved"); err == nil {
		t.Fatal("compaction released file rename exclusion")
	}
	if _, err := pin.Allocation(); err != nil {
		t.Fatal(err)
	}
	t.Logf("owned empty VHD compact while pinned: %+v", compacted)
}

// Native attachment lifetime is independent of WSL. Use only a newly created,
// empty disk with no drive letter; never attach a managed or caller-supplied disk.
func TestNativeDetachedWaitReleasesVirtualHandle(t *testing.T) {
	path := nativeEmptyVHD(t)
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	storage := struct {
		Device uint32
		Vendor windows.GUID
	}{3, windows.GUID{Data1: 0xec984aec, Data2: 0xa0f9, Data3: 0x47e9, Data4: [8]byte{0x90, 0x1f, 0x71, 0x41, 0x5a, 0x66, 0x34, 0x5b}}}
	parameters := struct {
		Version, InfoOnly, ReadOnly uint32
		Resiliency                  windows.GUID
	}{Version: 2}
	open := func() (windows.Handle, error) {
		var handle windows.Handle
		code, _, _ := openVirtualDisk.Call(uintptr(unsafe.Pointer(&storage)), uintptr(unsafe.Pointer(name)), 0, 1, uintptr(unsafe.Pointer(&parameters)), uintptr(unsafe.Pointer(&handle)))
		if code != 0 {
			return 0, syscall.Errno(code)
		}
		return handle, nil
	}
	owner, err := open()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if owner != 0 {
			_ = windows.CloseHandle(owner)
		}
	}()
	attachParameters := [2]uint32{1, 0}
	code, _, _ := virtualDiskDLL.NewProc("AttachVirtualDisk").Call(uintptr(owner), 0, 3, 0, uintptr(unsafe.Pointer(&attachParameters[0])), 0)
	if code == uintptr(windows.ERROR_PRIVILEGE_NOT_HELD) {
		t.Skip("native empty-disk attachment requires Windows manage-volume privilege")
	}
	if code != 0 {
		t.Fatalf("attach owned empty disk: %v", syscall.Errno(code))
	}
	// No permanent-lifetime flag: closing all owned virtual handles releases
	// this fixture even when an assertion fails. No detach of other disks.
	probe, err := open()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if probe != 0 {
			_ = windows.CloseHandle(probe)
		}
	}()
	if err := windows.CloseHandle(owner); err != nil {
		t.Fatal(err)
	}
	owner = 0
	first := true
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	handle, identity, attempts, err := waitDetachedVirtualDisk(ctx, func() (windows.Handle, error) {
		if first {
			first = false
			h := probe
			probe = 0
			return h, nil
		}
		return open()
	}, inspectDetachedDynamic, windows.CloseHandle)
	if err != nil {
		t.Fatalf("owned attachment did not release: attempts=%d error=%v", attempts, err)
	}
	defer func() {
		if err := windows.CloseHandle(handle); err != nil {
			t.Error(err)
		}
	}()
	if identity.Capacity != 256<<20 {
		t.Fatal(identity)
	}
	t.Logf("owned native disk detached with file intact: attempts=%d", attempts)
}

func TestDetachedOpenClosesAttachedHandlesBeforeWaiting(t *testing.T) {
	expected := virtualDiskIdentity{Capacity: 42, Identifier: [16]byte{1}}
	opens, closes := 0, 0
	live := false
	h, got, attempts, err := waitDetachedVirtualDisk(context.Background(), func() (windows.Handle, error) {
		if live {
			t.Fatal("reopened while prior virtual handle remained held")
		}
		opens++
		if opens == 1 {
			return 0, windows.ERROR_SHARING_VIOLATION
		}
		live = true
		return windows.Handle(opens), nil
	}, func(h windows.Handle) (virtualDiskIdentity, error) {
		if h == 2 {
			return virtualDiskIdentity{}, errVirtualDiskAttached
		}
		return expected, nil
	}, func(h windows.Handle) error {
		if h != 2 || !live {
			t.Fatal("closed wrong handle", h)
		}
		closes++
		live = false
		return nil
	})
	if err != nil || h != 3 || got != expected || attempts != 3 || closes != 1 || !live {
		t.Fatal(h, got, attempts, closes, err)
	}
	// Success transfers the detached handle to its caller, rather than closing it.
}

func TestDetachedOpenRefusesUnknownFailuresAndClosesFailedObservations(t *testing.T) {
	for _, mode := range []string{"open", "inspect", "close"} {
		t.Run(mode, func(t *testing.T) {
			opens, closes := 0, 0
			failure := windows.ERROR_ACCESS_DENIED
			h, _, attempts, err := waitDetachedVirtualDisk(context.Background(), func() (windows.Handle, error) {
				opens++
				if mode == "open" {
					return 0, failure
				}
				return 42, nil
			}, func(windows.Handle) (virtualDiskIdentity, error) {
				if mode == "close" {
					return virtualDiskIdentity{}, errVirtualDiskAttached
				}
				return virtualDiskIdentity{}, failure
			}, func(windows.Handle) error {
				closes++
				if mode == "close" {
					return failure
				}
				return nil
			})
			wantClose := 1
			if mode == "open" {
				wantClose = 0
			}
			if h != 0 || attempts != 1 || opens != 1 || closes != wantClose || !errors.Is(err, failure) {
				t.Fatal(h, attempts, opens, closes, err)
			}
		})
	}
}

func TestDetachedOpenCancellationClosesOwnedHandleAndDoesNotReopen(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	opens, closes := 0, 0
	h, _, attempts, err := waitDetachedVirtualDisk(ctx, func() (windows.Handle, error) { opens++; return 42, nil }, func(windows.Handle) (virtualDiskIdentity, error) {
		cancel()
		return virtualDiskIdentity{}, errVirtualDiskAttached
	}, func(windows.Handle) error { closes++; return nil })
	if h != 0 || attempts != 1 || opens != 1 || closes != 1 || !errors.Is(err, context.Canceled) || !errors.Is(err, errVirtualDiskAttached) {
		t.Fatal(h, attempts, opens, closes, err)
	}
	_, _, attempts, err = waitDetachedVirtualDisk(ctx, func() (windows.Handle, error) { t.Fatal("open after cancellation"); return 0, nil }, nil, nil)
	if attempts != 0 || !errors.Is(err, context.Canceled) {
		t.Fatal(attempts, err)
	}
}
