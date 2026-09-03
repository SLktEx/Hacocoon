package incus

import (
	"reflect"
	"strings"
	"testing"
)

func TestInteractiveShellWithPrompt(t *testing.T) {
	got := interactiveShellWithPrompt([]string{"/bin/bash", "-l"}, "prompt", "trusted-host")
	want := []string{
		"/usr/bin/env",
		"HACO_SHELL_CONTEXT=trusted-host",
		"HACO_PS1=prompt",
		"PROMPT_COMMAND=PS1=$HACO_PS1",
		"/bin/bash",
		"-l",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("interactiveShellWithPrompt() = %#v, want %#v", got, want)
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
