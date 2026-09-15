package agenthostcli

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"

	agenthostapp "github.com/SLktEx/Hacocoon/internal/agenthost"
	"github.com/SLktEx/Hacocoon/internal/client/ssh/config"
	"github.com/SLktEx/Hacocoon/internal/client/ssh/key"
	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/core"
)

const remoteWorkspacePath = "/workspace"

type managedSSHConfig struct {
	Alias        string
	IdentityFile string
	Connection   core.ClientConnection
	Distro       string
}

type clientFilesystem struct {
	Home string
	WSL  bool
}

func Main() {
	ctx := context.Background()
	app, err := composition.Local(ctx)
	if err != nil {
		fail(err)
	}
	if err := dispatch(ctx, app, os.Args[1:]); err != nil {
		fail(err)
	}
}

func dispatch(ctx context.Context, app *composition.App, args []string) error {
	if len(args) == 0 {
		usage()
		return core.ErrInvalidArgument
	}
	switch args[0] {
	case "prepare":
		return prepareCommand(ctx, app, args[1:])
	case "lookup":
		return lookupCommand(ctx, app, args[1:])
	case "release":
		return releaseCommand(ctx, app, args[1:])
	default:
		usage()
		return fmt.Errorf("unknown command %q: %w", args[0], core.ErrInvalidArgument)
	}
}

func prepareCommand(ctx context.Context, app *composition.App, args []string) error {
	fs := flag.NewFlagSet("prepare", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	sessionID := fs.String("session", "", "opaque trusted agent-session identity")
	identity := fs.String("identity", "", "SSH private key used by the VS Code client")
	codeCommand := fs.String("code", "code", "VS Code CLI command")
	noLaunch := fs.Bool("no-launch", false, "prepare the remote host without opening the VS Code Agents window")
	jsonOutput := fs.Bool("json", false, "emit a machine-readable session descriptor")
	readOnly := fs.Bool("read-only", false, "create a read-only Workspace lease")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *sessionID == "" || fs.NArg() > 1 {
		return fmt.Errorf("usage: haco-agent-host prepare --session <id> [options] [workspace]: %w", core.ErrInvalidArgument)
	}
	if strings.TrimSpace(*codeCommand) == "" {
		return fmt.Errorf("VS Code CLI command is empty: %w", core.ErrInvalidArgument)
	}

	workspaceArg := "."
	if fs.NArg() == 1 {
		workspaceArg = fs.Arg(0)
	}
	workspace, err := resolveWorkspacePath(workspaceArg)
	if err != nil {
		return err
	}

	clientFS, err := resolveClientFilesystem(ctx)
	if err != nil {
		return err
	}
	if *identity == "" {
		*identity = filepath.Join(clientFS.Home, ".ssh", "id_ed25519")
	}
	identityPath, err := filepath.Abs(*identity)
	if err != nil {
		return fmt.Errorf("resolve identity file: %w", err)
	}
	if info, statErr := os.Stat(identityPath); statErr != nil || info.IsDir() {
		if statErr == nil {
			statErr = fmt.Errorf("path is a directory")
		}
		return fmt.Errorf("SSH private key %q: %w", identityPath, statErr)
	}
	identityConfigValue, err := identityForClientConfig(ctx, clientFS, identityPath)
	if err != nil {
		return err
	}
	if err := validateSSHConfigValue(identityConfigValue); err != nil {
		return fmt.Errorf("SSH identity path cannot be represented safely in managed config: %w", err)
	}
	publicKeyPath := identityPath + ".pub"
	publicKey, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return fmt.Errorf("read SSH public key %q: %w", publicKeyPath, err)
	}

	mode := core.WorkspaceReadWrite
	if *readOnly {
		mode = core.WorkspaceReadOnly
	}
	binding, err := app.AgentHosts.Acquire(ctx, agenthostapp.Spec{
		SessionID:     *sessionID,
		WorkspacePath: workspace,
		AccessMode:    mode,
	})
	if err != nil {
		return err
	}

	status, err := app.Clients.Status(ctx, binding.EnvironmentName)
	if err != nil {
		return err
	}
	if status.State != core.EnvironmentRunning {
		return fmt.Errorf(
			"environment %q is %s; VS Code remote Agent Host adapter requires a running environment: %w",
			binding.EnvironmentName,
			status.State,
			core.ErrUnsupported,
		)
	}

	alias := agentSSHAlias(*sessionID)
	managedPath := managedConfigPath(clientFS.Home, alias)
	previous, previousErr := readManagedSSHConfig(managedPath)
	if previousErr != nil && !os.IsNotExist(previousErr) {
		return previousErr
	}
	if previousErr == nil && (previous.Alias != alias || previous.Distro != os.Getenv("WSL_DISTRO_NAME")) {
		return core.ErrIncompatibleState
	}
	connections, err := app.Clients.Connections(ctx, binding.EnvironmentName)
	if err != nil {
		return err
	}
	oldConnection := findSSHConnection(connections, previous.Connection.ID)
	connection := reusableSSHConnection(previous, alias, identityConfigValue, connections)
	preparedConnectionID := ""

	if connection.ID == "" {
		connection, err = app.Clients.SSH(ctx, binding.EnvironmentName, core.SSHAccessRequest{
			PublicKey: string(publicKey),
		})
		if err != nil {
			return fmt.Errorf("prepare SSH for agent environment %q: %w", binding.EnvironmentName, err)
		}
		preparedConnectionID = connection.ID
	}

	if err := ensureSSHInclude(clientFS.Home); err != nil {
		return cleanupPreparedConnection(ctx, app, binding.EnvironmentName, preparedConnectionID, err)
	}
	managed := managedSSHConfig{Alias: alias, Connection: connection, IdentityFile: identityConfigValue, Distro: os.Getenv("WSL_DISTRO_NAME")}
	if previous.Connection.Target != nil && !sameSSHEnvironment(previous.Connection.Target, connection.Target) {
		return cleanupPreparedConnection(ctx, app, binding.EnvironmentName, preparedConnectionID, core.ErrIncompatibleState)
	}
	if err := writeManagedSSHConfig(managedPath, managed); err != nil {
		return cleanupPreparedConnection(ctx, app, binding.EnvironmentName, preparedConnectionID, err)
	}

	if oldConnection.ID != "" && oldConnection.ID != connection.ID {
		if err := app.Clients.Unforward(context.WithoutCancel(ctx), binding.EnvironmentName, oldConnection.ID); err != nil {
			return errors.Join(
				fmt.Errorf("new agent-host SSH connection is active but old managed SSH connection %q could not be revoked: %w", oldConnection.ID, err),
				core.ErrRecoveryRequired,
			)
		}
	}

	descriptor := descriptorForBinding(binding)
	if err := writeAgentSessionDescriptor(os.Stdout, descriptor, *jsonOutput); err != nil {
		return err
	}
	if *noLaunch {
		return nil
	}

	cmd := exec.CommandContext(ctx, *codeCommand, agentsLaunchArgs(descriptor.FolderURI)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("launch VS Code Agents window: %w", err)
	}
	return nil
}

func releaseCommand(ctx context.Context, app *composition.App, args []string) error {
	fs := flag.NewFlagSet("release", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	sessionID := fs.String("session", "", "opaque trusted agent-session identity")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *sessionID == "" || fs.NArg() != 0 {
		return fmt.Errorf("usage: haco-agent-host release --session <id>: %w", core.ErrInvalidArgument)
	}

	releaseErr := app.AgentHosts.Release(ctx, *sessionID)
	if releaseErr != nil && !errors.Is(releaseErr, core.ErrNotFound) && !os.IsNotExist(releaseErr) {
		return releaseErr
	}

	alias := agentSSHAlias(*sessionID)
	clientFS, err := resolveClientFilesystem(context.WithoutCancel(ctx))
	if err != nil {
		if releaseErr == nil {
			return errors.Join(
				fmt.Errorf("agent environment was released but client SSH configuration could not be resolved: %w", err),
				core.ErrRecoveryRequired,
			)
		}
		return fmt.Errorf("session was already released but stale client SSH configuration could not be resolved: %w", err)
	}
	managedPath := managedConfigPath(clientFS.Home, alias)
	managed, readErr := readManagedSSHConfig(managedPath)
	if readErr != nil && !os.IsNotExist(readErr) {
		return errors.Join(readErr, core.ErrRecoveryRequired)
	}
	if readErr == nil && (managed.Alias != alias || managed.Distro != os.Getenv("WSL_DISTRO_NAME")) {
		return core.ErrIncompatibleState
	}
	if err := os.Remove(managedPath); err != nil && !os.IsNotExist(err) {
		if releaseErr == nil {
			return errors.Join(
				fmt.Errorf("agent environment was released but managed SSH config could not be removed: %w", err),
				core.ErrRecoveryRequired,
			)
		}
		return fmt.Errorf("remove stale managed SSH config: %w", err)
	}
	_, err = fmt.Fprintf(os.Stdout, "released: %s\n", alias)
	return err
}

func cleanupPreparedConnection(ctx context.Context, app *composition.App, environment, connectionID string, cause error) error {
	if connectionID == "" {
		return cause
	}
	cleanupErr := app.Clients.Unforward(context.WithoutCancel(ctx), environment, connectionID)
	if cleanupErr == nil {
		return cause
	}
	return errors.Join(
		cause,
		fmt.Errorf("cleanup failed after agent-host adapter setup error: %w", cleanupErr),
		core.ErrRecoveryRequired,
	)
}

// Grant rotation changes only the access grant; the Environment incarnation,
// Workspace, access mode and service must remain bound to the same target.
func sameSSHEnvironment(previous, next *core.StreamTarget) bool {
	if previous == nil || next == nil {
		return false
	}
	a, b := *previous, *next
	a.Grant, b.Grant = "", ""
	return a == b
}

func findSSHConnection(connections []core.ClientConnection, id string) core.ClientConnection {
	if id == "" {
		return core.ClientConnection{}
	}
	for _, connection := range connections {
		if connection.Kind == "ssh" && connection.ID == id && connection.Port == 0 && connection.Target != nil {
			return connection
		}
	}
	return core.ClientConnection{}
}

func reusableSSHConnection(previous managedSSHConfig, alias, identity string, connections []core.ClientConnection) core.ClientConnection {
	if previous.Alias != alias || previous.Connection.Target == nil || previous.IdentityFile != identity || previous.Distro != os.Getenv("WSL_DISTRO_NAME") {
		return core.ClientConnection{}
	}
	connection := findSSHConnection(connections, previous.Connection.ID)
	if connection.Target == nil || *connection.Target != *previous.Connection.Target || connection.HostPublicKey != previous.Connection.HostPublicKey {
		return core.ClientConnection{}
	}
	return connection
}

func resolveClientFilesystem(ctx context.Context) (clientFilesystem, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return clientFilesystem{}, fmt.Errorf("resolve home directory: %w", err)
	}
	if strings.TrimSpace(os.Getenv("WSL_DISTRO_NAME")) == "" {
		return clientFilesystem{Home: home}, nil
	}
	profileCommand := exec.CommandContext(ctx, "cmd.exe", "/C", "echo", "%USERPROFILE%")
	profileOutput, err := profileCommand.Output()
	if err != nil {
		return clientFilesystem{}, fmt.Errorf("resolve Windows user profile from WSL: %w", err)
	}
	windowsProfile := strings.TrimSpace(strings.ReplaceAll(string(profileOutput), "\r", ""))
	if windowsProfile == "" {
		return clientFilesystem{}, fmt.Errorf("resolve Windows user profile from WSL: %w", core.ErrNotFound)
	}
	wslPathCommand := exec.CommandContext(ctx, "wslpath", "-u", windowsProfile)
	wslPathOutput, err := wslPathCommand.Output()
	if err != nil {
		return clientFilesystem{}, fmt.Errorf("translate Windows user profile for WSL: %w", err)
	}
	clientHome := strings.TrimSpace(string(wslPathOutput))
	if clientHome == "" {
		return clientFilesystem{}, fmt.Errorf("translate Windows user profile for WSL: %w", core.ErrNotFound)
	}
	return clientFilesystem{Home: filepath.Clean(clientHome), WSL: true}, nil
}

func identityForClientConfig(ctx context.Context, clientFS clientFilesystem, identityPath string) (string, error) {
	relative, err := filepath.Rel(clientFS.Home, identityPath)
	if err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "~/" + filepath.ToSlash(relative), nil
	}
	if !clientFS.WSL {
		return filepath.Clean(identityPath), nil
	}
	command := exec.CommandContext(ctx, "wslpath", "-w", identityPath)
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("translate SSH identity path for Windows VS Code: %w", err)
	}
	translated := strings.TrimSpace(strings.ReplaceAll(string(output), "\r", ""))
	if translated == "" {
		return "", fmt.Errorf("translate SSH identity path for Windows VS Code: %w", core.ErrNotFound)
	}
	return strings.ReplaceAll(translated, "\\", "/"), nil
}

func resolveWorkspacePath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve workspace path: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolve workspace %q: %w", path, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workspace %q is not a directory: %w", resolved, core.ErrInvalidArgument)
	}
	return filepath.Clean(resolved), nil
}

func agentSSHAlias(sessionID string) string {
	sum := sha256.Sum256([]byte(sessionID))
	return fmt.Sprintf("haco-agent-%x", sum[:8])
}

func managedConfigPath(home, alias string) string {
	return filepath.Join(home, ".ssh", "hacocoon", alias+".conf")
}

func ensureSSHInclude(home string) error {
	sshDir := filepath.Join(home, ".ssh")
	managedDir := filepath.Join(sshDir, "hacocoon")
	if err := os.MkdirAll(managedDir, 0o700); err != nil {
		return fmt.Errorf("create Hacocoon SSH config directory: %w", err)
	}
	_ = os.Chmod(managedDir, 0o700)
	configPath := filepath.Join(sshDir, "config")
	content, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read SSH config: %w", err)
	}
	include := "Include ~/.ssh/hacocoon/*.conf"
	for _, line := range strings.Split(string(content), "\n") {
		if strings.TrimSpace(line) == include {
			return nil
		}
	}
	file, err := os.OpenFile(configPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open SSH config: %w", err)
	}
	defer func() { _ = file.Close() }()
	prefix := ""
	if len(content) > 0 && !strings.HasSuffix(string(content), "\n") {
		prefix = "\n"
	}
	if _, err := fmt.Fprintf(file, "%s%s\n", prefix, include); err != nil {
		return fmt.Errorf("write SSH config include: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close SSH config include: %w", err)
	}
	return nil
}

func writeManagedSSHConfig(path string, config managedSSHConfig) error {
	c := config.Connection
	command, commandErr := sshconfig.StreamCommand(c.Target, config.Distro)
	hostKey, keyErr := sshkey.NormalizePublicKey(c.HostPublicKey)
	if commandErr != nil || keyErr != nil || c.Kind != "ssh" || c.Host != "" || c.Port != 0 || c.TargetPort != 22 || c.User != "root" || c.Target.Grant != c.ID || !safeSSHAlias(config.Alias) {
		return core.ErrInvalidArgument
	}
	if err := validateSSHConfigValue(config.IdentityFile); err != nil {
		return err
	}
	meta, err := json.Marshal(config)
	if err != nil {
		return err
	}
	content := fmt.Sprintf("# Hacocoon agent connection %s\nHost %s\n    HostName %s\n    User root\n    IdentityFile %s\n    IdentitiesOnly yes\n    StrictHostKeyChecking yes\n    HostKeyAlias %s\n    UserKnownHostsFile ~/.ssh/hacocoon/%s.known_hosts\n    GlobalKnownHostsFile none\n    ProxyCommand %s\n", meta, config.Alias, config.Alias, quoteSSHValue(config.IdentityFile), config.Alias, config.Alias, command)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create managed SSH config directory: %w", err)
	}
	knownPath := filepath.Join(dir, config.Alias+".known_hosts")
	known := []byte(config.Alias + " " + hostKey + "\n")
	if existing, err := os.ReadFile(knownPath); err == nil {
		if string(existing) != string(known) {
			return core.ErrIncompatibleState
		}
	} else if !os.IsNotExist(err) {
		return err
	} else {
		keyFile, err := os.OpenFile(knownPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		_, writeErr := keyFile.Write(known)
		closeErr := keyFile.Close()
		if err = errors.Join(writeErr, closeErr); err != nil {
			return err
		}
	}
	temp, err := os.CreateTemp(dir, ".haco-agent-host-*.tmp")
	if err != nil {
		return fmt.Errorf("create managed SSH config temp file: %w", err)
	}
	tempPath := temp.Name()
	cleanup := func() {
		_ = temp.Close()
		_ = os.Remove(tempPath)
	}
	defer cleanup()
	_ = temp.Chmod(0o600)
	if _, err := temp.WriteString(content); err != nil {
		return fmt.Errorf("write managed SSH config: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync managed SSH config: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close managed SSH config: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace managed SSH config: %w", err)
	}
	_ = os.Chmod(path, 0o600)
	return nil
}

func readManagedSSHConfig(path string) (managedSSHConfig, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return managedSSHConfig{}, err
	}
	var config managedSSHConfig
	first, _, _ := strings.Cut(string(content), "\n")
	const prefix = "# Hacocoon agent connection "
	if !strings.HasPrefix(first, prefix) || json.Unmarshal([]byte(strings.TrimPrefix(first, prefix)), &config) != nil {
		return config, core.ErrIncompatibleState
	}
	return config, nil
}

func safeSSHAlias(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return true
}

func validateSSHConfigValue(value string) error {
	if value == "" {
		return core.ErrInvalidArgument
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return core.ErrInvalidArgument
		}
	}
	return nil
}

func quoteSSHValue(value string) string {
	return "\"" + strings.ReplaceAll(value, "\"", "\\\"") + "\""
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: haco-agent-host <prepare|lookup|release> [options]")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
