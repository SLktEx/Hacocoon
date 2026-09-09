package incus

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/basemanage"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestBaseDeletionUsesReviewedOwnerAndPreservesReferences(t *testing.T) {
	for _, mode := range []string{"ok", "foreign-owner", "alias", "instance", "saved-root", "catalog", "remains", "malformed", "delete-failure", "invalid-remaining", "configured-alias", "configured-fingerprint"} {
		t.Run(mode, func(t *testing.T) {
			id := basemanage.Identity{Name: "tools", Fingerprint: strings.Repeat("a", 64), BuildInstance: "env-" + strings.Repeat("b", 32)}
			deleted := false
			image := baseImage{Fingerprint: id.Fingerprint, Type: "container", Properties: map[string]string{"user.hacocoon.kind": "base-image", "user.hacocoon.base-name": "tools", "user.hacocoon.build-instance": id.BuildInstance}}
			if mode == "foreign-owner" {
				image.Properties["user.hacocoon.build-instance"] = "env-" + strings.Repeat("c", 32)
			}
			encode := func(v any) (host.Result, error) {
				raw, err := json.Marshal(v)
				return host.Result{Stdout: string(raw)}, err
			}
			runner := baseDeleteRunner(func(ctx context.Context, name string, args ...string) (host.Result, error) {
				if name != "incus" || len(args) < 4 || args[0] != "query" || args[1] != "-X" {
					t.Fatal(name, args)
				}
				method, path := args[2], args[3]
				if method == "DELETE" {
					if path != "/1.0/images/"+id.Fingerprint+"?project=hacocoon" {
						t.Fatal(path)
					}
					deleted = true
					if mode == "delete-failure" {
						return host.Result{ExitCode: 1}, nil
					}
					return host.Result{}, nil
				}
				if method != "GET" {
					t.Fatal(method)
				}
				switch path {
				case "/1.0/images?project=hacocoon&recursion=1":
					if mode == "malformed" {
						return host.Result{Stdout: "null"}, nil
					}
					if deleted && mode == "invalid-remaining" {
						return encode([]baseImage{{}})
					}
					if deleted && mode != "remains" {
						return encode([]baseImage{})
					}
					return encode([]baseImage{image})
				case "/1.0/images/aliases?project=hacocoon&recursion=1":
					aliases := []baseAlias{{Name: "hacocoon-base-tools", Target: id.Fingerprint, Type: "container", Description: builtBaseDescription}}
					if mode == "alias" {
						aliases = append(aliases, baseAlias{Name: "my-other-use", Target: id.Fingerprint, Type: "container"})
					}
					return encode(aliases)
				case "/1.0/instances?project=hacocoon&recursion=1":
					instances := []snapshotInstanceObservation{}
					if mode == "instance" {
						instances = append(instances, snapshotInstanceObservation{Name: "haco-dev", Config: map[string]string{"volatile.base_image": id.Fingerprint}})
					}
					if mode == "saved-root" {
						owner := strings.Repeat("d", 32)
						instances = append(instances, snapshotInstanceObservation{Name: "haco-snapshot-root-" + owner, Config: map[string]string{"volatile.base_image": id.Fingerprint, "user.hacocoon.kind": "snapshot-rootfs", "user.hacocoon.owner": owner}})
					}
					return encode(instances)
				default:
					t.Fatal(path)
					return host.Result{}, nil
				}
			})
			provider, err := NewBaseProvider(New(runner))
			if err != nil {
				t.Fatal(err)
			}
			if mode == "configured-alias" {
				provider.sources["retained"] = "local:hacocoon-base-tools"
			}
			if mode == "configured-fingerprint" {
				provider.sources["retained"] = id.Fingerprint
			}
			err = provider.DeleteBaseImage(context.Background(), id, func(context.Context) error {
				if mode == "catalog" {
					return core.ErrStorageBusy
				}
				return nil
			})
			good := mode == "ok" || mode == "saved-root"
			if good {
				if err != nil || !deleted {
					t.Fatal(err, deleted)
				}
			} else if err == nil {
				t.Fatal("unsafe deletion accepted")
			}
			if !good && mode != "remains" && mode != "delete-failure" && mode != "invalid-remaining" && deleted {
				t.Fatal("deleted before refusal")
			}
			if mode == "remains" && !errors.Is(err, core.ErrRecoveryRequired) {
				t.Fatal(err)
			}
		})
	}
}

type baseDeleteRunner func(context.Context, string, ...string) (host.Result, error)

func (f baseDeleteRunner) Run(ctx context.Context, name string, args ...string) (host.Result, error) {
	return f(ctx, name, args...)
}
