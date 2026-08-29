package host

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
)

func TestExecRunnerBoundsCapturedOutput(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	result, err := (ExecRunner{}).Run(context.Background(), executable, "-test.run=TestExecRunnerFloodHelper", "--", "__haco_flood_stdout__")
	if !errors.Is(err, ErrOutputLimit) {
		t.Fatalf("expected output limit error, got %v", err)
	}
	if len(result.Stdout) != maxCommandOutputBytes {
		t.Fatalf("captured stdout length = %d, want %d", len(result.Stdout), maxCommandOutputBytes)
	}
}

func TestExecRunnerFloodHelper(t *testing.T) {
	if len(os.Args) == 0 || os.Args[len(os.Args)-1] != "__haco_flood_stdout__" {
		return
	}
	chunk := bytes.Repeat([]byte{'x'}, 64<<10)
	remaining := maxCommandOutputBytes + len(chunk)
	for remaining > 0 {
		write := len(chunk)
		if write > remaining {
			write = remaining
		}
		if _, err := os.Stdout.Write(chunk[:write]); err != nil {
			os.Exit(0)
		}
		remaining -= write
	}
	os.Exit(0)
}
