package state

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSavedResourceSchemaUpgradeAndReceipt(t *testing.T) {
	store, snap := snapshotCatalogFixture(t)
	snap.State = "ready"
	for i := range snap.Components {
		snap.Components[i].State = "verified"
	}
	raw, err := os.ReadFile(store.path)
	mustSnapshot(t, err)
	var old environmentFileState
	mustSnapshot(t, json.Unmarshal(raw, &old))
	old.Version = 10
	old.Snapshots = map[string]core.Snapshot{snap.ID: snap}
	raw, err = json.Marshal(old)
	mustSnapshot(t, err)
	mustSnapshot(t, os.WriteFile(store.path, raw, 0600))
	target := core.PersistentResource{ID: "oci:restored", Owner: strings.Repeat("e", 32), Kind: "oci-containerd", NativeRef: "pool/restored", State: "creating", RestoreSource: snap.ID, CreatedAt: time.Now().UTC()}
	mustSnapshot(t, store.BeginSnapshotResourceRestore(context.Background(), snap, target))
	mustSnapshot(t, store.RecordPersistentResourceCreated(context.Background(), target))
	target.State = "created"
	raw, err = os.ReadFile(store.path)
	mustSnapshot(t, err)
	var current environmentFileState
	mustSnapshot(t, json.Unmarshal(raw, &current))
	if current.Version != 11 || !reflect.DeepEqual(current.Snapshots, old.Snapshots) || !reflect.DeepEqual(current.Environments, old.Environments) {
		t.Fatal("upgrade changed old data")
	}
	got, err := NewEnvironmentJSONStore(store.path).GetPersistentResource(context.Background(), target.ID)
	mustSnapshot(t, err)
	if got != target {
		t.Fatal("receipt lost")
	}
	current.Version = 10
	raw, err = json.Marshal(current)
	mustSnapshot(t, err)
	mustSnapshot(t, os.WriteFile(store.path, raw, 0600))
	if _, err := store.GetPersistentResource(context.Background(), target.ID); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatal("new receipt accepted as old schema", err)
	}
}
