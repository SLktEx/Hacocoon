package btrfs

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type minimumFailureFS struct {
	fakeFS
	err error
}

func (f *minimumFailureFS) MinimumSize(context.Context, string) (int64, error) {
	return 0, f.err
}

func TestPlanShrinkFailsClosedWhenMinimumSizeCannotBeProven(t *testing.T) {
	probeErr := errors.New("min-dev-size unavailable")
	fs := &minimumFailureFS{
		fakeFS: fakeFS{state: FilesystemState{Healthy: true, LogicalBytes: 100 << 30, UsedBytes: 20 << 30}},
		err:    probeErr,
	}
	storage := New(t.TempDir(), &fakeBlock{}, fs)

	plan, err := storage.PlanShrink(context.Background(), core.StorageHandle{ID: "local-default"}, 80<<30)
	if !errors.Is(err, probeErr) {
		t.Fatalf("expected minimum-size error, got plan=%+v err=%v", plan, err)
	}
	if plan.Feasible {
		t.Fatalf("unsafe plan was marked feasible when minimum size was unknown: %+v", plan)
	}
}
