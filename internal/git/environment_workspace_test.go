package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	gitadapter "github.com/SLktEx/Hacocoon/internal/adapters/git"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type environmentWorkBackend struct {
	localBackend
	failCreate, failDelete bool
	deleted                []Object
	cancel                 context.CancelFunc
}

func (*environmentWorkBackend) Plan(_ context.Context, kind, id string) (string, error) {
	return "pool/" + kind + "-" + id, nil
}
func (b *environmentWorkBackend) CreateVolume(_ context.Context, o Object, _ *Object) error {
	if o.Kind == "work" && b.failCreate {
		if b.cancel != nil {
			b.cancel()
		}
		return core.ErrRuntimeUnavailable
	}
	return nil
}
func (*environmentWorkBackend) RunGit(context.Context, gitadapter.AgentRequest) (gitadapter.Response, error) {
	return gitadapter.Response{Ref: "refs/heads/develop", OID: strings.Repeat("a", 40)}, nil
}
func (b *environmentWorkBackend) DeleteWorkspaceVolume(ctx context.Context, o Object) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	b.deleted = append(b.deleted, o)
	if b.failDelete {
		return core.ErrStorageBusy
	}
	return nil
}

func TestEnvironmentWorkspaceFixesRepositoryMembershipAndDefaultBranch(t *testing.T) {
	ctx := context.Background()
	for _, count := range []int{0, 1, 2} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			b := &environmentWorkBackend{}
			s := NewRepositoryService(t.TempDir(), b)
			for i := 0; i < count; i++ {
				name := fmt.Sprintf("repo-%d", i)
				if _, err := s.Add(ctx, name, "https://github.com/example/"+name); err != nil {
					t.Fatal(err)
				}
			}
			work, err := s.PrepareEnvironmentWorkspace(ctx, "new-work")
			if err != nil {
				t.Fatal(err)
			}
			original, err := s.Get("work", "new-work")
			if err != nil {
				t.Fatal(err)
			}
			if work.Path != "managed:new-work" || work.ID != core.WorkspaceID("workspace:managed:"+original.Owner) {
				t.Fatal(work, original)
			}
			if count > 0 {
				if len(original.Copies()) != count {
					t.Fatal(original)
				}
				for _, member := range original.Copies() {
					if member.Branch != "develop" {
						t.Fatal("ignored remote default", member)
					}
				}
			}
			if _, err := s.Add(ctx, "later", "https://github.com/example/later"); err != nil {
				t.Fatal(err)
			}
			if count > 0 {
				source, err := s.Get("repo", "repo-0")
				if err != nil {
					t.Fatal(err)
				}
				if err = s.UnregisterSource(ctx, source.ID, source.Owner); err != nil {
					t.Fatal(err)
				}
			}
			after, err := s.Get("work", "new-work")
			if err != nil || !reflect.DeepEqual(after, original) {
				t.Fatal("registration changed existing work", after, err)
			}
		})
	}
}

func TestEnvironmentWorkspaceFailureCleansOnlyNewOwnedResources(t *testing.T) {
	for _, count := range []int{0, 1, 2} {
		for _, cleanupFails := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/%t", count, cleanupFails), func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				b := &environmentWorkBackend{failDelete: cleanupFails, cancel: cancel}
				s := NewRepositoryService(t.TempDir(), b)
				for i := 0; i < count; i++ {
					name := fmt.Sprintf("repo-%d", i)
					if _, err := s.Add(ctx, name, "https://github.com/example/"+name); err != nil {
						t.Fatal(err)
					}
				}
				b.failCreate = true
				if _, err := s.PrepareEnvironmentWorkspace(ctx, "new-work"); !errors.Is(err, core.ErrRuntimeUnavailable) {
					t.Fatal(err)
				}
				if len(b.deleted) == 0 {
					t.Fatal("cancellation skipped owned cleanup")
				}
				for _, o := range b.deleted {
					if o.Kind != "work" || o.Owner == "" || o.NativeRef == "" {
						t.Fatal("cleanup lost exact ownership", o)
					}
				}
				_, err := s.Get("work", "new-work")
				if cleanupFails {
					if !errors.Is(err, core.ErrRecoveryRequired) {
						t.Fatal("ambiguous cleanup released owner", err)
					}
				} else if !errors.Is(err, core.ErrNotFound) {
					t.Fatal("completed cleanup retained record", err)
				}
				for i := 0; i < count; i++ {
					if _, err := s.Get("repo", fmt.Sprintf("repo-%d", i)); err != nil {
						t.Fatal("source damaged", err)
					}
				}
			})
		}
	}
}

func TestNewWorkspaceCleanupRefusesPublishedOrReplacedOwnership(t *testing.T) {
	ctx := context.Background()
	b := &environmentWorkBackend{}
	s := NewRepositoryService(t.TempDir(), b)
	if _, err := s.CreateEmptyWorkspace(ctx, "../work"); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatal(err)
	}
	ready, err := s.CreateEmptyWorkspace(ctx, "ready")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.cleanupNewWorkspace(ctx, ready); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatal("published work deleted", err)
	}
	staged := Object{Kind: "work", ID: "pending", Repository: "empty", Owner: strings.Repeat("a", 32), NativeRef: "pool/pending", State: "creating"}
	if err := s.reserve(staged); err != nil {
		t.Fatal(err)
	}
	replaced := staged
	replaced.Owner = strings.Repeat("b", 32)
	if err := s.cleanupNewWorkspace(ctx, replaced); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatal(err)
	}
	if len(b.deleted) != 0 {
		t.Fatal("unsafe cleanup reached backend")
	}
	s.Backend = localBackend{}
	if err := s.cleanupNewWorkspace(ctx, staged); !errors.Is(err, core.ErrUnsupported) {
		t.Fatal(err)
	}
}
