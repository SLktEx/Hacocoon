package incus

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestSnapshotInspectionReadsExactVolumeWithoutMutation(t *testing.T) {
	for _, mode := range []string{"present", "absent", "busy", "foreign", "duplicate", "malformed", "null", "truncated", "unavailable", "bad-binding"} {
		t.Run(mode, func(t *testing.T) {
			p := snapshotVolumeFixture("work")
			v := persistentVolumeObservation{Name: p.target(), Type: "custom", ContentType: "filesystem", Config: p.targetConfig(), UsedBy: []string{}}
			if mode == "foreign" {
				v.Config["user.hacocoon.owner"] = strings.Repeat("c", 32)
			}
			if mode == "busy" {
				v.UsedBy = []string{"/synthetic/private?token=do-not-emit"}
			}
			values := []persistentVolumeObservation{v}
			if mode == "absent" {
				values = []persistentVolumeObservation{}
			}
			if mode == "duplicate" {
				values = append(values, v)
			}
			data, _ := json.Marshal(values)
			calls := 0
			runtime := &Runtime{project: "hacocoon", runner: &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
				calls++
				if name != "incus" || !reflect.DeepEqual(args, []string{"query", "/1.0/storage-pools/pool/volumes/custom?project=hacocoon&recursion=1"}) {
					t.Fatal("non-GET or wrong target", name, args)
				}
				if mode == "unavailable" {
					return host.Result{ExitCode: 1}, errors.New("private-output")
				}
				if mode == "malformed" {
					return host.Result{Stdout: "bad json"}, nil
				}
				if mode == "null" {
					return host.Result{Stdout: "null"}, nil
				}
				return host.Result{Stdout: string(data), StdoutTruncated: mode == "truncated"}, nil
			}}}
			c, err := runtime.snapshotComponent(snapshotBinding{Version: 1, Project: "hacocoon", Volume: &p})
			if err != nil {
				t.Fatal(err)
			}
			if mode == "bad-binding" {
				c.NativeRef = "volume/foreign/target"
			}
			got, err := runtime.InspectSnapshotComponent(context.Background(), c)
			valid := mode == "present" || mode == "absent" || mode == "busy"
			if (err == nil) != valid {
				t.Fatal(got, err)
			}
			if mode == "bad-binding" {
				if calls != 0 || got.Object != "" {
					t.Fatal("invalid binding reached provider", got, calls)
				}
				return
			}
			if calls != 1 || got.Object != c.NativeRef || got.Project != "hacocoon" || got.Pool != "pool" || got.Backing != "uninspected" {
				t.Fatal(got, calls)
			}
			if valid {
				wantPresence, wantCheck := "present", "ready"
				if mode == "absent" {
					wantPresence, wantCheck = "absent", "absent"
				}
				if mode == "busy" {
					wantCheck = "busy"
					if got.References == nil || *got.References != 1 {
						t.Fatal(got)
					}
				}
				if got.Presence != wantPresence || got.Check != wantCheck {
					t.Fatal(got)
				}
			}
			if mode == "duplicate" || mode == "malformed" || mode == "null" || mode == "truncated" || mode == "unavailable" {
				if got.Presence != "unknown" {
					t.Fatal("uncertainty reported as presence", got)
				}
			}
			encoded, _ := json.Marshal(got)
			if strings.Contains(string(encoded), "do-not-emit") || strings.Contains(string(encoded), "private-output") || strings.Contains(string(encoded), "config") {
				t.Fatal("private provider data projected")
			}
		})
	}
}

func TestSnapshotInspectionRootfsUsesDeletionOwnershipChecks(t *testing.T) {
	for _, mode := range []string{"present", "absent", "foreign", "running", "duplicate"} {
		t.Run(mode, func(t *testing.T) {
			p, _ := rootfsFixture()
			devices := map[string]map[string]string{"root": {"type": "disk", "path": "/", "pool": p.Pool}}
			v := snapshotInstanceObservation{Name: p.target(), Type: "container", Status: "Stopped", Config: p.config(), ExpandedConfig: p.config(), Devices: devices, ExpandedDevices: devices, Profiles: []string{}}
			if mode == "foreign" {
				v.Config["user.hacocoon.owner"] = "foreign"
			}
			if mode == "running" {
				v.Status = "Running"
			}
			values := []snapshotInstanceObservation{v}
			if mode == "absent" {
				values = []snapshotInstanceObservation{}
			}
			if mode == "duplicate" {
				values = append(values, v)
			}
			data, _ := json.Marshal(values)
			runtime := &Runtime{project: "hacocoon", runner: &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
				if name != "incus" || !reflect.DeepEqual(args, []string{"query", "/1.0/instances?project=hacocoon&recursion=1"}) {
					t.Fatal("mutation", name, args)
				}
				return host.Result{Stdout: string(data)}, nil
			}}}
			c, err := runtime.snapshotComponent(snapshotBinding{Version: 1, Project: "hacocoon", Rootfs: &p})
			if err != nil {
				t.Fatal(err)
			}
			got, err := runtime.InspectSnapshotComponent(context.Background(), c)
			if (err == nil) != (mode == "present" || mode == "absent") {
				t.Fatal(got, err)
			}
			if mode == "foreign" && !errors.Is(err, core.ErrCapabilityStale) {
				t.Fatal(err)
			}
			if mode == "absent" && got.Presence != "absent" {
				t.Fatal(got)
			}
			if mode == "present" && (got.Presence != "present" || got.Check != "ready") {
				t.Fatal(got)
			}
		})
	}
}
