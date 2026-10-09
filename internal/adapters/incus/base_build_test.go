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
	for _, failure := range []string{"", "publish", "verify", "alias", "foreign", "other-build", "other-builder", "missing-builder"} {
		t.Run(failure, func(t *testing.T) {
			t.Setenv(baseConfigEnv, "")
			work := core.NewTemporaryWorkspace()
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
					if json.Unmarshal([]byte(args[5]), &body) != nil || body.Public || body.Properties["user.hacocoon.build-instance"] != id || body.Properties["user.hacocoon.build-environment"] != env.Name || body.Source["name"] != env.RuntimeRef {
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
					im := baseImage{Fingerprint: testFingerprintA, Type: "container", Properties: map[string]string{"user.hacocoon.kind": "base-image", "user.hacocoon.base-name": "tools", "user.hacocoon.build-instance": id, "user.hacocoon.build-environment": env.Name}}
					switch failure {
					case "other-build":
						// A valid owner for another build is not this publication's
						// ownership receipt, even with the expected alias and name.
						im.Properties["user.hacocoon.build-instance"] = "env-" + strings.Repeat("b", 32)
					case "other-builder":
						im.Properties["user.hacocoon.build-environment"] = "other-builder"
					case "missing-builder":
						delete(im.Properties, "user.hacocoon.build-environment")
					}
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
			if failure == "other-build" || failure == "other-builder" || failure == "missing-builder" {
				if pointer || got.Revision != "" || !errors.Is(err, core.ErrCapabilityStale) || !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatal("mismatched publication moved the Base pointer or lost recovery state", got, err)
				}
			}
		})
	}
}

func TestPublishBasePreservesHistoricalAliasOnMismatchedBuild(t *testing.T) {
	for _, mismatched := range []bool{false, true} {
		t.Run(map[bool]string{false: "exact-build", true: "other-build"}[mismatched], func(t *testing.T) {
			t.Setenv(baseConfigEnv, "")
			work := core.NewTemporaryWorkspace()
			id := "env-" + strings.Repeat("a", 32)
			oldID := "env-" + strings.Repeat("b", 32)
			env := core.Environment{Name: "builder", RuntimeRef: "haco-builder", Workspace: work}
			lease := core.WorkspaceLease{EnvironmentID: env.Name, RuntimeRef: env.RuntimeRef, InstanceID: id, WorkspaceID: work.ID, SourcePath: work.Path, State: core.WorkspaceLeaseActive}
			alias := baseAlias{Name: builtBasePrefix + "tools", Target: testFingerprintB, Type: "container", Description: builtBaseDescription}
			published, updated := false, false
			runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				joined := strings.Join(args, " ")
				var value any
				switch {
				case strings.HasPrefix(joined, "config get"):
					return host.Result{Stdout: id}, nil
				case args[0] == "list":
					return host.Result{Stdout: "haco-builder,STOPPED\n"}, nil
				case strings.HasPrefix(joined, "file push"):
					return host.Result{}, nil
				case strings.Contains(joined, "/metadata?"):
					value = map[string]any{"architecture": "x86_64", "templates": map[string]any{}}
				case strings.HasPrefix(joined, "query -X POST /1.0/images?"):
					published = true
					return host.Result{}, nil
				case strings.Contains(joined, "/aliases?") && args[2] == "GET":
					aliases := []baseAlias{alias}
					if published {
						aliases = append(aliases, baseAlias{Name: "hacocoon-build-" + id, Target: testFingerprintA, Type: "container", Description: builtBaseDescription})
					}
					value = aliases
				case strings.Contains(joined, "/images/"+testFingerprintB+"?"):
					// Historical revisions can lack the diagnostic builder name.
					value = baseImage{Fingerprint: testFingerprintB, Type: "container", Properties: map[string]string{"user.hacocoon.kind": "base-image", "user.hacocoon.base-name": "tools", "user.hacocoon.build-instance": oldID}}
				case strings.Contains(joined, "/images/"+testFingerprintA+"?"):
					owner := id
					if mismatched {
						owner = oldID
					}
					value = baseImage{Fingerprint: testFingerprintA, Type: "container", Properties: map[string]string{"user.hacocoon.kind": "base-image", "user.hacocoon.base-name": "tools", "user.hacocoon.build-instance": owner, "user.hacocoon.build-environment": env.Name}}
				case strings.HasPrefix(joined, "query -X PUT /1.0/images/aliases/"+alias.Name+"?"):
					var body baseAlias
					if json.Unmarshal([]byte(args[5]), &body) != nil || body.Name != alias.Name || body.Target != testFingerprintA || body.Description != builtBaseDescription {
						t.Fatal("unexpected alias update", joined)
					}
					alias.Target = body.Target
					updated = true
					return host.Result{}, nil
				default:
					t.Fatal("unexpected provider operation", joined)
				}
				data, _ := json.Marshal(value)
				return host.Result{Stdout: string(data)}, nil
			}}
			p, _ := NewBaseProvider(New(runner))
			got, err := p.PublishBase(context.Background(), env, lease, "tools")
			if !published {
				t.Fatal("historical alias prevented new publication", err)
			}
			if mismatched {
				if !errors.Is(err, core.ErrRecoveryRequired) || !errors.Is(err, core.ErrCapabilityStale) || updated || alias.Target != testFingerprintB || got.Revision != "" {
					t.Fatal("mismatched build replaced the historical revision", got, err)
				}
			} else if err != nil || !updated || alias.Target != testFingerprintA || got.Revision != "sha256:"+testFingerprintA {
				t.Fatal("exact build did not replace the historical revision", got, err)
			}
		})
	}
}
