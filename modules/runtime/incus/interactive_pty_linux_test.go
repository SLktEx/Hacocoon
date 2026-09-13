//go:build linux

package incus

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"golang.org/x/term"
)

type terminalTranscript struct {
	mu      sync.Mutex
	text    strings.Builder
	changed chan struct{}
}

func (o *terminalTranscript) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.text.Write(p)
	select {
	case o.changed <- struct{}{}:
	default:
	}
	return len(p), nil
}

func (o *terminalTranscript) wait(t *testing.T, ctx context.Context, text string) {
	t.Helper()
	for {
		o.mu.Lock()
		got := o.text.String()
		o.mu.Unlock()
		if strings.Contains(got, text) {
			return
		}
		select {
		case <-o.changed:
		case <-ctx.Done():
			t.Fatalf("missing %q in terminal output %q", text, got)
		}
	}
}

func TestSizedInteractivePTYReadlineResizeAndExit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	input, writer := io.Pipe()
	defer input.Close()
	defer writer.Close()
	output := &terminalTranscript{changed: make(chan struct{}, 1)}
	updates := make(chan core.TerminalSize, 1)
	metadata := core.TerminalMetadata{Columns: 80, Rows: 24, Term: "xterm-256color", Resizes: updates}
	argv := interactiveShellWithPrompt([]string{"/bin/bash", "--noprofile", "--norc", "-i"}, trustedHostPrompt, "trusted-host", metadata)
	// The real Incus guest has a separate, cooked PTY. Restore those settings
	// for this local Bash stand-in before exercising its readline behavior.
	cmd := exec.CommandContext(ctx, "/bin/sh", append([]string{"-c", `stty sane; exec "$@"`, "sh"}, argv...)...)
	cmd.Stdout, cmd.Stderr = output, output
	done := make(chan error, 1)
	go func() { done <- runSizedInteractiveCommand(ctx, cmd, input, metadata) }()
	output.wait(t, ctx, "[HACO-HOST]")
	send := func(s string) {
		t.Helper()
		if _, err := io.WriteString(writer, s); err != nil {
			t.Fatal(err)
		}
	}
	send("printf '__SIZE_%s__\\n' \"$(stty size)\"\n")
	output.wait(t, ctx, "__SIZE_24 80__")
	// Edit the tail of a command spanning several screen rows with readline
	// arrows and Delete, then verify the exact command result.
	prefix := strings.Repeat("x", 240)
	send("printf '__VALUE_%s__\\n' '" + prefix + "BAD'" + strings.Repeat("\x1b[D", 4) + strings.Repeat("\x1b[3~", 3) + "OK\x05\n")
	output.wait(t, ctx, "__VALUE_"+prefix+"OK__")
	// Wait until Bash has left readline and started a foreground command before
	// resizing. Seeing a command's stdout alone does not mean readline has
	// finished restoring its previous terminal settings. A concurrent resize
	// there can be overwritten by that restoration in the local Bash stand-in.
	send("printf '__RESIZE_%s__\\n' ready; until [ \"$(stty size)\" = '17 37' ]; do sleep 0.01; done; printf '__RESIZED_%s__\\n' \"$(stty size)\"\n")
	output.wait(t, ctx, "__RESIZE_ready__")
	updates <- core.TerminalSize{Columns: 37, Rows: 17}
	// The condition above is bounded by the test context, not by a guessed delay.
	output.wait(t, ctx, "__RESIZED_17 37__")
	send("printf '__FINAL_%s__\\n' complete; exit 17\n")
	select {
	case err := <-done:
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 17 {
			t.Fatalf("exit = %v, want 17", err)
		}
	case <-ctx.Done():
		t.Fatal("shell exit waited for open client input")
	}
	output.wait(t, ctx, "__FINAL_complete__")
}

func TestSizedInteractivePTYSendsSIGWINCH(t *testing.T) {
	if os.Getenv("HACO_TEST_RESIZE_CHILD") == "1" {
		changes := make(chan os.Signal, 1)
		signal.Notify(changes, syscall.SIGWINCH)
		fmt.Print("READY")
		<-changes
		columns, rows, err := term.GetSize(int(os.Stdout.Fd()))
		if err != nil {
			os.Exit(2)
		}
		fmt.Printf("SIZE=%dx%d", columns, rows)
		os.Exit(0)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	input, writer := io.Pipe()
	defer input.Close()
	defer writer.Close()
	output := &terminalTranscript{changed: make(chan struct{}, 1)}
	updates := make(chan core.TerminalSize, 1)
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSizedInteractivePTYSendsSIGWINCH$")
	cmd.Env = append(os.Environ(), "HACO_TEST_RESIZE_CHILD=1")
	cmd.Stdout, cmd.Stderr = output, output
	done := make(chan error, 1)
	go func() {
		done <- runSizedInteractiveCommand(ctx, cmd, input, core.TerminalMetadata{Columns: 80, Rows: 24, Resizes: updates})
	}()
	output.wait(t, ctx, "READY")
	updates <- core.TerminalSize{Columns: 120, Rows: 40}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("child did not receive SIGWINCH")
	}
	output.wait(t, ctx, "SIZE=120x40")
}

func TestSizedInteractivePTYDisconnectAndInvalidDimensions(t *testing.T) {
	for _, size := range [][2]int{{0, 24}, {80, 0}, {-1, 24}, {65536, 24}} {
		master, slave, err := openInteractivePTY(size[0], size[1])
		if master != nil {
			master.Close()
		}
		if slave != nil {
			slave.Close()
		}
		if err == nil {
			t.Fatalf("accepted dimensions %v", size)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	input, writer := io.Pipe()
	defer input.Close()
	output := &terminalTranscript{changed: make(chan struct{}, 1)}
	cmd := exec.CommandContext(ctx, "/bin/bash", "-c", "printf READY; read -r line")
	cmd.Stdout, cmd.Stderr = output, output
	done := make(chan error, 1)
	go func() {
		done <- runSizedInteractiveCommand(ctx, cmd, input, core.TerminalMetadata{Columns: 80, Rows: 24})
	}()
	output.wait(t, ctx, "READY")
	writer.Close()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("disconnected PTY kept the process alive")
	}
}
