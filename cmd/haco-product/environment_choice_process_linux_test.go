//go:build linux

package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestDesktopChoiceTerminalProcess(t *testing.T) {
	if os.Getenv("HACO_TEST_CHOICE_CHILD") == "1" {
		os.Exit(runSSH([]string{"setup"}))
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("Python PTY support unavailable")
	}
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen unavailable")
	}
	for _, answer := range []string{"\n", "2\n"} {
		t.Run(strings.TrimSpace(answer), func(t *testing.T) {
			selected := core.Environment{Name: "zeta", RuntimeRef: "runtime-zeta", CreatedAt: time.Unix(100, 0), Workspace: core.Workspace{ID: "work-z"}}
			server := control.NewServer()
			var access atomic.Int32
			if err := server.Register(controlapi.MethodEnvironmentList, func(context.Context, json.RawMessage) (any, error) {
				return []core.Environment{selected, {Name: "alpha", Workspace: core.Workspace{ID: "work-a"}}}, nil
			}); err != nil {
				t.Fatal(err)
			}
			if err := server.Register(controlapi.MethodEnvironmentStatus, func(_ context.Context, payload json.RawMessage) (any, error) {
				var req controlapi.EnvironmentNameRequest
				if json.Unmarshal(payload, &req) != nil || req.Environment != "zeta" {
					return nil, core.ErrInvalidArgument
				}
				return core.EnvironmentStatus{Environment: selected, State: core.EnvironmentRunning}, nil
			}); err != nil {
				t.Fatal(err)
			}
			if err := server.Register(controlapi.MethodEnvironmentSSH, func(_ context.Context, payload json.RawMessage) (any, error) {
				var req controlapi.EnvironmentSSHRequest
				if json.Unmarshal(payload, &req) != nil || req.Environment != "zeta" {
					return nil, core.ErrInvalidArgument
				}
				access.Add(1)
				return core.ClientConnection{ID: "ssh-23001", Kind: "ssh", Host: "127.0.0.1", Port: 23001, TargetPort: 22, User: "root", HostPublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAABAgMEBQYHCAkKCwwNDg8QERITFBUWFxgZGhscHR4f"}, nil
			}); err != nil {
				t.Fatal(err)
			}
			socket := filepath.Join(t.TempDir(), "control.sock")
			listener, err := control.ListenUnix(socket, 0600)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() { done <- server.Serve(ctx, listener) }()
			defer func() { cancel(); <-done }()
			script := `import os, pty, select, subprocess, sys, time
master, slave = pty.openpty()
env = os.environ.copy()
env.update(HACO_TEST_CHOICE_CHILD='1', HACO_CONTROL_SOCKET=sys.argv[2], HOME=sys.argv[3], WSL_INTEROP='', WSL_DISTRO_NAME='')
process = subprocess.Popen([sys.argv[1], '-test.run=^TestDesktopChoiceTerminalProcess$'], stdin=slave, stdout=slave, stderr=slave, env=env)
os.close(slave)
output = b''
sent = False
deadline = time.monotonic() + 20
try:
    while time.monotonic() < deadline:
        if select.select([master], [], [], .1)[0]:
            try: chunk = os.read(master, 4096)
            except OSError: break
            if not chunk: break
            output += chunk
            if len(output) > 65536: raise RuntimeError('unbounded output')
            if not sent and b'Choose an Environment' in output:
                os.write(master, sys.argv[4].encode())
                sent = True
        if process.poll() is not None: break
    assert sent, 'no interactive choice prompt'
    assert process.wait(timeout=2) == 0, output.decode(errors='replace')
    print(output.decode(errors='replace'))
finally:
    if process.poll() is None: process.kill()
    process.wait()
    os.close(master)
`
			command := exec.Command(python, "-I", "-c", script, os.Args[0], socket, t.TempDir(), answer)
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("terminal choice: %v %s", err, output)
			}
			if answer == "\n" {
				if access.Load() != 0 || strings.Contains(string(output), "SSH ready:") {
					t.Fatal("cancel prepared access")
				}
			} else if access.Load() != 1 || !strings.Contains(string(output), "SSH ready: haco-zeta") {
				t.Fatalf("selection not used: %d %s", access.Load(), output)
			}
		})
	}
}
