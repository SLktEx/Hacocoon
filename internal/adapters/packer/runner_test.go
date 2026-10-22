package packer

import (
	"os/exec"
	"runtime"
	"testing"
)

func TestNestedPackerWorkerContracts(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux trusted Host worker")
	}
	output, err := exec.Command("python3", "-I", "build_test.py").CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
}
