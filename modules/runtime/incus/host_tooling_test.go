package incus

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/hostsetup"
)

func TestHostToolingPythonContracts(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux Host provisioner")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	output, err := exec.Command(python, "-I", "testdata/host_tooling_test.py", "host_tooling.py").CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
}

func TestHostToolingWaitsForGuestManagerBeforeDispatch(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux Host provisioner")
	}
	for _, ready := range []bool{false, true} {
		dir := t.TempDir()
		probe := filepath.Join(dir, "busctl")
		body := "#!/bin/sh\nexit 1\n"
		if ready {
			body = "#!/bin/sh\nexit 0\n"
		}
		if err := os.WriteFile(probe, []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
		script := strings.ReplaceAll(hostToolingSystemdReady, "/usr/bin/busctl", probe)
		script = strings.ReplaceAll(script, "sleep 0.5", ":")
		out, err := exec.Command("/bin/sh", "-ec", script, "fixture", "/bin/echo", "dispatched").CombinedOutput()
		if ready {
			if err != nil || string(out) != "dispatched\n" {
				t.Fatalf("%v: %s", err, out)
			}
		} else if err == nil || len(out) != 0 {
			t.Fatalf("dispatch before readiness: %v %s", err, out)
		}
	}
}

func TestHostToolingRequiresOwnedSourceAndReportsBoundedStages(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux Host operation lock")
	}
	for _, mode := range []string{"ok", "foreign", "profile", "privileged", "not-nested", "pending-copy", "creating", "packages-fail", "tooling-fail", "services-fail", "truncated", "canceled"} {
		t.Run(mode, func(t *testing.T) {
			source := core.PersistentResource{ID: "oci-source:host", Kind: OCIStoreKind, Owner: strings.Repeat("a", 32), NativeRef: "pool/haco-persistent-" + strings.Repeat("a", 32), SourceOnly: true, State: "ready"}
			volume := persistentVolumeObservation{Name: "haco-persistent-" + source.Owner, Type: "custom", ContentType: "filesystem", Config: map[string]string{"user.hacocoon.owner": source.Owner, "user.hacocoon.resource": source.ID, "user.hacocoon.kind": source.Kind, "user.hacocoon.source-only": "true"}, UsedBy: []string{"/1.0/instances/haco-host?project=hacocoon"}}
			i := hostOCICopyInstance{Name: trustedHostName, Type: "container", Profiles: []string{}, StatusCode: 103, Config: map[string]string{trustedHostRoleKey: trustedHostRoleValue, hostOCIStoreKey: source.Owner, "security.nesting": "true"}, LocalConfig: map[string]string{trustedHostRoleKey: trustedHostRoleValue, hostOCIStoreKey: source.Owner, "security.nesting": "true"}, Devices: map[string]map[string]string{"oci": {"type": "disk", "pool": "pool", "source": volume.Name, "path": OCIStorePath}}}
			switch mode {
			case "foreign":
				delete(i.LocalConfig, trustedHostRoleKey)
			case "profile":
				i.Profiles = []string{"default"}
			case "privileged":
				i.Config["security.privileged"] = "true"
			case "not-nested":
				delete(i.LocalConfig, "security.nesting")
			case "pending-copy":
				i.Config[hostOCICopyKey] = "pending"
			case "creating":
				source.State = "creating"
			}
			var stages []string
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			encode := func(value any) host.Result { data, _ := json.Marshal(value); return host.Result{Stdout: string(data)} }
			runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
				if name != "incus" {
					t.Fatal(name)
				}
				switch {
				case args[0] == "query" && strings.Contains(args[1], "/volumes/custom?"):
					return encode([]persistentVolumeObservation{volume}), nil
				case args[0] == "query" && strings.Contains(args[1], "/instances/"):
					return encode(i), nil
				case args[0] == "exec" && args[len(args)-1] == hostOCILayoutVerify:
					return host.Result{}, nil
				case args[0] == "exec" && args[len(args)-2] == hostToolingScript:
					stage := args[len(args)-1]
					stages = append(stages, stage)
					joined := strings.Join(args, " ")
					for _, required := range []string{"exec haco-host --project hacocoon -- /bin/sh -ec", hostToolingSystemdReady, "/usr/bin/systemd-run", "--expand-environment=no", "--property=KillMode=control-group", "--property=RuntimeMaxSec=600s", "--unit=hacocoon-host-tooling"} {
						if !strings.Contains(joined, required) {
							t.Fatalf("missing %s", required)
						}
					}
					if mode == "canceled" {
						cancel()
						return host.Result{}, nil
					}
					if mode == "truncated" {
						return host.Result{StdoutTruncated: true}, nil
					}
					if mode == strings.TrimPrefix(stage, "host_")+"-fail" {
						return host.Result{Stderr: "secret helper data", ExitCode: 1}, errors.New("secret backend data")
					}
					return host.Result{}, nil
				default:
					t.Fatalf("unexpected provider call %v", args)
					return host.Result{}, nil
				}
			}}
			err := (&PersistentResourceBackend{Runtime: New(runner)}).ProvisionHostTools(ctx, source)
			if mode == "ok" {
				if err != nil || !reflect.DeepEqual(stages, []string{"host_packages", "host_tooling", "host_services"}) {
					t.Fatalf("stages=%v err=%v", stages, err)
				}
			} else {
				if err == nil {
					t.Fatal("unsafe/failed setup succeeded")
				}
				if strings.Contains(err.Error(), "secret") {
					t.Fatal("helper output leaked")
				}
				if len(stages) > 0 {
					stage, _ := hostsetup.Details(err)
					if stage != stages[len(stages)-1] {
						t.Fatalf("failure stage=%s operations=%v", stage, stages)
					}
				} else if strings.HasSuffix(mode, "-fail") {
					t.Fatal("failure injection not reached")
				}
			}
		})
	}
}
