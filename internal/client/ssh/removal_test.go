//go:build linux

package sshclient

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestRemovePreservesUnownedAndNewerSSHEntries(t *testing.T) {
	t.Setenv("WSL_DISTRO_NAME", "Hacocoon")
	for _, scenario := range []string{"owned", "missing", "user-config", "bad-json", "wrong-environment", "other-distro", "newer-grant", "oversized", "symlink", "hardlink"} {
		t.Run(scenario, func(t *testing.T) {
			home := t.TempDir()
			dir := filepath.Join(home, ".ssh", "hacocoon")
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			connection, err := (&fakeController{}).PrepareEnvironmentSSH(context.Background(), "dev", core.SSHAccessRequest{PublicKey: testKey})
			if err != nil {
				t.Fatal(err)
			}
			meta := saved{Runtime: "runtime", Connection: connection}
			grant := connection.ID
			switch scenario {
			case "wrong-environment":
				meta.Connection.Target.Environment = "other"
			case "other-distro":
				meta.Distro = "Other"
			case "newer-grant":
				grant = "revoked-old-grant"
			}
			data, err := json.Marshal(meta)
			if err != nil {
				t.Fatal(err)
			}
			body := prefix + string(data) + "\nHost haco-dev\n"
			switch scenario {
			case "user-config":
				body = "Host personal\n  HostName keep.example\n"
			case "bad-json":
				body = prefix + "{bad\n"
			case "oversized":
				body = strings.Repeat("x", 1024*1024+1)
			}
			path := filepath.Join(dir, "dev.conf")
			victim := filepath.Join(home, "personal")
			if err := os.WriteFile(victim, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "missing":
			case "symlink":
				err = os.Symlink(victim, path)
			case "hardlink":
				err = os.Link(victim, path)
			default:
				err = os.WriteFile(path, []byte(body), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			err = Remove(context.Background(), Desktop{Home: home}, "dev", grant)
			wantError := scenario == "user-config" || scenario == "bad-json" || scenario == "wrong-environment" || scenario == "oversized" || scenario == "symlink" || scenario == "hardlink"
			if (err != nil) != wantError {
				t.Fatalf("removal outcome: %v", err)
			}
			got, readErr := os.ReadFile(path)
			if scenario == "owned" || scenario == "missing" {
				if !errors.Is(readErr, os.ErrNotExist) {
					t.Fatalf("owned target retained: %v", readErr)
				}
			} else if readErr != nil || string(got) != body {
				t.Fatalf("unowned/newer entry changed: %v", readErr)
			}
			if got, err := os.ReadFile(victim); err != nil || string(got) != body {
				t.Fatal("unrelated file changed", err)
			}
		})
	}
}

func TestSSHMutationsRejectUnsafeNamesBeforeFilesystemAccess(t *testing.T) {
	d := Desktop{Home: filepath.Join(t.TempDir(), "absent")}
	for _, name := range []string{"", "../victim", "/absolute", "-option", "a\nHost *", strings.Repeat("a", 58)} {
		if err := Remove(context.Background(), d, name, ""); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatal(name, err)
		}
		if err := CleanupEnvironment(context.Background(), nil, d, name); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatal(name, err)
		}
		if _, err := Setup(context.Background(), nil, d, name); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatal(name, err)
		}
	}
	if _, err := os.Stat(d.Home); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("invalid name touched filesystem")
	}
}

func TestSSHDirectoryAndLockRefuseUnsafeOwnershipShapes(t *testing.T) {
	for _, scenario := range []string{"ssh-symlink", "ssh-writable", "managed-symlink", "managed-public", "lock-hardlink", "locked"} {
		t.Run(scenario, func(t *testing.T) {
			home := t.TempDir()
			sshDir := filepath.Join(home, ".ssh")
			if err := os.Mkdir(sshDir, 0700); err != nil {
				t.Fatal(err)
			}
			managed := filepath.Join(sshDir, "hacocoon")
			if err := os.Mkdir(managed, 0700); err != nil {
				t.Fatal(err)
			}
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			switch scenario {
			case "ssh-symlink":
				must(os.Rename(sshDir, filepath.Join(home, "unrelated")))
				must(os.Symlink(filepath.Join(home, "unrelated"), sshDir))
			case "ssh-writable":
				must(os.Chmod(sshDir, 0777))
			case "managed-symlink":
				must(os.Rename(managed, filepath.Join(home, "unrelated")))
				must(os.Symlink(filepath.Join(home, "unrelated"), managed))
			case "managed-public":
				must(os.Chmod(managed, 0755))
			case "lock-hardlink":
				victim := filepath.Join(home, "personal")
				must(os.WriteFile(victim, []byte("keep"), 0600))
				must(os.Link(victim, filepath.Join(managed, "setup.lock")))
			case "locked":
				first, err := openFiles(context.Background(), home, false)
				must(err)
				defer first.close()
			}
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			err := Remove(ctx, Desktop{Home: home}, "dev", "")
			if err == nil {
				t.Fatal("accepted unsafe or already locked SSH directory")
			}
			if scenario == "locked" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("lost cancellation: %v", err)
			}
		})
	}
}
