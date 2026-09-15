package workspace

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	cacheapp "github.com/SLktEx/Hacocoon/modules/standard/cache"
)

type interruptedGenerationBackend struct {
	*environmentDataBackend
	copies     int
	recoveries int
	reject     bool
}

func (b *interruptedGenerationBackend) CopyWithCompletion(ctx context.Context, source, target core.PersistentResource, completed func() error) error {
	b.copies++
	if err := b.Copy(ctx, source, target); err != nil {
		return err
	}
	if err := completed(); err != nil {
		return err
	}
	return errors.New("interrupted after durable completion")
}
func (b *interruptedGenerationBackend) RecoverCompletedCopy(_ context.Context, source, target core.PersistentResource) error {
	b.recoveries++
	if b.reject || target.CopySource != source.Ref() || !target.CopyCompleted {
		return core.ErrRecoveryRequired
	}
	return nil
}

func TestCacheRecoveryUsesPositiveReceiptWithoutRecopyOrRetainedDataDeletion(t *testing.T) {
	for _, mode := range []string{"completed", "unknown", "reset", "provider-refused", "wrong-owner"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			svc, catalog, backend, spec, resources := dataEnvironmentService(t, "")
			svc.runtime = &collectingRuntime{dataEnvironmentRuntime: svc.runtime.(*dataEnvironmentRuntime), status: core.EnvironmentRuntimeStatus{State: core.EnvironmentStopped}}
			env, err := svc.Create(ctx, spec)
			if err != nil {
				t.Fatal(err)
			}
			interrupted := &interruptedGenerationBackend{environmentDataBackend: backend}
			resources.Backend = interrupted
			if mode == "unknown" {
				backend.fail = "copy"
			}
			publication, err := svc.CollectEnvironmentResource(ctx, env.Name, "compiler")
			if !errors.Is(err, core.ErrRecoveryRequired) {
				t.Fatal(publication, err)
			}
			if mode == "reset" {
				_, err = catalog.ResetResourceGeneration(ctx, env.Attachments[0].Origin, env.Attachments[0].Origin.Compatibility)
				if err != nil {
					t.Fatal(err)
				}
			}
			if mode == "provider-refused" {
				interrupted.reject = true
			}
			if mode == "wrong-owner" {
				ref := publication.Candidate.Ref()
				ref.Owner = strings.Repeat("f", 32)
				if _, err := resources.RecoverEnvironmentGeneration(ctx, ref); !errors.Is(err, core.ErrCapabilityStale) || interrupted.recoveries != 0 {
					t.Fatal("wrong owner recovery", err)
				}
				if _, err := resources.RecoverCopy(ctx, ref); !errors.Is(err, core.ErrCapabilityStale) {
					t.Fatal("common recovery ignored owner", err)
				}
				return
			}
			w := &cacheapp.Workflow{Catalog: catalog, Recoverer: resources}
			result, err := w.Recover(ctx, env.Name, "compiler")
			if interrupted.copies != 1 {
				t.Fatal("replayed native copy", interrupted.copies)
			}
			if mode == "unknown" || mode == "provider-refused" {
				if !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatal(result, err)
				}
				if err := catalog.CheckEnvironmentResourceCopyIdle(ctx, env.Name); !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatal("uncertain source released", err)
				}
				if mode == "unknown" && interrupted.recoveries != 0 {
					t.Fatal("unknown copy reached provider recovery")
				}
				return
			}
			if err != nil || len(result.Entries) != 1 {
				t.Fatal(result, err)
			}
			want := "current"
			if mode == "reset" {
				want = "retained"
			}
			if result.Entries[0].State != want {
				t.Fatal(result)
			}
			if err := catalog.CheckEnvironmentResourceCopyIdle(ctx, env.Name); err != nil {
				t.Fatal("completed copy kept producer pinned", err)
			}
			if _, err := w.Recover(ctx, env.Name, "compiler"); err != nil || interrupted.copies != 1 || interrupted.recoveries != 1 {
				t.Fatal("retry repeated native work", err)
			}
			if err := svc.Delete(ctx, env.Name); err != nil {
				t.Fatal(err)
			}
			if _, err := catalog.GetPersistentResource(ctx, env.PersistentResource.ID); err != nil {
				t.Fatal("lost retained OCI", err)
			}
			if _, err := catalog.GetPersistentResource(ctx, publication.Candidate.ID); err != nil {
				t.Fatal("recovery discarded source", err)
			}
		})
	}
}
