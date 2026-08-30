package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
)

const trustedHostClientBinaryEnv = "HACO_TRUSTED_HOST_CLIENT_BINARY"

func hostCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: haco host <ensure|shell>: %w", core.ErrInvalidArgument)
	}
	switch args[0] {
	case "ensure":
		return ensureTrustedHostReady(ctx, app)
	case "shell":
		if err := ensureTrustedHostReady(ctx, app); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, trustedHostLoginWarning())
		return app.Runtime.ShellTrustedHost(ctx)
	default:
		return fmt.Errorf("unknown host command %q: %w", args[0], core.ErrInvalidArgument)
	}
}

func ensureTrustedHostReady(ctx context.Context, app *composition.App) error {
	if app == nil || app.Runtime == nil {
		return core.ErrInvalidArgument
	}
	clientBinary, err := trustedHostClientBinaryPath()
	if err != nil {
		return err
	}
	return app.Runtime.ProvisionTrustedHostClient(ctx, clientBinary, control.SocketPath())
}

func trustedHostClientBinaryPath() (string, error) {
	candidate := strings.TrimSpace(os.Getenv(trustedHostClientBinaryEnv))
	if candidate == "" {
		executable, err := os.Executable()
		if err != nil {
			return "", fmt.Errorf("resolve haco executable: %w", err)
		}
		if resolved, err := filepath.EvalSymlinks(executable); err == nil {
			executable = resolved
		}
		candidate = filepath.Join(filepath.Dir(executable), "haco-host")
	}
	if !filepath.IsAbs(candidate) {
		return "", fmt.Errorf("%s must be an absolute path: %w", trustedHostClientBinaryEnv, core.ErrInvalidArgument)
	}
	info, err := os.Lstat(candidate)
	if err != nil {
		return "", fmt.Errorf("trusted host client binary %q is unavailable: %w", candidate, err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o022 != 0 {
		return "", fmt.Errorf("trusted host client binary %q must be a non-writable regular file: %w", candidate, core.ErrIncompatibleState)
	}
	return candidate, nil
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
