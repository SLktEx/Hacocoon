package state

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func generationCopyFixture(t *testing.T) (*EnvironmentJSONStore, core.WorkspaceLease, core.PersistentResource, core.PersistentResource) {
	t.Helper()
	store, saved := savedDataFixture(t)
	ctx := context.Background()
	lease, err := store.GetWorkspaceLease(ctx, saved.Source.Environment.Name)
	mustSnapshot(t, err)
	source, err := store.GetPersistentResource(ctx, lease.Attachments[0].Resource.ID)
	mustSnapshot(t, err)
	target := core.PersistentResource{
		ID: "generation:" + strings.Repeat("d", 32), Owner: strings.Repeat("d", 32), Kind: source.Kind,
		NativeRef: "pool/publication", State: "creating", SourceOnly: true, CreatedAt: source.CreatedAt,
		CopySource: source.Ref(), Producer: source.Ref(), PublicationOrigin: lease.Attachments[0].Origin,
	}
	return store, lease, source, target
}

func TestGenerationCopyPinsExactProducerUntilCompletedCandidateIsReady(t *testing.T) {
	ctx := context.Background()
	store, lease, source, target := generationCopyFixture(t)
	savedBefore, err := store.ListSnapshots(ctx)
	mustSnapshot(t, err)
	mustSnapshot(t, store.BeginEnvironmentGenerationCopy(ctx, lease, source, target))
	store = NewEnvironmentJSONStore(store.path)
	held, err := store.GetPersistentResource(ctx, target.ID)
	mustSnapshot(t, err)
	if held != target {
		t.Fatal("publication lost its exact origin and producer", held)
	}
	for _, completed := range []bool{false, true} {
		if completed {
			mustSnapshot(t, store.MarkPersistentResourceCopyCompleted(ctx, held))
			store = NewEnvironmentJSONStore(store.path)
			held, err = store.GetPersistentResource(ctx, target.ID)
			mustSnapshot(t, err)
		}
		if err := store.CheckEnvironmentResourcesIdle(ctx, lease.EnvironmentID); !errors.Is(err, core.ErrRecoveryRequired) {
			t.Fatal("unfinished publication did not fence producer", completed, err)
		}
		if _, err := store.PrepareEnvironmentResourceDeletion(ctx, lease); !errors.Is(err, core.ErrRecoveryRequired) {
			t.Fatal("producer deletion released publication source", completed, err)
		}
		if _, err := store.BeginGenerationResourceDelete(ctx, target.Ref()); !errors.Is(err, core.ErrRecoveryRequired) {
			t.Fatal("unfinished publication deleted", completed, err)
		}
		if _, err := store.AdvanceResourceGeneration(ctx, target.PublicationOrigin, target.Ref()); !errors.Is(err, core.ErrIncompatibleState) {
			t.Fatal("unfinished candidate became current", completed, err)
		}
	}
	mustSnapshot(t, store.CommitPersistentResourceCreate(ctx, held))
	store = NewEnvironmentJSONStore(store.path)
	mustSnapshot(t, store.CheckEnvironmentResourcesIdle(ctx, lease.EnvironmentID))
	ready, err := store.GetPersistentResource(ctx, target.ID)
	mustSnapshot(t, err)
	want := target
	want.State, want.CopySource = "ready", core.PersistentResourceRef{}
	if ready != want {
		t.Fatal("committed publication lost provenance", ready)
	}
	selected, err := store.AdvanceResourceGeneration(ctx, target.PublicationOrigin, ready.Ref())
	mustSnapshot(t, err)
	if selected.Current != ready.Ref() || selected.Number != target.PublicationOrigin.Number+1 {
		t.Fatal("wrong generation selected", selected)
	}
	if retained, err := store.GetPersistentResource(ctx, source.ID); err != nil || retained != source {
		t.Fatal("publication altered producer data", retained, err)
	}
	if retained, err := store.GetWorkspaceLease(ctx, lease.EnvironmentID); err != nil || !retained.Equal(lease) {
		t.Fatal("publication altered parent lease", retained, err)
	}
	if savedAfter, err := store.ListSnapshots(ctx); err != nil || !reflect.DeepEqual(savedAfter, savedBefore) {
		t.Fatal("publication altered retained snapshots", savedAfter, err)
	}
}

func TestGenerationCopyRejectsChangedAuthorityWithoutPartialReservation(t *testing.T) {
	for _, defect := range []string{"parent-instance", "parent-absent", "source-owner", "source-instance", "source-kind", "source-not-ready", "origin-epoch", "producer", "writable-candidate", "candidate-workspace", "candidate-owner", "candidate-exists", "clear-pending", "snapshot-pending"} {
		t.Run(defect, func(t *testing.T) {
			ctx := context.Background()
			store, lease, source, target := generationCopyFixture(t)
			want := core.ErrCapabilityStale
			switch defect {
			case "parent-instance":
				lease.InstanceID = "env-" + strings.Repeat("f", 32)
			case "parent-absent":
				lease.EnvironmentID = "absent"
			case "source-owner":
				source.Owner = strings.Repeat("f", 32)
			case "source-instance":
				source.EnvironmentInstance = "env-" + strings.Repeat("f", 32)
			case "source-kind":
				source.Kind, target.Kind = "oci-containerd", "oci-containerd"
			case "source-not-ready":
				source.State = "clearing"
			case "origin-epoch":
				target.PublicationOrigin.Epoch = strings.Repeat("f", 32)
			case "producer":
				target.Producer = lease.Attachments[1].Resource
			case "writable-candidate":
				target.SourceOnly = false
			case "candidate-workspace":
				target.WorkspaceID = lease.WorkspaceID
			case "candidate-owner":
				target.Owner = source.Owner
				want = core.ErrInvalidArgument
			case "candidate-exists":
				existing := generationFixture(t, store, "d")
				target.ID = existing.ID
				want = core.ErrAlreadyExists
			case "clear-pending":
				_, err := store.BeginEnvironmentResourceClear(ctx, lease, source.Ref())
				mustSnapshot(t, err)
				want = core.ErrRecoveryRequired
			case "snapshot-pending":
				saved, err := store.ListSnapshots(ctx)
				mustSnapshot(t, err)
				pending := saved[0]
				pending.ID, pending.State = "snap-"+strings.Repeat("8", 32), "capturing"
				for i := range pending.Components {
					pending.Components[i].State = "planned"
					pending.Components[i].NativeRef += "-next"
				}
				mustSnapshot(t, store.BeginSnapshot(ctx, pending))
				want = core.ErrRecoveryRequired
			}
			before, err := os.ReadFile(store.path)
			mustSnapshot(t, err)
			if err := store.BeginEnvironmentGenerationCopy(ctx, lease, source, target); !errors.Is(err, want) {
				t.Fatal("invalid publication admitted", err)
			}
			after, err := os.ReadFile(store.path)
			mustSnapshot(t, err)
			if string(after) != string(before) {
				t.Fatal("rejected publication changed durable catalog")
			}
		})
	}
}
