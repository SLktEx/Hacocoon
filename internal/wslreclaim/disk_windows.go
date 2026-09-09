//go:build windows

// Package wslreclaim contains the Windows side of managed-WSL reclamation.
// Native file identity is not authority to stop a distribution or compact a disk.
package wslreclaim

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

type diskIdentity struct{ Volume, High, Low uint32 }
type diskAllocation struct{ LogicalBytes, AllocatedBytes uint64 }
type pinnedDisk struct {
	path     string
	handles  []windows.Handle
	identity diskIdentity
}

// A narrow local-drive path excludes UNC, device paths, alternate streams and
// Win32 normalization ambiguities before any file operation.
func diskPathParts(path string) ([]string, error) {
	if len(path) < 9 || len(path) > 240 || path[1:3] != `:\` ||
		!((path[0] >= 'A' && path[0] <= 'Z') || (path[0] >= 'a' && path[0] <= 'z')) ||
		filepath.Clean(path) != path || !strings.EqualFold(filepath.Ext(path), ".vhdx") {
		return nil, errors.New("unsupported VHDX path")
	}
	parts := strings.Split(path[3:], `\`)
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.TrimRight(part, " .") != part {
			return nil, errors.New("ambiguous VHDX path")
		}
		for _, c := range part {
			if c < 32 || strings.ContainsRune(`/:*?"<>|`, c) {
				return nil, errors.New("invalid VHDX path component")
			}
		}
	}
	return parts, nil
}

// pinDisk is measurement-only. The future caller must independently bind the
// exact WSL registration and installed product before authorizing any mutation.
// Hold each directory against write/delete sharing; do not follow reparse points.
// GENERIC_READ is intentional: attribute-only opens do not establish the
// sharing exclusion needed to prevent renames (covered by native regression).
func pinDisk(path string) (_ *pinnedDisk, err error) {
	parts, err := diskPathParts(path)
	if err != nil {
		return nil, err
	}
	p := &pinnedDisk{path: path}
	defer func() {
		if err != nil {
			_ = p.Close()
		}
	}()
	current := path[:3]
	for i := 0; i <= len(parts); i++ {
		directory := i < len(parts)
		if i > 0 {
			current = filepath.Join(current, parts[i-1])
		}
		ptr, e := windows.UTF16PtrFromString(current)
		if e != nil {
			return nil, e
		}
		share := uint32(windows.FILE_SHARE_READ)
		if !directory {
			share |= windows.FILE_SHARE_WRITE
		}
		h, e := windows.CreateFile(ptr, windows.GENERIC_READ, share, nil, windows.OPEN_EXISTING,
			windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
		if e != nil {
			return nil, fmt.Errorf("pin VHDX component: %w", e)
		}
		p.handles = append(p.handles, h)
		var info windows.ByHandleFileInformation
		if e := windows.GetFileInformationByHandle(h, &info); e != nil {
			return nil, e
		}
		if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 ||
			(info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0) != directory {
			return nil, errors.New("VHDX path contains a reparse point or wrong object type")
		}
		if !directory {
			if info.NumberOfLinks != 1 {
				return nil, errors.New("VHDX must have exactly one link")
			}
			p.identity = diskIdentity{info.VolumeSerialNumber, info.FileIndexHigh, info.FileIndexLow}
		}
	}
	if _, err := p.Allocation(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *pinnedDisk) Close() error {
	var errs []error
	for i := len(p.handles) - 1; i >= 0; i-- {
		errs = append(errs, windows.CloseHandle(p.handles[i]))
	}
	p.handles = nil
	return errors.Join(errs...)
}

// FILE_STANDARD_INFO is the native handle-based allocation observation, not
// os.Stat().Size(). Both fields are retained; zero allocation is valid.
// https://learn.microsoft.com/en-us/windows/win32/api/winbase/ns-winbase-file_standard_info
func (p *pinnedDisk) Allocation() (diskAllocation, error) {
	if p == nil || len(p.handles) == 0 {
		return diskAllocation{}, errors.New("closed VHDX observation")
	}
	h := p.handles[len(p.handles)-1]
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		return diskAllocation{}, err
	}
	if info.NumberOfLinks != 1 || info.FileAttributes&(windows.FILE_ATTRIBUTE_DIRECTORY|windows.FILE_ATTRIBUTE_REPARSE_POINT) != 0 ||
		(diskIdentity{info.VolumeSerialNumber, info.FileIndexHigh, info.FileIndexLow}) != p.identity {
		return diskAllocation{}, errors.New("VHDX identity changed")
	}
	var standard struct {
		AllocationSize, EndOfFile int64
		NumberOfLinks             uint32
		DeletePending, Directory  byte
		Padding                   [2]byte
	}
	if err := windows.GetFileInformationByHandleEx(h, 1, (*byte)(unsafe.Pointer(&standard)), uint32(unsafe.Sizeof(standard))); err != nil {
		return diskAllocation{}, err
	}
	if standard.AllocationSize < 0 || standard.EndOfFile < 0 || standard.NumberOfLinks != 1 || standard.DeletePending != 0 || standard.Directory != 0 {
		return diskAllocation{}, errors.New("invalid VHDX allocation observation")
	}
	return diskAllocation{uint64(standard.EndOfFile), uint64(standard.AllocationSize)}, nil
}
