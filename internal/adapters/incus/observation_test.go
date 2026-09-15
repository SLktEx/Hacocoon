package incus

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestIncusObservationsRejectIncompleteCommandResults(t *testing.T) {
	for _, bad := range []host.Result{
		{ExitCode: 1}, {StdoutTruncated: true}, {Stdout: "haco-demo\n", StdoutTruncated: true},
	} {
		runner := &fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) { return bad, nil }}
		r := New(runner)
		if exists, err := r.environmentExists(context.Background(), "haco-demo"); err == nil || exists {
			t.Fatalf("false absence: %v %v", exists, err)
		}
		if _, err := r.InspectEnvironment(context.Background(), "haco-demo"); err == nil {
			t.Fatal("partial status accepted")
		}
		if _, err := r.ResolveRuntimeRef(context.Background(), net.ParseIP("10.200.0.23")); err == nil {
			t.Fatal("partial authority accepted")
		}
		if caps, err := r.Probe(context.Background()); err != nil || caps.Available {
			t.Fatalf("failed probe became available: %#v %v", caps, err)
		}
	}
}

func TestInstanceAbsenceRequiresCompleteExactInventory(t *testing.T) {
	for _, tc := range []struct {
		raw             string
		present, failed bool
	}{
		{"", false, false}, {"haco-demo-copy\n", false, false}, {"haco-demo\n", true, false},
		{"haco-demo\nhaco-demo\n", false, true}, {"\"unfinished\n", false, true},
		{"haco-other,unexpected\n", false, true},
	} {
		r := New(&fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) {
			return host.Result{Stdout: tc.raw}, nil
		}})
		present, err := r.environmentExists(context.Background(), "haco-demo")
		if present != tc.present || (err != nil) != tc.failed {
			t.Fatalf("%q: %v %v", tc.raw, present, err)
		}
	}
}

func TestAbsentAndUnknownRuntimeObservationsAreDistinct(t *testing.T) {
	for _, tc := range []struct {
		raw    string
		absent bool
	}{
		{"", true}, {"haco-demo-copy,RUNNING\n", true}, {"haco-demo,FROZEN\n", false},
	} {
		r := New(&fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) {
			return host.Result{Stdout: tc.raw}, nil
		}})
		got, err := r.InspectEnvironment(context.Background(), "haco-demo")
		if err != nil || got.Absent != tc.absent || got.State != core.EnvironmentUnknown {
			t.Fatalf("%q: %#v %v", tc.raw, got, err)
		}
	}
}

func TestCanceledObservationCannotProveAbsence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := New(&fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) { cancel(); return host.Result{}, nil }})
	if absent, err := r.environmentExists(ctx, "haco-demo"); absent || !errors.Is(err, context.Canceled) {
		t.Fatalf("%v %v", absent, err)
	}
}

func TestDeleteCannotUseTruncatedInventoryAsAbsence(t *testing.T) {
	deletion := errors.New("delete failed")
	r := New(&fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if args[0] == "delete" {
			return host.Result{ExitCode: 1}, deletion
		}
		return host.Result{StdoutTruncated: true}, nil
	}})
	err := r.DeleteEnvironment(context.Background(), "haco-demo")
	if !errors.Is(err, deletion) || core.EnvironmentDeletionComplete(err) {
		t.Fatalf("unsafe deletion outcome: %v", err)
	}
}
