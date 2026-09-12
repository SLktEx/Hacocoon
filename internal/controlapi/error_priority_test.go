package controlapi

import (
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestRecoveryRequiredSurvivesJoinedOperationErrors(t *testing.T) {
	for _, cause := range []error{core.ErrNotFound, core.ErrRuntimeUnavailable, core.ErrInvalidArgument, core.ErrIncompatibleState} {
		var status *control.StatusError
		err := translateError(errors.Join(cause, core.ErrRecoveryRequired))
		if !errors.As(err, &status) || status.Code != "recovery_required" {
			t.Fatalf("cleanup obligation became %v", err)
		}
	}
}
