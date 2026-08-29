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
	request := req.CapabilityRequest
	if _, err := fmt.Fprintf(
		a.out,
		"Approve capability %s action=%s resource=%s environment=%s parameters=%s reason=%s? [y/N] ",
		request.Capability,
		request.Action,
		request.Resource,
		request.Environment,
		formatApprovalParameters(request.Parameters),
		req.Reason,
	); err != nil {
		return false, fmt.Errorf("display approval request: %w", err)
	}
	line, err := bufio.NewReader(a.in).ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

func formatApprovalParameters(parameters map[string]string) string {
	if len(parameters) == 0 {
		return "{}"
	}
	keys := make([]string, 0, len(parameters))
	for key := range parameters {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%q", key, parameters[key]))
	}
	return "{" + strings.Join(parts, ",") + "}"
}
