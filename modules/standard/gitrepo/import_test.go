package gitrepo

import (
	"bytes"
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
	"testing"
)

type workspaceImportBackend struct {
	ownershipBackend
	imports int
}

func (b *workspaceImportBackend) ImportWorkspaceVolume(ctx context.Context, o Object, r io.ReadSeeker) error {
	b.imports++
	data, err := io.ReadAll(r)
	if err != nil || string(data) != "saved data" {
		b.t.Fatal("wrong archive", err)
	}
	return b.CreateVolume(ctx, o, nil)
}
func TestWorkspaceImportUsesExistingOwnershipWithoutGitPopulation(t *testing.T) {
	for _, failure := range []string{"", "create", "inspect"} {
		t.Run(failure, func(t *testing.T) {
			b := &workspaceImportBackend{ownershipBackend: ownershipBackend{t: t, fail: failure}}
			s := NewRepositoryService(t.TempDir(), b)
			b.service = s
			o, err := s.ImportWorkspace(context.Background(), "imported", "repo", "https://github.com/SLktEx/Hacocoon-test.git", "main", bytes.NewReader([]byte("saved data")))
			if o.Owner == "" || o.NativeRef == "" || b.imports != 1 || b.populated {
				t.Fatal("import bypassed ownership or ran Git", o, err)
			}
			if failure == "" {
				if err != nil || o.State != "ready" {
					t.Fatal(o, err)
				}
			} else if !errors.Is(err, core.ErrRecoveryRequired) {
				t.Fatal(err)
			}
			saved, e := s.readObject("work", o.ID)
			if e != nil || saved.Owner != o.Owner || saved.NativeRef != o.NativeRef {
				t.Fatal("lost cleanup identity", e)
			}
			if failure != "" && saved.State == "ready" {
				t.Fatal("failed import published")
			}
			_, err = s.ImportWorkspace(context.Background(), "imported", "repo", "https://github.com/SLktEx/Hacocoon-test.git", "main", bytes.NewReader([]byte("saved data")))
			if !errors.Is(err, core.ErrAlreadyExists) || b.imports != 1 {
				t.Fatal("duplicate import reached native backend", err)
			}
		})
	}
}
func TestWorkspaceImportDoesNotAdoptLocalOrCredentialRouting(t *testing.T) {
	b := &workspaceImportBackend{ownershipBackend: ownershipBackend{t: t}}
	s := NewRepositoryService(t.TempDir(), b)
	b.service = s
	for _, remote := range []string{"file:///host/private/repo", "https://token@github.com/org/repo.git", "https://internal.local/repo", ""} {
		if _, err := s.ImportWorkspace(context.Background(), "imported", "repo", remote, "main", bytes.NewReader(nil)); err == nil {
			t.Fatal("unsafe routing accepted", remote)
		}
	}
	if b.imports != 0 {
		t.Fatal("invalid input created native resources")
	}
	if objects, err := s.ListWorkspaces(context.Background()); err != nil || len(objects) != 0 {
		t.Fatal("invalid input reserved ownership", err)
	}
}
