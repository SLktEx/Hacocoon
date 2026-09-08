package oci

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/persistentresource"
	"github.com/SLktEx/Hacocoon/internal/state"
	"path/filepath"
	"testing"
)

type hostSetupBackend struct {
	*workspaceCopyBackend
	prepared, verified int
	fail               bool
}

func (b *hostSetupBackend) PrepareHostSource(ctx context.Context, r core.PersistentResource) error {
	b.prepared++
	stored, err := b.store.GetPersistentResource(ctx, HostStoreID)
	if err != nil || stored != r || !r.SourceOnly || r.State != "creating" {
		return errors.New("preparation preceded ownership")
	}
	if b.fail {
		return core.ErrRecoveryRequired
	}
	return nil
}
func (b *hostSetupBackend) VerifyHostSource(context.Context, core.PersistentResource) error {
	b.verified++
	return nil
}
func TestHostStoreSetupUsesCanonicalOwnershipAndReusesBinding(t *testing.T) {
	st := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
	b := &hostSetupBackend{workspaceCopyBackend: &workspaceCopyBackend{store: st}}
	s := WorkspaceStores{Resources: &persistentresource.Service{Store: st, Backend: b}}
	ctx := context.Background()
	if err := s.EnsureHost(ctx, b); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureHost(ctx, b); err != nil {
		t.Fatal(err)
	}
	if b.prepared != 1 || b.verified != 1 {
		t.Fatal("Host layout recreated")
	}
	source, err := st.GetPersistentResource(ctx, HostStoreID)
	if err != nil || source.State != "ready" || !source.SourceOnly {
		t.Fatal("source not committed")
	}
}
func TestIncompleteHostPreparationIsNotRecreatedOrPublished(t *testing.T) {
	st := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
	b := &hostSetupBackend{workspaceCopyBackend: &workspaceCopyBackend{store: st}, fail: true}
	s := WorkspaceStores{Resources: &persistentresource.Service{Store: st, Backend: b}}
	for i := 0; i < 2; i++ {
		if err := s.EnsureHost(context.Background(), b); !errors.Is(err, core.ErrRecoveryRequired) {
			t.Fatal(err)
		}
	}
	source, err := st.GetPersistentResource(context.Background(), HostStoreID)
	if err != nil || source.State != "creating" || b.prepared != 1 {
		t.Fatal("unfinished ownership lost")
	}
}
