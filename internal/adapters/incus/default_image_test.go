package incus

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/host/setup"
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
			var events []hostsetup.Event
			ctx := hostsetup.Observe(context.Background(), func(e hostsetup.Event) { events = append(events, e) })
			image, err := provider.EnsureStandardImage(ctx)
			if copyFails {
				stage, reason := hostsetup.Details(err)
				if !errors.Is(err, core.ErrRuntimeUnavailable) || image != "" || stage != "default_image_copy" || reason != "unavailable" {
					t.Fatal(image, err)
				}
				if len(events) != 6 || events[5].State != "failed" || events[5].Reason != "unavailable" {
					t.Fatal(events)
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

func TestStandardImageProgressPreservesFailuresAndStopsDownstream(t *testing.T) {
	stages := []string{"default_image_project", "default_image_resolve", "default_image_copy"}
	for failed := -1; failed < len(stages); failed++ {
		name := "success"
		if failed >= 0 {
			name = stages[failed]
		}
		t.Run(name, func(t *testing.T) {
			t.Setenv(baseConfigEnv, "")
			failure := &exec.Error{Name: "SECRET-command", Err: errors.New("SECRET-provider-output https://secret.invalid/token")}
			runner := &fakeRunner{run: func(_ context.Context, call int, _ string, args []string) (host.Result, error) {
				if failed == 0 {
					switch call {
					case 0:
						assertStringSlice(t, args, []string{"project", "show", "hacocoon"})
					case 1:
						assertStringSlice(t, args, []string{"project", "create", "hacocoon", "--config", "features.profiles=false"})
					default:
						t.Fatal("operation after project failure", args)
					}
					return host.Result{ExitCode: 17, Stderr: "SECRET"}, failure
				}
				if call > 2 {
					t.Fatal("unexpected repeated operation", args)
				}
				if call == failed {
					return host.Result{ExitCode: 17, Stderr: "SECRET"}, failure
				}
				if call == 1 {
					return host.Result{Stdout: `{"fingerprint":"` + testFingerprintA + `"}`}, nil
				}
				return host.Result{}, nil
			}}
			provider, err := NewBaseProvider(New(runner))
			if err != nil {
				t.Fatal(err)
			}
			var events []hostsetup.Event
			ctx := hostsetup.Observe(context.Background(), func(e hostsetup.Event) { events = append(events, e) })
			image, err := provider.EnsureStandardImage(ctx)
			count, calls := 3, 3
			if failed >= 0 {
				count, calls = failed+1, failed+1
				if failed == 0 {
					calls = 2
				} // Preserve ensureProject's existing show/create behavior.
				stage, reason := hostsetup.Details(err)
				var original *exec.Error
				if !errors.Is(err, failure) || !errors.As(err, &original) || original != failure || stage != stages[failed] || reason != "failed" || image != "" || strings.Contains(err.Error(), "SECRET") {
					t.Fatal(stage, reason, err, image)
				}
			} else if err != nil || image != defaultBaseName {
				t.Fatal(image, err)
			}
			if len(runner.calls) != calls || len(events) != 2*count {
				t.Fatal(runner.calls, events)
			}
			for n := 0; n < count; n++ {
				start, end := events[2*n], events[2*n+1]
				state, reason := "succeeded", ""
				if n == failed {
					state, reason = "failed", "failed"
				}
				if start.Stage != stages[n] || start.State != "running" || end.Stage != stages[n] || end.State != state || end.Reason != reason || end.DurationMS < 0 {
					t.Fatal(events)
				}
			}
			encoded, _ := json.Marshal(events)
			if strings.Contains(string(encoded), "SECRET") || strings.Contains(string(encoded), testFingerprintA) || strings.Contains(string(encoded), "https://") {
				t.Fatal("provider data escaped progress")
			}
		})
	}
}

func TestStandardImageObservationDoesNotAddCancellationChecks(t *testing.T) {
	t.Setenv(baseConfigEnv, "")
	for cancelAt := -1; cancelAt < 3; cancelAt++ {
		ctx, cancel := context.WithCancel(context.Background())
		if cancelAt == -1 {
			cancel()
		}
		runner := &fakeRunner{run: func(_ context.Context, call int, _ string, _ []string) (host.Result, error) {
			if call == cancelAt {
				cancel()
			}
			if call == 1 {
				return host.Result{Stdout: `{"fingerprint":"` + testFingerprintA + `"}`}, nil
			}
			return host.Result{}, nil
		}}
		provider, err := NewBaseProvider(New(runner))
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		image, err := provider.EnsureStandardImage(ctx)
		cancel()
		if err != nil || image != defaultBaseName || len(runner.calls) != 3 {
			t.Fatal(cancelAt, image, err, runner.calls)
		}
	}
}

func TestStandardImagePreservesCancellationCauses(t *testing.T) {
	t.Setenv(baseConfigEnv, "")
	for failed, stage := range []string{"default_image_project", "default_image_resolve", "default_image_copy"} {
		for _, failure := range []error{context.Canceled, context.DeadlineExceeded} {
			t.Run(stage+"/"+hostsetup.Reason(failure), func(t *testing.T) {
				runner := &fakeRunner{run: func(_ context.Context, call int, _ string, _ []string) (host.Result, error) {
					if failed == 0 || call == failed {
						return host.Result{}, failure
					}
					if call == 1 {
						return host.Result{Stdout: `{"fingerprint":"` + testFingerprintA + `"}`}, nil
					}
					return host.Result{}, nil
				}}
				provider, err := NewBaseProvider(New(runner))
				if err != nil {
					t.Fatal(err)
				}
				_, err = provider.EnsureStandardImage(context.Background())
				gotStage, reason := hostsetup.Details(err)
				if !errors.Is(err, failure) || gotStage != stage || reason != hostsetup.Reason(failure) {
					t.Fatal(gotStage, reason, err)
				}
			})
		}
	}
}
