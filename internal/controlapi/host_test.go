package controlapi

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type fakeHostService struct {
	prepareErr error
}

func (f *fakeHostService) PrepareTrustedHostShellStream(context.Context) (func(context.Context, io.Reader, io.Writer, io.Writer) error, error) {
	if f.prepareErr != nil {
		return nil, f.prepareErr
	}
	return func(_ context.Context, stdin io.Reader, stdout, _ io.Writer) error {
		buffer := make([]byte, 4)
		if _, err := io.ReadFull(stdin, buffer); err != nil {
			return err
		}
		_, err := stdout.Write(buffer)
		return err
	}, nil
}

func TestTrustedHostShellRoundTripOverUnixSocket(t *testing.T) {
	client, cancel := startHostControlAPITestServer(t, &fakeHostService{})
	defer cancel()

	stream, err := client.OpenTrustedHostShell(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Write([]byte("host")); err != nil {
		stream.Close()
		t.Fatal(err)
	}
	response := make([]byte, 4)
	if _, err := io.ReadFull(stream, response); err != nil {
		stream.Close()
		t.Fatal(err)
	}
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
	if string(response) != "host" {
		t.Fatalf("host shell response = %q", response)
	}
}

func TestTrustedHostShellPreparationFailsBeforeStreamOpens(t *testing.T) {
	client, cancel := startHostControlAPITestServer(t, &fakeHostService{prepareErr: core.ErrRuntimeUnavailable})
	defer cancel()

	_, err := client.OpenTrustedHostShell(context.Background())
	var status *control.StatusError
	if !errors.As(err, &status) || status.Code != "unavailable" {
		t.Fatalf("error = %v, want unavailable StatusError", err)
	}
}

func startHostControlAPITestServer(t *testing.T, hosts *fakeHostService) (*Client, context.CancelFunc) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "control.sock")
	listener, err := control.ListenUnix(path, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	server := control.NewServer()
	if err := RegisterHost(server, hosts); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("controller did not stop")
		}
	})
	client, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	return client, cancel
}
