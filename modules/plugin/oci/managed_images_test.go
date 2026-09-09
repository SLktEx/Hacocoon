package oci

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"strings"
	"testing"
)

type managedImageFixture struct {
	upperReference                            bool
	target                                    ImageTarget
	resource                                  core.PersistentResource
	deleted, used, malformed, truncated, keep bool
	calls                                     []core.ExecutionRequest
}

func (f *managedImageFixture) GetEnvironment(context.Context, string) (core.Environment, error) {
	return core.Environment{Name: f.target.Environment, PersistentResource: f.target.Store}, nil
}
func (f *managedImageFixture) EnvironmentInstance(context.Context, core.Environment) (string, error) {
	return f.target.Instance, nil
}
func (f *managedImageFixture) GetPersistentResource(context.Context, string) (core.PersistentResource, error) {
	return f.resource, nil
}
func (f *managedImageFixture) ListSnapshots(context.Context) ([]core.Snapshot, error) {
	return nil, nil
}
func (f *managedImageFixture) ExecForResource(_ context.Context, name, instance string, ref core.PersistentResourceRef, r core.ExecutionRequest) (core.ExecutionResult, error) {
	if name != f.target.Environment || instance != f.target.Instance || ref != f.target.Store {
		return core.ExecutionResult{}, core.ErrCapabilityStale
	}
	f.calls = append(f.calls, r)
	a := r.Argv
	start := 0
	for i, s := range a {
		if s == "image" || s == "container" {
			start = i
			break
		}
	}
	a = a[start:]
	id := "sha256:" + strings.Repeat("1", 64)
	config := "sha256:" + strings.Repeat("2", 64)
	encode := func(v ...any) string {
		var b strings.Builder
		for _, x := range v {
			raw, _ := json.Marshal(x)
			b.Write(raw)
			b.WriteByte(' ')
		}
		return b.String()
	}
	result := core.ExecutionResult{}
	switch a[0] + " " + a[1] {
	case "image ls":
		if !f.deleted || f.keep {
			result.Stdout = id + "\n"
		}
	case "image inspect":
		if strings.Contains(a[3], ".Id") {
			inspectID := id
			if f.target.Runtime == "nerdctl" {
				inspectID = config
			}
			result.Stdout = encode(inspectID, []string{"example.local/app:dev"}, []string{"example.local/app@" + id})
		} else {
			result.Stdout = encode([]string{"example.local/app@" + id})
		}
	case "container ls":
		if f.used {
			result.Stdout = strings.Repeat("3", 64)
		}
	case "container inspect":
		image := id
		if f.target.Runtime == "nerdctl" {
			image = "example.local/app:dev"
			if f.upperReference {
				image = "example.local/app:Dev"
			}
		}
		result.Stdout = encode(image, "test-user")
	case "image rm":
		if a[2] != id || len(a) != 3 {
			return result, core.ErrInvalidArgument
		}
		f.deleted = true
	default:
		return result, core.ErrInvalidArgument
	}
	if f.malformed {
		result.Stdout = "untrusted malformed output"
	}
	result.StdoutTruncated = f.truncated
	return result, nil
}
func newManagedImageFixture(runtime string) (*ManagedImages, *managedImageFixture) {
	ref := core.PersistentResourceRef{ID: "oci:dev", Owner: strings.Repeat("a", 32)}
	f := &managedImageFixture{target: ImageTarget{Environment: "dev", Instance: "env-" + strings.Repeat("b", 32), Store: ref, Runtime: runtime}, resource: core.PersistentResource{ID: ref.ID, Owner: ref.Owner, Kind: StoreKind, State: "ready"}}
	return &ManagedImages{Catalog: f, Environments: f}, f
}
func TestManagedImageDeletionGuardsAndRuntimeIdentity(t *testing.T) {
	for _, runtime := range []string{"docker", "nerdctl"} {
		for _, mode := range []string{"unused", "used", "used-uppercase", "owner-changed", "source", "malformed", "truncated", "still-present"} {
			t.Run(runtime+"/"+mode, func(t *testing.T) {
				s, f := newManagedImageFixture(runtime)
				switch mode {
				case "used", "used-uppercase":
					f.used = true
					f.upperReference = mode == "used-uppercase"
				case "owner-changed":
					f.resource.Owner = strings.Repeat("c", 32)
				case "source":
					f.resource.SourceOnly = true
				case "malformed":
					f.malformed = true
				case "truncated":
					f.truncated = true
				case "still-present":
					f.keep = true
				}
				err := s.Delete(context.Background(), f.target, "sha256:"+strings.Repeat("1", 64))
				if mode == "unused" {
					if err != nil || !f.deleted {
						t.Fatalf("delete: %v", err)
					}
				} else {
					if err == nil {
						t.Fatal("unsafe success")
					}
					if mode != "still-present" && f.deleted {
						t.Fatal("mutated on refusal")
					}
				}
				if strings.HasPrefix(mode, "used") && !errors.Is(err, core.ErrStorageBusy) {
					t.Fatalf("in-use classification: %v", err)
				}
				if mode == "still-present" && !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatalf("absence classification: %v", err)
				}
				for _, call := range f.calls {
					if strings.Contains(strings.Join(call.Argv, " "), "--force") || len(call.Argv) < 4 || call.Argv[0] != "/usr/bin/env" || call.Argv[1] != "-i" {
						t.Fatalf("unsafe invocation: %v", call.Argv)
					}
				}
			})
		}
	}
}

type hostManagedImageFixture struct{ *managedImageFixture }

func (f *hostManagedImageFixture) ExecHostImage(ctx context.Context, resource core.PersistentResource, runtime string, args []string) (core.ExecutionResult, error) {
	if resource.Ref() != f.target.Store || !resource.SourceOnly || runtime != f.target.Runtime {
		return core.ExecutionResult{}, core.ErrCapabilityStale
	}
	return f.ExecForResource(ctx, "", "", resource.Ref(), core.ExecutionRequest{Argv: args})
}
func TestManagedHostImageUsesExactSourceWithoutEnvAuthority(t *testing.T) {
	for _, mode := range []string{"valid", "owner", "guest-source", "mixed-target", "used"} {
		t.Run(mode, func(t *testing.T) {
			service, fixture := newManagedImageFixture("docker")
			fixture.resource.ID = HostStoreID
			fixture.resource.SourceOnly = true
			fixture.target = ImageTarget{Host: true, Store: fixture.resource.Ref(), Runtime: "docker"}
			service.Host = &hostManagedImageFixture{fixture}
			service.Environments = nil
			target := fixture.target
			switch mode {
			case "owner":
				target.Store.Owner = strings.Repeat("f", 32)
			case "guest-source":
				fixture.resource.SourceOnly = false
			case "mixed-target":
				target.Environment = "guest"
			case "used":
				fixture.used = true
			}
			err := service.Delete(context.Background(), target, "sha256:"+strings.Repeat("1", 64))
			if mode == "valid" {
				if err != nil || !fixture.deleted {
					t.Fatalf("host delete: %v", err)
				}
			} else if err == nil || fixture.deleted {
				t.Fatalf("unsafe host delete: %v", err)
			}
		})
	}
}
