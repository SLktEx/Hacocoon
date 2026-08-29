package capability

import (
	"context"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type LocalEcho struct{}

func (LocalEcho) Capability() string { return "local.echo" }

func (LocalEcho) Execute(_ context.Context, req core.CapabilityRequest) (core.CapabilityResult, error) {
	if req.Action != "echo" {
		return core.CapabilityResult{}, fmt.Errorf("local.echo action %q: %w", req.Action, core.ErrUnsupported)
	}
	return core.CapabilityResult{Provider: "local.echo", Output: req.Parameters["message"]}, nil
}
