package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	agenthostapp "github.com/SLktEx/Hacocoon/internal/agenthost"
	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/core"
)

const remoteWorkspacePath = "/workspace"

type managedSSHConfig struct {
	Alias        string
	Port         int
	IdentityFile string
}

type clientFilesystem struct {
	Home string
	WSL  bool
}

func main() {
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
	hostPort := fs.Int("host-port", 0, "loopback SSH port (0 chooses a free port)")
	codeCommand := fs.String("code", "code", "VS Code CLI command")
	noLaunch := fs.Bool("no-launch", false, "prepare the remote host without opening the VS Code Agents window")
	readOnly := fs.Bool("read-only", false, "create a read-only Workspace lease")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *sessionID == "" || fs.NArg() > 1 {
		return fmt.Errorf("usage: haco-agent-host prepare --session <id> [options] [workspace]: %w", core.ErrInvalidArgument)
	}
	if *hostPort < 0 || *hostPort > 65535 {
		return fmt.Errorf("host port %d: %w", *hostPort, core.ErrInvalidArgument)
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
	previous, _ := readManagedSSHConfig(managedPath)
	connections, err := app.Clients.Connections(ctx, binding.EnvironmentName)
	if err != nil {
		return err
	}
	oldConnection := findSSHConnection(connections, previous.Port)
	connection := reusableSSHConnection(previous, alias, identityConfigValue, *hostPort, connections)
	preparedConnectionID := ""

	if connection.Port == 0 {
		if *hostPort != 0 && oldConnection.Port == *hostPort {
			return fmt.Errorf(
				"host port %d is still owned by the previous Hacocoon agent-host SSH connection; omit --host-port to rotate safely or release the old session first: %w",
				*hostPort,
				core.ErrAlreadyExists,
			)
		}
		port := *hostPort
		if port == 0 {
			port, err = freeLoopbackPort()
			if err != nil {
				return err
			}
		}
		connection, err = app.Clients.SSH(ctx, binding.EnvironmentName, core.SSHAccessRequest{
			PublicKey: string(publicKey),
			HostPort:  port,
		})
		if err != nil {
			return fmt.Errorf("prepare SSH for agent environment %q: %w", binding.EnvironmentName, err)
		}
		preparedConnectionID = connection.ID
	}

	if err := ensureSSHInclude(clientFS.Home); err != nil {
		return cleanupPreparedConnection(ctx, app, binding.EnvironmentName, preparedConnectionID, err)
	}
	managed := managedSSHConfig{Alias: alias, Port: connection.Port, IdentityFile: identityConfigValue}
	if err := writeManagedSSHConfig(managedPath, managed); err != nil {
		return cleanupPreparedConnection(ctx, app, binding.EnvironmentName, preparedConnectionID, err)
	}

	if oldConnection.Port != 0 && oldConnection.ID != connection.ID {
		if err := app.Clients.Unforward(context.WithoutCancel(ctx), binding.EnvironmentName, oldConnection.ID); err != nil {
			return errors.Join(
				fmt.Errorf("new agent-host SSH connection is active but old managed SSH connection %q could not be revoked: %w", oldConnection.ID, err),
				core.ErrRecoveryRequired,
			)
		}
	}

	fmt.Printf("environment: %s\n", binding.EnvironmentName)
	fmt.Printf("ssh: %s\n", alias)
	fmt.Printf("workspace: %s\n", remoteWorkspacePath)
	fmt.Printf("agents: New -> Remote -> SSH -> %s\n", alias)
	if *noLaunch {
		return nil
	}

	cmd := exec.CommandContext(ctx, *codeCommand, "--agents")
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
	if err := os.Remove(managedPath); err != nil && !os.IsNotExist(err) {
		if releaseErr == nil {
			return errors.Join(
				fmt.Errorf("agent environment was released but managed SSH config could not be removed: %w", err),
				core.ErrRecoveryRequired,
			)
		}
		return fmt.Errorf("remove stale managed SSH config: %w", err)
	}
	fmt.Printf("released: %s\n", alias)
	return nil
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

func findSSHConnection(connections []core.ClientConnection, port int) core.ClientConnection {
	if port == 0 {
		return core.ClientConnection{}
	}
	for _, connection := range connections {
		if connection.Kind == "ssh" && connection.Port == port {
			return connection
		}
	}
	return core.ClientConnection{}
}

func reusableSSHConnection(previous managedSSHConfig, alias, identity string, requestedPort int, connections []core.ClientConnection) core.ClientConnection {
	if previous.Alias != alias || previous.Port == 0 || previous.IdentityFile != identity {
		return core.ClientConnection{}
	}
	if requestedPort != 0 && requestedPort != previous.Port {
		return core.ClientConnection{}
	}
	return findSSHConnection(connections, previous.Port)
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

func freeLoopbackPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("choose SSH loopback port: %w", err)
	}
	defer listener.Close()
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok || address.Port < 1 {
		return 0, fmt.Errorf("choose SSH loopback port: %w", core.ErrRuntimeUnavailable)
	}
	return address.Port, nil
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
	defer file.Close()
	prefix := ""
	if len(content) > 0 && !strings.HasSuffix(string(content), "\n") {
		prefix = "\n"
	}
	if _, err := fmt.Fprintf(file, "%s%s\n", prefix, include); err != nil {
		return fmt.Errorf("write SSH config include: %w", err)
	}
	return nil
}

func writeManagedSSHConfig(path string, config managedSSHConfig) error {
	if config.Port < 1 || config.Port > 65535 || !safeSSHAlias(config.Alias) {
		return core.ErrInvalidArgument
	}
	if err := validateSSHConfigValue(config.IdentityFile); err != nil {
		return err
	}
	content := fmt.Sprintf(
		"Host %s\n    HostName 127.0.0.1\n    User root\n    Port %d\n    IdentityFile %s\n    IdentitiesOnly yes\n",
		config.Alias,
		config.Port,
		quoteSSHValue(config.IdentityFile),
	)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create managed SSH config directory: %w", err)
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
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}
		switch strings.ToLower(fields[0]) {
		case "host":
			config.Alias = fields[1]
		case "port":
			config.Port, _ = strconv.Atoi(fields[1])
		case "identityfile":
			config.IdentityFile = strings.Trim(strings.Join(fields[1:], " "), "\"")
		}
	}
	return config, nil
}

func safeSSHAlias(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
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
	fmt.Fprintln(os.Stderr, "usage: haco-agent-host <prepare|release> [options]")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
