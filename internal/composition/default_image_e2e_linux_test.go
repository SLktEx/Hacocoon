//go:build linux

package composition

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/adapters/incus"
	"github.com/SLktEx/Hacocoon/internal/env/creation"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/state"
)

// Uses the ordinary bootstrap image path with a private preferences catalog.
// It downloads into the normal Incus image cache but neither reinstalls the
// trusted Host nor alters any existing Environment or user's default setting.
func TestRealIncusInitialDefaultImageE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_INCUS_RESUME") != "1" {
		t.Skip("requires opt-in real Incus host and standard image download access")
	}
	if os.Geteuid() != 0 {
		t.Fatal("root Incus acceptance host required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	runner := host.ExecRunner{}
	provider, err := incus.NewBaseProvider(incus.New(runner))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "environments.json")
	catalog := state.NewEnvironmentJSONStore(path)
	app := &App{Creation: &creation.Service{Catalog: catalog, Images: provider}, InitialImage: provider.EnsureStandardImage}
	if err := app.initializeDefaultImage(ctx); err != nil {
		t.Fatal(err)
	}
	image, err := state.NewEnvironmentJSONStore(path).DefaultImage(ctx)
	if err != nil || image != "haco/ubuntu-26.04" {
		t.Fatal("standard default was not persisted", image, err)
	}
	info, err := provider.InspectBase(ctx, image)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint := strings.TrimPrefix(string(info.Revision), "sha256:")
	result, err := runner.Run(ctx, "incus", "image", "info", fingerprint, "--project", "hacocoon")
	if err != nil || result.ExitCode != 0 {
		t.Fatal("bootstrap default does not reference a downloaded Incus image")
	}
	if err := app.initializeDefaultImage(ctx); err != nil {
		t.Fatal("repeat bootstrap failed", err)
	}
	t.Log("PASS normal bootstrap standard Image acquisition, persisted initial default and repeat setup; existing Host/catalog untouched")
}
