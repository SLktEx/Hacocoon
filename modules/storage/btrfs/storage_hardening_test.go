package btrfs

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type failingUnmountFS struct {
	*fakeFS
	err error
}

func (f *failingUnmountFS) Unmount(context.Context, string) error {
	f.log.add("fs.unmount")
	return f.err
}

func TestStorageRejectsPathLikeIdentifiersBeforeBackendAccess(t *testing.T) {
	for _, id := range []string{"../victim", "../../etc", "/absolute", "nested/name", "..", "."} {
		t.Run(id, func(t *testing.T) {
			log := &eventLog{}
			storage := New(t.TempDir(), &fakeBlock{log: log}, &fakeFS{log: log, state: FilesystemState{LogicalBytes: 100 << 30}, min: 1})
			ctx := context.Background()
			handle := core.StorageHandle{ID: id}

			if _, err := storage.Ensure(ctx, core.StorageSpec{ID: id, SizeBytes: 1 << 30}); !errors.Is(err, core.ErrInvalidArgument) {
				t.Fatalf("Ensure id=%q err=%v", id, err)
			}
			if _, err := storage.Inspect(ctx, handle); !errors.Is(err, core.ErrInvalidArgument) {
				t.Fatalf("Inspect id=%q err=%v", id, err)
			}
			if err := storage.Delete(ctx, handle); !errors.Is(err, core.ErrInvalidArgument) {
				t.Fatalf("Delete id=%q err=%v", id, err)
			}
			if err := storage.Grow(ctx, handle, 101<<30); !errors.Is(err, core.ErrInvalidArgument) {
				t.Fatalf("Grow id=%q err=%v", id, err)
			}
			if _, err := storage.PlanShrink(ctx, handle, 90<<30); !errors.Is(err, core.ErrInvalidArgument) {
				t.Fatalf("PlanShrink id=%q err=%v", id, err)
			}
			if err := storage.Shrink(ctx, handle, core.ShrinkPlan{TargetBytes: 90 << 30}); !errors.Is(err, core.ErrInvalidArgument) {
				t.Fatalf("Shrink id=%q err=%v", id, err)
			}
			if err := storage.Compact(ctx, handle); !errors.Is(err, core.ErrInvalidArgument) {
				t.Fatalf("Compact id=%q err=%v", id, err)
			}
			if len(log.events) != 0 {
				t.Fatalf("unsafe id reached backend/filesystem: %v", log.events)
			}
		})
	}
}

func TestDeleteDoesNotDeleteBackingImageWhenUnmountFails(t *testing.T) {
	log := &eventLog{}
	backend := &fakeBlock{log: log}
	fs := &failingUnmountFS{fakeFS: &fakeFS{log: log}, err: errors.New("busy")}
	storage := New(t.TempDir(), backend, fs)

	if err := storage.Delete(context.Background(), core.StorageHandle{ID: "local-default"}); err == nil {
		t.Fatal("expected delete to fail when unmount fails")
	}
	if index(log.events, "fs.unmount") < 0 {
		t.Fatalf("unmount was not attempted: %v", log.events)
	}
	if index(log.events, "block.delete") >= 0 {
		t.Fatalf("backing image deleted after failed unmount: %v", log.events)
	}
}
