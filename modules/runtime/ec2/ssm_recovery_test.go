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

type ssmAmbiguityRunner struct {
	calls        []string
	sendOutput   string
	sendErr      error
	waitErr      error
	observations []ssmObservation
	observation  int
}

func (r *ssmAmbiguityRunner) Run(_ context.Context, name string, args ...string) (host.Result, error) {
	call := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, call)
	switch {
	case strings.Contains(call, " ssm send-command "):
		if r.sendErr != nil {
			return host.Result{ExitCode: 1}, r.sendErr
		}
		output := r.sendOutput
		if output == "" {
			output = "11111111-1111-1111-1111-111111111111\n"
		}
		return host.Result{Stdout: output}, nil
	case strings.Contains(call, " ssm wait command-executed "):
		if r.waitErr != nil {
			return host.Result{ExitCode: 1}, r.waitErr
		}
		return host.Result{}, nil
	case strings.Contains(call, " ssm get-command-invocation "):
		if r.observation >= len(r.observations) {
			return host.Result{ExitCode: 1}, errors.New("unexpected extra invocation observation")
		}
		observed := r.observations[r.observation]
		r.observation++
		return observed.result, observed.err
	default:
		return host.Result{}, errors.New("unexpected AWS call: " + call)
	}
}

func successfulSSMObservation() ssmObservation {
	return ssmObservation{result: host.Result{Stdout: `{"Status":"Success","ResponseCode":0,"StandardOutputContent":"done\n","StandardErrorContent":""}`}}
}

func countSSMCalls(calls []string, needle string) int {
	count := 0
	for _, call := range calls {
		if strings.Contains(call, needle) {
			count++
		}
	}
	return count
}

func TestRunSSMWaitFailureReconcilesSameCommand(t *testing.T) {
	runner := &ssmAmbiguityRunner{
		waitErr:      errors.New("waiter timed out"),
		observations: []ssmObservation{successfulSSMObservation()},
	}
	runtime := newTestRuntime(runner)
	result, err := runtime.runSSM(context.Background(), "i-0123456789abcdef0", "printf done")
	if err != nil || result.ExitCode != 0 || result.Stdout != "done\n" {
		t.Fatalf("result=%#v err=%v calls=%v", result, err, runner.calls)
	}
	if countSSMCalls(runner.calls, " ssm send-command ") != 1 || countSSMCalls(runner.calls, " ssm get-command-invocation ") != 1 {
		t.Fatalf("waiter failure caused unexpected command flow: %v", runner.calls)
	}
}

func TestRunSSMObservationFailureRetriesWithoutResend(t *testing.T) {
	runner := &ssmAmbiguityRunner{observations: []ssmObservation{
		{result: host.Result{ExitCode: 1}, err: errors.New("eventual consistency")},
		successfulSSMObservation(),
	}}
	runtime := newTestRuntime(runner)
	runtime.pollAttempts = 2
	result, err := runtime.runSSM(context.Background(), "i-0123456789abcdef0", "touch once")
	if err != nil || result.ExitCode != 0 {
		t.Fatalf("result=%#v err=%v calls=%v", result, err, runner.calls)
	}
	if countSSMCalls(runner.calls, " ssm send-command ") != 1 || countSSMCalls(runner.calls, " ssm get-command-invocation ") != 2 {
		t.Fatalf("observation retry resent command or used wrong identity: %v", runner.calls)
	}
}

func TestRunSSMPendingThenSuccessDoesNotResend(t *testing.T) {
	runner := &ssmAmbiguityRunner{observations: []ssmObservation{
		{result: host.Result{Stdout: `{"Status":"InProgress","ResponseCode":-1,"StandardOutputContent":"","StandardErrorContent":""}`}},
		successfulSSMObservation(),
	}}
	runtime := newTestRuntime(runner)
	runtime.pollAttempts = 2
	if _, err := runtime.runSSM(context.Background(), "i-0123456789abcdef0", "touch once"); err != nil {
		t.Fatalf("err=%v calls=%v", err, runner.calls)
	}
	if countSSMCalls(runner.calls, " ssm send-command ") != 1 {
		t.Fatalf("pending observation caused duplicate execution: %v", runner.calls)
	}
}

func TestRunSSMUnresolvedObservationRequiresRecovery(t *testing.T) {
	commandID := "11111111-1111-1111-1111-111111111111"
	runner := &ssmAmbiguityRunner{observations: []ssmObservation{
		{result: host.Result{ExitCode: 1}, err: errors.New("read failed 1")},
		{result: host.Result{ExitCode: 1}, err: errors.New("read failed 2")},
	}}
	runtime := newTestRuntime(runner)
	runtime.pollAttempts = 2
	_, err := runtime.runSSM(context.Background(), "i-0123456789abcdef0", "touch once")
	if !errors.Is(err, core.ErrRecoveryRequired) || !strings.Contains(err.Error(), commandID) {
		t.Fatalf("ambiguous command did not preserve recovery identity: %v", err)
	}
	if countSSMCalls(runner.calls, " ssm send-command ") != 1 || countSSMCalls(runner.calls, " ssm get-command-invocation ") != 2 {
		t.Fatalf("ambiguous observation caused duplicate execution: %v", runner.calls)
	}
}

func TestRunSSMSendFailureIsAmbiguousAndRequiresRecovery(t *testing.T) {
	runner := &ssmAmbiguityRunner{sendErr: errors.New("connection dropped after request")}
	runtime := newTestRuntime(runner)
	_, err := runtime.runSSM(context.Background(), "i-0123456789abcdef0", "touch once")
	if !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("send ambiguity must not be ordinary retryable failure: %v", err)
	}
	if countSSMCalls(runner.calls, " ssm send-command ") != 1 || countSSMCalls(runner.calls, " ssm get-command-invocation ") != 0 {
		t.Fatalf("unexpected retry after ambiguous send: %v", runner.calls)
	}
}

func TestRunSSMInvalidAcceptedCommandIDRequiresRecovery(t *testing.T) {
	runner := &ssmAmbiguityRunner{sendOutput: "bad\ncommand\n"}
	runtime := newTestRuntime(runner)
	_, err := runtime.runSSM(context.Background(), "i-0123456789abcdef0", "touch once")
	if !errors.Is(err, core.ErrRecoveryRequired) || !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("invalid accepted command id must fail closed: %v", err)
	}
	if countSSMCalls(runner.calls, " ssm send-command ") != 1 || countSSMCalls(runner.calls, " ssm get-command-invocation ") != 0 {
		t.Fatalf("invalid command id triggered unsafe follow-up: %v", runner.calls)
	}
}
