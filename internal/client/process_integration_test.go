package client_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	clientapp "github.com/SLktEx/Hacocoon/internal/client"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/state"
	"github.com/SLktEx/Hacocoon/modules/runtime/incus"
)

func TestClientAccessCrossesRealProcessBoundary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-backed fake Incus is for Linux/WSL test hosts")
	}
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, "incus.log")
	script := `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$HACO_FAKE_INCUS_LOG"
if [ "${1:-}" = "list" ]; then
  printf 'RUNNING\n'
fi
`
	if err := os.WriteFile(filepath.Join(bin, "incus"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HACO_FAKE_INCUS_LOG", logPath)

	store := state.NewEnvironmentJSONStore(filepath.Join(root, "state", "environments.json"))
	ctx := context.Background()
	if err := store.PutEnvironment(ctx, core.Environment{Name: "demo", RuntimeRef: "haco-demo"}); err != nil {
		t.Fatal(err)
	}
	service := clientapp.New(incus.New(host.ExecRunner{}), store)

	status, err := service.Status(ctx, "demo")
	if err != nil || status.State != core.EnvironmentRunning {
		t.Fatalf("status=%#v err=%v", status, err)
	}
	connection, err := service.Forward(ctx, "demo", core.LocalPortRequest{Protocol: "tcp", HostPort: 18080, TargetPort: 3000})
	if err != nil {
		t.Fatal(err)
	}
	if connection.Host != "127.0.0.1" {
		t.Fatalf("connection=%#v", connection)
	}
	if err := service.Unforward(ctx, "demo", connection.ID); err != nil {
		t.Fatal(err)
	}

	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(log)
	for _, want := range []string{
		"list haco-demo --project hacocoon --format csv -c s",
		"config device add haco-demo haco-tcp-18080-3000 proxy listen=tcp:127.0.0.1:18080 connect=tcp:127.0.0.1:3000 --project hacocoon",
		"config device remove haco-demo haco-tcp-18080-3000 --project hacocoon",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in process log:\n%s", want, text)
		}
	}
	if strings.Contains(text, "0.0.0.0") {
		t.Fatalf("unsafe broad listen escaped into v0.3 process path:\n%s", text)
	}
}
