package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/basebuild"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type cliPackerFixture struct {
	calls []basebuild.Definition
	fail  bool
}

func (f *cliPackerFixture) Build(_ context.Context, d basebuild.Definition) (basebuild.Result, error) {
	f.calls = append(f.calls, d)
	if f.fail {
		return basebuild.Result{State: "failed", Stage: "validate", Execution: &core.ExecutionResult{ExitCode: 2, Stderr: "private-source\x1b[2J"}}, core.ErrRuntimeUnavailable
	}
	return basebuild.Result{State: "ready", Base: core.BaseInfo{Name: d.Name}}, nil
}

func TestPackerBuildCLITransfersFilesAndGatesPrivateFailureOutput(t *testing.T) {
	setCLITestLocale(t, "en_US.UTF-8")
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
	files := map[string]string{"base.pkr.hcl": "source \"null\" \"base\" {}\n", "setup.sh": "printf '%s' '$literal'\n", ".env": "must-not-transfer"}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, failed := range []bool{false, true} {
		fixture.fail = failed
		for _, machine := range []bool{false, true} {
			for _, output := range []bool{false, true} {
				args := []string{"base", "build", "--name", "tools", "--from", "haco/ubuntu-26.04", "--builder", "packer-tools"}
				if machine {
					args = append(args, "--json")
				}
				if output {
					args = append(args, "--output")
				}
				args = append(args, directory)
				code, stdout, stderr := captureRun(t, args...)
				if (code == 1) != failed || code > 1 {
					t.Fatalf("exit=%d stderr=%q", code, stderr)
				}
				if strings.Contains(stdout+stderr, "\x1b") {
					t.Fatal("guest terminal controls were displayed")
				}
				if strings.Contains(stdout+stderr, "private-source") != (failed && output) {
					t.Fatal("private output opt-in differs", stdout, stderr)
				}
				if machine {
					var receipt basebuild.Result
					if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
						t.Fatal(err)
					}
					if (receipt.Execution != nil) != (failed && output) {
						t.Fatal("private JSON receipt differs", receipt)
					}
					if failed && receipt.Stage != "validate" {
						t.Fatal("failure stage lost")
					}
				}
			}
		}
	}
	for _, got := range fixture.calls {
		want := basebuild.Definition{Name: "tools", From: "haco/ubuntu-26.04", BuilderName: "packer-tools", Packer: &basebuild.PackerTemplate{Files: []basebuild.SourceFile{{Path: "base.pkr.hcl", Data: []byte(files["base.pkr.hcl"])}, {Path: "setup.sh", Data: []byte(files["setup.sh"])}}}}
		if !reflect.DeepEqual(got, want) {
			t.Fatal("HCL/script bytes or hidden-file exclusion changed", got)
		}
	}
	code, _, _ := captureRun(t, "base", "build", "--name", "../invalid", directory)
	if code != 2 || len(fixture.calls) != 8 {
		t.Fatal("invalid request reached controller")
	}
	code, _, _ = captureRun(t, "base", "build", "--name", "tools", "--builder", "../tools", directory)
	if code != 2 || len(fixture.calls) != 8 {
		t.Fatal("invalid builder reached controller")
	}
}
