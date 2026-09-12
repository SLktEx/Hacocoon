package capability

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/cliui"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type StdioApproval struct {
	in       io.Reader
	out      io.Writer
	language cliui.Language
}

func NewStdioApproval(in io.Reader, out io.Writer) *StdioApproval {
	return NewLocalizedStdioApproval(in, out, cliui.English)
}

// NewLocalizedStdioApproval owns its display language per terminal instance.
// Language selection cannot change the accepted input or authorization scope.
func NewLocalizedStdioApproval(in io.Reader, out io.Writer, language cliui.Language) *StdioApproval {
	return &StdioApproval{in: in, out: out, language: language}
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
	if _, err := fmt.Fprintf(a.out, a.language.Text("approval.prompt_header"), terminalSafe(request.Capability), terminalSafe(request.Action), terminalSafe(request.Resource), terminalSafe(request.Environment)); err != nil {
		return ApprovalDecision{}, fmt.Errorf("display approval request: %w", err)
	}
	if request.EnvironmentInstance != "" {
		if _, err := fmt.Fprintf(a.out, " instance=%s", terminalSafe(request.EnvironmentInstance)); err != nil {
			return ApprovalDecision{}, err
		}
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
	if persistent && req.SavedScope != nil {
		scope := *req.SavedScope
		if _, err := fmt.Fprintf(a.out, a.language.Text("approval.prompt_scope"), terminalSafe(scope.Capability), terminalSafe(scope.Action), terminalSafe(scope.Resource)); err != nil {
			return ApprovalDecision{}, err
		}
		keys := make([]string, 0, len(scope.Attributes))
		for key := range scope.Attributes {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if _, err := fmt.Fprintf(a.out, " %s=%s", terminalSafe(key), terminalSafe(scope.Attributes[key])); err != nil {
				return ApprovalDecision{}, err
			}
		}
	}
	options := a.language.Text("approval.options_once")
	if persistent {
		options = a.language.Text("approval.options_persistent")
		if !core.ValidEnvironmentInstanceID(request.EnvironmentInstance) {
			options = a.language.Text("approval.options_global")
		}
	}
	if _, err := fmt.Fprintf(a.out, a.language.Text("approval.prompt_reason"), terminalSafe(req.Reason), options); err != nil {
		return ApprovalDecision{}, fmt.Errorf("display approval request: %w", err)
	}
	reader := bufio.NewReader(io.LimitReader(a.in, 258))
	line, err := reader.ReadString('\n')
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
		case "5":
			decision.Save = AskEnvironment
		case "6":
			decision.Save = AskGlobal
		}
		if decision.Save != "" {
			if _, err := approvalSavedRule(req, decision.Save); err != nil {
				return ApprovalDecision{}, err
			}
		}
	}
	if decision.Save == AskEnvironment || decision.Save == AskGlobal {
		// Saving ask does not authorize this operation. Collect its one-shot answer
		// from the same buffered reader, including when both lines arrived together.
		decision.Approved, err = NewLocalizedStdioApproval(reader, a.out, a.language).Approve(ctx, req)
		if err != nil {
			return ApprovalDecision{}, err
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

func approvalSavedRule(req core.ApprovalRequest, choice SavedChoice) (PolicyRule, error) {
	if req.SavedScope != nil {
		return RuleForSavedScope(*req.SavedScope, choice)
	}
	return RuleForSavedChoice(req.CapabilityRequest, choice)
}
