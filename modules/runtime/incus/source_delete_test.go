package incus

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
	"strings"
	"testing"
)

func TestSourcePreflightRejectsForeignHostDeviceAndNativeSavedData(t *testing.T) {
	for _, mode := range []string{"ok", "foreign-host", "device-owner", "inherited", "duplicate-device", "foreign-user", "saved", "malformed", "query-failure", "volume-owner"} {
		t.Run(mode, func(t *testing.T) {
			o := gitrepo.Object{Kind: "repo", ID: "source", Repository: "source", NativeRef: "pool/haco-repo-source", Owner: strings.Repeat("a", 32)}
			device := map[string]string{"type": "disk", "pool": "pool", "source": "haco-repo-source", "path": gitrepo.RepositoryRoot + "/source"}
			devices := map[string]map[string]string{"haco-repo-source": device}
			expanded := map[string]map[string]string{"haco-repo-source": device}
			role := trustedHostRoleValue
			if mode == "foreign-host" {
				role = "foreign"
			}
			if mode == "device-owner" {
				device["source"] = "other"
			}
			if mode == "inherited" {
				delete(devices, "haco-repo-source")
			}
			if mode == "duplicate-device" {
				expanded["other"] = device
			}
			volume := persistentVolumeObservation{Name: "haco-repo-source", Type: "custom", ContentType: "filesystem", Config: volumeConfig(o), UsedBy: []string{"/1.0/instances/haco-host?project=hacocoon"}}
			if mode == "foreign-user" {
				volume.UsedBy = []string{"/1.0/instances/other?project=hacocoon"}
			}
			if mode == "volume-owner" {
				volume.Config["user.hacocoon.owner"] = "foreign"
			}
			encode := func(v any) (host.Result, error) {
				raw, err := json.Marshal(v)
				return host.Result{Stdout: string(raw)}, err
			}
			runner := baseDeleteRunner(func(_ context.Context, name string, args ...string) (host.Result, error) {
				if name != "incus" || len(args) != 2 || args[0] != "query" {
					t.Fatalf("unexpected mutation: %s %v", name, args)
				}
				if mode == "malformed" {
					return host.Result{Stdout: "null"}, nil
				}
				if mode == "query-failure" {
					return host.Result{ExitCode: 1}, nil
				}
				switch {
				case strings.HasPrefix(args[1], "/1.0/instances/"):
					return encode(map[string]any{"name": "haco-host", "type": "container", "config": map[string]string{trustedHostRoleKey: role}, "devices": devices, "expanded_devices": expanded})
				case strings.Contains(args[1], "recursion=1"):
					return encode([]persistentVolumeObservation{volume})
				default:
					if mode == "saved" {
						return encode([]string{"saved"})
					}
					return encode([]string{})
				}
			})
			b := &RepositoryBackend{Runtime: New(runner)}
			err := b.CheckSourceDeletion(context.Background(), o)
			if (err == nil) != (mode == "ok") {
				t.Fatalf("err=%v", err)
			}
			if mode == "saved" && !errors.Is(err, core.ErrStorageBusy) {
				t.Fatal(err)
			}
		})
	}
}
