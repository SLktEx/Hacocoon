//go:build linux

package incus

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// Incus already implements initial dimensions and SIGWINCH forwarding. Give
// its CLI a private raw PTY so it can use those native capabilities even when
// the controller's own stdin/stdout are sockets. No physical terminal is shared.
func runSizedInteractiveCommand(ctx context.Context, cmd *exec.Cmd, stdin io.Reader, metadata core.TerminalMetadata) error {
	master, slave, err := openInteractivePTY(metadata.Columns, metadata.Rows)
	if err != nil {
		return err
	}
	defer master.Close()
	defer slave.Close()
	stdout := cmd.Stdout
	cmd.Stdin, cmd.Stdout = slave, slave
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := cmd.Start(); err != nil {
		return err
	}
	_ = slave.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	resizeDone := make(chan error, 1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				resizeDone <- nil
				return
			case size, ok := <-metadata.Resizes:
				if !ok {
					resizeDone <- nil
					return
				}
				if err := setInteractivePTYSize(master, size.Columns, size.Rows); err != nil {
					resizeDone <- err
					_ = cmd.Process.Kill()
					return
				}
			}
		}
	}()
	go func() {
		_, _ = io.Copy(master, stdin)
		// Only interactive TTY callers use this path. Input EOF means the
		// client disconnected; typed Ctrl-D is a byte forwarded to Incus.
		_ = cmd.Process.Kill()
	}()
	outputDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(stdout, master)
		if errors.Is(err, syscall.EIO) { // Linux PTY EOF after the slave closes.
			err = nil
		}
		outputDone <- err
		if err != nil {
			_ = cmd.Process.Kill()
		}
	}()

	waitErr := cmd.Wait()
	cancel()
	resizeErr := <-resizeDone
	// Drain final bytes after process exit. Bound an inherited slave or a
	// disconnected output consumer rather than hanging controller teardown.
	timer := time.AfterFunc(5*time.Second, func() {
		_ = master.Close()
		if closer, ok := stdout.(io.Closer); ok {
			_ = closer.Close()
		}
	})
	outputErr := <-outputDone
	timer.Stop()
	return errors.Join(waitErr, resizeErr, outputErr)
}

func openInteractivePTY(columns, rows int) (*os.File, *os.File, error) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, nil, err
	}
	var slave *os.File
	fail := func(err error) (*os.File, *os.File, error) {
		master.Close()
		if slave != nil {
			slave.Close()
		}
		return nil, nil, err
	}
	if err := unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		return fail(err)
	}
	number, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
	if err != nil {
		return fail(err)
	}
	slave, err = os.OpenFile(fmt.Sprintf("/dev/pts/%d", number), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		return fail(err)
	}
	if _, err := term.MakeRaw(int(slave.Fd())); err != nil {
		return fail(err)
	}
	if err := setInteractivePTYSize(master, columns, rows); err != nil {
		return fail(err)
	}
	return master, slave, nil
}

func setInteractivePTYSize(master *os.File, columns, rows int) error {
	if columns < 1 || columns > 10000 || rows < 1 || rows > 10000 {
		return core.ErrInvalidArgument
	}
	return unix.IoctlSetWinsize(int(master.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Col: uint16(columns), Row: uint16(rows)})
}
