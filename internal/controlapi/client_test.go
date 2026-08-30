package controlapi

import (
	"context"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type fakeEnvironments struct{}

func (fakeEnvironments) Exec(_ context.Context, name string, request core.ExecutionRequest) (core.ExecutionResult, error) {
	if name != "demo" || len(request.Argv) != 2 || request.Argv[0] != "printf" || request.Argv[1] != "ok" {
		return core.ExecutionResult{}, core.ErrInvalidArgument
	}
	return core.ExecutionResult{ExitCode: 0, Stdout: "ok"}, nil
}

func (fakeEnvironments) ShellStream(_ context.Context, name string, stdin io.Reader, stdout, _ io.Writer) error {
	if name != "demo" {
		return core.ErrNotFound
	}
	buffer := make([]byte, 4)
	if _, err := io.ReadFull(stdin, buffer); err != nil {
		return err
	}
	_, err := stdout.Write(buffer)
	return err
}

func TestTypedClientOverUnixSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sock")
	listener, err := control.ListenUnix(path, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	server := control.NewServer()
	if err := Register(server, fakeEnvironments{}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("controller did not stop")
		}
	}()

	client, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	ping, err := client.Ping(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ping.ProtocolVersion != control.ProtocolVersion {
		t.Fatalf("protocol version = %d, want %d", ping.ProtocolVersion, control.ProtocolVersion)
	}

	result, err := client.ExecEnvironment(context.Background(), "demo", []string{"printf", "ok"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 || result.Stdout != "ok" {
		t.Fatalf("exec result = %#v", result)
	}

	stream, err := client.OpenEnvironmentShell(context.Background(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	if _, err := stream.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	response := make([]byte, 4)
	if _, err := io.ReadFull(stream, response); err != nil {
		t.Fatal(err)
	}
	if string(response) != "ping" {
		t.Fatalf("shell stream response = %q", response)
	}
}
