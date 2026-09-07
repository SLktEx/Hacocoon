package incus

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

type inputCaptureRunner struct {
	ownershipTestRunner
	input      []byte
	inputCalls int
}

func (r *inputCaptureRunner) RunWithInput(_ context.Context, input []byte, name string, args ...string) (host.Result, error) {
	r.inputCalls++
	r.input = append([]byte(nil), input...)
	r.calls = append(r.calls, ownershipRunnerCall{name: name, args: append([]string(nil), args...)})
	return host.Result{Stdout: "setup output", ExitCode: 17}, nil
}

func TestProductionDecoratorsPreserveExecutionInput(t *testing.T) {
	for _, seed := range []bool{false, true} {
		inner := &inputCaptureRunner{}
		var runner host.Runner = inner
		if seed {
			runner = WrapSeedHarvestRunner(runner)
		}
		runner = WrapEnvironmentNetworkOwnershipRunner(runner)
		runtime := New(runner)
		if !runtime.SupportsStdin() {
			t.Fatal("production decorator lost stdin support")
		}
		script := []byte("printf private-recipe")
		result, err := runtime.ExecEnvironment(context.Background(), "haco-demo", core.ExecutionRequest{Argv: []string{"/bin/bash", "-se"}, Stdin: script})
		if err != nil || result.ExitCode != 17 || result.Stdout != "setup output" || !reflect.DeepEqual(inner.input, script) || inner.inputCalls != 1 {
			t.Fatalf("input/result not preserved: %#v %v", result, err)
		}
		for _, arg := range inner.calls[0].args {
			if arg == string(script) {
				t.Fatal("script leaked into argv")
			}
		}
		input := runner.(host.InputRunner)
		bridge := environmentBridgeName("haco-demo")
		if _, err := input.RunWithInput(context.Background(), []byte("x"), "incus", "network", "delete", bridge); !errors.Is(err, core.ErrUnsupported) {
			t.Fatal("stdin route bypassed management checks")
		}
		if inner.inputCalls != 1 {
			t.Fatal("unsupported mutation reached backend")
		}
		if _, err := runner.Run(context.Background(), "incus", "network", "delete", bridge); !errors.Is(err, core.ErrIncompatibleState) {
			t.Fatal("ordinary ownership guard lost")
		}
	}
}

func TestProductionDecoratorsDoNotInventInputSupport(t *testing.T) {
	runner := WrapEnvironmentNetworkOwnershipRunner(WrapSeedHarvestRunner(&ownershipTestRunner{}))
	if _, ok := runner.(host.InputRunner); ok {
		t.Fatal("unsupported input advertised")
	}
}
