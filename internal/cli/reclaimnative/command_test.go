package reclaimnative

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/cli/ui"
)

const registrationID = "{11111111-2222-3333-4444-555555555555}"
const operationID = "{abcdefab-2222-3333-4444-555555555555}"
const prepared = `{"operation":"` + operationID + `","state":"pending"}`
const launched = "Dispatched Windows worker 42; inspect the prepared operation for completion.\n"

type fakeHelper struct {
	calls                           *[]string
	prepareRaw, launchRaw           []byte
	prepareErr, launchErr, closeErr error
	afterPrepare, afterLaunch       func()
}

func (h *fakeHelper) prepare(_ context.Context, r string) ([]byte, error) {
	*h.calls = append(*h.calls, "prepare:"+r)
	if h.afterPrepare != nil {
		h.afterPrepare()
	}
	return h.prepareRaw, h.prepareErr
}
func (h *fakeHelper) launch(_ context.Context, r, o string) ([]byte, error) {
	*h.calls = append(*h.calls, "launch:"+r+":"+o)
	if h.afterLaunch != nil {
		h.afterLaunch()
	}
	return h.launchRaw, h.launchErr
}
func (h *fakeHelper) close() error { *h.calls = append(*h.calls, "close"); return h.closeErr }
func fixture(calls *[]string) (actions, *fakeHelper) {
	h := &fakeHelper{calls: calls, prepareRaw: []byte(prepared), launchRaw: []byte(launched)}
	a := actions{
		resolve: func(name string) (string, error) {
			*calls = append(*calls, "resolve:"+name)
			return registrationID, nil
		},
		latest: func(context.Context, string) (savedStatus, error) {
			*calls = append(*calls, "status")
			return savedStatus{state: "none", document: []byte(`{"operation":"","state":"none"}`)}, nil
		},
		open: func(r string) (helper, error) { *calls = append(*calls, "open:"+r); return h, nil }, confirm: confirmFixture,
	}
	return a, h
}
func runFixture(ctx context.Context, args []string, input string, a actions) (int, string, string) {
	var out, diagnostic bytes.Buffer
	code := run(ctx, args, strings.NewReader(input), &out, &diagnostic, cliui.English, a)
	return code, out.String(), diagnostic.String()
}

func TestFreshCommandConfirmsAndLaunchesOnlyReturnedIdentity(t *testing.T) {
	for _, yes := range []bool{false, true} {
		t.Run(map[bool]string{false: "interactive", true: "yes"}[yes], func(t *testing.T) {
			var calls []string
			a, _ := fixture(&calls)
			args := []string{"--distribution", "Hacocoon"}
			if yes {
				args = append(args, "--yes")
			}
			code, out, diag := runFixture(context.Background(), args, "yes\n", a)
			want := []string{"resolve:Hacocoon", "open:" + registrationID, "prepare:" + registrationID, "launch:" + registrationID + ":" + operationID, "close"}
			if code != 0 || !reflect.DeepEqual(calls, want) || diag != "" || !strings.Contains(out, "not yet confirmed") {
				t.Fatalf("code=%d calls=%v out=%q diagnostic=%q", code, calls, out, diag)
			}
		})
	}
}
func TestInvalidSyntaxNeverResolvesOrInvokes(t *testing.T) {
	for _, args := range [][]string{nil, {"--yes"}, {"--distribution"}, {"--distribution", "Hacocoon", "--status", "--yes"}, {"--distribution", "Hacocoon", "--yes", "--yes"}, {"--distribution", "Hacocoon", "--review"}, {"--distribution", "Hacocoon", "--operation", operationID}, {"--distribution", "Hacocoon", "--distribution", "Other"}, {"--distribution", "--yes"}, {"--distribution", "a/b"}, {"--distribution", "a\n"}, {"--distribution", strings.Repeat("x", 65)}, {"_launch", registrationID, operationID}} {
		var calls []string
		a, _ := fixture(&calls)
		code, _, _ := runFixture(context.Background(), args, "yes\n", a)
		if code != 2 || len(calls) != 0 {
			t.Fatalf("args=%q code=%d calls=%v", args, code, calls)
		}
	}
}
func TestDeclinedOrMissingConfirmationDoesNotPrepare(t *testing.T) {
	for _, input := range []string{"no\n", "\n", "yes", strings.Repeat("y", 128) + "\n"} {
		var calls []string
		a, _ := fixture(&calls)
		code, _, _ := runFixture(context.Background(), []string{"--distribution", "Hacocoon"}, input, a)
		if code == 2 || !reflect.DeepEqual(calls, []string{"resolve:Hacocoon"}) {
			t.Fatalf("input=%q code=%d calls=%v", input, code, calls)
		}
	}
}
func TestNativeStatusNeverOpensHelperOrConfirms(t *testing.T) {
	for _, state := range []string{"none", "pending", "failed", "interrupted", "complete"} {
		var calls []string
		a, _ := fixture(&calls)
		a.latest = func(context.Context, string) (savedStatus, error) {
			calls = append(calls, "status")
			id := operationID
			if state == "none" {
				id = ""
			}
			return savedStatus{operation: id, state: state, document: []byte(`{"operation":"` + id + `","state":"` + state + `"}`)}, nil
		}
		a.confirm = func(context.Context, io.Reader, io.Writer, cliui.Language) (bool, error) {
			t.Fatal("status prompted")
			return false, nil
		}
		code, _, _ := runFixture(context.Background(), []string{"--distribution", "Hacocoon", "--status"}, "", a)
		want := 0
		if state == "failed" || state == "interrupted" {
			want = 1
		}
		if code != want || !reflect.DeepEqual(calls, []string{"resolve:Hacocoon", "status"}) {
			t.Fatalf("state=%s code=%d calls=%v", state, code, calls)
		}
	}
}
func TestSelectionFailureCannotFallBackToLatestOrHelper(t *testing.T) {
	for _, status := range []bool{false, true} {
		var calls []string
		a, _ := fixture(&calls)
		a.resolve = func(string) (string, error) { return "", errors.New("changed enrollment") }
		args := []string{"--distribution", "Hacocoon", "--yes"}
		if status {
			args[2] = "--status"
		}
		code, _, _ := runFixture(context.Background(), args, "", a)
		if code != 1 || len(calls) != 0 {
			t.Fatal(code, calls)
		}
	}
}
func TestPreparationAndDispatchFailureNeverRetryOrReadStatus(t *testing.T) {
	for _, failure := range []string{"prepare-error", "prepare-empty", "prepare-foreign-fields", "prepare-truncated", "launch-error", "launch-empty", "launch-oversized", "close-error"} {
		t.Run(failure, func(t *testing.T) {
			var calls []string
			a, h := fixture(&calls)
			switch failure {
			case "prepare-error":
				h.prepareErr = errors.New("unknown")
			case "prepare-empty":
				h.prepareRaw = nil
			case "prepare-foreign-fields":
				h.prepareRaw = []byte(`{"operation":"` + operationID + `","state":"pending","linux_started":false}`)
			case "prepare-truncated":
				h.prepareRaw = []byte(prepared[:len(prepared)-1])
			case "launch-error":
				h.launchErr = errors.New("unknown")
			case "launch-empty":
				h.launchRaw = nil
			case "launch-oversized":
				h.launchRaw = bytes.Repeat([]byte("a"), receiptLimit+1)
			case "close-error":
				h.closeErr = errors.New("close")
			}
			code, _, diagnostic := runFixture(context.Background(), []string{"--distribution", "Hacocoon", "--yes"}, "", a)
			if code != 1 || !strings.Contains(diagnostic, "unconfirmed") {
				t.Fatal(code, diagnostic)
			}
			want := 4
			if strings.HasPrefix(failure, "launch") || failure == "close-error" {
				want = 5
			}
			if len(calls) != want || calls[len(calls)-1] != "close" {
				t.Fatal(calls)
			}
		})
	}
}
func TestCancellationBeforeAndAfterPreparationNeverReplays(t *testing.T) {
	for _, where := range []string{"before", "confirmation", "after-prepare"} {
		t.Run(where, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var calls []string
			a, h := fixture(&calls)
			args := []string{"--distribution", "Hacocoon", "--yes"}
			switch where {
			case "before":
				cancel()
			case "confirmation":
				args = args[:2]
				a.confirm = func(context.Context, io.Reader, io.Writer, cliui.Language) (bool, error) { cancel(); return true, nil }
			case "after-prepare":
				h.afterPrepare = cancel
			}
			code, _, _ := runFixture(ctx, args, "yes\n", a)
			if code != 1 {
				t.Fatal(code)
			}
			for _, call := range calls {
				if strings.HasPrefix(call, "launch:") || call == "status" {
					t.Fatal(calls)
				}
			}
			if where == "before" && len(calls) != 0 {
				t.Fatal(calls)
			}
		})
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("output unavailable") }
func TestOutputFailureBeforeConfirmationPreventsPreparation(t *testing.T) {
	var calls []string
	a, _ := fixture(&calls)
	code := run(context.Background(), []string{"--distribution", "Hacocoon", "--yes"}, strings.NewReader(""), failingWriter{}, io.Discard, cliui.English, a)
	if code != 1 || !reflect.DeepEqual(calls, []string{"resolve:Hacocoon"}) {
		t.Fatal(code, calls)
	}
}

// Only tests provide a finite in-memory input stream. Production accepts the
// console's nonblocking event API, never an arbitrary blocking io.Reader.
func confirmFixture(ctx context.Context, in io.Reader, out io.Writer, language cliui.Language) (bool, error) {
	reader := bufio.NewReader(in)
	return confirmEvents(ctx, out, language, func() (confirmationKey, bool, error) {
		c, err := reader.ReadByte()
		if err != nil {
			return confirmationKey{}, false, err
		}
		if c == '\n' {
			c = '\r'
		}
		return confirmationKey{character: uint16(c), down: true, repeat: 1}, true, nil
	})
}

func TestHelpNeverReadsNativeState(t *testing.T) {
	for _, language := range []cliui.Language{cliui.English, cliui.Japanese} {
		var calls []string
		a, _ := fixture(&calls)
		var out bytes.Buffer
		if code := run(context.Background(), []string{"--help"}, nil, &out, io.Discard, language, a); code != 0 || len(calls) != 0 || !strings.Contains(out.String(), usage) {
			t.Fatal(code, calls, out.String())
		}
	}
}
func TestReadOnlyStatusFailureNeverFallsBack(t *testing.T) {
	for _, failure := range []string{"read", "unknown-state", "bad-id", "bad-document", "oversized"} {
		var calls []string
		a, _ := fixture(&calls)
		a.latest = func(context.Context, string) (savedStatus, error) {
			calls = append(calls, "status")
			s := savedStatus{state: "pending", operation: operationID, document: []byte(prepared)}
			switch failure {
			case "read":
				return s, errors.New("record read failed")
			case "unknown-state":
				s.state = "success"
			case "bad-id":
				s.operation = "old"
			case "bad-document":
				s.document = []byte("{")
			case "oversized":
				s.document = bytes.Repeat([]byte(" "), 16385)
			}
			return s, nil
		}
		code, _, _ := runFixture(context.Background(), []string{"--distribution", "Hacocoon", "--status"}, "", a)
		if code != 1 || !reflect.DeepEqual(calls, []string{"resolve:Hacocoon", "status"}) {
			t.Fatal(failure, code, calls)
		}
	}
}
func TestCancellationAfterLaunchKeepsUncertaintyWithoutAnotherAction(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls []string
	a, h := fixture(&calls)
	h.afterLaunch = cancel
	code, out, diagnostic := runFixture(ctx, []string{"--distribution", "Hacocoon", "--yes"}, "", a)
	if code != 1 || strings.Contains(out, "Worker dispatched") || !strings.Contains(diagnostic, "unconfirmed") || len(calls) != 5 || calls[4] != "close" {
		t.Fatal(code, calls, out, diagnostic)
	}
}
func TestFreshInvocationDoesNotReuseAnyEarlierReceipt(t *testing.T) {
	var calls []string
	a, h := fixture(&calls)
	if code, _, _ := runFixture(context.Background(), []string{"--distribution", "Hacocoon", "--yes"}, "", a); code != 0 {
		t.Fatal(code)
	}
	h.prepareErr = errors.New("pending existing operation")
	calls = nil
	if code, _, _ := runFixture(context.Background(), []string{"--distribution", "Hacocoon", "--yes"}, "", a); code != 1 || len(calls) != 4 {
		t.Fatal(code, calls)
	}
	for _, call := range calls {
		if strings.HasPrefix(call, "launch:") || call == "status" {
			t.Fatal(calls)
		}
	}
}
func TestHelpOutputFailureIsNonzeroWithoutNativeReads(t *testing.T) {
	var calls []string
	a, _ := fixture(&calls)
	if code := run(context.Background(), []string{"--help"}, nil, failingWriter{}, io.Discard, cliui.English, a); code != 1 || len(calls) != 0 {
		t.Fatal(code, calls)
	}
}
