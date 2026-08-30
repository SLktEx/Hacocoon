package main

import (
	"strings"
	"testing"
)

func TestIsHacocoonLoginRecognizesDedicatedInvocationName(t *testing.T) {
	for _, argv0 := range []string{"hacocoon-login", "/usr/local/libexec/hacocoon-login", "-hacocoon-login"} {
		if !isHacocoonLogin(argv0) {
			t.Fatalf("isHacocoonLogin(%q) = false", argv0)
		}
	}
	if isHacocoonLogin("haco") {
		t.Fatal("ordinary haco invocation was treated as WSL login")
	}
}

func TestPhysicalHostRecoveryMessageUsesWslDistroName(t *testing.T) {
	t.Setenv("WSL_DISTRO_NAME", "Hacocoon-Dev")
	message := physicalHostRecoveryMessage()
	if !strings.Contains(message, "wsl -d Hacocoon-Dev -u root") {
		t.Fatalf("message = %q", message)
	}
}

func TestPhysicalHostRecoveryMessageFallsBackToDefaultDistro(t *testing.T) {
	t.Setenv("WSL_DISTRO_NAME", "")
	message := physicalHostRecoveryMessage()
	if !strings.Contains(message, "wsl -d Hacocoon -u root") {
		t.Fatalf("message = %q", message)
	}
}

func TestTrustedHostWarningIsLocalized(t *testing.T) {
	t.Setenv("LC_ALL", "ja_JP.UTF-8")
	if warning := trustedHostLoginWarning(); !strings.Contains(warning, "特権管理環境") || !strings.Contains(warning, "Physical Host") {
		t.Fatalf("Japanese warning = %q", warning)
	}

	t.Setenv("LC_ALL", "C.UTF-8")
	if warning := trustedHostLoginWarning(); !strings.Contains(warning, "privileged management environment") || !strings.Contains(warning, "Physical Host") {
		t.Fatalf("English warning = %q", warning)
	}
}
