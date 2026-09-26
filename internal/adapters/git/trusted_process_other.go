//go:build !linux

package gitadapter

import (
	"fmt"
	"os/exec"
)

func configureGitProcess(*exec.Cmd) error {
	return fmt.Errorf("trusted Git execution requires a Linux Host (including WSL)")
}
