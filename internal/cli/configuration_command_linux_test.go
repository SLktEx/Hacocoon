//go:build linux

package cli

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/policy"
)

func configurationCommandFixture(t *testing.T) (*capability.PolicyConfiguration, string, string) {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("Python is required for the external editor fixture")
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	service := &capability.PolicyConfiguration{
		Evaluator: capability.NewFilePolicyEvaluator(filepath.Join(root, "policy.json")),
		Audit:     capability.NewJSONLAudit(filepath.Join(root, "audit.jsonl")),
	}
	server := control.NewServer()
	if err := controlapi.RegisterConfiguration(server, service); err != nil {
		t.Fatal(err)
	}
	socketDir, err := os.MkdirTemp("", "haco-config-cli-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
	socket := filepath.Join(socketDir, "control.sock")
	listener, err := control.ListenUnix(socket, 0600)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	t.Cleanup(func() { cancel(); <-done })
	t.Setenv("HACO_CONTROL_SOCKET", socket)
	setCLITestLocale(t, "C")
	t.Setenv("TMPDIR", t.TempDir())
	marker := filepath.Join(root, "editor-path")
	t.Setenv("HACO_CONFIG_EDITOR_PATH", marker)
	t.Setenv("HACO_CONFIG_EDITOR_MODE", "save")
	editor := filepath.Join(root, "editor with spaces.py")
	if err := os.WriteFile(editor, []byte(configurationEditorScript), 0600); err != nil {
		t.Fatal(err)
	}
	shellQuote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
	t.Setenv("VISUAL", shellQuote(python)+" "+shellQuote(editor))
	t.Setenv("EDITOR", "exit 99")
	return service, root, marker
}

const configurationEditorScript = `import json, os, pathlib, stat, sys
path = pathlib.Path(sys.argv[1])
assert len(sys.argv) == 2
assert stat.S_IMODE(path.stat().st_mode) == 0o600
assert stat.S_IMODE(path.parent.stat().st_mode) == 0o700
pathlib.Path(os.environ['HACO_CONFIG_EDITOR_PATH']).write_text(str(path))
snapshot = json.loads(path.read_text())
snapshot['policy'] = {'default': 'deny', 'rules': [
    {'capability':'local.echo', 'action':'echo', 'resource':'edited-marker',
     'environment':'*', 'decision':'deny'}]}
mode = os.environ['HACO_CONFIG_EDITOR_MODE']
print('editor status: review complete', flush=True)
if mode == 'invalid-json':
    path.write_text('retained invalid document')
elif mode == 'revision':
    snapshot['revision'] = 'sha256:' + 'f' * 64
    path.write_text(json.dumps(snapshot))
elif mode in ('symlink', 'hardlink', 'fifo'):
    target = path.parent / 'external.json'
    target.write_text(json.dumps(snapshot))
    path.unlink()
    if mode == 'symlink':
        path.symlink_to(target)
    elif mode == 'hardlink':
        os.link(target, path)
    else:
        os.mkfifo(path)
else:
    path.write_text(json.dumps(snapshot))
if mode == 'exit-error':
    sys.exit(7)
if mode == 'backup':
    (path.parent / 'editor.backup').write_text('retained user backup')
`

func configurationEditedPath(t *testing.T, marker string) string {
	t.Helper()
	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal("editor did not receive its input document", err)
	}
	return string(data)
}

func TestConfigurationEditorKeepsJSONOutputParseable(t *testing.T) {
	service, root, marker := configurationCommandFixture(t)
	before, err := service.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := captureRun(t, "config", "--edit", "--json")
	var receipt capability.PolicySnapshot
	if code != 0 || json.Unmarshal([]byte(stdout), &receipt) != nil || receipt.Revision == before.Revision || !strings.Contains(string(receipt.Policy), "edited-marker") {
		t.Fatalf("editor corrupted the machine result: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "editor status: review complete") {
		t.Fatal("editor feedback was lost", stderr)
	}
	after, err := service.Snapshot(context.Background())
	if err != nil || after.Revision != receipt.Revision || !strings.Contains(string(after.Policy), "edited-marker") {
		t.Fatal("saved receipt does not identify current Policy", after, err)
	}
	path := configurationEditedPath(t, marker)
	if _, err := os.Lstat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatal("successful edit left its temporary directory", err)
	}
	audit, err := os.ReadFile(filepath.Join(root, "audit.jsonl"))
	if err != nil || strings.Count(string(audit), `"type":"configuration-changed"`) != 1 || strings.Contains(string(audit), "edited-marker") {
		t.Fatal("save was replayed or audit exposed policy content", string(audit), err)
	}
}

func TestConfigurationEditorFailuresPreserveWorkWithoutApplying(t *testing.T) {
	for _, mode := range []string{"invalid-json", "revision", "exit-error", "symlink", "hardlink", "fifo"} {
		t.Run(mode, func(t *testing.T) {
			service, root, marker := configurationCommandFixture(t)
			t.Setenv("HACO_CONFIG_EDITOR_MODE", mode)
			before, err := service.Snapshot(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			code, stdout, stderr := captureRun(t, "config", "--edit", "--json")
			path := configurationEditedPath(t, marker)
			if code != 1 || stdout != "" || !strings.Contains(stderr, path) {
				t.Fatalf("failed edit was acknowledged or location lost: code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			if _, err := os.Lstat(path); err != nil {
				t.Fatal("failed editor work was deleted", err)
			}
			after, err := service.Snapshot(context.Background())
			if err != nil || after.Revision != before.Revision || string(after.Policy) != string(before.Policy) {
				t.Fatal("invalid edit changed current Policy", after, err)
			}
			if _, err := os.Lstat(filepath.Join(root, "audit.jsonl")); !os.IsNotExist(err) {
				t.Fatal("invalid editor output reached replacement", err)
			}
		})
	}
}

func TestConfigurationEditorSuccessRetainsUserBackup(t *testing.T) {
	_, _, marker := configurationCommandFixture(t)
	t.Setenv("HACO_CONFIG_EDITOR_MODE", "backup")
	code, stdout, stderr := captureRun(t, "config", "--edit", "--json")
	path := configurationEditedPath(t, marker)
	if code != 0 || !json.Valid([]byte(stdout)) || !strings.Contains(stderr, filepath.Dir(path)) {
		t.Fatal("successful save did not report retained editor files", code, stdout, stderr)
	}
	if data, err := os.ReadFile(filepath.Join(filepath.Dir(path), "editor.backup")); err != nil || string(data) != "retained user backup" {
		t.Fatal("cleanup removed the editor's backup", string(data), err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatal("successful edit retained the submitted document", err)
	}
}

func TestConfigurationEditorSelectionFallbacks(t *testing.T) {
	for _, selection := range []string{"EDITOR", "vi"} {
		t.Run(selection, func(t *testing.T) {
			_, _, marker := configurationCommandFixture(t)
			selected := os.Getenv("VISUAL")
			t.Setenv("VISUAL", "")
			if selection == "EDITOR" {
				t.Setenv("EDITOR", selected)
			} else {
				t.Setenv("EDITOR", "")
				bin := t.TempDir()
				if err := os.WriteFile(filepath.Join(bin, "vi"), []byte("#!/bin/sh\nexec "+selected+" \"$@\"\n"), 0700); err != nil {
					t.Fatal(err)
				}
				t.Setenv("PATH", bin)
			}
			code, stdout, stderr := captureRun(t, "config", "--edit", "--json")
			if code != 0 || !json.Valid([]byte(stdout)) || !strings.Contains(stderr, "editor status: review complete") {
				t.Fatal("documented fallback editor did not run", code, stdout, stderr)
			}
			configurationEditedPath(t, marker)
		})
	}
}

func TestConfigurationCanceledEditDoesNotLaunchEditor(t *testing.T) {
	service, _, marker := configurationCommandFixture(t)
	snapshot, err := service.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, retained, err := editConfiguration(ctx, snapshot)
	if err == nil || retained == "" {
		t.Fatal("canceled editor was accepted or recovery document lost", retained, err)
	}
	if _, err := os.Stat(retained); err != nil {
		t.Fatal("canceled edit deleted its recovery document", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("canceled edit launched the external editor", err)
	}
}
