//go:build linux

package sshclient

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type cleanupController struct {
	*fakeController
	status func() error
}

func (c cleanupController) EnvironmentStatus(context.Context, string) (core.EnvironmentStatus, error) {
	return core.EnvironmentStatus{}, c.status()
}

func TestDeletedEnvironmentCleanupProtectsConcurrentSetup(t *testing.T) {
	t.Setenv("WSL_DISTRO_NAME", "")
	for _, scenario := range []string{"deleted", "replacement-before-inspection", "replacement-during-inspection", "unavailable", "recovery"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			home := t.TempDir()
			dir := filepath.Join(home, ".ssh", "hacocoon")
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			fake := &fakeController{}
			old, err := fake.PrepareEnvironmentSSH(ctx, "dev", core.SSHAccessRequest{PublicKey: testKey})
			if err != nil {
				t.Fatal(err)
			}
			write := func(name string, conn core.ClientConnection) {
				t.Helper()
				data, err := json.Marshal(saved{Runtime: "owned", Connection: conn})
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(dir, name+".conf"), []byte(prefix+string(data)+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			write("dev", old)
			write("other", old) // The targeted cleanup must never inspect this entry.
			newer := old
			newTarget := *old.Target
			newTarget.Instance = "env-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
			newTarget.Grant = "ssh-newer"
			newer.ID, newer.Target = newTarget.Grant, &newTarget
			if scenario == "replacement-before-inspection" {
				write("dev", newer)
				fake.connection = newer
			}
			controller := cleanupController{fakeController: fake, status: func() error {
				switch scenario {
				case "replacement-before-inspection":
					return nil
				case "replacement-during-inspection":
					write("dev", newer)
					return core.ErrNotFound
				case "unavailable":
					return core.ErrRuntimeUnavailable
				case "recovery":
					return core.ErrRecoveryRequired
				default:
					return core.ErrNotFound
				}
			}}
			err = CleanupEnvironment(ctx, controller, Desktop{Home: home}, "dev")
			if (scenario == "unavailable" || scenario == "recovery") != (err != nil) {
				t.Fatalf("cleanup error: %v", err)
			}
			data, readErr := os.ReadFile(filepath.Join(dir, "dev.conf"))
			if scenario == "deleted" {
				if !errors.Is(readErr, os.ErrNotExist) {
					t.Fatalf("deleted entry retained: %v", readErr)
				}
			} else {
				if readErr != nil {
					t.Fatalf("live or ambiguous entry removed: %v", readErr)
				}
				var meta saved
				if err = json.Unmarshal(data[len(prefix):], &meta); err != nil {
					t.Fatal(err)
				}
				if scenario == "replacement-before-inspection" || scenario == "replacement-during-inspection" {
					if meta.Connection.ID != newer.ID {
						t.Fatal("concurrent setup lost")
					}
				}
			}
			if _, err = os.Stat(filepath.Join(dir, "other.conf")); err != nil {
				t.Fatal("unrelated entry removed")
			}
		})
	}
}
