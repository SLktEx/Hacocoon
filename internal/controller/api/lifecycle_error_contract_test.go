package controlapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync/atomic"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type recoveryAPIService struct{ calls atomic.Int32 }

func (s *recoveryAPIService) failure() error {
	s.calls.Add(1)
	return errors.Join(core.ErrNotFound, core.ErrRecoveryRequired)
}
func (s *recoveryAPIService) Create(context.Context, core.EnvironmentSpec) (core.Environment, error) {
	return core.Environment{}, s.failure()
}
func (s *recoveryAPIService) List(context.Context) ([]core.Environment, error) {
	return nil, s.failure()
}
func (s *recoveryAPIService) Exec(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error) {
	return core.ExecutionResult{}, s.failure()
}
func (s *recoveryAPIService) PrepareShellStream(context.Context, string) (func(context.Context, io.Reader, io.Writer, io.Writer) error, error) {
	return nil, s.failure()
}
func (s *recoveryAPIService) Delete(context.Context, string) error { return s.failure() }
func (s *recoveryAPIService) Status(context.Context, string) (core.EnvironmentStatus, error) {
	return core.EnvironmentStatus{}, s.failure()
}
func (s *recoveryAPIService) Connections(context.Context, string) ([]core.ClientConnection, error) {
	return nil, s.failure()
}
func (s *recoveryAPIService) Forward(context.Context, string, core.LocalPortRequest) (core.ClientConnection, error) {
	return core.ClientConnection{}, s.failure()
}
func (s *recoveryAPIService) Unforward(context.Context, string, string) error { return s.failure() }
func (s *recoveryAPIService) SSH(context.Context, string, core.SSHAccessRequest) (core.ClientConnection, error) {
	return core.ClientConnection{}, s.failure()
}

func TestLifecycleWireRejectsMalformedRequestsAndKeepsRecoveryStatus(t *testing.T) {
	service := &recoveryAPIService{}
	path := doctorTestSocket(t, func(s *control.Server) {
		if err := Register(s, service, service); err != nil {
			t.Fatal(err)
		}
	})
	client, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ method, payload string }{
		{MethodEnvironmentCreate, `{"name":"dev","workspace_path":"/retained"}`},
		{MethodEnvironmentList, `{}`},
		{MethodEnvironmentStatus, `{"environment":"dev"}`},
		{MethodEnvironmentConnections, `{"environment":"dev"}`},
		{MethodEnvironmentForward, `{"environment":"dev","protocol":"tcp","host_port":8080,"target_port":80}`},
		{MethodEnvironmentUnforward, `{"environment":"dev","connection_id":"owned-connection"}`},
		{MethodEnvironmentSSH, `{"environment":"dev","public_key":"reviewed-key"}`},
		{MethodEnvironmentExec, `{"environment":"dev","argv":["true"]}`},
		{MethodEnvironmentShell, `{"environment":"dev"}`},
		{MethodEnvironmentDelete, `{"environment":"dev"}`},
	} {
		t.Run(tc.method, func(t *testing.T) {
			call := func(payload string) error {
				if tc.method == MethodEnvironmentShell {
					conn, err := client.wire.OpenSession(context.Background(), tc.method, json.RawMessage(payload))
					if conn != nil {
						_ = conn.Close()
						t.Error("unconfirmed shell published")
					}
					return err
				}
				var result json.RawMessage
				err := client.wire.Call(context.Background(), tc.method, json.RawMessage(payload), &result)
				if len(result) != 0 {
					t.Error("unconfirmed lifecycle receipt published", string(result))
				}
				return err
			}
			before := service.calls.Load()
			if tc.method != MethodEnvironmentList {
				for _, payload := range []string{`"not-an-object"`, `{}`} {
					var status *control.StatusError
					if err := call(payload); !errors.As(err, &status) || status.Code != "invalid_argument" {
						t.Fatal("malformed request did not retain argument refusal", payload, err)
					}
				}
				if service.calls.Load() != before {
					t.Fatal("invalid request reached lifecycle")
				}
			}
			var status *control.StatusError
			if err := call(tc.payload); !errors.As(err, &status) || status.Code != "recovery_required" {
				t.Fatal("recovery obligation became ordinary absence", err)
			}
			if service.calls.Load() != before+1 {
				t.Fatal("lifecycle operation was repeated", service.calls.Load()-before)
			}
		})
	}
}
