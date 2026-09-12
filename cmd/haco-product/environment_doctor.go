package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

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
}
type environmentDoctorReport struct {
	Environment string                   `json:"environment"`
	Workspace   core.Workspace           `json:"workspace"`
	State       core.EnvironmentState    `json:"state"`
	Checks      []environmentDoctorCheck `json:"checks"`
	Connections []core.ClientConnection  `json:"connections"`
	Scope       string                   `json:"scope"`
}

func diagnoseEnvironment(ctx context.Context, c environmentDoctorClient, name string) (environmentDoctorReport, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	report := environmentDoctorReport{Environment: name, Scope: "Local prerequisites only; external DNS, egress Policy and desktop reachability are not tested."}
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
	probes := []struct{ name, script, action string }{
		{"workspace", "test -d /workspace", "Inspect the Environment Workspace attachment; do not delete retained data"},
		{"dns_service", "systemctl is-active --quiet hacocoon-dns.service && grep -qx 'nameserver 127.0.0.1' /etc/resolv.conf", "Inspect hacocoon-dns.service in the Environment; separately review network.resolve Policy"},
	}
	hasSSH := false
	for _, connection := range connections {
		if connection.Kind == "ssh" {
			hasSSH = true
		}
	}
	if hasSSH {
		probes = append(probes, struct{ name, script, action string }{"ssh_service", "systemctl is-active --quiet ssh.service", "Inspect the Environment ssh.service and rerun haco ssh setup " + name})
	}
	for _, probe := range probes {
		check := environmentDoctorCheck{Name: probe.name, Status: "skipped", Action: "Start the Environment before checking local services"}
		if status.State == core.EnvironmentRunning {
			result, runErr := c.ExecEnvironment(ctx, name, []string{"/bin/sh", "-ec", probe.script})
			check.Status = "ok"
			check.Action = ""
			if runErr != nil || result.ExitCode != 0 {
				check.Status = "failed"
				check.Action = probe.action
			}
		}
		report.Checks = append(report.Checks, check)
	}
	return report, nil
}
func writeEnvironmentDoctor(out io.Writer, report environmentDoctorReport, asJSON bool) int {
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
		if _, err := fmt.Fprintf(out, "Environment: %s\nWorkspace: %s (%s)\nState: %s\n", displayCell(report.Environment), displayCell(report.Workspace.Path), displayCell(string(report.Workspace.ID)), report.State); err != nil {
			return 1
		}
		for _, check := range report.Checks {
			fmt.Fprintf(out, "%s: %s\n", check.Name, check.Status)
			if check.Action != "" {
				fmt.Fprintln(out, "  Next:", check.Action)
			}
		}
		for _, connection := range report.Connections {
			fmt.Fprintf(out, "Connection: %s %s:%d -> %d\n", displayCell(connection.ID), displayCell(connection.Host), connection.Port, connection.TargetPort)
		}
		fmt.Fprintln(out, report.Scope)
	}
	if !healthy {
		return 1
	}
	return 0
}
