package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/basebuild"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
	"os"
	"time"
)

func readBaseDefinition(path string) (basebuild.Definition, error) {
	var d basebuild.Definition
	before, err := os.Stat(path)
	if err != nil {
		return d, err
	}
	if !before.Mode().IsRegular() {
		return d, core.ErrInvalidArgument
	}
	file, err := os.Open(path)
	if err != nil {
		return d, err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return d, err
	}
	if !stat.Mode().IsRegular() || stat.Size() > 4*basebuild.MaxScriptBytes {
		return d, core.ErrInvalidArgument
	}
	decoder := json.NewDecoder(io.LimitReader(file, 4*basebuild.MaxScriptBytes+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&d); err != nil {
		return d, core.ErrInvalidArgument
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return d, core.ErrInvalidArgument
	}
	return d, d.Validate()
}
func runBaseBuild(args []string) int {
	clean, jsonOutput, flagErr := splitJSONFlag(args)
	if flagErr != nil {
		fmt.Fprintln(os.Stderr, "haco:", flagErr)
		return 2
	}
	args = clean
	if len(args) != 1 || args[0] == "--help" {
		fmt.Fprintln(os.Stderr, "Usage: haco base build <definition.json> [--json]")
		if len(args) == 1 && args[0] == "--help" {
			return 0
		}
		return 2
	}
	d, err := readBaseDefinition(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco:", err)
		return 2
	}
	client, err := controlapi.NewDefaultClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco:", err)
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	result, err := client.BuildBase(ctx, d)
	if e := writeCLIResult(os.Stdout, result.Result, jsonOutput); e != nil {
		return 1
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco:", err)
		return 1
	}
	return 0
}
