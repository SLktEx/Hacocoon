//go:build windows && (amd64 || arm64)

package reclaimnative

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"time"
	"unsafe"

	"github.com/SLktEx/Hacocoon/internal/cli/ui"
	"golang.org/x/sys/windows"
)

func nativeRedirectedFixture(ctx context.Context) (err error) {
	input, output, err := os.Pipe()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, input.Close(), output.Close()) }()
	// No bytes are written: a cooked-read implementation would hang until the
	// outer fixture deadline. Production must reject this pipe before prompting.
	var prompt bytes.Buffer
	confirmed, readErr := confirmConsole(ctx, input, &prompt, cliui.English)
	if confirmed || readErr == nil || prompt.Len() != 0 {
		return errors.New("redirected confirmation was not refused before reading")
	}
	return nil
}

func nativeConsoleFixture(ctx context.Context) (err error) {
	stage := "console-abi"
	defer func() {
		if err != nil {
			err = &fixtureStageError{stage: stage, cause: err}
		}
	}()
	// Independent SDK layout expectations, not generated from the Go struct.
	// https://learn.microsoft.com/en-us/windows/console/input-record-str
	// https://learn.microsoft.com/en-us/windows/console/key-event-record-str
	var record consoleInputRecord
	if unsafe.Sizeof(record) != 20 || unsafe.Offsetof(record.keyDown) != 4 || unsafe.Offsetof(record.repeat) != 8 || unsafe.Offsetof(record.character) != 14 || unsafe.Offsetof(record.controlState) != 16 {
		return errors.New("console event ABI mismatch")
	}
	stage = "console-api"
	kernel := windows.NewLazySystemDLL("kernel32.dll")
	getConsole := kernel.NewProc("GetConsoleWindow")
	if err := getConsole.Find(); err != nil {
		return err
	}
	stage = "console-existing"
	if existing, _, _ := getConsole.Call(); existing != 0 {
		return errors.New("fixture unexpectedly inherited a console")
	}
	stage = "console-api"
	allocate, free := kernel.NewProc("AllocConsole"), kernel.NewProc("FreeConsole")
	if err := allocate.Find(); err != nil {
		return err
	}
	if err := free.Find(); err != nil {
		return err
	}
	// CREATE_NO_WINDOW is not DETACHED_PROCESS. Normalize only this isolated
	// scenario driver's attachment before allocating its own fresh console.
	// FreeConsole succeeds even if it was already detached. Never attach to,
	// flush, or change modes on the developer's console. The driver retains a
	// separate exact pipe handle for its bounded result across console changes.
	// https://learn.microsoft.com/en-us/windows/console/freeconsole
	// https://learn.microsoft.com/en-us/windows/console/allocconsole
	stage = "console-detach"
	if ok, _, callErr := free.Call(); ok == 0 {
		return callErr
	}
	stage = "console-allocate"
	if ok, _, callErr := allocate.Call(); ok == 0 {
		return callErr
	}
	defer func() {
		if ok, _, callErr := free.Call(); ok == 0 {
			stage = "console-cleanup"
			err = errors.Join(err, callErr)
		}
	}()
	stage = "console-input"
	name, err := windows.UTF16PtrFromString("CONIN$")
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return err
	}
	input := os.NewFile(uintptr(handle), "synthetic-console-input")
	if input == nil {
		return errors.Join(errors.New("console input wrapper unavailable"), windows.CloseHandle(handle))
	}
	defer func() {
		if closeErr := input.Close(); closeErr != nil {
			stage = "console-cleanup"
			err = errors.Join(err, closeErr)
		}
	}()
	stage = "console-mode"
	var original uint32
	if err := windows.GetConsoleMode(handle, &original); err != nil {
		return err
	}
	defer func() {
		var mode uint32
		if readErr := windows.GetConsoleMode(handle, &mode); readErr != nil {
			stage = "console-cleanup"
			err = errors.Join(err, readErr)
		} else if mode != original {
			stage = "console-cleanup"
			err = errors.Join(err, errors.New("confirmation changed console mode"))
		}
	}()

	// A fresh console has no affirmative input. Non-key notifications may occur;
	// the production NOWAIT reader must still honor cancellation without a reader
	// goroutine, blocking cooked read or console-mode changes.
	stage = "console-cancel"
	canceled, stop := context.WithTimeout(ctx, 75*time.Millisecond)
	confirmed, readErr := confirmConsole(canceled, input, io.Discard, cliui.English)
	stop()
	if confirmed || !errors.Is(readErr, context.DeadlineExceeded) {
		return errors.New("empty console did not honor bounded cancellation")
	}
	stage = "console-write"
	if err := writeFixtureConsoleKeys(handle, []fixtureConsoleKey{{character: 'y', down: true, repeat: 1}, {character: 'e', down: true, repeat: 1}, {character: 's', down: true, repeat: 1}, {character: '\r', down: true, repeat: 1}}); err != nil {
		return err
	}
	stage = "console-confirm"
	if confirmed, readErr = confirmConsole(ctx, input, io.Discard, cliui.English); readErr != nil || !confirmed {
		return errors.New("SDK-layout console confirmation was not accepted")
	}
	stage = "console-write"
	if err := writeFixtureConsoleKeys(handle, []fixtureConsoleKey{{character: 'y', down: false, repeat: 1}, {character: '\r', down: true, repeat: 1}}); err != nil {
		return err
	}
	stage = "console-key-up"
	if confirmed, readErr = confirmConsole(ctx, input, io.Discard, cliui.English); readErr != nil || confirmed {
		return errors.New("key-up event granted confirmation")
	}
	for _, sample := range []struct {
		keys  []fixtureConsoleKey
		yes   bool
		stage string
	}{
		{[]fixtureConsoleKey{{character: 'y', down: true, repeat: 2}, {character: '\r', down: true, repeat: 1}}, false, "console-repeat"},
		{[]fixtureConsoleKey{{character: 'y', down: true, repeat: 1}, {character: 'x', down: true, repeat: 1}, {character: '\b', down: true, repeat: 1}, {character: '\r', down: true, repeat: 1}}, true, "console-backspace"},
	} {
		stage = "console-write"
		if err := writeFixtureConsoleKeys(handle, sample.keys); err != nil {
			return err
		}
		stage = sample.stage
		if confirmed, readErr = confirmConsole(ctx, input, io.Discard, cliui.English); readErr != nil || confirmed != sample.yes {
			return errors.New("native console repeat or backspace result differs")
		}
	}
	return nil
}

type fixtureConsoleKey struct {
	character uint16
	down      bool
	repeat    uint16
}

// Construct SDK ABI bytes independently of consoleInputRecord so the fixture
// does not merely mirror an erroneous production struct. Writes go exclusively
// to the fresh private console owned by the synthetic scenario process.
// https://learn.microsoft.com/en-us/windows/console/writeconsoleinput
func writeFixtureConsoleKeys(input windows.Handle, keys []fixtureConsoleKey) error {
	if len(keys) == 0 || len(keys) > 16 {
		return errors.New("invalid fixture event count")
	}
	raw := make([]byte, 20*len(keys))
	for i, key := range keys {
		row := raw[i*20 : (i+1)*20]
		binary.LittleEndian.PutUint16(row[0:2], 1)
		if key.down {
			binary.LittleEndian.PutUint32(row[4:8], 1)
		}
		binary.LittleEndian.PutUint16(row[8:10], key.repeat)
		binary.LittleEndian.PutUint16(row[14:16], key.character)
	}
	write := windows.NewLazySystemDLL("kernel32.dll").NewProc("WriteConsoleInputW")
	if err := write.Find(); err != nil {
		return err
	}
	var count uint32
	ok, _, err := write.Call(uintptr(input), uintptr(unsafe.Pointer(&raw[0])), uintptr(len(keys)), uintptr(unsafe.Pointer(&count)))
	if ok == 0 {
		return err
	}
	if count != uint32(len(keys)) {
		return errors.New("incomplete console fixture input")
	}
	return nil
}
