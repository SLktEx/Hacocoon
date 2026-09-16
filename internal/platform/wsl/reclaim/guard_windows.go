//go:build windows

package wslreclaim

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var errContinuationBusy = errors.New("a reclamation continuation for this WSL registration is already active")

// continuationGuard reserves a kernel object name, not mutex thread ownership.
// Only creating a new object succeeds. Keeping the noninheritable handle alive
// excludes other continuations across processes and Windows login sessions.
// Closing it releases no storage or authority. A crash still needs a durable
// operation record before the future public workflow can safely retry.
type continuationGuard struct {
	mu     sync.Mutex
	handle windows.Handle
}

func acquireContinuation(id windows.GUID) (*continuationGuard, error) {
	if id == (windows.GUID{}) {
		return nil, errors.New("a nonzero WSL registration GUID is required")
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	sid := user.User.Sid.String()
	name, err := windows.UTF16PtrFromString(`Global\Hacocoon.Reclamation.` + sid + "." + id.String())
	if err != nil {
		return nil, err
	}
	// A different Windows user may deny service by precreating the name, but
	// cannot make an existing object count as a successful reservation.
	// The name/ACL are not permission to stop or compact any distribution.
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;GA;;;SY)(A;;GA;;;" + sid + ")")
	if err != nil {
		return nil, err
	}
	attrs := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	handle, err := windows.CreateMutexEx(&attrs, name, 0, windows.SYNCHRONIZE)
	if err != nil {
		if handle != 0 {
			err = errors.Join(err, windows.CloseHandle(handle))
		}
		if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
			return nil, errors.Join(errContinuationBusy, err)
		}
		return nil, fmt.Errorf("reserve WSL reclamation: %w", err)
	}
	return &continuationGuard{handle: handle}, nil
}

func (g *continuationGuard) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.handle == 0 {
		return nil
	}
	if err := windows.CloseHandle(g.handle); err != nil {
		return err
	}
	g.handle = 0
	return nil
}
