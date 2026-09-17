//go:build linux

package gitadapter

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// Git may spawn a remote or credential helper. Cancellation must terminate the
// entire request's process group, not leave a helper using Host credentials.
func configureGitProcess(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	return nil
}
