package incus

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestContainerdMaintenanceRequiresOwnedGenerationAndSingleMount(t *testing.T) {
	for _, mode := range []string{"ok", "host", "source", "owner", "generation", "shared", "wrong-project", "readonly", "shadow", "privileged", "privileged-alias", "missing-profiles", "truncated", "startup"} {
		t.Run(mode, func(t *testing.T) {
			generation, _ := core.NewEnvironmentInstanceID()
			resource := core.PersistentResource{ID: "oci:maintenance", Owner: strings.Repeat("a", 32), Kind: OCIStoreKind, State: "ready", NativeRef: "pool/haco-persistent-" + strings.Repeat("a", 32)}
			ref := "haco-maintenance"
			if mode == "host" {
				ref = trustedHostName
			}
			if mode == "source" {
				resource.SourceOnly = true
			}
			started := false
			runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
				if name != "incus" {
					t.Fatal(name)
				}
				if args[0] == "exec" {
					started = true
					if args[len(args)-1] != containerdMaintenanceStart {
						t.Fatal(args)
					}
					if mode == "startup" {
						return host.Result{ExitCode: 42}, nil
					}
					return host.Result{}, nil
				}
				var value any
				if strings.HasPrefix(args[1], "/1.0/storage-pools/") {
					owner := resource.Owner
					if mode == "owner" {
						owner = strings.Repeat("b", 32)
					}
					used := []string{"/1.0/instances/" + ref + "?project=hacocoon"}
					if mode == "shared" {
						used = append(used, used[0])
					}
					if mode == "wrong-project" {
						used[0] += "&project=foreign"
					}
					value = []persistentVolumeObservation{{Name: "haco-persistent-" + resource.Owner, Type: "custom", ContentType: "filesystem", UsedBy: used, Config: map[string]string{"user.hacocoon.owner": owner, "user.hacocoon.resource": resource.ID, "user.hacocoon.kind": resource.Kind}}}
				} else {
					config := map[string]string{environmentInstanceKey: generation, managedEnvironmentMarkerKey: managedEnvironmentMarkerValue}
					if mode == "generation" {
						config[environmentInstanceKey] = "env-" + strings.Repeat("b", 32)
					}
					if mode == "privileged-alias" {
						config["security.privileged"] = "1"
					}
					if mode == "privileged" {
						config["security.privileged"] = "true"
					}
					devices := map[string]map[string]string{"persistent-resource": {"type": "disk", "pool": "pool", "source": "haco-persistent-" + resource.Owner, "path": OCIStorePath}}
					if mode == "readonly" {
						devices["persistent-resource"]["readonly"] = "true"
					}
					if mode == "shadow" {
						devices["shadow"] = map[string]string{"path": OCIStorePath + "/containerd"}
					}
					profiles := []string{}
					if mode == "missing-profiles" {
						profiles = nil
					}
					value = snapshotInstanceObservation{Profiles: profiles, Name: ref, Type: "container", Status: "Running", Config: config, ExpandedConfig: config, Devices: devices, ExpandedDevices: devices}
				}
				data, _ := json.Marshal(value)
				return host.Result{Stdout: string(data), StdoutTruncated: mode == "truncated"}, nil
			}}
			p, err := NewSandboxProvider(New(runner))
			if err != nil {
				t.Fatal(err)
			}
			err = p.startContainerdMaintenance(context.Background(), ref, generation, resource)
			if mode == "ok" {
				if err != nil || !started {
					t.Fatal(err, started)
				}
			} else if err == nil || (mode != "startup" && started) {
				t.Fatal("unsafe maintenance accepted", err, started)
			}
		})
	}
}
func TestContainerdMaintenanceShellSyntax(t *testing.T) {
	path, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash unavailable")
	}
	cmd := exec.Command(path, "-n")
	cmd.Stdin = strings.NewReader(containerdMaintenanceStart)
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
}
