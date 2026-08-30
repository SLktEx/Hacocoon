package ec2

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestCreateRetryRejectsWorkspaceContentDriftBeforeSecondAWSMutation(t *testing.T) {
	journalDir := filepath.Join(t.TempDir(), "create-journal")
	workspace := t.TempDir()
	file := filepath.Join(workspace, "work.txt")
	if err := os.WriteFile(file, []byte("first\n"), 0o644); err != nil {
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
		t.Fatalf("first ambiguous create should be safely retryable, err=%v", err)
	}
	if runner.runAttempts != 1 {
		t.Fatalf("run attempts=%d", runner.runAttempts)
	}

	if err := os.WriteFile(file, []byte("changed-after-ambiguous-create\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	beforeCalls := len(runner.base.calls)
	second, err := NewWithCreateJournal(runner, testConfig(), journalDir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = second.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: workspace, ReadOnly: true})
	if !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("workspace drift err=%v, want recovery-required", err)
	}
	if runner.runAttempts != 1 {
		t.Fatalf("workspace drift reached RunInstances again: attempts=%d", runner.runAttempts)
	}
	newCalls := strings.Join(runner.base.calls[beforeCalls:], "\n")
	if strings.Contains(newCalls, " s3 cp ") || strings.Contains(newCalls, " ec2 run-instances ") {
		t.Fatalf("workspace drift reached AWS mutation:\n%s", newCalls)
	}
}

func TestWorkspaceDigestIgnoresMtimeOnlyChanges(t *testing.T) {
	workspace := t.TempDir()
	file := filepath.Join(workspace, "work.txt")
	if err := os.WriteFile(file, []byte("stable\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	before, err := digestWorkspace(context.Background(), workspace)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(file, info.ModTime().Add(123456789), info.ModTime().Add(123456789)); err != nil {
		t.Fatal(err)
	}
	after, err := digestWorkspace(context.Background(), workspace)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("mtime-only change altered workspace identity: before=%s after=%s", before, after)
	}
}
