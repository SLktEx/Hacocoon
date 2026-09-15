package controlapi

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type stopService struct {
	mu    sync.Mutex
	names []string
}

func (s *stopService) Stop(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.names = append(s.names, name)
	switch name {
	case "dev":
		return nil
	case "missing":
		return core.ErrNotFound
	case "busy":
		return core.ErrStorageBusy
	default:
		return errors.Join(core.ErrNotFound, core.ErrRecoveryRequired)
	}
}

func TestStopWirePreservesTargetAndRecoveryObligations(t *testing.T) {
	service := &stopService{}
	path := doctorTestSocket(t, func(s *control.Server) {
		if err := RegisterStop(s, service); err != nil {
			t.Fatal(err)
		}
	})
	client, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, tc := range []struct{ name, code string }{{"dev", ""}, {"missing", "not_found"}, {"busy", "busy"}, {"recovery", "recovery_required"}, {"", "invalid_argument"}} {
		err := client.StopEnvironment(ctx, tc.name)
		if tc.code == "" {
			if err != nil {
				t.Fatal(err)
			}
			continue
		}
		var status *control.StatusError
		if !errors.As(err, &status) || status.Code != tc.code {
			t.Fatal("stop status hid failure or recovery", tc.name, err)
		}
	}
	if err := client.wire.Call(ctx, MethodEnvironmentStop, "invalid request", nil); err == nil {
		t.Fatal("invalid stop request accepted")
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if !reflect.DeepEqual(service.names, []string{"dev", "missing", "busy", "recovery"}) {
		t.Fatal("stop lost target or dispatched invalid request", service.names)
	}
}
