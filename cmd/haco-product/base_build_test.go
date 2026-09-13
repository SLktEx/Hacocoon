package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/basebuild"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type cliBaseBuildFixture struct{ calls []basebuild.Definition }

func (f *cliBaseBuildFixture) Build(_ context.Context, d basebuild.Definition) (basebuild.Result, error) {
	f.calls = append(f.calls, d)
	return basebuild.Result{State: "ready", Base: core.BaseInfo{Name: d.Name, Revision: core.BaseRevision("sha256:" + strings.Repeat("a", 64))}}, nil
}

func TestBaseBuildCLIOutputModeThroughController(t *testing.T) {
	setCLITestLocale(t, "en_US.UTF-8")
	fixture := &cliBaseBuildFixture{}
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
	path := filepath.Join(t.TempDir(), "base.json")
	if err := os.WriteFile(path, []byte(`{"name":"tools","run":"true"}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, machine := range []bool{false, true} {
		args := []string{"base", "build"}
		if machine {
			args = append(args, "--json")
		}
		args = append(args, path)
		code, stdout, stderr := captureRun(t, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("code=%d err=%s", code, stderr)
		}
		if machine {
			var result basebuild.Result
			if err := json.Unmarshal([]byte(stdout), &result); err != nil || result.State != "ready" || result.Base.Name != "tools" || result.Base.Revision == "" {
				t.Fatalf("missing machine receipt: %+v %v", result, err)
			}
		} else if json.Valid([]byte(stdout)) || !strings.Contains(stdout, "state: ready") {
			t.Fatalf("unexpected human output: %s", stdout)
		}
	}
	if len(fixture.calls) != 2 || fixture.calls[0] != fixture.calls[1] {
		t.Fatal("output mode changed build operation", fixture.calls)
	}
}

func TestReadBaseDefinitionIsBoundedAndStrict(t *testing.T) {
	for _, data := range []string{`{"name":"tools","run":"true"}`, `{"name":"tools","run":"true","privileged":true}`, `{"name":"tools","run":"true"} {}`, `{"name":"../tools","run":"true"}`, strings.Repeat("x", 300000)} {
		file := filepath.Join(t.TempDir(), "base.json")
		if err := os.WriteFile(file, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		_, err := readBaseDefinition(file)
		if (err == nil) != (data == `{"name":"tools","run":"true"}`) {
			t.Fatal(err)
		}
	}
	if _, err := readBaseDefinition(t.TempDir()); err == nil {
		t.Fatal("directory accepted")
	}
}
