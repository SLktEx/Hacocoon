package incus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"testing"
	"time"
)

func TestInteractiveCommandExitsWhileClientInputStaysOpen(t *testing.T) {
	for _, code := range []int{0, 17} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			input, writer := io.Pipe()
			defer input.Close()
			defer writer.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "/bin/sh", "-c", "read line; printf '%s' \"$line\"; exit \"$1\"", "sh", fmt.Sprint(code))
			var output bytes.Buffer
			cmd.Stdout = &output
			cmd.Stderr = &output
			done := make(chan error, 1)
			go func() { done <- runInteractiveCommand(cmd, input) }()
			if _, err := io.WriteString(writer, "final shell output\n"); err != nil {
				t.Fatal(err)
			}
			// Keep writer open: an interactive user has typed exit, not closed
			// the controller connection or supplied local stdin EOF.
			select {
			case err := <-done:
				if code == 0 && err != nil {
					t.Fatal(err)
				}
				if code != 0 {
					var exit *exec.ExitError
					if !errors.As(err, &exit) || exit.ExitCode() != code {
						t.Fatalf("exit = %v, want %d", err, code)
					}
				}
				if output.String() != "final shell output" {
					t.Fatalf("output = %q", output.String())
				}
			case <-ctx.Done():
				t.Fatal("shell exit waited for client input EOF")
			}
		})
	}
}

func TestInteractiveCommandStartFailureDoesNotReadInput(t *testing.T) {
	input, writer := io.Pipe()
	defer input.Close()
	defer writer.Close()
	cmd := exec.Command("/haco-nonexistent-interactive-command")
	if err := runInteractiveCommand(cmd, input); err == nil {
		t.Fatal("missing command succeeded")
	}
}
