package streamio

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"
)

// A real child exercises OS pipe closure, process exit, and unread output.
func TestFramedProcessChild(t *testing.T) {
	mode := os.Getenv("HACO_TEST_FRAMED_CHILD")
	if mode == "" {
		return
	}
	if mode == "crash" {
		os.Exit(7)
	}
	if mode == "bridge" {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			os.Exit(16)
		}
		go func() {
			peer, err := listener.Accept()
			if err != nil {
				return
			}
			defer func() { _ = peer.Close() }()
			data, err := io.ReadAll(peer)
			if err != nil {
				return
			}
			_, _ = peer.Write(data)
			_ = peer.(*net.TCPConn).CloseWrite()
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err = BridgeStdio(ctx, os.Stdin, os.Stdout, func(ctx context.Context) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "tcp", listener.Addr().String())
		})
		if err != nil {
			os.Exit(17)
		}
		os.Exit(0)
	}
	c := NewFramedConn(os.Stdin, os.Stdout)
	switch mode {
	case "echo":
		data, err := io.ReadAll(c)
		if err != nil {
			os.Exit(11)
		}
		if _, err = c.Write(data); err != nil {
			os.Exit(12)
		}
		if err = c.CloseWrite(); err != nil {
			os.Exit(13)
		}
		os.Exit(0)
	case "blocked":
		time.Sleep(time.Minute)
		os.Exit(14)
	default:
		os.Exit(15)
	}
}

func framedChild(t *testing.T, mode string) *exec.Cmd {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestFramedProcessChild$")
	cmd.Env = append(os.Environ(), "HACO_TEST_FRAMED_CHILD="+mode)
	return cmd
}

func TestFramedProcessDrainsOutputAfterChildExit(t *testing.T) {
	for _, mode := range []string{"echo", "bridge"} {
		t.Run(mode, func(t *testing.T) { testFramedChildRoundTrip(t, framedChild(t, mode)) })
	}
}

func testFramedChildRoundTrip(t *testing.T, cmd *exec.Cmd) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := StartFramedProcess(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	data := bytes.Repeat([]byte{0, 255, 10, 13}, 512<<10)
	if _, err = conn.Write(data); err != nil {
		t.Fatal(err)
	}
	if err = conn.(interface{ CloseWrite() error }).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	// Deliberately let the child fill the pipe before the reader starts.
	time.Sleep(20 * time.Millisecond)
	got, err := io.ReadAll(conn)
	if err != nil || !bytes.Equal(data, got) {
		t.Fatalf("received %d/%d: %v", len(got), len(data), err)
	}
}

func TestFramedProcessCancellationReapsBlockedChild(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := framedChild(t, "blocked")
	conn, err := StartFramedProcess(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	done := make(chan error, 1)
	go func() { _, err := conn.Write(make([]byte, 8<<20)); done <- err }()
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("blocked write succeeded")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("blocked write survived cancellation")
	}
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() { _ = conn.Close() })
	}
	workers.Wait()
	if cmd.ProcessState == nil {
		t.Fatal("child was not reaped")
	}
}

func TestFramedProcessCrashIsNotApplicationEOF(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := framedChild(t, "crash")
	conn, err := StartFramedProcess(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.ReadAll(conn)
	_ = conn.Close()
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("crash: %v", err)
	}
	if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 7 {
		t.Fatal("missing exit state", cmd.ProcessState)
	}
}

func TestFramedProcessAlreadyCanceledDoesNotStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd := framedChild(t, "blocked")
	_, err := StartFramedProcess(ctx, cmd)
	if !errors.Is(err, context.Canceled) || cmd.Process != nil {
		t.Fatal(err, cmd.Process)
	}
}
