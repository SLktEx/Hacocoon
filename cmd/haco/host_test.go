package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestHostCommandIsTopLevel(t *testing.T) {
	err := dispatch(context.Background(), nil, []string{"host"})
	if !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("error = %v", err)
	}
}

func TestHostCommandRejectsUnknownSubcommandBeforeAppAccess(t *testing.T) {
	err := hostCommand(context.Background(), nil, []string{"unknown"})
	if !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("error = %v", err)
	}
}

func TestTrustedHostWarningUsesJapaneseLocale(t *testing.T) {
	t.Setenv("LC_ALL", "ja_JP.UTF-8")
	warning := trustedHostLoginWarning()
	if !strings.Contains(warning, "特権管理環境") || !strings.Contains(warning, "Physical Host") {
		t.Fatalf("warning = %q", warning)
	}
}

func TestTrustedHostWarningDefaultsToEnglish(t *testing.T) {
	t.Setenv("LC_ALL", "C")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "")
	warning := trustedHostLoginWarning()
	if !strings.Contains(warning, "privileged management environment") || !strings.Contains(warning, "Physical Host") {
		t.Fatalf("warning = %q", warning)
	}
}

func TestIsHacocoonLoginAcceptsNormalAndLoginArgv0(t *testing.T) {
	for _, argv0 := range []string{
		"/usr/local/libexec/hacocoon-login",
		"-hacocoon-login",
	} {
		if !isHacocoonLogin(argv0) {
			t.Fatalf("expected login mode for %q", argv0)
		}
	}
	if isHacocoonLogin("/usr/local/bin/haco") {
		t.Fatal("normal haco invocation entered login mode")
	}
}

func TestWslLoginRecoveryErrorNamesPhysicalHostEscapeHatch(t *testing.T) {
	t.Setenv("WSL_DISTRO_NAME", "Hacocoon-Dev")
	err := wslLoginRecoveryError(errors.New("boom"))
	if !strings.Contains(err.Error(), "wsl -d Hacocoon-Dev -u root") {
		t.Fatalf("error = %q", err)
	}
}
