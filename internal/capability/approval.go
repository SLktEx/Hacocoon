package capability

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type StdioApproval struct {
	in  io.Reader
	out io.Writer
}

func NewStdioApproval(in io.Reader, out io.Writer) *StdioApproval {
	return &StdioApproval{in: in, out: out}
}

func (a *StdioApproval) Approve(ctx context.Context, req core.ApprovalRequest) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}
	if a == nil || a.in == nil || a.out == nil {
		return false, fmt.Errorf("approval terminal unavailable")
	}
	_, _ = fmt.Fprintf(a.out, "Approve capability %s action=%s resource=%s", req.CapabilityRequest.Capability, req.CapabilityRequest.Action, req.CapabilityRequest.Resource)
	keys := make([]string, 0, len(req.CapabilityRequest.Attributes))
	for key := range req.CapabilityRequest.Attributes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		_, _ = fmt.Fprintf(a.out, " %s=%s", key, req.CapabilityRequest.Attributes[key])
	}
	_, _ = fmt.Fprint(a.out, "? [y/N] ")
	line, err := bufio.NewReader(a.in).ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}
