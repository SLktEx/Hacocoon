package controlapi

import (
	"context"
	"errors"
	"io"
	"net"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/base/build"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	runapp "github.com/SLktEx/Hacocoon/internal/env/run"
	"github.com/SLktEx/Hacocoon/internal/events"
	"github.com/SLktEx/Hacocoon/internal/host/recipes"
	"github.com/SLktEx/Hacocoon/internal/policy"
	"github.com/SLktEx/Hacocoon/internal/storage/reclamation"
	"github.com/SLktEx/Hacocoon/internal/workspace/workflow"
)

func TestUnavailableControllerCannotCreateReceiptsOrReplay(t *testing.T) {
	unavailable := errors.New("controller endpoint unavailable")
	var attempts atomic.Int32
	client, err := NewClientWithDialer(func(context.Context) (net.Conn, error) {
		attempts.Add(1)
		return nil, unavailable
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	spec := runapp.Spec{WorkspacePath: "/retained", Argv: []string{"true"}}
	for _, tc := range []struct {
		name string
		call func() (any, error)
	}{
		{"base-build", func() (any, error) { return client.BuildBase(ctx, basebuild.Definition{Name: "tools", Run: "true"}) }},
		{"environment-copy", func() (any, error) {
			return client.CopyEnvironment(ctx, EnvironmentCopyRequest{Source: "dev", Target: "copy"})
		}},
		{"snapshot-create", func() (any, error) {
			return client.Snapshot(ctx, SnapshotRequest{Operation: "create", Environment: "dev"})
		}},
		{"snapshot-restore", func() (any, error) {
			return client.RestoreSnapshot(ctx, SnapshotRestoreRequest{ID: "snap-" + strings.Repeat("a", 32), Environment: "restored"})
		}},
		{"workspace-reference", func() (any, error) {
			return client.WorkspaceWorkflow(ctx, WorkflowRequest{Operation: "reference", Reference: &workflow.Reference{Name: "saved"}})
		}},
		{"reclaim-linux", func() (any, error) {
			return client.ReclaimLinux(ctx, reclamation.WSLTarget{RegistrationID: "{11111111-1111-4111-8111-111111111111}", InstallationID: "11111111-1111-4111-8111-111111111111"})
		}},
		{"environment-connections", func() (any, error) { return client.EnvironmentConnections(ctx, "dev") }},
		{"prepare-forward", func() (any, error) { return client.PrepareEnvironmentForward(ctx, "dev", "127.0.0.1", 8080) }},
		{"open-forward", func() (any, error) {
			return client.OpenEnvironmentForward(ctx, core.EnvironmentTCPForward{Environment: "dev", Instance: "env-" + strings.Repeat("a", 32), Address: "127.0.0.1", Port: 8080})
		}},
		{"run", func() (any, error) { return client.Run(ctx, spec) }},
		{"run-process", func() (any, error) {
			return client.RunStream(ctx, spec, false, strings.NewReader(""), io.Discard, io.Discard)
		}},
		{"setup", func() (any, error) { return nil, client.SetupHost(ctx, recipes.Update{}) }},
		{"setup-progress", func() (any, error) { return nil, client.SetupHostProgress(ctx, recipes.Update{}, nil) }},
		{"export", func() (any, error) { return client.ExportEnvironment(ctx, "dev", io.Discard) }},
		{"capability", func() (any, error) {
			return client.RequestCapability(ctx, core.CapabilityRequest{Capability: "local.echo", Action: "echo"}, nil)
		}},
		{"saved-git-decision", func() (any, error) {
			return client.DecideGitWithSavedChoice(ctx, "reviewed", true, capability.AllowGlobal)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := attempts.Load()
			result, err := tc.call()
			if !errors.Is(err, unavailable) || (result != nil && !reflect.ValueOf(result).IsZero()) || attempts.Load() != before+1 {
				t.Fatal("lost connection error, fabricated result or replayed request", result, err, attempts.Load()-before)
			}
		})
	}
	before := attempts.Load()
	cursor, err := client.StreamEvents(ctx, 17, func(events.Event) error { t.Error("event fabricated without controller"); return nil })
	if !errors.Is(err, unavailable) || cursor != 17 || attempts.Load() != before+1 {
		t.Fatal("connection refusal lost event resume position", cursor, err)
	}
}

func TestClientConstructionUsesSelectedController(t *testing.T) {
	for _, path := range []string{"", " \t\n"} {
		if client, err := NewClient(path); client != nil || !errors.Is(err, control.ErrInvalidArgument) {
			t.Fatal("empty endpoint accepted", client, err)
		}
	}
	if client, err := NewClientWithDialer(nil); client != nil || !errors.Is(err, control.ErrInvalidArgument) {
		t.Fatal("missing transport accepted", client, err)
	}
	path := doctorTestSocket(t, func(s *control.Server) {
		if err := Register(s, &fakeEnvironments{}, fakeClients{}); err != nil {
			t.Fatal(err)
		}
	})
	t.Setenv("HACO_CONTROL_SOCKET", path)
	client := NewDefaultClient()
	response, err := client.Ping(context.Background())
	if err != nil || response.ProtocolVersion != control.ProtocolVersion {
		t.Fatal("default client did not contact selected controller", response, err)
	}
}

func TestDefaultClientDoesNotFallbackAfterEndpointSelection(t *testing.T) {
	path := doctorTestSocket(t, func(s *control.Server) {
		if err := Register(s, &fakeEnvironments{}, fakeClients{}); err != nil {
			t.Fatal(err)
		}
	})
	t.Setenv("HACO_CONTROL_SOCKET", filepath.Join(t.TempDir(), "missing.sock"))
	missing := NewDefaultClient()
	// A later environment change cannot redirect an already selected client to
	// a different authority, even when its original controller is unavailable.
	t.Setenv("HACO_CONTROL_SOCKET", path)
	if response, err := missing.Ping(context.Background()); !errors.Is(err, control.ErrUnavailable) || response.ProtocolVersion != 0 {
		t.Fatal("unavailable endpoint returned a receipt or changed authority", response, err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := missing.Ping(canceled); !errors.Is(err, context.Canceled) {
		t.Fatal("connection attempt lost cancellation", err)
	}
	current := NewDefaultClient()
	if response, err := current.Ping(context.Background()); err != nil || response.ProtocolVersion != control.ProtocolVersion {
		t.Fatal("new client did not use the newly selected endpoint", response, err)
	}
}
