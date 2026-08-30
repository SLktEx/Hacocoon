package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestControllerClientModeParsing(t *testing.T) {
	t.Setenv(controllerClientModeEnv, "")
	active, err := controllerClientMode()
	if err != nil || active {
		t.Fatalf("empty mode = %t, %v", active, err)
	}

	t.Setenv(controllerClientModeEnv, controllerClientModeValue)
	active, err = controllerClientMode()
	if err != nil || !active {
		t.Fatalf("controller mode = %t, %v", active, err)
	}

	t.Setenv(controllerClientModeEnv, "local")
	if _, err := controllerClientMode(); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("invalid mode error = %v", err)
	}
}

func TestControllerClientModeRequiresExplicitSocket(t *testing.T) {
	t.Setenv(controllerClientModeEnv, controllerClientModeValue)
	t.Setenv("HACO_CONTROL_SOCKET", "")
	handled, err := handleControllerClientModeArgs(
		context.Background(), []string{"base", "list"},
		func() (environmentControllerClient, error) { return &fakeEnvironmentController{}, nil },
		bytes.NewBuffer(nil), bytes.NewBuffer(nil), bytes.NewBuffer(nil),
	)
	if !handled || !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("handled=%t err=%v", handled, err)
	}
}

func TestControllerClientModeLeavesFirstClassEnvNamespaceToEnvClient(t *testing.T) {
	t.Setenv(controllerClientModeEnv, controllerClientModeValue)
	t.Setenv("HACO_CONTROL_SOCKET", "/tmp/hacocoon-control-test.sock")
	handled, err := handleControllerClientModeArgs(
		context.Background(), []string{"env", "list"},
		func() (environmentControllerClient, error) { return &fakeEnvironmentController{}, nil },
		bytes.NewBuffer(nil), bytes.NewBuffer(nil), bytes.NewBuffer(nil),
	)
	if err != nil || handled {
		t.Fatalf("handled=%t err=%v", handled, err)
	}
}

func TestControllerClientModeLeavesHostShellToHostClient(t *testing.T) {
	t.Setenv(controllerClientModeEnv, controllerClientModeValue)
	t.Setenv("HACO_CONTROL_SOCKET", "/tmp/hacocoon-control-test.sock")
	factoryCalls := 0
	handled, err := handleControllerClientModeArgs(
		context.Background(), []string{"host", "shell"},
		func() (environmentControllerClient, error) {
			factoryCalls++
			return &fakeEnvironmentController{}, nil
		},
		bytes.NewBuffer(nil), bytes.NewBuffer(nil), bytes.NewBuffer(nil),
	)
	if err != nil || handled {
		t.Fatalf("handled=%t err=%v", handled, err)
	}
	if factoryCalls != 0 {
		t.Fatalf("controller factory calls = %d, want 0", factoryCalls)
	}
}

func TestControllerClientModeRefusesHostEnsureLocalFallback(t *testing.T) {
	t.Setenv(controllerClientModeEnv, controllerClientModeValue)
	t.Setenv("HACO_CONTROL_SOCKET", "/tmp/hacocoon-control-test.sock")
	handled, err := handleControllerClientModeArgs(
		context.Background(), []string{"host", "ensure"},
		func() (environmentControllerClient, error) { return &fakeEnvironmentController{}, nil },
		bytes.NewBuffer(nil), bytes.NewBuffer(nil), bytes.NewBuffer(nil),
	)
	if !handled || !errors.Is(err, core.ErrUnsupported) || !strings.Contains(err.Error(), "Physical Host-local") {
		t.Fatalf("handled=%t err=%v", handled, err)
	}
}

func TestControllerClientModeRoutesLegacyEnvironmentAliasToController(t *testing.T) {
	t.Setenv(controllerClientModeEnv, controllerClientModeValue)
	t.Setenv("HACO_CONTROL_SOCKET", "/tmp/hacocoon-control-test.sock")
	client := &fakeEnvironmentController{created: core.Environment{
		Name:       "demo",
		Workspace:  core.Workspace{Path: "/work/demo"},
		AccessMode: core.WorkspaceReadWrite,
	}}
	var out bytes.Buffer
	handled, err := handleControllerClientModeArgs(
		context.Background(), []string{"create", "--workspace", "/work/demo", "demo"},
		func() (environmentControllerClient, error) { return client, nil },
		bytes.NewBuffer(nil), &out, bytes.NewBuffer(nil),
	)
	if err != nil || !handled {
		t.Fatalf("handled=%t err=%v", handled, err)
	}
	if client.createdRequest.Name != "demo" || client.createdRequest.WorkspacePath != "/work/demo" {
		t.Fatalf("request = %+v", client.createdRequest)
	}
	if out.String() != "demo\t/work/demo\trw\n" {
		t.Fatalf("output = %q", out.String())
	}
}

func TestControllerClientModeRefusesUnmigratedLocalCommand(t *testing.T) {
	t.Setenv(controllerClientModeEnv, controllerClientModeValue)
	t.Setenv("HACO_CONTROL_SOCKET", "/tmp/hacocoon-control-test.sock")
	factoryCalls := 0
	handled, err := handleControllerClientModeArgs(
		context.Background(), []string{"base", "list"},
		func() (environmentControllerClient, error) {
			factoryCalls++
			return &fakeEnvironmentController{}, nil
		},
		bytes.NewBuffer(nil), bytes.NewBuffer(nil), bytes.NewBuffer(nil),
	)
	if !handled || !errors.Is(err, core.ErrUnsupported) {
		t.Fatalf("handled=%t err=%v", handled, err)
	}
	if factoryCalls != 0 {
		t.Fatalf("controller factory calls = %d, want 0", factoryCalls)
	}
}
