package cli

import (
	"context"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/git"
)

type gitDoctorClient interface {
	GitConnectionStatus(context.Context, string) (gitrepo.ConnectionStatus, error)
	ConnectGit(context.Context, string) error
}

func diagnoseGitConnection(ctx context.Context, c environmentDoctorClient, report environmentDoctorReport) environmentDoctorCheck {
	check := environmentDoctorCheck{Name: "git_broker", Status: "not_applicable"}
	if !strings.HasPrefix(report.Workspace.Path, "managed:") {
		return check
	}
	git, ok := c.(gitDoctorClient)
	if !ok {
		check.Status = "unknown"
		return check
	}
	state, err := git.GitConnectionStatus(ctx, report.Environment)
	if err != nil {
		check.Status = "unknown"
		check.Action = "Inspect controller availability and Workspace ownership before retrying"
		check.actionMessage = "env.doctor.git_unknown"
		return check
	}
	if !state.Configured {
		return check
	}
	if report.State != core.EnvironmentRunning {
		check.Status = "skipped"
		check.Action = "Start the Environment before checking local services"
		check.actionMessage = "env.doctor.start_before_checks"
		return check
	}
	check.Status = "failed"
	check.Action = "Repair local Git wiring with haco doctor --fix " + report.Environment
	check.actionMessage = "env.doctor.git_fix"
	if !state.Connected {
		return check
	}
	// Presence-only guest observation. Never invoke Git, contact upstream or use
	// guest output to grant authority. Broker ownership and Policy remain authoritative.
	result, err := c.ExecEnvironment(ctx, report.Environment, []string{"/bin/sh", "-ec", "test -S /var/lib/hacocoon-git.sock && test -x /usr/local/bin/git-remote-haco"})
	if err == nil && result.ExitCode == 0 {
		check.Status, check.Action, check.actionMessage = "ok", "", ""
	} else if err != nil {
		check.Status = "unknown"
		check.Action = "Inspect controller availability and Workspace ownership before retrying"
		check.actionMessage = "env.doctor.git_unknown"
	}
	return check
}

func diagnoseAndRepairEnvironment(ctx context.Context, c environmentDoctorClient, name string, fix bool) (environmentDoctorReport, error) {
	report, err := diagnoseEnvironment(ctx, c, name)
	if err != nil || !fix {
		return report, err
	}
	for i, check := range report.Checks {
		if check.Name != "git_broker" || check.Status != "failed" {
			continue
		}
		git, ok := c.(gitDoctorClient)
		if !ok {
			return report, core.ErrUnsupported
		}
		if err := git.ConnectGit(ctx, name); err != nil {
			return report, err
		}
		report.Checks[i] = diagnoseGitConnection(ctx, c, report)
	}
	return report, nil
}
