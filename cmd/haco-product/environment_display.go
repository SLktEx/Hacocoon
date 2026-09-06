package main

import (
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
	"net"
	"regexp"
)

var configEnvironmentName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,56}$`)

func writeSSHConfig(out io.Writer, name string, connections []core.ClientConnection) error {
	if !configEnvironmentName.MatchString(name) {
		return core.ErrInvalidArgument
	}
	var ssh []core.ClientConnection
	for _, c := range connections {
		if c.Kind == "ssh" {
			ssh = append(ssh, c)
		}
	}
	if len(ssh) != 1 {
		return fmt.Errorf("expected one prepared SSH connection; run 'haco env ssh --key <public-key-file> %s', or disconnect extra SSH connections", name)
	}
	c := ssh[0]
	ip := net.ParseIP(c.Host)
	if ip == nil || !ip.IsLoopback() || c.Port < 1 || c.Port > 65535 || c.User != "root" {
		return core.ErrIncompatibleState
	}
	_, err := fmt.Fprintf(out, "# Use from the controller's Physical Host or its Windows loopback.\n# Keep IdentityFile and trusted host-key pinning on your SSH client.\nHost haco-%s\n  HostName %s\n  Port %d\n  User %s\n  StrictHostKeyChecking yes\n", name, ip.String(), c.Port, c.User)
	return err
}

func writeEnvironmentStatus(out io.Writer, status core.EnvironmentStatus) int {
	env := status.Environment
	if _, err := fmt.Fprintf(out, "Environment: %s\nState:       %s\nWorkspace:   %s\nAccess:      %s\n", env.Name, status.State, env.Workspace.Path, env.AccessMode); err != nil {
		return 1
	}
	if env.Base != nil {
		if _, err := fmt.Fprintf(out, "Base:        %s\nRevision:    %s\n", env.Base.Name, env.Base.Revision); err != nil {
			return 1
		}
	}
	if status.State == core.EnvironmentStopped {
		if _, err := fmt.Fprintln(out, "Workspace retained; this Environment is stopped."); err != nil {
			return 1
		}
	}
	return 0
}
