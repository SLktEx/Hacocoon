package incus

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestResumeValidatesNetworkBeforeStartingAndPreservesRuntime(t *testing.T) {
	for _, scenario := range []string{"stopped", "running", "foreign", "drift", "unknown", "foreign-network", "isolation-off", "post-start-drift", "missing-guard", "running-missing-guard", "guard-drift"} {
		t.Run(scenario, func(t *testing.T) {
			state := "STOPPED"
			if scenario == "running" || scenario == "running-missing-guard" {
				state = "RUNNING"
			}
			if scenario == "unknown" {
				state = "FROZEN"
			}
			starts, stops := 0, 0
			guardPresent := scenario != "missing-guard" && scenario != "running-missing-guard"
			guardWrites := 0
			runner := &fakeRunner{run: func(_ context.Context, _ int, command string, args []string) (host.Result, error) {
				if command == "sudo" && len(args) > 6 && args[2] == "nft" && args[6] == routedSandboxGuardTable("haco-demo") {
					if args[3] == "list" && !guardPresent {
						return host.Result{Stderr: "No such file or directory"}, errors.New("missing volatile guard")
					}
					if args[3] == "list" && scenario == "guard-drift" {
						return host.Result{Stdout: "table inet guard {}"}, nil
					}
					if args[3] == "add" {
						guardPresent = true
						guardWrites++
						return host.Result{}, nil
					}
					if args[3] == "delete" {
						t.Fatal("resume deleted an existing guard")
					}
				}
				if args[0] == "start" {
					if scenario == "missing-guard" && guardWrites != 5 {
						t.Fatal("started before source guard was restored")
					}
					starts++
					state = "RUNNING"
					return host.Result{}, nil
				}
				if args[0] == "stop" {
					stops++
					state = "STOPPED"
					return host.Result{}, nil
				}
				if args[0] == "delete" || args[0] == "init" {
					t.Fatal("resume recreated runtime")
				}
				if args[0] == "list" && len(args) > 1 && args[1] == "haco-demo" {
					return host.Result{Stdout: state}, nil
				}
				if len(args) > 3 && args[0] == "config" && args[1] == "get" && args[3] == managedEnvironmentMarkerKey {
					marker := managedEnvironmentMarkerValue
					if scenario == "foreign" {
						marker = "foreign"
					}
					return host.Result{Stdout: marker}, nil
				}
				if strings.Contains(strings.Join(args, " "), "device get haco-demo eth0 network") && (scenario == "drift" || (scenario == "post-start-drift" && starts > 0)) {
					return host.Result{Stdout: "foreign"}, nil
				}
				if len(args) > 3 && args[0] == "network" && args[1] == "get" && args[3] == environmentNetworkOwnerKey {
					value := environmentNetworkOwnerValue
					if scenario == "foreign-network" {
						value = "foreign"
					}
					return host.Result{Stdout: value}, nil
				}
				if len(args) > 5 && args[0] == "config" && args[1] == "device" && args[5] == "security.port_isolation" {
					value := "true"
					if scenario == "isolation-off" {
						value = "false"
					}
					return host.Result{Stdout: value}, nil
				}
				if result, ok := sandboxNetworkResult(args); ok {
					return result, nil
				}
				return host.Result{}, errors.New("unexpected provider call: " + strings.Join(args, " "))
			}}
			p, err := NewSandboxProvider(New(runner))
			if err != nil {
				t.Fatal(err)
			}
			err = p.StartEnvironment(context.Background(), "haco-demo")
			wantFailure := scenario != "stopped" && scenario != "running" && scenario != "missing-guard"
			if (err != nil) != wantFailure {
				t.Fatalf("err=%v", err)
			}
			wantStarts := 0
			if scenario == "stopped" || scenario == "post-start-drift" || scenario == "missing-guard" {
				wantStarts = 1
			}
			if scenario != "missing-guard" && guardWrites != 0 {
				t.Fatal("unexpected guard repair")
			}
			if starts != wantStarts {
				t.Fatalf("starts=%d err=%v", starts, err)
			}
			if scenario == "post-start-drift" && stops != 1 {
				t.Fatal("did not stop after failed verification")
			}
		})
	}
}
