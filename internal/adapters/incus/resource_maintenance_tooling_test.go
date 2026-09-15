package incus

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestMaintenanceToolingRequiresOwnedEmptyTargetAndVerifiedTransfer(t *testing.T) {
	for _, mode := range []string{"ok", "host", "generation", "changed-generation", "privileged", "attached", "unknown-mount", "profiles", "hash", "truncated", "transfer", "cleanup", "source", "source-link"} {
		t.Run(mode, func(t *testing.T) {
			directory := t.TempDir()
			for _, name := range []string{"containerd", "ctr", "nerdctl"} {
				if err := os.WriteFile(filepath.Join(directory, name), []byte("trusted-"+name), 0755); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "source-link" {
				if err := os.Remove(filepath.Join(directory, "ctr")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(directory, "containerd"), filepath.Join(directory, "ctr")); err != nil {
					t.Fatal(err)
				}
			}
			ref := "haco-maintenance"
			if mode == "host" {
				ref = trustedHostName
			}
			generation := "env-" + strings.Repeat("a", 32)
			queries, pushes, getters, releases := 0, 0, 0, 0
			prepared, installed := false, false
			runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
				if name != "incus" {
					t.Fatal("executed Host runtime", name)
				}
				if args[0] == "query" {
					queries++
					g := generation
					if mode == "generation" || (mode == "changed-generation" && queries > 1) {
						g = "env-" + strings.Repeat("b", 32)
					}
					config := map[string]string{environmentInstanceKey: g, managedEnvironmentMarkerKey: managedEnvironmentMarkerValue}
					if mode == "privileged" {
						config["security.privileged"] = "true"
					}
					devices := map[string]map[string]string{"root": {"type": "disk", "path": "/"}}
					if mode == "attached" {
						devices["persistent-resource"] = map[string]string{"type": "disk", "path": OCIStorePath}
					}
					if mode == "unknown-mount" {
						devices["other"] = map[string]string{"type": "disk", "path": "/var/lib"}
					}
					profiles := []string{}
					if mode == "profiles" {
						profiles = nil
					}
					data, _ := json.Marshal(snapshotInstanceObservation{Name: ref, Type: "container", Status: "Running", Profiles: profiles, Config: config, ExpandedConfig: config, Devices: devices, ExpandedDevices: devices})
					return host.Result{Stdout: string(data)}, nil
				}
				if args[0] == "file" {
					if !prepared {
						t.Fatal("transfer before private guest staging")
					}
					pushes++
					if mode == "transfer" {
						return host.Result{ExitCode: 1}, errors.New("transfer failed")
					}
					return host.Result{}, nil
				}
				if args[0] == "exec" {
					last := args[len(args)-1]
					if last == maintenanceToolingPrepare {
						prepared = true
						return host.Result{}, nil
					}
					if last == maintenanceToolingInstall {
						if pushes != 3 {
							t.Fatal("incomplete install")
						}
						installed = true
						return host.Result{}, nil
					}
					if args[len(args)-3] == "sha256sum" {
						digest := fmt.Sprintf("%x", sha256.Sum256([]byte("trusted-"+filepath.Base(last))))
						if mode == "hash" {
							digest = strings.Repeat("0", 64)
						}
						return host.Result{Stdout: digest + "  " + last + "\n", StdoutTruncated: mode == "truncated"}, nil
					}
				}
				t.Fatal("unexpected command", args)
				return host.Result{}, nil
			}}
			runtime := New(runner)
			runtime.ConfigureMaintenanceTooling(func(context.Context) (string, func() error, error) {
				getters++
				if mode == "source" {
					return "", nil, core.ErrRuntimeUnavailable
				}
				return directory, func() error {
					releases++
					if mode == "cleanup" {
						return errors.New("private tools cleanup failed")
					}
					return nil
				}, nil
			})
			p, err := NewSandboxProvider(runtime)
			if err != nil {
				t.Fatal(err)
			}
			err = p.provisionMaintenanceTooling(context.Background(), ref, generation)
			if (mode == "ok") != (err == nil) {
				t.Fatal("wrong outcome", mode, err)
			}
			if mode == "ok" && (!installed || pushes != 3 || releases != 1) {
				t.Fatal("incomplete successful installation")
			}
			if getters > 0 && mode != "source" && releases != 1 {
				t.Fatal("private files were not released")
			}
			if mode != "ok" && mode != "cleanup" && installed {
				t.Fatal("failed preparation installed tools")
			}
			if mode == "attached" || mode == "generation" || mode == "host" || mode == "privileged" || mode == "unknown-mount" || mode == "profiles" {
				if getters != 0 || pushes != 0 {
					t.Fatal("unsafe target acquired tools")
				}
			}
		})
	}
}
