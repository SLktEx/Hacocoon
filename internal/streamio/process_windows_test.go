package streamio

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

// Explicit native transport acceptance: the Linux test executable lives in a
// disposable source snapshot. No installed product, distro settings or Env is
// replaced. This proves pipe/WSL byte delivery, not packaged controller access.
func TestFramedNativeWindowsWSLBridge(t *testing.T) {
	child := os.Getenv("HACO_TEST_WSL_FRAMED_CHILD")
	distro := os.Getenv("HACO_TEST_WSL_DISTRIBUTION")
	if child == "" || distro == "" {
		t.Skip("native WSL fixture not supplied")
	}
	root := os.Getenv("SystemRoot")
	cmd := exec.Command(filepath.Join(root, "System32", "wsl.exe"), "--distribution", distro, "--exec", "/usr/bin/env", "-i", "PATH=/usr/local/bin:/usr/bin:/bin", "LANG=C.UTF-8", "HACO_TEST_FRAMED_CHILD=bridge", child, "-test.run=^TestFramedProcessChild$")
	cmd.Env = []string{"SystemRoot=" + root, "WINDIR=" + root}
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	testFramedChildRoundTrip(t, cmd)
}
