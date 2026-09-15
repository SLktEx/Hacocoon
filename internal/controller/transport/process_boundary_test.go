package control

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func TestProcessConnectionKeepsResizeSeparateFromFramedBytes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server := NewServer()
	resized := make(chan [2]int, 1)
	if err := server.RegisterStream("process", func(ctx context.Context, _ json.RawMessage) (Stream, error) {
		SetTerminalResizeHandler(ctx, func(columns, rows int) error { resized <- [2]int{columns, rows}; return nil })
		return func(ctx context.Context, conn net.Conn) error {
			return ServeProcess(ctx, conn, func(_ context.Context, in io.Reader, out, diagnostic io.Writer) ([]byte, error) {
				if n, err := in.Read(nil); n != 0 || err != nil {
					return nil, errors.New("empty read changed stdin")
				}
				if _, err := io.Copy(out, in); err != nil {
					return nil, err
				}
				if _, err := diagnostic.Write([]byte("separate stderr")); err != nil {
					return nil, err
				}
				return []byte("exact receipt"), nil
			})
		}, nil
	}); err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(func(context.Context) (net.Conn, error) {
		local, remote := net.Pipe()
		go server.serveConn(ctx, remote)
		return local, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := client.OpenSession(ctx, "process", nil)
	if err != nil {
		t.Fatal(err)
	}
	var diagnostic bytes.Buffer
	process, err := NewProcessConn(ctx, session, &diagnostic)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = process.Close() }()
	if !process.SupportsResize() || process.Resize(ctx, 132, 43) != nil {
		t.Fatal("process lost negotiated resize")
	}
	if got := <-resized; got != [2]int{132, 43} {
		t.Fatal("resize changed dimensions", got)
	}
	if n, err := process.Read(nil); n != 0 || err != nil {
		t.Fatal("empty read consumed output", n, err)
	}
	if _, err := process.Result(); !errors.Is(err, ErrProtocol) {
		t.Fatal("unfinished process confirmed a result", err)
	}
	want := []byte{'p', 0, 255, '\n'}
	written := make(chan error, 1)
	go func() {
		_, err := process.Write(want)
		if err == nil {
			err = process.CloseWrite()
		}
		written <- err
	}()
	got, err := io.ReadAll(process)
	if err != nil || !bytes.Equal(got, want) || diagnostic.String() != "separate stderr" {
		t.Fatal("process channels mixed", got, diagnostic.String(), err)
	}
	if err := <-written; err != nil {
		t.Fatal(err)
	}
	if err := process.CloseWrite(); err != nil {
		t.Fatal("repeated EOF changed completion", err)
	}
	if n, err := process.Write([]byte("late input")); n != 0 || !errors.Is(err, net.ErrClosed) {
		t.Fatal("input accepted after EOF", n, err)
	}
	if n, err := process.Read(make([]byte, 1)); n != 0 || !errors.Is(err, io.EOF) {
		t.Fatal("completed output reopened", n, err)
	}
	receipt, err := process.Result()
	if err != nil || string(receipt) != "exact receipt" {
		t.Fatal("receipt not confirmed", string(receipt), err)
	}
	receipt[0] = 'X'
	again, err := process.Result()
	if err != nil || string(again) != "exact receipt" {
		t.Fatal("caller changed cached completion", string(again), err)
	}
	if err := process.Resize(ctx, 80, 24); !errors.Is(err, io.EOF) {
		t.Fatal("completed session accepted resize", err)
	}
}

type processBoundaryWriter func([]byte) (int, error)

func (f processBoundaryWriter) Write(data []byte) (int, error) { return f(data) }

func TestProcessDiagnosticFailureCannotConfirmCompletion(t *testing.T) {
	for _, mode := range []string{"failure", "zero", "negative", "overreported"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			wire, remote := net.Pipe()
			failed := errors.New("diagnostic destination failed")
			writer := processBoundaryWriter(func(data []byte) (int, error) {
				switch mode {
				case "failure":
					return 0, failed
				case "negative":
					return -1, nil
				case "overreported":
					return len(data) + 1, nil
				default:
					return 0, nil
				}
			})
			process, err := NewProcessConn(ctx, completedProcessTestSession{Conn: wire}, writer)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = process.Close() }()
			done := make(chan error, 1)
			go func() {
				done <- ServeProcess(ctx, remote, func(_ context.Context, _ io.Reader, _, diagnostic io.Writer) ([]byte, error) {
					_, err := diagnostic.Write([]byte("diagnostic bytes"))
					return []byte("claimed receipt"), err
				})
			}()
			want := error(io.ErrShortWrite)
			if mode == "failure" {
				want = failed
			}
			if data, err := io.ReadAll(process); len(data) != 0 || !errors.Is(err, want) {
				t.Fatal("failed diagnostic became stdout or lost cause", string(data), err)
			}
			if _, err := process.Result(); !errors.Is(err, ErrProtocol) {
				t.Fatal("failed output confirmed completion", err)
			}
			if err := <-done; err == nil {
				t.Fatal("closed client reported successful process transport")
			}
		})
	}
}
