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
	if _, err := fmt.Fprint(out, cliLanguage().Text("reclaim.text.confirm")); err != nil {
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
		_, _ = fmt.Fprintln(out, cliLanguage().Text("reclaim.text.review_help"))
		return 0
	}
	yes := len(args) == 1 && args[0] == "--yes"
	if len(args) > 0 && !yes {
		_, _ = fmt.Fprintln(diagnostic, cliLanguage().Text("reclaim.text.review_usage"))
		return 2
	}
	selected, err := target(ctx)
	if err != nil || selected.Validate() != nil {
		_, _ = fmt.Fprintln(diagnostic, cliLanguage().Text("reclaim.text.review_unavailable"))
		return 1
	}
	raw, err := invoke(ctx, selected, "status")
	if err != nil {
		_, _ = fmt.Fprintln(diagnostic, cliLanguage().Text("reclaim.text.review_read_failed"))
		return 1
	}
	result, err := parseReclamationStatus(raw)
	if err != nil {
		_, _ = fmt.Fprintln(diagnostic, err)
		return 1
	}
	if result.State == "none" {
		_, _ = fmt.Fprintln(out, cliLanguage().Text("reclaim.text.review_none"))
		return 0
	}
	if result.State == "complete" {
		_, _ = fmt.Fprintln(out, cliLanguage().Text("reclaim.text.review_complete"))
		return 0
	}
	if result.State == "interrupted" {
		_, _ = fmt.Fprintln(out, cliLanguage().Text("reclaim.text.review_interrupted"))
		return 0
	}
	// Freeze target and operation before consent. Never select a newer operation.
	if _, err := fmt.Fprintf(out, cliLanguage().Text("reclaim.text.review_warning"), reclamationValue(result.State), result.Operation); err != nil {
		return 1
	}
	if !yes {
		confirmed, err := confirmReclamation(ctx, in, out)
		if err != nil {
			_, _ = fmt.Fprintln(diagnostic, cliLanguage().Text("reclaim.text.review_confirmation_failed"))
			return 1
		}
		if !confirmed {
			_, _ = fmt.Fprintln(out, cliLanguage().Text("reclaim.text.canceled"))
			return 0
		}
	}
	if ctx.Err() != nil {
		_, _ = fmt.Fprintln(diagnostic, cliLanguage().Text("reclaim.text.review_canceled"))
		return 1
	}
	if err := review(ctx, selected, result.Operation, result.State); err != nil {
		_, _ = fmt.Fprintln(diagnostic, cliLanguage().Text("reclaim.text.review_unknown"))
		return 1
	}
	if _, err := fmt.Fprintln(out, cliLanguage().Text("reclaim.text.review_retained")); err != nil {
		return 1
	}
	return 0
}
