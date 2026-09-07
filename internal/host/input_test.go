package host

import (
	"context"
	"testing"
)

func TestExecRunnerStreamsInputWithoutShellInterpolation(t *testing.T) {
	input := []byte("literal $(exit 7) ' \\" + "\n" + "second line\n")
	result, err := (ExecRunner{}).RunWithInput(context.Background(), input, "cat")
	if err != nil || result.Stdout != string(input) {
		t.Fatalf("stdin changed: %v", err)
	}
}
