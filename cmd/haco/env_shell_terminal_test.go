package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type shellOnlyEnvironmentController struct {
	conn net.Conn
	err  error
}

func (c *shellOnlyEnvironmentController) CreateEnvironment(context.Context, controlapi.EnvironmentCreateRequest) (core.Environment, error) {
	return core.Environment{}, nil
}

func (c *shellOnlyEnvironmentController) ListEnvironments(context.Context) ([]core.Environment, error) {
	return nil, nil
}

func (c *shellOnlyEnvironmentController) EnvironmentStatus(context.Context, string) (core.EnvironmentStatus, error) {
	return core.EnvironmentStatus{}, nil
}

func (c *shellOnlyEnvironmentController) ExecEnvironment(context.Context, string, []string) (core.ExecutionResult, error) {
	return core.ExecutionResult{}, nil
}

func (c *shellOnlyEnvironmentController) OpenEnvironmentShell(context.Context, string) (net.Conn, error) {
	return c.conn, c.err
}

func (c *shellOnlyEnvironmentController) DeleteEnvironment(context.Context, string) error {
	return nil
}

type shellFailingWriter struct {
	err error
}

func (w shellFailingWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func TestPrepareInteractiveTerminalLeavesNonTTYInputUntouched(t *testing.T) {
	restore, err := prepareInteractiveTerminal(bytes.NewBufferString("exit\n"))
	if err != nil {
		t.Fatal(err)
	}
	if restore != nil {
		t.Fatal("non-TTY input unexpectedly changed terminal mode")
	}
}

func TestEnvironmentClientShellPreparesAndRestoresTerminal(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	client := &shellOnlyEnvironmentController{conn: clientConn}

	go func() {
		_, _ = io.WriteString(serverConn, "shell-ready\r\n")
		_ = serverConn.Close()
	}()

	prepared := 0
	restored := 0
	var stdout bytes.Buffer
	err := environmentClientShellWithTerminal(
		context.Background(),
		client,
		[]string{"demo"},
		bytes.NewBuffer(nil),
		&stdout,
		func(io.Reader) (func() error, error) {
			prepared++
			return func() error {
				restored++
				return nil
			}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if prepared != 1 || restored != 1 {
		t.Fatalf("terminal lifecycle: prepared=%d restored=%d, want 1/1", prepared, restored)
	}
	if stdout.String() != "shell-ready\r\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestEnvironmentClientShellRestoresTerminalAfterStreamError(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	client := &shellOnlyEnvironmentController{conn: clientConn}
	writeErr := errors.New("stdout failed")

	go func() {
		_, _ = io.WriteString(serverConn, "output")
		_ = serverConn.Close()
	}()

	restored := 0
	err := environmentClientShellWithTerminal(
		context.Background(),
		client,
		[]string{"demo"},
		bytes.NewBuffer(nil),
		shellFailingWriter{err: writeErr},
		func(io.Reader) (func() error, error) {
			return func() error {
				restored++
				return nil
			}, nil
		},
	)
	if !errors.Is(err, writeErr) {
		t.Fatalf("error = %v, want %v", err, writeErr)
	}
	if restored != 1 {
		t.Fatalf("terminal restore calls = %d, want 1", restored)
	}
}

func TestEnvironmentClientShellReturnsRestoreFailure(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	client := &shellOnlyEnvironmentController{conn: clientConn}
	restoreErr := errors.New("restore failed")

	go func() {
		_ = serverConn.Close()
	}()

	err := environmentClientShellWithTerminal(
		context.Background(),
		client,
		[]string{"demo"},
		bytes.NewBuffer(nil),
		io.Discard,
		func(io.Reader) (func() error, error) {
			return func() error { return restoreErr }, nil
		},
	)
	if !errors.Is(err, restoreErr) {
		t.Fatalf("error = %v, want restore failure %v", err, restoreErr)
	}
}

func TestEnvironmentClientShellDoesNotTouchTerminalWhenOpenFails(t *testing.T) {
	openErr := errors.New("open failed")
	client := &shellOnlyEnvironmentController{err: openErr}
	prepareCalls := 0

	err := environmentClientShellWithTerminal(
		context.Background(),
		client,
		[]string{"demo"},
		bytes.NewBuffer(nil),
		io.Discard,
		func(io.Reader) (func() error, error) {
			prepareCalls++
			return nil, nil
		},
	)
	if !errors.Is(err, openErr) {
		t.Fatalf("error = %v, want %v", err, openErr)
	}
	if prepareCalls != 0 {
		t.Fatalf("terminal preparer called %d times after open failure", prepareCalls)
	}
}
