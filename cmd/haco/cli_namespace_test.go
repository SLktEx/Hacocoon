package main

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestLegacyImageCommandIsRemoved(t *testing.T) {
	err := dispatch(context.Background(), nil, []string{"image", "list"})
	if !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("err=%v", err)
	}
}

func TestBaseCommandIsTopLevel(t *testing.T) {
	err := dispatch(context.Background(), nil, []string{"base"})
	if !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("err=%v", err)
	}
}

func TestOCIImageDeleteRoutesThroughPluginNamespace(t *testing.T) {
	err := dispatch(context.Background(), nil, []string{"plugin", "oci", "image", "delete", "docker.io/library/node:24"})
	if !errors.Is(err, core.ErrRuntimeUnavailable) {
		t.Fatalf("err=%v", err)
	}
}

func TestLegacySeedCommandsAreRemoved(t *testing.T) {
	for _, action := range []string{"sample", "recommend", "build", "current", "pin", "unpin", "pins", "gc", "recover"} {
		err := dispatch(context.Background(), nil, []string{"plugin", "oci", "seed", action})
		if !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("%s: err=%v", action, err)
		}
	}
}
