package incus

import (
	"bytes"
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/host/setup"
)

func TestHostPackageFailureMatchesEmbeddedPythonReceipt(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux Host provisioner")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	// Load only definitions, then inject an exception into a no-op operation.
	// No package commands, provisioning or external requests are executed.
	script := "__name__ = 'contract_fixture'\n" + hostToolingScript + `
failure = {
    "timeout": subprocess.TimeoutExpired(["SECRET-command"], 1, stderr=b"SECRET-stderr"),
    "exit": subprocess.CalledProcessError(100, ["SECRET-command"], stderr=b"SECRET-stderr"),
    "other": RuntimeError("SECRET-error"),
}[sys.argv[2]]
def fail():
    raise failure
try:
    package_step(sys.argv[1], fail)
except Exception:
    raise SystemExit(1) from None
`
	for _, phase := range []string{"query", "update", "install", "git_verify", "gh_verify"} {
		for _, kind := range []string{"timeout", "exit", "other"} {
			cmd := exec.Command(python, "-B", "-I", "-c", script, phase, kind)
			var diagnostic bytes.Buffer
			cmd.Stderr = &diagnostic
			output, err := cmd.Output()
			if err == nil || cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 1 || diagnostic.Len() != 0 {
				t.Fatal("helper did not retain silent exit one", phase, kind, err)
			}
			result := host.Result{ExitCode: 1, Stdout: string(output)}
			if got := hostsetup.Reason(hostToolingFailure("host_packages", result)); got != "packages_"+phase+"_"+kind {
				t.Fatal("Python/Go diagnostic contract differs", phase, kind, got)
			}
		}
	}
}

func TestHostPackageFailureAcceptsOnlyCompleteFixedReceipts(t *testing.T) {
	for _, phase := range []string{"query", "update", "install", "git_verify", "gh_verify"} {
		for _, kind := range []string{"timeout", "exit", "other"} {
			result := host.Result{ExitCode: 1, Stdout: "haco-host-packages-v1:" + phase + ":" + kind + "\n", Stderr: "SECRET-stderr"}
			err := hostToolingFailure("host_packages", result)
			want := "packages_" + phase + "_" + kind
			if !errors.Is(err, core.ErrRuntimeUnavailable) || hostsetup.Reason(err) != want || !hostsetup.ValidReason(want) || strings.Contains(err.Error(), "SECRET") {
				t.Fatal("lost or unsafe package classification", phase, kind, err)
			}
			for _, stage := range []string{"host_tooling", "host_services", "host_packer", "SECRET-stage"} {
				if err := hostToolingFailure(stage, result); err != core.ErrRuntimeUnavailable {
					t.Fatal("accepted receipt in wrong mode", stage, err)
				}
			}
		}
	}
}

func TestHostPackageFailureRejectsUncertainOrArbitraryOutput(t *testing.T) {
	valid := "haco-host-packages-v1:install:timeout\n"
	cases := []host.Result{
		{ExitCode: 0, Stdout: valid},
		{ExitCode: -1, Stdout: valid},
		{ExitCode: 2, Stdout: valid},
		{ExitCode: 1, Stdout: valid, StdoutTruncated: true},
		{ExitCode: 1, Stdout: valid, StderrTruncated: true},
		{ExitCode: 1, Stderr: valid},
	}
	for _, output := range []string{"", "SECRET-output", strings.TrimSuffix(valid, "\n"), valid + valid,
		" " + valid, valid + "SECRET-output", "haco-host-packages-v2:install:timeout\n",
		"haco-host-packages-v1:SECRET:timeout\n", "haco-host-packages-v1:install:SECRET\n",
		strings.Repeat("SECRET", 20), strings.ReplaceAll(valid, "\n", "\r\n"), valid + "\x00"} {
		cases = append(cases, host.Result{ExitCode: 1, Stdout: output})
	}
	for i, result := range cases {
		if err := hostToolingFailure("host_packages", result); err != core.ErrRuntimeUnavailable {
			t.Fatal("unproven receipt classified", i, err)
		}
	}
}
