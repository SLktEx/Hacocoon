//go:build windows && (amd64 || arm64)

package wslreclaim

import (
	"unsafe"

	"github.com/SLktEx/Hacocoon/internal/storage/reclamation"
	"golang.org/x/sys/windows"
)

// Both APIs are read-only and use the current process / NULL Job. NULL selects
// the calling process's immediate Job, never the whole nested Job hierarchy:
// https://learn.microsoft.com/en-us/windows/win32/api/jobapi2/nf-jobapi2-queryinformationjobobject
func observeLauncherJob() reclamation.JobObservation {
	kernel := windows.NewLazySystemDLL("kernel32.dll")
	membership := kernel.NewProc("IsProcessInJob")
	query := kernel.NewProc("QueryInformationJobObject")
	return observeJob(func() (bool, error) {
		if err := membership.Find(); err != nil {
			return false, err
		}
		var inJob int32
		result, _, err := membership.Call(uintptr(windows.CurrentProcess()), 0, uintptr(unsafe.Pointer(&inJob)))
		if result == 0 {
			return false, err
		}
		return inJob != 0, nil
	}, func() (uint32, error) {
		if err := query.Find(); err != nil {
			return 0, err
		}
		var limits windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
		result, _, err := query.Call(0, uintptr(windows.JobObjectExtendedLimitInformation), uintptr(unsafe.Pointer(&limits)), unsafe.Sizeof(limits), 0)
		if result == 0 {
			return 0, err
		}
		return limits.BasicLimitInformation.LimitFlags, nil
	})
}
