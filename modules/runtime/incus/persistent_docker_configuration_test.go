package incus

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPersistentDockerConfigurationPreservesOptionsAndRefusesUnsafeLayouts(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux guest configuration")
	}
	for _, mode := range []string{"empty", "options", "managed-active", "active", "conflict", "data", "symlink", "directory-link", "hardlink", "writable", "oversized", "malformed"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			directory := filepath.Join(root, "etc/docker")
			config := filepath.Join(directory, "daemon.json")
			if err := os.MkdirAll(directory, 0755); err != nil {
				t.Fatal(err)
			}
			content := ""
			permissions := os.FileMode(0600)
			switch mode {
			case "options":
				content = `{"log-driver":"local"}`
			case "managed-active":
				content = `{"data-root":"/var/lib/hacocoon-oci/docker","exec-root":"/run/docker","log-driver":"local"}`
			case "conflict":
				content = `{"data-root":"/other"}`
			case "symlink", "hardlink", "writable":
				content = `{}`
			case "oversized":
				content = strings.Repeat(" ", 8193)
			case "malformed":
				content = `{`
			}
			if mode == "writable" {
				permissions = 0666
			}
			if content != "" {
				if err := os.WriteFile(config, []byte(content), permissions); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "writable" {
				if err := os.Chmod(config, 0666); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "symlink" || mode == "hardlink" {
				other := filepath.Join(root, "keep")
				if err := os.Rename(config, other); err != nil {
					t.Fatal(err)
				}
				var err error
				if mode == "symlink" {
					err = os.Symlink(other, config)
				} else {
					err = os.Link(other, config)
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			if mode == "directory-link" {
				other := filepath.Join(root, "keep-directory")
				if err := os.Rename(directory, other); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(other, directory); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "data" {
				data := filepath.Join(root, "var/lib/docker")
				if err := os.MkdirAll(data, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(data, "keep"), []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			bin := filepath.Join(root, "bin")
			if err := os.Mkdir(bin, 0700); err != nil {
				t.Fatal(err)
			}
			stub := "#!/bin/sh\nexit 3\n"
			if mode == "active" || mode == "managed-active" {
				stub = "#!/bin/sh\nexit 0\n"
			}
			if err := os.WriteFile(filepath.Join(bin, "systemctl"), []byte(stub), 0700); err != nil {
				t.Fatal(err)
			}
			script := strings.ReplaceAll(strings.ReplaceAll(persistentDockerConfiguration, "/etc/", root+"/etc/"), "/var/lib/docker", root+"/var/lib/docker")
			script = strings.ReplaceAll(script, "info.st_uid != 0 or ", "") // fixture runs as non-root; production retains uid validation
			command := exec.Command("/bin/sh", "-ec", script)
			command.Env = []string{"PATH=" + bin + ":/usr/bin:/bin"}
			output, err := command.CombinedOutput()
			success := mode == "empty" || mode == "options" || mode == "managed-active"
			if (err == nil) != success {
				t.Fatalf("%s: %v %s", mode, err, output)
			}
			if success {
				data, err := os.ReadFile(config)
				if err != nil {
					t.Fatal(err)
				}
				var values map[string]any
				if json.Unmarshal(data, &values) != nil || values["data-root"] != "/var/lib/hacocoon-oci/docker" || values["exec-root"] != "/run/docker" {
					t.Fatal("managed roots missing")
				}
				if mode != "empty" && values["log-driver"] != "local" {
					t.Fatal("unrelated option lost")
				}
			} else if content != "" {
				after, err := os.ReadFile(config)
				if err != nil || string(after) != content {
					t.Fatal("rejected configuration changed")
				}
			}
		})
	}
}
