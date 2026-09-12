package oci

import (
	"context"
	"errors"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/core"
	"strings"
	"testing"
)

type maintenanceContextKey struct{}
type scopedImageExecutor struct {
	fixture *managedImageFixture
	active  *bool
	t       *testing.T
}

func (s scopedImageExecutor) ExecForResource(ctx context.Context, name, generation string, resource core.PersistentResourceRef, req core.ExecutionRequest) (core.ExecutionResult, error) {
	if !*s.active || ctx.Value(maintenanceContextKey{}) != true {
		s.t.Fatal("image command escaped maintained context")
	}
	if !strings.Contains(strings.Join(req.Argv, " "), "--address /run/hacocoon-maintenance/containerd.sock --namespace default --snapshotter native") {
		s.t.Fatal("ordinary or arbitrary socket used")
	}
	return s.fixture.ExecForResource(ctx, name, generation, resource, req)
}
func TestDetachedImagesKeepOneOwnedEnvironmentPerOperation(t *testing.T) {
	for _, mode := range []string{"unused", "used", "owner-changed", "cleanup-failure", "busy", "wrong-binding", "malformed", "truncated"} {
		t.Run(mode, func(t *testing.T) {
			s, f := newManagedImageFixture("nerdctl")
			f.resource.WorkspaceID = "original-workspace"
			active, sessions := false, 0
			s.Environments = scopedImageExecutor{f, &active, t}
			s.Maintain = func(ctx context.Context, ref core.PersistentResourceRef, operation func(context.Context, core.Environment) error) error {
				sessions++
				if active || ref != f.resource.Ref() {
					t.Fatal("wrong maintenance reservation")
				}
				if mode == "busy" {
					return core.ErrStorageBusy
				}
				work, _ := core.NewTemporaryWorkspace()
				f.target.Environment = fmt.Sprintf("run-maintenance-%d", sessions)
				f.target.Instance = fmt.Sprintf("env-%032x", sessions)
				env := core.Environment{Name: f.target.Environment, Workspace: work, PersistentResource: ref}
				if mode == "wrong-binding" {
					env.PersistentResource.Owner = strings.Repeat("f", 32)
				}
				active = true
				err := operation(context.WithValue(ctx, maintenanceContextKey{}, true), env)
				active = false
				if mode == "cleanup-failure" {
					return core.ErrRecoveryRequired
				}
				return err
			}
			f.used = mode == "used"
			f.malformed = mode == "malformed"
			f.truncated = mode == "truncated"
			all, err := s.List(context.Background(), f.resource.ID, "nerdctl")
			if mode == "busy" || mode == "wrong-binding" || mode == "malformed" || mode == "truncated" || mode == "cleanup-failure" {
				if err == nil || active {
					t.Fatal("failed maintenance reported success", err)
				}
				if (mode == "busy" || mode == "wrong-binding") && len(f.calls) != 0 {
					t.Fatal("rejected lease executed commands")
				}
				return
			}
			if err != nil || sessions != 1 || !all.Target.Detached || all.Target.Environment != "" || all.Target.Instance != "" || all.Target.Store != f.resource.Ref() || len(all.Images) != 1 {
				t.Fatal("invalid detached inventory", all, err)
			}
			if mode == "owner-changed" {
				f.resource.Owner = strings.Repeat("f", 32)
			}
			err = s.Delete(context.Background(), all.Target, all.Images[0].ID)
			switch mode {
			case "owner-changed":
				if !errors.Is(err, core.ErrCapabilityStale) || sessions != 1 {
					t.Fatal("stale review borrowed replacement", err)
				}
			case "used":
				if !errors.Is(err, core.ErrStorageBusy) || f.deleted || sessions != 2 {
					t.Fatal("referenced image removed", err)
				}
			default:
				if err != nil || !f.deleted || sessions != 2 {
					t.Fatal("image deletion not confirmed", err)
				}
			}
			if active || f.resource.WorkspaceID != "original-workspace" {
				t.Fatal("borrowed Store rebound")
			}
		})
	}
}
func TestDetachedImageTargetsRejectMixedAuthority(t *testing.T) {
	s, f := newManagedImageFixture("nerdctl")
	s.Maintain = func(context.Context, core.PersistentResourceRef, func(context.Context, core.Environment) error) error {
		t.Fatal("invalid target acquired maintenance")
		return nil
	}
	if _, err := s.List(context.Background(), f.resource.ID, "docker"); !errors.Is(err, core.ErrUnsupported) {
		t.Fatal(err)
	}
	id := "sha256:" + strings.Repeat("a", 64)
	for _, mode := range []string{"host", "environment", "generation", "docker", "host-store"} {
		target := ImageTarget{Detached: true, Store: f.resource.Ref(), Runtime: "nerdctl"}
		switch mode {
		case "host":
			target.Host = true
		case "environment":
			target.Environment = "dev"
		case "generation":
			target.Instance = f.target.Instance
		case "docker":
			target.Runtime = "docker"
		case "host-store":
			target.Store.ID = HostStoreID
		}
		if ValidImageSelection(target, id) {
			t.Fatal("mixed target valid", mode)
		}
		if err := s.Delete(context.Background(), target, id); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatal(mode, err)
		}
	}
}
