//go:build windows && (amd64 || arm64)

package reclaimnative

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// This transport owns observation handles only, never a Job, console or a
// termination right used for cleanup. Closing a process handle is nonterminating:
// https://learn.microsoft.com/en-us/windows/win32/api/handleapi/nf-handleapi-closehandle
// No Wait goroutine, process/PID reopening, signal, retry or leaked owner exists.
type nativeProcess struct{ process, output windows.Handle }

func (h *installedHelper) start(ctx context.Context, args []string) (_ *nativeProcess, err error) {
	var observed *nativeProcess
	defer func() {
		if err != nil && observed != nil {
			err = errors.Join(err, observed.close())
		}
	}()
	system, err := windows.GetSystemDirectory()
	if err != nil {
		return nil, err
	}
	root, err := windows.GetWindowsDirectory()
	if err != nil {
		return nil, err
	}
	application, err := windows.UTF16PtrFromString(h.executable)
	if err != nil {
		return nil, err
	}
	command, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(append([]string{h.executable}, args...)))
	if err != nil {
		return nil, err
	}
	directory, err := windows.UTF16PtrFromString(system)
	if err != nil {
		return nil, err
	}
	environment := utf16.Encode([]rune(strings.Join([]string{"SystemRoot=" + root, "WINDIR=" + root}, "\x00") + "\x00\x00"))
	var read, write windows.Handle
	if err = windows.CreatePipe(&read, &write, nil, 4096); err != nil {
		return nil, err
	}
	defer func() {
		if read != 0 {
			err = errors.Join(err, windows.CloseHandle(read))
		}
	}()
	defer func() {
		if write != 0 {
			err = errors.Join(err, windows.CloseHandle(write))
		}
	}()
	// The documented API explicitly accepts CreatePipe handles. Establish the
	// nonblocking mode before dispatch or refuse: anonymous pipes do not support
	// overlapped I/O, and PeekNamedPipe alone may block in a multithreaded program.
	// This is bounded synchronous polling, not asynchronous pipe I/O.
	// https://learn.microsoft.com/en-us/windows/win32/api/namedpipeapi/nf-namedpipeapi-setnamedpipehandlestate
	mode := uint32(windows.PIPE_READMODE_BYTE | windows.PIPE_NOWAIT)
	if err = windows.SetNamedPipeHandleState(read, &mode, nil, nil); err != nil {
		return nil, err
	}
	nullName, err := windows.UTF16PtrFromString(`\\.\NUL`)
	if err != nil {
		return nil, err
	}
	null, err := windows.CreateFile(nullName, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, windows.CloseHandle(null)) }()
	handles := make([]windows.Handle, 3)
	defer func() {
		for _, handle := range handles {
			if handle != 0 {
				err = errors.Join(err, windows.CloseHandle(handle))
			}
		}
	}()
	for i, source := range []windows.Handle{null, write, null} {
		if err = windows.DuplicateHandle(windows.CurrentProcess(), source, windows.CurrentProcess(), &handles[i], 0, true, windows.DUPLICATE_SAME_ACCESS); err != nil {
			return nil, err
		}
	}
	attributes, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return nil, err
	}
	defer attributes.Delete()
	if err = attributes.Update(windows.PROC_THREAD_ATTRIBUTE_HANDLE_LIST, unsafe.Pointer(&handles[0]), uintptr(len(handles))*unsafe.Sizeof(handles[0])); err != nil {
		return nil, err
	}
	startup := windows.StartupInfoEx{}
	startup.Cb = uint32(unsafe.Sizeof(startup))
	startup.Flags = windows.STARTF_USESTDHANDLES | windows.STARTF_USESHOWWINDOW
	startup.ShowWindow = windows.SW_HIDE
	startup.StdInput, startup.StdOutput, startup.StdErr = handles[0], handles[1], handles[2]
	startup.ProcThreadAttributeList = attributes.List()
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	var process windows.ProcessInformation
	// An outer Job is unchanged. In particular, this launcher does not add a
	// private kill-on-close Job or weaken the worker's existing breakaway flags.
	err = windows.CreateProcess(application, command, nil, nil, true, windows.EXTENDED_STARTUPINFO_PRESENT|windows.CREATE_UNICODE_ENVIRONMENT|windows.CREATE_NO_WINDOW, &environment[0], directory, &startup.StartupInfo, &process)
	runtime.KeepAlive(handles)
	runtime.KeepAlive(environment)
	if err != nil {
		return nil, err
	}
	observed = &nativeProcess{process: process.Process, output: read}
	read = 0
	if err = windows.CloseHandle(process.Thread); err != nil {
		return nil, err
	}
	// Close our output writer before polling so EOF can actually be observed.
	err = windows.CloseHandle(write)
	write = 0
	if err != nil {
		return nil, err
	}
	return observed, nil
}

func (p *nativeProcess) read(dst []byte) (int, bool, error) {
	var n uint32
	err := windows.ReadFile(p.output, dst, &n, nil)
	switch {
	case errors.Is(err, windows.ERROR_NO_DATA):
		return int(n), false, nil
	case errors.Is(err, windows.ERROR_BROKEN_PIPE):
		return int(n), true, nil
	case err != nil:
		return int(n), false, err
	default:
		return int(n), false, nil
	}
}
func (p *nativeProcess) exit() (bool, uint32, error) {
	state, err := windows.WaitForSingleObject(p.process, 0)
	if err != nil {
		return false, 0, err
	}
	switch state {
	case uint32(windows.WAIT_TIMEOUT):
		return false, 0, nil
	case windows.WAIT_OBJECT_0:
		var code uint32
		err = windows.GetExitCodeProcess(p.process, &code)
		return true, code, err
	default:
		return false, 0, errors.New("unexpected helper process observation")
	}
}
func (p *nativeProcess) close() error {
	var errs []error
	if p.output != 0 {
		errs = append(errs, windows.CloseHandle(p.output))
		p.output = 0
	}
	if p.process != 0 {
		errs = append(errs, windows.CloseHandle(p.process))
		p.process = 0
	}
	return errors.Join(errs...)
}
