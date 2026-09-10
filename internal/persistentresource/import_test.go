package persistentresource_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/persistentresource"
	"github.com/SLktEx/Hacocoon/internal/state"
)

type importResourceBackend struct {
	*backend
	imports int
}

func (b *importResourceBackend) Import(ctx context.Context, r core.PersistentResource, source io.ReadSeeker) error {
	b.imports++
	data, err := io.ReadAll(source)
	if err != nil || string(data) != "archive" {
		return errors.New("incorrect archive")
	}
	return b.backend.Create(ctx, r)
}
func TestResourceImportUsesCanonicalFreshOwnershipAndRetainsFailure(t *testing.T) {
	for _, failure := range []string{"", "create", "verify"} {
		t.Run(failure, func(t *testing.T) {
			ctx := context.Background()
			store := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
			b := &importResourceBackend{backend: &backend{store: store, fail: failure}}
			service := persistentresource.Service{Store: store, Backend: b}
			resource, err := service.Import(ctx, "oci:imported", "oci-containerd", bytes.NewReader([]byte("archive")))
			if resource.Owner == "" || resource.NativeRef == "" || resource.SourceOnly || b.imports != 1 {
				t.Fatal("ownership missing", resource, err)
			}
			if failure != "" {
				if !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatal(err)
				}
			} else if err != nil || resource.State != "ready" {
				t.Fatal(resource, err)
			}
			saved, e := store.GetPersistentResource(ctx, resource.ID)
			if e != nil || saved.Owner != resource.Owner {
				t.Fatal("lost import identity", e)
			}
			if failure != "" && saved.State != "creating" {
				t.Fatal("failed import published")
			}
			if _, err := service.Import(ctx, resource.ID, resource.Kind, bytes.NewReader([]byte("archive"))); !errors.Is(err, core.ErrAlreadyExists) || b.imports != 1 {
				t.Fatal("duplicate imported", err)
			}
			if err := service.Delete(ctx, resource.ID); err != nil {
				t.Fatal(err)
			}
		})
	}
}
