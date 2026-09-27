package incus

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestImageTagRefusesForeignAliasAndChangedLocalImage(t *testing.T) {
	for _, mode := range []string{"missing", "aliases-fail", "image-fail", "changed-image", "post-fail"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv(baseConfigEnv, "")
			reads, images, posts := 0, 0, 0
			p, err := NewBaseProvider(New(&fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				if len(args) < 4 {
					t.Fatal(args)
				}
				var value any
				switch {
				case args[2] == "GET" && strings.Contains(args[3], "/aliases?"):
					reads++
					if mode == "aliases-fail" && reads > 1 {
						return host.Result{}, core.ErrRuntimeUnavailable
					}
					aliases := []baseAlias{}
					if mode != "missing" {
						aliases = append(aliases, baseAlias{Name: builtBasePrefix + "source", Target: testFingerprintA, Type: "container", Description: builtBaseDescription})
					}
					value = aliases
				case args[2] == "GET":
					images++
					if mode == "image-fail" && images > 1 {
						return host.Result{}, core.ErrRuntimeUnavailable
					}
					image := baseImage{Fingerprint: testFingerprintA, Type: "container", Properties: map[string]string{"user.hacocoon.kind": "base-image", "user.hacocoon.base-name": "source", "user.hacocoon.build-instance": testEnvironmentInstance}}
					if mode == "changed-image" && images > 1 {
						image.Fingerprint = testFingerprintB
					}
					value = image
				case args[2] == "POST":
					posts++
					return host.Result{}, core.ErrRuntimeUnavailable
				default:
					t.Fatal("unexpected provider mutation", args)
				}
				raw, err := json.Marshal(value)
				return host.Result{Stdout: string(raw)}, err
			}}))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := p.TagImage(context.Background(), "source", "target"); err == nil {
				t.Fatal("unsafe tag accepted")
			}
			if (posts == 1) != (mode == "post-fail") {
				t.Fatal("failure published alias", mode, posts)
			}
			if _, err := p.TagImage(context.Background(), "source", "../target"); !errors.Is(err, core.ErrInvalidArgument) {
				t.Fatal(err)
			}
			p.sources["reserved"] = p.sources["reserved"]
			if _, err := p.TagImage(context.Background(), "source", "reserved"); !errors.Is(err, core.ErrAlreadyExists) {
				t.Fatal(err)
			}
		})
	}
}

func TestImageTagDeletionAndLookupRequireOwnedContainerAlias(t *testing.T) {
	for _, mode := range []string{"missing", "foreign", "type", "fingerprint", "query", "delete"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv(baseConfigEnv, "")
			deletes := 0
			alias := baseAlias{Name: builtBasePrefix + "target", Target: testFingerprintA, Type: "container", Description: imageTagDescription}
			switch mode {
			case "foreign":
				alias.Description = "someone else"
			case "type":
				alias.Type = "virtual-machine"
			case "fingerprint":
				alias.Target = "invalid"
			}
			p, err := NewBaseProvider(New(&fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				if len(args) < 4 {
					t.Fatal(args)
				}
				if args[2] == "DELETE" {
					deletes++
					return host.Result{}, core.ErrRuntimeUnavailable
				}
				if mode == "query" {
					return host.Result{}, core.ErrRuntimeUnavailable
				}
				aliases := []baseAlias{}
				if mode != "missing" {
					aliases = append(aliases, alias)
				}
				raw, err := json.Marshal(aliases)
				return host.Result{Stdout: string(raw)}, err
			}}))
			if err != nil {
				t.Fatal(err)
			}
			if err := p.RemoveImageTag(context.Background(), "target"); err == nil {
				t.Fatal("unverified delete succeeded")
			}
			if (deletes == 1) != (mode == "delete") {
				t.Fatal("foreign alias deleted", mode, deletes)
			}
			if err := p.RemoveImageTag(context.Background(), "../target"); !errors.Is(err, core.ErrInvalidArgument) {
				t.Fatal(err)
			}
			if mode == "foreign" || mode == "type" || mode == "fingerprint" {
				if _, err := p.taggedImage(context.Background(), alias); !errors.Is(err, core.ErrCapabilityStale) {
					t.Fatal(err)
				}
			}
		})
	}
}
