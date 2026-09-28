package cli

import (
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/core"
	"os"
	"path/filepath"
	"strings"
	"testing"

	basebuild "github.com/SLktEx/Hacocoon/internal/base/build"
	controlapi "github.com/SLktEx/Hacocoon/internal/controller/api"
	control "github.com/SLktEx/Hacocoon/internal/controller/transport"
)

type cliPackerFixture struct {
	calls      int
	definition basebuild.Definition
	result     basebuild.Result
	failure    error
}

func (f *cliPackerFixture) Build(_ context.Context, definition basebuild.Definition) (basebuild.Result, error) {
	f.calls++
	f.definition = definition
	return f.result, f.failure
}
func TestPackerCannotReachControllerDefinitionBuilder(t *testing.T) {
	fixture := &cliPackerFixture{}
	server := control.NewServer()
	if err := controlapi.RegisterBaseBuild(server, fixture); err != nil {
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
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "base.pkr.hcl"), []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"base", "build", "--name", "../invalid", directory},
		{"base", "build", "--name", "tools", "--from", "haco/ubuntu-26.04", directory},
		{"base", "build", "--name", "tools", "--builder", "builder", directory},
	} {
		code, _, _ := captureRun(t, args...)
		if code != 2 || fixture.calls != 0 {
			t.Fatal("invalid request reached controller", code)
		}
	}

}

func TestJSONDefinitionCLIStillUsesCanonicalControllerBuild(t *testing.T) {
	fixture := &cliPackerFixture{result: basebuild.Result{Base: core.BaseInfo{Name: "tools", Revision: core.BaseRevision("sha256:" + strings.Repeat("a", 64))}, State: "ready"}}
	server := control.NewServer()
	if err := controlapi.RegisterBaseBuild(server, fixture); err != nil {
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
	file := filepath.Join(t.TempDir(), "base.json")
	if err := os.WriteFile(file, []byte(`{"name":"tools","from":"source","run":"echo json-builder"}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"ready", "publication-unconfirmed"} {
		fixture.result.State = state
		expected := 0
		if state != "ready" {
			fixture.failure = core.ErrRecoveryRequired
			fixture.result.Builder = "retained-builder"
			expected = 1
		}
		code, out, _ := captureRun(t, "base", "build", "--builder", "json-builder", "--json", file)
		var result basebuild.Result
		if code != expected || json.Unmarshal([]byte(out), &result) != nil || result.State != state || fixture.definition.BuilderName != "json-builder" || fixture.definition.Run != "echo json-builder" || fixture.definition.From != "source" {
			t.Fatal(code, out, fixture.definition)
		}
	}
	before := fixture.calls
	for _, args := range [][]string{{"--max-image-size", "2TiB", file}, {"--builder", "../foreign", file}, {"--unknown"}, {}, {"--help"}} {
		code, _, _ := captureRun(t, append([]string{"base", "build"}, args...)...)
		expected := 2
		if len(args) > 0 && args[0] == "--help" {
			expected = 0
		}
		if code != expected || fixture.calls != before {
			t.Fatal("invalid definition reached controller", args, code)
		}
	}
}
