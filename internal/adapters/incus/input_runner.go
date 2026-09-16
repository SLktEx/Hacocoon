package incus

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

// preserveExecInput retains the optional stdin contract through command
// decorators. Only Incus exec may use this route: management commands must
// continue through the decorator's ownership and acquisition checks.
func preserveExecInput(decorated, inner host.Runner) host.Runner {
	input, ok := inner.(host.InputRunner)
	if !ok {
		return decorated
	}
	return execInputRunner{Runner: decorated, input: input}
}

type execInputRunner struct {
	host.Runner
	input host.InputRunner
}

func (r execInputRunner) RunWithInput(ctx context.Context, input []byte, name string, args ...string) (host.Result, error) {
	if strings.ToLower(filepath.Base(name)) != "incus" || len(args) < 2 || args[0] != "exec" {
		return host.Result{ExitCode: -1}, core.ErrUnsupported
	}
	return r.input.RunWithInput(ctx, input, name, args...)
}
