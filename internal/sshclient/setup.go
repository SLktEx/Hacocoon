//go:build linux

// Package sshclient owns desktop SSH files; controller APIs receive public keys only.
package sshclient

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"unicode/utf8"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/sshkey"
)

type Controller interface {
	EnvironmentStatus(context.Context, string) (core.EnvironmentStatus, error)
	EnvironmentConnections(context.Context, string) ([]core.ClientConnection, error)
	StartEnvironment(context.Context, string) error
	PrepareEnvironmentSSH(context.Context, string, core.SSHAccessRequest) (core.ClientConnection, error)
	UnforwardEnvironment(context.Context, string, string) error
}
type Desktop struct {
	Home, NativeHome string
	Windows          bool
}
type saved struct {
	Runtime    string
	PublicKey  string
	Connection core.ClientConnection
}

const include = "Include ~/.ssh/hacocoon/*.conf\n"
const prefix = "# Hacocoon connection "

var namePattern = regexp.MustCompile("^[a-z0-9][a-z0-9-]{0,56}$")

func ResolveDesktop(ctx context.Context) (Desktop, error) {
	if os.Getenv("WSL_INTEROP") == "" && os.Getenv("WSL_DISTRO_NAME") == "" {
		home, err := os.UserHomeDir()
		return Desktop{Home: home}, err
	}
	command := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new(); ConvertTo-Json -Compress $env:USERPROFILE")
	b, err := command.Output()
	if err != nil {
		return Desktop{}, fmt.Errorf("resolve Windows SSH home: %w", err)
	}
	var native string
	if json.Unmarshal(b, &native) != nil || len(native) < 4 || native[1:3] != ":\\" || !((native[0] >= 'A' && native[0] <= 'Z') || (native[0] >= 'a' && native[0] <= 'z')) || strings.ContainsAny(native, "\r\n\x00") {
		return Desktop{}, fmt.Errorf("invalid Windows SSH home")
	}
	home := "/mnt/" + strings.ToLower(native[:1]) + "/" + strings.ReplaceAll(native[3:], "\\", "/")
	return Desktop{Home: home, NativeHome: native, Windows: true}, nil
}

func Setup(ctx context.Context, c Controller, d Desktop, name string) (alias string, resultErr error) {
	if !namePattern.MatchString(name) {
		return "", core.ErrInvalidArgument
	}
	f, err := openFiles(ctx, d.Home, d.Windows)
	if err != nil {
		return "", err
	}
	defer f.close()
	key, err := identity(ctx, f, d)
	if err != nil {
		return "", err
	}
	config, err := f.read("config")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if !utf8.Valid(config) || strings.IndexByte(string(config), 0) >= 0 {
		return "", fmt.Errorf("SSH config must use UTF-8 text")
	}
	config = []byte(strings.TrimPrefix(string(config), "\ufeff"))
	if !strings.HasPrefix(string(config), include) {
		if err = f.replace("config", append([]byte(include), config...)); err != nil {
			return "", err
		}
	}
	path := "hacocoon/" + name + ".conf"
	var previous saved
	old, err := f.read(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if len(old) > 0 {
		first, _, _ := strings.Cut(string(old), "\n")
		if !strings.HasPrefix(first, prefix) || json.Unmarshal([]byte(strings.TrimPrefix(first, prefix)), &previous) != nil {
			return "", fmt.Errorf("existing SSH entry is not managed by this client")
		}
	}
	status, err := c.EnvironmentStatus(ctx, name)
	if err != nil {
		return "", err
	}
	if status.State == core.EnvironmentStopped {
		if err = c.StartEnvironment(ctx, name); err != nil {
			return "", err
		}
		next, err := c.EnvironmentStatus(ctx, name)
		if err != nil {
			return "", err
		}
		if next.Environment.RuntimeRef != status.Environment.RuntimeRef {
			return "", core.ErrIncompatibleState
		}
		status = next
	}
	if status.State != core.EnvironmentRunning || status.Environment.RuntimeRef == "" {
		return "", core.ErrIncompatibleState
	}
	if previous.Runtime == status.Environment.RuntimeRef && previous.PublicKey == key {
		connections, err := c.EnvironmentConnections(ctx, name)
		if err != nil {
			return "", err
		}
		for _, actual := range connections {
			if sameConnection(actual, previous.Connection) {
				text, known, err := render(name, previous)
				if err != nil {
					return "", err
				}
				if err = f.replace(knownPath(previous), []byte(known)); err != nil {
					return "", err
				}
				if err = f.replace(path, []byte(text)); err != nil {
					return "", err
				}
				return "haco-" + name, nil
			}
		}
	}
	conn, err := c.PrepareEnvironmentSSH(ctx, name, core.SSHAccessRequest{PublicKey: key})
	if err != nil {
		return "", err
	}
	keep := false
	defer func() {
		if !keep {
			resultErr = errors.Join(resultErr, fmt.Errorf("prepared SSH connection %q retained for inspection: %w", conn.ID, core.ErrRecoveryRequired))
		}
	}()
	observed, err := c.EnvironmentStatus(ctx, name)
	if err != nil {
		return "", err
	}
	if observed.Environment.RuntimeRef != status.Environment.RuntimeRef {
		return "", core.ErrRecoveryRequired
	}
	next := saved{Runtime: status.Environment.RuntimeRef, PublicKey: key, Connection: conn}
	text, known, err := render(name, next)
	if err != nil {
		return "", err
	}
	if previous.Runtime == next.Runtime && previous.Connection.HostPublicKey != "" && previous.Connection.HostPublicKey != conn.HostPublicKey {
		return "", fmt.Errorf("SSH host key changed for the same Environment")
	}
	knownPath := knownPath(next)
	if err = f.replace(knownPath, []byte(known)); err != nil {
		return "", err
	}
	if err = f.replace(path, []byte(text)); err != nil {
		return "", err
	}
	keep = true
	return "haco-" + name, nil
}
func sameConnection(a, b core.ClientConnection) bool {
	return a.ID == b.ID && a.Kind == b.Kind && a.Host == b.Host && a.Port == b.Port && a.TargetPort == b.TargetPort && a.User == b.User
}
func knownPath(s saved) string {
	hash := sha256.Sum256([]byte(s.Runtime + "\n" + s.Connection.HostPublicKey))
	return fmt.Sprintf("hacocoon/known-%x", hash[:16])
}
func render(name string, s saved) (string, string, error) {
	c := s.Connection
	ip := net.ParseIP(c.Host)
	key, err := sshkey.NormalizePublicKey(c.HostPublicKey)
	if err != nil || ip == nil || !ip.IsLoopback() || c.Kind != "ssh" || c.User != "root" || c.TargetPort != 22 || c.Port < 1 || c.Port > 65535 || !namePattern.MatchString(name) || s.Runtime == "" {
		return "", "", core.ErrIncompatibleState
	}
	meta, err := json.Marshal(s)
	if err != nil {
		return "", "", err
	}
	alias := "haco-" + name
	content := fmt.Sprintf("%s%s\nHost %s\n  HostName %s\n  Port %d\n  User root\n  IdentityFile ~/.ssh/hacocoon/identity\n  IdentitiesOnly yes\n  StrictHostKeyChecking yes\n  HostKeyAlias %s\n  UserKnownHostsFile ~/.ssh/%s\n  GlobalKnownHostsFile none\n  CheckHostIP no\n  ProxyCommand none\n  ProxyJump none\n  ForwardAgent no\n  ClearAllForwardings no\n  GatewayPorts no\n  PermitLocalCommand no\n", prefix, meta, alias, ip.String(), c.Port, alias, knownPath(s))
	return content, alias + " " + key + "\n", nil
}
func identity(ctx context.Context, f *files, d Desktop) (string, error) {
	const pub = "hacocoon/identity.pub"
	b, err := f.read(pub)
	if err == nil {
		private, err := f.root.OpenFile("hacocoon/identity", os.O_RDONLY|syscall.O_NOFOLLOW, 0)
		if err != nil {
			return "", err
		}
		err = regular(private)
		private.Close()
		if err != nil {
			return "", err
		}
		return sshkey.NormalizePublicKey(string(b))
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if _, err := f.root.Lstat("hacocoon/identity"); !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("SSH identity is incomplete; preserve it and inspect before retrying")
	}
	// Unique staging directory avoids ssh-keygen overwrite prompts after a crash.
	stage, err := os.MkdirTemp(filepath.Join(d.Home, ".ssh", "hacocoon"), "key-")
	if err != nil {
		return "", err
	}
	binary := "ssh-keygen"
	target := filepath.Join(stage, "identity")
	if d.Windows {
		binary = "ssh-keygen.exe"
		rel, err := filepath.Rel(d.Home, target)
		if err != nil {
			return "", err
		}
		target = d.NativeHome + "\\" + strings.ReplaceAll(rel, "/", "\\")
	}
	command := exec.CommandContext(ctx, binary, "-q", "-t", "ed25519", "-N", "", "-f", target)
	if err = command.Run(); err != nil {
		return "", fmt.Errorf("generate client SSH identity: %w", err)
	}
	relative := "hacocoon/" + filepath.Base(stage) + "/identity"
	b, err = f.read(relative + ".pub")
	if err != nil {
		return "", err
	}
	key, err := sshkey.NormalizePublicKey(string(b))
	if err != nil {
		return "", err
	}
	if err = f.root.Rename(relative, "hacocoon/identity"); err != nil {
		return "", err
	}
	if err = f.root.Rename(relative+".pub", pub); err != nil {
		return "", err
	}
	_ = f.root.Remove("hacocoon/" + filepath.Base(stage))
	return key, nil
}
