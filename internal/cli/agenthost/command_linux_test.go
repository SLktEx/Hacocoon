//go:build linux

package agenthostcli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func commandExecutable(t *testing.T, directory, name, script string) string {
	t.Helper()
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nset -eu\n"+script), 0700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAgentPrepareLaunchesAgentsAtThePreparedWorkspace(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "launch-failure"}[fail], func(t *testing.T) {
			f := newCommandFixture(t)
			record := filepath.Join(f.home, "launch-args")
			t.Setenv("FIXTURE_ARGUMENTS", record)
			script := "printf '%s\\n' \"$@\" > \"$FIXTURE_ARGUMENTS\"\n"
			if fail {
				script += "exit 7\n"
			}
			editor := commandExecutable(t, f.home, "code with spaces", script)
			out, err := captureCommand(t, func() error {
				return dispatch(context.Background(), f.app, []string{"prepare", "--session", f.session, "--code", editor, "--json", f.workspace})
			})
			if (err != nil) != fail || f.backend.prepared != 1 || !strings.HasPrefix(out, "{") {
				t.Fatal("launch result lost prepared connection", out, err)
			}
			args, readErr := os.ReadFile(record)
			want := "--agents\n--folder-uri\n" + agentRemoteFolderURI(agentSSHAlias(f.session)) + "\n"
			if readErr != nil || string(args) != want {
				t.Fatal("editor arguments changed", string(args), readErr)
			}
			if _, err := f.app.AgentHosts.Lookup(context.Background(), f.session); err != nil || len(f.backend.revoked) != 0 {
				t.Fatal("editor failure revoked prepared environment", err)
			}
		})
	}
}

func TestAgentClientFilesystemUsesWSLTranslationsAsSingleArguments(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("WSL_DISTRO_NAME", "fixture-distro")
	t.Setenv("PATH", root)
	t.Setenv("FIXTURE_PROFILE", `C:\Users\name with spaces;literal`)
	t.Setenv("FIXTURE_HOME", filepath.Join(root, "windows user"))
	t.Setenv("FIXTURE_ARGUMENTS", filepath.Join(root, "args"))
	commandExecutable(t, root, "cmd.exe", "[ \"$#\" = 3 ] && [ \"$1\" = /C ] && [ \"$2\" = echo ] && [ \"$3\" = '%USERPROFILE%' ]\nprintf '%s\\r\\n' \"$FIXTURE_PROFILE\"\n")
	commandExecutable(t, root, "wslpath", "[ \"$#\" = 2 ]\nprintf '%s\\n' \"$@\" > \"$FIXTURE_ARGUMENTS\"\nif [ \"$1\" = -u ]; then printf '%s\\n' \"$FIXTURE_HOME\"; else printf 'D:\\\\keys\\\\identity with spaces\\r\\n'; fi\n")
	fs, err := resolveClientFilesystem(context.Background())
	if err != nil || !fs.WSL || fs.Home != os.Getenv("FIXTURE_HOME") {
		t.Fatal(fs, err)
	}
	args, err := os.ReadFile(os.Getenv("FIXTURE_ARGUMENTS"))
	if err != nil || string(args) != "-u\n"+os.Getenv("FIXTURE_PROFILE")+"\n" {
		t.Fatal(string(args), err)
	}
	key := filepath.Join(fs.Home, ".ssh", "identity with spaces")
	if value, err := identityForClientConfig(context.Background(), fs, key); err != nil || value != "~/.ssh/identity with spaces" {
		t.Fatal(value, err)
	}
	outside := filepath.Join(root, "outside")
	if value, err := identityForClientConfig(context.Background(), fs, outside); err != nil || value != "D:/keys/identity with spaces" {
		t.Fatal(value, err)
	}
	args, err = os.ReadFile(os.Getenv("FIXTURE_ARGUMENTS"))
	if err != nil || string(args) != "-w\n"+outside+"\n" {
		t.Fatal(string(args), err)
	}
	if value, err := identityForClientConfig(context.Background(), clientFilesystem{Home: fs.Home}, outside); err != nil || value != outside {
		t.Fatal(value, err)
	}
}

func TestAgentClientFilesystemRefusesUnavailableOrEmptyTranslations(t *testing.T) {
	for _, mode := range []string{"home", "profile-command", "profile-empty", "path-command", "path-empty", "identity-command", "identity-empty"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("HOME", root)
			t.Setenv("WSL_DISTRO_NAME", "fixture-distro")
			t.Setenv("PATH", root)
			profile, translation := "printf 'C:/Users/user\\n'\n", "printf '/mnt/c/Users/user\\n'\n"
			switch mode {
			case "home":
				t.Setenv("HOME", "")
			case "profile-command":
				profile = "exit 7\n"
			case "profile-empty":
				profile = "printf '\\r\\n'\n"
			case "path-command", "identity-command":
				translation = "exit 7\n"
			case "path-empty", "identity-empty":
				translation = "printf '\\n'\n"
			}
			commandExecutable(t, root, "cmd.exe", profile)
			commandExecutable(t, root, "wslpath", translation)
			if strings.HasPrefix(mode, "identity") {
				if value, err := identityForClientConfig(context.Background(), clientFilesystem{Home: filepath.Join(root, "home"), WSL: true}, filepath.Join(root, "key")); err == nil || value != "" {
					t.Fatal(value, err)
				}
			} else if fs, err := resolveClientFilesystem(context.Background()); err == nil || fs.Home != "" {
				t.Fatal(fs, err)
			}
		})
	}
}

func TestAgentReleaseReportsUnresolvedClientFilesystemAfterDeletingEnvironment(t *testing.T) {
	f := newCommandFixture(t)
	if _, err := f.prepare(t); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", "")
	for i := 0; i < 2; i++ {
		err := dispatch(context.Background(), f.app, []string{"release", "--session", f.session})
		if err == nil || f.backend.deleted != 1 || (i == 0 && !errors.Is(err, core.ErrRecoveryRequired)) {
			t.Fatal("release lost incomplete client cleanup", err, f.backend.deleted)
		}
	}
}
