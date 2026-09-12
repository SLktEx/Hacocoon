package incus

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDNSSetupWaitsForManagerWithoutRetryingMutations(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux guest shell contract")
	}
	for _, mode := range []string{"ready", "timeout", "reload-failure", "haco-conflict", "haco-reuse"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			for _, dir := range []string{"bin", "etc/systemd/system", "usr/local/libexec", "usr/local/bin"} {
				if err := os.MkdirAll(filepath.Join(root, dir), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(root, "usr/local/libexec/hacocoon-dns.next"), []byte("fixture"), 0700); err != nil {
				t.Fatal(err)
			}
			if mode == "haco-conflict" {
				if err := os.WriteFile(root+"/usr/local/bin/haco", []byte("user-file"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "haco-reuse" {
				if err := os.Symlink(root+"/usr/local/libexec/hacocoon-dns", root+"/usr/local/bin/haco"); err != nil {
					t.Fatal(err)
				}
			}
			mock := `#!/bin/sh
case "$*" in
'show --property=Version --value')
 count=0; if test -f "$COUNT"; then count=$(cat "$COUNT"); fi
 count=$((count+1)); printf '%s' "$count" > "$COUNT"
 test "$MODE" != timeout && test "$count" -ge 3 ;;
'daemon-reload')
 printf '%s\n' reload >> "$TRACE"
 test "$MODE" != reload-failure ;;
'enable hacocoon-dns.service'|'restart hacocoon-dns.service'|'is-active --quiet hacocoon-dns.service') exit 0 ;;
*) exit 99 ;;
esac
`
			for name, data := range map[string]string{"systemctl": mock, "sleep": "#!/bin/sh\nexit 0\n"} {
				if err := os.WriteFile(filepath.Join(root, "bin", name), []byte(data), 0700); err != nil {
					t.Fatal(err)
				}
			}
			script := strings.ReplaceAll(environmentDNSSetup, "/etc/", root+"/etc/")
			script = strings.ReplaceAll(script, "/usr/local/libexec/", root+"/usr/local/libexec/")
			script = strings.ReplaceAll(script, "/usr/local/bin/", root+"/usr/local/bin/")
			cmd := exec.Command("/bin/sh", "-ec", script)
			cmd.Env = []string{"PATH=" + root + "/bin:/usr/bin:/bin", "MODE=" + mode, "COUNT=" + root + "/count", "TRACE=" + root + "/trace"}
			output, err := cmd.CombinedOutput()
			count, _ := os.ReadFile(root + "/count")
			trace, _ := os.ReadFile(root + "/trace")
			resolver, readErr := os.ReadFile(root + "/etc/resolv.conf")
			switch mode {
			case "haco-conflict":
				data, _ := os.ReadFile(root + "/usr/local/bin/haco")
				if err == nil || string(data) != "user-file" || len(trace) != 0 {
					t.Fatal("existing haco overwritten", err)
				}
			case "ready", "haco-reuse":
				if err != nil || string(count) != "3" || string(trace) != "reload\n" || readErr != nil || !strings.Contains(string(resolver), "nameserver 127.0.0.1") {
					t.Fatalf("setup failed: %v %s", err, output)
				}
				target, linkErr := os.Readlink(root + "/usr/local/bin/haco")
				if linkErr != nil || target != root+"/usr/local/libexec/hacocoon-dns" {
					t.Fatal(target, linkErr)
				}
			case "timeout":
				if err == nil || string(count) != "60" || len(trace) != 0 || !os.IsNotExist(readErr) || dnsSetupFailureStage(string(output)) != "manager" {
					t.Fatalf("timeout contract: %v %s", err, output)
				}
			case "reload-failure":
				if err == nil || string(count) != "3" || string(trace) != "reload\n" || !os.IsNotExist(readErr) || dnsSetupFailureStage(string(output)) != "reload" {
					t.Fatalf("mutation retried or failure hidden: %v %s", err, output)
				}
			}
		})
	}
}
