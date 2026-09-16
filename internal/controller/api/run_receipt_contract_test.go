package controlapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	runapp "github.com/SLktEx/Hacocoon/internal/env/run"
)

func TestRunClientsRequireExplicitExecutionReceipt(t *testing.T) {
	for _, process := range []bool{false, true} {
		for _, mode := range []string{"null", "missing", "null-result", "unknown", "trailing", "broken", "success", "recovery-required", "invalid-argument"} {
			t.Run(map[bool]string{false: "captured", true: "process"}[process]+"/"+mode, func(t *testing.T) {
				want := runapp.Result{Environment: "run-owned", CleanedUp: true, Execution: runapp.ExecutionResult{Stdout: "done", StdoutBytes: 4}}
				response := runResponse{Result: want}
				valid := mode == "success" || mode == "recovery-required" || mode == "invalid-argument"
				if mode == "recovery-required" {
					want.CleanedUp = false
					want.Execution.ExitCode = 7
					response = runResponse{Result: want, Error: &responseStatus{Code: "recovery_required", Message: "owned Environment requires recovery", ExitCode: 7}}
				}
				if mode == "invalid-argument" {
					want = runapp.Result{}
					response = runResponse{Error: &responseStatus{Code: "invalid_argument", Message: "invalid run request"}}
				}
				data, err := json.Marshal(response)
				if err != nil {
					t.Fatal(err)
				}
				switch mode {
				case "null":
					data = []byte(`null`)
				case "missing":
					data = []byte(`{}`)
				case "null-result":
					data = []byte(`{"result":null}`)
				case "unknown":
					data = append(data[:len(data)-1], []byte(`,"unverified":true}`)...)
				case "trailing":
					data = append(data, []byte(` {}`)...)
				case "broken":
					data = []byte(`{broken}`)
				}
				var calls atomic.Int32
				path := doctorTestSocket(t, func(s *control.Server) {
					method := MethodRun
					if process {
						method = MethodRunStream
					}
					if err := s.RegisterStream(method, func(context.Context, json.RawMessage) (control.Stream, error) {
						calls.Add(1)
						return func(ctx context.Context, conn net.Conn) error {
							if process {
								return control.ServeProcess(ctx, conn, func(_ context.Context, in io.Reader, _, _ io.Writer) ([]byte, error) {
									_, err := io.Copy(io.Discard, in)
									return data, err
								})
							}
							_, err := conn.Write(data)
							return err
						}, nil
					}); err != nil {
						t.Fatal(err)
					}
				})
				client, err := NewClient(path)
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				spec := runapp.Spec{WorkspacePath: "/retained", Argv: []string{"true"}}
				var got runapp.Result
				if process {
					got, err = client.RunStream(ctx, spec, false, strings.NewReader(""), io.Discard, io.Discard)
				} else {
					got, err = client.Run(ctx, spec)
				}
				if !valid {
					if !errors.Is(err, control.ErrProtocol) || got != (runapp.Result{}) {
						t.Fatal("unconfirmed execution receipt was accepted", got, err)
					}
				} else {
					if !reflect.DeepEqual(got, want) {
						t.Fatal("execution or cleanup result changed", got, want)
					}
					if response.Error == nil {
						if err != nil {
							t.Fatal(err)
						}
					} else {
						var status *control.StatusError
						if !errors.As(err, &status) || status.Code != response.Error.Code {
							t.Fatal("failure receipt lost status", err)
						}
						if response.Error.ExitCode > 0 {
							var exit interface{ ExitCode() int }
							if !errors.As(err, &exit) || exit.ExitCode() != 7 {
								t.Fatal("execution exit lost behind cleanup failure", err)
							}
						}
					}
				}
				if calls.Load() != 1 {
					t.Fatal("unconfirmed run retried", calls.Load())
				}
			})
		}
	}
}

func TestRunStreamInvalidLocalIOCannotCreateEnvironment(t *testing.T) {
	var dials atomic.Int32
	client, err := NewClientWithDialer(func(context.Context) (net.Conn, error) {
		dials.Add(1)
		return nil, errors.New("unexpected controller contact")
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"client", "context", "stdin", "stdout", "stderr", "tty-reader"} {
		t.Run(mode, func(t *testing.T) {
			selected, ctx := client, context.Background()
			var in io.Reader = strings.NewReader("")
			out, diagnostic := io.Discard, io.Discard
			switch mode {
			case "client":
				selected = nil
			case "context":
				ctx = nil
			case "stdin":
				in = nil
			case "stdout":
				out = nil
			case "stderr":
				diagnostic = nil
			}
			result, err := selected.RunStream(ctx, runapp.Spec{Argv: []string{"true"}}, mode == "tty-reader", in, out, diagnostic)
			if !errors.Is(err, core.ErrInvalidArgument) || result != (runapp.Result{}) || dials.Load() != 0 {
				t.Fatal("invalid local stream contacted controller", result, dials.Load(), err)
			}
		})
	}
}
