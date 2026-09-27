package incus

import (
	"context"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"strings"
	"testing"
)

const fixtureCopyID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

func TestCacheCopyWaitRequiresExactTerminalReceipt(t *testing.T) {
	for _, mode := range []string{"success", "failed", "cancelled", "running", "foreign", "missing", "truncated", "malformed", "unknown-status"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
				calls++
				if name != "incus" || len(args) != 3 || args[1] != "--raw" || !strings.HasPrefix(args[2], "/1.0/operations/"+fixtureCopyID+"/wait?") {
					t.Fatal(name, args)
				}
				status, id := 200, fixtureCopyID
				switch mode {
				case "failed":
					status = 400
				case "cancelled":
					status = 401
				case "running":
					status = 103
					cancel()
				case "unknown-status":
					status = 999
				case "foreign":
					id = "ffffffff-bbbb-cccc-dddd-eeeeeeeeeeee"
				case "missing":
					return host.Result{Stdout: `{"type":"error","error_code":404}`}, nil
				case "malformed":
					return host.Result{Stdout: `{`}, nil
				}
				return host.Result{Stdout: fmt.Sprintf(`{"type":"sync","status_code":200,"metadata":{"id":%q,"status_code":%d}}`, id, status), StdoutTruncated: mode == "truncated"}, nil
			}}
			b := &PersistentResourceBackend{Runtime: New(runner)}
			_, err := b.waitCopyOperation(ctx, fixtureCopyID)
			if (err == nil) != (mode == "success" || mode == "failed" || mode == "cancelled") || calls != 1 {
				t.Fatal(err, calls)
			}
			before := calls
			if _, err := b.waitCopyOperation(context.Background(), "../foreign?x"); err == nil || calls != before {
				t.Fatal("unvalidated operation queried", err)
			}
		})
	}
}
func TestCacheCopyRecordsSubmissionBeforeWaiting(t *testing.T) {
	for _, mode := range []string{"ok", "record-failed", "foreign-url", "error-envelope"} {
		t.Run(mode, func(t *testing.T) {
			recorded, waits := false, 0
			runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				if len(args) > 3 && args[2] == "-X" {
					operation := "/1.0/operations/" + fixtureCopyID
					if mode == "foreign-url" {
						operation = "https://foreign/" + operation
					}
					if mode == "error-envelope" {
						return host.Result{Stdout: `{"type":"error","error_code":500}`}, nil
					}
					return host.Result{Stdout: fmt.Sprintf(`{"type":"async","status_code":100,"operation":%q,"metadata":{"id":%q}}`, operation, fixtureCopyID)}, nil
				}
				waits++
				if !recorded {
					t.Fatal("wait before durable receipt")
				}
				return host.Result{Stdout: fmt.Sprintf(`{"type":"sync","status_code":200,"metadata":{"id":%q,"status_code":200}}`, fixtureCopyID)}, nil
			}}
			b := &PersistentResourceBackend{Runtime: New(runner)}
			err := b.submitTrackedCopy(context.Background(), "pool", []byte(`{}`), func(id string) error {
				if id != fixtureCopyID {
					t.Fatal(id)
				}
				if mode == "record-failed" {
					return core.ErrRecoveryRequired
				}
				recorded = true
				return nil
			})
			if (err == nil) != (mode == "ok") || (waits == 1) != (mode == "ok") {
				t.Fatal(err, waits)
			}
		})
	}
}
