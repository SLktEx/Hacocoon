//go:build windows

// Package wslcoord coordinates native Hacocoon operations without granting WSL authority.
package wslcoord

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var ErrBusy = errors.New("another Hacocoon WSL operation is active")

// Guard reserves a kernel object name, not mutex thread ownership.
// Only creating a new object succeeds. Keeping the noninheritable handle alive
// excludes other continuations across processes and Windows login sessions.
// Closing it releases no storage or authority. A crash still needs a durable
// operation record before the future public workflow can safely retry.
type Guard struct {
	mu     sync.Mutex
	handle windows.Handle
}

// AcquireReclamation preserves the exact-registration continuation exclusion.
func AcquireReclamation(id windows.GUID) (*Guard, error) {
	if id == (windows.GUID{}) {
		return nil, errors.New("a nonzero WSL registration GUID is required")
	}
	return reserve("Reclamation", id.String())
}

var distributionName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,255}$`)

// AcquireLaunch excludes new background starts while reclaim owns the same
// Windows user's distribution name. This is coordination, never disk authority.
// Hold through child readiness, not just process creation or a preflight check.
func AcquireLaunch(distribution string) (*Guard, error) {
	if !distributionName.MatchString(distribution) {
		return nil, errors.New("invalid WSL distribution name")
	}
	sum := sha256.Sum256([]byte(strings.ToLower(distribution)))
	return reserve("WSLLaunch", fmt.Sprintf("%x", sum))
}

func reserve(scope, identity string) (*Guard, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	sid := user.User.Sid.String()
	name, err := windows.UTF16PtrFromString(`Global\Hacocoon.` + scope + "." + sid + "." + identity)
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
			return nil, errors.Join(ErrBusy, err)
		}
		return nil, fmt.Errorf("reserve WSL reclamation: %w", err)
	}
	return &Guard{handle: handle}, nil
}

func (g *Guard) Close() error {
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
