package incus

import (
	"bytes"
	"context"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/hostsetup"
	"os/exec"
	"time"
)

// Script bytes are stdin to the verified logical Host, never Physical Host code.
func (r *Runtime) RunTrustedHostCustomization(ctx context.Context, script []byte) error {
	text := string(script)
	if err := (hostsetup.Update{Script: &text}).Validate(); err != nil {
		return err
	}
	if err := r.verifyTrustedHostOwnership(ctx); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	seconds := int64(14 * 60)
	if deadline, ok := ctx.Deadline(); ok {
		remaining := int64((time.Until(deadline) - 10*time.Second) / time.Second)
		if remaining < seconds {
			seconds = remaining
		}
	}
	if seconds < 1 {
		return context.DeadlineExceeded
	}
	// A fixed transient unit refuses overlapping runs even after controller loss.
	// Its own cgroup deadline also stops descendants if the Incus client exits.
	command := exec.CommandContext(ctx, "incus", "exec", trustedHostName, "--project", r.project, "--cwd", "/root", "--",
		"/usr/bin/systemd-run", "--unit=hacocoon-user-setup", "--collect", "--wait", "--pipe", "--service-type=exec",
		"--working-directory=/root", "--property=KillMode=control-group", "--property=TimeoutStopSec=5s",
		fmt.Sprintf("--property=RuntimeMaxSec=%ds", seconds), "/bin/bash", "-se")
	command.Stdin = bytes.NewReader(script)
	command.WaitDelay = time.Second
	if err := command.Run(); err != nil {
		return fmt.Errorf("trusted Host customization failed: %w", err)
	}
	return nil
}
