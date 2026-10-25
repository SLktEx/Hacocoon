//go:build windows && (amd64 || arm64)

package reclaimnative

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

type installedHelper struct {
	executable, registration string
	pins                     []windows.Handle
}

// Resolve only the current user's OS-known directory, not LOCALAPPDATA/PATH.
// This is the installer's fixed per-registration helper, not the public binary
// or a caller-supplied executable. Keep its owner and ancestors pinned across
// both calls, without exposing a general command-running facility.
func openInstalledHelper(registration string) (_ helper, err error) {
	if !validGUID(registration) {
		return nil, errors.New("invalid registration")
	}
	root, err := windows.KnownFolderPath(windows.FOLDERID_LocalAppData, 0)
	if err != nil {
		return nil, err
	}
	compact := strings.ReplaceAll(strings.ToLower(registration[1:37]), "-", "")
	directory := filepath.Join(root, "Hacocoon", "reclamation", compact)
	h := &installedHelper{executable: filepath.Join(directory, "haco-wsl.exe"), registration: registration}
	defer func() {
		if err != nil {
			err = errors.Join(err, h.close())
		}
	}()
	parts, err := localParts(h.executable)
	if err != nil {
		return nil, err
	}
	current := h.executable[:3]
	for i := 0; i < len(parts); i++ {
		if i > 0 {
			current = filepath.Join(current, parts[i-1])
		}
		handle, e := pinComponent(current, true)
		if e != nil {
			return nil, e
		}
		h.pins = append(h.pins, handle)
	}
	owner, err := pinComponent(filepath.Join(directory, "installation.json"), false)
	if err != nil {
		return nil, err
	}
	h.pins = append(h.pins, owner)
	var info windows.ByHandleFileInformation
	if err = windows.GetFileInformationByHandle(owner, &info); err != nil {
		return nil, err
	}
	if info.FileSizeHigh != 0 || info.FileSizeLow == 0 || info.FileSizeLow > receiptLimit {
		return nil, errors.New("invalid helper ownership size")
	}
	raw := make([]byte, info.FileSizeLow)
	var n uint32
	if err = windows.ReadFile(owner, raw, &n, nil); err != nil {
		return nil, err
	}
	if n != info.FileSizeLow {
		return nil, errors.New("incomplete helper ownership")
	}
	if err = validateOwner(raw, registration); err != nil {
		return nil, err
	}
	binary, err := pinComponent(h.executable, false)
	if err != nil {
		return nil, err
	}
	h.pins = append(h.pins, binary)
	return h, nil
}

// Match the existing reclaim pin's local-drive/no-normalization contract.
func localParts(path string) ([]string, error) {
	if len(path) < 9 || len(path) > 240 || path[1:3] != `:\` || ((path[0] < 'A' || path[0] > 'Z') && (path[0] < 'a' || path[0] > 'z')) || filepath.Clean(path) != path {
		return nil, errors.New("unsupported installed helper path")
	}
	parts := strings.Split(path[3:], `\`)
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.TrimRight(part, " .") != part {
			return nil, errors.New("ambiguous installed helper path")
		}
		for _, c := range part {
			if c < 32 || strings.ContainsRune(`/:*?"<>|`, c) {
				return nil, errors.New("invalid installed helper path")
			}
		}
	}
	return parts, nil
}

func pinComponent(path string, directory bool) (windows.Handle, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	h, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return 0, err
	}
	var info windows.ByHandleFileInformation
	err = windows.GetFileInformationByHandle(h, &info)
	if err == nil && (info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || (info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0) != directory || !directory && info.NumberOfLinks != 1) {
		err = errors.New("redirected or nonordinary installed helper component")
	}
	if err != nil {
		return 0, errors.Join(err, windows.CloseHandle(h))
	}
	return h, nil
}

func (h *installedHelper) close() error {
	var errs []error
	for i := len(h.pins) - 1; i >= 0; i-- {
		errs = append(errs, windows.CloseHandle(h.pins[i]))
	}
	h.pins = nil
	return errors.Join(errs...)
}
func (h *installedHelper) prepare(ctx context.Context, registration string) ([]byte, error) {
	if !validGUID(registration) || !strings.EqualFold(registration, h.registration) {
		return nil, errors.New("invalid registration")
	}
	return h.call(ctx, []string{"_prepare-native", registration})
}
func (h *installedHelper) launch(ctx context.Context, registration, operation string) ([]byte, error) {
	if !validGUID(registration) || !validGUID(operation) || !strings.EqualFold(registration, h.registration) {
		return nil, errors.New("invalid exact handoff")
	}
	return h.call(ctx, []string{"_launch", registration, operation})
}
func (h *installedHelper) call(ctx context.Context, args []string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(h.pins) == 0 {
		return nil, errors.New("installed helper closed")
	}
	process, err := h.start(ctx, args)
	if err != nil {
		return nil, err
	}
	return observe(ctx, process)
}
