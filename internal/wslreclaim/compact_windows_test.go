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

func TestVirtualDiskOpenWaitIsBoundedAndOnlyBeforeMutation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	_, attempts, err := waitVirtualDiskOpen(ctx, func() (windows.Handle, error) { calls++; return 42, nil })
	if !errors.Is(err, context.Canceled) || calls != 0 || attempts != 0 {
		t.Fatal(calls, attempts, err)
	}
	_, attempts, err = waitVirtualDiskOpen(context.Background(), func() (windows.Handle, error) { return 0, windows.ERROR_ACCESS_DENIED })
	if !errors.Is(err, windows.ERROR_ACCESS_DENIED) || attempts != 1 {
		t.Fatal(attempts, err)
	}
	calls = 0
	handle, attempts, err := waitVirtualDiskOpen(context.Background(), func() (windows.Handle, error) {
		calls++
		if calls == 1 {
			return 0, windows.ERROR_SHARING_VIOLATION
		}
		return 42, nil
	})
	if err != nil || handle != 42 || attempts != 2 {
		t.Fatal(handle, attempts, err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	_, attempts, err = waitVirtualDiskOpen(ctx, func() (windows.Handle, error) { cancel(); return 0, windows.ERROR_SHARING_VIOLATION })
	if !errors.Is(err, context.Canceled) || !errors.Is(err, windows.ERROR_SHARING_VIOLATION) || attempts != 1 {
		t.Fatal(attempts, err)
	}
}

func TestVirtualDiskDetachWaitObservesWithoutReopeningOrMutating(t *testing.T) {
	expected := virtualDiskIdentity{Capacity: 42, Identifier: [16]byte{1}}
	calls := 0
	got, err := waitVirtualDiskDetached(context.Background(), func() (virtualDiskIdentity, error) {
		calls++
		if calls == 1 {
			return virtualDiskIdentity{}, errVirtualDiskAttached
		}
		return expected, nil
	})
	if err != nil || got != expected || calls != 2 {
		t.Fatal(got, calls, err)
	}
	for _, failure := range []error{windows.ERROR_ACCESS_DENIED, errors.New("invalid disk format")} {
		calls = 0
		_, err = waitVirtualDiskDetached(context.Background(), func() (virtualDiskIdentity, error) {
			calls++
			return virtualDiskIdentity{}, failure
		})
		if !errors.Is(err, failure) || calls != 1 {
			t.Fatal(calls, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	calls = 0
	_, err = waitVirtualDiskDetached(ctx, func() (virtualDiskIdentity, error) {
		calls++
		cancel()
		return virtualDiskIdentity{}, errVirtualDiskAttached
	})
	if !errors.Is(err, context.Canceled) || !errors.Is(err, errVirtualDiskAttached) || calls != 1 {
		t.Fatal(calls, err)
	}
	_, err = waitVirtualDiskDetached(ctx, func() (virtualDiskIdentity, error) {
		t.Fatal("inspection after cancellation")
		return expected, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
