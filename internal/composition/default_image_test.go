package composition

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/env/creation"
	"github.com/SLktEx/Hacocoon/internal/state"
	"path/filepath"
	"testing"
)

func TestSetupDownloadsBeforeSettingInitialDefault(t *testing.T) {
	ctx := context.Background()
	store := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "environments.json"))
	downloads := 0
	a := &App{Creation: &creation.Service{Catalog: store}, InitialImage: func(context.Context) (core.BaseName, error) { downloads++; return "standard", nil }}
	if err := a.initializeDefaultImage(ctx); err != nil {
		t.Fatal(err)
	}
	if got, err := store.DefaultImage(ctx); err != nil || got != "standard" || downloads != 1 {
		t.Fatal(got, err, downloads)
	}
	if err := store.SetDefaultImage(ctx, "custom", false); err != nil {
		t.Fatal(err)
	}
	if err := a.initializeDefaultImage(ctx); err != nil {
		t.Fatal(err)
	}
	if got, _ := store.DefaultImage(ctx); got != "custom" || downloads != 1 {
		t.Fatal("setup overwrote preference", got)
	}
}
func TestSetupDownloadFailureDoesNotSetDefault(t *testing.T) {
	ctx := context.Background()
	store := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "environments.json"))
	a := &App{Creation: &creation.Service{Catalog: store}, InitialImage: func(context.Context) (core.BaseName, error) { return "", errors.New("download") }}
	if err := a.initializeDefaultImage(ctx); err == nil {
		t.Fatal("download failure swallowed")
	}
	if got, _ := store.DefaultImage(ctx); got != "" {
		t.Fatal(got)
	}
}
