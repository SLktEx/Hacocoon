package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func hostCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: haco host <ensure [--oci]|shell>: %w", core.ErrInvalidArgument)
	}
	if args[0] == "shell" {
		if len(args) != 1 {
			return fmt.Errorf("usage: haco host shell: %w", core.ErrInvalidArgument)
		}
		return fmt.Errorf("haco host shell must use the controller client path: %w", core.ErrIncompatibleState)
	}
	if args[0] != "ensure" {
		return fmt.Errorf("unknown host command %q: %w", args[0], core.ErrInvalidArgument)
	}
	withOCI, err := parseHostEnsureArgs(args[1:])
	if err != nil {
		return err
	}
	if app == nil || app.Runtime == nil {
		return core.ErrInvalidArgument
	}
	if err := ensureTrustedHostAndClient(ctx, app); err != nil {
		return err
	}
	if withOCI {
		return app.Runtime.EnsureTrustedHostOCI(ctx)
	}
	return nil
}

func parseHostEnsureArgs(args []string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}
	if len(args) == 1 && args[0] == "--oci" {
		return true, nil
	}
	return false, fmt.Errorf("usage: haco host ensure [--oci]: %w", core.ErrInvalidArgument)
}

func ensureTrustedHostAndClient(ctx context.Context, app *composition.App) error {
	if err := app.Runtime.EnsureTrustedHost(ctx); err != nil {
		return err
	}
	clientBinary, err := trustedHostClientBinary()
	if err != nil {
		return err
	}
	if err := app.Runtime.ProvisionTrustedHostClient(ctx, clientBinary); err != nil {
		return err
	}
	generalClientBinary, err := trustedHostGeneralClientBinary()
	if err != nil {
		return err
	}
	return app.Runtime.ProvisionTrustedHostGeneralClient(ctx, generalClientBinary)
}

func trustedHostClientBinary() (string, error) {
	executable, err := trustedHostGeneralClientBinary()
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(filepath.Dir(executable), "haco-host")
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve companion haco-host binary %q: %w", candidate, err)
	}
	return resolved, nil
}

func trustedHostGeneralClientBinary() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve haco executable: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return "", fmt.Errorf("resolve haco executable %q: %w", executable, err)
	}
	return resolved, nil
}

func trustedHostLoginWarning() string {
	locale := strings.ToLower(strings.TrimSpace(os.Getenv("LC_ALL")))
	if locale == "" {
		locale = strings.ToLower(strings.TrimSpace(os.Getenv("LC_MESSAGES")))
	}
	if locale == "" {
		locale = strings.ToLower(strings.TrimSpace(os.Getenv("LANG")))
	}
	if strings.HasPrefix(locale, "ja") {
		return "⚠ `haco-host` は特権管理環境です。Windows 側のファイルや WSL interop、各種認証情報へアクセスできる場合があります。\n" +
			"ここでの操作は Physical Host / Windows / 外部サービスへ影響する可能性があります。通常の作業には Environment を使用してください。"
	}
	return "⚠ `haco-host` is a privileged management environment. It may have access to Windows files, WSL interop, and various credentials.\n" +
		"Actions performed here may affect the Physical Host, Windows, or external services. Use an Environment for normal work."
}