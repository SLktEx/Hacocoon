package reviewcli

import (
	"context"
	"errors"
	"time"

	"golang.org/x/sys/windows"
)

// reviewSession separates an owned process from a published COM endpoint.
// The event is readiness only, never an approval or a peer identity credential.
// All ownership operations run on the same locked OS thread.
type reviewSession struct {
	mutex windows.Handle
	ready windows.Handle
	owned bool
}

func openReviewSession(name string, wait time.Duration) (session *reviewSession, err error) {
	if wait <= 0 || wait > time.Minute {
		return nil, errors.New("invalid notification session wait")
	}
	mutexName, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	readyName, err := windows.UTF16PtrFromString(name + "-ready")
	if err != nil {
		return nil, err
	}
	s := &reviewSession{}
	defer func() {
		if session == nil {
			s.close()
		}
	}()
	s.mutex, err = windows.CreateMutex(nil, false, mutexName)
	if err != nil && err != windows.ERROR_ALREADY_EXISTS {
		return nil, err
	}
	s.ready, err = windows.CreateEvent(nil, 1, 0, readyName)
	if err != nil && err != windows.ERROR_ALREADY_EXISTS {
		return nil, err
	}
	// Prefer ownership when both are signaled: a stale event from an abandoned
	// process is cleared before the new owner does any native work.
	state, err := windows.WaitForMultipleObjects([]windows.Handle{s.mutex, s.ready}, false, uint32(wait/time.Millisecond))
	if err != nil {
		return nil, err
	}
	switch state {
	case windows.WAIT_OBJECT_0, windows.WAIT_ABANDONED:
		s.owned = true
		if err := s.withdraw(); err != nil {
			return nil, err
		}
	case windows.WAIT_OBJECT_0 + 1:
	case uint32(windows.WAIT_TIMEOUT):
		return nil, context.DeadlineExceeded
	default:
		return nil, errors.New("cannot own notification session")
	}
	return s, nil
}

func (s *reviewSession) publish() error {
	if !s.owned {
		return errors.New("notification session is not owned")
	}
	return windows.SetEvent(s.ready)
}
func (s *reviewSession) withdraw() error {
	if !s.owned {
		return errors.New("notification session is not owned")
	}
	return windows.ResetEvent(s.ready)
}
func (s *reviewSession) close() {
	if s.owned {
		_ = s.withdraw()
		_ = windows.ReleaseMutex(s.mutex)
		s.owned = false
	}
	if s.ready != 0 {
		_ = windows.CloseHandle(s.ready)
		s.ready = 0
	}
	if s.mutex != 0 {
		_ = windows.CloseHandle(s.mutex)
		s.mutex = 0
	}
}
