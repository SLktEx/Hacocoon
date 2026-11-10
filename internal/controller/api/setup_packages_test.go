package controlapi

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/host/recipes"
	"github.com/SLktEx/Hacocoon/internal/host/setup"
	"github.com/SLktEx/Hacocoon/internal/logging"
)

func TestPackageSetupFailureReachesProgressAndJournalOnce(t *testing.T) {
	old := logging.Root()
	var logs bytes.Buffer
	logging.SetRoot(slog.New(slog.NewJSONHandler(&logs, nil)))
	defer logging.SetRoot(old)
	calls := 0
	path := doctorTestSocket(t, func(s *control.Server) {
		if err := RegisterSetup(s, setupServiceFunc(func(ctx context.Context) error {
			calls++
			return hostsetup.Step(ctx, "host_storage", func() error {
				return hostsetup.Step(ctx, "host_packages", func() error {
					return errors.Join(&hostsetup.PackageFailure{Phase: "install", Kind: "timeout"}, errors.New("SECRET-backend-output"))
				})
			})
		})); err != nil {
			t.Fatal(err)
		}
	})
	c, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	var events []hostsetup.Event
	err = c.SetupHostProgress(context.Background(), recipes.Update{}, func(_ string, e hostsetup.Event) { events = append(events, e) })
	var status *control.StatusError
	if !errors.As(err, &status) || status.Code != "setup_failed" || strings.Contains(err.Error(), "SECRET") {
		t.Fatal("lost sanitized package failure", err)
	}
	seen := false
	for _, event := range events {
		if event.Stage == "host_packages" && event.State == "failed" && event.Reason == "packages_install_timeout" {
			seen = true
		}
	}
	if !seen || calls != 1 || strings.Contains(logs.String(), "SECRET") || !strings.Contains(logs.String(), "packages_install_timeout") || strings.Count(logs.String(), `"level":"ERROR"`) != 1 {
		t.Fatal("package failure lost, repeated or exposed output", calls, logs.String())
	}
}
