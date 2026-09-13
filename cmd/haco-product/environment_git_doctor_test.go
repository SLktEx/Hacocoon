package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type gitDoctorFixture struct {
	workspacePath string
	applicable    bool
	connected     bool
	guestHealthy  bool
	failDNS       bool
	statusCalls   int
	connectCalls  int
}

func (f *gitDoctorFixture) EnvironmentStatus(context.Context, string) (core.EnvironmentStatus, error) {
	path := f.workspacePath
	if path == "" {
		path = "managed:work"
	}
	return core.EnvironmentStatus{Environment: core.Environment{Name: "dev", Workspace: core.Workspace{ID: "workspace:managed:owner", Path: path}}, State: core.EnvironmentRunning}, nil
}
func (*gitDoctorFixture) EnvironmentConnections(context.Context, string) ([]core.ClientConnection, error) {
	return nil, nil
}
func (f *gitDoctorFixture) ExecEnvironment(_ context.Context, _ string, command []string) (core.ExecutionResult, error) {
	script := strings.Join(command, " ")
	if f.failDNS && strings.Contains(script, "hacocoon-dns.service") {
		return core.ExecutionResult{ExitCode: 1}, nil
	}
	if strings.Contains(script, "hacocoon-git.sock") && !f.guestHealthy {
		return core.ExecutionResult{ExitCode: 1}, nil
	}
	return core.ExecutionResult{}, nil
}
func (f *gitDoctorFixture) GitBrokerStatus(context.Context, string) (controlapi.GitBrokerStatusResponse, error) {
	f.statusCalls++
	return controlapi.GitBrokerStatusResponse{Applicable: f.applicable, Connected: f.connected}, nil
}
func (f *gitDoctorFixture) ConnectGit(context.Context, string) error {
	f.connectCalls++
	f.connected = true
	f.guestHealthy = true
	return nil
}

func environmentCheck(report environmentDoctorReport, name string) (environmentDoctorCheck, bool) {
	for _, check := range report.Checks {
		if check.Name == name {
			return check, true
		}
	}
	return environmentDoctorCheck{}, false
}

func TestEnvironmentDoctorReportsBrokenGitBrokerWithoutRepair(t *testing.T) {
	f := &gitDoctorFixture{applicable: true}
	report, err := diagnoseEnvironmentWithGit(context.Background(), f, "dev")
	if err != nil {
		t.Fatal(err)
	}
	check, ok := environmentCheck(report, "git_broker")
	if !ok || check.Status != "failed" || check.Action != "haco doctor --fix dev" || f.connectCalls != 0 {
		t.Fatalf("check=%+v connectCalls=%d", check, f.connectCalls)
	}
	var out bytes.Buffer
	if writeEnvironmentDoctor(&out, report, false) != 1 || !strings.Contains(out.String(), "git_broker: failed") {
		t.Fatalf("output=%q", out.String())
	}
}

func TestEnvironmentDoctorFixRepairsOnlyGitBroker(t *testing.T) {
	f := &gitDoctorFixture{applicable: true}
	report, err := diagnoseEnvironmentWithGit(context.Background(), f, "dev")
	if err != nil {
		t.Fatal(err)
	}
	report, err = repairEnvironmentGitBroker(context.Background(), f, "dev", report)
	if err != nil {
		t.Fatal(err)
	}
	check, ok := environmentCheck(report, "git_broker")
	if !ok || check.Status != "ok" || f.connectCalls != 1 {
		t.Fatalf("check=%+v connectCalls=%d", check, f.connectCalls)
	}
}

func TestEnvironmentDoctorGitlessManagedWorkspaceIsSkipped(t *testing.T) {
	f := &gitDoctorFixture{}
	report, err := diagnoseEnvironmentWithGit(context.Background(), f, "dev")
	if err != nil {
		t.Fatal(err)
	}
	check, ok := environmentCheck(report, "git_broker")
	if !ok || check.Status != "skipped" || f.connectCalls != 0 {
		t.Fatalf("check=%+v connectCalls=%d", check, f.connectCalls)
	}
	var out bytes.Buffer
	if writeEnvironmentDoctor(&out, report, false) != 0 {
		t.Fatalf("skipped Git broker failed diagnostics: %s", out.String())
	}
}

func TestEnvironmentDoctorExternalWorkspaceSkipsGitStatus(t *testing.T) {
	f := &gitDoctorFixture{workspacePath: "/work", applicable: true, connected: true, guestHealthy: true}
	report, err := diagnoseEnvironmentWithGit(context.Background(), f, "dev")
	if err != nil {
		t.Fatal(err)
	}
	check, ok := environmentCheck(report, "git_broker")
	if !ok || check.Status != "skipped" || f.statusCalls != 0 {
		t.Fatalf("check=%+v statusCalls=%d", check, f.statusCalls)
	}
}

func TestEnvironmentDoctorFixDoesNotRepairDNS(t *testing.T) {
	f := &gitDoctorFixture{applicable: true, connected: true, guestHealthy: true, failDNS: true}
	report, err := diagnoseEnvironmentWithGit(context.Background(), f, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if check, _ := environmentCheck(report, "dns_service"); check.Status != "failed" {
		t.Fatalf("dns check=%+v", check)
	}
	repaired, err := repairEnvironmentGitBroker(context.Background(), f, "dev", report)
	if err != nil || f.connectCalls != 0 {
		t.Fatalf("connectCalls=%d err=%v", f.connectCalls, err)
	}
	if check, _ := environmentCheck(repaired, "dns_service"); check.Status != "failed" {
		t.Fatalf("dns check changed=%+v", check)
	}
}

func TestDoctorFixRequiresEnvironmentAndGitConnectIsNotPublic(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := doctor(context.Background(), []string{"--fix"}, &stdout, &stderr); code != 2 {
		t.Fatalf("doctor --fix code=%d stderr=%q", code, stderr.String())
	}
	jsonOutput, fix, target, ok := parseDoctorArgs([]string{"dev", "--fix", "--json"})
	if !ok || !jsonOutput || !fix || target != "dev" {
		t.Fatalf("parsed json=%v fix=%v target=%q ok=%v", jsonOutput, fix, target, ok)
	}
	stdout.Reset()
	stderr.Reset()
	if code := repositoryCommand(context.Background(), "git", []string{"connect", "dev"}, &stdout, &stderr); code != 2 || strings.Contains(stderr.String(), "git connect") {
		t.Fatalf("git connect still public: code=%d stderr=%q", code, stderr.String())
	}
}
