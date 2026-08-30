package ec2

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

type ambiguousCreateRunner struct {
	base         fakeRunner
	runAttempts  int
	clientTokens []string
}

func (r *ambiguousCreateRunner) Run(ctx context.Context, name string, args ...string) (host.Result, error) {
	call := name + " " + strings.Join(args, " ")
	if name == "aws" && strings.Contains(call, " ec2 run-instances ") {
		r.runAttempts++
		for i, arg := range args {
			if arg == "--client-token" && i+1 < len(args) {
				r.clientTokens = append(r.clientTokens, args[i+1])
			}
		}
		if r.runAttempts == 1 {
			r.base.calls = append(r.base.calls, call)
			return host.Result{ExitCode: 255}, errors.New("connection lost after RunInstances request")
		}
	}
	return r.base.Run(ctx, name, args...)
}

func TestCreateRetryAfterAmbiguousRunInstancesReusesPersistedClientToken(t *testing.T) {
	journalDir := filepath.Join(t.TempDir(), "create-journal")
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "host.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &ambiguousCreateRunner{}

	first, err := NewWithCreateJournal(runner, testConfig(), journalDir)
	if err != nil {
		t.Fatal(err)
	}
	first.pollAttempts = 1
	first.pollDelay = 0
	_, err = first.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: workspace, ReadOnly: true})
	if err == nil || errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("ambiguous idempotent create must be retryable, err=%v", err)
	}
	if runner.runAttempts != 1 || len(runner.clientTokens) != 1 {
		t.Fatalf("first attempt runAttempts=%d tokens=%v", runner.runAttempts, runner.clientTokens)
	}
	firstToken := runner.clientTokens[0]
	if !createClientTokenPattern.MatchString(firstToken) {
		t.Fatalf("invalid persisted client token %q", firstToken)
	}
	if strings.Contains(strings.Join(runner.base.calls, "\n"), "s3 rm s3://hacocoon-workspaces-example/tests/demo") {
		t.Fatalf("ambiguous RunInstances removed staging needed for retry: %v", runner.base.calls)
	}

	// Construct a new Runtime over the same durable journal to model a process
	// restart/caller retry after the first response was lost.
	second, err := NewWithCreateJournal(runner, testConfig(), journalDir)
	if err != nil {
		t.Fatal(err)
	}
	second.pollAttempts = 1
	second.pollDelay = 0
	created, err := second.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: workspace, ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if runner.runAttempts != 2 || len(runner.clientTokens) != 2 || runner.clientTokens[1] != firstToken {
		t.Fatalf("retry did not reuse client token: attempts=%d tokens=%v", runner.runAttempts, runner.clientTokens)
	}
	ref, err := decodeRef(created.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if ref.ClientToken != firstToken || ref.CreateOperation == "" {
		t.Fatalf("runtime ref lost create identity: %#v", ref)
	}
}

func TestCreateRetryRejectsParameterDriftBeforeAWSMutation(t *testing.T) {
	journalDir := filepath.Join(t.TempDir(), "create-journal")
	workspace := t.TempDir()
	runner := &ambiguousCreateRunner{}
	first, err := NewWithCreateJournal(runner, testConfig(), journalDir)
	if err != nil {
		t.Fatal(err)
	}
	first.pollAttempts = 1
	first.pollDelay = 0
	_, err = first.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: workspace, ReadOnly: true})
	if err == nil || errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("first create should be safely retryable, err=%v", err)
	}
	before := len(runner.base.calls)

	changed := testConfig()
	changed.InstanceType = "m7i.large"
	second, err := NewWithCreateJournal(runner, changed, journalDir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = second.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: workspace, ReadOnly: true})
	if !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("parameter drift err=%v", err)
	}
	newCalls := strings.Join(runner.base.calls[before:], "\n")
	if strings.Contains(newCalls, "tar -czf") || strings.Contains(newCalls, "s3 cp") || strings.Contains(newCalls, "ec2 run-instances") {
		t.Fatalf("parameter drift reached side effects:\n%s", newCalls)
	}
}

func TestDeleteCompletesCreateJournalSoFutureLifecycleGetsNewToken(t *testing.T) {
	journalDir := filepath.Join(t.TempDir(), "create-journal")
	workspace := t.TempDir()
	runner := &fakeRunner{}
	runtime, err := NewWithCreateJournal(runner, testConfig(), journalDir)
	if err != nil {
		t.Fatal(err)
	}
	runtime.pollAttempts = 1
	runtime.pollDelay = 0
	created, err := runtime.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: workspace, ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	ref, err := decodeRef(created.Ref)
	if err != nil {
		t.Fatal(err)
	}
	firstToken := ref.ClientToken

	runner.instanceState = "terminated"
	if err := runtime.DeleteEnvironment(context.Background(), created.Ref); err != nil {
		t.Fatal(err)
	}

	runner.instanceState = ""
	runner.calls = nil
	recreated, err := runtime.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: workspace, ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	recreatedRef, err := decodeRef(recreated.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if recreatedRef.ClientToken == firstToken {
		t.Fatalf("completed lifecycle reused old client token %q", firstToken)
	}
}
