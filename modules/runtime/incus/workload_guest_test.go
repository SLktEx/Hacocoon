package incus

import (
	"path/filepath"
	"testing"
)

func TestWorkloadBrokerSocketPathFollowsConfiguredControllerDirectory(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HACO_CONTROL_SOCKET", filepath.Join(root, "control.sock"))
	got, err := WorkloadBrokerSocketPath("demo")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "workloads", "demo.sock")
	if got != want {
		t.Fatalf("WorkloadBrokerSocketPath = %q, want %q", got, want)
	}
}

func TestWorkloadBrokerSocketPathRejectsRelativeControllerSocket(t *testing.T) {
	t.Setenv("HACO_CONTROL_SOCKET", "relative/control.sock")
	if _, err := WorkloadBrokerSocketPath("demo"); err == nil {
		t.Fatal("relative HACO_CONTROL_SOCKET unexpectedly accepted")
	}
}
