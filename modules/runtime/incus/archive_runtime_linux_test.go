//go:build linux

package incus

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"strings"
	"testing"
)

func TestImportedRuntimeRecordsBeforeCurrentConfiguration(t *testing.T) {
	for _, mode := range []string{"ok", "receipt", "configuration", "ssh", "ssh-exit", "init-lost", "init-exit"} {
		t.Run(mode, func(t *testing.T) {

			copied, recorded, renewed, guard := false, false, false, false
			values := map[string]string{}

			runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				if copied && !recorded {
					t.Fatal("provider call before ownership receipt", args)
				}
				if args[0] == "image" || (len(args) > 2 && args[0] == "profile" && args[2] == "default") {
					t.Fatal("Base/default dependency", args)
				}
				if args[0] == "delete" {
					t.Fatal("provider duplicated caller cleanup")
				}
				if args[0] == "init" {
					if args[1] != "local:"+strings.Repeat("a", 64) || !strings.Contains(strings.Join(args, " "), "--no-profiles") {
						t.Fatal("source/profiles", args)
					}
					joined := strings.Join(args, " ")
					for _, required := range []string{"boot.autostart=false", "security.privileged=false", "security.nesting=false", environmentInstanceKey + "=env-" + strings.Repeat("f", 32)} {
						if !strings.Contains(joined, required) {
							t.Fatal("missing current boundary", required)
						}
					}
					if mode == "init-lost" {
						return host.Result{}, errors.New("lost reply")
					}
					if mode == "init-exit" {
						return host.Result{ExitCode: 1}, nil
					}
					copied = true
					return host.Result{}, nil
				}
				if len(args) > 6 && args[2] == "nft" && args[6] == routedSandboxGuardTable("haco-demo") {
					if args[3] == "list" && !guard {
						return host.Result{Stderr: "No such file or directory"}, errors.New("missing")
					}
					if args[3] == "add" && args[4] == "table" {
						guard = true
					}
				}
				if result, ok := sandboxNetworkResult(args); ok {
					return result, nil
				}
				if len(args) >= 4 && args[0] == "config" && args[1] == "set" {
					if mode == "configuration" {
						return host.Result{}, errors.New("configuration failed")
					}
					parts := strings.SplitN(args[3], "=", 2)
					values[parts[0]] = parts[1]
					return host.Result{}, nil
				}
				if len(args) >= 4 && args[0] == "config" && args[1] == "get" {
					return host.Result{Stdout: values[args[3]]}, nil
				}
				if args[0] == "exec" && args[len(args)-1] == freshGuestSSHIdentity {
					renewed = true
					if mode == "ssh-exit" {
						return host.Result{ExitCode: 1}, nil
					}
					if mode == "ssh" {
						return host.Result{}, errors.New("reset failed")
					}
				}
				return host.Result{}, nil
			}}
			p, err := NewSandboxProvider(New(runner))
			if err != nil {
				t.Fatal(err)
			}
			p.setRootPool("fixture")
			result, err := p.createEnvironmentFromImportedImage(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", InstanceID: "env-" + strings.Repeat("f", 32), WorkspacePath: "/tmp/work"}, strings.Repeat("a", 64), func(created core.EnvironmentRuntime) error {
				if !copied || recorded || created.Ref != "haco-demo" || created.Base != nil {
					t.Fatal("bad receipt", created)
				}
				recorded = true
				if mode == "receipt" {
					return errors.New("record failed")
				}
				return nil
			})
			if mode == "init-lost" || mode == "init-exit" {
				if recorded || !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatal("uncertain native init accepted", err)
				}
				return
			}
			if !copied || !recorded || result.Ref != "haco-demo" {
				t.Fatal("ownership missing", result, err)
			}
			if mode == "ok" {
				if err != nil || !renewed {
					t.Fatal("current configuration incomplete", err)
				}
			} else if err == nil {
				t.Fatal("failure published")
			}
		})
	}
}
