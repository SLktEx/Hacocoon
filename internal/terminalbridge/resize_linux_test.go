//go:build linux

package terminalbridge

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"reflect"
	"testing"
	"time"

	"golang.org/x/term"
)

type resizeTestConn struct {
	net.Conn
	peer net.Conn
}

func (*resizeTestConn) SupportsResize() bool { return true }
func (c *resizeTestConn) Resize(_ context.Context, columns, rows int) error {
	_, err := fmt.Fprintf(c.peer, "RESIZE=%dx%d\n", columns, rows)
	if columns == 37 && rows == 17 {
		c.peer.Close()
	}
	return err
}

func TestTerminalResizeSignalAndRestoreProcess(t *testing.T) {
	if mode := os.Getenv("HACO_TEST_BRIDGE_RESIZE"); mode != "" {
		before, err := term.GetState(int(os.Stdin.Fd()))
		if err != nil {
			t.Fatal(err)
		}
		local, remote := net.Pipe()
		defer remote.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		var stream net.Conn = &resizeTestConn{Conn: local, peer: remote}
		if mode == "ended" {
			resized := make(chan struct{})
			stream = &endedResizeConn{Conn: local, resized: resized}
			go func() {
				<-resized
				fmt.Fprintln(remote, "FINAL OUTPUT")
				remote.Close()
			}()
		}
		if err := Bridge(ctx, stream, os.Stdin, os.Stdout); err != nil {
			t.Fatal(err)
		}
		after, err := term.GetState(int(os.Stdin.Fd()))
		if err != nil || !reflect.DeepEqual(before, after) {
			t.Fatalf("terminal was not restored: %v", err)
		}
		fmt.Println("RESTORED")
		return
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("Python PTY support unavailable")
	}
	script := `import fcntl, os, pty, select, signal, struct, subprocess, sys, termios, time
master, slave = pty.openpty()
fcntl.ioctl(master, termios.TIOCSWINSZ, struct.pack('HHHH', 24, 80, 0, 0))
env = dict(os.environ, HACO_TEST_BRIDGE_RESIZE=sys.argv[2])
process = subprocess.Popen([sys.argv[1], '-test.run=^TestTerminalResizeSignalAndRestoreProcess$'], stdin=slave, stdout=slave, stderr=slave, env=env)
os.close(slave)
output = b''
resized = False
try:
    deadline = time.monotonic() + 10
    while time.monotonic() < deadline:
        if select.select([master], [], [], .1)[0]:
            try: chunk = os.read(master, 4096)
            except OSError: break
            if not chunk: break
            output += chunk
            if len(output) > 65536: raise RuntimeError('unbounded output')
            if not resized and b'RESIZE=80x24' in output:
                fcntl.ioctl(master, termios.TIOCSWINSZ, struct.pack('HHHH', 17, 37, 0, 0))
                process.send_signal(signal.SIGWINCH)
                resized = True
    assert process.wait(timeout=2) == 0, output
    expected = b'FINAL OUTPUT' if sys.argv[2] == 'ended' else b'RESIZE=37x17'
    assert expected in output and b'RESTORED' in output, output
finally:
    if process.poll() is None: process.kill()
    process.wait()
    os.close(master)
`
	for _, mode := range []string{"resize", "ended"} {
		cmd := exec.Command(python, "-I", "-c", script, os.Args[0], mode)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("terminal resize process %s: %v\n%s", mode, err, output)
		}
	}
}

// A late resize must not discard final output after normal session completion.
type endedResizeConn struct {
	net.Conn
	resized chan struct{}
}

func (*endedResizeConn) SupportsResize() bool                     { return true }
func (c *endedResizeConn) Resize(context.Context, int, int) error { close(c.resized); return io.EOF }
