//go:build linux

package sshclient

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderWindowsProxyCommand(t *testing.T) {
	c := &fakeController{}
	conn, err := c.PrepareEnvironmentSSH(context.Background(), "my-project", core.SSHAccessRequest{PublicKey: testKey})
	if err != nil {
		t.Fatal(err)
	}
	s := saved{Distro: "Hacocoon", Runtime: "owned", Connection: conn}
	config, _, err := render("my-project", s)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(config, "\n  Port ") || !strings.Contains(config, "ProxyCommand C:/Windows/System32/wsl.exe --distribution Hacocoon --exec /usr/local/bin/haco stream ") {
		t.Fatal(config)
	}
	// Opt-in export lets the native Windows parser inspect the actual renderer output.
	if output := os.Getenv("HACO_TEST_RENDER_OUTPUT"); output != "" {
		if err = os.WriteFile(output, []byte(config), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, distro := range []string{"-u", `name%h`, "name & whoami", "name\nHost evil"} {
		s.Distro = distro
		if _, _, err = render("my-project", s); err == nil {
			t.Fatal("accepted unsafe distro")
		}
	}
	s.Distro = "Hacocoon"
	conn.Target.Grant = "other"
	s.Connection = conn
	if _, _, err = render("my-project", s); err == nil {
		t.Fatal("accepted different grant")
	}
}
func TestRemoveProtectsOwnershipAndReplacement(t *testing.T) {
	t.Setenv("WSL_DISTRO_NAME", "")
	home := t.TempDir()
	dir := filepath.Join(home, ".ssh", "hacocoon")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	c := &fakeController{}
	conn, _ := c.PrepareEnvironmentSSH(context.Background(), "dev", core.SSHAccessRequest{PublicKey: testKey})
	meta := saved{Runtime: "owned", Connection: conn}
	b, _ := json.Marshal(meta)
	path := filepath.Join(dir, "dev.conf")
	owned := []byte(prefix + string(b) + "\nHost haco-dev\n")
	if err := os.WriteFile(path, owned, 0600); err != nil {
		t.Fatal(err)
	}
	d := Desktop{Home: home}
	ctx := context.Background()
	if err := Remove(ctx, d, "dev", "older-grant"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("removed replacement")
	}
	meta.Distro = "another"
	b, _ = json.Marshal(meta)
	os.WriteFile(path, []byte(prefix+string(b)+"\n"), 0600)
	if err := Remove(ctx, d, "dev", conn.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("removed another installation")
	}
	os.WriteFile(path, []byte("Host user-owned\n"), 0600)
	if err := Remove(ctx, d, "dev", conn.ID); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatal(err)
	}
	os.WriteFile(path, owned, 0600)
	if err := Remove(ctx, d, "dev", conn.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("owned grant not removed")
	}
}
