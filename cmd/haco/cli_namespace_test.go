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

func TestOCISeedRoutesThroughPluginNamespace(t *testing.T) {
	err := dispatch(context.Background(), nil, []string{"plugin", "oci", "seed", "recommend"})
	if !errors.Is(err, core.ErrRuntimeUnavailable) {
		t.Fatalf("err=%v", err)
	}
}
