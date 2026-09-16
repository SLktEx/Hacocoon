//go:build linux

package controlapi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	runapp "github.com/SLktEx/Hacocoon/internal/env/run"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

func TestRunTerminalPreservesDimensionsInputAndRestoresLocalState(t *testing.T) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = master.Close() }()
	if err := unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		t.Fatal(err)
	}
	number, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
	if err != nil {
		t.Fatal(err)
	}
	slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", number), os.O_RDWR|unix.O_NOCTTY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = slave.Close() }()
	before, err := term.GetState(int(slave.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Col: 82, Row: 27}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("COLORTERM", "truecolor")
	lifecycle := &processTestLifecycle{cleaned: make(chan error, 1)}
	lifecycle.execute = func(ctx context.Context, in io.Reader, out, diagnostic io.Writer) (core.ExecutionResult, error) {
		metadata := core.TerminalMetadataFromContext(ctx)
		if metadata.Columns != 82 || metadata.Rows != 27 || metadata.Term != "xterm-256color" || metadata.ColorTerm != "truecolor" {
			return core.ExecutionResult{}, fmt.Errorf("terminal identity changed: %+v", metadata)
		}
		data := make([]byte, 5)
		if _, err := io.ReadFull(in, data); err != nil {
			return core.ExecutionResult{}, err
		}
		if string(data) != "ping\n" {
			return core.ExecutionResult{}, errors.New("terminal input changed")
		}
		if _, err := out.Write(data); err != nil {
			return core.ExecutionResult{}, err
		}
		_, err := io.WriteString(diagnostic, "diagnostic")
		return core.ExecutionResult{}, err
	}
	client := processTestClient(t, lifecycle)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := io.WriteString(master, "ping\n"); err != nil {
		t.Fatal(err)
	}
	var out, diagnostic bytes.Buffer
	result, err := client.RunStream(ctx, runapp.Spec{WorkspacePath: "/retained", Argv: []string{"interactive-tool"}}, true, slave, &out, &diagnostic)
	if err != nil || !result.CleanedUp || out.String() != "ping\n" || diagnostic.String() != "diagnostic" || lifecycle.created.Load() != 1 {
		t.Fatal("interactive run lost IO or cleanup", result, out.String(), diagnostic.String(), err)
	}
	if err := <-lifecycle.cleaned; err != nil {
		t.Fatal("cleanup lost independent context", err)
	}
	after, err := term.GetState(int(slave.Fd()))
	if err != nil || !reflect.DeepEqual(after, before) {
		t.Fatal("local terminal state was not restored", err)
	}
	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{}); err != nil {
		t.Fatal(err)
	}
	_, err = client.RunStream(ctx, runapp.Spec{Argv: []string{"interactive-tool"}}, true, slave, io.Discard, io.Discard)
	if !errors.Is(err, core.ErrInvalidArgument) || lifecycle.created.Load() != 1 {
		t.Fatal("terminal without dimensions created an Environment", lifecycle.created.Load(), err)
	}
}
