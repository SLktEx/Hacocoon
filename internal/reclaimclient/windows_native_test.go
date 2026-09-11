//go:build windows

package reclaimclient

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

func TestNativePowerShellReclamationProtocol(t *testing.T) {
	system, err := windows.GetSystemDirectory()
	if err != nil {
		t.Fatal(err)
	}
	powershell := filepath.Join(system, "WindowsPowerShell", "v1.0", "powershell.exe")
	for _, mode := range []string{"start", "status", "prepare-refused"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			helper := filepath.Join(root, "Hacocoon", "reclamation", strings.ReplaceAll(strings.Trim(commandReclaimTarget.RegistrationID, "{}"), "-", ""), "haco-wsl.exe")
			if err := os.MkdirAll(filepath.Dir(helper), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(helper, []byte("non-executable protocol fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			action := mode
			if action == "prepare-refused" {
				action = "start"
			}
			script, err := windowsReclaimScript(commandReclaimTarget, action)
			if err != nil {
				t.Fatal(err)
			}
			quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
			// Replace only known-folder discovery to keep the actual script's command and
			// JSON/exit handling on an isolated ordinary-file fixture.
			script = strings.Replace(script, "[Environment]::GetFolderPath('LocalApplicationData')", quote(root), 1)
			preparation := `'{"operation":"{33333333-3333-4333-8333-333333333333}","state":"pending"}';$global:LASTEXITCODE=0`
			if mode == "prepare-refused" {
				preparation = `$global:LASTEXITCODE=1;return`
			}
			prefix := `Set-Item -LiteralPath ('function:'+` + quote(helper) + `) -Value {switch($args[0]){'_prepare'{` + preparation + `}'_launch'{if($args[1] -ine '` + commandReclaimTarget.RegistrationID + `' -or $args[2] -ine '{33333333-3333-4333-8333-333333333333}'){throw 'changed identity'};'Dispatched Windows worker 42; inspect the prepared operation for completion.';$global:LASTEXITCODE=0}'_status'{'{"operation":"{33333333-3333-4333-8333-333333333333}","state":"pending"}';$global:LASTEXITCODE=0}default{throw 'unexpected command'}}};`
			text := utf16.Encode([]rune(prefix + script))
			raw := make([]byte, len(text)*2)
			for i, v := range text {
				binary.LittleEndian.PutUint16(raw[i*2:], v)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, powershell, "-NoProfile", "-NonInteractive", "-EncodedCommand", base64.StdEncoding.EncodeToString(raw))
			command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			out, err := command.Output()
			if mode == "prepare-refused" {
				if err == nil || len(out) != 0 {
					t.Fatal("refused preparation advanced", string(out), err)
				}
				return
			}
			if err != nil {
				if e, ok := err.(*exec.ExitError); ok {
					t.Fatalf("PowerShell protocol: %v: %s", err, e.Stderr)
				}
				t.Fatal(err)
			}
			var result map[string]any
			if err := json.Unmarshal(out, &result); err != nil {
				t.Fatal(string(out), err)
			}
			if result["operation"] != "{33333333-3333-4333-8333-333333333333}" {
				t.Fatal(result)
			}
			if mode == "start" && result["worker_pid"] != float64(42) {
				t.Fatal(result)
			}
			if mode == "status" && result["state"] != "pending" {
				t.Fatal(result)
			}
		})
	}
}
