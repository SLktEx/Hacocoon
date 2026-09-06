package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type fakeHostController struct {
	stream    net.Conn
	openErr   error
	openCalls int
}

func (f *fakeHostController) OpenTrustedHostShell(context.Context) (net.Conn, error) {
	f.openCalls++
	return f.stream, f.openErr
}

func TestHandleHostClientArgsRejectsEnsureBeforeLocalComposition(t *testing.T) {
	factoryCalls := 0
	handled, err := handleHostClientArgs(context.Background(), []string{"host", "ensure"}, func() (hostControllerClient, error) {
		factoryCalls++
		return &fakeHostController{}, nil
	}, bytes.NewBuffer(nil), bytes.NewBuffer(nil), bytes.NewBuffer(nil))
	if !handled || !errors.Is(err, core.ErrUnsupported) || !strings.Contains(err.Error(), "haco setup") {
		t.Fatalf("legacy bootstrap was not rejected before composition: handled=%v err=%v", handled, err)
	}
	if factoryCalls != 0 {
		t.Fatalf("controller factory calls = %d, want 0", factoryCalls)
	}
}

func TestHandleHostClientArgsShellWorksWithoutLocalComposition(t *testing.T) {
	t.Setenv("LC_ALL", "C")
	clientSide, serverSide := net.Pipe()
	defer clientSide.Close()
	serverDone := make(chan error, 1)
	go func() {
		defer serverSide.Close()
		request := make([]byte, 4)
		if _, err := io.ReadFull(serverSide, request); err != nil {
			serverDone <- err
			return
		}
		if string(request) != "ping" {
			serverDone <- errors.New("unexpected request")
			return
		}
		_, err := serverSide.Write([]byte("pong"))
		serverDone <- err
	}()

	client := &fakeHostController{stream: clientSide}
	var stdout, stderr bytes.Buffer
	handled, err := handleHostClientArgs(context.Background(), []string{"host", "shell"}, func() (hostControllerClient, error) {
		return client, nil
	}, bytes.NewBufferString("ping"), &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("haco host shell was not intercepted before local composition")
	}
	if client.openCalls != 1 {
		t.Fatalf("OpenTrustedHostShell calls = %d, want 1", client.openCalls)
	}
	if stdout.String() != "pong" {
		t.Fatalf("stdout = %q, want pong", stdout.String())
	}
	if !strings.Contains(stderr.String(), "haco-host") || !strings.Contains(stderr.String(), "privileged management environment") {
		t.Fatalf("warning = %q", stderr.String())
	}
	if err := <-serverDone; err != nil {
		t.Fatal(err)
	}
}

func TestHostClientShellRejectsArguments(t *testing.T) {
	err := hostClientShell(context.Background(), &fakeHostController{}, []string{"extra"}, bytes.NewBuffer(nil), bytes.NewBuffer(nil), bytes.NewBuffer(nil))
	if !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("error = %v, want invalid argument", err)
	}
}

func TestLocalHostDispatchCannotFallbackForShell(t *testing.T) {
	err := hostCommand(context.Background(), nil, []string{"shell"})
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("error = %v, want incompatible state", err)
	}
}
