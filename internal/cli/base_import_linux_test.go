//go:build linux

package cli

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/base/build"
	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestBaseImportCLIStreamsChosenFileAndShowsNextStep(t *testing.T) {
	server := control.NewServer()
	calls := 0
	fail := false
	if err := controlapi.RegisterBaseImport(server, func(_ context.Context, r io.Reader, req basebuild.ImportRequest) (basebuild.Result, error) {
		calls++
		data, err := io.ReadAll(r)
		if err != nil {
			return basebuild.Result{}, err
		}
		if string(data) != "archive bytes" {
			t.Error("wrong input")
		}
		result := basebuild.Result{Base: core.BaseInfo{Name: req.Name, Revision: core.BaseRevision("sha256:" + strings.Repeat("a", 64))}, State: "ready"}
		if fail {
			result.State = "publication-unconfirmed"
			result.Builder = "build-" + strings.Repeat("b", 32)
			return result, core.ErrRecoveryRequired
		}
		return result, nil
	}); err != nil {
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
	file := filepath.Join(t.TempDir(), "image.tar")
	if err := os.WriteFile(file, []byte("archive bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, lang := range []string{"en", "ja"} {
		t.Setenv("HACO_UI_LANGUAGE", lang)
		code, out, diagnostic := captureRun(t, "base", "import", "--name", "tools", file)
		if code != 0 || diagnostic != "" || !strings.Contains(out, "--base tools") {
			t.Fatal(code, out, diagnostic)
		}
		want := "is ready"
		if lang == "ja" {
			want = "取り込みました"
		}
		if !strings.Contains(out, want) {
			t.Fatal("missing translation", out)
		}
	}
	fail = true
	code, out, diagnostic := captureRun(t, "base", "import", "--name", "tools", "--json", file)
	var result basebuild.Result
	if code != 1 || diagnostic == "" || json.Unmarshal([]byte(out), &result) != nil || result.Builder == "" {
		t.Fatal(code, out, diagnostic)
	}
	before := calls
	link := file + "-link"
	if err := os.Symlink(file, link); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{link, filepath.Dir(file)} {
		code, _, _ := captureRun(t, "base", "import", "--name", "tools", input)
		if code != 1 {
			t.Fatal(input, code)
		}
	}
	code, _, _ = captureRun(t, "base", "import", file)
	if code != 2 || calls != before {
		t.Fatal("invalid input reached import", code, calls)
	}
	original, err := os.ReadFile(file)
	if err != nil || string(original) != "archive bytes" {
		t.Fatal("source changed", err)
	}
}
