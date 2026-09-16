package clientforward

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/cli/ui"
	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/storage/reclamation"
)

func testDelegation() delegation {
	return delegation{Version: 1, Installation: reclamation.WSLTarget{RegistrationID: "{11111111-1111-1111-1111-111111111111}", InstallationID: "22222222-2222-2222-2222-222222222222"}, Expires: time.Now().Add(20 * time.Second).UnixMilli(), Request: prepared{Target: core.EnvironmentTCPForward{Environment: "demo", Instance: "env-00000000000000000000000000000001", Address: "127.0.0.1", Port: 8080}, Listen: "127.0.0.1:0", Duration: time.Minute, Language: cliui.English}}
}

func TestDelegationRejectsUntrustedSelections(t *testing.T) {
	for _, change := range []func(*delegation){
		func(d *delegation) { d.Version = 2 },
		func(d *delegation) { d.Installation.RegistrationID = "--user root" },
		func(d *delegation) { d.Installation.InstallationID = "" },
		func(d *delegation) { d.Request.Target.Instance = "" },
		func(d *delegation) { d.Request.Target.Environment = "demo\nwarning" },
		func(d *delegation) { d.Request.Target.Address = "169.254.169.254" },
		func(d *delegation) { d.Request.Listen = "0.0.0.0:80" },
		func(d *delegation) { d.Request.Listen = "localhost:80" },
		func(d *delegation) { d.Request.Language = "ja;exit" },
		func(d *delegation) { d.Expires = time.Now().Add(-time.Second).UnixMilli() },
		func(d *delegation) { d.Expires = time.Now().Add(2 * time.Hour).UnixMilli() },
		func(d *delegation) { d.Request.Duration = 2 * time.Hour },
	} {
		d := testDelegation()
		change(&d)
		data, _ := json.Marshal(d)
		var input bytes.Buffer
		_ = binary.Write(&input, binary.BigEndian, uint32(len(data)))
		input.Write(data)
		if _, err := readDelegation(&input); err == nil {
			t.Fatalf("accepted %#v", d)
		}
	}
	for _, raw := range [][]byte{{}, {0, 0, 0, 0}, {0, 0, 32, 1}, {0, 0, 0, 10, '{'}} {
		if _, err := readDelegation(bytes.NewReader(raw)); err == nil {
			t.Fatalf("accepted %v", raw)
		}
	}
	for _, suffix := range []string{`,"command":"sh"}`, `} {}`} {
		data, _ := json.Marshal(testDelegation())
		data = append(data[:len(data)-1], suffix...)
		var input bytes.Buffer
		_ = binary.Write(&input, binary.BigEndian, uint32(len(data)))
		input.Write(data)
		if _, err := readDelegation(&input); err == nil {
			t.Fatal("accepted extra data")
		}
	}
}

type installationFixture struct{ target reclamation.WSLTarget }

func (f installationFixture) ReclamationTarget(context.Context) (reclamation.WSLTarget, error) {
	return f.target, nil
}

func delegationController(t *testing.T, installed reclamation.WSLTarget, application string) func(reclamation.WSLTarget) (*controlapi.Client, error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := control.NewServer()
	if err := controlapi.RegisterReclamationTarget(server, installationFixture{installed}); err != nil {
		t.Fatal(err)
	}
	if err := controlapi.RegisterForwardStreams(server, forwardFixture{address: application}); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, l) }()
	t.Cleanup(func() { cancel(); <-done })
	return func(reclamation.WSLTarget) (*controlapi.Client, error) {
		return controlapi.NewClientWithDialer(func(ctx context.Context) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "tcp", l.Addr().String())
		})
	}
}

func TestDelegationRefusesReinstalledAndReplacedTargetsBeforeListening(t *testing.T) {
	for _, kind := range []string{"installation", "environment"} {
		t.Run(kind, func(t *testing.T) {
			d := testDelegation()
			connect := delegationController(t, d.Installation, "")
			if kind == "installation" {
				d.Installation.InstallationID = "33333333-3333-3333-3333-333333333333"
			} else {
				d.Request.Target.Instance = "env-00000000000000000000000000000002"
			}
			var out, diagnostic bytes.Buffer
			if code := runInstalled(context.Background(), d, &out, &diagnostic, connect); code != 1 || out.Len() != 0 {
				t.Fatalf("code %d output %q", code, out.String())
			}
		})
	}
}

func TestDelegationLeaseClosesListener(t *testing.T) {
	for _, extra := range []bool{false, true} {
		t.Run(map[bool]string{false: "parent EOF", true: "extra bytes"}[extra], func(t *testing.T) {
			d := testDelegation()
			connect := delegationController(t, d.Installation, "")
			reader, writer := io.Pipe()
			t.Cleanup(func() { _ = reader.Close(); _ = writer.Close() })
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			ready := make(readyOutput, 1)
			var diagnostic bytes.Buffer
			done := make(chan int, 1)
			go func() { done <- RunDelegated(ctx, reader, ready, &diagnostic, connect) }()
			if err := writeDelegation(writer, d); err != nil {
				t.Fatal(err)
			}
			var address string
			select {
			case text := <-ready:
				address = regexp.MustCompile(`127\.0\.0\.1:[0-9]+`).FindString(text)
			case <-ctx.Done():
				t.Fatal("no listener")
			}
			if extra {
				_, _ = writer.Write([]byte{1})
			} else {
				_ = writer.Close()
			}
			select {
			case code := <-done:
				if (extra && code != 1) || (!extra && code != 0) {
					t.Fatalf("exit %d: %s", code, diagnostic.String())
				}
			case <-ctx.Done():
				t.Fatal("listener survived parent loss")
			}
			c, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
			if err == nil {
				_ = c.Close()
				t.Fatal("listener remains")
			}
		})
	}
}

// A real OS child proves the command's pipe lease and cancellation semantics on
// both Linux and Windows, without product-only fixture modes or WSL authority.
func TestCompanionChild(t *testing.T) {
	if os.Getenv("HACO_TEST_TUNNEL_CHILD") != "1" {
		return
	}
	code := RunDelegated(context.Background(), os.Stdin, os.Stdout, os.Stderr, func(target reclamation.WSLTarget) (*controlapi.Client, error) {
		return controlapi.NewClientWithDialer(func(ctx context.Context) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "tcp", os.Getenv("HACO_TEST_TUNNEL_CONTROLLER"))
		})
	})
	os.Exit(code)
}

func TestRealCompanionCancellationReapsChild(t *testing.T) {
	testRealCompanionCancellation(t, exec.CommandContext, func(cancel context.CancelFunc) { cancel() })
}

func testRealCompanionCancellation(t *testing.T, command func(context.Context, string, ...string) *exec.Cmd, interrupt func(context.CancelFunc)) {
	t.Helper()
	d := testDelegation()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	application, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_ = application.(*net.TCPListener).SetDeadline(time.Now().Add(8 * time.Second))
	applicationDone := make(chan struct{})
	go func() {
		defer close(applicationDone)
		c, err := application.Accept()
		if err != nil {
			return
		}
		defer func() { _ = c.Close() }()
		_ = c.SetDeadline(time.Now().Add(8 * time.Second))
		_, _ = io.Copy(c, c)
	}()
	defer func() { _ = application.Close(); <-applicationDone }()
	// Expose only a read-only fixture controller to this test-owned child.
	management, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := control.NewServer()
	if err := controlapi.RegisterReclamationTarget(server, installationFixture{d.Installation}); err != nil {
		t.Fatal(err)
	}
	if err := controlapi.RegisterForwardStreams(server, forwardFixture{address: application.Addr().String()}); err != nil {
		t.Fatal(err)
	}
	serverDone := make(chan error, 1)
	go func() { serverDone <- server.Serve(ctx, management) }()
	defer func() { cancel(); <-serverDone }()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	childCtx, stop := context.WithCancel(ctx)
	defer stop()
	cmd := command(childCtx, exe, "-test.run=^TestCompanionChild$")
	cmd.Env = append(os.Environ(), "HACO_TEST_TUNNEL_CHILD=1", "HACO_TEST_TUNNEL_CONTROLLER="+management.Addr().String())
	ready := make(readyOutput, 1)
	var diagnostic bytes.Buffer
	done := make(chan error, 1)
	go func() {
		code, err := runCompanion(childCtx, cmd, d, ready, &diagnostic)
		if code != 0 && err == nil {
			err = errors.New("child failed")
		}
		done <- err
	}()
	var address string
	select {
	case text := <-ready:
		address = regexp.MustCompile(`127\.0\.0\.1:[0-9]+`).FindString(text)
	case err := <-done:
		t.Fatalf("child before readiness: %v %s", err, diagnostic.String())
	case <-ctx.Done():
		t.Fatal("child not ready")
	}
	active, err := net.DialTimeout("tcp", address, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = active.Close() }()
	_ = active.SetDeadline(time.Now().Add(6 * time.Second))
	data := bytes.Repeat([]byte{0, 255, 13, 10}, 256<<10)
	writeDone := make(chan error, 1)
	go func() { _, err := active.Write(data); writeDone <- err }()
	answer := make([]byte, len(data))
	if _, err := io.ReadFull(active, answer); err != nil {
		t.Fatal(err)
	}
	if err := <-writeDone; err != nil || !bytes.Equal(data, answer) {
		t.Fatal("binary exchange", err)
	}
	interrupt(stop)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("cancel: %v %s", err, diagnostic.String())
		}
	case <-ctx.Done():
		t.Fatal("child not reaped")
	}
	var one [1]byte
	if n, err := active.Read(one[:]); n != 0 || err == nil {
		t.Fatal("active client survived cancellation")
	}
	select {
	case <-applicationDone:
	case <-ctx.Done():
		t.Fatal("upstream survived cancellation")
	}
	if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
		t.Fatal("missing child completion")
	}
	c, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
	if err == nil {
		_ = c.Close()
		t.Fatal("child listener survived")
	}
	if strings.Contains(diagnostic.String(), "invalid") {
		t.Fatal(diagnostic.String())
	}
}
