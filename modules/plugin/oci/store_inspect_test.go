package oci

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"reflect"
	"strings"
	"testing"
)

type storeUseCatalog struct {
	envs      []core.Environment
	leases    []core.WorkspaceLease
	snapshots []core.Snapshot
	err       error
}

func (c storeUseCatalog) ListEnvironments(context.Context) ([]core.Environment, error) {
	return c.envs, c.err
}
func (c storeUseCatalog) ListWorkspaceLeases(context.Context) ([]core.WorkspaceLease, error) {
	return c.leases, c.err
}
func (c storeUseCatalog) ListSnapshots(context.Context) ([]core.Snapshot, error) {
	return c.snapshots, c.err
}
func TestStoreUsesSeparatesUsersCopiesAndIndependentProvenance(t *testing.T) {
	r := core.PersistentResource{ID: "oci:dev", Owner: strings.Repeat("a", 32), Kind: StoreKind}
	source := r
	source.ID = "oci:source"
	source.SourceOnly = true
	copy := r
	copy.ID = "oci:copy"
	copy.CopySource = r.Ref()
	saved := core.Snapshot{ID: "saved"}
	saved.Source.Environment.PersistentResource = r.Ref()
	old := saved
	old.ID = "old-owner"
	old.Source.Environment.PersistentResource.Owner = strings.Repeat("b", 32)
	c := storeUseCatalog{envs: []core.Environment{{Name: "z", PersistentResource: r.Ref()}}, leases: []core.WorkspaceLease{{EnvironmentID: "a", PersistentResource: r.Ref()}, {EnvironmentID: "z", PersistentResource: r.Ref()}}, snapshots: []core.Snapshot{saved, old}}
	uses, err := StoreUses(context.Background(), c, []core.PersistentResource{r, source, copy})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(uses[0].Environments, []string{"a", "z"}) || !reflect.DeepEqual(uses[0].IndependentSnapshots, []string{"saved"}) || !reflect.DeepEqual(uses[0].PendingCopies, []string{"oci:copy"}) || uses[1].Role != "host-source" {
		t.Fatalf("%+v", uses)
	}
	c.err = core.ErrRecoveryRequired
	if _, err := StoreUses(context.Background(), c, []core.PersistentResource{r}); !errors.Is(err, c.err) {
		t.Fatal("unavailable catalog became empty references")
	}
}
