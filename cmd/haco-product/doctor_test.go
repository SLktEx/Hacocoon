package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/buildinfo"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/diagnostics"
)

func productDoctorServer(t *testing.T, failed bool) string {
	t.Helper()
	server := control.NewServer()
	_ = server.Register(controlapi.MethodPing, func(context.Context, json.RawMessage) (any, error) {
		return controlapi.PingResponse{ProtocolVersion: control.ProtocolVersion}, nil
	})
	_ = server.Register(controlapi.MethodDoctor, func(context.Context, json.RawMessage) (any, error) {
		report := diagnostics.Report{}
		for _, name := range diagnostics.CheckNames() {
			report.Checks = append(report.Checks, diagnostics.Check{Name: name, Status: diagnostics.OK, Summary: "Verified predicate"})
		}
		if failed {
			report.Checks[1].Status = diagnostics.Failed
			report.Checks[1].Summary = "Configured storage differs"
			report.Checks[1].Action = "Inspect the configured Incus pool"
		}
		return controlapi.DoctorResponse{ProtocolVersion: control.ProtocolVersion, Controller: buildinfo.Current(), Report: report}, nil
	})
	path := filepath.Join(t.TempDir(), "control.sock")
	listener, err := control.ListenUnix(path, 0600)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	t.Cleanup(func() { cancel(); <-done })
	return path
}

func TestProductDoctorUsesControllerWithoutLegacyOrLocalRuntime(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "healthy", true: "failed-check"}[failed], func(t *testing.T) {
			t.Setenv("HACO_CONTROL_SOCKET", productDoctorServer(t, failed))
			t.Setenv("PATH", t.TempDir()) // No hacoq, incus, shell or sudo to fall back to.
			t.Setenv("HACO_ROOT", filepath.Join(t.TempDir(), "must-not-create"))
			for _, args := range [][]string{nil, {"--json"}} {
				var stdout, stderr bytes.Buffer
				code := doctor(context.Background(), args, &stdout, &stderr)
				if _, err := os.Stat(os.Getenv("HACO_ROOT")); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("doctor constructed local state: %v", err)
				}
				want := 0
				if failed {
					want = 1
				}
				if code != want {
					t.Fatalf("code=%d stderr=%s", code, stderr.String())
				}
				if len(args) > 0 {
					var response controlapi.DoctorResponse
					if err := json.Unmarshal(stdout.Bytes(), &response); err != nil || response.Healthy() == failed {
						t.Fatalf("invalid JSON %q: %v", stdout.String(), err)
					}
				} else if !strings.Contains(stdout.String(), "Hacocoon Host diagnostics") {
					t.Fatalf("output=%q", stdout.String())
				}
				if failed && !strings.Contains(stdout.String(), "Inspect the configured Incus pool") {
					t.Fatalf("missing next action: %s", stdout.String())
				}
				if failed && !strings.Contains(stderr.String(), "operation=doctor") {
					t.Fatalf("missing structured failure: %s", stderr.String())
				}
				if !failed && stderr.Len() != 0 {
					t.Fatalf("unexpected stderr: %s", stderr.String())
				}
			}
		})
	}
}

func TestProductDoctorHelpAndUsageNeedNoController(t *testing.T) {
	t.Setenv("HACO_CONTROL_SOCKET", filepath.Join(t.TempDir(), "missing.sock"))
	for _, args := range [][]string{{"--help"}, {"--repair"}, {"--json", "--json"}, {"target"}} {
		var stdout, stderr bytes.Buffer
		code := doctor(context.Background(), args, &stdout, &stderr)
		if _, err := os.Stat(os.Getenv("HACO_ROOT")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("doctor constructed local state: %v", err)
		}
		want := 2
		if args[0] == "--help" {
			want = 0
		}
		if code != want {
			t.Fatalf("args=%v code=%d stderr=%s", args, code, stderr.String())
		}
	}
}

func TestProductDoctorUnavailableDoesNotReturnHealthyJSON(t *testing.T) {
	t.Setenv("HACO_CONTROL_SOCKET", filepath.Join(t.TempDir(), "missing.sock"))
	var stdout, stderr bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if code := doctor(ctx, []string{"--json"}, &stdout, &stderr); code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "timed out") {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}

type startupDoctorClient struct {
	ping        func(context.Context) error
	doctorCalls int
}

func (c *startupDoctorClient) Ping(ctx context.Context) (controlapi.PingResponse, error) {
	return controlapi.PingResponse{ProtocolVersion: control.ProtocolVersion}, c.ping(ctx)
}
func (c *startupDoctorClient) Doctor(context.Context) (controlapi.DoctorResponse, error) {
	c.doctorCalls++
	// A failed infrastructure check is still a completed diagnostic response.
	return controlapi.DoctorResponse{ProtocolVersion: control.ProtocolVersion}, nil
}

func TestDoctorWaitsForControllerThenDiagnosesOnce(t *testing.T) {
	calls := 0
	client := &startupDoctorClient{ping: func(ctx context.Context) error {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 30*time.Second {
			t.Fatal("missing bounded controller readiness")
		}
		calls++
		if calls == 1 {
			return control.ErrUnavailable
		}
		return nil
	}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	response, err := collectDoctor(ctx, client)
	if err != nil || calls != 2 || client.doctorCalls != 1 || response.Healthy() {
		t.Fatalf("pings=%d diagnostics=%d response=%+v error=%v", calls, client.doctorCalls, response, err)
	}
}

func TestDoctorDoesNotRetryRejectedReadiness(t *testing.T) {
	for _, rejected := range []error{control.ErrProtocol, control.NewStatusError("forbidden", "denied")} {
		calls := 0
		client := &startupDoctorClient{ping: func(context.Context) error { calls++; return rejected }}
		_, err := collectDoctor(context.Background(), client)
		if !errors.Is(err, rejected) || calls != 1 || client.doctorCalls != 0 {
			t.Fatalf("pings=%d diagnostics=%d error=%v", calls, client.doctorCalls, err)
		}
	}
}

func TestDoctorCancellationStopsReadinessWithoutDiagnosing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &startupDoctorClient{ping: func(context.Context) error {
		cancel()
		return control.ErrUnavailable
	}}
	_, err := collectDoctor(ctx, client)
	if !errors.Is(err, context.Canceled) || client.doctorCalls != 0 {
		t.Fatalf("diagnostics=%d error=%v", client.doctorCalls, err)
	}
}
