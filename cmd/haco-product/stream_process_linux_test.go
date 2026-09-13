//go:build linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type processStreamService struct{ socket string }

func (s processStreamService) DialStream(ctx context.Context, _ core.StreamTarget) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, "unix", s.socket)
}

// Exercise the real stdio adapter in a separate process, including readiness
// before the controller socket exists. Provider identity is covered separately.
func TestStreamProxyProcess(t *testing.T) {
	if os.Getenv("HACO_STREAM_PROCESS_CHILD") == "1" {
		os.Exit(runStream([]string{os.Getenv("HACO_STREAM_PROCESS_TARGET")}))
	}
	for _, remoteFirst := range []bool{false, true} {
		t.Run(map[bool]string{false: "input-half-close", true: "remote-EOF-with-open-input"}[remoteFirst], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			socket := filepath.Join(t.TempDir(), "controller.sock")
			targetSocket := filepath.Join(t.TempDir(), "service.sock")
			upstream, err := net.Listen("unix", targetSocket)
			if err != nil {
				t.Fatal(err)
			}
			defer upstream.Close()
			payload := bytes.Repeat([]byte{0, 255, 13, 10, 128}, 8192)
			targetDone := make(chan error, 1)
			go func() {
				c, e := upstream.Accept()
				if e != nil {
					targetDone <- e
					return
				}
				defer c.Close()
				if !remoteFirst {
					b, e := io.ReadAll(c)
					if e != nil {
						targetDone <- e
						return
					}
					if !bytes.Equal(b, payload) {
						targetDone <- io.ErrUnexpectedEOF
						return
					}
				}
				_, e = c.Write(payload)
				targetDone <- e
			}()
			token, err := core.EncodeStreamTarget(core.StreamTarget{Environment: "dev", Instance: "env-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Workspace: "work", AccessMode: core.WorkspaceReadWrite, Service: "ssh", Grant: "ssh-one"})
			if err != nil {
				t.Fatal(err)
			}
			command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestStreamProxyProcess$")
			command.Env = append(os.Environ(), "HACO_STREAM_PROCESS_CHILD=1", "HACO_STREAM_PROCESS_TARGET="+token, "HACO_CONTROL_SOCKET="+socket)
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr
			input, err := command.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			defer input.Close()
			if err = command.Start(); err != nil {
				t.Fatal(err)
			}
			time.Sleep(150 * time.Millisecond)
			server := control.NewServer()
			if err = server.Register(controlapi.MethodPing, func(context.Context, json.RawMessage) (any, error) {
				return controlapi.PingResponse{ProtocolVersion: 1}, nil
			}); err != nil {
				t.Fatal(err)
			}
			if err = controlapi.RegisterEnvironmentStreams(server, processStreamService{targetSocket}); err != nil {
				t.Fatal(err)
			}
			listener, err := control.ListenUnix(socket, 0600)
			if err != nil {
				t.Fatal(err)
			}
			serverDone := make(chan error, 1)
			go func() { serverDone <- server.Serve(ctx, listener) }()
			defer func() { cancel(); <-serverDone }()
			if !remoteFirst {
				if _, err = input.Write(payload); err != nil {
					t.Fatal(err)
				}
				input.Close()
			}
			if err = command.Wait(); err != nil {
				t.Fatalf("stdio process: %v stderr=%q", err, stderr.String())
			}
			if !bytes.Equal(stdout.Bytes(), payload) || stderr.Len() != 0 {
				t.Fatalf("stream contamination: stdout=%d stderr=%q", stdout.Len(), stderr.String())
			}
			select {
			case err = <-targetDone:
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal("target leaked")
			}
		})
	}
}
