//go:build linux

package sshclient

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func changedHostKey(t *testing.T) string {
	t.Helper()
	wire, err := base64.StdEncoding.DecodeString(strings.Fields(testKey)[1])
	if err != nil {
		t.Fatal(err)
	}
	wire[len(wire)-1] ^= 1
	return "ssh-ed25519 " + base64.StdEncoding.EncodeToString(wire)
}

func TestSetupReuseRefusesChangedHostKeyBeforeRewritingFiles(t *testing.T) {
	desktop, controller := preparedSSHFiles(t)
	before := sshFileSnapshot(t, desktop.Home)
	controller.connection.HostPublicKey = changedHostKey(t)
	alias, err := Setup(context.Background(), controller, desktop, "dev")
	if err == nil || alias != "" || controller.count != 1 {
		t.Fatal("changed host key was treated as a reusable connection", alias, err, controller.count)
	}
	if !reflect.DeepEqual(before, sshFileSnapshot(t, desktop.Home)) {
		t.Fatal("host-key drift rewrote pinned or personal files")
	}
}

type setupContractController struct {
	*fakeController
	statusCalls       int
	statusErrorAt     int
	statusError       error
	replacement       bool
	connectionsError  error
	preparationError  error
	preparationChecks func()
}

func (c *setupContractController) EnvironmentStatus(ctx context.Context, name string) (core.EnvironmentStatus, error) {
	c.statusCalls++
	if c.statusCalls == c.statusErrorAt {
		return core.EnvironmentStatus{}, c.statusError
	}
	status, err := c.fakeController.EnvironmentStatus(ctx, name)
	if c.replacement && c.statusCalls > 1 {
		status.Environment.RuntimeRef = "replacement"
	}
	return status, err
}

func (c *setupContractController) EnvironmentConnections(context.Context, string) ([]core.ClientConnection, error) {
	return nil, c.connectionsError
}

func (c *setupContractController) PrepareEnvironmentSSH(ctx context.Context, name string, request core.SSHAccessRequest) (core.ClientConnection, error) {
	if c.preparationError != nil {
		return core.ClientConnection{}, c.preparationError
	}
	connection, err := c.fakeController.PrepareEnvironmentSSH(ctx, name, request)
	if err != nil {
		return connection, err
	}
	c.connection.ID, c.connection.Target.Grant = "ssh-next", "ssh-next"
	if c.preparationChecks != nil {
		c.preparationChecks()
	}
	return c.connection, nil
}

func TestSetupObservationFailurePreservesPinnedConnection(t *testing.T) {
	for _, boundary := range []string{"status", "connections", "prepare"} {
		t.Run(boundary, func(t *testing.T) {
			desktop, base := preparedSSHFiles(t)
			unavailable := errors.New("controller observation failed")
			controller := &setupContractController{fakeController: base}
			switch boundary {
			case "status":
				controller.statusErrorAt, controller.statusError = 1, unavailable
			case "connections":
				controller.connectionsError = unavailable
			case "prepare":
				controller.preparationError = unavailable
			}
			before := sshFileSnapshot(t, desktop.Home)
			alias, err := Setup(context.Background(), controller, desktop, "dev")
			if !errors.Is(err, unavailable) || alias != "" || base.count != 1 || base.starts != 0 {
				t.Fatal("failed observation returned an alias or prepared a connection", alias, err, base.count, base.starts)
			}
			if !reflect.DeepEqual(before, sshFileSnapshot(t, desktop.Home)) {
				t.Fatal("observation failure rewrote owned files")
			}
		})
	}
}

func TestSetupFailureAfterPreparationRetainsExactGrantAndExistingFiles(t *testing.T) {
	for _, defect := range []string{"status-failed", "runtime-changed", "host-key-changed", "instance-changed", "malformed-target", "missing-distribution", "linked-pin", "linked-fragment"} {
		t.Run(defect, func(t *testing.T) {
			desktop, base := preparedSSHFiles(t)
			controller := &setupContractController{fakeController: base}
			observationError := errors.New("post-prepare observation failed")
			if defect == "status-failed" {
				controller.statusErrorAt, controller.statusError = 2, observationError
			}
			if defect == "runtime-changed" {
				controller.replacement = true
			}
			if defect == "missing-distribution" {
				desktop.Windows = true
				t.Setenv("WSL_DISTRO_NAME", "")
			}
			before := sshFileSnapshot(t, desktop.Home)
			controller.preparationChecks = func() {
				switch defect {
				case "host-key-changed":
					base.connection.HostPublicKey = changedHostKey(t)
				case "instance-changed":
					base.connection.Target.Instance = "env-" + strings.Repeat("f", 32)
				case "malformed-target":
					base.connection.Target = nil
				case "linked-pin", "linked-fragment":
					name := knownPath(saved{Runtime: "runtime-owned", Connection: base.connection})
					if defect == "linked-fragment" {
						name = "hacocoon/dev.conf"
					}
					path := filepath.Join(desktop.Home, ".ssh", name)
					if err := os.Rename(path, path+".retained"); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(path+".retained", path); err != nil {
						t.Fatal(err)
					}
					before = sshFileSnapshot(t, desktop.Home)
				}
			}
			alias, err := Setup(context.Background(), controller, desktop, "dev")
			if !errors.Is(err, core.ErrRecoveryRequired) || !strings.Contains(err.Error(), `"ssh-next"`) || alias != "" || base.count != 2 || base.starts != 0 {
				t.Fatal("prepared grant lost its recovery identity", alias, err, base.count, base.starts)
			}
			if defect == "status-failed" && !errors.Is(err, observationError) {
				t.Fatal("post-prepare failure cause was lost", err)
			}
			if !reflect.DeepEqual(before, sshFileSnapshot(t, desktop.Home)) {
				t.Fatal("failed preparation overwrote existing owned or personal data")
			}
		})
	}
}
