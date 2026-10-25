package reclaimnative

import (
	"context"
	"errors"
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
