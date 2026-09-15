package control

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListenUnixDefaultModeIsPrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sock")
	listener, err := ListenUnix(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("socket mode = %o, want 600", got)
	}
}

func TestListenUnixRefusesActiveSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sock")
	listener, err := ListenUnix(path, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	second, err := ListenUnix(path, 0o600)
	if second != nil {
		second.Close()
		t.Fatal("second listener unexpectedly succeeded")
	}
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("error = %v, want ErrAlreadyRunning", err)
	}
}

func TestReadEnvelopeLineRejectsOversizedInput(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(strings.Repeat("x", maxControlEnvelopeBytes+1)))
	_, err := readEnvelopeLine(reader)
	if !errors.Is(err, ErrProtocol) {
		t.Fatalf("error = %v, want ErrProtocol", err)
	}
}
