//go:build linux

package terminalbridge

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

const terminationChildMode = "HACO_TEST_BRIDGE_TERMINATION_CONTEXT"

func TestBridgeTerminationSignalRestoresPTY(t *testing.T) {
	if mode := os.Getenv(terminationChildMode); mode != "" {
		runTerminationChild(t, mode)
		return
	}
	for _, mode := range []string{"background", "caller-sigterm"} {
		for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGHUP} {
			t.Run(mode+"/"+sig.String(), func(t *testing.T) {
				checkTerminationPTY(t, mode, sig)
			})
		}
	}
}

func checkTerminationPTY(t *testing.T, mode string, sig syscall.Signal) {
	t.Helper()
	slave, master := inputTTY(t)
	// Match inherited stdin while retaining the exact caller-owned descriptor.
	inputFD(t, slave, func(fd uintptr) {
		if err := unix.SetNonblock(int(fd), false); err != nil {
			t.Fatal(err)
		}
	})
	before, flags := terminationTermios(t, slave), inputFlags(t, slave)
	if before.Lflag&(unix.ICANON|unix.ECHO) != unix.ICANON|unix.ECHO || before.Oflag&(unix.OPOST|unix.ONLCR) != unix.OPOST|unix.ONLCR {
		t.Fatal("private PTY did not start in canonical echo mode")
	}
	child := startTerminationChild(t, slave, mode)
	child.expect(t, "READY\n") // Forwarded by Bridge only after real preparation.
	raw := terminationTermios(t, slave)
	if raw.Lflag&(unix.ICANON|unix.ECHO|unix.ISIG) != 0 || raw.Oflag&unix.OPOST != 0 {
		t.Fatal("Bridge did not put the actual stdin PTY into raw mode")
	}
	writeTerminationInput(t, master, "x")
	child.expect(t, "INPUT=x\n") // No newline: the live input copier must deliver it.
	if err := child.cmd.Process.Signal(sig); err != nil {
		t.Fatal(err)
	}
	child.expect(t, "CANCELED AND CLOSED\n")
	if after := terminationTermios(t, slave); *after != *before {
		t.Fatalf("terminal state was not restored: before=%+v after=%+v", before, after)
	}
	if got := inputFlags(t, slave); got != flags {
		t.Fatalf("caller input flags changed: %x -> %x", flags, got)
	}
	// The child remains alive and reads the same os.Stdin after Bridge returns.
	// Echo and a complete canonical read also expose a copier stealing later input.
	writeTerminationInput(t, master, "next")
	readTerminationEcho(t, master, "next")
	writeTerminationInput(t, master, "\n")
	readTerminationEcho(t, master, "\r\n")
	child.expect(t, "REUSED=next\n")
	child.wait(t)
}

func runTerminationChild(t *testing.T, mode string) {
	t.Helper()
	ctx := context.Background()
	if mode == "caller-sigterm" {
		var stop context.CancelFunc
		ctx, stop = signal.NotifyContext(ctx, syscall.SIGTERM)
		defer stop()
	} else if mode != "background" {
		t.Fatalf("unknown child context %q", mode)
	}
	local, remote := net.Pipe()
	defer func() { _ = remote.Close() }()
	done := make(chan error, 1)
	go func() { done <- Bridge(ctx, local, os.Stdin, os.Stdout) }()
	if _, err := io.WriteString(remote, "READY\n"); err != nil {
		t.Fatal(err)
	}
	var first [1]byte
	if _, err := io.ReadFull(remote, first[:]); err != nil || first[0] != 'x' {
		t.Fatalf("raw input did not reach the peer: %q, %v", first, err)
	}
	fmt.Println("INPUT=x")
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Bridge did not return cooperative cancellation: %v", err)
	}
	if n, err := remote.Read(first[:]); n != 0 || err != io.EOF {
		t.Fatalf("Bridge left its stream open: n=%d, err=%v", n, err)
	}
	fmt.Println("CANCELED AND CLOSED")
	var line [32]byte
	if n, err := os.Stdin.Read(line[:]); err != nil || string(line[:n]) != "next\n" {
		t.Fatalf("caller canonical stdin was not reusable: %q, %v", line[:n], err)
	}
	fmt.Println("REUSED=next")
}

type terminationChild struct {
	cmd    *exec.Cmd
	output *os.File
	reader *bufio.Reader
	done   chan struct{}
	err    error
	stderr bytes.Buffer
}

func startTerminationChild(t *testing.T, stdin *os.File, mode string) *terminationChild {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	output, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = output.Close(); _ = writer.Close() })
	child := &terminationChild{output: output, reader: bufio.NewReader(output), done: make(chan struct{})}
	child.cmd = exec.Command(executable, "-test.run=^TestBridgeTerminationSignalRestoresPTY$")
	child.cmd.Env = append(os.Environ(), terminationChildMode+"="+mode)
	child.cmd.Stdin, child.cmd.Stdout, child.cmd.Stderr = stdin, writer, &child.stderr
	if err := child.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { child.err = child.cmd.Wait(); close(child.done) }()
	t.Cleanup(func() { child.cleanup(t) })
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return child
}

func (c *terminationChild) expect(t *testing.T, want string) {
	t.Helper()
	if err := c.output.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	line, err := c.reader.ReadSlice('\n')
	if err != nil || string(line) != want {
		t.Fatalf("child handshake: want %q, got %q, err=%v", want, line, err)
	}
}

func (c *terminationChild) wait(t *testing.T) {
	t.Helper()
	select {
	case <-c.done:
		if c.err != nil {
			t.Fatalf("child did not exit normally: %v", c.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("child did not exit after canonical stdin reuse")
	}
}

func (c *terminationChild) cleanup(t *testing.T) {
	t.Helper()
	select {
	case <-c.done:
	default:
		// Forced termination is failure cleanup only, never acceptance evidence.
		if !t.Failed() {
			t.Error("child still running at cleanup")
		}
		if err := c.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			t.Errorf("kill owned test child: %v", err)
		}
		select {
		case <-c.done:
		case <-time.After(5 * time.Second):
			t.Error("owned test child was not reaped")
			return
		}
	}
	if t.Failed() {
		t.Logf("child result: %v; stderr: %s", c.err, c.stderr.String())
	}
}

func terminationTermios(t *testing.T, file *os.File) *unix.Termios {
	t.Helper()
	var state *unix.Termios
	var err error
	inputFD(t, file, func(fd uintptr) { state, err = unix.IoctlGetTermios(int(fd), unix.TCGETS) })
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func writeTerminationInput(t *testing.T, master *os.File, value string) {
	t.Helper()
	if err := master.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(master, value); err != nil {
		t.Fatal(err)
	}
}

func readTerminationEcho(t *testing.T, master *os.File, want string) {
	t.Helper()
	if err := master.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(want))
	if _, err := io.ReadFull(master, got); err != nil || string(got) != want {
		t.Fatalf("canonical echo: want %q, got %q, err=%v", want, got, err)
	}
}
