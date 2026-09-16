//go:build linux

package notify

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf16"

	controlapi "github.com/SLktEx/Hacocoon/internal/controller/api"
	control "github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	eventsapp "github.com/SLktEx/Hacocoon/internal/events"
)

type notifyCommandController struct {
	sync.Mutex
	fail    bool
	offsets []int64
}

func notifyCommandFixture(t *testing.T) (*notifyCommandController, string, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("HACO_UI_LANGUAGE", "en")
	t.Setenv("WSL_DISTRO_NAME", "")
	t.Setenv("HACO_CLIENT_MODE", "controller")
	t.Setenv("HACO_ROOT", filepath.Join(home, "unused-private-audit"))
	bin := filepath.Join(home, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	capture := filepath.Join(home, "arguments")
	t.Setenv("NOTIFY_COMMAND_ARGUMENTS", capture)
	if err := os.WriteFile(filepath.Join(bin, "notify-send"), []byte("#!/bin/sh\nprintf '%s\\0' \"$@\" >> \"$NOTIFY_COMMAND_ARGUMENTS\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	state := &notifyCommandController{}
	server := control.NewServer()
	err := server.RegisterStream(controlapi.MethodEventsStream, func(_ context.Context, raw json.RawMessage) (control.Stream, error) {
		var request controlapi.EventsStreamRequest
		if err := json.Unmarshal(raw, &request); err != nil {
			return nil, err
		}
		state.Lock()
		state.offsets = append(state.offsets, request.SinceOffset)
		failed := state.fail
		state.Unlock()
		if failed {
			return nil, control.NewStatusError("recovery_required", "PRIVATE_CONTROLLER_DETAIL")
		}
		return func(_ context.Context, conn net.Conn) error {
			encoder := json.NewEncoder(conn)
			event := eventsapp.Event{Type: "policy-decision", Decision: core.PolicyRequireApproval, RequestID: "0123456789abcdef0123456789abcdef", Environment: "dev", Capability: "git.push", Action: "push", NextOffset: 10, Resource: "PRIVATE_RESOURCE", Attributes: map[string]string{"credential": "PRIVATE_ATTRIBUTE"}, Reason: "PRIVATE_REASON"}
			if request.SinceOffset < 10 {
				if err := encoder.Encode(map[string]any{"event": event, "next_offset": int64(10)}); err != nil {
					return err
				}
			}
			return encoder.Encode(map[string]any{"done": true, "next_offset": int64(10)})
		}, nil
	})
	if err != nil {
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
	t.Cleanup(func() { cancel(); <-done })
	t.Setenv("HACO_CONTROL_SOCKET", socket)
	return state, home, capture
}

func TestNativeCommandUsesControllerAndPersistsDelivery(t *testing.T) {
	state, _, capture := notifyCommandFixture(t)
	original := os.Args
	os.Args = []string{"haco-notify", "native", "--once"}
	defer func() { os.Args = original }()
	Main()
	first, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	want := "--app-name=Hacocoon\x00Hacocoon approval required\x00dev · git.push · push\x00"
	if string(first) != want {
		t.Fatal("notification exposed private fields or changed argv", string(first))
	}
	if err := dispatch(context.Background(), []string{"native", "--once", "--backend", "linux"}); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(capture)
	if err != nil || string(second) != want {
		t.Fatal("saved event was replayed", string(second), err)
	}
	saved, err := loadState(defaultStatePath())
	if err != nil || saved.Offset != 10 || len(saved.SeenEventIDs) != 1 {
		t.Fatal("delivery cursor lost", saved, err)
	}
	state.Lock()
	defer state.Unlock()
	if len(state.offsets) != 2 || state.offsets[0] != 0 || state.offsets[1] != 10 {
		t.Fatal("CLI did not resume controller events", state.offsets)
	}
}

func TestNativeCommandFailureRetainsCursorForRetry(t *testing.T) {
	for _, mode := range []string{"delivery", "controller", "from-now"} {
		t.Run(mode, func(t *testing.T) {
			state, home, capture := notifyCommandFixture(t)
			path := filepath.Join(home, "saved", "notify.json")
			args := []string{"native", "--once", "--state", path}
			if mode == "from-now" {
				args = append(args, "--from-now")
				if err := dispatch(context.Background(), args); err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(capture); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("from-now replayed history", err)
				}
				saved, err := loadState(path)
				if err != nil || saved.Offset != 10 || len(saved.SeenEventIDs) != 0 {
					t.Fatal("from-now cursor lost", saved, err)
				}
				return
			}
			if err := saveState(path, notifyState{Offset: 0}); err != nil {
				t.Fatal(err)
			}
			if mode == "controller" {
				state.Lock()
				state.fail = true
				state.Unlock()
			} else if err := os.WriteFile(filepath.Join(home, "bin", "notify-send"), []byte("#!/bin/sh\necho PRIVATE_SUBPROCESS_DETAIL >&2\nexit 7\n"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := dispatch(context.Background(), args); err == nil || strings.Contains(err.Error(), "PRIVATE") {
				t.Fatal("private failure exposed or reported successful", err)
			}
			saved, err := loadState(path)
			if err != nil || saved.Offset != 0 || len(saved.SeenEventIDs) != 0 {
				t.Fatal("failed delivery advanced cursor", saved, err)
			}
			state.Lock()
			state.fail = false
			state.Unlock()
			if err := os.WriteFile(filepath.Join(home, "bin", "notify-send"), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := dispatch(context.Background(), args); err != nil {
				t.Fatal("retry failed", err)
			}
			saved, err = loadState(path)
			if err != nil || saved.Offset != 10 || len(saved.SeenEventIDs) != 1 {
				t.Fatal("retry did not commit delivery", saved, err)
			}
		})
	}
}

func TestNotifyCommandRejectsInvalidInvocationBeforeSubscription(t *testing.T) {
	state, _, _ := notifyCommandFixture(t)
	for _, args := range [][]string{
		nil, {"unknown"}, {"native", "--bad"}, {"native", "extra"}, {"native", "--poll", "1ms"}, {"native", "--backend", "unknown"},
		{"web", "--bad"}, {"web", "extra"}, {"web", "--listen", "0.0.0.0:1"},
	} {
		if err := dispatch(context.Background(), args); err == nil {
			t.Fatal("invalid invocation accepted", args)
		}
	}
	t.Setenv("HACO_CLIENT_MODE", "invalid")
	for _, args := range [][]string{{"web"}, {"native", "--once"}} {
		if err := dispatch(context.Background(), args); err == nil {
			t.Fatal("invalid reader configuration accepted")
		}
	}
	state.Lock()
	defer state.Unlock()
	if len(state.offsets) != 0 {
		t.Fatal("invalid invocation subscribed to events", state.offsets)
	}
}

func TestNativeBackendSelectionAndIsolatedWindowsArguments(t *testing.T) {
	for _, mode := range []string{"linux", "windows", "auto-windows", "auto-linux-fallback", "missing-windows", "missing-linux", "missing-auto"} {
		t.Run(mode, func(t *testing.T) {
			_, home, capture := notifyCommandFixture(t)
			bin := filepath.Join(home, "bin")
			t.Setenv("WSL_DISTRO_NAME", "Hacocoon-Test")
			backend := "auto"
			switch mode {
			case "linux", "missing-linux":
				backend = "linux"
			case "windows", "missing-windows":
				backend = "windows"
			}
			if strings.HasPrefix(mode, "missing-") {
				if err := os.Remove(filepath.Join(bin, "notify-send")); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "windows" || mode == "auto-windows" {
				if err := os.WriteFile(filepath.Join(bin, "powershell.exe"), []byte("#!/bin/sh\nprintf '%s\\0' \"$@\" >> \"$NOTIFY_COMMAND_ARGUMENTS\"\n"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			presenter, err := chooseNotifier(backend)
			if strings.HasPrefix(mode, "missing-") {
				if err == nil || presenter != nil {
					t.Fatal("missing backend selected", presenter, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			title, body := "quoted ' title", "body <xml> \" ; $secret"
			if err := presenter.Notify(context.Background(), title, body); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(capture)
			if err != nil {
				t.Fatal(err)
			}
			args := strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00")
			if mode == "windows" || mode == "auto-windows" {
				if len(args) != 4 || strings.Join(args[:3], " ") != "-NoProfile -NonInteractive -EncodedCommand" {
					t.Fatal("Windows invocation changed", args)
				}
				raw, err := base64.StdEncoding.DecodeString(args[3])
				if err != nil || len(raw)%2 != 0 {
					t.Fatal("invalid PowerShell encoding", err)
				}
				units := make([]uint16, len(raw)/2)
				for i := range units {
					units[i] = binary.LittleEndian.Uint16(raw[i*2:])
				}
				script := string(utf16.Decode(units))
				if strings.Contains(script, title) || strings.Contains(script, body) || !strings.Contains(script, base64.StdEncoding.EncodeToString([]byte(body))) {
					t.Fatal("native text was interpolated or lost")
				}
				reviewer, ok := presenter.(interface {
					NotifyReview(context.Context, string, string, string) error
				})
				if !ok {
					t.Fatal("Windows review route missing")
				}
				if err := reviewer.NotifyReview(context.Background(), title, body, "invalid?id"); err == nil {
					t.Fatal("invalid review request launched")
				}
				unchanged, err := os.ReadFile(capture)
				if err != nil || string(unchanged) != string(data) {
					t.Fatal("invalid review invoked desktop", err)
				}
				if err := reviewer.NotifyReview(context.Background(), title, body, "0123456789abcdef0123456789abcdef"); err != nil {
					t.Fatal(err)
				}
			} else if len(args) != 3 || args[0] != "--app-name=Hacocoon" || args[1] != title || args[2] != body {
				t.Fatal("Linux notification changed literal arguments", args)
			}
		})
	}
}

func TestWebCommandDoesNotAnnounceFailedListener(t *testing.T) {
	notifyCommandFixture(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	output, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = output.Close() }()
	original := os.Stdout
	os.Stdout = output
	defer func() { os.Stdout = original }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := dispatch(ctx, []string{"web", "--listen", listener.Addr().String()}); err == nil {
		t.Fatal("occupied listener reported success")
	}
	data, err := os.ReadFile(output.Name())
	if err != nil || len(data) != 0 {
		t.Fatal("failed listener announced a browser endpoint", string(data), err)
	}
}

func TestWebCommandPublishesBoundEndpointAndStops(t *testing.T) {
	notifyCommandFixture(t)
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = read.Close(); _ = write.Close() }()
	if err := read.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = write
	defer func() { os.Stdout = original }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- dispatch(ctx, []string{"web", "--listen", "127.0.0.1:0"}) }()
	line, err := bufio.NewReader(read).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	endpoint := strings.TrimSuffix(strings.TrimPrefix(line, "Hacocoon browser notifications: "), "\n")
	if !strings.HasPrefix(endpoint, "http://127.0.0.1:") || strings.HasSuffix(endpoint, ":0/") {
		t.Fatal("endpoint was not bound", endpoint)
	}
	client := &http.Client{Timeout: 5 * time.Second}
	defer client.CloseIdleConnections()
	response, err := client.Get(endpoint + "api/v1/events?offset=0&limit=1")
	if err != nil {
		t.Fatal(err)
	}
	data, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil || response.StatusCode != http.StatusOK || !strings.Contains(string(data), "approval-required") || strings.Contains(string(data), "PRIVATE") {
		t.Fatal("browser projection failed", string(data), readErr)
	}
	if response.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("private events became cacheable")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("notification server did not shut down")
	}
}

func TestWebCommandFailsWhenBoundEndpointCannotBeReported(t *testing.T) {
	notifyCommandFixture(t)
	closed, err := os.CreateTemp(t.TempDir(), "closed-output")
	if err != nil {
		t.Fatal(err)
	}
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = closed
	defer func() { os.Stdout = original }()
	if err := dispatch(context.Background(), []string{"web", "--listen", "127.0.0.1:0"}); err == nil {
		t.Fatal("undiscoverable endpoint started successfully")
	}
}

func TestNativeCommandReportsOnlyAllowedFailureDetails(t *testing.T) {
	for _, mode := range []string{"native-stage", "missing-file", "missing-command"} {
		t.Run(mode, func(t *testing.T) {
			_, home, _ := notifyCommandFixture(t)
			program := filepath.Join(home, "PRIVATE_EXECUTABLE")
			want := "native notification command could not start (system error 2)"
			switch mode {
			case "native-stage":
				if err := os.WriteFile(program, []byte("#!/bin/sh\necho PRIVATE_DIAGNOSTIC >&2\necho HACO_NATIVE_FAILURE:show:-123 >&2\nexit 7\n"), 0700); err != nil {
					t.Fatal(err)
				}
				want = "native notification failed at show (code -123)"
			case "missing-command":
				program = "PRIVATE_UNKNOWN_COMMAND"
				want = "native notification command could not start"
			}
			presenter := commandNotifier{command: func(ctx context.Context, _, _ string) *exec.Cmd { return exec.CommandContext(ctx, program) }}
			if err := presenter.Notify(context.Background(), "title", "body"); err == nil || err.Error() != want {
				t.Fatal("native failure exposed private detail or lost safe category", err)
			}
		})
	}
}
