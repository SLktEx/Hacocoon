package controlapi

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/reclamation"
)

type reclamationTargetFunc func(context.Context) (reclamation.WSLTarget, error)

func (f reclamationTargetFunc) ReclamationTarget(ctx context.Context) (reclamation.WSLTarget, error) {
	return f(ctx)
}

func TestReclamationTargetHasNoCallerSelectionOrSideEffects(t *testing.T) {
	calls := 0
	path := doctorTestSocket(t, func(s *control.Server) {
		err := RegisterReclamationTarget(s, reclamationTargetFunc(func(ctx context.Context) (reclamation.WSLTarget, error) {
			calls++
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) > 15*time.Second {
				t.Fatal("unbounded discovery")
			}
			return rpcReclaimTarget, nil
		}))
		if err != nil {
			t.Fatal(err)
		}
	})
	wire, _ := control.NewClient(control.UnixDialer(path))
	for _, payload := range []any{nil, map[string]string{"registration_id": rpcReclaimTarget.RegistrationID}, map[string]string{"path": "/foreign"}, []string{}} {
		var status *control.StatusError
		if err := wire.Call(context.Background(), MethodReclamationTarget, payload, nil); !errors.As(err, &status) || status.Code != "invalid_argument" {
			t.Fatal("selection accepted", err)
		}
	}
	if calls != 0 {
		t.Fatal("selected request reached service")
	}
	client, _ := NewClient(path)
	target, err := client.ReclamationTarget(context.Background())
	if err != nil || target != rpcReclaimTarget || calls != 1 {
		t.Fatal(target, err, calls)
	}
}
func TestReclamationTargetWithholdsInvalidIdentityAndBackendErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		target reclamation.WSLTarget
		err    error
	}{
		{"invalid", reclamation.WSLTarget{RegistrationID: "token=private"}, nil},
		{"unavailable", rpcReclaimTarget, errors.New("token=private")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := doctorTestSocket(t, func(s *control.Server) {
				if err := RegisterReclamationTarget(s, reclamationTargetFunc(func(context.Context) (reclamation.WSLTarget, error) { return tc.target, tc.err })); err != nil {
					t.Fatal(err)
				}
			})
			client, _ := NewClient(path)
			target, err := client.ReclamationTarget(context.Background())
			if err == nil || target != (reclamation.WSLTarget{}) || strings.Contains(err.Error(), "private") {
				t.Fatal(target, err)
			}
		})
	}
}
func TestReclamationTargetClientRejectsUnprovenResponse(t *testing.T) {
	path := doctorTestSocket(t, func(s *control.Server) {
		if err := s.Register(MethodReclamationTarget, func(context.Context, json.RawMessage) (any, error) { return nil, nil }); err != nil {
			t.Fatal(err)
		}
	})
	client, _ := NewClient(path)
	if _, err := client.ReclamationTarget(context.Background()); err == nil {
		t.Fatal("empty identity accepted")
	}
}
