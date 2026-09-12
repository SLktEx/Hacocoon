package state

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func readyWorkspaceCopySnapshot(t *testing.T) (*EnvironmentJSONStore, core.Snapshot) {
	s, saved := snapshotCatalogFixture(t)
	ctx := context.Background()
	mustSnapshot(t, s.BeginSnapshot(ctx, saved))
	for _, c := range saved.Components {
		mustSnapshot(t, s.RecordSnapshotComponent(ctx, saved.ID, c, "created"))
		c.State = "created"
		mustSnapshot(t, s.RecordSnapshotComponent(ctx, saved.ID, c, "verified"))
	}
	mustSnapshot(t, s.CommitSnapshot(ctx, saved.ID))
	saved, err := s.GetSnapshot(ctx, saved.ID)
	mustSnapshot(t, err)
	return s, saved
}
func TestSnapshotWorkspaceCopyOwnershipSurvivesRestart(t *testing.T) {
	s, saved := readyWorkspaceCopySnapshot(t)
	ctx := context.Background()
	owner := strings.Repeat("d", 32)
	mustSnapshot(t, s.BeginSnapshotWorkspaceCopy(ctx, saved, "restored", owner))
	s = NewEnvironmentJSONStore(s.path)
	if err := s.BeginSnapshotDelete(ctx, saved.ID); !errors.Is(err, core.ErrStorageBusy) {
		t.Fatal("source deleted during copy", err)
	}
	if err := s.FinishSnapshotWorkspaceCopy(ctx, saved.ID, "restored", strings.Repeat("e", 32)); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatal("foreign owner released hold", err)
	}
	if err := s.BeginSnapshotWorkspaceCopy(ctx, saved, "restored", owner); !errors.Is(err, core.ErrAlreadyExists) {
		t.Fatal(err)
	}
	mustSnapshot(t, s.FinishSnapshotWorkspaceCopy(ctx, saved.ID, "restored", owner))
	mustSnapshot(t, s.BeginSnapshotDelete(ctx, saved.ID))
	if err := s.BeginSnapshotWorkspaceCopy(ctx, saved, "another", owner); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatal("deleting source accepted", err)
	}
}
func TestSnapshotWorkspaceCopyDeleteRace(t *testing.T) {
	for i := 0; i < 20; i++ {
		s, saved := readyWorkspaceCopySnapshot(t)
		ctx := context.Background()
		start := make(chan struct{})
		copied := make(chan error, 1)
		deleted := make(chan error, 1)
		go func() {
			<-start
			copied <- NewEnvironmentJSONStore(s.path).BeginSnapshotWorkspaceCopy(ctx, saved, "restored", strings.Repeat("d", 32))
		}()
		go func() { <-start; deleted <- NewEnvironmentJSONStore(s.path).BeginSnapshotDelete(ctx, saved.ID) }()
		close(start)
		a, b := <-copied, <-deleted
		if a == nil {
			if !errors.Is(b, core.ErrStorageBusy) {
				t.Fatal(a, b)
			}
		} else if !errors.Is(a, core.ErrCapabilityStale) || b != nil {
			t.Fatal(a, b)
		}
	}
}
func TestSnapshotWorkspaceCopySchemaPreservation(t *testing.T) {
	for _, version := range []int{12, 13} {
		s, saved := readyWorkspaceCopySnapshot(t)
		ctx := context.Background()
		owner := strings.Repeat("d", 32)
		mustSnapshot(t, s.BeginSnapshotWorkspaceCopy(ctx, saved, "restored", owner))
		raw, err := os.ReadFile(s.path)
		mustSnapshot(t, err)
		var d environmentFileState
		mustSnapshot(t, json.Unmarshal(raw, &d))
		d.Version = version
		raw, err = json.Marshal(d)
		mustSnapshot(t, err)
		mustSnapshot(t, os.WriteFile(s.path, raw, 0600))
		_, err = NewEnvironmentJSONStore(s.path).GetSnapshot(ctx, saved.ID)
		if version == 12 {
			if !errors.Is(err, core.ErrIncompatibleState) {
				t.Fatal("accepted relabeled source hold", err)
			}
		} else {
			mustSnapshot(t, err)
		}
	}
}
