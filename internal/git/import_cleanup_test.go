package gitrepo

import (
	"bytes"
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"testing"
)

type failedImportCleanupBackend struct {
	workspaceImportBackend
	mode    string
	deletes int
	cancel  context.CancelFunc
}

func (b *failedImportCleanupBackend) InspectVolume(ctx context.Context, o Object) error {
	b.record(o, "created")
	switch b.mode {
	case "owner-changed":
		o.Owner = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		if err := b.service.save(o); err != nil {
			b.t.Fatal(err)
		}
	case "published":
		o.State = "ready"
		if err := b.service.save(o); err != nil {
			b.t.Fatal(err)
		}
	case "canceled":
		b.cancel()
	}
	return errors.New("injected verification failure")
}
func (b *failedImportCleanupBackend) DeleteWorkspaceVolume(ctx context.Context, o Object) error {
	b.deletes++
	b.record(o, "created")
	if ctx.Err() != nil {
		b.t.Fatal("cleanup inherited canceled request")
	}
	if _, ok := ctx.Deadline(); !ok {
		b.t.Fatal("cleanup has no deadline")
	}
	if b.mode == "cleanup-failed" {
		return errors.New("absence unknown")
	}
	return nil
}
func TestFailedWorkspaceImportCleansOnlyCompletedUnpublishedCreation(t *testing.T) {
	for _, mode := range []string{"cleaned", "cleanup-failed", "owner-changed", "published", "canceled", "creation-unknown"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			b := &failedImportCleanupBackend{workspaceImportBackend: workspaceImportBackend{ownershipBackend: ownershipBackend{t: t}}, mode: mode, cancel: cancel}
			if mode == "creation-unknown" {
				b.fail = "create"
			}
			s := NewRepositoryService(t.TempDir(), b)
			b.service = s
			got, err := s.ImportWorkspace(ctx, "imported", "repo", "https://github.com/SLktEx/Hacocoon-test.git", "main", bytes.NewReader([]byte("saved data")))
			if err == nil {
				t.Fatal("cleanup hid import failure")
			}
			saved, readErr := s.readObject("work", "imported")
			if mode == "cleaned" || mode == "canceled" {
				if got.ID != "" || b.deletes != 1 || !errors.Is(readErr, core.ErrNotFound) || !errors.Is(err, core.ErrRuntimeUnavailable) || errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatal("completed cleanup not reported accurately", got, err, readErr)
				}
			} else {
				if got.ID == "" || readErr != nil || !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatal("lost residue identity", got, err, readErr)
				}
				want := 0
				if mode == "cleanup-failed" {
					want = 1
				}
				if b.deletes != want {
					t.Fatal("unknown, changed or published creation deleted", b.deletes)
				}
				if mode == "creation-unknown" && saved.State != "creating" {
					t.Fatal("unknown native creation state lost")
				}
				if mode == "published" && saved.State != "ready" {
					t.Fatal("published record changed")
				}
			}
		})
	}
}
