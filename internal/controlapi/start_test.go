package controlapi

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type startService struct{}

func (startService) Start(_ context.Context, name string) error {
	if name == "recovery" {
		return core.ErrRecoveryRequired
	}
	return nil
}
func TestStartRPCReportsRecoveryAndAcceptsExistingEnvironment(t *testing.T) {
	socket := doctorTestSocket(t, func(s *control.Server) {
		if err := RegisterStart(s, startService{}); err != nil {
			t.Fatal(err)
		}
	})
	client, err := NewClient(socket)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.StartEnvironment(context.Background(), "dev"); err != nil {
		t.Fatal(err)
	}
	err = client.StartEnvironment(context.Background(), "recovery")
	var status *control.StatusError
	if !errors.As(err, &status) || status.Code != "recovery_required" {
		t.Fatalf("err=%v", err)
	}
}
