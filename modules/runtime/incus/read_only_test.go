package incus

import (
	"context"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestCreateEnvironmentMarksReadOnlyWorkspace(t *testing.T) {
	runner := &fakeRunner{}
	runtime := New(runner)
	_, err := runtime.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: "/tmp/workspace", ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range runner.calls {
		for _, arg := range call.args {
			if arg == "readonly=true" {
				return
			}
		}
	}
	t.Fatalf("readonly=true missing from calls: %#v", runner.calls)
}
