package hostsetup

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestPackageFailurePreservesReasonsAndRuntimeIdentity(t *testing.T) {
	for _, phase := range []string{"query", "update", "install", "git_verify", "gh_verify"} {
		for _, kind := range []string{"timeout", "exit", "other"} {
			failure := &PackageFailure{Phase: phase, Kind: kind}
			var events []Event
			ctx := Observe(context.Background(), func(e Event) { events = append(events, e) })
			err := Step(ctx, "host_storage", func() error {
				return Step(ctx, "host_packages", func() error { return failure })
			})
			stage, reason := Details(err)
			want := "packages_" + phase + "_" + kind
			if stage != "host_packages" || reason != want || !ValidReason(want) || !errors.Is(err, core.ErrRuntimeUnavailable) || len(events) != 4 || events[2].Reason != want || events[3].Reason != want {
				t.Fatal("lost package classification or error identity", stage, reason, events, err)
			}
			for _, cancellation := range []error{context.Canceled, context.DeadlineExceeded} {
				if Reason(errors.Join(failure, cancellation)) != Reason(cancellation) {
					t.Fatal("package classification overrode cancellation")
				}
			}
		}
	}
}

func TestPackageFailureRejectsUnknownDiagnosticValues(t *testing.T) {
	for _, failure := range []*PackageFailure{nil, {}, {Phase: "SECRET", Kind: "timeout"}, {Phase: "install", Kind: "SECRET"}} {
		if failure.Error() != "unavailable" || Reason(failure) != "unavailable" || !errors.Is(failure, core.ErrRuntimeUnavailable) {
			t.Fatal("unknown package diagnostic leaked", failure)
		}
	}
	for _, reason := range []string{"packages_SECRET_timeout", "packages_install_SECRET", "packages_install_timeout\nSECRET", "packages__exit", "packages_install"} {
		if ValidReason(reason) || strings.Contains((&PackageFailure{Phase: reason, Kind: "other"}).Error(), "SECRET") {
			t.Fatal("unknown package reason admitted", reason)
		}
	}
}
