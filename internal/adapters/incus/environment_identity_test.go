package incus

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

const testEnvironmentInstance = "env-11111111111111111111111111111111"

func TestProviderSnapshotIdentityRejectsReplacementWithoutRepair(t *testing.T) {
	for _, mode := range []string{"valid", "missing", "replaced", "truncated", "failed", "invalid-id", "host", "option"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
				calls++
				if name != "incus" || !reflect.DeepEqual(args, []string{"config", "get", "haco-demo", environmentInstanceKey, "--project", "hacocoon"}) {
					t.Fatal("unexpected ownership operation", args)
				}
				switch mode {
				case "missing":
					return host.Result{}, nil
				case "replaced":
					return host.Result{Stdout: "env-22222222222222222222222222222222"}, nil
				case "truncated":
					return host.Result{Stdout: testEnvironmentInstance, StdoutTruncated: true}, nil
				case "failed":
					return host.Result{}, errors.New("unavailable")
				}
				return host.Result{Stdout: testEnvironmentInstance + "\n"}, nil
			}}
			id, ref := testEnvironmentInstance, "haco-demo"
			switch mode {
			case "invalid-id":
				id = "bad"
			case "host":
				ref = "haco-host"
			case "option":
				ref = "--help"
			}
			err := New(runner).VerifyEnvironmentIdentity(context.Background(), ref, id)
			if mode == "valid" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("unsafe source accepted")
			}
			if (mode == "invalid-id" || mode == "host" || mode == "option") && calls != 0 {
				t.Fatal("invalid source reached provider")
			}
			if calls > 1 {
				t.Fatal("source was repaired or retried")
			}
		})
	}
}
func TestInvalidCreationIdentityFailsBeforeProviderMutation(t *testing.T) {
	for _, sandbox := range []bool{false, true} {
		t.Run(map[bool]string{false: "runtime", true: "sandbox"}[sandbox], func(t *testing.T) {
			runner := &fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) {
				t.Fatal("provider called for invalid creation ID")
				return host.Result{}, nil
			}}
			runtime := New(runner)
			var create func(context.Context, core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) = runtime.CreateEnvironment
			if sandbox {
				provider, err := NewSandboxProvider(runtime)
				if err != nil {
					t.Fatal(err)
				}
				create = provider.CreateEnvironment
			}
			_, err := create(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: "/work", InstanceID: "bad"})
			if !errors.Is(err, core.ErrInvalidArgument) {
				t.Fatal(err)
			}
		})
	}
}
