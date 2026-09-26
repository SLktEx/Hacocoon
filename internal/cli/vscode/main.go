//go:build linux

package vscodecli

import (
	"context"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/cli/ui"
	"github.com/SLktEx/Hacocoon/internal/client/ssh"
	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/pkg/clientadapter"
)

// The standalone adapter shares product SSH ownership and the controller API.
// It owns no listener, provider composition, key store or SSH config renderer.
func Main() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	if err := runAdapter(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "haco-vscode:", err)
		os.Exit(1)
	}
}

func runAdapter(ctx context.Context, args []string) error {
	return runAdapterWithIO(ctx, args, os.Stdout, os.Stderr, cliui.Resolve(os.Getenv))
}

func runAdapterWithIO(ctx context.Context, args []string, out, diagnostic io.Writer, language cliui.Language) error {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		writeAdapterHelp(out, "", language)
		return nil
	}
	if len(args) > 0 && args[0] != "open" && args[0] != "delete" {
		return fmt.Errorf("%s\n%s", language.Text("vscode.usage"), language.Format("vscode.unknown", args[0]))
	}
	if len(args) == 0 || (args[0] != "open" && args[0] != "delete") {
		writeAdapterHelp(diagnostic, "", language)
		return errors.New(language.Text("vscode.usage"))
	}
	fs := flag.NewFlagSet("haco-vscode", flag.ContinueOnError)
	fs.SetOutput(diagnostic)
	fs.Usage = func() {} // Route requested help and invalid input to different streams below.
	name := fs.String("name", "", language.Text("vscode.name"))
	readOnly := fs.Bool("read-only", false, language.Text("vscode.read_only"))
	noLaunch := fs.Bool("no-launch", false, language.Text("vscode.no_launch"))
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			writeAdapterHelp(out, args[0], language)
			return nil
		}
		writeAdapterHelp(diagnostic, args[0], language)
		return err
	}
	if fs.NArg() != 1 {
		return errors.New(language.Text("vscode.path_required"))
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
	adapter := clientadapter.NewController()
	client := controlapi.NewDefaultClient()
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
			return errors.New(language.Text("vscode.binding_mismatch"))
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
	if _, err := fmt.Fprintln(out, language.Text("vscode.ready"), alias); err != nil {
		return err
	}
	if *noLaunch {
		return nil
	}
	return sshclient.OpenVSCode(ctx, desktop, alias, func(editor string) error {
		cmd := exec.Command(editor, "--folder-uri", "vscode-remote://ssh-remote+"+alias+"/workspace")
		if err := cmd.Start(); err != nil {
			return err
		}
		return cmd.Process.Release()
	})
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
