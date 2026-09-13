package incus

import (
	"context"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/recipes"
	"strings"
	"time"
)

// Script bytes are stdin to the verified logical Host, never Physical Host code.
func (r *Runtime) RunTrustedHostCustomization(ctx context.Context, script []byte) (host.Result, error) {
	text := string(script)
	if err := (recipes.Update{Script: &text}).Validate(); err != nil {
		return host.Result{ExitCode: -1}, err
	}
	if err := r.verifyTrustedHostOwnership(ctx); err != nil {
		return host.Result{ExitCode: -1}, err
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
		return host.Result{ExitCode: -1}, context.DeadlineExceeded
	}
	// A fixed transient unit refuses overlapping runs even after controller loss.
	// Its own cgroup deadline also stops descendants if the Incus client exits.
	return (host.ExecRunner{MaxOutputBytes: 64 << 10}).RunWithInput(ctx, script, "incus", "exec", trustedHostName, "--project", r.project, "--cwd", "/root", "--",
		"/usr/bin/systemd-run", "--unit=hacocoon-user-setup", "--collect", "--wait", "--pipe", "--service-type=exec",
		"--working-directory=/root", "--setenv=HOME=/root", "--property=KillMode=control-group", "--property=TimeoutStopSec=5s",
		fmt.Sprintf("--property=RuntimeMaxSec=%ds", seconds), "/bin/bash", "-se")
}

// The provider's incarnation UUID changes on recreation and is never supplied
// by a client or inferred from the reusable haco-host name.
func (r *Runtime) TrustedHostIdentity(ctx context.Context) (string, error) {
	if err := r.verifyTrustedHostOwnership(ctx); err != nil {
		return "", err
	}
	result, err := r.runner.Run(ctx, "incus", "config", "get", trustedHostName, "volatile.uuid", "--project", r.project)
	if err != nil || result.ExitCode != 0 || result.StdoutTruncated {
		return "", core.ErrIncompatibleState
	}
	id := strings.TrimSpace(result.Stdout)
	probe := recipes.HostResult{Instance: id, Digest: strings.Repeat("0", 64), State: "succeeded"}
	if !probe.Valid() {
		return "", core.ErrIncompatibleState
	}
	return id, nil
}
