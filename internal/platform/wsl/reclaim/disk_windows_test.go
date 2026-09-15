//go:build windows

package wslreclaim

import (
	"encoding/binary"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"testing"
	"unsafe"
)

func TestDiskPathRefusesAmbiguousTargets(t *testing.T) {
	for _, path := range []string{`relative.vhdx`, `\\server\share\ext4.vhdx`, `\\?\C:\ext4.vhdx`, `C:\x\..\ext4.vhdx`, `C:\x.\ext4.vhdx`, `C:\x \ext4.vhdx`, `C:\x:stream\ext4.vhdx`, `C:\x\ext4.vhd`, `C:/x/ext4.vhdx`} {
		if _, err := diskPathParts(path); err == nil {
			t.Errorf("accepted %q", path)
		}
	}
	if _, err := diskPathParts(`C:\Users\owner\disk\ext4.vhdx`); err != nil {
		t.Fatal(err)
	}
}

func TestPinnedDiskMeasuresSparseAllocationAndPreventsReplacement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.vhdx")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	var returned uint32
	// FSCTL_SET_SPARSE; this fixture is a new ordinary test file, never a WSL disk.
	if err := windows.DeviceIoControl(windows.Handle(file.Fd()), 0x900c4, nil, 0, nil, 0, &returned, nil); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Truncate(32 << 20); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if _, err := file.WriteAt([]byte("retained"), 0); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	pinned, err := pinDisk(path)
	if err != nil {
		t.Fatal(err)
	}
	defer pinned.Close()
	size, err := pinned.Allocation()
	if err != nil {
		t.Fatal(err)
	}
	if size.LogicalBytes != 32<<20 || size.AllocatedBytes == 0 || size.AllocatedBytes >= size.LogicalBytes {
		t.Fatalf("sparse allocation not distinguished: %+v", size)
	}
	if err := os.Rename(path, path+".moved"); err == nil {
		t.Fatal("pinned disk was replaced")
	}
	if err := os.Rename(dir, dir+"-moved"); err == nil {
		t.Fatal("pinned parent was replaced")
	}
	if err := pinned.Close(); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 8)
	read, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	if _, err := read.Read(data); err != nil || string(data) != "retained" {
		t.Fatal("retained bytes changed", err)
	}
	t.Logf("logical=%d allocated=%d; rename refusal and retained bytes passed", size.LogicalBytes, size.AllocatedBytes)
}

func TestPinnedDiskRefusesHardlinksAndClosedHandles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.vhdx")
	if err := os.WriteFile(path, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(path, filepath.Join(dir, "other.vhdx")); err != nil {
		t.Fatal(err)
	}
	if p, err := pinDisk(path); err == nil {
		p.Close()
		t.Fatal("hardlink accepted")
	}
	var p *pinnedDisk
	if _, err := p.Allocation(); err == nil {
		t.Fatal("nil handle accepted")
	}
}

func TestWindowsStandardInfoLayout(t *testing.T) {
	var info struct {
		Allocation, End    int64
		Links              uint32
		Deleted, Directory byte
		Padding            [2]byte
	}
	if unsafe.Sizeof(info) != 24 {
		t.Fatal("FILE_STANDARD_INFO layout changed")
	}
}

func TestPinnedDiskRefusesReparseAncestor(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	if err := os.Mkdir(real, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "test.vhdx"), []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "linked")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("native symlink fixture unavailable: %v", err)
	}
	if p, err := pinDisk(filepath.Join(link, "test.vhdx")); err == nil {
		p.Close()
		t.Fatal("reparse ancestor accepted")
	}
}

func TestPinnedDiskRefusesJunctionAncestor(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	link := filepath.Join(dir, "junction")
	if err := os.Mkdir(real, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(link, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "test.vhdx"), []byte("retained"), 0600); err != nil {
		t.Fatal(err)
	}
	// A mount-point reparse fixture requires no symlink privilege. Construct it
	// through the native API, avoiding cmd.exe path interpolation.
	substitute, _ := windows.UTF16FromString(`\??\` + real)
	printable, _ := windows.UTF16FromString(real)
	data := make([]byte, 16+2*(len(substitute)+len(printable)))
	binary.LittleEndian.PutUint32(data, 0xa0000003)
	binary.LittleEndian.PutUint16(data[4:], uint16(len(data)-8))
	binary.LittleEndian.PutUint16(data[10:], uint16(2*(len(substitute)-1)))
	binary.LittleEndian.PutUint16(data[12:], uint16(2*len(substitute)))
	binary.LittleEndian.PutUint16(data[14:], uint16(2*(len(printable)-1)))
	for i, v := range append(substitute, printable...) {
		binary.LittleEndian.PutUint16(data[16+2*i:], v)
	}
	name, _ := windows.UTF16PtrFromString(link)
	h, err := windows.CreateFile(name, windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		t.Fatal(err)
	}
	var returned uint32
	setErr := windows.DeviceIoControl(h, 0x900a4, &data[0], uint32(len(data)), nil, 0, &returned, nil)
	closeErr := windows.CloseHandle(h)
	if setErr != nil || closeErr != nil {
		t.Fatal("junction fixture", setErr, closeErr)
	}
	if p, err := pinDisk(filepath.Join(link, "test.vhdx")); err == nil {
		p.Close()
		t.Fatal("junction ancestor accepted")
	}
	retained, err := os.ReadFile(filepath.Join(real, "test.vhdx"))
	if err != nil || string(retained) != "retained" {
		t.Fatal("junction target changed", err)
	}
}

func TestDedicatedWSLVHDAllocation(t *testing.T) {
	path := os.Getenv("HACO_E2E_RECLAIM_VHD")
	if path == "" {
		t.Skip("set exact dedicated WSL VHD path for read-only observation")
	}
	pinned, err := pinDisk(path)
	if err != nil {
		t.Fatal(err)
	}
	defer pinned.Close()
	size, err := pinned.Allocation()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("actual VHDX file logical=%d allocated=%d; identity=%+v; no stop/compact performed", size.LogicalBytes, size.AllocatedBytes, pinned.identity)
}
