package persistentresource_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
)

type interruptedGenerationBackend struct {
	*maintenanceBackend
	recordCompletion bool
	recoveryErr      error
	verifyRef        core.PersistentResourceRef
	recoveries       int
}

func (b *interruptedGenerationBackend) CopyWithCompletion(ctx context.Context, source, target core.PersistentResource, completed func() error) error {
	if err := b.Copy(ctx, source, target); err != nil {
		return err
	}
	if b.recordCompletion {
		if err := completed(); err != nil {
			return err
		}
	}
	return errors.New("provider connection lost while restoring source writers")
}

func (b *interruptedGenerationBackend) Verify(ctx context.Context, resource core.PersistentResource) error {
	if resource.Ref() == b.verifyRef {
		return core.ErrCapabilityStale
	}
	return b.maintenanceBackend.Verify(ctx, resource)
}

func (b *interruptedGenerationBackend) RecoverCompletedCopy(ctx context.Context, source, target core.PersistentResource) error {
	b.recoveries++
	held, err := b.store.GetPersistentResource(ctx, target.ID)
	if err != nil || held != target || !held.CopyCompleted || held.CopySource != source.Ref() || held.Producer != source.Ref() {
		return errors.New("restoring source writers without exact durable completion")
	}
	return b.recoveryErr
}

type resetDuringRecoveryStore struct{ *state.EnvironmentJSONStore }

func (s resetDuringRecoveryStore) AdvanceResourceGeneration(ctx context.Context, expected core.ResourceGeneration, ref core.PersistentResourceRef) (core.ResourceGeneration, error) {
	if _, err := s.ResetResourceGeneration(ctx, expected, expected.Compatibility); err != nil {
		return core.ResourceGeneration{}, err
	}
	return s.EnvironmentJSONStore.AdvanceResourceGeneration(ctx, expected, ref)
}

func TestInterruptedEnvironmentPublicationRequiresReceiptBeforeResumingWriters(t *testing.T) {
	for _, mode := range []string{"completed", "unconfirmed", "verification-failed", "source-resume-failed", "reset-during-adoption"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			svc, base, lease := activeResourceEnvironment(t)
			b := &interruptedGenerationBackend{maintenanceBackend: &maintenanceBackend{environmentContractBackend: base}, recordCompletion: mode != "unconfirmed"}
			svc.Backend = b
			area := lease.Attachments[0]
			publication, err := svc.PublishEnvironmentGeneration(ctx, lease, area)
			if !errors.Is(err, core.ErrRecoveryRequired) || publication.State != "recovery-required" || publication.Candidate.State != "creating" || b.copies != 1 {
				t.Fatal("interrupted copy was published", publication, err)
			}
			reopened := state.NewEnvironmentJSONStore(base.path)
			svc.Store = reopened
			candidate, err := reopened.GetPersistentResource(ctx, publication.Candidate.ID)
			if err != nil || candidate != publication.Candidate || candidate.CopyCompleted != b.recordCompletion || candidate.Producer != area.Resource || candidate.PublicationOrigin != area.Origin {
				t.Fatal("restart lost publication provenance", candidate, err)
			}
			if err := reopened.CheckEnvironmentResourcesIdle(ctx, lease.EnvironmentID); !errors.Is(err, core.ErrRecoveryRequired) {
				t.Fatal("interrupted publication released source writers", err)
			}
			foreign := candidate.Ref()
			foreign.Owner = strings.Repeat("f", 32)
			if _, err := svc.RecoverEnvironmentGeneration(ctx, foreign); !errors.Is(err, core.ErrCapabilityStale) || b.recoveries != 0 {
				t.Fatal("changed identity resumed source writers", err)
			}
			switch mode {
			case "verification-failed":
				b.verifyRef = candidate.Ref()
			case "source-resume-failed":
				b.recoveryErr = errors.New("source remains stopped")
			case "reset-during-adoption":
				svc.Store = resetDuringRecoveryStore{reopened}
			}
			result, err := svc.RecoverEnvironmentGeneration(ctx, candidate.Ref())
			if mode == "unconfirmed" || mode == "verification-failed" || mode == "source-resume-failed" {
				if !errors.Is(err, core.ErrRecoveryRequired) || result.State != "recovery-required" || result.Candidate != candidate {
					t.Fatal("unsafe publication recovery", result, err)
				}
				if held, err := reopened.GetPersistentResource(ctx, candidate.ID); err != nil || held != candidate || base.deletes != 0 || b.copies != 1 {
					t.Fatal("refused recovery changed owned data", held, err)
				}
				if mode != "source-resume-failed" && b.recoveries != 0 {
					t.Fatal("unverified copy resumed source writers")
				}
				if mode == "unconfirmed" {
					if err := svc.DeleteUnselectedGeneration(ctx, candidate.Ref()); !errors.Is(err, core.ErrRecoveryRequired) {
						t.Fatal("unknown copy outcome allowed candidate deletion", err)
					}
					return
				}
				b.verifyRef, b.recoveryErr = core.PersistentResourceRef{}, nil
				result, err = svc.RecoverEnvironmentGeneration(ctx, candidate.Ref())
			}
			wantState := "published"
			if mode == "reset-during-adoption" {
				wantState = "retained"
			}
			if err != nil || result.State != wantState || result.Candidate.Ref() != candidate.Ref() || result.Candidate.State != "ready" || b.copies != 1 || base.deletes != 0 {
				t.Fatal("recovery recopied or cleaned up an existing candidate", result, err)
			}
			if (result.Generation.Current == candidate.Ref()) != (wantState == "published") {
				t.Fatal("recovery overwrote a concurrent generation reset", result)
			}
			if err := reopened.CheckEnvironmentResourcesIdle(ctx, lease.EnvironmentID); err != nil {
				t.Fatal("completed copy did not release source writers", err)
			}
			before := b.recoveries
			svc.Store = reopened
			again, err := svc.RecoverEnvironmentGeneration(ctx, candidate.Ref())
			if err != nil || again.State != wantState || b.recoveries != before || b.copies != 1 || base.deletes != 0 {
				t.Fatal("finished recovery was not idempotent", again, err)
			}
		})
	}
}
