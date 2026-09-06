package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/SLktEx/Hacocoon/internal/buildinfo"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/terminalbridge"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
	"golang.org/x/term"
)

const (
	loginAlias = "hacocoon-login"

	trustedHostNoticeEnglish  = "Entering trusted haco-host. Host authority is available here; use an Environment for ordinary development work."
	trustedHostNoticeJapanese = "信頼済みの haco-host に入ります。ここでは Host 権限を利用できます。通常の開発作業には Environment を使用してください。"
)

// Cold WSL starts Incus before the controller. A populated Incus installation
// can exceed 30 seconds; bound startup without delaying an already-ready host.
const controllerStartupTimeout = 2 * time.Minute

func main() {
	if filepath.Base(os.Args[0]) == "git-remote-haco" {
		if err := gitrepo.Helper(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr, gitrepo.UnixExchange(gitrepo.GuestSocket)); err != nil {
			fmt.Fprintln(os.Stderr, "git-remote-haco:", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) == 2 && os.Args[1] == "_git-agent" {
		if err := gitrepo.Agent(context.Background(), os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "haco: invalid trusted Git operation")
			os.Exit(1)
		}
		return
	}
	if isLoginAlias(os.Args[0]) {
		if err := runLoginShim(os.Args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "haco:", err)
			os.Exit(1)
		}
		return
	}

	code := run(os.Args[1:])
	if code != 0 {
		os.Exit(code)
	}
}

func run(args []string) int {
	if len(args) == 0 {
		writeHelp(os.Stdout)
		return 0
	}

	switch args[0] {
	case "--help", "-h", "help":
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "haco: usage: haco help")
			return 2
		}
		writeHelp(os.Stdout)
		return 0
	case "--version":
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "haco: usage: haco --version")
			return 2
		}
		writeShortVersion()
		return 0
	case "version":
		return runVersion(args[1:])
	case "setup":
		return runSetup(args[1:])
	case "doctor":
		return runDoctor(args[1:])
	case "env":
		return runEnvironment(args[1:])
	case "repo", "workspace", "git":
		return runRepository(args[0], args[1:])
	default:
		fmt.Fprintf(os.Stderr, "haco: command %q is not available yet; run 'haco help'\n", args[0])
		return 2
	}
}

func runVersion(args []string) int {
	info := buildinfo.Current()
	if len(args) == 0 {
		fmt.Printf("Hacocoon\n  checkpoint: %s\n  version: %s\n  commit: %s\n  built: %s\n",
			info.Checkpoint, info.Version, info.Commit, info.BuildDate)
		return 0
	}
	if len(args) == 1 && args[0] == "--json" {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(info); err != nil {
			fmt.Fprintln(os.Stderr, "haco:", err)
			return 1
		}
		return 0
	}
	fmt.Fprintln(os.Stderr, "haco: usage: haco version [--json]")
	return 2
}

func writeShortVersion() {
	info := buildinfo.Current()
	fmt.Printf("haco %s (checkpoint %s, commit %s)\n", info.Version, info.Checkpoint, buildinfo.ShortCommit(info.Commit))
}

func writeHelp(out *os.File) {
	fmt.Fprintln(out, "Hacocoon")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  haco <command>")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Commands:")
	fmt.Fprintln(out, "  setup      Prepare the installed Host through its controller")
	fmt.Fprintln(out, "  doctor     Diagnose the Physical Host through its controller")
	fmt.Fprintln(out, "  env        Create, inspect and access development Environments")
	fmt.Fprintln(out, "  repo       Clone a repository inside the trusted Host")
	fmt.Fprintln(out, "  workspace  Prepare an independent managed repository copy")
	fmt.Fprintln(out, "  git        Connect Git and review pending push approvals")
	fmt.Fprintln(out, "  help       Show this help")
	fmt.Fprintln(out, "  version    Show Hacocoon version information")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "The product CLI is being rebuilt from the basic workflow outward.")
}

func isLoginAlias(argv0 string) bool {
	name := strings.TrimPrefix(filepath.Base(argv0), "-")
	return name == loginAlias
}

func runLoginShim(args []string) error {
	// Explicit shell arguments and non-interactive callers stay on the WSL
	// Physical Host. Only a normal interactive distro entry is redirected into
	// the trusted haco-host.
	if len(args) != 0 || !stdioIsInteractive() {
		return execProcess("/bin/bash", append([]string{"bash"}, args...))
	}

	client, err := controlapi.NewDefaultClient()
	if err != nil {
		return fmt.Errorf("open Hacocoon controller client: %w", err)
	}
	ctx := context.Background()
	readyCtx, cancelReady := context.WithTimeout(ctx, controllerStartupTimeout)
	defer cancelReady()
	if err := waitForController(readyCtx, func(ctx context.Context) error {
		_, err := client.Ping(ctx)
		return err
	}); err != nil {
		return fmt.Errorf("wait for Physical Host controller: %w", err)
	}
	stream, err := client.OpenTrustedHostShell(ctx)
	if err != nil {
		return fmt.Errorf("enter trusted haco-host: %w", err)
	}
	defer stream.Close()
	writeTrustedHostNotice(os.Stderr)
	return terminalbridge.Bridge(ctx, stream, os.Stdin, os.Stdout)
}

func writeTrustedHostNotice(out *os.File) {
	useColor := term.IsTerminal(int(out.Fd())) && os.Getenv("NO_COLOR") == ""
	fmt.Fprintln(out, formatTrustedHostNotice(trustedHostNotice(), useColor))
}

func trustedHostNotice() string {
	if isJapaneseLocale(currentMessagesLocale()) {
		return trustedHostNoticeJapanese
	}
	return trustedHostNoticeEnglish
}

func currentMessagesLocale() string {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func isJapaneseLocale(locale string) bool {
	locale = strings.ToLower(strings.TrimSpace(locale))
	if separator := strings.IndexAny(locale, ".@"); separator >= 0 {
		locale = locale[:separator]
	}
	return locale == "ja" || strings.HasPrefix(locale, "ja_") || strings.HasPrefix(locale, "ja-")
}

func formatTrustedHostNotice(message string, useColor bool) string {
	if !useColor {
		return message
	}
	return "\x1b[33m" + message + "\x1b[0m"
}

// WSL can start a login shell before its enabled systemd controller is ready.
// Retry only transport unavailability through a read-only probe; never create
// another controller, restart services, or retry a rejected host operation.
func waitForController(ctx context.Context, ping func(context.Context) error) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := ping(ctx); !errors.Is(err, control.ErrUnavailable) {
			return err
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func stdioIsInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func execProcess(path string, argv []string) error {
	if err := syscall.Exec(path, argv, os.Environ()); err != nil {
		return fmt.Errorf("exec %s: %w", path, err)
	}
	return nil
}
