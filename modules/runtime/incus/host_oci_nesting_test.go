package incus

import (
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"strings"
	"testing"
)

func TestHostNestingRequiresOwnedUnprivilegedSourceAndVerifiesMutation(t *testing.T) {
	for _, mode := range []string{"ok", "reuse", "foreign-role", "inherited-role", "privileged", "profiles", "missing-profiles", "vm", "wrong-source", "extra-consumer", "wrong-path", "pending", "paused", "creating", "failed-read", "truncated-read", "failed-set", "unconfirmed-set", "changed-owner"} {
		t.Run(mode, func(t *testing.T) {
			source := core.PersistentResource{ID: "oci-source:host", Kind: OCIStoreKind, Owner: strings.Repeat("a", 32), NativeRef: "pool/haco-persistent-" + strings.Repeat("a", 32), SourceOnly: true, State: "ready"}
			volume := persistentVolumeObservation{Name: "haco-persistent-" + source.Owner, Type: "custom", ContentType: "filesystem", Config: map[string]string{"user.hacocoon.owner": source.Owner, "user.hacocoon.resource": source.ID, "user.hacocoon.kind": source.Kind, "user.hacocoon.source-only": "true"}, UsedBy: []string{"/1.0/instances/haco-host?project=hacocoon"}}
			i := hostOCICopyInstance{Name: trustedHostName, Type: "container", Profiles: []string{}, StatusCode: 103, Config: map[string]string{trustedHostRoleKey: trustedHostRoleValue, hostOCIStoreKey: source.Owner}, LocalConfig: map[string]string{trustedHostRoleKey: trustedHostRoleValue, hostOCIStoreKey: source.Owner}, Devices: map[string]map[string]string{"oci": {"type": "disk", "pool": "pool", "source": volume.Name, "path": OCIStorePath}}}
			switch mode {
			case "reuse":
				i.Config["security.nesting"] = "true"
				i.LocalConfig["security.nesting"] = "true"
			case "foreign-role":
				i.Config[trustedHostRoleKey] = "foreign"
			case "inherited-role":
				delete(i.LocalConfig, trustedHostRoleKey)
			case "privileged":
				i.Config["security.privileged"] = "true"
			case "profiles":
				i.Profiles = []string{"default"}
			case "missing-profiles":
				i.Profiles = nil
			case "vm":
				i.Type = "virtual-machine"
			case "wrong-source":
				volume.Config["user.hacocoon.owner"] = "foreign"
			case "extra-consumer":
				volume.UsedBy = append(volume.UsedBy, volume.UsedBy[0])
			case "wrong-path":
				i.Devices["oci"]["path"] = "/other"
			case "pending":
				i.Config[hostOCICopyKey] = "pending"
			case "paused":
				i.StatusCode = 110
			case "creating":
				source.State = "creating"
			}
			sets := 0
			encode := func(value any) host.Result { data, _ := json.Marshal(value); return host.Result{Stdout: string(data)} }
			runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
				if name != "incus" {
					t.Fatal(name)
				}
				switch {
				case args[0] == "query" && strings.Contains(args[1], "/volumes/custom?"):
					return encode([]persistentVolumeObservation{volume}), nil
				case args[0] == "query" && strings.Contains(args[1], "/instances/"):
					if mode == "failed-read" {
						return host.Result{ExitCode: 1}, nil
					}
					if mode == "truncated-read" {
						return host.Result{StdoutTruncated: true}, nil
					}
					return encode(i), nil
				case args[0] == "exec":
					if args[len(args)-1] != hostOCILayoutVerify {
						t.Fatal(args)
					}
					return host.Result{}, nil
				case args[0] == "config":
					if strings.Join(args, " ") != "config set haco-host security.nesting true --project hacocoon" {
						t.Fatal(args)
					}
					sets++
					if mode == "failed-set" {
						return host.Result{ExitCode: 1}, nil
					}
					if mode != "unconfirmed-set" {
						i.Config["security.nesting"] = "true"
						i.LocalConfig["security.nesting"] = "true"
					}
					if mode == "changed-owner" {
						delete(i.LocalConfig, trustedHostRoleKey)
					}
					return host.Result{}, nil
				default:
					t.Fatalf("unexpected %v", args)
					return host.Result{}, nil
				}
			}}
			err := (&PersistentResourceBackend{Runtime: New(runner)}).EnableHostOCI(context.Background(), source)
			if mode == "ok" || mode == "reuse" {
				want := 1
				if mode == "reuse" {
					want = 0
				}
				if err != nil || sets != want {
					t.Fatalf("%v sets=%d", err, sets)
				}
			} else {
				if err == nil {
					t.Fatal("unsafe configuration accepted")
				}
				if mode != "failed-set" && mode != "unconfirmed-set" && mode != "changed-owner" && sets != 0 {
					t.Fatal("mutation before ownership verification")
				}
			}
		})
	}
}
