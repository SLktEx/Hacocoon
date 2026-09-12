package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/SLktEx/Hacocoon/internal/cliui"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type environmentDoctorClient interface {
	EnvironmentStatus(context.Context, string) (core.EnvironmentStatus, error)
	EnvironmentConnections(context.Context, string) ([]core.ClientConnection, error)
	ExecEnvironment(context.Context, string, []string) (core.ExecutionResult, error)
}
type environmentDoctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Action string `json:"action,omitempty"`

	// Presentation metadata is private to this CLI report. Never change Action
	// or add a field to JSON merely because a human reader selects Japanese.
	actionMessage string
}
type environmentDoctorReport struct {
	Environment string                   `json:"environment"`
	Workspace   core.Workspace           `json:"workspace"`
	State       core.EnvironmentState    `json:"state"`
	Checks      []environmentDoctorCheck `json:"checks"`
	Connections []core.ClientConnection  `json:"connections"`
	Scope       string                   `json:"scope"`

	scopeMessage string
}

func diagnoseEnvironment(ctx context.Context, c environmentDoctorClient, name string) (environmentDoctorReport, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	report := environmentDoctorReport{Environment: name, Scope: "Local prerequisites only; external DNS, egress Policy and desktop reachability are not tested.", scopeMessage: "env.doctor.scope"}
	status, err := c.EnvironmentStatus(ctx, name)
	if err != nil {
		return report, err
	}
	if status.Environment.Name != name {
		return report, core.ErrIncompatibleState
	}
	report.Workspace = status.Environment.Workspace
	report.State = status.State
	check := environmentDoctorCheck{Name: "runtime", Status: "ok"}
	if status.State != core.EnvironmentRunning {
		check.Status = "failed"
		check.Action = "Start the Environment with haco env start " + name
		check.actionMessage = "env.doctor.start"
	}
	report.Checks = append(report.Checks, check)
	connections, err := c.EnvironmentConnections(ctx, name)
	if err != nil {
		return report, err
	}
	// Desktop host public keys and suggested commands are not diagnostic output.
	for _, connection := range connections {
		report.Connections = append(report.Connections, core.ClientConnection{ID: connection.ID, Kind: connection.Kind, Host: connection.Host, Port: connection.Port, TargetPort: connection.TargetPort})
	}
	probes := []struct{ name, script, action, message string }{
		{"workspace", "test -d /workspace", "Inspect the Environment Workspace attachment; do not delete retained data", "env.doctor.workspace"},
		{"dns_service", "systemctl is-active --quiet hacocoon-dns.service && grep -qx 'nameserver 127.0.0.1' /etc/resolv.conf", "Inspect hacocoon-dns.service in the Environment; separately review network.resolve Policy", "env.doctor.dns"},
	}
	hasSSH := false
	for _, connection := range connections {
		if connection.Kind == "ssh" {
			hasSSH = true
		}
	}
	if hasSSH {
		probes = append(probes, struct{ name, script, action, message string }{"ssh_service", "systemctl is-active --quiet ssh.service", "Inspect the Environment ssh.service and rerun haco ssh setup " + name, "env.doctor.ssh"})
	}
	for _, probe := range probes {
		check := environmentDoctorCheck{Name: probe.name, Status: "skipped", Action: "Start the Environment before checking local services", actionMessage: "env.doctor.start_before_checks"}
		if status.State == core.EnvironmentRunning {
			result, runErr := c.ExecEnvironment(ctx, name, []string{"/bin/sh", "-ec", probe.script})
			check.Status = "ok"
			check.Action = ""
			check.actionMessage = ""
			if runErr != nil || result.ExitCode != 0 {
				check.Status = "failed"
				check.Action = probe.action
				check.actionMessage = probe.message
			}
		}
		report.Checks = append(report.Checks, check)
	}
	return report, nil
}

func environmentDoctorAction(language cliui.Language, report environmentDoctorReport, check environmentDoctorCheck) string {
	switch check.actionMessage {
	case "env.doctor.start", "env.doctor.ssh":
		return language.Format(check.actionMessage, displayCell(report.Environment))
	case "env.doctor.start_before_checks", "env.doctor.workspace", "env.doctor.dns":
		return language.Text(check.actionMessage)
	default:
		// Unknown reports keep their original details, not a guessed translation.
		return displayCell(check.Action)
	}
}

func writeEnvironmentDoctor(out io.Writer, report environmentDoctorReport, asJSON bool) int {
	return writeEnvironmentDoctorLanguage(out, report, asJSON, cliLanguage())
}

func writeEnvironmentDoctorLanguage(out io.Writer, report environmentDoctorReport, asJSON bool, language cliui.Language) int {
	healthy := true
	for _, check := range report.Checks {
		if check.Status != "ok" {
			healthy = false
		}
	}
	if asJSON {
		if err := json.NewEncoder(out).Encode(report); err != nil {
			return 1
		}
	} else {
		if _, err := fmt.Fprint(out, language.Format("env.doctor.header", displayCell(report.Environment), displayCell(report.Workspace.Path), displayCell(string(report.Workspace.ID)), displayCell(string(report.State)))); err != nil {
			return 1
		}
		for _, check := range report.Checks {
			if _, err := fmt.Fprintf(out, "%s: %s\n", displayCell(check.Name), displayCell(check.Status)); err != nil {
				return 1
			}
			if check.Action != "" {
				if _, err := fmt.Fprint(out, language.Format("env.doctor.next", environmentDoctorAction(language, report, check))); err != nil {
					return 1
				}
			}
		}
		for _, connection := range report.Connections {
			if _, err := fmt.Fprint(out, language.Format("env.doctor.connection", displayCell(connection.ID), displayCell(connection.Host), connection.Port, connection.TargetPort)); err != nil {
				return 1
			}
		}
		scope := displayCell(report.Scope)
		if report.scopeMessage == "env.doctor.scope" {
			scope = language.Text("env.doctor.scope")
		}
		if _, err := fmt.Fprintln(out, scope); err != nil {
			return 1
		}
	}
	if !healthy {
		return 1
	}
	return 0
}
