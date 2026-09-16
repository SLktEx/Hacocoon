package incus

import (
	"context"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestSandboxReadOnlyWorkspaceNeverRequestsWritableMount(t *testing.T) {
	runner := &fakeRunner{}
	provider := testSandboxProvider(t, New(runner))
	err := provider.addWorkspaceDevice(context.Background(), "haco-demo", core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: "/tmp/work space", ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 1 || strings.Contains(strings.Join(runner.calls[0].args, " "), "shift=true") {
		t.Fatal("read-only mount changed identity mapping or started extra operations", runner.calls)
	}
	assertRunnerCall(t, runner.calls[0], "incus", "config", "device", "add", "haco-demo", "workspace", "disk", "source=/tmp/work space", "path=/workspace", "readonly=true", "--project", defaultProject)
}
