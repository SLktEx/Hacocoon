package main

import (
	"context"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
)

type environmentGitStatusClient interface {
	GitBrokerStatus(context.Context, string) (controlapi.GitBrokerStatusResponse, error)
}

type environmentGitRepairClient interface {
	ConnectGit(context.Context, string) error
}

func diagnoseEnvironmentWithGit(ctx context.Context, c environmentDoctorClient, name string) (environmentDoctorReport, error) {
	report, err := diagnoseEnvironment(ctx, c, name)
	if err != nil {
		return report, err
	}
	check := environmentDoctorCheck{Name: "git_broker", Status: "skipped"}
	if !strings.HasPrefix(report.Workspace.Path, "managed:") {
		report.Checks = append(report.Checks, check)
		return report, nil
	}
	statusClient, ok := c.(environmentGitStatusClient)
	if !ok {
		return report, core.ErrUnsupported
	}
	probeCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	status, err := statusClient.GitBrokerStatus(probeCtx, name)
	if err != nil {
		return report, err
	}
	if !status.Applicable {
		report.Checks = append(report.Checks, check)
		return report, nil
	}
	if !status.Connected {
		check.Status = "failed"
		check.Action = "haco doctor --fix " + name
		report.Checks = append(report.Checks, check)
		return report, nil
	}
	if report.State != core.EnvironmentRunning {
		check.Action = "Start the Environment before checking guest Git broker wiring"
		report.Checks = append(report.Checks, check)
		return report, nil
	}
	result, runErr := c.ExecEnvironment(probeCtx, name, []string{"/bin/sh", "-ec", "test -S " + gitrepo.GuestSocket + " && test -x /usr/local/bin/git-remote-haco"})
	if runErr != nil || result.ExitCode != 0 {
		check.Status = "failed"
		check.Action = "haco doctor --fix " + name
	} else {
		check.Status = "ok"
	}
	report.Checks = append(report.Checks, check)
	return report, nil
}

// repairEnvironmentGitBroker repairs only the managed Git broker route reported
// by diagnostics. DNS, SSH and other failed checks remain read-only.
func repairEnvironmentGitBroker(ctx context.Context, c environmentDoctorClient, name string, report environmentDoctorReport) (environmentDoctorReport, error) {
	needsRepair := false
	for _, check := range report.Checks {
		if check.Name == "git_broker" && check.Status == "failed" {
			needsRepair = true
			break
		}
	}
	if !needsRepair {
		return report, nil
	}
	repairClient, ok := c.(environmentGitRepairClient)
	if !ok {
		return report, core.ErrUnsupported
	}
	if err := repairClient.ConnectGit(ctx, name); err != nil {
		return report, err
	}
	return diagnoseEnvironmentWithGit(ctx, c, name)
}
