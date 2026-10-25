//go:build windows && (amd64 || arm64)

package reclaimnative

import (
	"context"
	"errors"
	"io"
	"os"
	"unsafe"

	"github.com/SLktEx/Hacocoon/internal/cli/ui"
	"golang.org/x/sys/windows"
)

// INPUT_RECORD has a WORD tag, two-byte padding, and a 16-byte event union.
// KEY_EVENT_RECORD occupies that whole union on both supported architectures.
type consoleInputRecord struct {
	eventType, padding                      uint16
	keyDown                                 int32
	repeat, virtualKey, scanCode, character uint16
	controlState                            uint32
}

func confirmConsole(ctx context.Context, in io.Reader, out io.Writer, language cliui.Language) (bool, error) {
	file, ok := in.(*os.File)
	if !ok {
		return false, errors.New("interactive confirmation requires a Windows console; use --yes for redirected input")
	}
	handle := windows.Handle(file.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		return false, errors.New("interactive confirmation requires a Windows console; use --yes for redirected input")
	}
	// ReadConsoleInputExW is a documented dynamically-linked API. NOWAIT removes
	// available events and returns immediately even if another reader consumed
	// the buffer. No Peek/Read race or cooked ReadFile awaiting newline remains.
	// https://learn.microsoft.com/en-us/windows/console/readconsoleinputex
	read := windows.NewLazySystemDLL("kernel32.dll").NewProc("ReadConsoleInputExW")
	if err := read.Find(); err != nil {
		return false, err
	}
	return confirmEvents(ctx, out, language, func() (confirmationKey, bool, error) {
		var event consoleInputRecord
		var count uint32
		const consoleReadNowait = 0x0002
		ok, _, err := read.Call(uintptr(handle), uintptr(unsafe.Pointer(&event)), 1, uintptr(unsafe.Pointer(&count)), consoleReadNowait)
		if ok == 0 {
			return confirmationKey{}, false, err
		}
		if count > 1 {
			return confirmationKey{}, false, errors.New("invalid console event count")
		}
		if count == 0 {
			return confirmationKey{}, false, nil
		}
		if event.eventType != 1 {
			return confirmationKey{}, true, nil
		}
		return confirmationKey{character: event.character, repeat: event.repeat, down: event.keyDown != 0}, true, nil
	})
}
