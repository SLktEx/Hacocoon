package incus

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

// This gate needs only empty stopped instances. It changes no shared network,
// default profile, security policy or image cache, and claims no guest acceptance.
func TestRealIncusObservationAndDeletion(t *testing.T) {
	if os.Getenv("HACO_E2E_INCUS_OBSERVATION") != "1" {
		t.Skip("set HACO_E2E_INCUS_OBSERVATION=1 on a dedicated Incus host")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	r := New(host.ExecRunner{})
	owner := fmt.Sprintf("haco-observation-%d", time.Now().UnixNano())
	r.project = owner
	pool := owner
	t.Logf("owned fixture project/pool: %s", owner)
	run := func(args ...string) {
		t.Helper()
		if _, err := r.runner.Run(ctx, "incus", args...); err != nil {
			t.Fatal(err)
		}
	}
	run("project", "create", owner, "--config", "features.profiles=false", "--config", "user.hacocoon.test-owner="+owner)
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 30*time.Second)
		defer done()
		marker, err := r.readIncusOutput(cleanup, "project", "get", owner, "user.hacocoon.test-owner")
		if err != nil || marker != owner+"\n" {
			t.Errorf("project ownership unresolved: %v", err)
			return
		}
		if _, err := r.runner.Run(cleanup, "incus", "project", "delete", owner); err != nil {
			t.Errorf("project retained: %v", err)
		}
	})
	run("storage", "create", pool, "dir", "user.hacocoon.test-owner="+owner)
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 30*time.Second)
		defer done()
		marker, err := r.readIncusOutput(cleanup, "storage", "get", pool, "user.hacocoon.test-owner")
		if err != nil || marker != owner+"\n" {
			t.Errorf("pool ownership unresolved: %v", err)
			return
		}
		if _, err := r.runner.Run(cleanup, "incus", "storage", "delete", pool); err != nil {
			t.Errorf("pool retained: %v", err)
		}
	})
	for _, ref := range []string{"haco-observe", "haco-observe-copy"} {
		run("init", ref, "--empty", "--no-profiles", "--project", owner, "--storage", pool, "--config", "user.hacocoon.test-owner="+owner)
		t.Cleanup(func() {
			cleanup, done := context.WithTimeout(context.Background(), 30*time.Second)
			defer done()
			exists, err := r.environmentExists(cleanup, ref)
			if err != nil {
				t.Errorf("instance observation failed: %v", err)
				return
			}
			if !exists {
				return
			}
			marker, err := r.readIncusOutput(cleanup, "config", "get", ref, "user.hacocoon.test-owner", "--project", owner)
			if err != nil || marker != owner+"\n" {
				t.Errorf("instance ownership unresolved: %v", err)
				return
			}
			if err := r.DeleteEnvironment(cleanup, ref); !core.EnvironmentDeletionComplete(err) {
				t.Errorf("instance retained: %v", err)
			}
		})
	}
	status, err := r.InspectEnvironment(ctx, "haco-observe")
	if err != nil || status.Absent || status.State != core.EnvironmentStopped {
		t.Fatalf("stopped observation: %#v %v", status, err)
	}
	if err := r.DeleteEnvironment(ctx, "haco-observe"); err != nil {
		t.Fatal(err)
	}
	status, err = r.InspectEnvironment(ctx, "haco-observe")
	if err != nil || !status.Absent || status.State != core.EnvironmentUnknown {
		t.Fatalf("absence observation: %#v %v", status, err)
	}
	if exists, err := r.environmentExists(ctx, "haco-observe-copy"); err != nil || !exists {
		t.Fatalf("prefix resource lost: %v %v", exists, err)
	}
	if err := r.DeleteEnvironment(ctx, "haco-observe"); !errors.Is(err, core.ErrNotFound) || !core.EnvironmentDeletionComplete(err) {
		t.Fatalf("repeat deletion: %v", err)
	}
}
