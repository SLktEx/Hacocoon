package agenthostcli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/SLktEx/Hacocoon/internal/agenthost"
	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type agentSessionDescriptor struct {
	SessionID       string `json:"session_id"`
	Environment     string `json:"environment"`
	WorkspacePath   string `json:"workspace_path"`
	RemoteWorkspace string `json:"remote_workspace"`
	SSHAlias        string `json:"ssh_alias"`
	FolderURI       string `json:"folder_uri"`
}

func lookupCommand(ctx context.Context, app *composition.App, args []string) error {
	fs := flag.NewFlagSet("lookup", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	sessionID := fs.String("session", "", "opaque trusted agent-session identity")
	jsonOutput := fs.Bool("json", false, "emit a machine-readable session descriptor")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *sessionID == "" || fs.NArg() != 0 {
		return fmt.Errorf("usage: haco-agent-host lookup --session <id> [--json]: %w", core.ErrInvalidArgument)
	}
	binding, err := app.AgentHosts.Lookup(ctx, *sessionID)
	if err != nil {
		return err
	}
	return writeAgentSessionDescriptor(os.Stdout, descriptorForBinding(binding), *jsonOutput)
}

func descriptorForBinding(binding agenthost.Binding) agentSessionDescriptor {
	alias := agentSSHAlias(binding.SessionID)
	descriptor := agentSessionDescriptor{
		SessionID:       binding.SessionID,
		Environment:     binding.EnvironmentName,
		WorkspacePath:   binding.WorkspacePath,
		RemoteWorkspace: remoteWorkspacePath,
		SSHAlias:        alias,
		FolderURI:       agentRemoteFolderURI(alias),
	}
	return descriptor
}

func agentRemoteFolderURI(alias string) string {
	return (&url.URL{
		Scheme: "vscode-remote",
		Host:   "ssh-remote+" + alias,
		Path:   remoteWorkspacePath,
	}).String()
}

func agentsLaunchArgs(folderURI string) []string {
	return []string{"--agents", "--folder-uri", folderURI}
}

func writeAgentSessionDescriptor(out io.Writer, descriptor agentSessionDescriptor, jsonOutput bool) error {
	if jsonOutput {
		encoder := json.NewEncoder(out)
		encoder.SetEscapeHTML(false)
		return encoder.Encode(descriptor)
	}
	_, err := fmt.Fprintf(out,
		"environment: %s\nssh: %s\nworkspace: %s\nfolder-uri: %s\n",
		descriptor.Environment,
		descriptor.SSHAlias,
		descriptor.RemoteWorkspace,
		descriptor.FolderURI,
	)
	return err
}
