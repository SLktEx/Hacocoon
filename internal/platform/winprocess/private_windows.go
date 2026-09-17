// Package winprocess owns private Windows pipe processes and their descendants.
package winprocess

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Private holds exact kernel ownership, never a PID/name-based cleanup target.
// Close must succeed before a caller releases a launch reservation.
type Private struct {
	Input   *os.File
	Output  *os.File
	job     windows.Handle
	process windows.Handle
	once    sync.Once
	err     error
}

// Start attaches the process to a private kill-on-close job at creation, before
// it can spawn children. Only the three anonymous standard handles are inherited.
// A failed job/handle-list setup refuses launch; there is no uncontained fallback.
func Start(file string, args, env []string, dir string) (_ *Private, err error) {
	if !filepath.IsAbs(file) || !filepath.IsAbs(dir) || env == nil {
		return nil, errors.New("private process requires explicit executable, directory and environment")
	}
	for _, value := range append(append([]string{file, dir}, args...), env...) {
		if strings.ContainsRune(value, 0) {
			return nil, errors.New("invalid private process argument")
		}
	}
	application, err := windows.UTF16PtrFromString(file)
	if err != nil {
		return nil, err
	}
	command, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(append([]string{file}, args...)))
	if err != nil {
		return nil, err
	}
	directory, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return nil, err
	}
	environment := utf16.Encode([]rune(strings.Join(env, "\x00") + "\x00\x00"))
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			windows.CloseHandle(job)
		}
	}()
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return nil, err
	}
	inputRead, inputWrite, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer inputRead.Close()
	defer func() {
		if err != nil {
			inputWrite.Close()
		}
	}()
	outputRead, outputWrite, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer outputWrite.Close()
	defer func() {
		if err != nil {
			outputRead.Close()
		}
	}()
	// Raw child diagnostics may contain credentials; never inherit a terminal.
	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return nil, err
	}
	defer null.Close()
	handles := make([]windows.Handle, 3)
	defer func() {
		for _, handle := range handles {
			if handle != 0 {
				windows.CloseHandle(handle)
			}
		}
	}()
	for i, source := range []*os.File{inputRead, outputWrite, null} {
		if err = windows.DuplicateHandle(windows.CurrentProcess(), windows.Handle(source.Fd()), windows.CurrentProcess(), &handles[i], 0, true, windows.DUPLICATE_SAME_ACCESS); err != nil {
			return nil, err
		}
	}
	attributes, err := windows.NewProcThreadAttributeList(2)
	if err != nil {
		return nil, err
	}
	defer attributes.Delete()
	if err = attributes.Update(windows.PROC_THREAD_ATTRIBUTE_HANDLE_LIST, unsafe.Pointer(&handles[0]), uintptr(len(handles))*unsafe.Sizeof(handles[0])); err != nil {
		return nil, err
	}
	// PROC_THREAD_ATTRIBUTE_JOB_LIST (Windows 10+), documented by
	// UpdateProcThreadAttribute; x/sys does not expose this constant yet.
	const jobList = 0x0002000d
	if err = attributes.Update(jobList, unsafe.Pointer(&job), unsafe.Sizeof(job)); err != nil {
		return nil, err
	}
	startup := windows.StartupInfoEx{}
	startup.Cb = uint32(unsafe.Sizeof(startup))
	startup.Flags = windows.STARTF_USESTDHANDLES | windows.STARTF_USESHOWWINDOW
	startup.ShowWindow = windows.SW_HIDE
	startup.StdInput, startup.StdOutput, startup.StdErr = handles[0], handles[1], handles[2]
	startup.ProcThreadAttributeList = attributes.List()
	var process windows.ProcessInformation
	err = windows.CreateProcess(application, command, nil, nil, true,
		windows.EXTENDED_STARTUPINFO_PRESENT|windows.CREATE_UNICODE_ENVIRONMENT|windows.CREATE_NO_WINDOW,
		&environment[0], directory, &startup.StartupInfo, &process)
	runtime.KeepAlive(handles)
	runtime.KeepAlive(job)
	if err != nil {
		return nil, err
	}
	windows.CloseHandle(process.Thread)
	return &Private{Input: inputWrite, Output: outputRead, job: job, process: process.Process}, nil
}

// Close cancels the entire owned tree and confirms it is empty. An observation
// failure retains the job handle (kill-on-parent-exit) and reports failure; it
// must not authorize releasing a caller's launch reservation.
func (p *Private) Close() error {
	p.once.Do(func() {
		p.Input.Close()
		p.Output.Close()
		defer windows.CloseHandle(p.process)
		if err := windows.TerminateJobObject(p.job, 1); err != nil {
			p.err = err
			return
		}
		deadline := time.Now().Add(2 * time.Second)
		for {
			// JOBOBJECT_BASIC_ACCOUNTING_INFORMATION, winnt.h. Querying
			// membership avoids a race with descendants after root exit.
			var accounting struct {
				TotalUser, TotalKernel, PeriodUser, PeriodKernel int64
				PageFaults, Total, Active, Terminated            uint32
			}
			if err := windows.QueryInformationJobObject(p.job, windows.JobObjectBasicAccountingInformation, uintptr(unsafe.Pointer(&accounting)), uint32(unsafe.Sizeof(accounting)), nil); err != nil {
				p.err = err
				return
			}
			if accounting.Active == 0 {
				p.err = windows.CloseHandle(p.job)
				return
			}
			if time.Now().After(deadline) {
				p.err = errors.New("private process cleanup not confirmed")
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	})
	return p.err
}
