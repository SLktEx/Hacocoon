package gitrepo

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type ownershipBackend struct {
	localBackend
	service   *RepositoryService
	t         *testing.T
	fail      string
	populated bool
}

func (b *ownershipBackend) record(object Object, state string) {
	b.t.Helper()
	data, err := os.ReadFile(b.service.path(object.Kind, object.ID))
	if err != nil {
		b.t.Fatal(err)
	}
	var saved Object
	if json.Unmarshal(data, &saved) != nil || saved.Owner != object.Owner || saved.NativeRef != object.NativeRef || saved.State != state {
		b.t.Fatalf("ownership not durable before next operation: %s", data)
	}
}
func (b *ownershipBackend) CreateVolume(_ context.Context, object Object, _ *Object) error {
	b.record(object, "creating")
	if b.fail == "create" {
		return errors.New("ambiguous provider result")
	}
	return nil
}
func (b *ownershipBackend) InspectVolume(_ context.Context, object Object) error {
	b.record(object, "created")
	if b.fail == "inspect" {
		return errors.New("unknown owner")
	}
	return nil
}
func (b *ownershipBackend) Populate(_ context.Context, object Object) error {
	b.record(object, "created")
	b.populated = true
	return nil
}
func TestVolumeOwnershipPrecedesFallibleWork(t *testing.T) {
	for _, failure := range []string{"", "create", "inspect"} {
		t.Run(failure, func(t *testing.T) {
			backend := &ownershipBackend{t: t, fail: failure}
			service := NewRepositoryService(t.TempDir(), backend)
			backend.service = service
			object, err := service.Clone(context.Background(), "demo", "https://github.com/example/repo.git", "main")
			if failure == "" {
				if err != nil || object.State != "ready" || !backend.populated {
					t.Fatalf("object=%+v err=%v", object, err)
				}
			} else {
				if !errors.Is(err, core.ErrRecoveryRequired) || backend.populated {
					t.Fatalf("err=%v populated=%v", err, backend.populated)
				}
				if _, err := service.Get("repo", "demo"); !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatalf("incomplete record reusable: %v", err)
				}
			}
			if _, err := service.Clone(context.Background(), "demo", "https://github.com/example/repo.git", "main"); !errors.Is(err, core.ErrAlreadyExists) {
				t.Fatalf("replaced owned record: %v", err)
			}
		})
	}
}
func TestInvalidRepositoryInputHasNoProviderEffects(t *testing.T) {
	for _, id := range []string{"../escape", "/absolute", "--option", "", "a\nb", strings.Repeat("a", 49)} {
		service := NewRepositoryService(t.TempDir(), nil)
		if _, err := service.Clone(context.Background(), id, "https://github.com/example/repo.git", "main"); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("id=%q err=%v", id, err)
		}
	}
	for _, remote := range []string{"https://token@github.com/example/repo", "https://github.com.evil/example/repo", "ext::sh bad", "file://host/root", "https://github.com/example/repo?token=secret"} {
		if ValidateRemote(remote) == nil {
			t.Fatalf("accepted remote %q", remote)
		}
	}
}
