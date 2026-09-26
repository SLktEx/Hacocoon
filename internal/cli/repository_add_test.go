package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
)

func TestProductRepoAddKeepsJSONAndProgressSeparate(t *testing.T) {
	server := control.NewServer()
	if err := server.RegisterStream(controlapi.MethodRepositoryAdd, func(_ context.Context, raw json.RawMessage) (control.Stream, error) {
		var req map[string]string
		if json.Unmarshal(raw, &req) != nil || len(req) != 2 || req["id"] != "sample" || req["remote"] != "https://github.com/example/repo.git" {
			t.Error("incorrect registration", string(raw))
		}
		return func(_ context.Context, c net.Conn) error {
			_, err := io.WriteString(c, "{\"progress\":\"Receiving objects: 100% (2/2), done.\"}\n{\"done\":true,\"result\":{\"kind\":\"repo\",\"id\":\"sample\",\"remote\":\"https://github.com/example/repo.git\",\"state\":\"ready\"}}\n")
			return err
		}, nil
	}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "control.sock")
	listener, err := control.ListenUnix(path, 0600)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	defer func() { cancel(); <-done }()
	t.Setenv("HACO_CONTROL_SOCKET", path)
	var out, diag bytes.Buffer
	if code := repositoryCommand(ctx, "repo", []string{"add", "--json", "sample", "https://github.com/example/repo.git"}, &out, &diag); code != 0 {
		t.Fatal(code, &diag)
	}
	if !json.Valid(out.Bytes()) || strings.Contains(out.String(), "Receiving") || !strings.Contains(diag.String(), "Receiving objects:") {
		t.Fatal(&out, &diag)
	}
}

func TestProductRepoAddRejectsOldRegistrationSyntax(t *testing.T) {
	for _, args := range [][]string{{"clone", "--branch", "main", "sample", "https://github.com/example/repo.git"}, {"add", "--branch", "main", "sample", "https://github.com/example/repo.git"}} {
		var out, diag bytes.Buffer
		if code := repositoryCommand(context.Background(), "repo", args, &out, &diag); code != 2 || out.Len() != 0 {
			t.Fatal(code, &out, &diag)
		}
	}
}

func TestRepositoryProgressWriterAnimatesRepeatedStageOnTerminal(t *testing.T) {
	var out bytes.Buffer
	progress := &repositoryProgressWriter{out: &out, terminal: true}
	for _, chunk := range []string{
		"Cloning into managed repository...\n",
		"Receiving objects: 10% (1/10)\r\n",
		"Receiving objects: 20% (2/10)\n",
		"Resolving deltas: 100% (1/1), done.\n",
	} {
		if _, err := io.WriteString(progress, chunk); err != nil {
			t.Fatal(err)
		}
	}
	if err := progress.Finish(); err != nil {
		t.Fatal(err)
	}
	want := "\r\x1b[2KCloning into managed repository...\n" +
		"\r\x1b[2KReceiving objects: 10% (1/10)" +
		"\r\x1b[2KReceiving objects: 20% (2/10)\n" +
		"\r\x1b[2KResolving deltas: 100% (1/1), done.\n"
	if out.String() != want {
		t.Fatalf("unexpected terminal progress:\nwant %q\n got %q", want, out.String())
	}
}

func TestRepositoryProgressWriterKeepsNonTerminalOutputPlain(t *testing.T) {
	var out bytes.Buffer
	progress := &repositoryProgressWriter{out: &out, terminal: false}
	input := "Receiving objects: 10% (1/10)\nReceiving objects: 20% (2/10)\n"
	if _, err := io.WriteString(progress, input); err != nil {
		t.Fatal(err)
	}
	if err := progress.Finish(); err != nil {
		t.Fatal(err)
	}
	if out.String() != input {
		t.Fatalf("non-terminal progress changed: %q", out.String())
	}
}

func TestRepositoryProgressWriterBuffersSplitLinesAndFlushesPendingData(t *testing.T) {
	var out bytes.Buffer
	progress := &repositoryProgressWriter{out: &out, terminal: true}

	if _, err := io.WriteString(progress, "Receiving objects: 10"); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("partial line rendered early: %q", out.String())
	}
	if _, err := io.WriteString(progress, "% (1/10)\nResolving deltas: 100% (1/1), done."); err != nil {
		t.Fatal(err)
	}
	if err := progress.Finish(); err != nil {
		t.Fatal(err)
	}

	want := "\r\x1b[2KReceiving objects: 10% (1/10)\n" +
		"\r\x1b[2KResolving deltas: 100% (1/1), done.\n"
	if out.String() != want {
		t.Fatalf("unexpected split-line progress:\nwant %q\n got %q", want, out.String())
	}
}

func TestRepositoryProgressWriterIgnoresEmptyTerminalLines(t *testing.T) {
	var out bytes.Buffer
	progress := &repositoryProgressWriter{out: &out, terminal: true}
	if _, err := io.WriteString(progress, "\n"); err != nil {
		t.Fatal(err)
	}
	if err := progress.Finish(); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("empty progress should not render: %q", out.String())
	}
}

type repositoryProgressFDWriter struct {
	bytes.Buffer
}

func (*repositoryProgressFDWriter) Fd() uintptr {
	return ^uintptr(0)
}

func TestNewRepositoryProgressWriterChecksFileDescriptors(t *testing.T) {
	var out repositoryProgressFDWriter
	progress := newRepositoryProgressWriter(&out)
	if progress.out != &out {
		t.Fatal("progress writer changed output")
	}
	if progress.terminal {
		t.Fatal("invalid file descriptor detected as terminal")
	}
}

var errRepositoryProgressWrite = errors.New("repository progress write failed")

type repositoryProgressFailWriter struct {
	writes int
	failAt int
}

func (w *repositoryProgressFailWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes == w.failAt {
		return 0, errRepositoryProgressWrite
	}
	return len(p), nil
}

func TestRepositoryProgressWriterReportsOutputErrors(t *testing.T) {
	t.Run("render", func(t *testing.T) {
		progress := &repositoryProgressWriter{out: &repositoryProgressFailWriter{failAt: 1}, terminal: true}
		if _, err := io.WriteString(progress, "Receiving objects: 10% (1/10)\n"); !errors.Is(err, errRepositoryProgressWrite) {
			t.Fatalf("expected render error, got %v", err)
		}
	})

	t.Run("stage separator", func(t *testing.T) {
		out := &repositoryProgressFailWriter{failAt: 2}
		progress := &repositoryProgressWriter{out: out, terminal: true}
		if _, err := io.WriteString(progress, "Receiving objects: 10% (1/10)\n"); err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(progress, "Resolving deltas: 100% (1/1), done.\n"); !errors.Is(err, errRepositoryProgressWrite) {
			t.Fatalf("expected stage separator error, got %v", err)
		}
	})

	t.Run("pending render on finish", func(t *testing.T) {
		progress := &repositoryProgressWriter{out: &repositoryProgressFailWriter{failAt: 1}, terminal: true}
		if _, err := io.WriteString(progress, "Receiving objects: 10% (1/10)"); err != nil {
			t.Fatal(err)
		}
		if err := progress.Finish(); !errors.Is(err, errRepositoryProgressWrite) {
			t.Fatalf("expected pending render error, got %v", err)
		}
	})

	t.Run("final newline", func(t *testing.T) {
		out := &repositoryProgressFailWriter{failAt: 2}
		progress := &repositoryProgressWriter{out: out, terminal: true}
		if _, err := io.WriteString(progress, "Receiving objects: 10% (1/10)"); err != nil {
			t.Fatal(err)
		}
		if err := progress.Finish(); !errors.Is(err, errRepositoryProgressWrite) {
			t.Fatalf("expected final newline error, got %v", err)
		}
	})
}
