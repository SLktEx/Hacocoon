package capability

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
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
	if _, err := fmt.Fprintf(a.out, "Approve capability %s action=%s resource=%s environment=%s", terminalSafe(request.Capability), terminalSafe(request.Action), terminalSafe(request.Resource), terminalSafe(request.Environment)); err != nil {
		return false, fmt.Errorf("display approval request: %w", err)
	}
	keys := make([]string, 0, len(request.Attributes))
	for key := range request.Attributes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, err := fmt.Fprintf(a.out, " %s=%s", terminalSafe(key), terminalSafe(request.Attributes[key])); err != nil {
			return false, fmt.Errorf("display approval request: %w", err)
		}
	}
	if _, err := fmt.Fprintf(a.out, " reason=%s? [y/N] ", terminalSafe(req.Reason)); err != nil {
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

func terminalSafe(value string) string {
	quoted := strconv.QuoteToGraphic(value)
	if len(quoted) < 2 {
		return quoted
	}
	return quoted[1 : len(quoted)-1]
}
