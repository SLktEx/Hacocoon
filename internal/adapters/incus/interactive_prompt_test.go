package incus

import (
	"os/exec"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestInteractiveShellWithPrompt(t *testing.T) {
	got := interactiveShellWithPrompt(
		[]string{"/bin/bash", "-l"},
		"prompt",
		"trusted-host",
		core.TerminalMetadata{Term: "xterm-256color", ColorTerm: "truecolor"},
	)
	want := []string{
		"/usr/bin/env",
		"HACO_SHELL_CONTEXT=trusted-host",
		"HACO_PS1=prompt",
		"PROMPT_COMMAND=PS1=$HACO_PS1",
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"/bin/bash",
		"-l",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("interactiveShellWithPrompt() = %#v, want %#v", got, want)
	}
}

func TestHostPresentationIsSessionOnlyAndCannotConfigureGuestLocale(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Incus shell execution requires Linux")
	}
	for _, context := range []string{"trusted-host", "environment"} {
		for _, language := range []string{"en", "ja", "", "ja;PATH=/tmp"} {
			argv := interactiveShellWithPrompt([]string{"/bin/sh", "-c", `printf '%s|%s|%s' "$HACO_UI_LANGUAGE" "$LANG" "$LC_ALL"`}, "prompt", context, core.TerminalMetadata{DisplayLanguage: language})
			cmd := exec.Command(argv[0], argv[1:]...)
			cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C.UTF-8", "LC_ALL=C"}
			output, err := cmd.CombinedOutput()
			want := "|C.UTF-8|C"
			if context == "trusted-host" && (language == "en" || language == "ja") {
				want = language + want
			}
			if err != nil || string(output) != want {
				t.Fatalf("%s %q: %q %v", context, language, output, err)
			}
		}
	}
}

func TestInteractiveShellWithPromptOmitsMissingTerminalIdentity(t *testing.T) {
	got := interactiveShellWithPrompt([]string{"/bin/bash"}, "prompt", "environment", core.TerminalMetadata{})
	for _, arg := range got {
		if strings.HasPrefix(arg, "TERM=") || strings.HasPrefix(arg, "COLORTERM=") {
			t.Fatalf("interactiveShellWithPrompt() unexpectedly injected %q", arg)
		}
	}
}

func TestTrustedHostPromptIsWarningStyleAndReadlineSafe(t *testing.T) {
	for _, fragment := range []string{
		`\[\e[1;33;41m\]`,
		"[HACO-HOST]",
		`\[\e[0m\]`,
		`\u@\h:\w\$ `,
	} {
		if !strings.Contains(trustedHostPrompt, fragment) {
			t.Fatalf("trustedHostPrompt %q does not contain %q", trustedHostPrompt, fragment)
		}
	}
}

func TestEnvironmentPromptIsGreenAndIncludesLogicalName(t *testing.T) {
	got := environmentPrompt("haco-demo")
	for _, fragment := range []string{
		`\[\e[1;30;42m\]`,
		"[HACO-ENV:demo]",
		`\[\e[0m\]`,
		`\u@\h:\w\$ `,
	} {
		if !strings.Contains(got, fragment) {
			t.Fatalf("environmentPrompt() = %q, missing %q", got, fragment)
		}
	}
}

func TestEnvironmentPromptSanitizesTerminalControlCharacters(t *testing.T) {
	got := environmentPrompt("haco-demo\x1b[31m")
	if strings.ContainsRune(got, '\x1b') {
		t.Fatalf("environmentPrompt() retained terminal escape: %q", got)
	}
	if !strings.Contains(got, "[HACO-ENV:demo??31m]") {
		t.Fatalf("environmentPrompt() did not sanitize label: %q", got)
	}
}

func TestSafePromptLabelUsesFallbackForEmptyValue(t *testing.T) {
	if got := safePromptLabel(""); got != "?" {
		t.Fatalf("safePromptLabel(\"\") = %q, want ?", got)
	}
}
