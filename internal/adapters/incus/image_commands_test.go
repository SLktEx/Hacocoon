package incus

import (
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"strings"
	"testing"
)

func TestImageTagUsesLocalFingerprintAndRemoveOnlyAlias(t *testing.T) {
	t.Setenv(baseConfigEnv, "")
	aliases := []baseAlias{{Name: builtBasePrefix + "source", Target: testFingerprintA, Type: "container", Description: builtBaseDescription}}
	original := baseImage{Fingerprint: testFingerprintA, Type: "container", Properties: map[string]string{"user.hacocoon.kind": "base-image", "user.hacocoon.base-name": "source", "user.hacocoon.build-instance": testEnvironmentInstance}}
	posts, deletes := 0, 0
	p, _ := NewBaseProvider(New(&fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		var value any
		switch {
		case len(args) > 3 && args[2] == "GET" && strings.Contains(args[3], "/aliases?"):
			value = aliases
		case len(args) > 3 && args[2] == "GET" && strings.Contains(args[3], "/images/"+testFingerprintA):
			value = original
		case len(args) > 5 && args[2] == "POST" && strings.Contains(args[3], "/aliases?"):
			var body map[string]string
			if json.Unmarshal([]byte(args[5]), &body) != nil || body["target"] != testFingerprintA || body["name"] != builtBasePrefix+"target" || body["description"] != imageTagDescription {
				t.Fatal(args)
			}
			posts++
			aliases = append(aliases, baseAlias{Name: body["name"], Target: body["target"], Description: body["description"], Type: "container"})
			return host.Result{}, nil
		case len(args) > 3 && args[2] == "DELETE" && strings.Contains(args[3], "/aliases/"+builtBasePrefix+"target?"):
			deletes++
			aliases = aliases[:1]
			return host.Result{}, nil
		default:
			t.Fatal("unexpected mutation/provider call", args)
		}
		raw, _ := json.Marshal(value)
		return host.Result{Stdout: string(raw)}, nil
	}}))
	ctx := context.Background()
	got, err := p.TagImage(ctx, "source", "target")
	if err != nil || got.Revision != core.BaseRevision("sha256:"+testFingerprintA) || posts != 1 {
		t.Fatal(got, err, posts)
	}
	resolved, err := p.resolveBase(ctx, "target")
	if err != nil || resolved.pinnedSource != "local:"+testFingerprintA {
		t.Fatal(resolved, err)
	}
	if _, err := p.TagImage(ctx, "source", "target"); err == nil || posts != 1 {
		t.Fatal("duplicate tag replaced", err, posts)
	}
	if err := p.RemoveImageTag(ctx, "target"); err != nil || deletes != 1 {
		t.Fatal(err, deletes)
	}
	if _, err := p.resolveBase(ctx, "source"); err != nil {
		t.Fatal("source image changed", err)
	}
}
func TestCommitCopiesRootfsWithoutChangingSource(t *testing.T) {
	for _, status := range []string{"Running", "Stopped"} {
		t.Run(status, func(t *testing.T) {
			t.Setenv(baseConfigEnv, "")
			_, source := rootfsFixture()
			source.Status = status
			var target *snapshotInstanceObservation
			aliases := []baseAlias{}
			var published baseImage
			deletes := 0
			p, _ := NewBaseProvider(New(&fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				var value any
				switch {
				case args[0] == "delete":
					if target == nil || args[1] != target.Name {
						t.Fatal("wrong cleanup", args)
					}
					deletes++
					target = nil
					return host.Result{}, nil
				case len(args) == 2 && strings.Contains(args[1], "/instances?"):
					list := []snapshotInstanceObservation{source}
					if target != nil {
						list = append(list, *target)
					}
					value = list
				case len(args) == 2 && args[1] == "/1.0/storage-pools/pool":
					value = map[string]string{"name": "pool", "driver": "btrfs"}
				case len(args) > 3 && args[2] == "POST" && args[3] == "--wait":
					var req struct {
						Name     string
						Config   map[string]string
						Devices  map[string]map[string]string
						Profiles []string
						Source   map[string]any
					}
					if json.Unmarshal([]byte(args[len(args)-1]), &req) != nil {
						t.Fatal(args)
					}
					if req.Source["source"] != source.Name || req.Source["live"] != false || req.Devices["workspace"]["type"] != "none" || req.Config["environment.TEST_TOKEN"] != "" || len(req.Profiles) != 0 {
						t.Fatal("copied workspace or authority", req)
					}
					target = &snapshotInstanceObservation{Name: req.Name, Type: "container", Status: "Stopped", Config: req.Config, ExpandedConfig: req.Config, Devices: req.Devices, ExpandedDevices: req.Devices}
					return host.Result{}, nil
				case len(args) > 3 && args[2] == "GET" && strings.Contains(args[3], "/aliases?"):
					value = aliases
				case len(args) > 5 && args[2] == "POST" && strings.Contains(args[3], "/images?"):
					var req struct {
						Properties map[string]string
						Source     map[string]string
						Aliases    []map[string]string
					}
					if json.Unmarshal([]byte(args[5]), &req) != nil || target == nil || req.Source["name"] != target.Name || req.Properties["user.hacocoon.snapshot-owner"] == "" {
						t.Fatal(args)
					}
					published = baseImage{Fingerprint: testFingerprintA, Type: "container", Properties: req.Properties}
					aliases = []baseAlias{{Name: req.Aliases[0]["name"], Description: req.Aliases[0]["description"], Target: testFingerprintA, Type: "container"}}
					return host.Result{}, nil
				case len(args) > 3 && args[2] == "GET" && strings.Contains(args[3], "/images/"+testFingerprintA):
					value = published
				default:
					t.Fatal("unexpected source mutation", args)
				}
				raw, _ := json.Marshal(value)
				return host.Result{Stdout: string(raw)}, nil
			}}))
			work := core.Workspace{ID: core.WorkspaceID("workspace:managed:" + strings.Repeat("b", 32)), Path: "managed:source"}
			env := core.Environment{Name: "source", RuntimeRef: source.Name, Workspace: work}
			lease := core.WorkspaceLease{EnvironmentID: env.Name, RuntimeRef: env.RuntimeRef, InstanceID: testEnvironmentInstance, WorkspaceID: work.ID, SourcePath: work.Path, State: core.WorkspaceLeaseActive}
			got, err := p.CommitImage(context.Background(), env, lease, "committed")
			if err != nil || got.Name != "committed" || target != nil || deletes != 1 || source.Status != status {
				t.Fatal(got, err, deletes)
			}
		})
	}
}
