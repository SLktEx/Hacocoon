package incus

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestFailedCreationCleanupRetainsOriginalFailureAndRequiresPositiveAbsence(t *testing.T) {
	for _, outcome := range []string{"deleted", "absent", "ambiguous", "timeout"} {
		t.Run(outcome, func(t *testing.T) {
			runtime := New(&fakeRunner{})
			runtime.cleanupTimeout = 20 * time.Millisecond
			parent, cancel := context.WithCancel(context.Background())
			cancel()
			cause := errors.New("Workspace mount refused")
			retained := true
			started := time.Now()
			_, err := runtime.cleanupFailedEnvironment(parent, "haco-owned", cause, func(ctx context.Context, ref string) error {
				if ctx.Err() != nil || ref != "haco-owned" {
					t.Fatal("cleanup lost its bounded independent context or target", ref, ctx.Err())
				}
				switch outcome {
				case "deleted":
					retained = false
					return nil
				case "absent":
					retained = false
					return core.ErrNotFound
				case "timeout":
					<-ctx.Done()
					return ctx.Err()
				default:
					return core.ErrRuntimeUnavailable
				}
			})
			if !errors.Is(err, cause) || errors.Is(err, core.ErrRecoveryRequired) != retained {
				t.Fatal("cleanup hid cause or claimed absence", retained, err)
			}
			if outcome == "timeout" && (!errors.Is(err, context.DeadlineExceeded) || time.Since(started) > time.Second) {
				t.Fatal("cleanup deadline was lost", err)
			}
		})
	}
}

func TestExistingIncusProjectIsReused(t *testing.T) {
	runner := &fakeRunner{}
	if err := New(runner).ensureProject(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 1 {
		t.Fatal("existing project was mutated", runner.calls)
	}
	assertRunnerCall(t, runner.calls[0], "incus", "project", "show", defaultProject)
}
