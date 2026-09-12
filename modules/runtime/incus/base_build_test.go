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

func TestBuiltBaseUsesOwnedNativeImageAndPinnedRevision(t *testing.T) {
	for _, bad := range []string{"", "public", "owner", "foreign", "duplicate", "truncated", "wrong-fingerprint"} {
		t.Run(bad, func(t *testing.T) {
			t.Setenv(baseConfigEnv, "")
			a := baseAlias{Name: builtBasePrefix + "tools", Target: testFingerprintA, Description: builtBaseDescription, Type: "container"}
			im := baseImage{Fingerprint: testFingerprintA, Type: "container", Properties: map[string]string{"user.hacocoon.kind": "base-image", "user.hacocoon.base-name": "tools", "user.hacocoon.build-instance": "env-" + strings.Repeat("a", 32)}}
			switch bad {
			case "public":
				im.Public = true
			case "owner":
				im.Properties["user.hacocoon.build-instance"] = ""
			case "foreign":
				a.Description = "foreign"
			case "wrong-fingerprint":
				im.Fingerprint = testFingerprintB
			}
			runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				var value any = im
				if strings.Contains(strings.Join(args, " "), "/aliases?") {
					list := []baseAlias{a}
					if bad == "duplicate" {
						list = append(list, a)
					}
					value = list
				}
				data, _ := json.Marshal(value)
				return host.Result{Stdout: string(data), StdoutTruncated: bad == "truncated"}, nil
			}}
			p, _ := NewBaseProvider(New(runner))
			got, err := p.resolveBase(context.Background(), "tools")
			if bad != "" {
				if err == nil {
					t.Fatal("unowned image accepted", got)
				}
				return
			}
			if err != nil || got.pinnedSource != "local:"+testFingerprintA || !got.built {
				t.Fatal(got, err)
			}
		})
	}
}
func TestPublishBaseRecordsOwnershipBeforeVerificationAndPreservesImages(t *testing.T) {
	for _, failure := range []string{"", "publish", "verify", "alias", "foreign"} {
		t.Run(failure, func(t *testing.T) {
			t.Setenv(baseConfigEnv, "")
			work, _ := core.NewTemporaryWorkspace()
			id := "env-" + strings.Repeat("a", 32)
			env := core.Environment{Name: "builder", RuntimeRef: "haco-builder", Workspace: work}
			lease := core.WorkspaceLease{EnvironmentID: env.Name, RuntimeRef: env.RuntimeRef, InstanceID: id, WorkspaceID: work.ID, SourcePath: work.Path, State: core.WorkspaceLeaseActive}
			published := false
			pointer := false
			ownerRecorded := false
			runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				joined := strings.Join(args, " ")
				if strings.HasPrefix(joined, "file push") {
					if joined != "file push /dev/null haco-builder/etc/machine-id --project hacocoon --uid 0 --gid 0 --mode 0644" {
						t.Fatal("unsafe file source", joined)
					}
					return host.Result{}, nil
				}
				if strings.Contains(joined, "delete") {
					t.Fatal("image or builder guessed cleanup")
				}
				if strings.HasPrefix(joined, "config get") {
					return host.Result{Stdout: id}, nil
				}
				if args[0] == "list" {
					return host.Result{Stdout: "haco-builder,STOPPED\n"}, nil
				}
				if strings.Contains(joined, "/metadata?") {
					return host.Result{Stdout: `{"architecture":"x86_64","templates":{}}`}, nil
				}
				if strings.HasPrefix(joined, "query -X POST /1.0/images?") {
					var body struct {
						Public     bool              `json:"public"`
						Properties map[string]string `json:"properties"`
						Source     map[string]string `json:"source"`
					}
					if json.Unmarshal([]byte(args[5]), &body) != nil || body.Public || body.Properties["user.hacocoon.build-instance"] != id || body.Source["name"] != env.RuntimeRef {
						t.Fatal("ownership not atomic", joined)
					}
					ownerRecorded = true
					published = true
					if failure == "publish" {
						return host.Result{}, errors.New("timeout")
					}
					return host.Result{}, nil
				}
				if strings.Contains(joined, "/aliases?") && args[2] == "GET" {
					list := []baseAlias{}
					if failure == "foreign" {
						list = append(list, baseAlias{Name: builtBasePrefix + "tools", Target: testFingerprintB, Type: "container", Description: "foreign"})
					}
					if published {
						if !ownerRecorded {
							t.Fatal("verify before ownership")
						}
						list = append(list, baseAlias{Name: "hacocoon-build-" + id, Target: testFingerprintA, Type: "container", Description: builtBaseDescription})
					}
					data, _ := json.Marshal(list)
					return host.Result{Stdout: string(data)}, nil
				}
				if strings.Contains(joined, "/images/"+testFingerprintA+"?") {
					if failure == "verify" {
						return host.Result{Stdout: "{}"}, nil
					}
					im := baseImage{Fingerprint: testFingerprintA, Type: "container", Properties: map[string]string{"user.hacocoon.kind": "base-image", "user.hacocoon.base-name": "tools", "user.hacocoon.build-instance": id}}
					data, _ := json.Marshal(im)
					return host.Result{Stdout: string(data)}, nil
				}
				if strings.Contains(joined, "/aliases?") && args[2] == "POST" {
					pointer = true
					if failure == "alias" {
						return host.Result{}, errors.New("timeout")
					}
					return host.Result{}, nil
				}
				t.Fatal("unexpected", joined)
				return host.Result{}, nil
			}}
			p, _ := NewBaseProvider(New(runner))
			got, err := p.PublishBase(context.Background(), env, lease, "tools")
			if (err != nil) != (failure != "") {
				t.Fatal(got, err)
			}
			if failure == "" && (!pointer || got.Revision != "sha256:"+testFingerprintA) {
				t.Fatal(got)
			}
			if (failure == "publish" || failure == "verify" || failure == "foreign") && pointer {
				t.Fatal("bad image registered")
			}
			if failure == "foreign" && published {
				t.Fatal("foreign alias overwritten")
			}
		})
	}
}
