package oci

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type selectionExecutor struct{}

func (selectionExecutor) ExecEnvironment(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error) {
	return core.ExecutionResult{}, nil
}

func newSelectionService(t *testing.T) (*Service, *Store) {
	t.Helper()
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "oci-usage.json"))
	service, err := New(selectionExecutor{}, filepath.Join(dir, "environments.json"), store, DriverNerdctl)
	if err != nil {
		t.Fatal(err)
	}
	return service, store
}

func TestPinSeedImageOutsideAutoPromotionStillSelects(t *testing.T) {
	service, store := newSelectionService(t)
	now := time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	var target string
	for i := 0; i < 11; i++ {
		digest := fmt.Sprintf("sha256:%064x", i+1)
		reference := fmt.Sprintf("example.invalid/image-%02d:latest", i)
		if i == 10 {
			target = reference + "@" + digest
		}
		if err := store.Put(context.Background(), Snapshot{
			Environment: fmt.Sprintf("env-%02d", i),
			SampledAt:   now,
			Images: []Image{{
				Repository: fmt.Sprintf("example.invalid/image-%02d", i),
				Tag:        "latest",
				Digest:     digest,
			}},
		}); err != nil {
			t.Fatal(err)
		}
	}
	before, err := service.Recommend(context.Background(), 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 11 || before[10].AutoPromote {
		t.Fatalf("unexpected automatic ranking: %#v", before)
	}

	selection, err := service.PinSeedImage(context.Background(), target, false)
	if err != nil {
		t.Fatal(err)
	}
	if !selection.Pinned || selection.Deleted || selection.Reenabled {
		t.Fatalf("selection=%#v", selection)
	}
	after, err := service.Recommend(context.Background(), 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, recommendation := range after {
		if recommendation.Reference+"@"+recommendation.Digest != target {
			continue
		}
		found = true
		if recommendation.AutoPromote || !recommendation.Pinned || recommendation.PinnedAt == nil {
			t.Fatalf("pinned recommendation=%#v", recommendation)
		}
	}
	if !found {
		t.Fatalf("pinned target %q missing from recommendations: %#v", target, after)
	}
}

func TestPinDeletedSeedImageRequiresExplicitReenable(t *testing.T) {
	service, store := newSelectionService(t)
	digest := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	identity := "docker.io/library/node:24@" + digest
	deletedAt := time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)
	if err := store.PutDeletion(context.Background(), Deletion{
		Reference: "docker.io/library/node:24",
		Digest:    digest,
		DeletedAt: deletedAt,
	}); err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return deletedAt.Add(time.Minute) }

	selection, err := service.PinSeedImage(context.Background(), identity, false)
	if !errors.Is(err, core.ErrInvalidArgument) || !selection.Deleted || selection.Pinned {
		t.Fatalf("selection=%#v err=%v", selection, err)
	}
	selection, err = service.PinSeedImage(context.Background(), identity, true)
	if err != nil {
		t.Fatal(err)
	}
	if !selection.Pinned || !selection.Reenabled || selection.Deleted || selection.ReenabledAt == nil {
		t.Fatalf("selection=%#v", selection)
	}

	recommendations, err := service.Recommend(context.Background(), 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(recommendations) != 1 || !recommendations[0].Pinned || !recommendations[0].Reenabled || recommendations[0].AutoPromote {
		t.Fatalf("recommendations=%#v", recommendations)
	}
}

func TestReenableRestoresDeletedObservedImageWithoutPinning(t *testing.T) {
	service, store := newSelectionService(t)
	digest := "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	identity := "docker.io/library/postgres:18@" + digest
	deletedAt := time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)
	if err := store.Put(context.Background(), Snapshot{
		Environment: "a",
		SampledAt:   deletedAt,
		Images:      []Image{{Repository: "docker.io/library/postgres", Tag: "18", Digest: digest}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.PutDeletion(context.Background(), Deletion{Reference: "docker.io/library/postgres:18", Digest: digest, DeletedAt: deletedAt}); err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return deletedAt.Add(time.Minute) }

	before, err := service.Recommend(context.Background(), 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 0 {
		t.Fatalf("deleted image survived before re-enable: %#v", before)
	}
	selection, err := service.ReenableSeedImage(context.Background(), identity)
	if err != nil {
		t.Fatal(err)
	}
	if selection.Pinned || !selection.Reenabled || selection.Deleted {
		t.Fatalf("selection=%#v", selection)
	}
	after, err := service.Recommend(context.Background(), 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 1 || after[0].Pinned || !after[0].Reenabled || !after[0].AutoPromote {
		t.Fatalf("recommendations=%#v", after)
	}
}

func TestLaterDeletionSupersedesReenableAndExistingPin(t *testing.T) {
	service, store := newSelectionService(t)
	digest := "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	identity := "example.invalid/app:stable@" + digest
	t1 := time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)
	if err := store.PutDeletion(context.Background(), Deletion{Reference: "example.invalid/app:stable", Digest: digest, DeletedAt: t1}); err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return t1.Add(time.Minute) }
	if _, err := service.PinSeedImage(context.Background(), identity, true); err != nil {
		t.Fatal(err)
	}
	t3 := t1.Add(2 * time.Minute)
	if err := store.PutDeletion(context.Background(), Deletion{Reference: "example.invalid/app:stable", Digest: digest, DeletedAt: t3}); err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return t3.Add(time.Minute) }

	selection, err := service.seedSelection(context.Background(), immutableImageIdentity{Reference: "example.invalid/app:stable", Digest: digest})
	if err != nil {
		t.Fatal(err)
	}
	if !selection.Pinned || selection.Reenabled || !selection.Deleted {
		t.Fatalf("selection=%#v", selection)
	}
	recommendations, err := service.Recommend(context.Background(), 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(recommendations) != 0 {
		t.Fatalf("later deletion must suppress pinned image: %#v", recommendations)
	}
}

func TestUnpinDoesNotEraseReenableDecision(t *testing.T) {
	service, store := newSelectionService(t)
	digest := "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	identity := "example.invalid/tool:latest@" + digest
	t1 := time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)
	if err := store.PutDeletion(context.Background(), Deletion{Reference: "example.invalid/tool:latest", Digest: digest, DeletedAt: t1}); err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return t1.Add(time.Minute) }
	if _, err := service.PinSeedImage(context.Background(), identity, true); err != nil {
		t.Fatal(err)
	}
	selection, err := service.UnpinSeedImage(context.Background(), identity)
	if err != nil {
		t.Fatal(err)
	}
	if selection.Pinned || !selection.Reenabled || selection.Deleted {
		t.Fatalf("selection=%#v", selection)
	}
}

func TestReenableRequiresExistingDeletionTombstone(t *testing.T) {
	service, _ := newSelectionService(t)
	service.now = func() time.Time { return time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC) }
	_, err := service.ReenableSeedImage(context.Background(), "example.invalid/app:latest@sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee")
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestSeedSelectionStateRejectsTamperedKey(t *testing.T) {
	service, store := newSelectionService(t)
	path := store.selectionPath()
	if err := writeSeedSelectionFile(path, seedSelectionFileState{
		Version: seedSelectionStateVersion,
		Pins: map[string]SeedPin{
			"wrong@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": {
				Reference: "example.invalid/app:latest",
				Digest:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				PinnedAt:  time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC),
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	_, err := service.Recommend(context.Background(), 24*time.Hour)
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("err=%v", err)
	}
}
