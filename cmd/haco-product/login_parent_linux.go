//go:build linux

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// WSL starts /bin/login -f on a private PTY to establish the systemd user
// session before launching the real interactive shell from /init. TTY presence
// cannot distinguish them. A login-managed shell stays on the Physical Host.
// This is UX routing, never an authorization check: either route retains the
// caller's existing authority, and controller operations still authorize peers.
func loginBootstrapParent() (bool, error) {
	comm, err := os.ReadFile("/proc/" + strconv.Itoa(os.Getppid()) + "/comm")
	if err != nil {
		return false, fmt.Errorf("inspect login-shell parent: %w", err)
	}
	return strings.TrimSpace(string(comm)) == "login", nil
}
