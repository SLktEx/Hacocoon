//go:build linux

package sshclient

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditorDiscoveryAndExtensionFailuresDoNotReportReady(t *testing.T) {
	for _, scenario := range []string{"installed", "missing-extension", "similarly-named-extension", "list-failed", "install-failed", "missing-editor", "canceled"} {
		t.Run(scenario, func(t *testing.T) {
			bin := filepath.Join(t.TempDir(), "editor with spaces")
			if err := os.Mkdir(bin, 0700); err != nil {
				t.Fatal(err)
			}
			calls := filepath.Join(t.TempDir(), "calls")
			t.Setenv("PATH", bin)
			t.Setenv("SSH_EDITOR_CALLS", calls)
			t.Setenv("SSH_EDITOR_SCENARIO", scenario)
			program := `#!/bin/sh
printf '%s\n' "$*" >> "$SSH_EDITOR_CALLS"
case "$1" in
  --list-extensions)
    test "$#" = 1 || exit 91
    case "$SSH_EDITOR_SCENARIO" in
      installed) printf 'other.extension\n MS-VSCODE-REMOTE.REMOTE-SSH \r\n';;
      similarly-named-extension) printf 'ms-vscode-remote.remote-ssh-extra\n';;
      list-failed) echo PRIVATE_EDITOR_DIAGNOSTIC >&2; exit 7;;
    esac;;
  --install-extension)
    test "$#" = 2 && test "$2" = ms-vscode-remote.remote-ssh || exit 92
    test "$SSH_EDITOR_SCENARIO" != install-failed || exit 8;;
  *) exit 93;;
esac
`
			path := filepath.Join(bin, "code")
			if scenario != "missing-editor" {
				if err := os.WriteFile(path, []byte(program), 0700); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if scenario == "canceled" {
				cancel()
			}
			selected, err := Editor(ctx, Desktop{Home: t.TempDir()})
			failed := scenario == "list-failed" || scenario == "install-failed" || scenario == "missing-editor" || scenario == "canceled"
			if failed {
				if err == nil || selected != "" || strings.Contains(err.Error(), "PRIVATE_EDITOR_DIAGNOSTIC") {
					t.Fatal("failed editor was reported ready or diagnostics leaked", selected, err)
				}
				if scenario == "canceled" && !errors.Is(err, context.Canceled) {
					t.Fatal("lost cancellation", err)
				}
				if scenario == "list-failed" || scenario == "install-failed" {
					var exit *exec.ExitError
					if !errors.As(err, &exit) {
						t.Fatal("lost editor exit result", err)
					}
				}
			} else if err != nil || selected != path {
				t.Fatal("selected another editor", selected, err)
			}
			observed, readErr := os.ReadFile(calls)
			if scenario == "missing-editor" || scenario == "canceled" {
				if !errors.Is(readErr, os.ErrNotExist) {
					t.Fatal("unexpected editor execution", string(observed), readErr)
				}
				return
			}
			if readErr != nil {
				t.Fatal(readErr)
			}
			want := "--list-extensions\n"
			if scenario != "installed" && scenario != "list-failed" {
				want += "--install-extension ms-vscode-remote.remote-ssh\n"
			}
			if string(observed) != want {
				t.Fatal("unexpected installation or argument order", string(observed))
			}
		})
	}
}

func TestDesktopDiscoveryDoesNotFallbackAfterWindowsFailure(t *testing.T) {
	t.Setenv("WSL_INTEROP", "")
	t.Setenv("WSL_DISTRO_NAME", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	desktop, err := ResolveDesktop(context.Background())
	if err != nil || desktop != (Desktop{Home: home}) {
		t.Fatal(desktop, err)
	}
	t.Setenv("HOME", "")
	if desktop, err := ResolveDesktop(context.Background()); err == nil || desktop.Home != "" {
		t.Fatal("missing home was guessed", desktop, err)
	}
	t.Setenv("HOME", home)
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	program := `#!/bin/sh
test "$#" = 4 && test "$1" = -NoProfile && test "$2" = -NonInteractive && test "$3" = -Command || exit 91
test "$4" = '[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new(); ConvertTo-Json -Compress $env:USERPROFILE' || exit 92
printf '%s' "$SSH_DESKTOP_OBSERVATION"
exit "$SSH_DESKTOP_EXIT"
`
	if err := os.WriteFile(filepath.Join(bin, "powershell.exe"), []byte(program), 0700); err != nil {
		t.Fatal(err)
	}
	for _, hint := range []string{"WSL_INTEROP", "WSL_DISTRO_NAME"} {
		t.Run(hint, func(t *testing.T) {
			t.Setenv(hint, "selected-wsl")
			t.Setenv("SSH_DESKTOP_EXIT", "0")
			t.Setenv("SSH_DESKTOP_OBSERVATION", `"D:\\Users\\名前 Space"`)
			desktop, err := ResolveDesktop(context.Background())
			if err != nil || desktop != (Desktop{Home: "/mnt/d/Users/名前 Space", NativeHome: `D:\Users\名前 Space`, Windows: true}) {
				t.Fatal("Windows profile projection changed", desktop, err)
			}
			for _, malformed := range []string{"", "not-json", `{}`, `"C:x"`, `"\\\\server\\profile"`, `"1:\\profile"`, `"C:\\Users\\bad\nname"`, `"C:\\Users\\bad\u0000name"`} {
				t.Setenv("SSH_DESKTOP_OBSERVATION", malformed)
				if desktop, err := ResolveDesktop(context.Background()); err == nil || desktop != (Desktop{}) {
					t.Fatal("invalid Windows profile fell back or was accepted", desktop, err)
				}
			}
			t.Setenv("SSH_DESKTOP_OBSERVATION", `"D:\\Users\\valid"`)
			t.Setenv("SSH_DESKTOP_EXIT", "7")
			if desktop, err := ResolveDesktop(context.Background()); err == nil || desktop != (Desktop{}) {
				t.Fatal("failed observation accepted partial output or Linux fallback", desktop, err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if desktop, err := ResolveDesktop(ctx); !errors.Is(err, context.Canceled) || desktop != (Desktop{}) {
				t.Fatal("Windows discovery lost cancellation", desktop, err)
			}
		})
	}
}
