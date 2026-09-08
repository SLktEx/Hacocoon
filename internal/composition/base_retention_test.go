package composition

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	environmentapp "github.com/SLktEx/Hacocoon/internal/environment"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/state"
	"github.com/SLktEx/Hacocoon/modules/runtime/incus"
)

func TestComposedBaseRetentionPublishesCanonicalProviderBeforeEnvironmentInit(t *testing.T) {
	ctx := context.Background()
	catalog := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "environments.json"))
	fp := strings.Repeat("a", 64)
	base := core.BaseRef{Name: "haco/ubuntu-26.04", Revision: core.BaseRevision("sha256:" + fp)}
	creates := 0
	var retained map[string]any
	runner := &storageRunnerFunc{run: func(name string, args []string) (host.Result, error) {
		if name != "incus" {
			t.Fatal(name)
		}
		switch {
		case args[0] == "image":
			return host.Result{Stdout: `{"fingerprint":"` + fp + `"}`}, nil
		case args[0] == "profile":
			return host.Result{Stdout: `{"devices":{"root":{"type":"disk","path":"/","pool":"pool"}}}`}, nil
		case args[0] == "project":
			return host.Result{}, nil
		case args[0] == "query" && args[1] == "/1.0/storage-pools/pool":
			return host.Result{Stdout: `{"name":"pool","driver":"btrfs"}`}, nil
		case args[0] == "query" && args[1] == "/1.0/instances?project=hacocoon&recursion=1":
			raw, _ := json.Marshal([]any{retained})
			return host.Result{Stdout: string(raw)}, nil
		case args[0] == "init" && strings.HasPrefix(args[2], "haco-base-"):
			creates++
			config := map[string]string{"volatile.base_image": fp}
			for i := 8; i < len(args); i += 2 {
				k, v, _ := strings.Cut(args[i+1], "=")
				config[k] = v
			}
			devices := map[string]any{"root": map[string]string{"type": "disk", "path": "/", "pool": "pool"}}
			retained = map[string]any{"name": args[2], "type": "container", "status": "Stopped", "profiles": []string{}, "config": config, "expanded_config": config, "devices": devices, "expanded_devices": devices}
			return host.Result{}, nil
		default:
			// The real creation path may now proceed; this fixture stops before any
			// Environment mutation and checks the composed catalog, not fake receipts.
			a, err := catalog.FindBaseAsset(ctx, base, environmentapp.ProviderIncus, "hacocoon/pool")
			if err != nil || a.State != "ready" {
				t.Fatalf("Environment operation before ready Base: %v %v %v", args, a, err)
			}
			return host.Result{ExitCode: 1}, core.ErrRuntimeUnavailable
		}
	}}
	provider, err := incus.NewBaseProvider(incus.New(runner))
	if err != nil {
		t.Fatal(err)
	}
	configureBaseRetention(provider, catalog)
	id, err := core.NewEnvironmentInstanceID()
	if err != nil {
		t.Fatal(err)
	}
	_, _ = provider.CreateEnvironment(ctx, core.EnvironmentRuntimeSpec{InstanceID: id, Name: "composed", WorkspacePath: t.TempDir()})
	a, err := catalog.FindBaseAsset(ctx, base, environmentapp.ProviderIncus, "hacocoon/pool")
	if err != nil || a.State != "ready" || creates != 1 {
		t.Fatalf("retention failed: %+v %v creates=%d", a, err, creates)
	}
}
