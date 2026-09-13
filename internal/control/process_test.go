package control

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

type completedProcessTestSession struct {
	net.Conn
	err error
}

func (s completedProcessTestSession) Wait(context.Context) error { return s.err }

func TestProcessStreamingPreservesLargeInputSeparateOutputAndEOF(t *testing.T) {
	server, wire := net.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	data := bytes.Repeat([]byte{0, 1, 255, '\n'}, 300000)
	done := make(chan error, 1)
	go func() {
		done <- ServeProcess(ctx, server, func(ctx context.Context, in io.Reader, out, diagnostic io.Writer) ([]byte, error) {
			got, err := io.ReadAll(in)
			if err != nil {
				return nil, err
			}
			if !bytes.Equal(got, data) || ctx.Err() != nil {
				return nil, errors.New("stdin EOF canceled or changed data")
			}
			if _, err := out.Write(got); err != nil {
				return nil, err
			}
			if _, err := diagnostic.Write([]byte("separate diagnostic")); err != nil {
				return nil, err
			}
			return []byte(`{"exit_code":17,"cleaned_up":true}`), nil
		})
	}()
	var diagnostic bytes.Buffer
	client, err := NewProcessConn(ctx, completedProcessTestSession{Conn: wire}, &diagnostic)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	written := make(chan error, 1)
	go func() {
		_, err := client.Write(data)
		if err == nil {
			err = client.CloseWrite()
		}
		written <- err
	}()
	got, err := io.ReadAll(client)
	if err != nil || !bytes.Equal(got, data) || diagnostic.String() != "separate diagnostic" {
		t.Fatal("output changed", len(got), err)
	}
	if err := <-written; err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	result, err := client.Result()
	if err != nil || string(result) != `{"exit_code":17,"cleaned_up":true}` {
		t.Fatal(string(result), err)
	}
}

func TestProcessDisconnectCancelsWithBackpressuredUnconsumedStdin(t *testing.T) {
	server, wire := net.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	started, cleaned := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- ServeProcess(ctx, server, func(ctx context.Context, _ io.Reader, _, _ io.Writer) ([]byte, error) {
			close(started)
			<-ctx.Done()
			close(cleaned)
			return []byte("cleanup observed"), nil
		})
	}()
	client, err := NewProcessConn(ctx, completedProcessTestSession{Conn: wire}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	output := make(chan error, 1)
	go func() { _, err := io.Copy(io.Discard, client); output <- err }()
	written := make(chan error, 1)
	go func() { _, err := client.Write(make([]byte, processChunk*4)); written <- err }()
	<-started
	_ = client.Close()
	select {
	case <-cleaned:
	case <-time.After(time.Second):
		t.Fatal("ignored stdin hid disconnect")
	}
	if err := <-written; err == nil {
		t.Fatal("closed input succeeded")
	}
	if err := <-done; err == nil {
		t.Fatal("disconnect succeeded")
	}
	<-output
	if _, err := client.Result(); err == nil {
		t.Fatal("unknown cleanup confirmed")
	}
}

func TestProcessClientRejectsInvalidCompletionAndCredit(t *testing.T) {
	for _, mode := range []string{"no-result", "trailing-output", "duplicate-result", "excess-credit", "oversized", "partial-header"} {
		t.Run(mode, func(t *testing.T) {
			server, wire := net.Pipe()
			client, err := NewProcessConn(context.Background(), completedProcessTestSession{Conn: wire}, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			go func() {
				defer server.Close()
				w := &processWriter{conn: server}
				switch mode {
				case "trailing-output", "duplicate-result":
					_ = w.frame(processInputStop, nil)
					_, _, _ = readProcessFrame(server)
					_ = w.frame(processResult, []byte("receipt"))
					kind := processOutput
					if mode == "duplicate-result" {
						kind = processResult
					}
					// A hostile peer bypasses the production writer's final seal.
					_ = (&processWriter{conn: server}).frame(kind, []byte("trailing"))
				case "excess-credit":
					var credit [4]byte
					binary.BigEndian.PutUint32(credit[:], processChunk+1)
					_ = w.frame(processCredit, credit[:])
				case "oversized":
					var header [5]byte
					header[0] = processOutput
					binary.BigEndian.PutUint32(header[1:], processChunk+1)
					_, _ = server.Write(header[:])
				case "partial-header":
					_, _ = server.Write([]byte{processResult})
				}
			}()
			if _, err := io.ReadAll(client); !errors.Is(err, ErrProtocol) {
				t.Fatal("invalid stream accepted", err)
			}
			if _, err := client.Result(); err == nil {
				t.Fatal("invalid completion published")
			}
		})
	}
}

func TestProcessServerRejectsInputAfterEOFAndCreditOverrun(t *testing.T) {
	for _, mode := range []string{"after-eof", "duplicate-eof", "credit-overrun"} {
		t.Run(mode, func(t *testing.T) {
			server, client := net.Pipe()
			defer client.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				done <- ServeProcess(ctx, server, func(ctx context.Context, _ io.Reader, _, _ io.Writer) ([]byte, error) {
					<-ctx.Done()
					return []byte("receipt"), nil
				})
			}()
			if kind, _, err := readProcessFrame(client); kind != processCredit || err != nil {
				t.Fatal(kind, err)
			}
			w := &processWriter{conn: client}
			switch mode {
			case "credit-overrun":
				if err := w.frame(processInput, make([]byte, processChunk)); err != nil {
					t.Fatal(err)
				}
				_ = w.frame(processInput, []byte("excess"))
			default:
				if err := w.frame(processInputEOF, nil); err != nil {
					t.Fatal(err)
				}
				if mode == "after-eof" {
					_ = w.frame(processInput, []byte("after"))
				} else {
					_ = w.frame(processInputEOF, nil)
				}
			}
			if err := <-done; !errors.Is(err, ErrProtocol) {
				t.Fatal("invalid input did not cancel", err)
			}
		})
	}
}
