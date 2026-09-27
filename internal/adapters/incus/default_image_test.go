package incus

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestStandardImageUsesConfiguredRemoteAndTargetProject(t *testing.T) {
	for _, copyFails := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "copy failure"}[copyFails], func(t *testing.T) {
			t.Setenv(baseConfigEnv, "")
			copies := 0
			runner := &fakeRunner{run: func(_ context.Context, call int, command string, args []string) (host.Result, error) {
				if command != "incus" {
					t.Fatalf("unexpected command %q", command)
				}
				switch call {
				case 0:
					assertStringSlice(t, args, []string{"project", "show", "hacocoon"})
					return host.Result{}, nil
				case 1:
					assertStringSlice(t, args, []string{"image", "info", "images:ubuntu/26.04", "--format", "json"})
					return host.Result{Stdout: `{"fingerprint":"` + testFingerprintA + `"}`}, nil
				case 2:
					// No local: override: the controller may use a configured TLS remote.
					// --project selects the source; only --target-project selects the cache.
					assertStringSlice(t, args, []string{"image", "copy", "images:" + testFingerprintA, "", "--target-project", "hacocoon"})
					copies++
					if copyFails {
						return host.Result{ExitCode: 1}, nil
					}
					return host.Result{}, nil
				default:
					t.Fatalf("unexpected call %d: %v", call, args)
					return host.Result{}, nil
				}
			}}
			provider, err := NewBaseProvider(New(runner))
			if err != nil {
				t.Fatal(err)
			}
			image, err := provider.EnsureStandardImage(context.Background())
			if copyFails {
				if !errors.Is(err, core.ErrRuntimeUnavailable) || image != "" {
					t.Fatal(image, err)
				}
			} else if err != nil || image != defaultBaseName {
				t.Fatal(image, err)
			}
			if copies != 1 {
				t.Fatalf("copies = %d", copies)
			}
		})
	}
}
