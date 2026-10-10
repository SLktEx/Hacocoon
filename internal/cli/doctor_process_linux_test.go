//go:build linux

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/buildinfo"
	controlapi "github.com/SLktEx/Hacocoon/internal/controller/api"
	control "github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/diagnostics"
)

// The shipped entrypoint, signal handling, RPC client, output streams and OS
// exit status are real. Only the controller responses are fixtures: this is
// process-composition coverage, not installed or unhealthy-Host acceptance.
func TestDoctorProcess(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "haco")
	buildCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	build := exec.CommandContext(buildCtx, "go", "build", "-o", bin, "../../cmd/haco")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build haco: %v\n%s", err, out)
	}

	for _, state := range []string{diagnostics.OK, diagnostics.Failed, diagnostics.Skipped, diagnostics.Pending} {
		t.Run(state, func(t *testing.T) { testDoctorProcessReport(t, bin, state) })
	}
	for _, name := range []string{"partial-report", "malformed-frame", "truncated-frame", "control-summary", "control-action", "control-build", "peer-error"} {
		t.Run(name, func(t *testing.T) { testDoctorProcessInvalidResponse(t, bin, name) })
	}
	for _, signal := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
		t.Run(signal.String(), func(t *testing.T) { testDoctorProcessCancellation(t, bin, signal) })
	}
	for _, endpoint := range []string{"missing-socket", "refused-socket"} {
		t.Run(endpoint, func(t *testing.T) {
			// Both cases retain the real two-minute readiness window. Run these
			// private fixtures together instead of altering product deadlines.
			t.Parallel()
			testDoctorProcessUnavailableEndpoint(t, bin, endpoint)
		})
	}
}

func testDoctorProcessReport(t *testing.T, bin, state string) {
	t.Helper()
	want := doctorProcessReport()
	switch state {
	case diagnostics.Failed:
		want.Checks[1].Status = diagnostics.Failed
		want.Checks[1].Summary = "Configured storage differs"
		want.Checks[1].Action = "Inspect the configured Incus pool"
		want.Checks[2].Status = diagnostics.Skipped
		want.Checks[2].Summary = "Configuration unavailable"
		want.Checks[2].Action = "Resolve storage configuration first"
	case diagnostics.Skipped:
		want.Checks[5].Status = diagnostics.Skipped
		want.Checks[5].Summary = "Connectivity check unavailable"
		want.Checks[5].Action = "Inspect trusted Host connectivity"
	case diagnostics.Pending:
		want.Checks[2].Status = diagnostics.Pending
		want.Checks[2].Summary = "Live mount policy differs"
		want.Checks[2].Action = "Arrange Incus-owned maintenance"
	}
	socket, requested := doctorProcessPeer(t, doctorProcessReply(doctorProcessFrame(t, want)))
	cmd, stdout, stderr := doctorProcessCommand(t, bin, socket, 10*time.Second)
	err := cmd.Run()
	wantCode := 0
	if state != diagnostics.OK {
		wantCode = 1
	}
	assertDoctorProcessExit(t, err, wantCode)
	assertDoctorProcessRequested(t, requested)
	var got controlapi.DoctorResponse
	// Unmarshal also rejects extra non-JSON stdout and multiple results.
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not one JSON report: %v\n%q", err, stdout.String())
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("report or next action changed: got %+v, want %+v", got, want)
	}
	if wantCode == 0 {
		assertDoctorProcessDiagnostic(t, stderr, "")
	} else {
		assertDoctorProcessDiagnostic(t, stderr, "Host diagnostic checks did not pass; see the reported checks")
	}
}

func testDoctorProcessInvalidResponse(t *testing.T, bin, name string) {
	t.Helper()
	const marker = "doctor-peer-must-not-echo"
	const invalid = "Physical Host controller returned invalid or incompatible diagnostics"
	const incomplete = "Physical Host controller could not provide diagnostics; check the current installation"
	report := doctorProcessReport()
	report.Checks[0].Summary = marker
	message := invalid
	var frame []byte
	switch name {
	case "partial-report":
		report.Checks = report.Checks[:len(report.Checks)-1]
	case "malformed-frame":
		frame = []byte(`{"version":1,"payload":` + marker + "}\n")
	case "truncated-frame":
		frame = []byte(`{"version":1,"payload":{"checks":"` + marker)
		message = incomplete // EOF before a complete frame, not a decoded report.
	case "control-summary":
		report.Checks[0].Summary += "\x1b[2J"
	case "control-action":
		report.Checks[5].Status = diagnostics.Failed
		report.Checks[5].Action = marker + "\x1b[2J"
	case "control-build":
		report.Controller.Version = marker + "\x1b[2J"
	case "peer-error":
		frame = []byte(`{"version":1,"error":{"code":"diagnostics_failed","message":"` + marker + `"}}` + "\n")
		message = incomplete
	}
	if frame == nil {
		frame = doctorProcessFrame(t, report)
	}
	socket, requested := doctorProcessPeer(t, doctorProcessReply(frame))
	cmd, stdout, stderr := doctorProcessCommand(t, bin, socket, 10*time.Second)
	assertDoctorProcessExit(t, cmd.Run(), 1)
	assertDoctorProcessRequested(t, requested)
	if stdout.Len() != 0 {
		t.Fatalf("invalid response produced stdout: %q", stdout.String())
	}
	assertDoctorProcessDiagnostic(t, stderr, message)
	if strings.Contains(stderr.String(), marker) || strings.ContainsRune(stderr.String(), '\x1b') {
		t.Fatalf("peer text leaked into diagnostics: %q", stderr.String())
	}
}

func testDoctorProcessCancellation(t *testing.T, bin string, signal os.Signal) {
	t.Helper()
	disconnected := make(chan struct{})
	socket, requested := doctorProcessPeer(t, func(conn net.Conn) error {
		var b [1]byte
		n, err := conn.Read(b[:])
		if n != 0 || !errors.Is(err, io.EOF) {
			return fmt.Errorf("expected client connection closure, got %d bytes, %v", n, err)
		}
		close(disconnected)
		return nil
	})
	cmd, stdout, stderr := doctorProcessCommand(t, bin, socket, 10*time.Second)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// Always reap this exact child, including a failed request-start wait.
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var waitErr error
	waited := false
	t.Cleanup(func() {
		if !waited {
			_ = cmd.Process.Kill()
			<-done
		}
	})
	select {
	case <-requested:
	case waitErr = <-done:
		waited = true
		t.Fatalf("doctor exited before its diagnostic request: %v; stderr=%q", waitErr, stderr.String())
	case <-time.After(5 * time.Second):
		t.Fatal("doctor did not send its diagnostic request")
	}
	// Request receipt proves the executable installed signal handling and
	// reached Doctor, rather than canceling a readiness probe or sleeping.
	if err := cmd.Process.Signal(signal); err != nil {
		t.Fatal(err)
	}
	select {
	case waitErr = <-done:
		waited = true
	case <-time.After(5 * time.Second):
		t.Fatal("doctor did not exit promptly after cancellation")
	}
	assertDoctorProcessExit(t, waitErr, 1)
	if stdout.Len() != 0 {
		t.Fatalf("canceled doctor produced stdout: %q", stdout.String())
	}
	assertDoctorProcessDiagnostic(t, stderr, "Host diagnostics timed out or were canceled")
	select {
	case <-disconnected:
	case <-time.After(5 * time.Second):
		t.Fatal("doctor did not close its diagnostic connection")
	}
}

func testDoctorProcessUnavailableEndpoint(t *testing.T, bin, endpoint string) {
	t.Helper()
	socket, checkEndpoint := doctorProcessUnavailableEndpoint(t, endpoint)
	cmd, stdout, stderr := doctorProcessCommand(t, bin, socket, controllerStartupTimeout+10*time.Second)
	assertDoctorProcessExit(t, cmd.Run(), 1)
	if stdout.Len() != 0 {
		t.Fatalf("unavailable controller produced stdout: %q", stdout.String())
	}
	// Repeated unavailable Ping ends at the existing readiness deadline;
	// the command reports that fixed timeout without exposing the path.
	assertDoctorProcessDiagnostic(t, stderr, "Host diagnostics timed out or were canceled")
	if strings.Contains(stderr.String(), socket) {
		t.Fatalf("selected socket path leaked into diagnostics: %q", stderr.String())
	}
	checkEndpoint()
}

func doctorProcessUnavailableEndpoint(t *testing.T, endpoint string) (string, func()) {
	t.Helper()
	socket := filepath.Join(t.TempDir(), "control.sock")
	if endpoint == "refused-socket" {
		return doctorProcessRefusedEndpoint(t, socket)
	}
	checkEndpoint := func() {
		t.Helper()
		if _, err := os.Lstat(socket); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("missing selected socket was created: %v", err)
		}
	}
	checkEndpoint()
	assertDoctorProcessDialError(t, socket, syscall.ENOENT)
	return socket, checkEndpoint
}

func doctorProcessRefusedEndpoint(t *testing.T, socket string) (string, func()) {
	t.Helper()
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		t.Fatalf("create owned refusal fixture: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	listener.SetUnlinkOnClose(false)
	if err := os.Chmod(socket, 0600); err != nil {
		t.Fatal(err)
	}
	owned, err := os.Lstat(socket)
	if err != nil || owned.Mode()&os.ModeSocket == 0 {
		t.Fatalf("refusal fixture is not an owned socket: %v", err)
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	checkEndpoint := func() {
		t.Helper()
		current, err := os.Lstat(socket)
		if err != nil || current.Mode()&os.ModeSocket == 0 || !os.SameFile(owned, current) {
			t.Fatalf("owned refused socket changed: %v", err)
		}
	}
	checkEndpoint()
	assertDoctorProcessDialError(t, socket, syscall.ECONNREFUSED)
	return socket, checkEndpoint
}

func assertDoctorProcessDialError(t *testing.T, socket string, want syscall.Errno) {
	t.Helper()
	// A generic dialing failure is not evidence of ENOENT/ECONNREFUSED.
	// In particular, restricted runners must fail explicitly on EPERM.
	conn, err := net.DialTimeout("unix", socket, time.Second)
	if conn != nil {
		_ = conn.Close()
		t.Fatal("unavailable fixture unexpectedly accepted a connection")
	}
	if !errors.Is(err, want) {
		t.Fatalf("endpoint preflight: want %v, got %v", want, err)
	}
}

func doctorProcessReport() controlapi.DoctorResponse {
	report := controlapi.DoctorResponse{ProtocolVersion: control.ProtocolVersion, Controller: buildinfo.Current()}
	for _, name := range diagnostics.CheckNames() {
		report.Checks = append(report.Checks, diagnostics.Check{Name: name, Status: diagnostics.OK, Summary: "Verified predicate"})
	}
	return report
}

func doctorProcessFrame(t *testing.T, payload any) []byte {
	t.Helper()
	frame, err := json.Marshal(struct {
		Version int `json:"version"`
		Payload any `json:"payload"`
	}{control.ProtocolVersion, payload})
	if err != nil {
		t.Fatal(err)
	}
	return append(frame, '\n')
}

func doctorProcessReply(frame []byte) func(net.Conn) error {
	return func(conn net.Conn) error {
		_, err := conn.Write(frame)
		return err
	}
}

// A private raw peer can supply incomplete/malformed frames that the normal
// controller encoder cannot produce. It accepts only read-only Ping then Doctor,
// with no target/repair payload; no controller service or provider is started.
func doctorProcessPeer(t *testing.T, reply func(net.Conn) error) (string, <-chan struct{}) {
	t.Helper()
	socket := filepath.Join(t.TempDir(), "control.sock")
	listener, err := control.ListenUnix(socket, 0600)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	requested := make(chan struct{})
	done := make(chan struct{})
	var serveErr error
	ping := doctorProcessFrame(t, controlapi.PingResponse{ProtocolVersion: control.ProtocolVersion})
	go func() {
		defer close(done)
		defer func() { _ = listener.Close() }()
		for _, method := range []string{controlapi.MethodPing, controlapi.MethodDoctor} {
			conn, err := listener.Accept()
			if err != nil {
				serveErr = err
				return
			}
			serveErr = handleDoctorProcessRequest(ctx, conn, method, ping, requested, reply)
			if serveErr != nil {
				return
			}
		}
	}()
	t.Cleanup(func() {
		cancel()
		_ = listener.Close()
		<-done
		if serveErr != nil && !errors.Is(serveErr, net.ErrClosed) {
			t.Errorf("doctor peer: %v", serveErr)
		}
	})
	return socket, requested
}

func handleDoctorProcessRequest(ctx context.Context, conn net.Conn, method string, ping []byte, requested chan<- struct{}, reply func(net.Conn) error) error {
	defer func() { _ = conn.Close() }()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	var request struct {
		Version int             `json:"version"`
		Method  string          `json:"method"`
		Stream  bool            `json:"stream"`
		Session bool            `json:"session"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.NewDecoder(io.LimitReader(conn, 4096)).Decode(&request); err != nil {
		return err
	}
	payload := strings.TrimSpace(string(request.Payload))
	if request.Version != control.ProtocolVersion || request.Method != method || request.Stream || request.Session || (payload != "" && payload != "null" && payload != "{}") {
		return fmt.Errorf("unexpected controller request: %+v", request)
	}
	if method == controlapi.MethodPing {
		return doctorProcessReply(ping)(conn)
	}
	close(requested)
	return reply(conn)
}

func doctorProcessCommand(t *testing.T, bin, socket string, watchdog time.Duration) (*exec.Cmd, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	productRoot := filepath.Join(root, "must-not-create")
	ctx, cancel := context.WithTimeout(context.Background(), watchdog)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, bin, "doctor", "--json")
	cmd.Dir = root
	cmd.WaitDelay = time.Second
	// Do not inherit credentials, product settings or executable fallbacks.
	cmd.Env = []string{
		"HOME=" + home, "HACO_ROOT=" + productRoot, "HACO_CONTROL_SOCKET=" + socket,
		"PATH=" + filepath.Join(root, "no-executables"), "HACO_UI_LANGUAGE=en",
		"HACO_LOG_FORMAT=json", "HACO_LOG_LEVEL=debug",
	}
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	t.Cleanup(func() {
		if _, err := os.Lstat(productRoot); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("doctor constructed local product state: %v", err)
		}
		entries, err := os.ReadDir(home)
		if err != nil || len(entries) != 0 {
			t.Errorf("doctor changed private HOME: entries=%v, error=%v", entries, err)
		}
	})
	return cmd, stdout, stderr
}

func assertDoctorProcessExit(t *testing.T, err error, want int) {
	t.Helper()
	if want == 0 && err == nil {
		return
	}
	var exit *exec.ExitError
	if want != 0 && errors.As(err, &exit) && exit.ExitCode() == want {
		return
	}
	t.Fatalf("doctor process error = %v, want OS exit %d", err, want)
}

func assertDoctorProcessRequested(t *testing.T, requested <-chan struct{}) {
	t.Helper()
	select {
	case <-requested:
	default:
		t.Fatal("doctor never sent its diagnostic request")
	}
}

func assertDoctorProcessDiagnostic(t *testing.T, stderr *bytes.Buffer, want string) {
	t.Helper()
	errorsSeen := 0
	decoder := json.NewDecoder(bytes.NewReader(stderr.Bytes()))
	for {
		var record map[string]any
		err := decoder.Decode(&record)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("stderr is not structured diagnostics: %v\n%q", err, stderr.String())
		}
		// Safe DEBUG/INFO/WARN observations may accompany the one failure
		// boundary. Do not couple this process contract to their presence.
		switch record["level"] {
		case "DEBUG", "INFO", "WARN":
			continue
		case "ERROR":
			errorsSeen++
			if record["component"] != "cli" || record["operation"] != "doctor" || record["msg"] != want {
				t.Fatalf("unexpected doctor diagnostic: %v", record)
			}
		default:
			t.Fatalf("unrecognized structured diagnostic: %v", record)
		}
	}
	wantErrors := 0
	if want != "" {
		wantErrors = 1
	}
	if errorsSeen != wantErrors {
		t.Fatalf("doctor logged %d failures, want %d: %q", errorsSeen, wantErrors, stderr.String())
	}
}
