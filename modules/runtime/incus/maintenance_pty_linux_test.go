//go:build linux

package incus

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"

	"golang.org/x/term"
)

func openMaintenanceTestPTY() (*os.File, *os.File, error) {
	return openInteractivePTY(80, 24)
}

func TestMaintenanceConfirmationUsesRealTerminal(t *testing.T) {
	if os.Getenv("HACO_TEST_MAINTENANCE_TERMINAL") == "1" {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			os.Exit(2)
		}
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil || line != "no\n" {
			os.Exit(3)
		}
		os.Exit(0)
	}
	master, slave, err := openMaintenanceTestPTY()
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	defer slave.Close()
	if _, err := io.WriteString(master, "no\n"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestMaintenanceConfirmationUsesRealTerminal$")
	cmd.Env = append(os.Environ(), "HACO_TEST_MAINTENANCE_TERMINAL=1")
	cmd.Stdin = slave
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
}
