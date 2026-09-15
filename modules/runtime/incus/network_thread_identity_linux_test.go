//go:build linux

package incus

import (
	"fmt"
	"os"
	"runtime"
	"testing"

	"golang.org/x/sys/unix"
)

func TestNativeNetworkNamespaceUsesCallingThread(t *testing.T) {
	if os.Getenv("HACO_E2E_NETWORK_NAMESPACE") != "1" {
		t.Skip("set HACO_E2E_NETWORK_NAMESPACE=1 with existing namespace creation authority")
	}
	result := make(chan error, 1)
	finished := make(chan struct{})
	var probe func()
	probe = func() {
		runtime.LockOSThread()
		if unix.Gettid() == os.Getpid() {
			// Leave the process leader untouched and pinned while another thread probes.
			go probe()
			<-finished
			runtime.UnlockOSThread()
			return
		}
		// As in the product dialer, a changed thread must never reenter Go's pool.
		result <- inspectCallingThreadNamespace()
	}
	go probe()
	err := <-result
	close(finished)
	if err != nil {
		t.Fatal(err)
	}
}

func inspectCallingThreadNamespace() error {
	if err := unix.Unshare(unix.CLONE_NEWNET); err != nil {
		return fmt.Errorf("create isolated probe namespace: %w", err)
	}
	fd, err := openCurrentNetworkNamespace()
	if err != nil {
		return err
	}
	defer func() { _ = unix.Close(fd) }()
	var observed, current, leader unix.Stat_t
	if err = unix.Fstat(fd, &observed); err != nil {
		return err
	}
	if err = unix.Stat("/proc/thread-self/ns/net", &current); err != nil {
		return err
	}
	if err = unix.Stat("/proc/self/ns/net", &leader); err != nil {
		return err
	}
	if current.Dev == leader.Dev && current.Ino == leader.Ino {
		return fmt.Errorf("probe did not separate calling thread from process leader")
	}
	if observed.Dev != current.Dev || observed.Ino != current.Ino {
		return fmt.Errorf("namespace identity came from another thread")
	}
	return nil
}
