// Package packer runs the real Packer inside an ordinary owned Environment.
// It has only a lease-bound execution callback, no Host process/provider API.
package packer

import (
	"context"
	_ "embed"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/basebuild"
	"github.com/SLktEx/Hacocoon/internal/core"
)

//go:embed prepare.py
var prepare string

//go:embed dependencies.sh
var dependencies string

const Directory = "/run/hacocoon/packer"

type Runner struct{}

func (Runner) Provision(ctx context.Context, template basebuild.PackerTemplate, execute basebuild.Execute) error {
	if err := template.Validate(); err != nil {
		return err
	}
	input, err := json.Marshal(template)
	if err != nil || len(input) > core.MaxExecutionInputBytes {
		return core.ErrInvalidArgument
	}
	run := func(stage string, request core.ExecutionRequest) error {
		result, err := execute(ctx, request)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil || result.ExitCode != 0 {
			if err != nil && result.ExitCode == 0 {
				result.ExitCode = -1 // No successful guest exit was confirmed.
			}
			// Provider output is private and bounded even if a replacement adapter
			// supplies a larger result. Raw errors never enter the error/log chain.
			if len(result.Stdout) > 16384 {
				result.Stdout = result.Stdout[:16384]
				result.StdoutTruncated = true
			}
			if len(result.Stderr) > 16384 {
				result.Stderr = result.Stderr[:16384]
				result.StderrTruncated = true
			}
			return &basebuild.ProvisionFailure{Stage: stage, Execution: result}
		}
		return nil
	}
	if err := run("dependencies", core.ExecutionRequest{Argv: []string{"/bin/sh", "-eu", "-s"}, Stdin: []byte(dependencies)}); err != nil {
		return err
	}
	if err := run("prepare", core.ExecutionRequest{Argv: []string{"/usr/bin/python3", "-I", "-c", prepare}, Stdin: input}); err != nil {
		return err
	}
	for _, stage := range []string{"fmt", "init", "validate", "build"} {
		// All evaluation, plugins and shell-local/post-processors have guest-only
		// authority. Only the staged context is provided; no Host path is mounted.
		args := []string{"/bin/sh", "-eu", "-c", "set -a; . /run/hacocoon/packer/environment; set +a; exec /run/hacocoon/packer/bin/packer \"$@\"", "haco-packer", stage}
		if stage == "build" {
			args = append(args, "-color=false", "-on-error=abort")
		}
		args = append(args, ".")
		if err := run(stage, core.ExecutionRequest{WorkingDirectory: Directory + "/source", Argv: args}); err != nil {
			return err
		}
	}
	return nil
}
