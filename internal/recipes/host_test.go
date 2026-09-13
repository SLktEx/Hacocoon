//go:build linux

package recipes

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
)

const testHostUUID = "12345678-1234-1234-1234-123456789abc"

func TestHostRecipeIncarnationExplicitReplayAndOutput(t *testing.T) {
	root := privateDir(t)
	instance := testHostUUID
	calls := 0
	s := &HostService{Root: root, Identity: func(context.Context) (string, error) { return instance, nil }}
	s.Execute = func(_ context.Context, script []byte) (host.Result, error) {
		calls++
		if string(script) != "echo hello\n" {
			t.Fatalf("normalization: %q", script)
		}
		b, err := os.ReadFile(filepath.Join(root, "result.json"))
		var pending HostResult
		if err != nil || json.Unmarshal(b, &pending) != nil || pending.State != "running" {
			t.Fatal("execution before durable intent")
		}
		return host.Result{Stdout: "private output", Stderr: "private diagnostic"}, nil
	}
	ctx := context.Background()
	if err := s.Apply(ctx, Update{}); err != nil || calls != 0 {
		t.Fatal("unconfigured fresh setup", err)
	}
	script := "\ufeffecho hello\r\n"
	if err := s.Apply(ctx, Update{Script: &script}); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err := s.Apply(ctx, Update{}); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatal("ordinary setup replayed", calls)
	}
	instance = "abcdefab-1234-1234-1234-123456789abc"
	if err := s.Apply(ctx, Update{}); err != nil || calls != 2 {
		t.Fatal("recreated Host not initialized", err, calls)
	}
	if err := s.Apply(ctx, Update{Reapply: true}); err != nil || calls != 3 {
		t.Fatal("explicit replay", err)
	}
	var result HostResult
	ctx = ObserveHostResult(ctx, func(r HostResult) { result = r })
	if err := s.Apply(ctx, Update{ResultOnly: true}); err != nil || calls != 3 || result.Execution.Stdout != "private output" || result.Execution.Stderr != "private diagnostic" || result.Instance != instance {
		t.Fatal("lost private receipt", err, result)
	}
	if err := s.Apply(ctx, Update{Clear: true}); err != nil {
		t.Fatal(err)
	}
	instance = testHostUUID
	if err := s.Apply(ctx, Update{}); err != nil || calls != 3 {
		t.Fatal("cleared script replayed", err)
	}
}

func TestHostFailureAndUnknownCompletionNeverReplayAutomatically(t *testing.T) {
	for _, state := range []string{"failed", "running"} {
		t.Run(state, func(t *testing.T) {
			root := privateDir(t)
			calls := 0
			s := &HostService{Root: root, Identity: func(context.Context) (string, error) { return testHostUUID, nil }, Execute: func(context.Context, []byte) (host.Result, error) {
				calls++
				return host.Result{ExitCode: 23, Stderr: "SECRET"}, errors.New("SECRET")
			}}
			script := "exit 23"
			if err := s.Apply(context.Background(), Update{Script: &script}); !errors.Is(err, ErrExecutionFailed) || strings.Contains(err.Error(), "SECRET") {
				t.Fatal(err)
			}
			if state == "running" {
				files, err := openStore(root)
				if err != nil {
					t.Fatal(err)
				}
				b, _ := files.readFile("result.json")
				var result HostResult
				_ = json.Unmarshal(b, &result)
				result.State, result.Execution = "running", host.Result{ExitCode: -1}
				b, _ = json.Marshal(result)
				if err := files.saveFile("result.json", b); err != nil {
					t.Fatal(err)
				}
				files.close()
			}
			if err := s.Apply(context.Background(), Update{}); !errors.Is(err, ErrExecutionFailed) || calls != 1 {
				t.Fatal("automatic retry", err, calls)
			}
			if err := s.Apply(context.Background(), Update{ResultOnly: true}); err != nil || calls != 1 {
				t.Fatal("inspection executed", err)
			}
			s.Execute = func(context.Context, []byte) (host.Result, error) { calls++; return host.Result{}, nil }
			if err := s.Apply(context.Background(), Update{Reapply: true}); err != nil || calls != 2 {
				t.Fatal("deliberate retry failed", err)
			}
		})
	}
}

func TestHostReceiptRejectsUnsafeStorageAndConcurrentChanges(t *testing.T) {
	for _, kind := range []string{"symlink", "hardlink", "public", "fifo", "malformed"} {
		t.Run(kind, func(t *testing.T) {
			root := privateDir(t)
			outside := filepath.Join(t.TempDir(), "outside")
			if err := os.WriteFile(outside, []byte("private"), 0600); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "result.json")
			var err error
			switch kind {
			case "symlink":
				err = os.Symlink(outside, path)
			case "hardlink":
				err = os.Link(outside, path)
			case "public":
				err = os.WriteFile(path, []byte("{}"), 0644)
			case "fifo":
				err = makeFIFO(path)
			case "malformed":
				err = os.WriteFile(path, []byte("{}"), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			s := &HostService{Root: root, Identity: func(context.Context) (string, error) { t.Fatal("unsafe receipt accepted"); return "", nil }, Execute: func(context.Context, []byte) (host.Result, error) {
				t.Fatal("unsafe receipt executed")
				return host.Result{}, nil
			}}
			script := "true"
			for _, update := range []Update{{}, {Script: &script}, {Reapply: true}, {Clear: true}, {ResultOnly: true}} {
				if err := s.Apply(context.Background(), update); err == nil {
					t.Fatal("unsafe receipt accepted", update)
				}
			}
		})
	}
	root := privateDir(t)
	entered, release := make(chan struct{}), make(chan struct{})
	s := &HostService{Root: root, Identity: func(context.Context) (string, error) { return testHostUUID, nil }, Execute: func(context.Context, []byte) (host.Result, error) {
		close(entered)
		<-release
		return host.Result{}, nil
	}}
	script := "true"
	done := make(chan error, 1)
	go func() { done <- s.Apply(context.Background(), Update{Script: &script}) }()
	<-entered
	for _, update := range []Update{{}, {Clear: true}, {Script: &script}, {Reapply: true}} {
		if err := s.Apply(context.Background(), update); err == nil {
			t.Error("concurrent mutation accepted")
		}
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestHostCancellationAndReplacementFailClosed(t *testing.T) {
	for _, kind := range []string{"canceled", "replaced"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			instance := testHostUUID
			s := &HostService{Root: privateDir(t), Identity: func(context.Context) (string, error) { return instance, nil }, Execute: func(context.Context, []byte) (host.Result, error) {
				if kind == "canceled" {
					cancel()
				} else {
					instance = "abcdefab-1234-1234-1234-123456789abc"
				}
				return host.Result{}, nil
			}}
			script := "true"
			if err := s.Apply(ctx, Update{Script: &script}); !errors.Is(err, ErrExecutionFailed) {
				t.Fatal("ambiguous success", err)
			}
		})
	}
}
