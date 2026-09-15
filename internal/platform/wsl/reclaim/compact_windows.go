//go:build windows && (amd64 || arm64)

package wslreclaim

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var errVirtualDiskAttached = errors.New("VHDX is still attached")

var virtualDiskDLL = windows.NewLazySystemDLL("virtdisk.dll")
var openVirtualDisk = virtualDiskDLL.NewProc("OpenVirtualDisk")
var getVirtualDiskInformation = virtualDiskDLL.NewProc("GetVirtualDiskInformation")
var compactVirtualDisk = virtualDiskDLL.NewProc("CompactVirtualDisk")

type virtualDiskIdentity struct {
	Capacity   uint64
	Identifier [16]byte
}
type compactObservation struct {
	Before, After        diskAllocation
	Virtual              virtualDiskIdentity
	Attempted, Completed bool
	OpenAttempts         int
}

// Resolve the held file to a volume GUID path rather than reusing a drive letter
// that could be remapped. The returned path is never accepted from an RPC caller.
func (p *pinnedDisk) volumePath() (string, error) {
	if _, err := p.Allocation(); err != nil {
		return "", err
	}
	buffer := make([]uint16, 1024)
	n, err := windows.GetFinalPathNameByHandle(p.handles[len(p.handles)-1], &buffer[0], uint32(len(buffer)), 1)
	if err != nil {
		return "", err
	}
	if n == 0 || n >= uint32(len(buffer)) {
		return "", errors.New("invalid final VHDX path length")
	}
	name := windows.UTF16ToString(buffer[:n])
	end := strings.Index(name, `}\`)
	if !strings.HasPrefix(name, `\\?\Volume{`) || end < 11 {
		return "", errors.New("VHDX final path is not volume-relative")
	}
	if _, err := windows.GUIDFromString(name[10 : end+1]); err != nil {
		return "", err
	}
	return name, nil
}

// The SDK GET_VIRTUAL_DISK_INFO union is aligned to 8 bytes on amd64/arm64.
func virtualInfo(h windows.Handle, version uint32) ([40]byte, error) {
	var raw [40]byte
	binary.LittleEndian.PutUint32(raw[:], version)
	size := uint32(len(raw))
	var used uint32
	code, _, _ := getVirtualDiskInformation.Call(uintptr(h), uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&raw[0])), uintptr(unsafe.Pointer(&used)))
	if code != 0 {
		return raw, syscall.Errno(code)
	}
	if size > uint32(len(raw)) || used > uint32(len(raw)) {
		return raw, errors.New("oversized virtual disk information")
	}
	return raw, nil
}

func inspectDetachedDynamic(h windows.Handle) (virtualDiskIdentity, error) {
	var result virtualDiskIdentity
	kind, err := virtualInfo(h, 7)
	if err != nil {
		return result, err
	}
	if binary.LittleEndian.Uint32(kind[8:]) != 3 {
		return result, errors.New("only dynamic non-differencing VHDX is supported")
	}
	loaded, err := virtualInfo(h, 13)
	if err != nil {
		return result, err
	}
	if binary.LittleEndian.Uint32(loaded[8:]) != 0 {
		return result, errVirtualDiskAttached
	}
	format, err := virtualInfo(h, 6)
	if err != nil {
		return result, err
	}
	if binary.LittleEndian.Uint32(format[8:]) != 3 {
		return result, errors.New("not a VHDX virtual disk")
	}
	size, err := virtualInfo(h, 1)
	if err != nil {
		return result, err
	}
	result.Capacity = binary.LittleEndian.Uint64(size[8:])
	id, err := virtualInfo(h, 2)
	if err != nil {
		return result, err
	}
	copy(result.Identifier[:], id[8:24])
	if result.Capacity == 0 || result.Identifier == ([16]byte{}) {
		return result, errors.New("empty virtual disk identity")
	}
	return result, nil
}

// compact is an internal native operation, not WSL authorization/orchestration.
// Caller must have selected the exact managed distribution and stopped it. No
// parent chain is opened, no disk is attached, resized or replaced. Keep file and
// parent handles throughout; sharing failures are returned, never bypassed.
func (p *pinnedDisk) compact(ctx context.Context) (result compactObservation, err error) {
	if err := ctx.Err(); err != nil {
		return result, err
	}
	result.Before, err = p.Allocation()
	if err != nil {
		return result, err
	}
	path, err := p.volumePath()
	if err != nil {
		return result, err
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return result, err
	}
	var storage struct {
		Device uint32
		Vendor windows.GUID
	}
	storage.Device = 3 // VHDX, not extension-driven arbitrary disk format.
	storage.Vendor = windows.GUID{Data1: 0xec984aec, Data2: 0xa0f9, Data3: 0x47e9, Data4: [8]byte{0x90, 0x1f, 0x71, 0x41, 0x5a, 0x66, 0x34, 0x5b}}
	parameters := struct {
		Version, InfoOnly, ReadOnly uint32
		Resiliency                  windows.GUID
	}{Version: 2}
	// WSL 2.7.13 can retain the backing disk until the shared VM idle timer
	// expires (native acceptance observed about 58s). Allow that natural release;
	// never stop another distribution. Only wait before mutation.
	openCtx, cancelOpen := context.WithTimeout(ctx, 90*time.Second)
	defer cancelOpen()
	h, attempts, openErr := waitVirtualDiskOpen(openCtx, func() (windows.Handle, error) {
		var handle windows.Handle
		// V2 uses ACCESS_NONE. NO_PARENTS prevents following a differencing chain.
		code, _, _ := openVirtualDisk.Call(uintptr(unsafe.Pointer(&storage)), uintptr(unsafe.Pointer(name)), 0, 1, uintptr(unsafe.Pointer(&parameters)), uintptr(unsafe.Pointer(&handle)))
		if code != 0 {
			return 0, syscall.Errno(code)
		}
		return handle, nil
	})
	result.OpenAttempts = attempts
	if openErr != nil {
		return result, fmt.Errorf("open virtual disk: %w", openErr)
	}
	defer func() { err = errors.Join(err, windows.CloseHandle(h)) }()
	result.Virtual, err = waitVirtualDiskDetached(openCtx, func() (virtualDiskIdentity, error) {
		return inspectDetachedDynamic(h)
	})
	if err != nil {
		return result, err
	}
	if _, err = p.Allocation(); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	parametersCompact := [2]uint32{1, 0}
	result.Attempted = true
	code, _, _ := compactVirtualDisk.Call(uintptr(h), 0, uintptr(unsafe.Pointer(&parametersCompact[0])), 0)
	if code == 0 {
		result.Completed = true
	} else {
		err = fmt.Errorf("compact virtual disk: %w", syscall.Errno(code))
	}
	var measureErr error
	result.After, measureErr = p.Allocation()
	after, identityErr := inspectDetachedDynamic(h)
	if identityErr == nil && after != result.Virtual {
		identityErr = errors.New("virtual capacity or disk identity changed")
	}
	// A synchronous kernel operation can finish after cancellation. Preserve its
	// actual observations rather than claiming the operation never happened.
	return result, errors.Join(err, measureErr, identityErr, ctx.Err())
}

// No retry after a handle has been returned or for any error other than the
// sharing violation observed at native open. The held file/parents are unchanged.
func waitVirtualDiskOpen(ctx context.Context, open func() (windows.Handle, error)) (windows.Handle, int, error) {
	attempts := 0
	for {
		if err := ctx.Err(); err != nil {
			return 0, attempts, err
		}
		attempts++
		handle, err := open()
		if err == nil {
			return handle, attempts, nil
		}
		if !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			return 0, attempts, err
		}
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return 0, attempts, errors.Join(ctx.Err(), err)
		case <-timer.C:
		}
	}
}

// Native open may succeed before WSL releases the attachment. Keep the same
// handle and all ownership pins; only observe until detached within the shared
// open/detach deadline. No compaction or stop operation is retried.
func waitVirtualDiskDetached(ctx context.Context, inspect func() (virtualDiskIdentity, error)) (virtualDiskIdentity, error) {
	for {
		if err := ctx.Err(); err != nil {
			return virtualDiskIdentity{}, err
		}
		identity, err := inspect()
		if err == nil || !errors.Is(err, errVirtualDiskAttached) {
			return identity, err
		}
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return virtualDiskIdentity{}, errors.Join(ctx.Err(), err)
		case <-timer.C:
		}
	}
}
