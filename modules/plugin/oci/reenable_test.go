package oci

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestReenableImageRemovesOnlyExactTombstone(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "usage.json"))
	digest := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	deletion := Deletion{Reference: "docker.io/library/node:24", Digest: digest, DeletedAt: time.Now()}
	if err := store.PutDeletion(context.Background(), deletion); err != nil {
		t.Fatal(err)
	}
	service := &Service{store: store}
	deleted, err := service.IsImageDeleted(context.Background(), deletion.Reference, deletion.Digest)
	if err != nil || !deleted {
		t.Fatalf("deleted=%v err=%v", deleted, err)
	}
	report, err := service.ReenableImage(context.Background(), deletion.Key())
	if err != nil {
		t.Fatal(err)
	}
	if !report.Removed || report.Reference != deletion.Reference || report.Digest != deletion.Digest {
		t.Fatalf("report=%#v", report)
	}
	deleted, err = service.IsImageDeleted(context.Background(), deletion.Reference, deletion.Digest)
	if err != nil || deleted {
		t.Fatalf("deleted=%v err=%v after re-enable", deleted, err)
	}
}

func TestReenableImageRequiresImmutableIdentity(t *testing.T) {
	service := &Service{store: NewStore(filepath.Join(t.TempDir(), "usage.json"))}
	for _, raw := range []string{"docker.io/library/node:24", "docker.io/library/node:24@sha256:abc", " docker.io/library/node:24@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} {
		if _, err := service.ReenableImage(context.Background(), raw); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("raw=%q err=%v want ErrInvalidArgument", raw, err)
		}
	}
}

func TestReenableImageIsIdempotent(t *testing.T) {
	service := &Service{store: NewStore(filepath.Join(t.TempDir(), "usage.json"))}
	identity := "docker.io/library/node:24@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	report, err := service.ReenableImage(context.Background(), identity)
	if err != nil || report.Removed {
		t.Fatalf("report=%#v err=%v", report, err)
	}
}
