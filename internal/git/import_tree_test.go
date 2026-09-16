package gitrepo

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type treeImportBackend struct {
	ownershipBackend
	imports int
}

func (b *treeImportBackend) InspectVolume(ctx context.Context, o Object) error {
	if o.Kind == "repo" {
		return nil
	}
	return b.ownershipBackend.InspectVolume(ctx, o)
}
func (b *treeImportBackend) ImportWorkspaceTreeVolume(ctx context.Context, o Object, r io.Reader) error {
	b.imports++
	raw, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if string(raw) != "input" {
		b.t.Fatal("wrong body")
	}
	return b.CreateVolume(ctx, o, nil)
}
func TestTreeImportPinsRegisteredRoutingAndCommonCreationReceipt(t *testing.T) {
	for _, fail := range []string{"", "create", "inspect"} {
		b := &treeImportBackend{ownershipBackend: ownershipBackend{t: t, fail: fail}}
		s := NewRepositoryService(t.TempDir(), b)
		b.service = s
		repo := Object{ID: "source", Kind: "repo", Repository: "source", Remote: "https://github.com/example/source.git", Owner: strings.Repeat("a", 32), NativeRef: "source-volume", State: "ready"}
		if err := s.reserve(repo); err != nil {
			t.Fatal(err)
		}
		object, err := s.ImportWorkspaceTree(context.Background(), "input", "source", strings.NewReader("input"))
		if (err != nil) != (fail != "") || object.Owner == "" || object.Remote != repo.Remote || object.Branch != repo.Branch || b.populated || b.imports != 1 {
			t.Fatal(object, err)
		}
		_, err = s.ImportWorkspaceTree(context.Background(), "input", "source", strings.NewReader("input"))
		if !errors.Is(err, core.ErrAlreadyExists) || b.imports != 1 {
			t.Fatal("duplicate import", err)
		}
		_, err = s.ImportWorkspaceTree(context.Background(), "other", "missing", strings.NewReader("input"))
		if !errors.Is(err, core.ErrNotFound) || b.imports != 1 {
			t.Fatal("guessed source", err)
		}
	}
}
