package incus

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestRuntimeRejectsOptionLikeInstanceRefsBeforeIncus(t *testing.T) {
	cases := []struct {
		name string
		call func(*Runtime) error
	}{
		{name: "start-all", call: func(r *Runtime) error { return r.Start(context.Background(), "--all") }},
		{name: "stop-all", call: func(r *Runtime) error { return r.Stop(context.Background(), "--all") }},
		{name: "delete-force", call: func(r *Runtime) error { return r.DeleteEnvironment(context.Background(), "--force") }},
		{name: "shell-project", call: func(r *Runtime) error { return r.ShellEnvironment(context.Background(), "--project") }},
		{name: "inspect-all-projects", call: func(r *Runtime) error {
			_, err := r.InspectEnvironment(context.Background(), "--all-projects")
			return err
		}},
		{name: "exec-help", call: func(r *Runtime) error {
			_, err := r.ExecEnvironment(context.Background(), "--help", core.ExecutionRequest{Argv: []string{"true"}})
			return err
		}},
		{name: "forward-project", call: func(r *Runtime) error {
			_, err := r.ForwardLocalPort(context.Background(), "--project", core.LocalPortRequest{HostPort: 2222, TargetPort: 22})
			return err
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := &fakeRunner{}
			err := tc.call(New(runner))
			if !errors.Is(err, core.ErrInvalidArgument) {
				t.Fatalf("err=%v", err)
			}
			if len(runner.calls) != 0 {
				t.Fatalf("invalid ref reached Incus: %#v", runner.calls)
			}
		})
	}
}

func TestClientAccessRejectsUnmanagedRefBeforeIncus(t *testing.T) {
	runner := &fakeRunner{}
	_, err := New(runner).PrepareSSHAccess(context.Background(), "--project", core.SSHAccessRequest{HostPort: 2222, PublicKey: "ssh-ed25519 AAAA"})
	if !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("err=%v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("invalid ref reached Incus: %#v", runner.calls)
	}
}

func TestManagedIncusRefAllowsExpectedNames(t *testing.T) {
	for _, ref := range []string{"haco-demo", "haco-a1", "haco-0123456789abcdef0123456789abcdef", trustedHostName} {
		if err := validateManagedInstanceRef(ref); err != nil {
			t.Fatalf("ref %q: %v", ref, err)
		}
	}
}

func TestCreateEnvironmentRejectsInvalidNameBeforeIncus(t *testing.T) {
	runner := &fakeRunner{}
	_, err := New(runner).CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "--force", WorkspacePath: "/tmp/workspace"})
	if !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("err=%v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("invalid environment name reached Incus: %#v", runner.calls)
	}
}
