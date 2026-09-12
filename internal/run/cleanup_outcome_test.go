package run

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestCleanupFailuresAlwaysRemainRecoveryRequired(t *testing.T) {
	failure := errors.New("cleanup unavailable")
	for _, reconcile := range []bool{false, true} {
		for _, markerFailure := range []bool{false, true} {
			name := "run-outcome"
			t.Run(fmtOutcome(reconcile, markerFailure), func(t *testing.T) {
				store := newFakeRunStore()
				env := &fakeEnvironments{}
				if markerFailure {
					store.deleteErr = failure
				} else {
					env.deleteErr = failure
				}
				service := recoveryService(env, store)
				service.newName = func() (string, error) { return name, nil }
				service.acquireOwnership = func(string, string, bool) (runOwnershipLock, bool, error) {
					return &fakeOwnershipLock{}, true, nil
				}
				var err error
				if reconcile {
					store.runs[name] = core.EphemeralRun{EnvironmentID: name, State: core.EphemeralRunActive, CreatedAt: time.Now().UTC()}
					err = service.Reconcile(context.Background())
				} else {
					var result Result
					result, err = service.Run(context.Background(), Spec{WorkspacePath: "/work/retained", Argv: []string{"true"}})
					// cleaned_up describes resource cleanup, independently of a failed
					// marker removal. A failed marker removal still requires retry.
					if result.CleanedUp != markerFailure {
						t.Fatalf("result=%+v", result)
					}
				}
				if !errors.Is(err, failure) || !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatalf("err=%v", err)
				}
				if _, ok := store.runs[name]; !ok {
					t.Fatal("failed cleanup lost durable marker")
				}
				env.deleteErr = nil
				store.deleteErr = nil
				if err := service.Reconcile(context.Background()); err != nil {
					t.Fatal(err)
				}
				if len(store.runs) != 0 {
					t.Fatal("retry did not finalize marker")
				}
			})
		}
	}
}

func fmtOutcome(reconcile, marker bool) string {
	route := "run"
	if reconcile {
		route = "reconcile"
	}
	if marker {
		return route + "/marker"
	}
	return route + "/resources"
}

type activationFailingStore struct {
	*fakeRunStore
	failure error
}

func (s activationFailingStore) PutEphemeralRun(ctx context.Context, marker core.EphemeralRun) error {
	if marker.State == core.EphemeralRunActive {
		return s.failure
	}
	return s.fakeRunStore.PutEphemeralRun(ctx, marker)
}

func TestFailedActivationUsesOwnedCleanupAndPreservesBothFailures(t *testing.T) {
	activationErr := errors.New("activation persistence failed")
	cleanupErr := errors.New("delete unavailable")
	for _, failCleanup := range []bool{false, true} {
		t.Run(fmtOutcome(false, failCleanup), func(t *testing.T) {
			store := newFakeRunStore()
			env := &fakeEnvironments{}
			if failCleanup {
				env.deleteErr = cleanupErr
			}
			service := NewWithRecovery(env, activationFailingStore{store, activationErr}, t.TempDir())
			service.newName = func() (string, error) { return "run-activation", nil }
			result, err := service.Run(context.Background(), Spec{WorkspacePath: "/work/retained", Argv: []string{"true"}})
			if !errors.Is(err, activationErr) || result.CleanedUp == failCleanup {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			for _, call := range env.calls {
				if strings.HasPrefix(call, "exec:") {
					t.Fatal("executed after activation failed")
				}
			}
			if failCleanup {
				if !errors.Is(err, cleanupErr) || !errors.Is(err, core.ErrRecoveryRequired) || store.runs[result.Environment].State != core.EphemeralRunCleanupRequired {
					t.Fatalf("lost recovery: %v %+v", err, store.runs)
				}
			} else if len(store.runs) != 0 {
				t.Fatal("completed cleanup retained marker")
			}
		})
	}
}

func TestFailedCreateKeepsTemporaryDataUntilCanonicalRecoveryCompletes(t *testing.T) {
	store := newFakeRunStore()
	env := &fakeEnvironments{createErr: errors.Join(context.Canceled, core.ErrRecoveryRequired)}
	service := NewWithRecovery(env, store, t.TempDir())
	service.ConfigureTemporaryWorkspace(func(context.Context, core.Workspace) error {
		t.Fatal("temporary data cleanup ran before runtime absence")
		return nil
	})
	service.newName = func() (string, error) { return "run-create", nil }
	result, err := service.Run(context.Background(), Spec{Argv: []string{"true"}})
	if !errors.Is(err, context.Canceled) || !errors.Is(err, core.ErrRecoveryRequired) || result.CleanedUp {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	marker, ok := store.runs[result.Environment]
	if !ok || marker.State != core.EphemeralRunCleanupRequired || marker.TemporaryWorkspace == nil {
		t.Fatal("lost temporary ownership", marker)
	}
	if len(env.calls) != 1 || env.calls[0] != "create" {
		t.Fatal("retried deletion outside canonical create", env.calls)
	}
}
