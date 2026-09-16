//go:build linux

package sshclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/client/vscode"
	"github.com/SLktEx/Hacocoon/internal/experimental"
)

// OpenVSCode is the shared ordinary/standalone adapter path. Configuration is
// read in the trusted invoking Host; only the selected subtree reaches the Env.
func OpenVSCode(ctx context.Context, d Desktop, alias string, launch func(string) error) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	store, err := experimental.DefaultStore()
	if err != nil {
		return err
	}
	snap, err := store.Read(ctx)
	if err != nil {
		return err
	}
	editor, err := Editor(ctx, d)
	if err != nil {
		return err
	}
	if !snap.Present {
		return launch(editor)
	}
	cfg, err := experimental.VSCodeFromObject(snap.Object)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(alias, "haco-") || !namePattern.MatchString(strings.TrimPrefix(alias, "haco-")) {
		return fmt.Errorf("invalid SSH alias")
	}
	ssh := "ssh"
	if d.Windows {
		ssh = "ssh.exe"
	}
	remote := func(command string, stdin io.Reader) ([]byte, error) {
		c := exec.CommandContext(ctx, ssh, "-T", "-o", "BatchMode=yes", alias, command)
		c.Stdin = stdin
		var out boundedOutput
		c.Stdout = &out
		if err := c.Run(); err != nil {
			return nil, fmt.Errorf("VS Code Env preparation failed; inspect Remote-SSH readiness, settings JSON and extension network access")
		}
		return out.Bytes(), nil
	}
	arch, err := remote("uname -m", nil)
	if err != nil {
		return err
	}
	platform := ""
	switch strings.TrimSpace(string(arch)) {
	case "x86_64":
		platform = "linux-x64"
	case "aarch64":
		platform = "linux-arm64"
	default:
		return fmt.Errorf("unsupported VS Code Env architecture")
	}
	install, err := vscode.Resolve(ctx, vscode.Gallery{}, cfg, platform, time.Now().UTC())
	if err != nil {
		return err
	}
	var version *exec.Cmd
	if d.Windows {
		cli, err := projectedWindowsPath(filepath.Join(filepath.Dir(editor), "bin", "code.cmd"))
		if err != nil {
			return err
		}
		version = windowsEditorArgs(ctx, cli, []string{"--version"})
	} else {
		version = exec.CommandContext(ctx, editor, "--version")
	}
	var output boundedOutput
	version.Stdout = &output
	if version.Run() != nil {
		return fmt.Errorf("cannot inspect VS Code version")
	}
	lines := strings.Fields(output.String())
	if len(lines) != 3 {
		return fmt.Errorf("invalid VS Code version response")
	}
	command, err := vscode.RemoteCommand(lines[1])
	if err != nil {
		return err
	}
	if cfg.Settings == nil {
		cfg.Settings = map[string]any{}
	}
	if install == nil {
		install = []vscode.Install{}
	}
	plan, err := json.Marshal(struct {
		Settings map[string]any   `json:"settings"`
		Install  []vscode.Install `json:"install"`
	}{cfg.Settings, install})
	if err != nil {
		return err
	}
	if err := launch(editor); err != nil {
		return err
	}
	result, err := remote(command, bytes.NewReader(plan))
	if err != nil {
		return err
	}
	if string(result) != "haco-vscode-applied\n" {
		return fmt.Errorf("invalid VS Code application receipt")
	}
	return nil
}

// Guest and editor diagnostics never become unbounded memory or raw Host logs.
type boundedOutput struct{ buffer bytes.Buffer }

func (b *boundedOutput) Bytes() []byte  { return b.buffer.Bytes() }
func (b *boundedOutput) String() string { return b.buffer.String() }

func (b *boundedOutput) Write(p []byte) (int, error) {
	if b.buffer.Len()+len(p) > 8192 {
		return 0, fmt.Errorf("client response exceeds size limit")
	}
	return b.buffer.Write(p)
}
