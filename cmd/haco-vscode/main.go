//go:build linux

package main

import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/sshclient"
	"github.com/SLktEx/Hacocoon/pkg/clientadapter"
)

// The standalone adapter shares product SSH ownership and the controller API.
// It owns no listener, provider composition, key store or SSH config renderer.
func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	if err := runAdapter(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "haco-vscode:", err)
		os.Exit(1)
	}
}

func runAdapter(ctx context.Context, args []string) error {
	if len(args) > 0 && args[0] != "open" && args[0] != "delete" {
		return fmt.Errorf("usage: haco-vscode <open|delete> [options] <workspace>\nunknown command %q", args[0])
	}
	if len(args) == 0 || (args[0] != "open" && args[0] != "delete") {
		return fmt.Errorf("usage: haco-vscode <open|delete> [--name name] [--read-only] [--no-launch] <workspace>")
	}
	fs := flag.NewFlagSet("haco-vscode", flag.ContinueOnError)
	name := fs.String("name", "", "Environment name")
	readOnly := fs.Bool("read-only", false, "read-only Workspace")
	noLaunch := fs.Bool("no-launch", false, "prepare SSH only")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("exact Workspace path required")
	}
	path, err := filepath.Abs(fs.Arg(0))
	if err != nil {
		return err
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	if *name == "" {
		*name = defaultEnvironmentName(path)
	}
	adapter, err := clientadapter.NewController()
	if err != nil {
		return err
	}
	client, err := controlapi.NewDefaultClient()
	if err != nil {
		return err
	}
	desktop, err := sshclient.ResolveDesktop(ctx)
	if err != nil {
		return err
	}
	mode := clientadapter.ReadWrite
	if *readOnly {
		mode = clientadapter.ReadOnly
	}
	if args[0] == "delete" {
		status, err := client.EnvironmentStatus(ctx, *name)
		if err != nil {
			return err
		}
		if filepath.Clean(status.Environment.Workspace.Path) != path {
			return fmt.Errorf("Workspace binding mismatch")
		}
		if err = client.DeleteEnvironment(ctx, *name); err != nil {
			return err
		}
		return sshclient.Remove(ctx, desktop, *name, "")
	}
	if _, _, err = adapter.Ensure(ctx, clientadapter.EnsureRequest{Name: *name, WorkspacePath: path, AccessMode: mode}); err != nil {
		return err
	}
	alias, err := sshclient.Setup(ctx, client, desktop, *name)
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "SSH ready:", alias)
	if *noLaunch {
		return nil
	}
	editor, err := sshclient.Editor(ctx, desktop)
	if err != nil {
		return err
	}
	cmd := exec.Command(editor, "--folder-uri", "vscode-remote://ssh-remote+"+alias+"/workspace")
	if err = cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

var nonNameCharacter = regexp.MustCompile(`[^a-z0-9]+`)

func defaultEnvironmentName(workspace string) string {
	base := strings.Trim(nonNameCharacter.ReplaceAllString(strings.ToLower(filepath.Base(workspace)), "-"), "-")
	if base == "" {
		base = "workspace"
	}
	if len(base) > 38 {
		base = strings.Trim(base[:38], "-")
	}
	sum := sha256.Sum256([]byte(filepath.Clean(workspace)))
	return fmt.Sprintf("vscode-%s-%x", base, sum[:4])
}
