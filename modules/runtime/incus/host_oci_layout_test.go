package incus

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFreshHostLayoutNeverHidesExistingDataOrConfig(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux Host shell")
	}
	for _, mode := range []string{"empty", "data", "config", "link", "file"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			data := filepath.Join(root, "var/lib/docker")
			config := filepath.Join(root, "etc/docker/daemon.json")
			if err := os.MkdirAll(filepath.Dir(data), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(config), 0700); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "empty":
				if err := os.Mkdir(data, 0700); err != nil {
					t.Fatal(err)
				}
			case "data":
				if err := os.Mkdir(data, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(data, "keep"), []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
			case "config":
				if err := os.WriteFile(config, []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
			case "link":
				if err := os.Symlink("/missing", data); err != nil {
					t.Fatal(err)
				}
			case "file":
				if err := os.WriteFile(data, []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			script := strings.ReplaceAll(strings.ReplaceAll(hostOCIEmptyLayout, "/var/lib/", root+"/var/lib/"), "/etc/", root+"/etc/")
			err := exec.Command("/bin/sh", "-ec", script).Run()
			if (mode == "empty") != (err == nil) {
				t.Fatalf("layout %s: %v", mode, err)
			}
			if mode == "data" {
				if content, err := os.ReadFile(filepath.Join(data, "keep")); err != nil || string(content) != "keep" {
					t.Fatal("existing data changed")
				}
			}
			if mode == "config" {
				if content, err := os.ReadFile(config); err != nil || string(content) != "keep" {
					t.Fatal("existing config changed")
				}
			}
		})
	}
}

func TestHostLayoutVerificationRejectsDrift(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux Host Python contract")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	for _, mode := range []string{"valid", "native", "wrong-snapshotter", "wrong-root", "malformed", "oversized", "missing", "symlink"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			containerd := root + "/etc/containerd/config.toml"
			docker := root + "/etc/docker/daemon.json"
			for _, path := range []string{containerd, docker} {
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
			}
			config := `version = 2
root = "/var/lib/hacocoon-oci/containerd"
state = "/run/containerd"
[grpc]
  address = "/run/containerd/containerd.sock"
`
			if mode == "native" || mode == "wrong-snapshotter" {
				config += "\n[[plugins.\"io.containerd.transfer.v1.local\".unpack_config]]\n  platform = \"linux/" + runtime.GOARCH + "\"\n  snapshotter = \"native\"\n"
				if mode == "wrong-snapshotter" {
					config = strings.ReplaceAll(config, "\"native\"", "\"overlayfs\"")
				}
			}
			if err := os.WriteFile(containerd, []byte(config), 0600); err != nil {
				t.Fatal(err)
			}
			content := `{"data-root":"/var/lib/hacocoon-oci/docker","exec-root":"/run/docker"}`
			switch mode {
			case "wrong-root":
				content = strings.ReplaceAll(content, "hacocoon-oci/docker", "other")
			case "malformed":
				content = "{"
			case "oversized":
				content += strings.Repeat(" ", 8193)
			}
			if mode == "symlink" {
				if err := os.Symlink(containerd, docker); err != nil {
					t.Fatal(err)
				}
			} else if mode != "missing" {
				if err := os.WriteFile(docker, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			script := strings.ReplaceAll(hostOCILayoutVerify, "/etc/", root+"/etc/")
			output, err := exec.Command(python, "-I", "-c", script).CombinedOutput()
			if (mode == "valid" || mode == "native") != (err == nil) {
				t.Fatalf("%s: %v %s", mode, err, output)
			}
			if len(output) != 0 {
				t.Fatalf("configuration contents must not be reflected: %q", output)
			}
		})
	}
}
