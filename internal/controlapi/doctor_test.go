package controlapi

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/buildinfo"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/diagnostics"
)

type doctorServiceFunc func(context.Context) (diagnostics.Report, error)

func (f doctorServiceFunc) DiagnoseHost(ctx context.Context) (diagnostics.Report, error) {
	return f(ctx)
}

func healthyDoctorReport() diagnostics.Report {
	report := diagnostics.Report{}
	for _, name := range diagnostics.CheckNames() {
		report.Checks = append(report.Checks, diagnostics.Check{Name: name, Status: diagnostics.OK, Summary: "Verified predicate"})
	}
	return report
}

func doctorTestSocket(t *testing.T, register func(*control.Server)) string {
	t.Helper()
	server := control.NewServer()
	register(server)
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

func TestDoctorRoundTripIncludesControllerIdentityAndBoundedReadOnlyService(t *testing.T) {
	path := doctorTestSocket(t, func(server *control.Server) {
		err := RegisterDoctor(server, doctorServiceFunc(func(ctx context.Context) (diagnostics.Report, error) {
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) > 35*time.Second {
				t.Error("missing server deadline")
			}
			return healthyDoctorReport(), nil
		}))
		if err != nil {
			t.Fatal(err)
		}
	})
	client, _ := NewClient(path)
	response, err := client.Doctor(context.Background())
	if err != nil || !response.Healthy() || response.Controller != buildinfo.Current() {
		t.Fatalf("response=%+v err=%v", response, err)
	}
	wire, _ := control.NewClient(control.UnixDialer(path))
	err = wire.Call(context.Background(), MethodDoctor, map[string]string{"repair": "true"}, nil)
	var status *control.StatusError
	if !errors.As(err, &status) || status.Code != "invalid_argument" {
		t.Fatalf("accepted parameters: %v", err)
	}
}

func TestDoctorClientRejectsIncompleteOrMalformedResponse(t *testing.T) {
	for _, name := range []string{"missing", "duplicate", "unknown-status", "control-text", "protocol", "empty-payload", "missing-build", "invalid-build", "missing-action", "control-action", "oversized-action", "healthy-action"} {
		t.Run(name, func(t *testing.T) {
			path := doctorTestSocket(t, func(server *control.Server) {
				_ = server.Register(MethodDoctor, func(context.Context, json.RawMessage) (any, error) {
					r := DoctorResponse{ProtocolVersion: control.ProtocolVersion, Controller: buildinfo.Current(), Report: healthyDoctorReport()}
					switch name {
					case "missing":
						r.Checks = r.Checks[:4]
					case "duplicate":
						r.Checks[1].Name = r.Checks[0].Name
					case "unknown-status":
						r.Checks[1].Status = "maybe"
					case "control-text":
						r.Checks[1].Summary = "bad\x1b[2J"
					case "protocol":
						r.ProtocolVersion++
					case "missing-build":
						r.Controller = buildinfo.Info{}
					case "invalid-build":
						r.Controller.Version = "bad\x1b[2J"
					case "missing-action":
						r.Checks[1].Status = diagnostics.Failed
					case "control-action":
						r.Checks[1].Status = diagnostics.Failed
						r.Checks[1].Action = "bad\x1b[2J"
					case "oversized-action":
						r.Checks[1].Status = diagnostics.Failed
						r.Checks[1].Action = strings.Repeat("x", 257)
					case "healthy-action":
						r.Checks[1].Action = "Unnecessary repair"
					case "empty-payload":
						return nil, nil
					}
					return r, nil
				})
			})
			client, _ := NewClient(path)
			if _, err := client.Doctor(context.Background()); !errors.Is(err, control.ErrProtocol) {
				t.Fatalf("accepted malformed response: %v", err)
			}
		})
	}
}

func TestDoctorServiceErrorDoesNotExposeBackendOutput(t *testing.T) {
	path := doctorTestSocket(t, func(server *control.Server) {
		_ = RegisterDoctor(server, doctorServiceFunc(func(context.Context) (diagnostics.Report, error) {
			return diagnostics.Report{}, errors.New("Bearer secret")
		}))
	})
	client, _ := NewClient(path)
	if _, err := client.Doctor(context.Background()); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("error=%v", err)
	}
}

func TestDoctorCancellationClosesUnresponsivePeer(t *testing.T) {
	path := doctorTestSocket(t, func(server *control.Server) {
		_ = server.Register(MethodDoctor, func(ctx context.Context, _ json.RawMessage) (any, error) { <-ctx.Done(); return nil, ctx.Err() })
	})
	client, _ := NewClient(path)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := client.Doctor(ctx)
	if err == nil || time.Since(started) > time.Second {
		t.Fatalf("unbounded canceled request: %v", err)
	}
}
