//go:build windows

package wslreclaim

import (
	"github.com/SLktEx/Hacocoon/internal/wslcoord"
	"golang.org/x/sys/windows"
)

var errContinuationBusy = wslcoord.ErrBusy

type continuationGuard = wslcoord.Guard

func acquireContinuation(id windows.GUID) (*continuationGuard, error) {
	return wslcoord.AcquireReclamation(id)
}
