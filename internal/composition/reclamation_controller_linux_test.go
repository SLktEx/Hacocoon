//go:build linux && (amd64 || arm64)

package composition

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/reclamation"
	"github.com/SLktEx/Hacocoon/modules/runtime/incus"
)

var reclaimFixtureTarget = reclamation.WSLTarget{RegistrationID: "{11111111-1111-4111-8111-111111111111}", InstallationID: "22222222-2222-4222-8222-222222222222"}

type reclaimFixture struct {
	calls                       *[]string
	poolErr, outerErr, closeErr error
}

func (f *reclaimFixture) TrimPool(context.Context) (incus.PoolTrimObservation, error) {
	*f.calls = append(*f.calls, "pool")
	return incus.PoolTrimObservation{FilesystemBefore: &incus.ReclaimFilesystemUsage{CapacityBytes: 100, UsedBytes: 80}, FilesystemAfter: &incus.ReclaimFilesystemUsage{CapacityBytes: 100, UsedBytes: 70}, Before: incus.ReclaimAllocation{LogicalBytes: 100, AllocatedBytes: 80}, After: incus.ReclaimAllocation{LogicalBytes: 100, AllocatedBytes: 40}, Attempted: true, KernelReportKnown: true, KernelTrimmedBytes: 80}, f.poolErr
}
func (f *reclaimFixture) TrimBackingFilesystem(context.Context) (incus.OuterTrimObservation, error) {
	*f.calls = append(*f.calls, "outer")
	return incus.OuterTrimObservation{FilesystemBefore: &incus.ReclaimFilesystemUsage{CapacityBytes: 1000, UsedBytes: 800}, FilesystemAfter: &incus.ReclaimFilesystemUsage{CapacityBytes: 1000, UsedBytes: 750}, Attempted: true, KernelReportKnown: true, KernelTrimmedBytes: 1000}, f.outerErr
}
func (f *reclaimFixture) Close() error { *f.calls = append(*f.calls, "close"); return f.closeErr }

func TestReclaimLinuxRetainsPartialStagesAndRechecksInstallation(t *testing.T) {
	failure := errors.New("untrusted backend output token=secret")
	for _, tc := range []struct {
		name, reason                string
		poolErr, outerErr, closeErr error
		change                      bool
		order                       []string
	}{
		{"complete", "", nil, nil, nil, false, []string{"identity", "select", "pool", "identity", "outer", "close"}},
		{"pool", "pool_trim_failed", failure, nil, nil, false, []string{"identity", "select", "pool", "close"}},
		{"changed", "identity_changed", nil, nil, nil, true, []string{"identity", "select", "pool", "identity", "close"}},
		{"outer", "outer_trim_failed", nil, failure, nil, false, []string{"identity", "select", "pool", "identity", "outer", "close"}},
		{"close", "cleanup_failed", nil, nil, failure, false, []string{"identity", "select", "pool", "identity", "outer", "close"}},
		{"pool_and_close", "pool_trim_failed", failure, nil, failure, false, []string{"identity", "select", "pool", "close"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := []string{}
			reads := 0
			target := &reclaimFixture{&calls, tc.poolErr, tc.outerErr, tc.closeErr}
			report := reclaimLinux(context.Background(), reclaimFixtureTarget, func(context.Context) (reclamation.WSLTarget, error) {
				calls = append(calls, "identity")
				reads++
				actual := reclaimFixtureTarget
				if tc.change && reads == 2 {
					actual.InstallationID = "33333333-3333-4333-8333-333333333333"
				}
				return actual, nil
			}, func(context.Context) (linuxReclaimTarget, error) { calls = append(calls, "select"); return target, nil })
			if report.CleanupFailed != (tc.closeErr != nil) {
				t.Fatal("lost cleanup failure", report)
			}
			if !reflect.DeepEqual(calls, tc.order) || report.Failure != tc.reason || report.Complete() != (tc.reason == "") || report.Validate() != nil {
				t.Fatal(calls, report, report.Validate())
			}
			if !report.Pool.Attempted || report.Pool.Before.AllocatedBytes != 80 || report.Pool.After.AllocatedBytes != 40 || *report.Pool.KernelTrimmedBytes != 80 {
				t.Fatal("lost pool observations", report)
			}
			if tc.poolErr != nil || tc.change {
				if report.Outer.Status != "skipped" || report.Outer.Attempted {
					t.Fatal("outer trim after refusal", report)
				}
			}
		})
	}
}
func TestReclaimLinuxRejectsIdentityBeforePoolAccess(t *testing.T) {
	for _, mode := range []string{"invalid", "canceled", "missing", "changed"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			expected := reclaimFixtureTarget
			if mode == "invalid" {
				expected.RegistrationID = "--shutdown"
			}
			if mode == "canceled" {
				cancel()
			}
			report := reclaimLinux(ctx, expected, func(context.Context) (reclamation.WSLTarget, error) {
				if mode == "invalid" || mode == "canceled" {
					t.Fatal("identity read before rejection")
				}
				if mode == "missing" {
					return reclamation.WSLTarget{}, errors.New("missing")
				}
				return reclamation.WSLTarget{}, nil
			}, func(context.Context) (linuxReclaimTarget, error) {
				t.Fatal("pool access without identity")
				return nil, nil
			})
			if report.Validate() != nil || report.Complete() || report.Pool.Attempted || report.Outer.Attempted {
				t.Fatal(report)
			}
		})
	}
}

type reclaimIdentityRunner func(context.Context, string, ...string) (host.Result, error)

func (f reclaimIdentityRunner) Run(ctx context.Context, name string, args ...string) (host.Result, error) {
	return f(ctx, name, args...)
}
func TestReclaimInstallationReadUsesFixedBoundedHelper(t *testing.T) {
	valid := `{"schema_version":1,"registration_id":"{11111111-1111-4111-8111-111111111111}","installation_id":"22222222-2222-4222-8222-222222222222"}`
	for _, tc := range []struct {
		name, body string
		exit       int
		truncated  bool
	}{
		{"valid", valid, 0, false}, {"malformed", "{", 0, false}, {"extra", valid + "{}", 0, false},
		{"unknown", strings.Replace(valid, "schema_version", "other", 1), 0, false},
		{"oversized", strings.Repeat(" ", 4097), 0, false}, {"failed", valid, 1, false}, {"truncated", valid, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := readReclaimInstallation(context.Background(), reclaimIdentityRunner(func(_ context.Context, name string, args ...string) (host.Result, error) {
				want := []string{"-i", "PATH=/usr/sbin:/usr/bin:/sbin:/bin", "/usr/bin/python3", "-I", "/usr/local/libexec/hacocoon-wsl-interop", "--read-registration"}
				if name != "/usr/bin/env" || !reflect.DeepEqual(args, want) {
					t.Fatal("caller-controlled identity reader", name, args)
				}
				return host.Result{Stdout: tc.body, ExitCode: tc.exit, StdoutTruncated: tc.truncated}, nil
			}))
			if tc.name == "valid" {
				if err != nil || got != reclaimFixtureTarget {
					t.Fatal(got, err)
				}
			} else if err == nil {
				t.Fatal("invalid identity output accepted")
			}
		})
	}
}
