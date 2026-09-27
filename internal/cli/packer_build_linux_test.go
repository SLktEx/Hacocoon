package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	basebuild "github.com/SLktEx/Hacocoon/internal/base/build"
	controlapi "github.com/SLktEx/Hacocoon/internal/controller/api"
	control "github.com/SLktEx/Hacocoon/internal/controller/transport"
)

type cliPackerFixture struct{ calls int }

func (f *cliPackerFixture) Build(context.Context, basebuild.Definition) (basebuild.Result, error) {
	f.calls++
	return basebuild.Result{}, nil
}
func TestPackerCannotReachControllerDefinitionBuilder(t *testing.T) {
	fixture := &cliPackerFixture{}
	server := control.NewServer()
	if err := controlapi.RegisterBaseBuild(server, fixture); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(t.TempDir(), "control.sock")
	listener, err := control.ListenUnix(socket, 0600)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	t.Cleanup(func() { cancel(); <-done })
	t.Setenv("HACO_CONTROL_SOCKET", socket)
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "base.pkr.hcl"), []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"base", "build", "--name", "../invalid", directory},
		{"base", "build", "--name", "tools", "--from", "haco/ubuntu-26.04", directory},
		{"base", "build", "--name", "tools", "--builder", "builder", directory},
	} {
		code, _, _ := captureRun(t, args...)
		if code != 2 || fixture.calls != 0 {
			t.Fatal("invalid request reached controller", code)
		}
	}

}
