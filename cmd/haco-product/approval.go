package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type approvalClient interface {
	PendingApprovals(context.Context) ([]core.ApprovalRequest, error)
	DecideApproval(context.Context, string, capability.ApprovalDecision) (core.CapabilityResult, error)
}

func runApproval(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	client, err := controlapi.NewDefaultClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco: open approval review from the trusted Host")
		return 1
	}
	return approvalCommand(ctx, client, args, os.Stdin, os.Stdout, os.Stderr)
}

func approvalCommand(ctx context.Context, client approvalClient, args []string, in io.Reader, out, diagnostic io.Writer) int {
	flags := flag.NewFlagSet("haco approve", flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	list := flags.Bool("list", false, "list pending requests without deciding")
	jsonResult := flags.Bool("json", false, "print the decision receipt as JSON")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() > 1 || (*list && flags.NArg() != 0) {
		fmt.Fprintln(diagnostic, "Usage: haco approve [--json] [request-id] | haco approve --list")
		return 2
	}
	requests, err := client.PendingApprovals(ctx)
	if err != nil {
		fmt.Fprintln(diagnostic, "haco: cannot read pending approvals")
		return 1
	}
	if *list {
		// The full details are read through the trusted management boundary.
		if err := json.NewEncoder(out).Encode(requests); err != nil {
			return 1
		}
		return 0
	}
	if len(requests) == 0 {
		if flags.NArg() != 0 {
			fmt.Fprintln(diagnostic, "haco: request is no longer pending")
			return 1
		}
		if *jsonResult {
			fmt.Fprintln(out, "null")
		} else {
			fmt.Fprintln(out, "No pending approvals.")
		}
		return 0
	}
	fmt.Fprintf(diagnostic, "[waiting_approval] %d pending request(s); no decision has been submitted.\n", len(requests))
	if !interactiveInput(in) {
		fmt.Fprintln(diagnostic, "Approval remains pending. Use a terminal to review, or haco approve --list for scripts.")
		return 2
	}
	reader := bufio.NewReader(io.LimitReader(in, 4096))
	selected := -1
	if flags.NArg() == 1 {
		for i, request := range requests {
			if request.RequestID == flags.Arg(0) {
				selected = i
				break
			}
		}
	} else if len(requests) == 1 {
		selected = 0
	} else {
		for i, request := range requests {
			r := request.CapabilityRequest
			if _, err := fmt.Fprintf(diagnostic, "%d. %q %q environment=%q request=%q\n", i+1, r.Capability, r.Action, r.Environment, request.RequestID); err != nil {
				return 1
			}
		}
		if _, err := fmt.Fprint(diagnostic, "Choose a request (blank cancels): "); err != nil {
			return 1
		}
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return 1
		}
		if len(line) > 128 {
			return 2
		}
		if strings.TrimSpace(line) == "" {
			return 0
		}
		choice, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil || choice < 1 || choice > len(requests) {
			return 2
		}
		selected = choice - 1
	}
	if selected < 0 {
		fmt.Fprintln(diagnostic, "haco: request is no longer pending")
		return 1
	}
	request := requests[selected]
	if request.RequestID == "" {
		fmt.Fprintln(diagnostic, "haco: invalid approval request")
		return 1
	}
	decision, err := capability.NewStdioApproval(reader, diagnostic).Decide(ctx, request)
	if err != nil {
		fmt.Fprintln(diagnostic, "haco: approval answer was not submitted")
		return 1
	}
	result, err := client.DecideApproval(ctx, request.RequestID, decision)
	if result.RequestID == request.RequestID {
		result.Output, result.Provider = "", ""
		if displayErr := writeApprovalReceipt(out, result, decision, err, *jsonResult); displayErr != nil {
			return 1
		}
	}
	if err != nil {
		fmt.Fprintf(diagnostic, "haco: review outcome unavailable or unsuccessful for %q; inspect Policy and audit before retrying\n", request.RequestID)
		return 1
	}
	if result.RequestID != request.RequestID {
		fmt.Fprintln(diagnostic, "haco: approval receipt does not match the request")
		return 1
	}
	return 0
}

func writeApprovalReceipt(out io.Writer, result core.CapabilityResult, decision capability.ApprovalDecision, operationErr error, asJSON bool) error {
	if asJSON {
		return json.NewEncoder(out).Encode(result)
	}
	message := "Request outcome is not confirmed."
	if operationErr == nil {
		message = "Denied."
		if decision.Approved {
			message = "Approved."
		}
	} else {
		switch result.ExecutionState {
		case core.CapabilitySucceeded:
			message = "Capability completed; final confirmation failed."
		case core.CapabilityNotExecuted:
			message = "Request was not executed."
		case core.CapabilityFailed:
			message = "Request execution failed."
		}
	}
	if _, err := fmt.Fprintln(out, message); err != nil {
		return err
	}
	choices := map[string]string{
		string(capability.AllowEnvironment): "allow this scope in this Environment",
		string(capability.DenyEnvironment):  "deny this scope in this Environment",
		string(capability.AskEnvironment):   "ask every time for this scope in this Environment",
		string(capability.AllowGlobal):      "allow this scope in all Environments",
		string(capability.DenyGlobal):       "deny this scope in all Environments",
		string(capability.AskGlobal):        "ask every time for this scope in all Environments",
	}
	if choice := choices[result.SavedChoice]; choice != "" {
		if _, err := fmt.Fprintln(out, "Saved Policy:", choice); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(out, "Request: %q\n", result.RequestID)
	return err
}
