package ec2

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

type ssmObservation struct {
	result host.Result
	err    error
}

type ssmSequenceRunner struct {
	calls        []string
	observations []ssmObservation
	sendErr      error
	onSend       func()
	sendCount    int
	getCount     int
}

func (r *ssmSequenceRunner) Run(_ context.Context, name string, args ...string) (host.Result, error) {
	call := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, call)
	switch {
	case strings.Contains(call, " ssm send-command "):
		r.sendCount++
		if r.onSend != nil {
			r.onSend()
		}
		if r.sendErr != nil {
			return host.Result{ExitCode: 255}, r.sendErr
		}
		return host.Result{Stdout: "11111111-1111-1111-1111-111111111111\n"}, nil
	case strings.Contains(call, " ssm get-command-invocation "):
		r.getCount++
		if len(r.observations) == 0 {
			return host.Result{ExitCode: 255, Stderr: "observation unavailable"}, errors.New("observation unavailable")
		}
		observation := r.observations[0]
		r.observations = r.observations[1:]
		return observation.result, observation.err
	default:
		return host.Result{}, nil
	}
}

func newSSMTestRuntime(runner host.Runner, attempts int) *Runtime {
	runtime := New(runner, testConfig())
	runtime.pollAttempts = attempts
	runtime.pollDelay = 0
	return runtime
}

func TestRunSSMReconcilesSameCommandAfterObservationFailure(t *testing.T) {
	runner := &ssmSequenceRunner{observations: []ssmObservation{
		{result: host.Result{ExitCode: 255, Stderr: "InvocationDoesNotExist"}, err: errors.New("eventual consistency")},
		{result: host.Result{Stdout: `{"Status":"Pending","ResponseCode":-1,"StandardOutputContent":"","StandardErrorContent":""}`}},
		{result: host.Result{Stdout: `{"Status":"Success","ResponseCode":0,"StandardOutputContent":"done\n","StandardErrorContent":""}`}},
	}}
	runtime := newSSMTestRuntime(runner, 3)

	result, err := runtime.runSSM(context.Background(), "i-0123456789abcdef0", "touch /tmp/once")
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 || result.Stdout != "done\n" {
		t.Fatalf("result=%#v", result)
	}
	if runner.sendCount != 1 || runner.getCount != 3 {
		t.Fatalf("send=%d get=%d calls=%v", runner.sendCount, runner.getCount, runner.calls)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, "get-command-invocation") && !strings.Contains(call, "--command-id 11111111-1111-1111-1111-111111111111") {
			t.Fatalf("observation changed command identity: %s", call)
		}
		if strings.Contains(call, " command-executed ") {
			t.Fatalf("waiter should not hide command observation ambiguity: %s", call)
		}
	}
}

func TestRunSSMObservationFailureReturnsNonRetryableAmbiguity(t *testing.T) {
	runner := &ssmSequenceRunner{observations: []ssmObservation{
		{result: host.Result{ExitCode: 255}, err: errors.New("network 1")},
		{result: host.Result{ExitCode: 255}, err: errors.New("network 2")},
		{result: host.Result{ExitCode: 255}, err: errors.New("network 3")},
	}}
	runtime := newSSMTestRuntime(runner, 3)

	_, err := runtime.runSSM(context.Background(), "i-0123456789abcdef0", "do-side-effect")
	if !errors.Is(err, core.ErrExecutionOutcomeUnknown) {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(err.Error(), "11111111-1111-1111-1111-111111111111") {
		t.Fatalf("recovery error lost command id: %v", err)
	}
	if runner.sendCount != 1 || runner.getCount != 3 {
		t.Fatalf("send=%d get=%d calls=%v", runner.sendCount, runner.getCount, runner.calls)
	}
}

func TestRunSSMNonTerminalTimeoutReturnsAmbiguousWithoutResend(t *testing.T) {
	runner := &ssmSequenceRunner{observations: []ssmObservation{
		{result: host.Result{Stdout: `{"Status":"Pending","ResponseCode":-1}`}},
		{result: host.Result{Stdout: `{"Status":"InProgress","ResponseCode":-1}`}},
	}}
	runtime := newSSMTestRuntime(runner, 2)

	_, err := runtime.runSSM(context.Background(), "i-0123456789abcdef0", "do-side-effect")
	if !errors.Is(err, core.ErrExecutionOutcomeUnknown) {
		t.Fatalf("err=%v", err)
	}
	if runner.sendCount != 1 {
		t.Fatalf("accepted command was resent: %v", runner.calls)
	}
}

func TestRunSSMCancelledCommandIsAmbiguousForSideEffects(t *testing.T) {
	runner := &ssmSequenceRunner{observations: []ssmObservation{{
		result: host.Result{Stdout: `{"Status":"Cancelled","ResponseCode":-1,"StandardOutputContent":"","StandardErrorContent":""}`},
	}}}
	runtime := newSSMTestRuntime(runner, 1)

	_, err := runtime.runSSM(context.Background(), "i-0123456789abcdef0", "do-side-effect")
	if !errors.Is(err, core.ErrExecutionOutcomeUnknown) {
		t.Fatalf("err=%v", err)
	}
	if runner.sendCount != 1 {
		t.Fatalf("accepted command was resent: %v", runner.calls)
	}
}

func TestRunSSMContextCancellationAfterAcceptanceIsAmbiguous(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	runner := &ssmSequenceRunner{}
	runner.onSend = cancel
	runtime := newSSMTestRuntime(runner, 2)

	_, err := runtime.runSSM(ctx, "i-0123456789abcdef0", "do-side-effect")
	if !errors.Is(err, core.ErrExecutionOutcomeUnknown) || !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	if runner.sendCount != 1 {
		t.Fatalf("accepted command was resent: %v", runner.calls)
	}
}

func TestRunSSMSendFailureIsNotExecutionAmbiguity(t *testing.T) {
	runner := &ssmSequenceRunner{sendErr: errors.New("send rejected")}
	runtime := newSSMTestRuntime(runner, 3)

	_, err := runtime.runSSM(context.Background(), "i-0123456789abcdef0", "do-side-effect")
	if err == nil || errors.Is(err, core.ErrExecutionOutcomeUnknown) {
		t.Fatalf("err=%v", err)
	}
	if runner.sendCount != 1 || runner.getCount != 0 {
		t.Fatalf("send=%d get=%d calls=%v", runner.sendCount, runner.getCount, runner.calls)
	}
}
