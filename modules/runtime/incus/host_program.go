package incus

import (
	"context"
	"fmt"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

// RunTrustedHostPython is a composition-only adapter for shipped integrations.
// The program is trusted code; request data is stdin, never interpolated into it.
func (r *Runtime) RunTrustedHostPython(ctx context.Context, program string, input []byte) ([]byte, error) {
	if len(program) == 0 || len(program) > 65536 || len(input) > 8192 {
		return nil, core.ErrInvalidArgument
	}
	if err := r.verifyTrustedHostOwnership(ctx); err != nil {
		return nil, err
	}
	runner, ok := r.runner.(host.InputRunner)
	if !ok {
		return nil, core.ErrUnsupported
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	result, err := runner.RunWithInput(ctx, input, "incus", "exec", trustedHostName, "--project", r.project, "--cwd", "/root", "--",
		"/usr/bin/systemd-run", "--quiet", "--collect", "--wait", "--pipe", "--service-type=exec",
		"--working-directory=/root", "--property=KillMode=control-group", "--property=TimeoutStopSec=5s", "--property=RuntimeMaxSec=110s", "--property=MemoryMax=512M", "--property=TasksMax=64",
		"/usr/bin/env", "-i", "PATH=/usr/local/bin:/usr/bin:/bin", "HOME=/root", "/usr/bin/python3", "-I", "-c", program)
	if err != nil || result.ExitCode != 0 || result.StdoutTruncated {
		return nil, fmt.Errorf("trusted Host integration failed: %w", core.ErrRuntimeUnavailable)
	}
	return []byte(result.Stdout), nil
}
