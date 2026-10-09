//go:build linux

package terminalbridge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

func inputFD(t *testing.T, file *os.File, f func(uintptr)) {
	t.Helper()
	raw, err := file.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	if err := raw.Control(f); err != nil {
		t.Fatal(err)
	}
}

func inputFlags(t *testing.T, file *os.File) int {
	t.Helper()
	var flags int
	var err error
	inputFD(t, file, func(fd uintptr) { flags, err = unix.FcntlInt(fd, unix.F_GETFL, 0) })
	if err != nil {
		t.Fatal(err)
	}
	return flags
}

func inputTerminalState(t *testing.T, file *os.File) *term.State {
	t.Helper()
	var state *term.State
	var err error
	inputFD(t, file, func(fd uintptr) { state, err = term.GetState(int(fd)) })
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func inputPipe(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	// Match inherited stdin: a blocking descriptor, rather than os.Pipe's
	// already-pollable descriptor. Opening our reader must not change it.
	var fds [2]int
	if err := unix.Pipe2(fds[:], unix.O_CLOEXEC); err != nil {
		t.Fatal(err)
	}
	reader, writer := os.NewFile(uintptr(fds[0]), "input"), os.NewFile(uintptr(fds[1]), "producer")
	t.Cleanup(func() { _ = writer.Close(); _ = reader.Close() })
	return reader, writer
}

func inputTTY(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	fd, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NONBLOCK|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	master := os.NewFile(uintptr(fd), "terminal-master")
	t.Cleanup(func() { _ = master.Close() })
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		t.Fatal(err)
	}
	number, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	if err != nil {
		t.Fatal(err)
	}
	fd, err = unix.Open(fmt.Sprintf("/dev/pts/%d", number), unix.O_RDWR|unix.O_NONBLOCK|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	slave := os.NewFile(uintptr(fd), "terminal-input")
	t.Cleanup(func() { _ = slave.Close() })
	return slave, master
}

func TestBridgeOwnsBlockedFileInputUntilEOFOrCancellation(t *testing.T) {
	for _, kind := range []string{"pipe", "pollable-pipe", "tty"} {
		for _, ending := range []string{"EOF", "cancel", "output-failure"} {
			t.Run(kind+"/"+ending, func(t *testing.T) {
				factory := inputPipe
				switch kind {
				case "tty":
					factory = inputTTY
				case "pollable-pipe":
					factory = func(t *testing.T) (*os.File, *os.File) {
						reader, writer, err := os.Pipe()
						if err != nil {
							t.Fatal(err)
						}
						t.Cleanup(func() { _ = writer.Close(); _ = reader.Close() })
						return reader, writer
					}
				}
				input, producer := factory(t)
				flags := inputFlags(t, input)
				var terminalState *term.State
				if kind == "tty" {
					terminalState = inputTerminalState(t, input)
				}
				client, remote := net.Pipe()
				defer func() { _ = remote.Close() }()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				prepared := make(chan *os.File, 1)
				done := make(chan error, 1)
				outputFailure := errors.New("output failed")
				output := io.Discard
				if ending == "output-failure" {
					output = failedInputTestOutput{outputFailure}
				}
				go func() {
					done <- BridgeWithTerminal(ctx, client, input, output, func(reader io.Reader) (func() error, error) {
						restore, err := PrepareInteractiveTerminal(reader)
						prepared <- reader.(*os.File)
						return restore, err
					})
				}()
				var owned *os.File
				select {
				case owned = <-prepared:
				case <-ctx.Done():
					t.Fatal("input was not prepared")
				}
				if _, err := producer.Write([]byte("a")); err != nil {
					t.Fatal(err)
				}
				_ = remote.SetReadDeadline(time.Now().Add(time.Second))
				var first [1]byte
				if _, err := io.ReadFull(remote, first[:]); err != nil || first[0] != 'a' {
					t.Fatal("input did not reach the stream", err)
				}
				switch ending {
				case "cancel":
					cancel()
				case "output-failure":
					_, _ = remote.Write([]byte("output"))
				default:
					_ = remote.Close()
				}
				select {
				case err := <-done:
					if ending == "cancel" && !errors.Is(err, context.Canceled) || ending == "EOF" && err != nil || ending == "output-failure" && !errors.Is(err, outputFailure) {
						t.Fatal("wrong stream outcome", err)
					}
				case <-time.After(time.Second):
					t.Fatal("blocked stdin prevented bridge completion")
				}
				if owned == input {
					t.Fatal("bridge did not acquire an independently owned input descriptor")
				}
				if _, err := owned.Stat(); !errors.Is(err, os.ErrClosed) {
					t.Fatal("bridge returned without closing its input descriptor", err)
				}
				if got := inputFlags(t, input); got != flags {
					t.Fatalf("caller input flags changed: %x -> %x", flags, got)
				}
				if kind == "tty" && !reflect.DeepEqual(inputTerminalState(t, input), terminalState) {
					t.Fatal("terminal state was not restored before closing owned input")
				}
				// A leftover copier would steal these bytes from the next caller.
				if _, err := producer.Write([]byte("next\n")); err != nil {
					t.Fatal(err)
				}
				continued := make(chan error, 1)
				go func() {
					data := make([]byte, 5)
					_, err := io.ReadFull(input, data)
					if err == nil && string(data) != "next\n" {
						err = fmt.Errorf("later input changed: %q", data)
					}
					continued <- err
				}()
				select {
				case err := <-continued:
					if err != nil {
						t.Fatal("caller input became unusable", err)
					}
				case <-time.After(time.Second):
					_ = producer.Close()
					t.Fatal("old bridge retained the caller's input")
				}
			})
		}
	}
}

type failedInputTestOutput struct{ err error }

func (w failedInputTestOutput) Write([]byte) (int, error) { return 0, w.err }

func TestBridgeRegularInputKeepsEOFAndHalfClose(t *testing.T) {
	file, err := os.Create(filepath.Join(t.TempDir(), "stdin"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	if _, err := file.WriteString("skipinput"); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Seek(4, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	conn := newScriptedConn("response")
	var output strings.Builder
	if err := Bridge(context.Background(), conn, file, &output); err != nil {
		t.Fatal(err)
	}
	if conn.input.String() != "input" || output.String() != "response" {
		t.Fatal("regular-file EOF/half-close or stream bytes changed")
	}
	if position, err := file.Seek(0, io.SeekCurrent); err != nil || position != 9 {
		t.Fatal("caller file closed or position changed", position, err)
	}
}

func TestBridgeInputPreparationFailureClosesOnlyOwnedInput(t *testing.T) {
	input, _ := inputPipe(t)
	failure := errors.New("terminal preparation failed")
	var owned *os.File
	err := BridgeWithTerminal(context.Background(), newScriptedConn(""), input, io.Discard, func(reader io.Reader) (func() error, error) {
		owned = reader.(*os.File)
		return nil, failure
	})
	if !errors.Is(err, failure) || owned == nil || owned == input {
		t.Fatal("preparation outcome changed", err)
	}
	if _, err := owned.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("owned descriptor survived preparation failure", err)
	}
	if _, err := input.Stat(); err != nil {
		t.Fatal("caller input was closed", err)
	}
}

func TestRegularInputPreservesBorrowedOffsetAndCaller(t *testing.T) {
	file, err := os.Create(filepath.Join(t.TempDir(), "stdin"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	if _, err := file.WriteString("skipnextremaining"); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Seek(4, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	flags := inputFlags(t, file)
	reader, closeReader, err := ownInput(file)
	if err != nil || closeReader != nil || reader != file {
		t.Fatal("regular input ownership changed", err)
	}
	data := make([]byte, 4)
	if _, err := io.ReadFull(reader, data); err != nil || string(data) != "next" {
		t.Fatal("regular file position changed", string(data), err)
	}
	remaining, err := io.ReadAll(file)
	if err != nil || string(remaining) != "remaining" || inputFlags(t, file) != flags {
		t.Fatal("caller file position, ownership or flags changed", string(remaining), err)
	}
}

func TestOwnedInputPreservesFiniteReadersAndRefusesUnreadableDescriptors(t *testing.T) {
	reader := strings.NewReader("finite")
	got, closeReader, err := ownInput(reader)
	if err != nil || got != reader || closeReader != nil {
		t.Fatal("non-file reader ownership changed", err)
	}
	_, writer := inputPipe(t)
	if _, closeReader, err := ownInput(writer); err == nil || closeReader != nil {
		t.Fatal("write-only descriptor gained read authority", err)
	}
	file, err := os.Create(filepath.Join(t.TempDir(), "closed"))
	if err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	if _, closeReader, err := ownInput(file); err == nil || closeReader != nil {
		t.Fatal("closed file accepted", err)
	}
	fd, err := unix.Open(file.Name(), unix.O_PATH|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	pathOnly := os.NewFile(uintptr(fd), "path-only")
	defer func() { _ = pathOnly.Close() }()
	if _, closeReader, err := ownInput(pathOnly); err == nil || closeReader != nil {
		t.Fatal("path-only descriptor gained read authority", err)
	}
	null, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = null.Close() }()
	if got, closeReader, err := ownInput(null); err != nil || closeReader != nil || got != null {
		t.Fatal("non-terminal device input semantics changed", err)
	}
}

func TestOwnedInputRefusesPTYMasterWithoutRetargeting(t *testing.T) {
	slave, master := inputTTY(t)
	flags := inputFlags(t, master)
	prepared := false
	err := BridgeWithTerminal(context.Background(), newScriptedConn(""), master, io.Discard, func(io.Reader) (func() error, error) {
		prepared = true
		return nil, nil
	})
	if err == nil || prepared {
		t.Fatal("PTY master was retargeted or prepared", err)
	}
	if inputFlags(t, master) != flags {
		t.Fatal("refusal changed the caller's descriptor")
	}
	if _, err := master.Write([]byte("next\n")); err != nil {
		t.Fatal("refusal closed the master", err)
	}
	if err := slave.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var data [5]byte
	if _, err := io.ReadFull(slave, data[:]); err != nil || string(data[:]) != "next\n" {
		t.Fatal("refusal changed the caller's terminal pair", err)
	}
}

type blockedInputWriteConn struct {
	net.Conn
	writing, outputEOF, closed chan struct{}
	once                       sync.Once
	writeErr                   error
}

func (c *blockedInputWriteConn) Read([]byte) (int, error) {
	<-c.outputEOF
	return 0, io.EOF
}
func (c *blockedInputWriteConn) Write([]byte) (int, error) {
	close(c.writing)
	<-c.closed
	return 0, c.writeErr
}
func (c *blockedInputWriteConn) Close() error {
	c.once.Do(func() { close(c.closed) })
	return nil
}

func TestBridgeJoinsBlockedInputWriteWithoutLosingSuccessfulExit(t *testing.T) {
	failure := errors.New("input write failed")
	for _, writeErr := range []error{io.ErrClosedPipe, failure} {
		t.Run(writeErr.Error(), func(t *testing.T) {
			input, producer := inputPipe(t)
			conn := &blockedInputWriteConn{writing: make(chan struct{}), outputEOF: make(chan struct{}), closed: make(chan struct{}), writeErr: writeErr}
			defer func() { _ = conn.Close() }()
			done := make(chan error, 1)
			go func() { done <- Bridge(context.Background(), conn, input, io.Discard) }()
			if _, err := producer.Write([]byte("stdin")); err != nil {
				t.Fatal(err)
			}
			select {
			case <-conn.writing:
			case <-time.After(time.Second):
				t.Fatal("input copier did not block writing to the stream")
			}
			close(conn.outputEOF)
			select {
			case err := <-done:
				if writeErr == io.ErrClosedPipe && err != nil {
					t.Fatal("teardown discarded successful remote exit", err)
				}
				if writeErr == failure && !errors.Is(err, failure) {
					t.Fatal("meaningful input failure was hidden", err)
				}
			case <-time.After(time.Second):
				t.Fatal("bridge did not join blocked stream writer")
			}
		})
	}
}

type completedInputProcessSession struct {
	net.Conn
	done <-chan error
}

func (s completedInputProcessSession) Wait(ctx context.Context) error {
	select {
	case err := <-s.done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestBridgeOwnedInputPreservesProcessCompletionReceipt(t *testing.T) {
	input, producer := inputPipe(t)
	server, wire := net.Pipe()
	defer func() { _ = wire.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	completed := make(chan error, 1)
	receipt := `{"exit_code":17}`
	go func() {
		completed <- control.ServeProcess(ctx, server, func(_ context.Context, stdin io.Reader, stdout, stderr io.Writer) ([]byte, error) {
			var first [1]byte
			if _, err := io.ReadFull(stdin, first[:]); err != nil {
				return nil, err
			}
			if _, err := stdout.Write(first[:]); err != nil {
				return nil, err
			}
			if _, err := io.WriteString(stderr, "diagnostic"); err != nil {
				return nil, err
			}
			return []byte(receipt), nil
		})
	}()
	var output, diagnostic strings.Builder
	stream, err := control.NewProcessConn(ctx, completedInputProcessSession{Conn: wire, done: completed}, &diagnostic)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := producer.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := BridgeWithTerminal(ctx, stream, input, &output, func(io.Reader) (func() error, error) { return nil, nil }); err != nil {
		t.Fatal("input cleanup lost process completion", err)
	}
	result, err := stream.Result()
	if err != nil || string(result) != receipt || output.String() != "x" || diagnostic.String() != "diagnostic" {
		t.Fatal("process receipt or separated output changed", string(result), err)
	}
}
