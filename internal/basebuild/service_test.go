package basebuild

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"reflect"
	"strings"
	"testing"
)

type fakeEnv struct {
	t      *testing.T
	calls  []string
	work   core.Workspace
	script string
	fail   string
	cancel context.CancelFunc
}

func (f *fakeEnv) Create(_ context.Context, s core.EnvironmentSpec) (core.Environment, error) {
	f.calls = append(f.calls, "create")
	if s.TemporaryWorkspace == nil || !core.ValidTemporaryWorkspace(*s.TemporaryWorkspace) || !s.SkipDefaultResource || s.WorkspacePath != "" {
		f.t.Fatal("not isolated", s)
	}
	f.work = *s.TemporaryWorkspace
	if f.fail == "create" {
		return core.Environment{}, core.ErrRecoveryRequired
	}
	return core.Environment{Name: s.Name, Workspace: f.work}, nil
}
func (f *fakeEnv) ExecForWorkspace(_ context.Context, _ string, id core.WorkspaceID, r core.ExecutionRequest) (core.ExecutionResult, error) {
	if id != f.work.ID {
		f.t.Fatal("workspace drift")
	}
	step := "clean"
	if r.Stdin != nil {
		step = "script"
		if string(r.Stdin) != f.script || !reflect.DeepEqual(r.Argv, []string{"/bin/sh", "-eu", "-s"}) {
			f.t.Fatal("script not on stdin")
		}
	}
	f.calls = append(f.calls, step)
	if f.fail == step {
		if f.cancel != nil {
			f.cancel()
		}
		return core.ExecutionResult{ExitCode: 9}, errors.New("private guest output")
	}
	return core.ExecutionResult{}, nil
}
func (f *fakeEnv) StopForWorkspace(_ context.Context, _ string, id core.WorkspaceID) error {
	f.calls = append(f.calls, "stop")
	if id != f.work.ID {
		f.t.Fatal("wrong stop")
	}
	if f.fail == "stop" {
		return core.ErrRuntimeUnavailable
	}
	return nil
}
func (f *fakeEnv) PublishTemporaryBase(_ context.Context, _ string, w core.Workspace, n core.BaseName) (core.BaseInfo, error) {
	f.calls = append(f.calls, "publish")
	if w != f.work {
		f.t.Fatal("wrong publisher")
	}
	if f.fail == "publish" {
		return core.BaseInfo{Name: n}, core.ErrRecoveryRequired
	}
	return core.BaseInfo{Name: n, Revision: core.BaseRevision("sha256:" + strings.Repeat("a", 64))}, nil
}
func (f *fakeEnv) DeleteTemporary(ctx context.Context, _ string, w core.Workspace) error {
	f.calls = append(f.calls, "delete")
	if w != f.work || ctx.Err() != nil {
		f.t.Fatal("unsafe cleanup")
	}
	if _, ok := ctx.Deadline(); !ok {
		f.t.Fatal("unbounded cleanup")
	}
	if f.fail == "delete" {
		return core.ErrRecoveryRequired
	}
	return nil
}
func TestBuildPreservesBoundariesAndFailureEvidence(t *testing.T) {
	for _, failure := range []string{"", "create", "script", "clean", "stop", "publish", "delete"} {
		t.Run(failure, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			f := &fakeEnv{t: t, fail: failure, script: "echo example", cancel: cancel}
			got, err := (&Service{Environments: f}).Build(ctx, Definition{Name: "my-tools", Run: f.script})
			if (err != nil) != (failure != "") {
				t.Fatal(got, err)
			}
			if failure == "" && (!reflect.DeepEqual(f.calls, []string{"create", "script", "clean", "stop", "publish", "delete"}) || got.Builder != "" || got.State != "ready") {
				t.Fatal(f.calls, got)
			}
			if failure == "publish" && (got.Builder == "" || f.calls[len(f.calls)-1] != "publish") {
				t.Fatal("uncertain native operation lost", f.calls, got)
			}
			if failure == "delete" && (got.Builder == "" || got.Base.Revision == "") {
				t.Fatal("lost published image or cleanup identity", got)
			}
			if failure == "create" && !reflect.DeepEqual(f.calls, []string{"create"}) {
				t.Fatal("guessed cleanup", f.calls)
			}
		})
	}
}
func TestDefinitionRejectsAuthorityShapingInput(t *testing.T) {
	for _, d := range []Definition{{Name: "../x", Run: "true"}, {Name: "--public", Run: "true"}, {Name: "haco/x", Run: "true"}, {Name: "x", Run: ""}, {Name: "x", From: "x", Run: "true"}, {Name: "x", Run: strings.Repeat("a", MaxScriptBytes+1)}} {
		if d.Validate() == nil {
			t.Fatal(d)
		}
	}
}
