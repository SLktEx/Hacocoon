package incus

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPersistentOCIConfigurationWaitsForGuestManager(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux guest shell contract")
	}
	for _, mode := range []string{"absent", "active", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			tools := filepath.Join(root, "bin")
			if err := os.Mkdir(tools, 0700); err != nil {
				t.Fatal(err)
			}
			systemctl := `#!/bin/sh
set -eu
case "$*" in
  'show --property=Version --value')
    count=0; if test -f "$COUNT"; then count=$(cat "$COUNT"); fi
    count=$((count+1)); printf '%s' "$count" > "$COUNT"
    test "$MODE" != timeout && test "$count" -ge 3 ;;
  'is-active --quiet containerd') test "$MODE" = active ;;
  'stop containerd'|'daemon-reload'|'start containerd') printf '%s\n' "$*" >> "$TRACE" ;;
  *) exit 99 ;;
esac
`
			for name, content := range map[string]string{"systemctl": systemctl, "sleep": "#!/bin/sh\nexit 0\n"} {
				if err := os.WriteFile(filepath.Join(tools, name), []byte(content), 0700); err != nil {
					t.Fatal(err)
				}
			}
			script := strings.ReplaceAll(persistentOCIConfiguration, "/etc/", root+"/etc/")
			command := exec.Command("/bin/sh", "-c", script)
			command.Env = []string{"PATH=" + tools + ":/usr/bin:/bin", "MODE=" + mode, "COUNT=" + filepath.Join(root, "count"), "TRACE=" + filepath.Join(root, "trace")}
			output, err := command.CombinedOutput()
			config, readErr := os.ReadFile(filepath.Join(root, "etc/containerd/config.toml"))
			if mode == "timeout" {
				if err == nil || !strings.Contains(string(output), "did not become ready") || !os.IsNotExist(readErr) {
					t.Fatalf("timeout must fail before configuration: %v %s read=%v", err, output, readErr)
				}
				return
			}
			if err != nil || readErr != nil {
				t.Fatalf("configure: %v %s read=%v", err, output, readErr)
			}
			if !strings.Contains(string(config), `root = "/var/lib/hacocoon-oci/containerd"`) || !strings.Contains(string(config), `state = "/run/containerd"`) {
				t.Fatal("persistent/transient roots differ from contract")
			}
			trace, err := os.ReadFile(filepath.Join(root, "trace"))
			if err != nil {
				t.Fatal(err)
			}
			expected := "daemon-reload\n"
			if mode == "active" {
				expected = "stop containerd\ndaemon-reload\nstart containerd\n"
			}
			if string(trace) != expected {
				t.Fatalf("runtime sequence: %q", trace)
			}
			count, err := os.ReadFile(filepath.Join(root, "count"))
			if err != nil || string(count) != "3" {
				t.Fatalf("readiness probe: %q %v", count, err)
			}
		})
	}
}
