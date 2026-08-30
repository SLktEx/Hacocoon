package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"

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
	HostPort        int    `json:"host_port,omitempty"`
	FolderURI       string `json:"folder_uri"`
}

type orchestrationPrepareOptions struct {
	SessionID   string
	CodeCommand string
	NoLaunch    bool
	JSON        bool
}

// init intercepts only the Agent Host orchestration commands whose process
// behavior needs to compose the existing prepare/release implementation with
// VS Code's current Agents window. Keeping this outside Core preserves the
// existing rule that AHP/VS Code concepts belong to the trusted client adapter.
func init() {
	handled, err := handleAgentHostOrchestrationArgs(context.Background(), os.Args[1:])
	if !handled {
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func handleAgentHostOrchestrationArgs(ctx context.Context, args []string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}
	if args[0] != "prepare" && args[0] != "lookup" {
		return false, nil
	}
	app, err := composition.Local(ctx)
	if err != nil {
		return true, err
	}
	switch args[0] {
	case "prepare":
		return true, prepareOrchestrationCommand(ctx, app, args[1:])
	case "lookup":
		return true, lookupOrchestrationCommand(ctx, app, args[1:])
	default:
		return false, nil
	}
}

func prepareOrchestrationCommand(ctx context.Context, app *composition.App, args []string) error {
	options, passthrough, err := normalizeOrchestrationPrepareArgs(args)
	if err != nil {
		return err
	}

	// Reuse the hardened v0.10 prepare path, but suppress its legacy launch so
	// the adapter can launch the Agents window directly on the remote workspace.
	legacyOutput, err := captureStdout(func() error {
		return prepareCommand(ctx, app, passthrough)
	})
	if err != nil {
		return err
	}

	binding, err := app.AgentHosts.Lookup(ctx, options.SessionID)
	if err != nil {
		return fmt.Errorf("resolve prepared agent session: %w", err)
	}
	descriptor := descriptorForBinding(ctx, binding)
	if options.JSON {
		if err := writeAgentSessionDescriptor(os.Stdout, descriptor, true); err != nil {
			return err
		}
	} else {
		if err := writeLegacyCompatibleDescriptor(os.Stdout, legacyOutput, descriptor); err != nil {
			return err
		}
	}

	if options.NoLaunch {
		return nil
	}
	cmd := exec.CommandContext(ctx, options.CodeCommand, agentsLaunchArgs(descriptor.FolderURI)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("launch VS Code Agents window: %w", err)
	}
	return nil
}

func lookupOrchestrationCommand(ctx context.Context, app *composition.App, args []string) error {
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
	return writeAgentSessionDescriptor(os.Stdout, descriptorForBinding(ctx, binding), *jsonOutput)
}

func normalizeOrchestrationPrepareArgs(args []string) (orchestrationPrepareOptions, []string, error) {
	options := orchestrationPrepareOptions{CodeCommand: "code"}
	passthrough := make([]string, 0, len(args)+1)
	parsingFlags := true

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if parsingFlags && arg == "--" {
			parsingFlags = false
			passthrough = append(passthrough, arg)
			continue
		}
		if !parsingFlags {
			passthrough = append(passthrough, arg)
			continue
		}

		switch {
		case arg == "--json" || arg == "-json":
			options.JSON = true
			continue
		case strings.HasPrefix(arg, "--json=") || strings.HasPrefix(arg, "-json="):
			value := strings.TrimPrefix(strings.TrimPrefix(arg, "--json="), "-json=")
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return orchestrationPrepareOptions{}, nil, fmt.Errorf("invalid --json value %q: %w", value, core.ErrInvalidArgument)
			}
			options.JSON = parsed
			continue
		case arg == "--no-launch" || arg == "-no-launch":
			options.NoLaunch = true
			continue
		case strings.HasPrefix(arg, "--no-launch=") || strings.HasPrefix(arg, "-no-launch="):
			value := strings.TrimPrefix(strings.TrimPrefix(arg, "--no-launch="), "-no-launch=")
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return orchestrationPrepareOptions{}, nil, fmt.Errorf("invalid --no-launch value %q: %w", value, core.ErrInvalidArgument)
			}
			options.NoLaunch = parsed
			continue
		case arg == "--session" || arg == "-session":
			if i+1 >= len(args) {
				return orchestrationPrepareOptions{}, nil, fmt.Errorf("missing --session value: %w", core.ErrInvalidArgument)
			}
			i++
			options.SessionID = args[i]
			passthrough = append(passthrough, arg, args[i])
			continue
		case strings.HasPrefix(arg, "--session=") || strings.HasPrefix(arg, "-session="):
			value := strings.TrimPrefix(strings.TrimPrefix(arg, "--session="), "-session=")
			options.SessionID = value
			passthrough = append(passthrough, arg)
			continue
		case arg == "--code" || arg == "-code":
			if i+1 >= len(args) {
				return orchestrationPrepareOptions{}, nil, fmt.Errorf("missing --code value: %w", core.ErrInvalidArgument)
			}
			i++
			options.CodeCommand = args[i]
			passthrough = append(passthrough, arg, args[i])
			continue
		case strings.HasPrefix(arg, "--code=") || strings.HasPrefix(arg, "-code="):
			value := strings.TrimPrefix(strings.TrimPrefix(arg, "--code="), "-code=")
			options.CodeCommand = value
			passthrough = append(passthrough, arg)
			continue
		default:
			passthrough = append(passthrough, arg)
		}
	}

	if options.SessionID == "" {
		// Let the public error stay explicit even though the legacy command would
		// also reject it; the session identity is required for post-prepare lookup.
		return orchestrationPrepareOptions{}, nil, fmt.Errorf("usage: haco-agent-host prepare --session <id> [options] [workspace]: %w", core.ErrInvalidArgument)
	}
	if strings.TrimSpace(options.CodeCommand) == "" {
		return orchestrationPrepareOptions{}, nil, fmt.Errorf("VS Code CLI command is empty: %w", core.ErrInvalidArgument)
	}

	// The legacy prepare parser uses Go's flag package, so the forced flag must
	// appear before any positional workspace argument.
	passthrough = append([]string{"--no-launch"}, passthrough...)
	return options, passthrough, nil
}

func descriptorForBinding(ctx context.Context, binding agenthost.Binding) agentSessionDescriptor {
	alias := agentSSHAlias(binding.SessionID)
	descriptor := agentSessionDescriptor{
		SessionID:       binding.SessionID,
		Environment:     binding.EnvironmentName,
		WorkspacePath:   binding.WorkspacePath,
		RemoteWorkspace: remoteWorkspacePath,
		SSHAlias:        alias,
		FolderURI:       agentRemoteFolderURI(alias),
	}
	if clientFS, err := resolveClientFilesystem(ctx); err == nil {
		if managed, readErr := readManagedSSHConfig(managedConfigPath(clientFS.Home, alias)); readErr == nil && managed.Alias == alias {
			descriptor.HostPort = managed.Port
		}
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

func writeLegacyCompatibleDescriptor(out io.Writer, legacyOutput string, descriptor agentSessionDescriptor) error {
	wroteAny := false
	for _, line := range strings.Split(strings.TrimSuffix(legacyOutput, "\n"), "\n") {
		if strings.HasPrefix(line, "agents:") || line == "" {
			continue
		}
		if _, err := fmt.Fprintln(out, line); err != nil {
			return err
		}
		wroteAny = true
	}
	if !wroteAny {
		if err := writeAgentSessionDescriptor(out, descriptor, false); err != nil {
			return err
		}
		return nil
	}
	_, err := fmt.Fprintf(out, "folder-uri: %s\n", descriptor.FolderURI)
	return err
}

func captureStdout(run func() error) (string, error) {
	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = writer
	runErr := run()
	closeErr := writer.Close()
	os.Stdout = original
	content, readErr := io.ReadAll(reader)
	_ = reader.Close()
	if runErr != nil {
		return "", runErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	if readErr != nil {
		return "", readErr
	}
	return string(content), nil
}
