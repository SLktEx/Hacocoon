package incus

import (
	"os/exec"
	"runtime"
	"testing"
)

func TestHostToolingRepeatFastPathPythonContracts(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux Host provisioner")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	output, err := exec.Command(python, "-I", "testdata/host_tooling_fastpath_test.py", "host_tooling.py").CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
}
