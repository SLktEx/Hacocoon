package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
)

func TestEnvironmentDeletionCompleteRequiresOnlyAbsence(t *testing.T) {
	for _, tc := range []struct {
		name     string
		err      error
		complete bool
	}{
		{"success", nil, true},
		{"absent", ErrNotFound, true},
		{"wrapped", fmt.Errorf("instance: %w", ErrNotFound), true},
		{"absent components", errors.Join(ErrNotFound, fmt.Errorf("instance: %w", ErrNotFound)), true},
		{"guard failure", errors.Join(ErrNotFound, ErrRecoveryRequired), false},
		{"unknown failure", fmt.Errorf("delete: %w", errors.Join(ErrNotFound, errors.New("guard failed"))), false},
		{"canceled", errors.Join(ErrNotFound, context.Canceled), false},
		{"timeout", context.DeadlineExceeded, false},
		{"wrong owner", errors.Join(ErrNotFound, ErrCapabilityStale), false},
		{"missing executable", &os.PathError{Op: "exec", Path: "incus", Err: os.ErrNotExist}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := EnvironmentDeletionComplete(tc.err); got != tc.complete {
				t.Fatalf("complete=%v for %v", got, tc.err)
			}
		})
	}
}
