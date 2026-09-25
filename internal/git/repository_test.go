package gitrepo

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/adapters/git"
	"os"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type ownershipBackend struct {
	localBackend
	service       *RepositoryService
	t             *testing.T
	fail          string
	createCalls   int
	populateCalls int
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
	b.createCalls++
	if b.fail == "create" {
		return errors.New("ambiguous provider result")
	}
	return nil
}
func (b *ownershipBackend) InspectVolume(_ context.Context, object Object) error {
	if object.State != "created" && object.State != "ready" {
		b.t.Fatalf("unexpected inspect state %q", object.State)
	}
	b.record(object, object.State)
	if b.fail == "inspect" {
		return errors.New("unknown owner")
	}
	return nil
}
func (b *ownershipBackend) Populate(_ context.Context, object Object) error {
	b.record(object, "created")
	b.populateCalls++
	if b.fail == "populate" {
		return errors.New("clone interrupted")
	}
	return nil
}
func TestVolumeOwnershipPrecedesFallibleWork(t *testing.T) {
	for _, failure := range []string{"", "create", "inspect", "populate"} {
		t.Run(failure, func(t *testing.T) {
			backend := &ownershipBackend{t: t, fail: failure}
			service := NewRepositoryService(t.TempDir(), backend)
			backend.service = service
			object, err := service.Add(context.Background(), "demo", "https://github.com/example/repo.git")
			if failure == "" {
				if err != nil || object.State != "ready" || backend.populateCalls != 1 {
					t.Fatalf("object=%+v err=%v populate=%d", object, err, backend.populateCalls)
				}
			} else {
				if !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatalf("err=%v", err)
				}
				if _, err := service.Get("repo", "demo"); !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatalf("incomplete record unexpectedly ready: %v", err)
				}
			}
			owner, native := object.Owner, object.NativeRef
			backend.fail = ""
			retried, err := service.Add(context.Background(), "demo", "https://github.com/example/repo.git")
			if err != nil || retried.State != "ready" || retried.Owner != owner || retried.NativeRef != native {
				t.Fatalf("retry did not converge: object=%+v err=%v", retried, err)
			}
			wantCreate := 1
			if failure == "create" {
				wantCreate = 2
			}
			wantPopulate := 1
			if failure == "populate" {
				wantPopulate = 2
			}
			if backend.createCalls != wantCreate || backend.populateCalls != wantPopulate {
				t.Fatalf("retry repeated wrong stages: create=%d populate=%d", backend.createCalls, backend.populateCalls)
			}
			again, err := service.Add(context.Background(), "demo", "https://github.com/example/repo.git")
			if err != nil || again.Owner != owner || again.NativeRef != native || backend.populateCalls != wantPopulate {
				t.Fatalf("ready add was not idempotent: object=%+v err=%v populate=%d", again, err, backend.populateCalls)
			}
			if _, err := service.Add(context.Background(), "demo", "https://github.com/example/other.git"); !errors.Is(err, core.ErrAlreadyExists) {
				t.Fatalf("different remote adopted existing identity: %v", err)
			}
		})
	}
}
func TestInvalidRepositoryInputHasNoProviderEffects(t *testing.T) {
	for _, id := range []string{"../escape", "/absolute", "--option", "", "a\nb", strings.Repeat("a", 49)} {
		service := NewRepositoryService(t.TempDir(), nil)
		if _, err := service.Add(context.Background(), id, "https://github.com/example/repo.git"); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("id=%q err=%v", id, err)
		}
	}
	for _, remote := range []string{"https://token@github.com/example/repo", "https://github.com.evil/example/repo", "ext::sh bad", "file://host/root", "https://github.com/example/repo?token=secret"} {
		if gitadapter.ValidateRemote(remote) == nil {
			t.Fatalf("accepted remote %q", remote)
		}
	}
}

type canceledRegistrationBackend struct {
	ownershipBackend
	started chan struct{}
	calls   int
}

func (b *canceledRegistrationBackend) Populate(ctx context.Context, object Object) error {
	b.record(object, "created")
	b.calls++
	if b.calls == 1 {
		close(b.started)
		<-ctx.Done()
		return ctx.Err()
	}
	b.populateCalls++
	return nil
}

func TestCanceledRegistrationRetainsExactOwnership(t *testing.T) {
	b := &canceledRegistrationBackend{ownershipBackend: ownershipBackend{t: t}, started: make(chan struct{})}
	s := NewRepositoryService(t.TempDir(), b)
	b.service = s
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := s.Add(ctx, "demo", "https://github.com/example/repo.git"); done <- err }()
	<-b.started
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) || !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal("cancellation lost recovery obligation", err)
	}
	retained, err := s.Get("repo", "demo")
	if !errors.Is(err, core.ErrRecoveryRequired) || retained.State != "created" || retained.Owner == "" || retained.NativeRef == "" || retained.Branch != "" {
		t.Fatal("cancellation discarded source ownership", retained, err)
	}
	retried, err := s.Add(context.Background(), "demo", "https://github.com/example/repo.git")
	if err != nil || retried.State != "ready" || retried.Owner != retained.Owner || retried.NativeRef != retained.NativeRef {
		t.Fatal("retry did not resume canceled registration", retried, err)
	}
}
