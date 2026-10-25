package reclaimnative

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type observation struct {
	data             string
	eof, done        bool
	code             uint32
	readErr, exitErr error
}
type fakeProcess struct {
	steps      []observation
	at, closed int
	closeErr   error
	cancel     context.CancelFunc
}

func (p *fakeProcess) read(b []byte) (int, bool, error) {
	s := p.steps[p.at]
	if p.cancel != nil {
		p.cancel()
	}
	return copy(b, s.data), s.eof, s.readErr
}
func (p *fakeProcess) exit() (bool, uint32, error) {
	s := p.steps[p.at]
	if p.at < len(p.steps)-1 {
		p.at++
	}
	return s.done, s.code, s.exitErr
}
func (p *fakeProcess) close() error { p.closed++; return p.closeErr }
func TestObserveNeedsCompleteOutputAndExactSuccessfulExit(t *testing.T) {
	p := &fakeProcess{steps: []observation{{data: "a"}, {data: "b", done: true}, {eof: true, done: true}}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	raw, err := observe(ctx, p)
	if err != nil || string(raw) != "ab" || p.closed != 1 {
		t.Fatal(string(raw), err, p.closed)
	}
}
func TestObserveUncertaintyNeverTerminatesAndAlwaysClosesHandles(t *testing.T) {
	cases := map[string]*fakeProcess{
		"running-empty": {steps: []observation{{}}}, "root-exit-inherited-writer": {steps: []observation{{data: "receipt", done: true}, {done: true}}}, "eof-live-root": {steps: []observation{{eof: true}}}, "partial-exit-failure": {steps: []observation{{data: "plausible", done: true, code: 1}}}, "read-error": {steps: []observation{{readErr: errors.New("read")}}}, "exit-error": {steps: []observation{{exitErr: errors.New("exit")}}}, "close-error": {steps: []observation{{eof: true, done: true}}, closeErr: errors.New("close")},
	}
	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 35*time.Millisecond)
			defer cancel()
			_, err := observe(ctx, p)
			if err == nil || p.closed != 1 {
				t.Fatal(err, p.closed)
			}
		})
	}
}
func TestObserveCancellationAfterPossibleDispatchClosesOnlyObserver(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	p := &fakeProcess{steps: []observation{{data: "partial"}}, cancel: cancel}
	_, err := observe(ctx, p)
	if !errors.Is(err, context.Canceled) || p.closed != 1 {
		t.Fatal(err, p.closed)
	}
}
func TestObserveOversizedOutputRefuses(t *testing.T) {
	p := &fakeProcess{steps: []observation{{data: string(make([]byte, receiptLimit+1))}}}
	_, err := observe(context.Background(), p)
	if err == nil || p.closed != 1 {
		t.Fatal(err, p.closed)
	}
}
func TestEOFAloneIsNotAReceipt(t *testing.T) {
	p := &fakeProcess{steps: []observation{{eof: true, done: true}}}
	raw, err := observe(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodePrepared(raw); err == nil {
		t.Fatal("EOF proved preparation")
	}
	if _, err := decodeLaunch(raw); err == nil {
		t.Fatal("EOF proved dispatch")
	}
}

// The native fixture's wire diagnostics are deliberately independent of error
// strings. Keep these helpers portable so malformed receipts can be tested here
// without starting Windows processes or invoking any console API.
const fixtureReceiptLimit = 64

type fixtureStageError struct {
	stage string
	cause error
}

func (e *fixtureStageError) Error() string { return "synthetic fixture stage failed" }
func (e *fixtureStageError) Unwrap() error { return e.cause }

func allowedFixtureStage(stage string) bool {
	switch stage {
	case "console-abi", "console-api", "console-existing", "console-detach",
		"console-allocate", "console-input", "console-mode", "console-cancel",
		"console-write", "console-confirm", "console-key-up", "console-repeat",
		"console-backspace", "console-cleanup":
		return true
	default:
		return false
	}
}

func encodeFixtureResult(err error) string {
	if err == nil {
		return "OK\n"
	}
	var staged *fixtureStageError
	if errors.As(err, &staged) && allowedFixtureStage(staged.stage) {
		return "FAIL:" + staged.stage + "\n"
	}
	return "FAIL\n"
}

// Even an exact success receipt cannot overcome an unsuccessful/unobserved
// root exit. All returned messages are local constants or allowlisted stages.
func decodeFixtureResult(raw []byte, waitErr error) error {
	if len(raw) > fixtureReceiptLimit {
		return errors.New("invalid synthetic fixture receipt")
	}
	text := string(raw)
	if text == "OK\n" {
		if waitErr != nil {
			return errors.New("synthetic native scenario did not exit successfully")
		}
		return nil
	}
	if text == "FAIL\n" {
		return errors.New("synthetic native scenario failed")
	}
	if strings.HasPrefix(text, "FAIL:") && strings.HasSuffix(text, "\n") {
		stage := text[len("FAIL:") : len(text)-1]
		if allowedFixtureStage(stage) {
			return errors.New("synthetic native scenario failed at " + stage)
		}
	}
	return errors.New("invalid synthetic fixture receipt")
}

func TestFixtureDiagnosticsAreBoundedAndAllowlisted(t *testing.T) {
	secret := errors.New("must-not-appear: private child output or path")
	if got := encodeFixtureResult(nil); got != "OK\n" {
		t.Fatal("success encoding changed")
	}
	if got := encodeFixtureResult(secret); got != "FAIL\n" {
		t.Fatal("untyped error was exposed")
	}
	for _, stage := range []string{"console-abi", "console-api", "console-existing", "console-detach", "console-allocate", "console-input", "console-mode", "console-cancel", "console-write", "console-confirm", "console-key-up", "console-repeat", "console-backspace", "console-cleanup"} {
		wrapped := &fixtureStageError{stage: stage, cause: secret}
		if !errors.Is(wrapped, secret) {
			t.Fatal("stage lost underlying failure")
		}
		raw := encodeFixtureResult(wrapped)
		if len(raw) > fixtureReceiptLimit || raw != "FAIL:"+stage+"\n" {
			t.Fatal("stage encoding differs")
		}
		for _, waitErr := range []error{nil, secret} {
			err := decodeFixtureResult([]byte(raw), waitErr)
			if err == nil || err.Error() != "synthetic native scenario failed at "+stage {
				t.Fatal("stage failure was accepted or not allowlisted")
			}
		}
	}
	if got := encodeFixtureResult(&fixtureStageError{stage: secret.Error(), cause: secret}); got != "FAIL\n" {
		t.Fatal("unknown stage was exposed")
	}
	if err := decodeFixtureResult([]byte("OK\n"), nil); err != nil {
		t.Fatal("exact success was refused")
	}
	if err := decodeFixtureResult([]byte("OK\n"), secret); err == nil || strings.Contains(err.Error(), secret.Error()) {
		t.Fatal("failed root was accepted or exposed")
	}
	for _, raw := range []string{"", "OK", "OK\nextra", "OK\r\n", "FAIL", "FAIL:console-allocate", "FAIL:unknown\n", "FAIL:console-allocate\nOK\n", "FAIL:console-allocate\n\n", strings.Repeat("x", fixtureReceiptLimit+1), secret.Error()} {
		err := decodeFixtureResult([]byte(raw), nil)
		if err == nil || err.Error() != "invalid synthetic fixture receipt" {
			t.Fatal("malformed receipt was accepted or exposed")
		}
	}
}
