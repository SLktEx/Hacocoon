package incus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const managedSSHProvisionScript = `
set -eu
export DEBIAN_FRONTEND=noninteractive
if ! command -v sshd >/dev/null 2>&1; then
  apt-get update
  apt-get install -y --no-install-recommends openssh-server
fi
systemctl enable --now ssh
install -d -m 0700 /root/.ssh
key="$1"
marker="$2"
tmp="$(mktemp /root/.ssh/authorized_keys.haco.XXXXXX)"
trap 'rm -f "$tmp"' EXIT
if [ -f /root/.ssh/authorized_keys ]; then
  awk -v marker="$marker" 'index($0, " " marker) == 0 { print }' /root/.ssh/authorized_keys > "$tmp"
fi
printf '%s %s\n' "$key" "$marker" >> "$tmp"
chmod 0600 "$tmp"
mv "$tmp" /root/.ssh/authorized_keys
trap - EXIT
`

const managedSSHRevokeScript = `
set -eu
file=/root/.ssh/authorized_keys
marker="$1"
[ -f "$file" ] || exit 0
tmp="$(mktemp /root/.ssh/authorized_keys.haco.XXXXXX)"
trap 'rm -f "$tmp"' EXIT
awk -v marker="$marker" 'index($0, " " marker) == 0 { print }' "$file" > "$tmp"
chmod 0600 "$tmp"
mv "$tmp" "$file"
trap - EXIT
`

func (r *Runtime) PrepareSSHAccess(ctx context.Context, ref string, req core.SSHAccessRequest) (core.ClientConnection, error) {
	if err := validateDirectSSHRequest(req); err != nil {
		return core.ClientConnection{}, err
	}
	if err := validateManagedRef(ref); err != nil {
		return core.ClientConnection{}, err
	}
	id := fmt.Sprintf("ssh-%d", req.HostPort)
	if err := r.addLoopbackProxy(ctx, ref, id, req.HostPort, 22); err != nil {
		return core.ClientConnection{}, err
	}

	marker := "haco:" + id
	if _, err := r.runner.Run(ctx, "incus", "exec", ref, "--project", r.project, "--", "sh", "-ceu", managedSSHProvisionScript, "haco-ssh", req.PublicKey, marker); err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), r.cleanupTimeout)
		defer cancel()
		cleanupErr := r.RemoveClientConnection(cleanupCtx, ref, id)
		return core.ClientConnection{}, errors.Join(fmt.Errorf("prepare SSH in %s: %w", ref, err), cleanupErr)
	}

	return core.ClientConnection{
		ID:         id,
		Kind:       "ssh",
		Host:       "127.0.0.1",
		Port:       req.HostPort,
		TargetPort: 22,
		User:       "root",
		Command:    fmt.Sprintf("ssh -p %d root@127.0.0.1", req.HostPort),
	}, nil
}

func (r *Runtime) RevokeSSHAccess(ctx context.Context, ref, connectionID string) error {
	if err := validateManagedRef(ref); err != nil {
		return err
	}
	if err := validateSSHConnectionID(connectionID); err != nil {
		return err
	}
	marker := "haco:" + connectionID
	if _, err := r.runner.Run(ctx, "incus", "exec", ref, "--project", r.project, "--", "sh", "-ceu", managedSSHRevokeScript, "haco-ssh-revoke", marker); err != nil {
		return fmt.Errorf("revoke SSH key %s in %s: %w", connectionID, ref, err)
	}
	if err := r.RemoveClientConnection(ctx, ref, connectionID); err != nil {
		return fmt.Errorf("remove SSH proxy %s: %w", connectionID, err)
	}
	return nil
}

func validateDirectSSHRequest(req core.SSHAccessRequest) error {
	key := strings.TrimSpace(req.PublicKey)
	if req.HostPort < 1 || req.HostPort > 65535 || key == "" || key != req.PublicKey || strings.ContainsAny(key, "\r\n\x00") {
		return fmt.Errorf("SSH access request: %w", core.ErrInvalidArgument)
	}
	return nil
}

func validateSSHConnectionID(connectionID string) error {
	if !strings.HasPrefix(connectionID, "ssh-") {
		return fmt.Errorf("SSH connection id %q: %w", connectionID, core.ErrInvalidArgument)
	}
	rawPort := strings.TrimPrefix(connectionID, "ssh-")
	port, err := strconv.Atoi(rawPort)
	if err != nil || port < 1 || port > 65535 || fmt.Sprintf("ssh-%d", port) != connectionID {
		return fmt.Errorf("SSH connection id %q: %w", connectionID, core.ErrInvalidArgument)
	}
	return nil
}

func (r *Runtime) ListClientConnections(ctx context.Context, ref string) ([]core.ClientConnection, error) {
	if err := validateManagedRef(ref); err != nil {
		return nil, err
	}
	result, err := r.runner.Run(ctx, "incus", "config", "show", ref, "--project", r.project, "--format", "json")
	if err != nil {
		return nil, err
	}
	var config struct {
		Devices map[string]map[string]string `json:"devices"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &config); err != nil {
		return nil, fmt.Errorf("decode Incus client devices: %w", err)
	}

	connections := make([]core.ClientConnection, 0)
	for deviceName, device := range config.Devices {
		if device["type"] != "proxy" || !strings.HasPrefix(deviceName, "haco-") {
			continue
		}
		id := strings.TrimPrefix(deviceName, "haco-")
		connection, err := clientConnectionFromProxy(id, device["listen"], device["connect"])
		if err != nil {
			return nil, fmt.Errorf("reconcile client device %q: %w", deviceName, err)
		}
		connections = append(connections, connection)
	}
	sort.Slice(connections, func(i, j int) bool { return connections[i].ID < connections[j].ID })
	return connections, nil
}

func clientConnectionFromProxy(id, listen, connect string) (core.ClientConnection, error) {
	listenHost, listenPort, err := parseTCPProxyEndpoint(listen)
	if err != nil {
		return core.ClientConnection{}, fmt.Errorf("listen endpoint: %w", err)
	}
	_, targetPort, err := parseTCPProxyEndpoint(connect)
	if err != nil {
		return core.ClientConnection{}, fmt.Errorf("connect endpoint: %w", err)
	}
	if listenHost != "127.0.0.1" {
		return core.ClientConnection{}, fmt.Errorf("managed proxy %q is not loopback-only: %w", id, core.ErrUnsupported)
	}

	connection := core.ClientConnection{
		ID:         id,
		Kind:       "tcp",
		Host:       listenHost,
		Port:       listenPort,
		TargetPort: targetPort,
	}
	if strings.HasPrefix(id, "ssh-") && targetPort == 22 {
		connection.Kind = "ssh"
		connection.User = "root"
		connection.Command = fmt.Sprintf("ssh -p %d root@127.0.0.1", listenPort)
	}
	return connection, nil
}

func parseTCPProxyEndpoint(endpoint string) (string, int, error) {
	if !strings.HasPrefix(endpoint, "tcp:") {
		return "", 0, fmt.Errorf("endpoint %q is not tcp: %w", endpoint, core.ErrUnsupported)
	}
	host, rawPort, err := net.SplitHostPort(strings.TrimPrefix(endpoint, "tcp:"))
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil || port < 1 || port > 65535 {
		return "", 0, fmt.Errorf("invalid port %q: %w", rawPort, core.ErrInvalidArgument)
	}
	return host, port, nil
}
