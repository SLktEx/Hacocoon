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

	"github.com/SLktEx/Hacocoon/internal/buildinfo"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/diagnostics"
)

func productDoctorServer(t *testing.T, failed bool) string {
	t.Helper()
	server := control.NewServer()
	_ = server.Register(controlapi.MethodDoctor, func(context.Context, json.RawMessage) (any, error) {
		report := diagnostics.Report{}
		for _, name := range diagnostics.CheckNames() {
			report.Checks = append(report.Checks, diagnostics.Check{Name: name, Status: diagnostics.OK, Summary: "Verified predicate"})
		}
		if failed {
			report.Checks[1].Status = diagnostics.Failed
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
	if code := doctor(context.Background(), []string{"--json"}, &stdout, &stderr); code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "unavailable") {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}
