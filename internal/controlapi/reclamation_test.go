package controlapi

import (
	"bytes"
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/logging"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/reclamation"
)

type reclaimServiceFunc func(context.Context, reclamation.WSLTarget) reclamation.LinuxReport

func (f reclaimServiceFunc) ReclaimLinux(ctx context.Context, target reclamation.WSLTarget) reclamation.LinuxReport {
	return f(ctx, target)
}

var rpcReclaimTarget = reclamation.WSLTarget{RegistrationID: "{11111111-1111-4111-8111-111111111111}", InstallationID: "22222222-2222-4222-8222-222222222222"}

func rpcReclaimSuccess() reclamation.LinuxReport {
	count := uint64(0)
	return reclamation.LinuxReport{
		Pool:  reclamation.Stage{FilesystemBefore: &reclamation.FilesystemUsage{CapacityBytes: 100}, FilesystemAfter: &reclamation.FilesystemUsage{CapacityBytes: 100}, Status: "complete", Attempted: true, Before: &reclamation.Allocation{LogicalBytes: 100}, After: &reclamation.Allocation{LogicalBytes: 100}, KernelTrimmedBytes: &count},
		Outer: reclamation.Stage{FilesystemBefore: &reclamation.FilesystemUsage{CapacityBytes: 100}, FilesystemAfter: &reclamation.FilesystemUsage{CapacityBytes: 100}, Status: "complete", Attempted: true, KernelTrimmedBytes: &count}}
}
func TestReclamationRPCPreservesFailureAndRejectsCallerPaths(t *testing.T) {
	calls := 0
	path := doctorTestSocket(t, func(s *control.Server) {
		if err := RegisterReclamation(s, reclaimServiceFunc(func(ctx context.Context, target reclamation.WSLTarget) reclamation.LinuxReport {
			calls++
			if target != rpcReclaimTarget {
				t.Fatal("changed identity")
			}
			d, ok := ctx.Deadline()
			if !ok || time.Until(d) > 5*time.Minute {
				t.Fatal("missing controller bound")
			}
			r := rpcReclaimSuccess()
			r.Outer.Status = "failed"
			r.Failure = "outer_trim_failed"
			return r
		})); err != nil {
			t.Fatal(err)
		}
	})
	wire, _ := control.NewClient(control.UnixDialer(path))
	for _, payload := range []any{nil, map[string]string{"pool": "other"}, map[string]string{"registration_id": rpcReclaimTarget.RegistrationID, "installation_id": rpcReclaimTarget.InstallationID, "path": "/foreign"}} {
		err := wire.Call(context.Background(), MethodReclaimLinux, payload, nil)
		var status *control.StatusError
		if !errors.As(err, &status) || status.Code != "invalid_argument" {
			t.Fatal("invalid request accepted", err)
		}
	}
	if calls != 0 {
		t.Fatal("invalid request performed work")
	}
	client, _ := NewClient(path)
	got, err := client.ReclaimLinux(context.Background(), rpcReclaimTarget)
	if err != nil || calls != 1 || got.Complete() || got.Failure != "outer_trim_failed" || got.Pool.Status != "complete" || !got.Outer.Attempted {
		t.Fatal(got, err)
	}
}
func TestReclamationRPCExcludesConcurrentRequest(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	path := doctorTestSocket(t, func(s *control.Server) {
		if err := RegisterReclamation(s, reclaimServiceFunc(func(context.Context, reclamation.WSLTarget) reclamation.LinuxReport {
			close(entered)
			<-release
			return rpcReclaimSuccess()
		})); err != nil {
			t.Fatal(err)
		}
	})
	client, _ := NewClient(path)
	done := make(chan error, 1)
	go func() { _, err := client.ReclaimLinux(context.Background(), rpcReclaimTarget); done <- err }()
	<-entered
	_, err := client.ReclaimLinux(context.Background(), rpcReclaimTarget)
	close(release)
	var status *control.StatusError
	if !errors.As(err, &status) || status.Code != "busy" {
		t.Error("concurrent trim accepted", err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
func TestReclamationRPCRejectsUnprovenSuccess(t *testing.T) {
	path := doctorTestSocket(t, func(s *control.Server) {
		if err := RegisterReclamation(s, reclaimServiceFunc(func(context.Context, reclamation.WSLTarget) reclamation.LinuxReport {
			r := rpcReclaimSuccess()
			r.Pool.Attempted = false
			return r
		})); err != nil {
			t.Fatal(err)
		}
	})
	client, _ := NewClient(path)
	if _, err := client.ReclaimLinux(context.Background(), rpcReclaimTarget); err == nil {
		t.Fatal("unproven native operation accepted")
	}
}

func TestReclamationFailureBoundaryDoesNotLogMalformedBackendReport(t *testing.T) {
	var diagnostic bytes.Buffer
	previous := logging.Root()
	logging.SetRoot(slog.New(slog.NewJSONHandler(&diagnostic, nil)))
	t.Cleanup(func() { logging.SetRoot(previous) })
	path := doctorTestSocket(t, func(s *control.Server) {
		if err := RegisterReclamation(s, reclaimServiceFunc(func(context.Context, reclamation.WSLTarget) reclamation.LinuxReport {
			return reclamation.NotStarted("token=private-backend-output")
		})); err != nil {
			t.Fatal(err)
		}
	})
	client, _ := NewClient(path)
	_, err := client.ReclaimLinux(context.Background(), rpcReclaimTarget)
	if err == nil || strings.Contains(err.Error(), "private-backend-output") || strings.Contains(diagnostic.String(), "private-backend-output") || !strings.Contains(diagnostic.String(), "reclaim_storage") {
		t.Fatal("unsafe failure boundary", err, diagnostic.String())
	}
}
