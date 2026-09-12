package incus

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"strings"
	"testing"
)

func TestPersistentCopyUsesOfflineBtrfsAndNewOwnership(t *testing.T) {
	source := core.PersistentResource{ID: "oci:source", Kind: OCIStoreKind, Owner: strings.Repeat("a", 32), NativeRef: "pool/haco-persistent-" + strings.Repeat("a", 32)}
	target := core.PersistentResource{ID: "oci:target", Kind: OCIStoreKind, Owner: strings.Repeat("b", 32), NativeRef: "pool/haco-persistent-" + strings.Repeat("b", 32), CopySource: source.Ref()}
	for _, scenario := range []string{"ok", "empty", "busy", "foreign", "duplicate", "malformed", "truncated", "not-btrfs", "bad-idmap", "missing", "lost-reply"} {
		t.Run(scenario, func(t *testing.T) {
			copies := 0
			runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
				if name != "incus" || args[0] != "query" {
					t.Fatalf("unexpected provider command %s %v", name, args)
				}
				if args[1] == "/1.0/storage-pools/pool" {
					driver := "btrfs"
					if scenario == "not-btrfs" {
						driver = "dir"
					}
					return host.Result{Stdout: `{"name":"pool","driver":"` + driver + `"}`}, nil
				}
				if args[1] == "-X" {
					copies++
					var request struct {
						Name   string
						Config map[string]string
						Source struct {
							Type, Name, Pool, Project string
							VolumeOnly                bool `json:"volume_only"`
						}
					}
					if err := json.Unmarshal([]byte(args[len(args)-1]), &request); err != nil {
						t.Fatal(err)
					}
					if request.Name != "haco-persistent-"+target.Owner || request.Config["user.hacocoon.owner"] != target.Owner || request.Config["user.hacocoon.resource"] != target.ID || request.Source.Type != "copy" || request.Source.Name != "haco-persistent-"+source.Owner || request.Source.Pool != "pool" || request.Source.Project != "hacocoon" || !request.Source.VolumeOnly {
						t.Fatalf("unsafe copy request %+v", request)
					}
					if request.Config["security.unmapped"] != "" || request.Config["user.secret"] != "" {
						t.Fatal("inherited arbitrary source configuration")
					}
					if scenario != "empty" && request.Config["volatile.idmap.last"] != `[{"Isuid":true,"Hostid":1000000,"Nsid":0,"Maprange":65536}]` {
						t.Fatal("idmap not preserved")
					}
					if scenario == "lost-reply" {
						return host.Result{}, errors.New("lost reply")
					}
					return host.Result{}, nil
				}
				if scenario == "malformed" {
					return host.Result{Stdout: "{}"}, nil
				}
				if scenario == "truncated" {
					return host.Result{Stdout: "[]", StdoutTruncated: true}, nil
				}
				if scenario == "missing" {
					return host.Result{Stdout: "[]"}, nil
				}
				v := persistentVolumeObservation{Name: "haco-persistent-" + source.Owner, Type: "custom", ContentType: "filesystem", Config: map[string]string{"user.hacocoon.owner": source.Owner, "user.hacocoon.resource": source.ID, "user.hacocoon.kind": source.Kind, "security.unmapped": "true", "user.secret": "not-forwarded"}}
				if scenario != "empty" {
					v.Config["volatile.idmap.last"] = `[{"Isuid":true,"Hostid":1000000,"Nsid":0,"Maprange":65536}]`
				}
				if scenario == "bad-idmap" {
					v.Config["volatile.idmap.last"] = "invalid"
				}
				if scenario == "busy" {
					v.UsedBy = []string{"/1.0/instances/foreign"}
				}
				if scenario == "foreign" {
					v.Config["user.hacocoon.owner"] = "foreign"
				}
				volumes := []persistentVolumeObservation{v}
				if scenario == "duplicate" {
					volumes = append(volumes, v)
				}
				data, _ := json.Marshal(volumes)
				return host.Result{Stdout: string(data)}, nil
			}}
			err := (&PersistentResourceBackend{Runtime: New(runner)}).Copy(context.Background(), source, target)
			succeeds := scenario == "ok" || scenario == "empty"
			if (err == nil) != succeeds {
				t.Fatalf("unexpected result: %v", err)
			}
			expectedCalls := 0
			if succeeds || scenario == "lost-reply" {
				expectedCalls = 1
			}
			if copies != expectedCalls {
				t.Fatalf("copies=%d", copies)
			}
		})
	}
}
