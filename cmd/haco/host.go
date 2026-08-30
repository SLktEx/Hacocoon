package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func hostCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: haco host <ensure|shell>: %w", core.ErrInvalidArgument)
	}
	switch args[0] {
	case "ensure":
		return app.Runtime.EnsureTrustedHost(ctx)
	case "shell":
		if err := app.Runtime.EnsureTrustedHost(ctx); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, trustedHostLoginWarning())
		return app.Runtime.ShellTrustedHost(ctx)
	default:
		return fmt.Errorf("unknown host command %q: %w", args[0], core.ErrInvalidArgument)
	}
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
