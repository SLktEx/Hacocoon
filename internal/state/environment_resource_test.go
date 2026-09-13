package state

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func environmentDataFixture(t *testing.T, count int) (*EnvironmentJSONStore, core.WorkspaceLease, []core.EnvironmentResourcePlan) {
	t.Helper()
	s := NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
	lease := core.WorkspaceLease{EnvironmentID: "cache-env", Owner: "cache-env", InstanceID: "env-" + strings.Repeat("e", 32), WorkspaceID: "workspace", SourcePath: "/work", AccessMode: core.WorkspaceReadWrite, State: core.WorkspaceLeaseAcquiring, AcquiredAt: time.Now().UTC()}
	plans := make([]core.EnvironmentResourcePlan, 0, count)
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("cache-%02d", i)
		g, err := s.EnsureResourceGeneration(context.Background(), name, "build-cache", strings.Repeat("a", 64))
		if err != nil {
			t.Fatal(err)
		}
		r := core.PersistentResource{ID: fmt.Sprintf("env-data:%032x", i+1), Owner: fmt.Sprintf("%032x", i+10), EnvironmentInstance: lease.InstanceID, Kind: g.Kind, NativeRef: "pool/" + name, State: "planned", CreatedAt: lease.AcquiredAt}
		a := core.EnvironmentAttachment{Key: name, Target: "/home/dev/.cache/" + name, Resource: r.Ref(), Origin: g}
		lease.Attachments = append(lease.Attachments, a)
		plans = append(plans, core.EnvironmentResourcePlan{Attachment: a, Resource: r})
	}
	return s, lease, plans
}

func TestEnvironmentDataReservationIsAtomic(t *testing.T) {
	for _, fault := range []string{"stale-second", "foreign-owner", "duplicate-native", "mismatch", "generic-create"} {
		t.Run(fault, func(t *testing.T) {
			s, l, p := environmentDataFixture(t, 2)
			ctx := context.Background()
			before, _ := os.ReadFile(s.path)
			switch fault {
			case "stale-second":
				p[1].Attachment.Origin.Epoch = strings.Repeat("f", 32)
				l.Attachments[1] = p[1].Attachment
			case "foreign-owner":
				p[1].Resource.EnvironmentInstance = "env-" + strings.Repeat("f", 32)
			case "duplicate-native":
				p[1].Resource.NativeRef = p[0].Resource.NativeRef
			case "mismatch":
				p[1].Attachment.Target = "/other"
			}
			var err error
			if fault == "generic-create" {
				err = s.BeginEnvironmentCreate(ctx, l)
			} else {
				err = s.BeginEnvironmentCreateWithResources(ctx, l, p)
			}
			if err == nil {
				t.Fatal("invalid aggregate admitted")
			}
			after, _ := os.ReadFile(s.path)
			if string(before) != string(after) {
				t.Fatal("partial reservation persisted")
			}
			if _, err = s.GetWorkspaceLease(ctx, l.EnvironmentID); !errors.Is(err, core.ErrNotFound) {
				t.Fatal(err)
			}
		})
	}
}

func TestEnvironmentDataDeleteKeepsParentUntilAllChildrenAbsent(t *testing.T) {
	s, l, p := environmentDataFixture(t, 2)
	ctx := context.Background()
	if err := s.BeginEnvironmentCreateWithResources(ctx, l, p); err != nil {
		t.Fatal(err)
	}
	for _, a := range l.Attachments {
		if _, err := s.BeginPersistentResourceDelete(ctx, a.Resource.ID); !errors.Is(err, core.ErrStorageBusy) {
			t.Fatal(err)
		}
		if _, err := s.BeginEnvironmentResourceDelete(ctx, l.InstanceID, a.Resource); !errors.Is(err, core.ErrStorageBusy) {
			t.Fatal(err)
		}
	}
	r, err := s.BeginEnvironmentResourceMaterialization(ctx, l, l.Attachments[0].Resource)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CommitPersistentResourceCreate(ctx, r); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("creation without completion receipt: %v", err)
	}
	r, err = s.RecordEnvironmentResourceCreated(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CommitPersistentResourceCreate(ctx, r); err != nil {
		t.Fatal(err)
	}
	if err = s.FinalizeEnvironmentDelete(ctx, l.EnvironmentID); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal(err)
	}
	absent, err := s.PrepareEnvironmentResourceDeletion(ctx, l)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.BeginEnvironmentResourceMaterialization(ctx, l, l.Attachments[1].Resource); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatal(err)
	}
	if _, err = s.BeginEnvironmentResourceDelete(ctx, "env-"+strings.Repeat("f", 32), l.Attachments[0].Resource); !errors.Is(err, core.ErrStorageBusy) {
		t.Fatal(err)
	}
	for i, a := range l.Attachments {
		deleting, err := s.BeginEnvironmentResourceDelete(ctx, l.InstanceID, a.Resource)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.FinalizeEnvironmentDelete(ctx, l.EnvironmentID); !errors.Is(err, core.ErrRecoveryRequired) {
			t.Fatal(err)
		}
		if err = s.FinalizePersistentResourceDelete(ctx, deleting); err != nil {
			t.Fatal(err)
		}
		held, err := s.GetWorkspaceLease(ctx, l.EnvironmentID)
		if err != nil || !held.Equal(absent) {
			t.Fatal(held, err)
		}
		if i == 0 {
			if err = s.FinalizeEnvironmentDelete(ctx, l.EnvironmentID); !errors.Is(err, core.ErrRecoveryRequired) {
				t.Fatal(err)
			}
		}
	}
	if err = s.FinalizeEnvironmentDelete(ctx, l.EnvironmentID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.GetWorkspaceLease(ctx, l.EnvironmentID); !errors.Is(err, core.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestEnvironmentDataUnknownCreateRetainsOwner(t *testing.T) {
	s, l, p := environmentDataFixture(t, 1)
	ctx := context.Background()
	if err := s.BeginEnvironmentCreateWithResources(ctx, l, p); err != nil {
		t.Fatal(err)
	}
	r, err := s.BeginEnvironmentResourceMaterialization(ctx, l, l.Attachments[0].Resource)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.PrepareEnvironmentResourceDeletion(ctx, l); err != nil {
		t.Fatal(err)
	}
	if _, err = s.BeginEnvironmentResourceDelete(ctx, l.InstanceID, r.Ref()); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal(err)
	}
	if _, err = s.RecordEnvironmentResourceCreated(ctx, r); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatal(err)
	}
	if err = s.FinalizeEnvironmentDelete(ctx, l.EnvironmentID); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal(err)
	}
	held, err := s.GetPersistentResource(ctx, r.ID)
	if err != nil || held != r {
		t.Fatal(held, err)
	}
}

func TestEnvironmentDataPlannedCopyPinsSourceUntilCancelled(t *testing.T) {
	s, l, p := environmentDataFixture(t, 1)
	ctx := context.Background()
	source := generationFixture(t, s, "b")
	g, err := s.AdvanceResourceGeneration(ctx, p[0].Attachment.Origin, source.Ref())
	if err != nil {
		t.Fatal(err)
	}
	p[0].Attachment.Origin = g
	p[0].Resource.CopySource = g.Current
	l.Attachments[0] = p[0].Attachment
	if err = s.BeginEnvironmentCreateWithResources(ctx, l, p); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ResetResourceGeneration(ctx, g, g.Compatibility); err != nil {
		t.Fatal(err)
	}
	if _, err = s.BeginGenerationResourceDelete(ctx, source.Ref()); !errors.Is(err, core.ErrStorageBusy) {
		t.Fatal(err)
	}
	if _, err = s.PrepareEnvironmentResourceDeletion(ctx, l); err != nil {
		t.Fatal(err)
	}
	child, err := s.BeginEnvironmentResourceDelete(ctx, l.InstanceID, p[0].Resource.Ref())
	if err != nil {
		t.Fatal(err)
	}
	if child.CopySource != (core.PersistentResourceRef{}) {
		t.Fatal("cancelled plan retained source pin")
	}
	if err = s.FinalizePersistentResourceDelete(ctx, child); err != nil {
		t.Fatal(err)
	}
	if _, err = s.BeginGenerationResourceDelete(ctx, source.Ref()); err != nil {
		t.Fatal(err)
	}
}

func TestEnvironmentDataClaimIsExclusiveAcrossStoreHandles(t *testing.T) {
	s, l, p := environmentDataFixture(t, 1)
	ctx := context.Background()
	if err := s.BeginEnvironmentCreateWithResources(ctx, l, p); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	start := make(chan struct{})
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := NewEnvironmentJSONStore(s.path).BeginEnvironmentResourceMaterialization(ctx, l, p[0].Resource.Ref())
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, core.ErrRecoveryRequired) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatal(success)
	}
}

func TestEnvironmentDataCatalogCorruptionFailsClosedWithoutMigration(t *testing.T) {
	cases := map[string]func(*environmentFileState){
		"old-format":       func(d *environmentFileState) { d.Version = 15 },
		"missing-resource": func(d *environmentFileState) { d.PersistentResources = map[string]core.PersistentResource{} },
		"missing-parent":   func(d *environmentFileState) { d.Leases = map[string]core.WorkspaceLease{} },
		"invalid-state":    func(d *environmentFileState) { l := d.Leases["cache-env"]; l.State = ""; d.Leases[l.EnvironmentID] = l },
		"active-without-env": func(d *environmentFileState) {
			l := d.Leases["cache-env"]
			l.State = core.WorkspaceLeaseActive
			d.Leases[l.EnvironmentID] = l
		},
		"runtime-absence-without-cleanup": func(d *environmentFileState) {
			l := d.Leases["cache-env"]
			l.RuntimeAbsent = true
			d.Leases[l.EnvironmentID] = l
		},
		"alias-instance": func(d *environmentFileState) {
			l := d.Leases["cache-env"]
			l.EnvironmentID = "other"
			l.Attachments = nil
			d.Leases[l.EnvironmentID] = l
		},
		"foreign-resource": func(d *environmentFileState) {
			for id, r := range d.PersistentResources {
				r.EnvironmentInstance = "env-" + strings.Repeat("f", 32)
				d.PersistentResources[id] = r
			}
		},
		"overlapping-target": func(d *environmentFileState) {
			l := d.Leases["cache-env"]
			l.Attachments = slices.Clone(l.Attachments)
			l.Attachments[1].Target = l.Attachments[0].Target + "/child"
			d.Leases[l.EnvironmentID] = l
		},
	}
	for name, corrupt := range cases {
		t.Run(name, func(t *testing.T) {
			s, l, p := environmentDataFixture(t, 2)
			ctx := context.Background()
			if err := s.BeginEnvironmentCreateWithResources(ctx, l, p); err != nil {
				t.Fatal(err)
			}
			data, err := s.readEnvironments()
			if err != nil {
				t.Fatal(err)
			}
			corrupt(&data)
			writeGenerationFixture(t, s.path, data)
			before, _ := os.ReadFile(s.path)
			if _, err = s.GetWorkspaceLease(ctx, l.EnvironmentID); !errors.Is(err, core.ErrIncompatibleState) {
				t.Fatal(err)
			}
			if _, err = s.EnvironmentInstance(ctx, core.Environment{Name: l.EnvironmentID}); !errors.Is(err, core.ErrIncompatibleState) {
				t.Fatal(err)
			}
			after, _ := os.ReadFile(s.path)
			if string(before) != string(after) {
				t.Fatal("read repaired corrupt state")
			}
		})
	}
}
