package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/git"
)

type gitDoctorFixture struct {
	unknownAfterRepair bool
	doctorEnvironmentFixture
	configured, connected, helper bool
	repaired                      int
	statusErr, repairErr          error
}

func (f *gitDoctorFixture) EnvironmentStatus(context.Context, string) (core.EnvironmentStatus, error) {
	return core.EnvironmentStatus{Environment: core.Environment{Name: "dev", Workspace: core.Workspace{ID: "work", Path: "managed:work"}}, State: f.state}, nil
}
func (f *gitDoctorFixture) GitConnectionStatus(context.Context, string) (gitrepo.ConnectionStatus, error) {
	if f.unknownAfterRepair && f.repaired > 0 {
		return gitrepo.ConnectionStatus{}, core.ErrRuntimeUnavailable
	}
	return gitrepo.ConnectionStatus{Configured: f.configured, Connected: f.connected}, f.statusErr
}
func (f *gitDoctorFixture) ConnectGit(context.Context, string) error {
	f.repaired++
	if f.repairErr != nil {
		return f.repairErr
	}
	f.connected, f.helper = true, true
	return nil
}
func (f *gitDoctorFixture) ExecEnvironment(ctx context.Context, name string, args []string) (core.ExecutionResult, error) {
	if strings.Contains(strings.Join(args, " "), "git") {
		if !f.helper {
			return core.ExecutionResult{ExitCode: 1}, nil
		}
		return core.ExecutionResult{}, nil
	}
	return f.doctorEnvironmentFixture.ExecEnvironment(ctx, name, args)
}
func TestGitDoctorRequiresExplicitRepairAndRechecks(t *testing.T) {
	for _, language := range []string{"en", "ja"} {
		t.Run(language, func(t *testing.T) {
			t.Setenv("HACO_UI_LANGUAGE", language)
			for _, fix := range []bool{false, true} {
				for _, broken := range []string{"broker", "helper"} {
					f := &gitDoctorFixture{doctorEnvironmentFixture: doctorEnvironmentFixture{state: core.EnvironmentRunning}, configured: true, connected: broken != "broker", helper: broken != "helper"}
					report, err := diagnoseAndRepairEnvironment(context.Background(), f, "dev", fix)
					if err != nil {
						t.Fatal(err)
					}
					var out bytes.Buffer
					code := writeEnvironmentDoctor(&out, report, false)
					if fix {
						if code != 0 || f.repaired != 1 || !strings.Contains(out.String(), "git_broker: ok") {
							t.Fatal(code, f.repaired, out.String())
						}
					} else if code != 1 || f.repaired != 0 || !strings.Contains(out.String(), "haco doctor --fix dev") {
						t.Fatal(code, f.repaired, out.String())
					}
				}
			}
		})
	}
}
func TestGitDoctorNeverRepairsOfflineStoppedUnknownOrHealthy(t *testing.T) {
	for _, mode := range []string{"offline", "stopped", "unknown", "healthy"} {
		t.Run(mode, func(t *testing.T) {
			f := &gitDoctorFixture{doctorEnvironmentFixture: doctorEnvironmentFixture{state: core.EnvironmentRunning}, configured: true, connected: true, helper: true}
			switch mode {
			case "offline":
				f.configured = false
				f.connected = false
			case "stopped":
				f.state = core.EnvironmentStopped
			case "unknown":
				f.statusErr = errors.New("secret backend diagnostic")
			}
			report, err := diagnoseAndRepairEnvironment(context.Background(), f, "dev", true)
			if err != nil || f.repaired != 0 {
				t.Fatal(err, f.repaired)
			}
			var out bytes.Buffer
			code := writeEnvironmentDoctor(&out, report, true)
			if strings.Contains(out.String(), "secret") {
				t.Fatal(out.String())
			}
			if (mode == "offline" || mode == "healthy") && code != 0 {
				t.Fatal(out.String())
			}
			if (mode == "unknown" || mode == "stopped") && code == 0 {
				t.Fatal("unverified reported healthy")
			}
		})
	}
}
func TestGitDoctorRepairFailureDoesNotRetry(t *testing.T) {
	f := &gitDoctorFixture{doctorEnvironmentFixture: doctorEnvironmentFixture{state: core.EnvironmentRunning}, configured: true, repairErr: core.ErrIncompatibleState}
	if _, err := diagnoseAndRepairEnvironment(context.Background(), f, "dev", true); !errors.Is(err, core.ErrIncompatibleState) || f.repaired != 1 {
		t.Fatal(err, f.repaired)
	}
}
func TestGitDoctorRequiresTargetAndRetiresConnect(t *testing.T) {
	t.Setenv("HACO_CONTROL_SOCKET", "/missing-m2-controller.sock")
	for _, args := range [][]string{{"--fix"}, {"--json", "--fix"}, {"--fix", "dev", "other"}, {"--fix", "--fix", "dev"}} {
		var out, diag bytes.Buffer
		if code := doctor(context.Background(), args, &out, &diag); code != 2 || out.Len() != 0 {
			t.Fatal(args, code, out.String(), diag.String())
		}
	}
	var out, diag bytes.Buffer
	if code := repositoryCommand(context.Background(), "git", []string{"connect", "dev"}, &out, &diag); code != 2 {
		t.Fatal(code)
	}
}

func TestGitDoctorUnconfirmedRepairIsNotRetried(t *testing.T) {
	f := &gitDoctorFixture{doctorEnvironmentFixture: doctorEnvironmentFixture{state: core.EnvironmentRunning}, configured: true, unknownAfterRepair: true}
	report, err := diagnoseAndRepairEnvironment(context.Background(), f, "dev", true)
	if err != nil || f.repaired != 1 {
		t.Fatal(err, f.repaired)
	}
	var out bytes.Buffer
	if code := writeEnvironmentDoctor(&out, report, true); code != 1 || !strings.Contains(out.String(), `"status":"unknown"`) || strings.Contains(out.String(), "no repair was attempted") {
		t.Fatal(code, out.String())
	}
}
