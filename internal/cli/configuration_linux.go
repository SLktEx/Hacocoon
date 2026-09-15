//go:build linux

package cli

import (
	"bytes"
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

	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/policy"
)

const maxConfigurationBytes = 2 << 20

type configurationClient interface {
	ReadConfiguration(context.Context) (capability.PolicySnapshot, error)
	ReplaceConfiguration(context.Context, capability.PolicySnapshot) (capability.PolicySnapshot, error)
}

func runConfiguration(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	client, err := controlapi.NewDefaultClient()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, cliMessage("error.controller"))
		return 1
	}
	return configurationCommand(ctx, client, args, os.Stdout, os.Stderr, editConfiguration)
}

func configurationCommand(ctx context.Context, client configurationClient, args []string, out, diagnostic io.Writer, edit func(context.Context, capability.PolicySnapshot) (capability.PolicySnapshot, string, error)) int {
	flags := flag.NewFlagSet("haco config", flag.ContinueOnError)
	configureCLIFlags(flags, diagnostic)
	interactive := flags.Bool("edit", false, cliMessage("detail.config_edit"))
	file := flags.String("file", "", cliMessage("detail.config_file"))
	jsonOutput := flags.Bool("json", false, cliMessage("flag.json"))
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 || (*interactive && *file != "") {
		flags.Usage()
		return 2
	}
	var snapshot capability.PolicySnapshot
	var err error
	retained := ""
	if *file != "" {
		var data []byte
		data, err = readConfigurationFile(*file)
		if err == nil {
			d := json.NewDecoder(bytes.NewReader(data))
			d.DisallowUnknownFields()
			if d.Decode(&snapshot) != nil || d.Decode(new(any)) != io.EOF {
				err = core.ErrInvalidArgument
			}
		}
	} else {
		snapshot, err = client.ReadConfiguration(ctx)
		if err == nil && *interactive {
			snapshot, retained, err = edit(ctx, snapshot)
		}
	}
	if err == nil && (*interactive || *file != "") {
		snapshot, err = client.ReplaceConfiguration(ctx, snapshot)
	}
	if err != nil {
		_, _ = fmt.Fprintln(diagnostic, cliMessage("config.unconfirmed"), err)
		if retained != "" {
			_, _ = fmt.Fprintln(diagnostic, cliMessage("config.edit_retained", displayCell(retained)))
		}
		return 1
	}
	if snapshot.Revision == "" || !json.Valid(snapshot.Policy) {
		_, _ = fmt.Fprintln(diagnostic, cliMessage("config.invalid_receipt"))
		if retained != "" {
			_, _ = fmt.Fprintln(diagnostic, cliMessage("config.edit_retained", displayCell(retained)))
		}
		return 1
	}
	if retained != "" {
		if err := removeConfigurationEdit(retained); err != nil {
			_, _ = fmt.Fprintln(diagnostic, cliMessage("config.files_retained", displayCell(filepath.Dir(retained))))
		}
	}
	if err := writeCLIResult(out, snapshot, *jsonOutput); err != nil {
		return 1
	}
	if !*jsonOutput {
		key := "config.inspect"
		if *interactive || *file != "" {
			key = "config.saved"
		}
		if _, err := fmt.Fprintln(out, cliMessage(key)); err != nil {
			return 1
		}
	}
	return 0
}

func readConfigurationFile(path string) ([]byte, error) {
	parent, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("cannot open configuration directory")
	}
	defer parent.Close()
	f, err := parent.OpenFile(filepath.Base(path), os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("cannot open regular configuration file")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, core.ErrInvalidArgument
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Nlink != 1 {
		return nil, core.ErrInvalidArgument
	}
	data, err := io.ReadAll(io.LimitReader(f, maxConfigurationBytes+1))
	if err != nil || len(data) > maxConfigurationBytes {
		return nil, core.ErrInvalidArgument
	}
	return data, nil
}

func editConfiguration(ctx context.Context, snapshot capability.PolicySnapshot) (capability.PolicySnapshot, string, error) {
	if snapshot.Revision == "" || !json.Valid(snapshot.Policy) {
		return snapshot, "", core.ErrInvalidArgument
	}
	directory, err := os.MkdirTemp("", "haco-config-")
	if err != nil {
		return snapshot, "", err
	}
	path := filepath.Join(directory, "configuration.json")
	formatted, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return snapshot, path, core.ErrInvalidArgument
	}
	if err := os.WriteFile(path, append(formatted, '\n'), 0600); err != nil {
		return snapshot, path, err
	}
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	// The operator's editor setting is trusted shell configuration. The generated
	// pathname is a separate positional argument, never interpolated as code.
	command := exec.CommandContext(ctx, "/bin/sh", "-c", "exec "+editor+" \"$1\"", "haco-config-editor", path)
	// Keep stdout available for the saved receipt, including --json output.
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stderr, os.Stderr
	if err := command.Run(); err != nil {
		return snapshot, path, fmt.Errorf("editor did not complete")
	}
	data, err := readConfigurationFile(path)
	if err != nil {
		return snapshot, path, err
	}
	var edited capability.PolicySnapshot
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&edited) != nil || decoder.Decode(new(any)) != io.EOF || edited.Revision != snapshot.Revision || !json.Valid(edited.Policy) {
		return snapshot, path, core.ErrInvalidArgument
	}
	return edited, path, nil
}

func removeConfigurationEdit(path string) error {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.Remove(filepath.Base(path)); err != nil {
		return err
	}
	// Never recursively remove editor backups or unexpected contents.
	return os.Remove(filepath.Dir(path))
}
