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

func TestSnapshotImageReadRejectsMissingOrChangedOwnership(t *testing.T) {
	for _, mode := range []string{"valid", "missing", "owner", "properties", "aliases-query", "image-query"} {
		t.Run(mode, func(t *testing.T) {
			root, _ := rootfsFixture()
			plan := snapshotImagePlan{Name: "saved", Owner: strings.Repeat("f", 32), Rootfs: root}
			r := New(&fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				if len(args) < 4 || args[2] != "GET" {
					t.Fatal("read mutated provider", args)
				}
				var value any
				if strings.Contains(args[3], "/aliases?") {
					if mode == "aliases-query" {
						return host.Result{}, core.ErrRuntimeUnavailable
					}
					aliases := []baseAlias{{Name: builtBasePrefix + "unrelated", Target: testFingerprintB, Type: "container", Description: builtBaseDescription}}
					if mode != "missing" {
						aliases = append(aliases, baseAlias{Name: builtBasePrefix + plan.Name, Target: testFingerprintA, Type: "container", Description: builtBaseDescription})
					}
					value = aliases
				} else {
					if mode == "image-query" {
						return host.Result{}, core.ErrRuntimeUnavailable
					}
					properties := map[string]string{"user.hacocoon.kind": "base-image", "user.hacocoon.base-name": plan.Name, "user.hacocoon.build-instance": root.SourceInstanceID, "user.hacocoon.snapshot-owner": plan.Owner}
					if mode == "owner" {
						properties["user.hacocoon.snapshot-owner"] = strings.Repeat("e", 32)
					}
					if mode == "properties" {
						properties["user.hacocoon.base-name"] = "replacement"
					}
					value = baseImage{Fingerprint: testFingerprintA, Type: "container", Properties: properties}
				}
				raw, err := json.Marshal(value)
				return host.Result{Stdout: string(raw)}, err
			}})
			component, err := r.snapshotComponent(snapshotBinding{Version: 1, Project: r.project, Image: &plan})
			if err != nil {
				t.Fatal(err)
			}
			image, err := r.SnapshotImage(context.Background(), component)
			if mode == "valid" {
				if err != nil || image.Name != "saved" || image.Revision != core.BaseRevision("sha256:"+testFingerprintA) {
					t.Fatal(image, err)
				}
			} else if err == nil {
				t.Fatal("unverified Image accepted", mode)
			}
			inspection, err := r.InspectSnapshotComponent(context.Background(), component)
			if mode == "valid" {
				if err != nil || inspection.Presence != "present" || inspection.Check != "verified" {
					t.Fatal(inspection, err)
				}
			} else if err == nil || inspection.Presence != "unknown" || inspection.Check != "unconfirmed" {
				t.Fatal(inspection, err)
			}
			component.Owner = "replacement"
			if _, err := r.SnapshotImage(context.Background(), component); !errors.Is(err, core.ErrCapabilityStale) {
				t.Fatal(err)
			}
			plain, err := r.snapshotComponent(snapshotBinding{Version: 1, Project: r.project, Rootfs: &root})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := r.SnapshotImage(context.Background(), plain); !errors.Is(err, core.ErrInvalidArgument) {
				t.Fatal(err)
			}
		})
	}
}

func TestSnapshotImageDeletionRequiresPositiveAbsence(t *testing.T) {
	for _, mode := range []string{"absent", "null", "duplicate", "query-failure", "delete-failure", "still-present", "recheck-failure"} {
		t.Run(mode, func(t *testing.T) {
			root, _ := rootfsFixture()
			plan := snapshotImagePlan{Name: "saved", Owner: strings.Repeat("f", 32), Rootfs: root}
			reads, deletes := 0, 0
			r := New(&fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				if len(args) < 4 {
					t.Fatal(args)
				}
				if args[2] == "DELETE" {
					deletes++
					if !strings.Contains(args[3], testFingerprintA) {
						t.Fatal("wrong image deleted", args)
					}
					if mode == "delete-failure" {
						return host.Result{}, core.ErrRuntimeUnavailable
					}
					return host.Result{}, nil
				}
				if args[2] != "GET" {
					t.Fatal(args)
				}
				reads++
				if mode == "query-failure" || mode == "recheck-failure" && reads > 1 {
					return host.Result{}, core.ErrRuntimeUnavailable
				}
				if mode == "null" {
					return host.Result{Stdout: "null"}, nil
				}
				image := baseImage{Fingerprint: testFingerprintA, Type: "container", Properties: map[string]string{"user.hacocoon.kind": "base-image", "user.hacocoon.base-name": plan.Name, "user.hacocoon.build-instance": root.SourceInstanceID, "user.hacocoon.snapshot-owner": plan.Owner}}
				images := []baseImage{{Fingerprint: testFingerprintB, Type: "container", Properties: map[string]string{"user.hacocoon.snapshot-owner": "foreign"}}}
				if mode != "absent" {
					images = append(images, image)
				}
				if mode == "duplicate" {
					images = append(images, image)
				}
				raw, err := json.Marshal(images)
				return host.Result{Stdout: string(raw)}, err
			}})
			err := r.deleteSnapshotImage(context.Background(), plan)
			if mode == "absent" {
				if err != nil || deletes != 0 {
					t.Fatal(err, deletes)
				}
			} else if err == nil {
				t.Fatal("unconfirmed deletion accepted")
			}
			if mode == "still-present" || mode == "recheck-failure" || mode == "duplicate" {
				if !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatal(err)
				}
			}
			if mode == "null" || mode == "duplicate" || mode == "query-failure" {
				if deletes != 0 {
					t.Fatal("untrusted inventory permitted delete")
				}
			}
		})
	}
}
