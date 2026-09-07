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
	decision, err := a.decide(ctx, req, false)
	return decision.Approved, err
}
func (a *StdioApproval) Decide(ctx context.Context, req core.ApprovalRequest) (ApprovalDecision, error) {
	return a.decide(ctx, req, true)
}

func (a *StdioApproval) decide(ctx context.Context, req core.ApprovalRequest, persistent bool) (ApprovalDecision, error) {
	select {
	case <-ctx.Done():
		return ApprovalDecision{}, ctx.Err()
	default:
	}
	if a == nil || a.in == nil || a.out == nil {
		return ApprovalDecision{}, fmt.Errorf("approval terminal unavailable")
	}
	request := req.CapabilityRequest
	if _, err := fmt.Fprintf(a.out, "Approve capability %s action=%s resource=%s environment=%s", terminalSafe(request.Capability), terminalSafe(request.Action), terminalSafe(request.Resource), terminalSafe(request.Environment)); err != nil {
		return ApprovalDecision{}, fmt.Errorf("display approval request: %w", err)
	}
	keys := make([]string, 0, len(request.Attributes))
	for key := range request.Attributes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, err := fmt.Fprintf(a.out, " %s=%s", terminalSafe(key), terminalSafe(request.Attributes[key])); err != nil {
			return ApprovalDecision{}, fmt.Errorf("display approval request: %w", err)
		}
	}
	options := "y/N"
	if persistent {
		options = "y/N; 1=allow this Environment, 2=deny this Environment, 3=allow all Environments, 4=deny all Environments"
	}
	if _, err := fmt.Fprintf(a.out, " reason=%s? [%s] ", terminalSafe(req.Reason), options); err != nil {
		return ApprovalDecision{}, fmt.Errorf("display approval request: %w", err)
	}
	line, err := bufio.NewReader(io.LimitReader(a.in, 129)).ReadString('\n')
	if err != nil && err != io.EOF {
		return ApprovalDecision{}, err
	}
	select {
	case <-ctx.Done():
		return ApprovalDecision{}, ctx.Err()
	default:
	}
	if len(line) > 128 {
		return ApprovalDecision{}, core.ErrInvalidArgument
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	decision := ApprovalDecision{Approved: answer == "y" || answer == "yes"}
	if persistent {
		switch answer {
		case "1":
			decision = ApprovalDecision{Approved: true, Save: AllowEnvironment}
		case "2":
			decision = ApprovalDecision{Save: DenyEnvironment}
		case "3":
			decision = ApprovalDecision{Approved: true, Save: AllowGlobal}
		case "4":
			decision = ApprovalDecision{Save: DenyGlobal}
		}
		if decision.Save != "" {
			if _, err := RuleForSavedChoice(request, decision.Save); err != nil {
				return ApprovalDecision{}, err
			}
		}
	}
	return decision, nil
}

func terminalSafe(value string) string {
	quoted := strconv.QuoteToGraphic(value)
	if len(quoted) < 2 {
		return quoted
	}
	return quoted[1 : len(quoted)-1]
}
