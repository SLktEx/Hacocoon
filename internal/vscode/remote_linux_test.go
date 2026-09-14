//go:build linux

package vscode

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoteCommandRejectsInjection(t *testing.T) {
	for _, commit := range []string{"", "../escape", "a;touch /tmp/escape", strings.Repeat("a", 39)} {
		if _, err := RemoteCommand(commit); err == nil {
			t.Fatal("accepted commit")
		}
	}
	s, err := RemoteCommand(strings.Repeat("a", 40))
	if err != nil || strings.Contains(s, "/workspace") || !strings.Contains(s, "Stable-"+strings.Repeat("a", 40)) {
		t.Fatal(err)
	}
}

func TestGuestApplicationPreservesSettingsAndPinsExtensions(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js unavailable for guest helper integration")
	}
	if _, err := exec.LookPath("flock"); err != nil {
		t.Skip("flock unavailable")
	}
	home := t.TempDir()
	server := filepath.Join(t.TempDir(), "server")
	if err := os.MkdirAll(filepath.Join(server, "out"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(node, filepath.Join(server, "node")); err != nil {
		t.Fatal(err)
	}
	// A faithful CLI boundary fixture checks exact version and suppressed implicit
	// dependency resolution; all subprocess state stays in this disposable home.
	cli := `const fs=require('fs'), p=require('path').join(require('os').homedir(),'installed.json');
const lock=require('path').join(require('os').homedir(),'.vscode-server','.haco-apply.lock');
if(require('child_process').spawnSync('/usr/bin/flock',['-n',lock,'true']).status!==1)process.exit(5);
let installed={}; try {installed=JSON.parse(fs.readFileSync(p,'utf8'))}catch(e){if(e.code!=='ENOENT')throw e}
const a=process.argv.slice(2);
if(a.includes('--install-extension')) {
 if(!a.includes('--do-not-include-pack-dependencies')||!a.includes('--do-not-sync')||a.includes('--force'))process.exit(2);
 const value=a[a.indexOf('--install-extension')+1];
 if(value==='a.fail@1.0.0')process.exit(1);
 const [id,version]=value.split('@'); if(!version)process.exit(3);
 installed[id]=version; fs.writeFileSync(p,JSON.stringify(installed));
} else if(a.includes('--list-extensions'))for(const [id,v]of Object.entries(installed))console.log(id+'@'+v);
else process.exit(4);`
	if err := os.WriteFile(filepath.Join(server, "out", "server-main.js"), []byte(cli), 0600); err != nil {
		t.Fatal(err)
	}
	nodePath, _ := json.Marshal(filepath.Join(server, "node"))
	script := "process.execPath=" + string(nodePath) + ";\n" + applyScript
	run := func(settings map[string]any, install []Install) error {
		if install == nil {
			install = []Install{}
		}
		b, _ := json.Marshal(map[string]any{"settings": settings, "install": install})
		c := exec.Command(node, "-e", script)
		c.Dir = home
		c.Env = append(os.Environ(), "HOME="+home)
		c.Stdin = bytes.NewReader(b)
		out, err := c.Output()
		if err == nil && string(out) != "haco-vscode-applied\n" {
			t.Fatalf("invalid receipt %q", out)
		}
		return err
	}
	dir := filepath.Join(home, ".vscode-server", "data", "Machine")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(file, []byte(`{"existing":42}`), 0600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := run(map[string]any{"editor.formatOnSave": true, "__proto__": map[string]any{"polluted": true}}, []Install{{ID: "a.b", Version: "1.2.3"}}); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := os.ReadFile(file)
	if !strings.Contains(string(b), `"existing": 42`) || !strings.Contains(string(b), `"editor.formatOnSave": true`) || !strings.Contains(string(b), `"__proto__"`) {
		t.Fatal("lost settings")
	}
	if err := run(map[string]any{}, nil); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(file)
	if strings.Contains(string(b), "editor.formatOnSave") || strings.Contains(string(b), "__proto__") || !strings.Contains(string(b), "existing") {
		t.Fatal("managed deletion lost unrelated settings")
	}
	if err := run(map[string]any{}, []Install{{ID: "a.fail", Version: "1.0.0"}}); err == nil {
		t.Fatal("install failure ignored")
	}
	if err := os.WriteFile(file, []byte("// unmanaged JSONC\n{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run(map[string]any{}, nil); err == nil {
		t.Fatal("unparseable settings overwritten")
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(home, "repo-settings.json")
	if err := os.WriteFile(outside, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, file); err != nil {
		t.Fatal(err)
	}
	if err := run(map[string]any{"danger": true}, nil); err == nil {
		t.Fatal("symlink accepted")
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(outside, file); err != nil {
		t.Fatal(err)
	}
	if err := run(map[string]any{"danger": true}, nil); err == nil {
		t.Fatal("hardlink accepted")
	}
	b, _ = os.ReadFile(outside)
	if string(b) != "{}" {
		t.Fatal("repository modified")
	}
}
