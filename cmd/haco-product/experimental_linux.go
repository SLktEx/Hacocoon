//go:build linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/SLktEx/Hacocoon/internal/experimental"
	"gopkg.in/yaml.v2"
)

func runExperimental(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	s, err := experimental.DefaultStore()
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco: cannot locate configuration")
		return 1
	}
	return experimentalCommand(ctx, s, args, os.Stdin, os.Stdout, os.Stderr, editVSCode)
}

func experimentalCommand(ctx context.Context, s experimental.Store, args []string, in io.Reader, out, diagnostic io.Writer, edit func(context.Context, []byte) ([]byte, string, error)) int {
	usage := "Usage: haco experimental edit vscode [--file <subtree.yaml> | --json [ - ]]\nExperimental: configuration compatibility is not guaranteed."
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(out, usage)
		return 0
	}
	if len(args) < 2 || args[0] != "edit" || args[1] != "vscode" {
		fmt.Fprintln(diagnostic, usage)
		return 2
	}
	f := flag.NewFlagSet("haco experimental edit vscode", flag.ContinueOnError)
	f.SetOutput(diagnostic)
	file := f.String("file", "", "replace experimental.vscode with a YAML subtree file")
	jsonMode := f.Bool("json", false, "print JSON; with -, replace from JSON stdin")
	f.Usage = func() { fmt.Fprintln(diagnostic, usage); f.PrintDefaults() }
	if err := f.Parse(args[2:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	stdin := f.NArg() == 1 && f.Arg(0) == "-" && *jsonMode
	if f.NArg() != 0 && !stdin || *file != "" && *jsonMode {
		fmt.Fprintln(diagnostic, usage)
		return 2
	}
	snap, err := s.Read(ctx)
	if err != nil {
		fmt.Fprintln(diagnostic, "haco:", err)
		return 1
	}
	if *jsonMode && !stdin {
		e := json.NewEncoder(out)
		e.SetIndent("", "  ")
		if e.Encode(snap.Object) != nil {
			return 1
		}
		return 0
	}
	var b []byte
	retained := ""
	switch {
	case stdin:
		b, err = readExperimentalInput(ctx, in)
		if err == nil && !json.Valid(b) {
			err = fmt.Errorf("stdin must be a JSON object")
		}
	case *file != "":
		b, err = readConfigurationFile(*file)
	default:
		b, err = yaml.Marshal(snap.Object)
		if err == nil {
			b, retained, err = edit(ctx, b)
		}
	}
	var object map[string]any
	if err == nil {
		object, err = experimental.DecodeObject(b)
	}
	if err == nil {
		err = s.Replace(ctx, snap.Revision, object)
	}
	if err != nil {
		fmt.Fprintln(diagnostic, "haco: experimental configuration was not acknowledged; inspect YAML before retrying:", err)
		if retained != "" {
			fmt.Fprintln(diagnostic, "Edited subtree retained:", retained)
		}
		return 1
	}
	if retained != "" {
		if removeConfigurationEdit(retained) != nil {
			fmt.Fprintln(diagnostic, "Editor files retained:", filepath.Dir(retained))
		}
	}
	// JSON apply has no stdout, so pipelines carry subtree data only.
	fmt.Fprintln(diagnostic, "Saved experimental.vscode:", s.Path)
	return 0
}

func readExperimentalInput(ctx context.Context, in io.Reader) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	type result struct {
		data []byte
		err  error
	}
	done := make(chan result, 1)
	go func() {
		b, err := io.ReadAll(io.LimitReader(in, experimental.MaxBytes+1))
		done <- result{b, err}
	}()
	select {
	case r := <-done:
		return r.data, r.err
	case <-ctx.Done():
		// CLI stdin is an os.File. Closing it interrupts a stalled pipe reader;
		// a producer that never sends EOF must not swallow SIGINT/SIGTERM.
		if closer, ok := in.(io.Closer); ok {
			_ = closer.Close()
		}
		return nil, ctx.Err()
	}
}

func editVSCode(ctx context.Context, data []byte) ([]byte, string, error) {
	dir, err := os.MkdirTemp("", "haco-vscode-")
	if err != nil {
		return nil, "", err
	}
	path := filepath.Join(dir, "vscode.yaml")
	if err := os.WriteFile(path, data, 0600); err != nil {
		return nil, path, err
	}
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}
	// Operator-owned shell configuration; the pathname is a separate argument.
	c := exec.CommandContext(ctx, "/bin/sh", "-c", "exec "+editor+" \"$1\"", "haco-experimental-editor", path)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stderr, os.Stderr
	if c.Run() != nil {
		return nil, path, fmt.Errorf("editor did not complete")
	}
	b, err := readConfigurationFile(path)
	return b, path, err
}
