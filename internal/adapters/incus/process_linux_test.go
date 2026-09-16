//go:build linux

package incus

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestProcessAdapterStreamsStdinWithoutTTYOrShellInterpolation(t *testing.T) {
	dir := t.TempDir()
	script := `#!/bin/sh
set -eu
[ "$1" = exec ] && [ "$2" = haco-demo ] && [ "$3" = --project ] && [ "$4" = test ]
[ "$5" = --cwd ] && [ "$6" = /workspace ] && [ "$7" = --force-noninteractive ] && [ "$8" = -- ]
shift 8
exec "$@"
`
	if err := os.WriteFile(filepath.Join(dir, "incus"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	runtime := &Runtime{project: "test"}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	data := bytes.Repeat([]byte{0, 255, '\n'}, 400000)
	literal := "--literal ; $(must-not-execute)"
	var out, diagnostic bytes.Buffer
	result, err := runtime.ExecEnvironmentStream(ctx, "haco-demo", core.ProcessRequest{WorkingDirectory: "/workspace", Argv: []string{"/bin/sh", "-c", `cat; printf '%s' "$1" >&2; exit 17`, "sh", literal}}, bytes.NewReader(data), &out, &diagnostic)
	var exit interface{ ExitCode() int }
	if !errors.As(err, &exit) || exit.ExitCode() != 17 || result.ExitCode != 17 || !bytes.Equal(out.Bytes(), data) || diagnostic.String() != literal {
		t.Fatal("stream/options/argv changed", result, err, diagnostic.String())
	}
}

func TestProcessAdapterRejectsUnownedTargetsAndInvalidRequests(t *testing.T) {
	runtime := &Runtime{project: "test"}
	t.Setenv("PATH", t.TempDir())
	for _, entry := range []struct {
		ref     string
		request core.ProcessRequest
	}{
		{"--project=other", core.ProcessRequest{Argv: []string{"true"}}},
		{"haco-demo", core.ProcessRequest{WorkingDirectory: "relative", Argv: []string{"true"}}},
		{"haco-demo", core.ProcessRequest{Argv: []string{"bad\x00command"}}},
		{"haco-demo", core.ProcessRequest{Argv: []string{"bash"}, TTY: true}},
	} {
		_, err := runtime.ExecEnvironmentStream(context.Background(), entry.ref, entry.request, strings.NewReader(""), io.Discard, io.Discard)
		if !errors.Is(err, core.ErrInvalidArgument) && !errors.Is(err, core.ErrIncompatibleState) {
			t.Fatal("invalid request reached command execution", err)
		}
	}
}
