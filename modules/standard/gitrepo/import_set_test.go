package gitrepo

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type collectionImportBackend struct {
	localBackend
	t         *testing.T
	service   *RepositoryService
	fail      string
	imported  []Object
	populated bool
	plans     int
}

func (b *collectionImportBackend) Plan(_ context.Context, _, id string) (string, error) {
	b.plans++
	if b.fail == "duplicate-ref" {
		return "pool/same", nil
	}
	return "pool/" + id, nil
}
func (b *collectionImportBackend) ImportWorkspaceVolume(_ context.Context, o Object, r io.ReadSeeker) error {
	record, err := b.service.readObject("work", "both")
	if err != nil || record.State != "creating" || len(record.Members) != 2 {
		b.t.Fatal("whole reservation absent", err)
	}
	i := len(b.imported)
	if !reflect.DeepEqual(record.Members[i], o) || o.State != "creating" {
		b.t.Fatal("member ownership not durable")
	}
	if _, err := b.service.Get("work", "both"); !errors.Is(err, core.ErrRecoveryRequired) {
		b.t.Fatal("partial collection published", err)
	}
	if _, err := b.service.Get("work", o.ID); !errors.Is(err, core.ErrNotFound) {
		b.t.Fatal("member separately published", err)
	}
	data, err := io.ReadAll(r)
	if err != nil || string(data) != o.Repository {
		b.t.Fatal("archive routing mismatch", err)
	}
	b.imported = append(b.imported, o)
	if b.fail == "second-create" && i == 1 {
		return errors.New("lost native reply")
	}
	return nil
}
func (b *collectionImportBackend) InspectVolume(_ context.Context, o Object) error {
	record, err := b.service.readObject("work", "both")
	if err != nil {
		b.t.Fatal(err)
	}
	for _, m := range record.Members {
		if m.ID == o.ID && !reflect.DeepEqual(m, o) {
			b.t.Fatal("created receipt not durable")
		}
	}
	if o.State != "created" {
		b.t.Fatal("wrong inspection state")
	}
	if b.fail == "second-inspect" && o.Repository == "two" {
		return errors.New("verification failed")
	}
	return nil
}
func (b *collectionImportBackend) Populate(context.Context, Object) error {
	b.populated = true
	return errors.New("import must not populate")
}
func collectionInputs() []WorkspaceImport {
	return []WorkspaceImport{
		{Repository: "one", Remote: "https://github.com/example/one.git", Branch: "main", Archive: bytes.NewReader([]byte("one"))},
		{Repository: "two", Remote: "https://github.com/example/two.git", Branch: "dev", Archive: bytes.NewReader([]byte("two"))},
	}
}
func TestWorkspaceCollectionImportKeepsAtomicOwnership(t *testing.T) {
	for _, failure := range []string{"", "second-create", "second-inspect"} {
		t.Run(failure, func(t *testing.T) {
			b := &collectionImportBackend{t: t, fail: failure}
			s := NewRepositoryService(t.TempDir(), b)
			b.service = s
			o, err := s.ImportWorkspaceSet(context.Background(), "both", collectionInputs())
			if b.populated || len(b.imported) != 2 || len(o.Members) != 2 {
				t.Fatal(o, err)
			}
			if o.Owner == o.Members[0].Owner || o.Owner == o.Members[1].Owner || o.Members[0].Owner == o.Members[1].Owner {
				t.Fatal("owners reused")
			}
			saved, readErr := s.readObject("work", "both")
			if readErr != nil || !reflect.DeepEqual(saved, o) {
				t.Fatal("receipt lost", readErr)
			}
			if failure == "" {
				if err != nil || o.State != "ready" || o.Members[0].State != "ready" || o.Members[1].State != "ready" {
					t.Fatal(o, err)
				}
			} else {
				want := "creating"
				if failure == "second-inspect" {
					want = "created"
				}
				if !errors.Is(err, core.ErrRecoveryRequired) || o.State != "creating" || o.Members[0].State != "ready" || o.Members[1].State != want {
					t.Fatal(o, err)
				}
				if _, err := s.Get("work", "both"); !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatal("partial collection usable", err)
				}
			}
			if _, err := s.ImportWorkspaceSet(context.Background(), "both", collectionInputs()); !errors.Is(err, core.ErrAlreadyExists) || len(b.imported) != 2 {
				t.Fatal("duplicate repeated native import", err)
			}
			for _, m := range o.Members {
				if _, err := s.Get("work", m.ID); !errors.Is(err, core.ErrNotFound) {
					t.Fatal("member independently usable", err)
				}
			}
		})
	}
}
func TestWorkspaceCollectionImportRejectsBeforeReservation(t *testing.T) {
	for _, failure := range []string{"duplicate", "local", "credential", "nil", "one", "duplicate-ref"} {
		t.Run(failure, func(t *testing.T) {
			b := &collectionImportBackend{t: t, fail: failure}
			s := NewRepositoryService(t.TempDir(), b)
			b.service = s
			in := collectionInputs()
			switch failure {
			case "duplicate":
				in[1].Repository = in[0].Repository
			case "local":
				in[1].Remote = "file:///host/private"
			case "credential":
				in[1].Remote = "https://token@github.com/example/two"
			case "nil":
				in[1].Archive = nil
			case "one":
				in = in[:1]
			}
			if _, err := s.ImportWorkspaceSet(context.Background(), "both", in); err == nil {
				t.Fatal("invalid collection accepted")
			}
			if len(b.imported) != 0 {
				t.Fatal("invalid collection touched native storage")
			}
			if b.plans != 0 && failure != "duplicate-ref" {
				t.Fatal("invalid input reached planning")
			}
			if objects, err := s.ListWorkspaces(context.Background()); err != nil || len(objects) != 0 {
				t.Fatal("invalid input reserved", err)
			}
		})
	}
}
