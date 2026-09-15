package cache

import (
	"context"
	"errors"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type GenerationRecoverer interface {
	RecoverEnvironmentGeneration(context.Context, core.PersistentResourceRef) (core.ResourceGenerationPublication, error)
}
type RecoveryResult struct {
	Entries []HistoryEntry `json:"entries"`
}

func (w *Workflow) Recover(ctx context.Context, name, key string) (RecoveryResult, error) {
	result := RecoveryResult{Entries: []HistoryEntry{}}
	if w == nil || w.Recoverer == nil {
		return result, core.ErrUnsupported
	}
	history, _, candidates, err := w.historySnapshot(ctx, name, key)
	if err != nil {
		return result, err
	}
	var failures []error
	for i, r := range candidates {
		entry := history.Entries[i]
		if r.State == "deleting" {
			entry.State = "cleanup-required"
			failures = append(failures, core.ErrRecoveryRequired)
		} else {
			recovered, err := w.Recoverer.RecoverEnvironmentGeneration(ctx, r.Ref())
			entry.State = recovered.State
			if entry.State == "published" {
				entry.State = "current"
			}
			if entry.State == "" {
				entry.State = "recovery-required"
			}
			if err != nil {
				failures = append(failures, err)
			}
		}
		result.Entries = append(result.Entries, entry)
	}
	return result, errors.Join(failures...)
}
