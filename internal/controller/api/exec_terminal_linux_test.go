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
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

func TestExecTerminalPreservesDimensionsInputAndRestoresLocalState(t *testing.T) {
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
	lifecycle := &processTestLifecycle{}
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
	result, err := client.ExecStream(ctx, "dev", core.ProcessRequest{Argv: []string{"interactive-tool"}, TTY: true}, slave, &out, &diagnostic)
	if err != nil || out.String() != "ping\n" || diagnostic.String() != "diagnostic" || lifecycle.calls.Load() != 1 {
		t.Fatal("interactive run lost IO or cleanup", result, out.String(), diagnostic.String(), err)
	}
	after, err := term.GetState(int(slave.Fd()))
	if err != nil || !reflect.DeepEqual(after, before) {
		t.Fatal("local terminal state was not restored", err)
	}
	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{}); err != nil {
		t.Fatal(err)
	}
	_, err = client.ExecStream(ctx, "dev", core.ProcessRequest{Argv: []string{"interactive-tool"}, TTY: true}, slave, io.Discard, io.Discard)
	if !errors.Is(err, core.ErrInvalidArgument) || lifecycle.calls.Load() != 1 {
		t.Fatal("terminal without dimensions created an Environment", lifecycle.calls.Load(), err)
	}
}

func TestExecTerminalSizePreservesCallerFileFlags(t *testing.T) {
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
	raw, err := slave.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var before, after int
	var descriptor uintptr
	var controlErr error
	if err := raw.Control(func(fd uintptr) {
		descriptor = fd
		controlErr = unix.IoctlSetWinsize(int(fd), unix.TIOCSWINSZ, &unix.Winsize{Col: 83, Row: 29})
		if controlErr == nil {
			before, controlErr = unix.FcntlInt(fd, unix.F_GETFL, 0)
		}
	}); err != nil || controlErr != nil {
		t.Fatal(err, controlErr)
	}
	if before&unix.O_NONBLOCK == 0 {
		t.Fatal("fixture must be a Go-pollable terminal before metadata inspection")
	}
	columns, rows, err := execTerminalSize(slave)
	if err != nil || columns != 83 || rows != 29 {
		t.Fatal("terminal metadata changed", columns, rows, err)
	}
	if err := raw.Control(func(fd uintptr) { after, controlErr = unix.FcntlInt(fd, unix.F_GETFL, 0) }); err != nil || controlErr != nil {
		t.Fatal(err, controlErr)
	}
	if before != after {
		t.Fatalf("metadata inspection changed caller flags: %x -> %x", before, after)
	}
	// -t without -i supplies an empty reader that still identifies its TTY.
	columns, rows, err = execTerminalSize(metadataInput{Reader: bytes.NewReader(nil), fd: descriptor})
	if err != nil || columns != 83 || rows != 29 {
		t.Fatal("metadata-only terminal input changed", columns, rows, err)
	}
	if _, _, err := execTerminalSize(bytes.NewReader(nil)); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatal("non-terminal reader accepted", err)
	}
	_ = slave.Close()
	if _, _, err := execTerminalSize(slave); err == nil {
		t.Fatal("closed terminal accepted")
	}
	if _, _, err := execTerminalSize((*os.File)(nil)); err == nil {
		t.Fatal("nil terminal accepted")
	}
	if _, _, err := execTerminalSize(metadataInput{Reader: bytes.NewReader(nil), fd: 1 << 30}); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatal("invalid terminal descriptor accepted", err)
	}
}

type metadataInput struct {
	io.Reader
	fd uintptr
}

func (r metadataInput) Fd() uintptr { return r.fd }
