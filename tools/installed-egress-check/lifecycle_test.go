package main

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	"testing"
	"time"
)

type lifecycleFixture struct {
	calls, fail int
	old         core.Environment
	badIdentity bool
}

func (f *lifecycleFixture) step() error {
	f.calls++
	if f.calls == f.fail {
		return errors.New("fixture failure")
	}
	return nil
}
func (f *lifecycleFixture) ExecEnvironment(context.Context, string, []string) (core.ExecutionResult, error) {
	return core.ExecutionResult{Stdout: "verified"}, f.step()
}
func (f *lifecycleFixture) StopEnvironment(context.Context, string) error   { return f.step() }
func (f *lifecycleFixture) StartEnvironment(context.Context, string) error  { return f.step() }
func (f *lifecycleFixture) DeleteEnvironment(context.Context, string) error { return f.step() }
func (f *lifecycleFixture) CreateEnvironment(_ context.Context, r controlapi.EnvironmentCreateRequest) (core.Environment, error) {
	if err := f.step(); err != nil {
		return core.Environment{}, err
	}
	if r.Name != f.old.Name || r.WorkspacePath != f.old.Workspace.Path || r.AccessMode != f.old.AccessMode {
		return core.Environment{}, errors.New("recreation scope lost")
	}
	result := f.old
	result.CreatedAt = result.CreatedAt.Add(time.Second)
	if f.badIdentity {
		result.Workspace.Path = "/other"
	}
	return result, nil
}
func TestLifecycleAcceptanceStopsAtEachFailedPhase(t *testing.T) {
	old := core.Environment{Name: "fixture", Workspace: core.Workspace{ID: "workspace", Path: "/fixture"}, AccessMode: core.WorkspaceReadWrite, CreatedAt: time.Now()}
	for failure := 0; failure <= 8; failure++ {
		f := &lifecycleFixture{old: old, fail: failure}
		err := checkRetainedWorkspace(context.Background(), f, old)
		if failure == 0 && (err != nil || f.calls != 8) {
			t.Fatal(err, f.calls)
		}
		if failure != 0 && (err == nil || f.calls != failure) {
			t.Fatal("failure accepted or execution continued", failure, err, f.calls)
		}
	}
	f := &lifecycleFixture{old: old, badIdentity: true}
	if err := checkRetainedWorkspace(context.Background(), f, old); err == nil || f.calls != 7 {
		t.Fatal("replacement identity accepted", err, f.calls)
	}
}
