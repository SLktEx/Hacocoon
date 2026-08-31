package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRepositoryMilestoneLockBlackBox(t *testing.T) {
	root, err := findRoot()
	if err != nil {
		t.Fatal(err)
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is required for repository milestone lock black-box test")
	}
	cmd := exec.Command(python, filepath.Join(root, "tools/test_milestone_lock.py"))
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("milestone lock black-box suite failed: %v\n%s", err, output)
	}
}
