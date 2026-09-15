package packer

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/basebuild"
	"github.com/SLktEx/Hacocoon/internal/core"
	"strings"
	"testing"
)

func TestPackerStagesUseOnlyGuestExecutionAndStopOnFailure(t *testing.T) {
	template := basebuild.PackerTemplate{Files: []basebuild.SourceFile{{Path: "base.pkr.hcl", Data: []byte("untrusted HCL")}, {Path: "setup.sh", Data: []byte("private script")}}}
	for fail := 0; fail < 7; fail++ {
		calls := 0
		err := (Runner{}).Provision(context.Background(), template, func(_ context.Context, r core.ExecutionRequest) (core.ExecutionResult, error) {
			calls++
			switch calls {
			case 1:
				if strings.Join(r.Argv, " ") != "/bin/sh -eu -s" || string(r.Stdin) != dependencies {
					t.Fatal("dependency setup escaped guest stdin")
				}
			case 2:
				var got basebuild.PackerTemplate
				if len(r.Argv) != 4 || r.Argv[0] != "/usr/bin/python3" || r.Argv[1] != "-I" || json.Unmarshal(r.Stdin, &got) != nil || string(got.Files[1].Data) != "private script" {
					t.Fatal("source escaped stdin")
				}
			default:
				stage := []string{"fmt", "init", "validate", "build"}[calls-3]
				if r.Argv[5] != stage || r.WorkingDirectory != Directory+"/source" || strings.Contains(strings.Join(r.Argv, " "), "untrusted") {
					t.Fatal("not real guest Packer stage", r.Argv)
				}
			}
			if calls == fail {
				return core.ExecutionResult{ExitCode: 3, Stderr: strings.Repeat("x", 20000)}, errors.New("private process token")
			}
			return core.ExecutionResult{}, nil
		})
		if fail == 0 {
			if err != nil || calls != 6 {
				t.Fatal(err, calls)
			}
			continue
		}
		var failure *basebuild.ProvisionFailure
		if !errors.As(err, &failure) || calls != fail || len(failure.Execution.Stderr) != 16384 || !failure.Execution.StderrTruncated || strings.Contains(err.Error(), "private") {
			t.Fatal("unsafe failure/result", err, calls)
		}
	}
}

func TestPackerCancellationAndUnconfirmedExitDoNotAdvance(t *testing.T) {
	template := basebuild.PackerTemplate{Files: []basebuild.SourceFile{{Path: "base.pkr.hcl", Data: []byte("source")}}}
	for _, canceled := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		calls := 0
		err := (Runner{}).Provision(ctx, template, func(context.Context, core.ExecutionRequest) (core.ExecutionResult, error) {
			calls++
			if canceled {
				cancel()
			}
			return core.ExecutionResult{}, errors.New("private provider detail")
		})
		cancel()
		if calls != 1 || strings.Contains(err.Error(), "private") {
			t.Fatal("failure advanced/leaked", err, calls)
		}
		if canceled {
			if !errors.Is(err, context.Canceled) {
				t.Fatal("cancellation lost", err)
			}
		} else {
			var failure *basebuild.ProvisionFailure
			if !errors.As(err, &failure) || failure.Execution.ExitCode != -1 || failure.Stage != "dependencies" {
				t.Fatal("unconfirmed exit presented as success", err)
			}
		}
	}
}
