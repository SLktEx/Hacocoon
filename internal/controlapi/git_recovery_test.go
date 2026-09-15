package controlapi

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
)

func TestGitRecoveryHandlerRefusesUnknownFieldsAndTrailingRequests(t *testing.T) {
	for _, reconcile := range []bool{false, true} {
		for _, payload := range []string{`{"environment":"dev","remote":"https://github.com/other/repo.git"}`, `{"environment":"dev"}{}`, strings.Repeat(" ", 4097)} {
			_, err := gitStatusHandler(nil, reconcile)(context.Background(), json.RawMessage(payload))
			if !errors.Is(err, control.ErrInvalidArgument) {
				t.Fatalf("payload reached broker: %v", err)
			}
		}
	}
}

type emptyGitAuditHistory struct{}

func (emptyGitAuditHistory) StreamAudit(context.Context, int64, func(core.CapabilityAuditEvent, int64) error) (int64, error) {
	return 0, nil
}

func TestGitRecoveryControllerRoutesStatusAndRefusesUnrecordedRead(t *testing.T) {
	broker := gitrepo.NewBroker(nil, nil, "")
	broker.AuditHistory = emptyGitAuditHistory{}
	path := doctorTestSocket(t, func(server *control.Server) {
		if err := RegisterRepositories(server, nil, broker); err != nil {
			t.Fatal(err)
		}
	})
	client, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	status, err := client.GitPushStatus(context.Background(), GitStatusRequest{Environment: "dev"}, false)
	if err != nil || status.Found {
		t.Fatalf("%+v %v", status, err)
	}
	var statusErr *control.StatusError
	if _, err := client.GitPushStatus(context.Background(), GitStatusRequest{Environment: "dev"}, true); !errors.As(err, &statusErr) || statusErr.Code != "recovery_required" {
		t.Fatal(err)
	}
}
