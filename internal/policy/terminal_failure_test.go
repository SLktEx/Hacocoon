package capability

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type boundedDisplay struct {
	bytes.Buffer
	remaining int
}

func (w *boundedDisplay) Write(p []byte) (int, error) {
	n := len(p)
	if n > w.remaining {
		n = w.remaining
	}
	w.remaining -= n
	_, _ = w.Buffer.Write(p[:n])
	if n != len(p) {
		return n, io.ErrClosedPipe
	}
	return n, nil
}

// A yes/save answer never authorizes an operation whose security prompt was
// only partly displayed, including the second question for a saved ask choice.
func TestIncompleteApprovalDisplayCannotGrantOrSaveAuthority(t *testing.T) {
	request := core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target", Environment: "dev", EnvironmentInstance: "env-11111111111111111111111111111111", Attributes: map[string]string{"revision": "exact-revision", "target": "exact-target"}}
	scope := request
	scope.Attributes = map[string]string{"revision": "*", "target": "exact-target"}
	review := core.ApprovalRequest{CapabilityRequest: request, SavedScope: &scope, Reason: "explicit security decision"}
	for _, answer := range []string{"1\n", "5\nyes\n"} {
		var complete bytes.Buffer
		decision, err := NewStdioApproval(strings.NewReader(answer), &complete).Decide(context.Background(), review)
		if err != nil || !decision.Approved || decision.Save == "" {
			t.Fatal("complete visible prompt rejected valid answer", decision, err)
		}
		for limit := 0; limit < complete.Len(); limit++ {
			display := &boundedDisplay{remaining: limit}
			decision, err := NewStdioApproval(strings.NewReader(answer), display).Decide(context.Background(), review)
			if !errors.Is(err, io.ErrClosedPipe) || decision.Approved || decision.Save != "" {
				t.Fatalf("incomplete prompt at byte %d granted authority: %+v, %v", limit, decision, err)
			}
			if !bytes.Equal(display.Bytes(), complete.Bytes()[:limit]) {
				t.Fatalf("partial display changed review data at byte %d", limit)
			}
		}
	}
}

type approvalReaderFunc func([]byte) (int, error)

func (f approvalReaderFunc) Read(p []byte) (int, error) { return f(p) }

func TestUnavailableOrCancelledApprovalInputCannotGrantAuthority(t *testing.T) {
	for _, terminal := range []*StdioApproval{nil, NewStdioApproval(nil, io.Discard), NewStdioApproval(strings.NewReader("yes\n"), nil)} {
		if decision, err := terminal.Decide(context.Background(), core.ApprovalRequest{}); err == nil || decision.Approved || decision.Save != "" {
			t.Fatal("unavailable terminal granted authority", decision, err)
		}
	}
	for _, failure := range []string{"read-error", "cancel-before", "cancel-during", "oversized", "unknown-save"} {
		t.Run(failure, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var input io.Reader = strings.NewReader("yes\n")
			want := error(core.ErrInvalidArgument)
			switch failure {
			case "read-error":
				reader, writer := io.Pipe()
				t.Cleanup(func() { _ = reader.Close() })
				if err := writer.CloseWithError(io.ErrUnexpectedEOF); err != nil {
					t.Fatal(err)
				}
				input, want = reader, io.ErrUnexpectedEOF
			case "cancel-before":
				cancel()
				want = context.Canceled
			case "cancel-during":
				input = approvalReaderFunc(func(p []byte) (int, error) {
					cancel()
					return copy(p, "yes\n"), nil
				})
				want = context.Canceled
			case "oversized":
				input = strings.NewReader("yes" + strings.Repeat(" ", 128) + "\n")
			case "unknown-save":
				// Environment-scoped saving requires a verified creation identity.
				input = strings.NewReader("1\n")
			}
			var output bytes.Buffer
			decision, err := NewStdioApproval(input, &output).Decide(ctx, core.ApprovalRequest{CapabilityRequest: core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target"}})
			if !errors.Is(err, want) || decision.Approved || decision.Save != "" {
				t.Fatal("input failure granted authority", decision, err)
			}
			if failure == "cancel-before" && output.Len() != 0 {
				t.Fatal("cancelled request prompted for new authority")
			}
		})
	}
}
