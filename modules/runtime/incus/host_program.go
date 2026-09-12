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
	result, err := runner.RunWithInput(ctx, input, "incus", r.trustedHostPythonArgs(program, "110s")...)
	if err != nil || result.ExitCode != 0 || result.StdoutTruncated {
		return nil, fmt.Errorf("trusted Host integration failed: %w", core.ErrRuntimeUnavailable)
	}
	return []byte(result.Stdout), nil
}
