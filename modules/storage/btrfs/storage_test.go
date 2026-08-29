package btrfs

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/storage/btrfs/internal/block"
)

type eventLog struct {
	events []string
}

func (l *eventLog) add(event string) {
	if l != nil {
		l.events = append(l.events, event)
	}
}

type fakeBlock struct {
	log       *eventLog
	shrinkErr error
}

func (*fakeBlock) ID() string { return "block.fake" }
func (*fakeBlock) Probe(context.Context) (block.Capabilities, error) {
	return block.Capabilities{Available: true, Shrink: true, Compact: true}, nil
}
func (f *fakeBlock) Ensure(context.Context, block.Spec) (block.Handle, error) {
	f.log.add("block.ensure")
	return block.Handle{ID: "local-default", Path: "image", Device: "/dev/fake", Bytes: 100 << 30}, nil
}
func (*fakeBlock) Inspect(context.Context, block.Handle) (block.State, error) {
	return block.State{Healthy: true, Bytes: 100 << 30, Device: "/dev/fake"}, nil
}
func (f *fakeBlock) Attach(context.Context, block.Handle) (block.Handle, error) {
	f.log.add("block.attach")
	return block.Handle{ID: "local-default", Path: "image", Device: "/dev/fake", Bytes: 100 << 30}, nil
}
func (f *fakeBlock) Detach(context.Context, block.Handle) error {
	f.log.add("block.detach")
	return nil
}
func (f *fakeBlock) Grow(context.Context, block.Handle, int64) (block.Handle, error) {
	f.log.add("block.grow")
	return block.Handle{ID: "local-default", Path: "image", Device: "/dev/fake"}, nil
}
func (f *fakeBlock) Shrink(context.Context, block.Handle, int64) (block.Handle, error) {
	f.log.add("block.shrink")
	if f.shrinkErr != nil {
		return block.Handle{}, f.shrinkErr
	}
	return block.Handle{ID: "local-default", Path: "image", Device: "/dev/fake"}, nil
}
func (f *fakeBlock) Compact(context.Context, block.Handle) error {
	f.log.add("block.compact")
	return nil
}
func (f *fakeBlock) Delete(context.Context, block.Handle) error {
	f.log.add("block.delete")
	return nil
}

type fakeFS struct {
	log       *eventLog
	state     FilesystemState
	min       int64
	shrinkErr error
}

func (*fakeFS) Probe(context.Context) error { return nil }
func (f *fakeFS) Ensure(context.Context, string, string) error {
	f.log.add("fs.ensure")
	return nil
}
func (f *fakeFS) Inspect(context.Context, string) (FilesystemState, error) { return f.state, nil }
func (f *fakeFS) Grow(context.Context, string) error {
	f.log.add("fs.grow")
	return nil
}
func (f *fakeFS) MinimumSize(context.Context, string) (int64, error) { return f.min, nil }
func (f *fakeFS) Compact(context.Context, string) error {
	f.log.add("fs.compact")
	return nil
}
func (f *fakeFS) Shrink(context.Context, string, int64) error {
	f.log.add("fs.shrink")
	return f.shrinkErr
}
func (f *fakeFS) Unmount(context.Context, string) error {
	f.log.add("fs.unmount")
	return nil
}
func (f *fakeFS) Mount(context.Context, string, string) error {
	f.log.add("fs.mount")
	return nil
}
func (f *fakeFS) Verify(context.Context, string, int64) error {
	f.log.add("fs.verify")
	return nil
}

func TestStorageUsesCanonicalID(t *testing.T) {
	storage := New(t.TempDir(), &fakeBlock{}, &fakeFS{})
	if got := storage.ID(); got != "storage.btrfs" {
		t.Fatalf("unexpected storage module ID %q", got)
	}
}

func TestShrinkNeverTouchesOuterBeforeFilesystemSuccess(t *testing.T) {
	log := &eventLog{}
	backend := &fakeBlock{log: log}
	fs := &fakeFS{
		log:       log,
		state:     FilesystemState{Healthy: true, LogicalBytes: 100 << 30, UsedBytes: 20 << 30},
		min:       25 << 30,
		shrinkErr: errors.New("cannot shrink"),
	}
	storage := New(t.TempDir(), backend, fs)
	handle := core.StorageHandle{ID: "local-default"}
	plan, err := storage.PlanShrink(context.Background(), handle, 80<<30)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Feasible {
		t.Fatalf("expected feasible plan: %+v", plan)
	}
	if err := storage.Shrink(context.Background(), handle, plan); err == nil {
		t.Fatal("expected shrink error")
	}
	if index(log.events, "block.shrink") >= 0 {
		t.Fatalf("outer image was touched after filesystem shrink failure: %v", log.events)
	}
}

func TestShrinkOrdersFilesystemBeforeOuter(t *testing.T) {
	log := &eventLog{}
	backend := &fakeBlock{log: log}
	fs := &fakeFS{
		log:   log,
		state: FilesystemState{Healthy: true, LogicalBytes: 100 << 30, UsedBytes: 20 << 30},
		min:   25 << 30,
	}
	storage := New(t.TempDir(), backend, fs)
	handle := core.StorageHandle{ID: "local-default"}
	plan, err := storage.PlanShrink(context.Background(), handle, 75<<30)
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.Shrink(context.Background(), handle, plan); err != nil {
		t.Fatal(err)
	}
	fsShrink := index(log.events, "fs.shrink")
	fsUnmount := index(log.events, "fs.unmount")
	outerShrink := index(log.events, "block.shrink")
	if fsShrink < 0 || fsUnmount < 0 || outerShrink < 0 {
		t.Fatalf("missing shrink events: %v", log.events)
	}
	if !(fsShrink < fsUnmount && fsUnmount < outerShrink) {
		t.Fatalf("unsafe shrink order: %v", log.events)
	}
}

func TestUnsafeShrinkIsRefused(t *testing.T) {
	log := &eventLog{}
	backend := &fakeBlock{log: log}
	fs := &fakeFS{
		log:   log,
		state: FilesystemState{Healthy: true, LogicalBytes: 100 << 30, UsedBytes: 70 << 30},
		min:   72 << 30,
	}
	storage := New(filepath.Join(t.TempDir(), "storage"), backend, fs)
	plan, err := storage.PlanShrink(context.Background(), core.StorageHandle{ID: "local-default"}, 60<<30)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Feasible {
		t.Fatalf("unexpected feasible plan: %+v", plan)
	}
	if err := storage.Shrink(context.Background(), core.StorageHandle{ID: "local-default"}, plan); !errors.Is(err, core.ErrUnsafeShrink) {
		t.Fatalf("expected unsafe-shrink refusal, got %v", err)
	}
	if index(log.events, "block.shrink") >= 0 {
		t.Fatalf("unsafe plan touched backing image: %v", log.events)
	}
}

func index(events []string, want string) int {
	for i, event := range events {
		if event == want {
			return i
		}
	}
	return -1
}
