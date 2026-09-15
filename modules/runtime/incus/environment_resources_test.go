package incus

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"testing"
)

func TestEnvironmentResourcesUnsupportedEntriesNeverSilentlyDropData(t *testing.T) {
	ctx := context.Background()
	spec := core.EnvironmentRuntimeSpec{Attachments: []core.EnvironmentRuntimeAttachment{{}}}
	calls := map[string]func() error{
		"runtime": func() error { _, err := (*Runtime)(nil).CreateEnvironment(ctx, spec); return err },
		"base":    func() error { _, err := (*BaseProvider)(nil).CreateEnvironment(ctx, spec); return err },
		"sandbox": func() error { _, err := (*SandboxProvider)(nil).CreateEnvironment(ctx, spec); return err },
		"snapshot-create": func() error {
			_, err := (*SandboxProvider)(nil).CreateEnvironmentFromSnapshot(ctx, spec, core.Snapshot{}, nil)
			return err
		},
		"snapshot-plan": func() error {
			_, err := (*Runtime)(nil).PlanSnapshot(ctx, core.SnapshotSource{Environment: core.Environment{Attachments: []core.EnvironmentAttachment{{}}}}, "")
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			expected := core.ErrUnsupported
			if name == "snapshot-plan" || name == "snapshot-create" {
				expected = core.ErrInvalidArgument
			}
			if err := call(); !errors.Is(err, expected) {
				t.Fatal(err)
			}
		})
	}
}
