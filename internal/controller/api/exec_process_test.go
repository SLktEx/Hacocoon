package controlapi

import (
	"bytes"
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type processTestLifecycle struct {
	calls   atomic.Int32
	execute func(context.Context, io.Reader, io.Writer, io.Writer) (core.ExecutionResult, error)
}

func (r *processTestLifecycle) ExecStream(ctx context.Context, name string, req core.ProcessRequest, in io.Reader, out, diagnostic io.Writer) (core.ExecutionResult, error) {
	r.calls.Add(1)
	if name != "dev" {
		return core.ExecutionResult{}, core.ErrInvalidArgument
	}
	return r.execute(ctx, in, out, diagnostic)
}
func processTestClient(t *testing.T, lifecycle *processTestLifecycle) *Client {
	t.Helper()
	path := doctorTestSocket(t, func(s *control.Server) {
		if err := RegisterExec(s, lifecycle); err != nil {
			t.Fatal(err)
		}
	})
	c, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func TestExecStreamPreservesIOExitAndCancellation(t *testing.T) {
	t.Run("IO and exit", func(t *testing.T) {
		data := bytes.Repeat([]byte("streamed\x00input\n"), 100000)
		service := &processTestLifecycle{execute: func(ctx context.Context, in io.Reader, out, diagnostic io.Writer) (core.ExecutionResult, error) {
			if _, err := io.Copy(out, in); err != nil {
				return core.ExecutionResult{}, err
			}
			io.WriteString(diagnostic, "stderr")
			return core.ExecutionResult{ExitCode: 17}, &control.SessionExitError{Code: 17}
		}}
		client := processTestClient(t, service)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var out, diagnostic bytes.Buffer
		result, err := client.ExecStream(ctx, "dev", core.ProcessRequest{Argv: []string{"cat"}}, bytes.NewReader(data), &out, &diagnostic)
		var code interface{ ExitCode() int }
		if !errors.As(err, &code) || code.ExitCode() != 17 || result.ExitCode != 17 || !bytes.Equal(data, out.Bytes()) || diagnostic.String() != "stderr" {
			t.Fatal(result, err)
		}
	})
	t.Run("disconnect", func(t *testing.T) {
		started := make(chan struct{})
		ended := make(chan struct{})
		service := &processTestLifecycle{execute: func(ctx context.Context, _ io.Reader, _, _ io.Writer) (core.ExecutionResult, error) {
			close(started)
			<-ctx.Done()
			close(ended)
			return core.ExecutionResult{}, ctx.Err()
		}}
		client := processTestClient(t, service)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() {
			_, err := client.ExecStream(ctx, "dev", core.ProcessRequest{Argv: []string{"wait"}}, bytes.NewReader(make([]byte, 1<<20)), io.Discard, io.Discard)
			done <- err
		}()
		select {
		case <-started:
		case <-time.After(3 * time.Second):
			t.Fatal("not started")
		}
		cancel()
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("false success")
			}
		case <-time.After(3 * time.Second):
			t.Fatal("client stuck")
		}
		select {
		case <-ended:
		case <-time.After(3 * time.Second):
			t.Fatal("server stuck")
		}
	})
}
func TestExecRejectsMalformedRequestAndReceipt(t *testing.T) {
	service := &processTestLifecycle{execute: func(context.Context, io.Reader, io.Writer, io.Writer) (core.ExecutionResult, error) {
		return core.ExecutionResult{}, nil
	}}
	client := processTestClient(t, service)
	for _, request := range []any{ExecStreamRequest{Environment: "dev", Process: core.ProcessRequest{Argv: []string{"bash"}, TTY: true}}, ExecStreamRequest{Environment: "dev", Process: core.ProcessRequest{Argv: []string{"bad\x00"}}}, ExecStreamRequest{Environment: "../dev", Process: core.ProcessRequest{Argv: []string{"true"}}}, map[string]any{"unreviewed": true}, "wrong type"} {
		conn, err := client.wire.OpenSession(context.Background(), MethodExecStream, request)
		if conn != nil {
			conn.Close()
		}
		if err == nil || service.calls.Load() != 0 {
			t.Fatal("accepted invalid request", err)
		}
	}
	for _, receipt := range []string{`{}`, `{"result":null}`, `{"result":{}} {}`, `{"result":{},"unknown":true}`} {
		if _, err := decodeExecResult([]byte(receipt)); !errors.Is(err, control.ErrProtocol) {
			t.Fatal("invalid receipt accepted", receipt, err)
		}
	}
	if _, err := client.ExecStream(context.Background(), "dev", core.ProcessRequest{Argv: []string{"true"}}, strings.NewReader(""), io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
}
