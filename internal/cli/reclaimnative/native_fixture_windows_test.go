//go:build windows && (amd64 || arm64)

package reclaimnative

// SOURCE-ONLY verification fixture. Do not infer a native pass from compilation.
// Deferred execution requires explicit approval, a disposable native Windows
// amd64/arm64 machine, and HACO_TEST_NATIVE_RECLAIM_COMPONENTS=1. Run only the
// four TestNativeFrontend* entry points below, with -failfast, a bounded test-runner
// deadline and mandatory no-skip result checks. Required API/Job/console failures
// fail the opted-in run. No installed helper, WSL, enrollment, registry, disk,
// credentials, persistent settings or production continuation is a test target.
//
// Each scenario driver is the exact current test executable in a creation-time
// private test Job. Every synthetic child and inherited writer belongs to that
// Job, including uncertain production start() errors. The outer test always
// checks Close's descendant-absence result. A child's watchdog is only a second
// bound, never proof of ownership or absence. Do not replace this outer Job with
// PID reopening or assign the real test runner/user process to a Job.

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/SLktEx/Hacocoon/internal/platform/winprocess"
	"golang.org/x/sys/windows"
)

const nativeFixtureMagic = "haco-reclaim-synthetic-native-v1"
const nativeFixtureOptIn = "HACO_TEST_NATIVE_RECLAIM_COMPONENTS"
const fixtureHold = 8 * time.Second

func TestNativeFrontendTransport(t *testing.T) {
	requireNativeFixture(t)
	for _, scenario := range []string{"no-data-cancel", "partial", "oversized", "eof-live-root", "root-exit-writer", "nonzero-exit", "pre-canceled-result"} {
		if !t.Run(scenario, func(t *testing.T) { runNativeFixture(t, scenario) }) {
			return
		}
	}
}
func TestNativeFrontendHandleInheritance(t *testing.T) {
	requireNativeFixture(t)
	runNativeFixture(t, "handles")
}
func TestNativeFrontendConsole(t *testing.T) {
	requireNativeFixture(t)
	for _, scenario := range []string{"console", "redirected"} {
		if !t.Run(scenario, func(t *testing.T) { runNativeFixture(t, scenario) }) {
			return
		}
	}
}
func requireNativeFixture(t *testing.T) {
	t.Helper()
	if os.Getenv(nativeFixtureOptIn) != "1" {
		t.Skip("synthetic native component execution requires explicit opt-in")
	}
	if failfast := flag.Lookup("test.failfast"); failfast == nil || failfast.Value.String() != "true" {
		t.Fatal("synthetic native verification requires -failfast to stop on uncertain cleanup")
	}
}
func fixtureArguments(kind, name string, extra ...string) []string {
	return append([]string{"-test.run=^TestNativeReclaimFixtureProcess$", "--", nativeFixtureMagic, kind, name}, extra...)
}

func runNativeFixture(t *testing.T, scenario string) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal("test executable unavailable")
	}
	system, err := windows.GetSystemDirectory()
	if err != nil {
		t.Fatal("system directory unavailable")
	}
	root, err := windows.GetWindowsDirectory()
	if err != nil {
		t.Fatal("Windows directory unavailable")
	}
	driver, err := winprocess.Start(executable, fixtureArguments("scenario", scenario), []string{"SystemRoot=" + root, "WINDIR=" + root}, system)
	if err != nil {
		t.Fatal("creation-time fixture ownership unavailable")
	}
	defer func() {
		if err := driver.Close(); err != nil {
			t.Error("synthetic fixture descendant cleanup was not confirmed")
		}
	}()
	// Establish nonblocking observation while the driver still holds its writer
	// and waits at the fixed private-stdin gate. Mode setup failure cannot dispatch
	// a scenario, and the creation-time Job still owns the waiting driver.
	mode := uint32(windows.PIPE_READMODE_BYTE | windows.PIPE_NOWAIT)
	if err := windows.SetNamedPipeHandleState(windows.Handle(driver.Output.Fd()), &mode, nil, nil); err != nil {
		t.Fatal("fixture nonblocking output unavailable")
	}
	if n, err := driver.Input.Write([]byte{1}); err != nil || n != 1 {
		t.Fatal("fixture dispatch gate failed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	waitErr := driver.Wait(ctx)
	// A failed exit may still carry one fixed diagnostic stage. Reading it never
	// converts the failed exit into success or changes the owned-Job cleanup.
	raw, err := readFixtureResult(ctx, driver.Output)
	if err != nil {
		t.Fatal("synthetic native scenario receipt was not confirmed")
	}
	if err := decodeFixtureResult(raw, waitErr); err != nil {
		t.Fatal(err) // Decoder errors contain fixed allowlisted text only.
	}
}

// Do not use an unbounded reader even for fixture diagnostics. Driver stdout is
// exactly OK\n, FAIL\n or an allowlisted failure stage, never arbitrary child
// output, underlying errors, paths or environment contents.
func readFixtureResult(ctx context.Context, file *os.File) ([]byte, error) {
	p := nativeProcess{output: windows.Handle(file.Fd())}
	var raw []byte
	var buffer [fixtureReceiptLimit + 1]byte
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		n, eof, err := p.read(buffer[:])
		if err != nil {
			return nil, err
		}
		if n > fixtureReceiptLimit-len(raw) {
			return nil, errors.New("oversized fixture receipt")
		}
		raw = append(raw, buffer[:n]...)
		if eof {
			return raw, nil
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// AllocConsole initializes standard handles. Keep an exact, noninheritable
// duplicate of the driver result pipe before any scenario changes its console.
// This duplicates no process ownership and is closed before successful exit.
func runFixtureScenario(name string) int {
	source := windows.Handle(os.Stdout.Fd())
	if kind, err := windows.GetFileType(source); err != nil || kind != windows.FILE_TYPE_PIPE {
		return 1
	}
	var held windows.Handle
	if err := windows.DuplicateHandle(windows.CurrentProcess(), source, windows.CurrentProcess(), &held, 0, false, windows.DUPLICATE_SAME_ACCESS); err != nil {
		return 1
	}
	output := os.NewFile(uintptr(held), "synthetic-result-pipe")
	if output == nil {
		_ = windows.CloseHandle(held)
		return 1
	}
	scenarioErr := nativeFixtureScenario(name)
	_, writeErr := io.WriteString(output, encodeFixtureResult(scenarioErr))
	closeErr := output.Close()
	if scenarioErr != nil || writeErr != nil || closeErr != nil {
		return 1
	}
	return 0
}

// Only fixed self-test modes are accepted. No arbitrary program, command, path,
// distribution or operation identity is ever executed by this fixture process.
func TestNativeReclaimFixtureProcess(t *testing.T) {
	args := flag.Args()
	if len(args) < 3 || args[0] != nativeFixtureMagic {
		t.Skip("fixed synthetic child entry only")
	}
	watchdog := time.AfterFunc(10*time.Second, func() { os.Exit(125) })
	defer watchdog.Stop()
	switch args[1] {
	case "scenario":
		if len(args) != 3 {
			os.Exit(2)
		}
		var gate [1]byte
		if _, err := io.ReadFull(os.Stdin, gate[:]); err != nil || gate[0] != 1 {
			os.Exit(2)
		}
		os.Exit(runFixtureScenario(args[2]))
	case "child":
		os.Exit(nativeFixtureChild(args[2], args[3:]))
	default:
		os.Exit(2)
	}
}

func withSyntheticChild(ctx context.Context, mode string, extra []string, visit func(*nativeProcess, windows.Handle) error) (err error) {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	h := installedHelper{executable: executable}
	p, err := h.start(ctx, fixtureArguments("child", mode, extra...))
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, p.close()) }()
	// This is an exact duplicate of the original kernel process handle. It is
	// retained solely within this bounded fixture scope, never reopened by PID.
	var held windows.Handle
	if err = windows.DuplicateHandle(windows.CurrentProcess(), p.process, windows.CurrentProcess(), &held, 0, false, windows.DUPLICATE_SAME_ACCESS); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, windows.CloseHandle(held)) }()
	// The outer creation-time fixture Job owns/reaps children left running here.
	return visit(p, held)
}
func requireSyntheticAlive(held windows.Handle) error {
	state, err := windows.WaitForSingleObject(held, 0)
	if err != nil {
		return err
	}
	if state != uint32(windows.WAIT_TIMEOUT) {
		return errors.New("observer terminated synthetic child")
	}
	return nil
}
func requireSyntheticExit(held windows.Handle, code uint32) error {
	state, err := windows.WaitForSingleObject(held, 0)
	if err != nil {
		return err
	}
	if state != windows.WAIT_OBJECT_0 {
		return errors.New("synthetic child exit unconfirmed")
	}
	var actual uint32
	if err := windows.GetExitCodeProcess(held, &actual); err != nil {
		return err
	}
	if actual != code {
		return errors.New("unexpected synthetic exit")
	}
	return nil
}
func shortObservation(parent context.Context, p *nativeProcess) error {
	ctx, cancel := context.WithTimeout(parent, 100*time.Millisecond)
	defer cancel()
	_, err := observe(ctx, p)
	if !errors.Is(err, context.DeadlineExceeded) {
		return errors.New("uncertainty was not retained to the observation bound")
	}
	if p.process != 0 || p.output != 0 {
		return errors.New("observer handles were not released")
	}
	return nil
}
func awaitSynthetic(ctx context.Context, check func() (bool, error)) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		done, err := check()
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func nativeFixtureScenario(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	switch name {
	case "no-data-cancel":
		return withSyntheticChild(ctx, "idle", nil, func(p *nativeProcess, held windows.Handle) error {
			var data [16]byte
			n, eof, err := p.read(data[:])
			if err != nil || n != 0 || eof {
				return errors.New("empty PIPE_NOWAIT read did not return no-data")
			}
			canceled, stop := context.WithCancel(ctx)
			stop()
			_, err = observe(canceled, p)
			if !errors.Is(err, context.Canceled) || p.output != 0 || p.process != 0 {
				return errors.New("cancellation did not release observation")
			}
			return requireSyntheticAlive(held)
		})
	case "partial":
		return partialNativeFixture(ctx)
	case "oversized":
		return withSyntheticChild(ctx, "oversized", nil, func(p *nativeProcess, held windows.Handle) error {
			_, err := observe(ctx, p)
			if err == nil || !strings.Contains(err.Error(), "helper output exceeds receipt bound") {
				return errors.New("oversized pipe output did not reach its bound refusal")
			}
			return requireSyntheticAlive(held)
		})
	case "eof-live-root":
		return withSyntheticChild(ctx, "close-output", nil, func(p *nativeProcess, held windows.Handle) error {
			if err := awaitSynthetic(ctx, func() (bool, error) {
				var data [1]byte
				n, eof, err := p.read(data[:])
				if n != 0 {
					return false, errors.New("unexpected EOF fixture data")
				}
				return eof, err
			}); err != nil {
				return err
			}
			if err := requireSyntheticAlive(held); err != nil {
				return err
			}
			if err := shortObservation(ctx, p); err != nil {
				return err
			}
			return requireSyntheticAlive(held)
		})
	case "root-exit-writer":
		return withSyntheticChild(ctx, "spawn-writer", nil, func(p *nativeProcess, held windows.Handle) error {
			if err := awaitSynthetic(ctx, func() (bool, error) {
				done, code, err := p.exit()
				if done && code != 0 {
					return false, errors.New("writer parent failed")
				}
				return done, err
			}); err != nil {
				return err
			}
			if err := requireSyntheticExit(held, 0); err != nil {
				return err
			}
			var data [1]byte
			n, eof, err := p.read(data[:])
			if err != nil || n != 0 || eof {
				return errors.New("inherited writer was not retained")
			}
			// Root exit is not EOF. The outer fixture Job, not the dead root handle,
			// owns the inherited writer and must confirm its absence in cleanup.
			return shortObservation(ctx, p)
		})
	case "nonzero-exit":
		return withSyntheticChild(ctx, "nonzero", nil, func(p *nativeProcess, held windows.Handle) error {
			_, err := observe(ctx, p)
			if err == nil || !strings.Contains(err.Error(), "helper failed; operation outcome unconfirmed") {
				return errors.New("nonzero exit did not reach its exact exit refusal")
			}
			return requireSyntheticExit(held, 7)
		})
	case "pre-canceled-result":
		// This checks the returned refusal only. It does not independently count
		// Job members before cleanup; zero-dispatch ordering remains source and
		// portable-mock evidence, not a native membership claim.
		executable, err := os.Executable()
		if err != nil {
			return err
		}
		canceled, stop := context.WithCancel(ctx)
		stop()
		h := installedHelper{executable: executable}
		p, err := h.start(canceled, fixtureArguments("child", "idle"))
		if p != nil {
			_ = p.close()
			return errors.New("pre-canceled start created observer")
		}
		if !errors.Is(err, context.Canceled) {
			return errors.New("pre-canceled start did not refuse")
		}
		return nil
	case "handles":
		return nativeHandleFixture(ctx)
	case "console":
		return nativeConsoleFixture(ctx)
	case "redirected":
		return nativeRedirectedFixture(ctx)
	default:
		return errors.New("unknown synthetic scenario")
	}
}

func partialNativeFixture(ctx context.Context) (err error) {
	id, err := windows.GenerateGUID()
	if err != nil {
		return err
	}
	name := `Local\Hacocoon-NativeFixture-` + id.String()
	nativeName, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	gate, err := windows.CreateEvent(nil, 1, 0, nativeName)
	if err != nil {
		if gate != 0 {
			err = errors.Join(err, windows.CloseHandle(gate))
		}
		return err
	}
	defer func() { err = errors.Join(err, windows.CloseHandle(gate)) }()
	return withSyntheticChild(ctx, "partial", []string{name}, func(p *nativeProcess, held windows.Handle) error {
		var first []byte
		if err := awaitSynthetic(ctx, func() (bool, error) {
			var data [receiptLimit]byte
			n, eof, err := p.read(data[:])
			if err != nil {
				return false, err
			}
			if eof {
				return false, errors.New("partial writer closed early")
			}
			first = append(first, data[:n]...)
			if len(first) > 20 {
				return false, errors.New("partial receipt exceeded gated prefix")
			}
			return len(first) == 20, nil
		}); err != nil {
			return err
		}
		if string(first) != prepared[:20] {
			return errors.New("partial receipt was not isolated")
		}
		if err := windows.SetEvent(gate); err != nil {
			return err
		}
		rest, err := observe(ctx, p)
		if err != nil {
			return err
		}
		if operation, err := decodePrepared(append(first, rest...)); err != nil || operation != operationID {
			return errors.New("partial receipt did not complete exactly")
		}
		return requireSyntheticExit(held, 0)
	})
}

func nativeHandleFixture(ctx context.Context) (err error) {
	// A deliberately inheritable, private event must NOT pass production's exact
	// standard-handle list. Setting a colliding unrelated child-local handle cannot
	// signal this exact event, so the parent-side object is the observation.
	attributes := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), InheritHandle: 1}
	sentinel, err := windows.CreateEvent(&attributes, 1, 0, nil)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, windows.CloseHandle(sentinel)) }()
	if err := os.Setenv("HACO_SYNTHETIC_ENV_MARKER", "must-not-inherit"); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, os.Unsetenv("HACO_SYNTHETIC_ENV_MARKER")) }()
	return withSyntheticChild(ctx, "handles", []string{strconv.FormatUint(uint64(sentinel), 10)}, func(p *nativeProcess, held windows.Handle) error {
		raw, err := observe(ctx, p)
		if err != nil {
			return err
		}
		if _, err := decodeLaunch(raw); err != nil {
			return err
		}
		if err := requireSyntheticExit(held, 0); err != nil {
			return err
		}
		state, err := windows.WaitForSingleObject(sentinel, 0)
		if err != nil {
			return err
		}
		if state != uint32(windows.WAIT_TIMEOUT) {
			return errors.New("nonstandard inheritable handle escaped handle list")
		}
		return nil
	})
}

func nativeFixtureChild(mode string, extra []string) int {
	switch mode {
	case "idle", "hold-writer":
		if len(extra) != 0 {
			return 2
		}
		time.Sleep(fixtureHold)
		return 0
	case "oversized":
		if len(extra) != 0 {
			return 2
		}
		if _, err := os.Stdout.Write(bytes.Repeat([]byte("x"), receiptLimit+1)); err != nil {
			return 3
		}
		time.Sleep(fixtureHold)
		return 0
	case "close-output":
		if len(extra) != 0 {
			return 2
		}
		if err := os.Stdout.Close(); err != nil {
			return 3
		}
		time.Sleep(fixtureHold)
		return 0
	case "partial":
		if len(extra) != 1 || !strings.HasPrefix(extra[0], `Local\Hacocoon-NativeFixture-`) {
			return 2
		}
		if _, err := io.WriteString(os.Stdout, prepared[:20]); err != nil {
			return 3
		}
		name, err := windows.UTF16PtrFromString(extra[0])
		if err != nil {
			return 3
		}
		gate, err := windows.OpenEvent(windows.SYNCHRONIZE, false, name)
		if err != nil {
			return 3
		}
		state, waitErr := windows.WaitForSingleObject(gate, 5000)
		closeErr := windows.CloseHandle(gate)
		if waitErr != nil || closeErr != nil || state != windows.WAIT_OBJECT_0 {
			return 3
		}
		if _, err := io.WriteString(os.Stdout, prepared[20:]); err != nil {
			return 3
		}
		return 0
	case "nonzero":
		if len(extra) != 0 {
			return 2
		}
		if _, err := io.WriteString(os.Stdout, launched); err != nil {
			return 3
		}
		return 7
	case "spawn-writer":
		if len(extra) != 0 {
			return 2
		}
		executable, err := os.Executable()
		if err != nil {
			return 3
		}
		child := exec.Command(executable, fixtureArguments("child", "hold-writer")...)
		child.Env = os.Environ()
		child.Stdin = os.Stdin
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		child.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW, HideWindow: true}
		if err := child.Start(); err != nil {
			return 3
		}
		if err := child.Process.Release(); err != nil {
			return 3
		}
		return 0
	case "handles":
		if len(extra) != 1 {
			return 2
		}
		value, err := strconv.ParseUint(extra[0], 10, 64)
		if err != nil || value == 0 {
			return 2
		}
		_ = windows.SetEvent(windows.Handle(value))
		for _, stream := range []struct {
			file *os.File
			kind uint32
		}{{os.Stdin, windows.FILE_TYPE_CHAR}, {os.Stdout, windows.FILE_TYPE_PIPE}, {os.Stderr, windows.FILE_TYPE_CHAR}} {
			kind, err := windows.GetFileType(windows.Handle(stream.file.Fd()))
			if err != nil || kind != stream.kind {
				return 3
			}
		}
		var mode uint32
		if windows.GetConsoleMode(windows.Handle(os.Stdin.Fd()), &mode) == nil {
			return 3
		}
		var data [1]byte
		var n uint32
		if err := windows.ReadFile(windows.Handle(os.Stdin.Fd()), data[:], &n, nil); err != nil || n != 0 {
			return 3
		}
		if os.Getenv("HACO_SYNTHETIC_ENV_MARKER") != "" || os.Getenv("PATH") != "" || os.Getenv("WSLENV") != "" {
			return 3
		}
		system, err := windows.GetSystemDirectory()
		if err != nil {
			return 3
		}
		cwd, err := os.Getwd()
		if err != nil || !strings.EqualFold(cwd, system) {
			return 3
		}
		if _, err := io.WriteString(os.Stderr, "synthetic-discarded-diagnostic\n"); err != nil {
			return 3
		}
		if _, err := fmt.Fprint(os.Stdout, launched); err != nil {
			return 3
		}
		return 0
	default:
		return 2
	}
}
