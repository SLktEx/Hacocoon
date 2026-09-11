package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/reclamation"
)

func TestReclaimReviewKeepsConfirmedOperationAndNeverLaunches(t *testing.T) {
	for _, tc := range []struct {
		name, state, answer string
		args                []string
		fail                bool
		reviews, code       int
	}{
		{name: "decline", state: "pending", answer: "n\n"},
		{name: "pending", state: "pending", answer: "yes\n", reviews: 1},
		{name: "failed", state: "failed", args: []string{"--yes"}, reviews: 1},
		{name: "busy or changed", state: "pending", answer: "y\n", fail: true, reviews: 1, code: 1},
		{name: "already reviewed", state: "interrupted"},
		{name: "invalid state", state: "other", code: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, diagnostic bytes.Buffer
			reads, targets, reviews := 0, 0, 0
			code := reclaimReviewCommand(context.Background(), tc.args, strings.NewReader(tc.answer), &out, &diagnostic,
				func(context.Context) (reclamation.WSLTarget, error) { targets++; return commandReclaimTarget, nil },
				func(_ context.Context, target reclamation.WSLTarget, mode string) ([]byte, error) {
					reads++
					if mode != "status" || target != commandReclaimTarget {
						t.Fatal("mutation/changed target")
					}
					return []byte(`{"operation":"` + commandReclaimOperation + `","state":"` + tc.state + `"}`), nil
				},
				func(_ context.Context, target reclamation.WSLTarget, operation, state string) error {
					reviews++
					if target != commandReclaimTarget || operation != commandReclaimOperation || state != tc.state {
						t.Fatal("consent changed")
					}
					if tc.fail {
						return errors.New("private token=hidden")
					}
					return nil
				})
			if code != tc.code || reviews != tc.reviews || reads != 1 || targets != 1 {
				t.Fatal(code, reviews, reads, targets, out.String(), diagnostic.String())
			}
			if strings.Contains(diagnostic.String(), "hidden") {
				t.Fatal("raw review error exposed")
			}
			if tc.reviews == 1 && !tc.fail && !strings.Contains(out.String(), "No new operation was started") {
				t.Fatal(out.String())
			}
		})
	}
}

func TestReclaimReviewRefusesArgumentsBeforeDiscovery(t *testing.T) {
	for _, args := range [][]string{{"foreign"}, {"--yes", "--yes"}, {"--status"}} {
		var out bytes.Buffer
		code := reclaimReviewCommand(context.Background(), args, strings.NewReader(""), &out, &out,
			func(context.Context) (reclamation.WSLTarget, error) {
				t.Fatal("invalid request discovered target")
				return commandReclaimTarget, nil
			}, nil, nil)
		if code != 2 {
			t.Fatal(code)
		}
	}
}

func TestReclamationConfirmationFailureIsNotConsent(t *testing.T) {
	confirmed, err := confirmReclamation(context.Background(), strings.NewReader("y\n"), failedReclaimWriter{})
	if confirmed || err == nil {
		t.Fatal("unreadable confirmation accepted")
	}
}
