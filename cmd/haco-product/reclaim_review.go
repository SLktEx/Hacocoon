package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/reclamation"
)

func confirmReclamation(ctx context.Context, in io.Reader, out io.Writer) (bool, error) {
	if _, err := fmt.Fprint(out, "Continue? [y/N] "); err != nil {
		return false, err
	}
	answers := make(chan bool, 1)
	go func() {
		answer, err := bufio.NewReader(io.LimitReader(in, 128)).ReadString('\n')
		answers <- err == nil && (strings.EqualFold(strings.TrimSpace(answer), "y") || strings.EqualFold(strings.TrimSpace(answer), "yes"))
	}()
	select {
	case answer := <-answers:
		return answer, nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

func reclaimReviewCommand(ctx context.Context, args []string, in io.Reader, out, diagnostic io.Writer,
	target func(context.Context) (reclamation.WSLTarget, error),
	invoke func(context.Context, reclamation.WSLTarget, string) ([]byte, error),
	review func(context.Context, reclamation.WSLTarget, string, string) error,
) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(out, "Usage: haco reclaim --review [--yes]\nRetain an unsuccessful operation without starting another run.")
		return 0
	}
	yes := len(args) == 1 && args[0] == "--yes"
	if len(args) > 0 && !yes {
		fmt.Fprintln(diagnostic, "Usage: haco reclaim --review [--yes]")
		return 2
	}
	selected, err := target(ctx)
	if err != nil || selected.Validate() != nil {
		fmt.Fprintln(diagnostic, "Managed Windows installation unavailable.")
		return 1
	}
	raw, err := invoke(ctx, selected, "status")
	if err != nil {
		fmt.Fprintln(diagnostic, "Saved reclamation result unavailable; nothing was reviewed.")
		return 1
	}
	result, err := parseReclamationStatus(raw)
	if err != nil {
		fmt.Fprintln(diagnostic, err)
		return 1
	}
	if result.State == "complete" {
		fmt.Fprintln(out, "Completed reclamation needs no review.")
		return 0
	}
	if result.State == "interrupted" {
		fmt.Fprintln(out, "Interrupted evidence is already retained. Run haco reclaim separately to start a new operation.")
		return 0
	}
	// Freeze target and operation before consent. Never select a newer operation.
	if _, err := fmt.Fprintf(out, "Review %s operation %s. Original evidence will be retained; an interrupted outcome remains unknown. This may reopen the enrolled WSL for identity checking. No reclamation or retry will start.\n", result.State, result.Operation); err != nil {
		return 1
	}
	if !yes {
		confirmed, err := confirmReclamation(ctx, in, out)
		if err != nil {
			fmt.Fprintln(diagnostic, "Confirmation unavailable; nothing was reviewed.")
			return 1
		}
		if !confirmed {
			fmt.Fprintln(out, "Canceled.")
			return 0
		}
	}
	if ctx.Err() != nil {
		fmt.Fprintln(diagnostic, "Review canceled before Windows invocation.")
		return 1
	}
	if err := review(ctx, selected, result.Operation, result.State); err != nil {
		fmt.Fprintln(diagnostic, "Review could not be confirmed. Inspect haco reclaim --status; a live or changed operation is not replaced.")
		return 1
	}
	if _, err := fmt.Fprintln(out, "Original evidence retained. No new operation was started. Run haco reclaim separately when ready."); err != nil {
		return 1
	}
	return 0
}
