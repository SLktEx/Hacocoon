//go:build linux

package incus

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestHostProgramStreamPreservesFramesAndIsolatesInput(t *testing.T) {
	dir := shellBoundaryChild(t, "/bin/cat\n/usr/bin/head -c 131072 /dev/zero\nprintf 'private provider diagnostic' >&2\n")
	r := New(&programRunner{owned: true})
	input := []byte("opaque framed request\n")
	const program = "print('trusted program')"
	var out bytes.Buffer
	if err := r.RunTrustedHostPythonStream(context.Background(), program, input, &out); err != nil {
		t.Fatal(err)
	}
	want := append(append([]byte(nil), input...), make([]byte, 131072)...)
	if !bytes.Equal(out.Bytes(), want) {
		t.Fatal("stream was truncated, changed or mixed with provider diagnostics")
	}
	args := shellBoundaryArgs(t, dir)
	if len(args) < 8 || !slices.Equal(args[:8], []string{"exec", "haco-host", "--project", "hacocoon", "--cwd", "/root", "--", "/usr/bin/systemd-run"}) {
		t.Fatalf("stream target or execution boundary changed: %q", args)
	}
	for _, required := range []string{"--wait", "--pipe", "--property=KillMode=control-group", "--property=RuntimeMaxSec=590s", "--property=MemoryMax=512M", "--property=TasksMax=64"} {
		if !slices.Contains(args, required) {
			t.Errorf("stream lacks lifetime/resource boundary %q", required)
		}
	}
	const python = "/usr/bin/python3"
	index := slices.Index(args, "/usr/bin/env")
	if index < 0 || !slices.Equal(args[index:], []string{"/usr/bin/env", "-i", "PATH=/usr/local/bin:/usr/bin:/bin", "HOME=/root", python, "-I", "-c", program}) || strings.Contains(strings.Join(args, " "), string(input)) {
		t.Fatal("stream input or inherited environment entered program arguments")
	}
}

func TestHostProgramStreamRefusesInvalidInputOrUnownedHostBeforeLaunch(t *testing.T) {
	for _, test := range []struct {
		name, program string
		input         []byte
		out           io.Writer
		owned         bool
	}{
		{name: "empty program", out: io.Discard, owned: true},
		{name: "large program", program: strings.Repeat("x", 65537), out: io.Discard, owned: true},
		{name: "large input", program: "pass", input: make([]byte, 8193), out: io.Discard, owned: true},
		{name: "missing output", program: "pass", owned: true},
		{name: "unowned Host", program: "pass", out: io.Discard},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := shellBoundaryChild(t, "exit 0\n")
			err := New(&programRunner{owned: test.owned}).RunTrustedHostPythonStream(context.Background(), test.program, test.input, test.out)
			if err == nil || test.owned && !errors.Is(err, core.ErrInvalidArgument) {
				t.Fatalf("invalid stream request accepted: %v", err)
			}
			if _, err := os.Stat(filepath.Join(dir, "args")); !os.IsNotExist(err) {
				t.Fatal("invalid stream request launched a process")
			}
		})
	}
}

func TestHostProgramStreamDoesNotReportSuccessOnIncompleteDelivery(t *testing.T) {
	for _, mode := range []string{"child exit", "output failure", "canceled", "missing command"} {
		t.Run(mode, func(t *testing.T) {
			body := "printf 'partial frame'\nprintf 'private provider diagnostic' >&2\n"
			if mode == "child exit" {
				body += "exit 17\n"
			}
			dir := shellBoundaryChild(t, body)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "canceled" {
				cancel()
			}
			if mode == "missing command" {
				if err := os.Remove(filepath.Join(dir, "incus")); err != nil {
					t.Fatal(err)
				}
			}
			out := io.Discard
			if mode == "output failure" {
				out = shellOutputFailure{errors.New("private output diagnostic")}
			}
			err := New(&programRunner{owned: true}).RunTrustedHostPythonStream(ctx, "pass", nil, out)
			if !errors.Is(err, core.ErrRuntimeUnavailable) || strings.Contains(err.Error(), "private") {
				t.Fatalf("incomplete stream returned success or leaked diagnostics: %v", err)
			}
		})
	}
}
