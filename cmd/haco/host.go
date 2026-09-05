package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// Bootstrap now belongs to the controller-backed product command. Keep this
// dispatch guard so no legacy invocation can reconstruct local host setup.
func hostCommand(_ context.Context, _ *composition.App, args []string) error {
	if len(args) == 1 && args[0] == "shell" {
		return fmt.Errorf("host shell must use the controller: %w", core.ErrIncompatibleState)
	}
	if len(args) == 1 && args[0] == "ensure" {
		return fmt.Errorf("use haco setup: %w", core.ErrUnsupported)
	}
	return fmt.Errorf("use haco setup or the controller-backed host shell: %w", core.ErrInvalidArgument)
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
