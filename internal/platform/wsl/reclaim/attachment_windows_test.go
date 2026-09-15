//go:build windows && (amd64 || arm64)

package wslreclaim

import (
	"context"
	"encoding/binary"
	"os"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// This check only reads native attachment state. WSL's stopped listing alone
// must not be treated as proof that its virtual disk is detached.
func TestDedicatedWSLVHDDetachedState(t *testing.T) {
	path := os.Getenv("HACO_E2E_RECLAIM_VHD")
	if path == "" {
		t.Skip("requires exact dedicated WSL VHDX")
	}
	pin, err := pinDisk(path)
	if err != nil {
		t.Fatal(err)
	}
	defer pin.Close()
	canonical, err := pin.volumePath()
	if err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(canonical)
	if err != nil {
		t.Fatal(err)
	}
	storage := struct {
		Device uint32
		Vendor windows.GUID
	}{
		3, windows.GUID{Data1: 0xec984aec, Data2: 0xa0f9, Data3: 0x47e9, Data4: [8]byte{0x90, 0x1f, 0x71, 0x41, 0x5a, 0x66, 0x34, 0x5b}},
	}
	params := struct {
		Version, InfoOnly, ReadOnly uint32
		Resiliency                  windows.GUID
	}{Version: 2, InfoOnly: 1, ReadOnly: 1}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	started := time.Now()
	h, attempts, err := waitVirtualDiskOpen(ctx, func() (windows.Handle, error) {
		var h windows.Handle
		code, _, _ := openVirtualDisk.Call(uintptr(unsafe.Pointer(&storage)), uintptr(unsafe.Pointer(name)), 0, 1, uintptr(unsafe.Pointer(&params)), uintptr(unsafe.Pointer(&h)))
		if code != 0 {
			return 0, syscall.Errno(code)
		}
		return h, nil
	})
	t.Logf("read-only native readiness: attempts=%d elapsed=%s", attempts, time.Since(started))
	if err != nil {
		t.Fatalf("read-only native open: %v", err)
	}
	defer windows.CloseHandle(h)
	loaded, err := virtualInfo(h, 13)
	if err != nil {
		t.Fatal(err)
	}
	state := binary.LittleEndian.Uint32(loaded[8:])
	t.Logf("native attachment state=%d (zero means detached); no mutation attempted", state)
	if state != 0 {
		t.Fatal("dedicated WSL VHDX remains attached")
	}
}
