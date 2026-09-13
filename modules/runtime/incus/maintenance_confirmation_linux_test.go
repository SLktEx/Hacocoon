//go:build linux

package incus

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"
)

// Exercise the shipped terminal requirement with a private PTY. A pipe carrying
// "yes" is deliberately not an interactive deletion decision in the product.
func maintenanceConfirmationInput(answer string) (*os.File, func(), error) {
	master, slave, err := openInteractivePTY(80, 24)
	if err != nil {
		return nil, nil, err
	}
	closeTerminal := func() { _ = slave.Close(); _ = master.Close() }
	if _, err := master.WriteString(answer); err != nil {
		closeTerminal()
		return nil, nil, err
	}
	return slave, closeTerminal, nil
}

func TestMaintenanceConfirmationUsesTerminal(t *testing.T) {
	for _, answer := range []string{"yes", "no"} {
		t.Run(answer, func(t *testing.T) {
			in, closeTerminal, err := maintenanceConfirmationInput(answer + "\n")
			if err != nil {
				t.Fatal(err)
			}
			defer closeTerminal()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "sh", "-c", `test -t 0 && IFS= read -r answer && printf '%s' "$answer"`)
			cmd.Stdin = in
			out, err := cmd.Output()
			if err != nil || string(out) != answer {
				t.Fatalf("terminal answer: %q, %v", out, err)
			}
		})
	}
}
