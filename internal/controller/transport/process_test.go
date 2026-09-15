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
	defer func() { _ = client.Close() }()
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
	for _, mode := range []string{
		"no-result", "trailing-output", "duplicate-result", "excess-credit", "oversized", "partial-header",
		"empty-output", "empty-stderr", "short-credit", "zero-credit", "cumulative-credit", "stop-payload",
		"duplicate-stop", "output-after-stop", "empty-result", "result-before-stop", "unknown-frame", "partial-payload", "oversized-result",
	} {
		t.Run(mode, func(t *testing.T) {
			server, wire := net.Pipe()
			client, err := NewProcessConn(context.Background(), completedProcessTestSession{Conn: wire}, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = client.Close() }()
			if err := wire.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			done := make(chan struct{})
			go func() {
				defer close(done)
				defer func() { _ = server.Close() }()
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
				case "empty-output":
					_ = w.frame(processOutput, nil)
				case "empty-stderr":
					_ = w.frame(processErrorOutput, nil)
				case "short-credit":
					_ = w.frame(processCredit, []byte{1})
				case "zero-credit", "cumulative-credit":
					var credit [4]byte
					if mode == "cumulative-credit" {
						binary.BigEndian.PutUint32(credit[:], processChunk)
						_ = w.frame(processCredit, credit[:])
					}
					_ = w.frame(processCredit, credit[:])
				case "stop-payload":
					_ = w.frame(processInputStop, []byte("not empty"))
				case "duplicate-stop", "output-after-stop", "empty-result":
					_ = w.frame(processInputStop, nil)
					_, _, _ = readProcessFrame(server)
					kind, data := processResult, []byte(nil)
					switch mode {
					case "duplicate-stop":
						kind = processInputStop
					case "output-after-stop":
						kind, data = processOutput, []byte("late")
					}
					_ = (&processWriter{conn: server}).frame(kind, data)
				case "result-before-stop":
					_ = w.frame(processResult, []byte("unconfirmed receipt"))
				case "unknown-frame":
					_ = w.frame(255, nil)
				case "partial-payload", "oversized-result":
					var header [5]byte
					header[0] = processResult
					size := uint32(1)
					if mode == "oversized-result" {
						size = MaxProcessResult + 1
					}
					binary.BigEndian.PutUint32(header[1:], size)
					_, _ = server.Write(header[:])
				}
			}()
			if _, err := io.ReadAll(client); !errors.Is(err, ErrProtocol) {
				t.Fatal("invalid stream accepted", err)
			}
			if _, err := client.Result(); err == nil {
				t.Fatal("invalid completion published")
			}
			<-done
		})
	}
}

func TestProcessServerRejectsInputAfterEOFAndCreditOverrun(t *testing.T) {
	for _, mode := range []string{"after-eof", "duplicate-eof", "credit-overrun", "empty-input", "eof-payload", "unknown-frame"} {
		t.Run(mode, func(t *testing.T) {
			server, client := net.Pipe()
			defer func() { _ = client.Close() }()
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
			case "empty-input":
				_ = w.frame(processInput, nil)
			case "eof-payload":
				_ = w.frame(processInputEOF, []byte("not empty"))
			case "unknown-frame":
				_ = w.frame(255, nil)
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
