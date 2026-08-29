package state

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type childProcess struct {
	cmd    *exec.Cmd
	output *bytes.Buffer
}

func TestEnvironmentJSONStoreChildWriter(t *testing.T) {
	if os.Getenv("HACO_STATE_CHILD") != "1" {
		return
	}
	path := os.Getenv("HACO_STATE_PATH")
	name := os.Getenv("HACO_STATE_NAME")
	barrier := os.Getenv("HACO_STATE_BARRIER")
	for {
		if _, err := os.Stat(barrier); err == nil {
			break
		}
		time.Sleep(time.Millisecond)
	}
	store := NewEnvironmentJSONStore(path)
	environment := core.Environment{
		Name:       name,
		Workspace:  core.Workspace{ID: core.WorkspaceID("workspace:" + name), Path: "/workspace/" + name},
		AccessMode: core.WorkspaceReadWrite,
		RuntimeRef: "haco-" + name,
		CreatedAt:  time.Now().UTC(),
	}
	if err := store.PutEnvironment(context.Background(), environment); err != nil {
		t.Fatal(err)
	}
}

func TestEnvironmentJSONStoreSerializesIndependentProcesses(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("process-wide flock implementation is Linux-specific")
	}

	root := t.TempDir()
	path := filepath.Join(root, "state", "environments.json")
	barrier := filepath.Join(root, "go")
	const writers = 8
	children := make([]childProcess, 0, writers)

	for i := 0; i < writers; i++ {
		name := fmt.Sprintf("writer-%d", i)
		cmd := exec.Command(os.Args[0], "-test.run=^TestEnvironmentJSONStoreChildWriter$")
		cmd.Env = append(os.Environ(),
			"HACO_STATE_CHILD=1",
			"HACO_STATE_PATH="+path,
			"HACO_STATE_NAME="+name,
			"HACO_STATE_BARRIER="+barrier,
		)
		output := &bytes.Buffer{}
		cmd.Stdout = output
		cmd.Stderr = output
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		children = append(children, childProcess{cmd: cmd, output: output})
	}

	if err := os.WriteFile(barrier, []byte("go"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, child := range children {
		if err := child.cmd.Wait(); err != nil {
			t.Fatalf("child writer failed: %v\n%s", err, child.output.String())
		}
	}

	store := NewEnvironmentJSONStore(path)
	for i := 0; i < writers; i++ {
		name := fmt.Sprintf("writer-%d", i)
		if _, err := store.GetEnvironment(context.Background(), name); err != nil {
			t.Fatalf("environment %q was lost in a concurrent state update: %v", name, err)
		}
	}
}
