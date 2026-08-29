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
	"regexp"
	"strconv"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/core"
)

const remoteWorkspacePath = "/workspace"

var nonNameCharacter = regexp.MustCompile(`[^a-z0-9]+`)

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
	case "open":
		return openCommand(ctx, app, args[1:])
	case "delete":
		return deleteCommand(ctx, app, args[1:])
	default:
		usage()
		return fmt.Errorf("unknown command %q: %w", args[0], core.ErrInvalidArgument)
	}
}

func openCommand(ctx context.Context, app *composition.App, args []string) error {
	fs := flag.NewFlagSet("open", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	name := fs.String("name", "", "Hacocoon Environment name")
	identity := fs.String("identity", "", "SSH private key used by the VS Code client")
	hostPort := fs.Int("host-port", 0, "loopback SSH port (0 chooses a free port)")
	codeCommand := fs.String("code", "code", "VS Code CLI command")
	noLaunch := fs.Bool("no-launch", false, "prepare the connection without launching VS Code")
	readOnly := fs.Bool("read-only", false, "create a read-only Workspace lease")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		return fmt.Errorf("usage: haco-vscode open [options] [workspace]: %w", core.ErrInvalidArgument)
	}
	workspaceArg := "."
	if fs.NArg() == 1 {
		workspaceArg = fs.Arg(0)
	}
	workspace, err := resolveWorkspacePath(workspaceArg)
	if err != nil {
		return err
	}
	if *name == "" {
		*name = defaultEnvironmentName(workspace)
	}
	if *hostPort < 0 || *hostPort > 65535 {
		return fmt.Errorf("host port %d: %w", *hostPort, core.ErrInvalidArgument)
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
	publicKeyPath := identityPath + ".pub"
	publicKey, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return fmt.Errorf("read SSH public key %q: %w", publicKeyPath, err)
	}

	status, err := app.Clients.Status(ctx, *name)
	created := false
	if err != nil {
		if !errors.Is(err, core.ErrNotFound) && !os.IsNotExist(err) {
			return err
		}
		mode := core.WorkspaceReadWrite
		if *readOnly {
			mode = core.WorkspaceReadOnly
		}
		_, err = app.Environments.Create(ctx, core.EnvironmentSpec{
			Name:          *name,
			WorkspacePath: workspace,
			AccessMode:    mode,
		})
		if err != nil {
			return err
		}
		created = true
		status, err = app.Clients.Status(ctx, *name)
		if err != nil {
			return err
		}
	}
	if filepath.Clean(status.Environment.Workspace.Path) != filepath.Clean(workspace) {
		return fmt.Errorf("environment %q belongs to workspace %q, not %q: %w", *name, status.Environment.Workspace.Path, workspace, core.ErrAlreadyExists)
	}
	if status.State != core.EnvironmentRunning {
		return fmt.Errorf("environment %q is %s; VS Code adapter currently requires a running environment: %w", *name, status.State, core.ErrUnsupported)
	}

	alias := "haco-vscode-" + *name
	managedPath := managedConfigPath(clientFS.Home, *name)
	previous, _ := readManagedSSHConfig(managedPath)
	connections, err := app.Clients.Connections(ctx, *name)
	if err != nil {
		return err
	}

	var connection core.ClientConnection
	if previous.Port != 0 && previous.IdentityFile == identityConfigValue {
		for _, candidate := range connections {
			if candidate.Kind == "ssh" && candidate.Port == previous.Port {
				connection = candidate
				break
			}
		}
	}

	if connection.Port == 0 {
		port := *hostPort
		if port == 0 {
			port, err = freeLoopbackPort()
			if err != nil {
				return err
			}
		}
		connection, err = app.Clients.SSH(ctx, *name, core.SSHAccessRequest{
			PublicKey: string(publicKey),
			HostPort:  port,
		})
		if err != nil {
			if created {
				return fmt.Errorf("prepare SSH for newly created environment %q: %w", *name, err)
			}
			return err
		}
	}

	if err := ensureSSHInclude(clientFS.Home); err != nil {
		return err
	}
	managed := managedSSHConfig{Alias: alias, Port: connection.Port, IdentityFile: identityConfigValue}
	if err := writeManagedSSHConfig(managedPath, managed); err != nil {
		return err
	}

	fmt.Printf("environment: %s\nssh: %s\nworkspace: %s\n", *name, alias, remoteWorkspacePath)
	if *noLaunch {
		return nil
	}
	cmd := exec.CommandContext(ctx, *codeCommand, "--remote", "ssh-remote+"+alias, remoteWorkspacePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("launch VS Code: %w", err)
	}
	return nil
}

func deleteCommand(ctx context.Context, app *composition.App, args []string) error {
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	name := fs.String("name", "", "Hacocoon Environment name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		return fmt.Errorf("usage: haco-vscode delete [--name <environment>] [workspace]: %w", core.ErrInvalidArgument)
	}
	clientFS, err := resolveClientFilesystem(ctx)
	if err != nil {
		return err
	}
	if *name == "" {
		workspaceArg := "."
		if fs.NArg() == 1 {
			workspaceArg = fs.Arg(0)
		}
		workspace, err := resolveWorkspacePath(workspaceArg)
		if err != nil {
			return err
		}
		*name = defaultEnvironmentName(workspace)
	}
	if err := app.Environments.Delete(ctx, *name); err != nil {
		return err
	}
	path := managedConfigPath(clientFS.Home, *name)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove managed SSH config: %w", err)
	}
	fmt.Printf("deleted: %s\n", *name)
	return nil
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

func defaultEnvironmentName(workspace string) string {
	base := strings.ToLower(filepath.Base(workspace))
	base = nonNameCharacter.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-")
	if base == "" {
		base = "workspace"
	}
	if len(base) > 38 {
		base = strings.Trim(base[:38], "-")
	}
	sum := sha256.Sum256([]byte(filepath.Clean(workspace)))
	return fmt.Sprintf("vscode-%s-%x", base, sum[:4])
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

func managedConfigPath(home, environment string) string {
	return filepath.Join(home, ".ssh", "hacocoon", environment+".conf")
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
	if config.Port < 1 || config.Port > 65535 || config.Alias == "" || config.IdentityFile == "" {
		return core.ErrInvalidArgument
	}
	content := fmt.Sprintf("Host %s\n    HostName 127.0.0.1\n    User root\n    Port %d\n    IdentityFile %s\n    IdentitiesOnly yes\n", config.Alias, config.Port, quoteSSHValue(config.IdentityFile))
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write managed SSH config: %w", err)
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

func quoteSSHValue(value string) string {
	return "\"" + strings.ReplaceAll(value, "\"", "\\\"") + "\""
}

func samePath(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && filepath.Clean(leftAbs) == filepath.Clean(rightAbs)
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: haco-vscode <open|delete> [options] [workspace]")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
