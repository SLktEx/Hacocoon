package main

import (
	"bytes"
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"strings"
	"testing"
)

type doctorEnvironmentFixture struct {
	state core.EnvironmentState
	calls int
	fail  bool
}

func (f *doctorEnvironmentFixture) EnvironmentStatus(context.Context, string) (core.EnvironmentStatus, error) {
	return core.EnvironmentStatus{Environment: core.Environment{Name: "dev", Workspace: core.Workspace{ID: "work", Path: "/repo"}}, State: f.state}, nil
}
func (f *doctorEnvironmentFixture) EnvironmentConnections(context.Context, string) ([]core.ClientConnection, error) {
	return []core.ClientConnection{{Kind: "ssh", ID: "ssh-40000", Host: "127.0.0.1", Port: 40000, TargetPort: 22, HostPublicKey: "do-not-render-key", Command: "do-not-render-command"}}, nil
}
func (f *doctorEnvironmentFixture) ExecEnvironment(context.Context, string, []string) (core.ExecutionResult, error) {
	f.calls++
	if f.fail {
		return core.ExecutionResult{ExitCode: 1, Stderr: "private backend output"}, errors.New("private backend error")
	}
	return core.ExecutionResult{}, nil
}
func TestEnvironmentDoctorReportsPrerequisitesWithoutRepair(t *testing.T) {
	for _, state := range []core.EnvironmentState{core.EnvironmentRunning, core.EnvironmentStopped} {
		f := &doctorEnvironmentFixture{state: state}
		report, err := diagnoseEnvironment(context.Background(), f, "dev")
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		code := writeEnvironmentDoctor(&out, report, false)
		if state == core.EnvironmentRunning && (code != 0 || f.calls != 3) {
			t.Fatal("running checks missing")
		}
		if state == core.EnvironmentStopped && (code != 1 || f.calls != 0) {
			t.Fatal("probed stopped runtime")
		}
		if strings.Contains(out.String(), "do-not-render") || !strings.Contains(out.String(), "not tested") {
			t.Fatal("diagnostic scope or redaction wrong")
		}
	}
}
func TestEnvironmentDoctorDoesNotExposeProbeFailureOutput(t *testing.T) {
	f := &doctorEnvironmentFixture{state: core.EnvironmentRunning, fail: true}
	report, err := diagnoseEnvironment(context.Background(), f, "dev")
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if writeEnvironmentDoctor(&out, report, true) != 1 || strings.Contains(out.String(), "private backend") {
		t.Fatal("failure hidden or exposed")
	}
}
