package kubernetes

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const (
	forwardStateVersion = 1
	forwardReadyTimeout = 10 * time.Second
	forwardStopTimeout  = 3 * time.Second
	forwardTokenEnv     = "HACO_KUBE_FORWARD_TOKEN"
)

const managedKubeSSHProvisionScript = `
set -eu
if ! command -v sshd >/dev/null 2>&1; then
  echo 'openssh-server is required in the Kubernetes Environment image' >&2
  exit 78
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

const managedKubeSSHRevokeScript = `
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

type forwardRecord struct {
	Version        int    `json:"version"`
	EnvironmentRef string `json:"environment_ref"`
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	HostPort       int    `json:"host_port"`
	TargetPort     int    `json:"target_port"`
	Token          string `json:"token"`
	PID            int    `json:"pid"`
	ProcStartTicks uint64 `json:"proc_start_ticks"`
	State          string `json:"state"`
}

func (p *Provider) ForwardLocalPort(ctx context.Context, ref string, req core.LocalPortRequest) (core.ClientConnection, error) {
	if req.Protocol != "" && req.Protocol != "tcp" {
		return core.ClientConnection{}, fmt.Errorf("protocol %q: %w", req.Protocol, core.ErrUnsupported)
	}
	if req.HostPort < 1 || req.HostPort > 65535 || req.TargetPort < 1 || req.TargetPort > 65535 {
		return core.ClientConnection{}, core.ErrInvalidArgument
	}
	if _, err := p.validateOwnedRef(ctx, ref); err != nil {
		return core.ClientConnection{}, err
	}
	id := fmt.Sprintf("tcp-%d-%d", req.HostPort, req.TargetPort)
	return p.createClientForward(ctx, ref, id, "tcp", req.HostPort, req.TargetPort)
}

func (p *Provider) RemoveClientConnection(ctx context.Context, ref, connectionID string) error {
	if _, err := p.validateOwnedRef(ctx, ref); err != nil {
		return err
	}
	if err := validateForwardID(connectionID); err != nil {
		return err
	}
	return p.stopClientForward(ctx, ref, connectionID)
}

func (p *Provider) ListClientConnections(ctx context.Context, ref string) ([]core.ClientConnection, error) {
	if _, err := p.validateOwnedRef(ctx, ref); err != nil {
		return nil, err
	}
	dir, err := kubeForwardStateDir()
	if err != nil {
		return nil, err
	}
	if err := ensurePrivateStateDir(dir); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	prefix := ref + "--"
	connections := make([]core.ClientConnection, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		record, err := readForwardRecord(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		if record.EnvironmentRef != ref {
			return nil, fmt.Errorf("forward record %q environment mismatch: %w", record.ID, core.ErrIncompatibleState)
		}
		alive, err := exactForwardProcessAlive(record)
		if err != nil {
			return nil, errors.Join(err, core.ErrRecoveryRequired)
		}
		if !alive {
			if err := removeForwardFiles(dir, record); err != nil {
				return nil, errors.Join(err, core.ErrRecoveryRequired)
			}
			continue
		}
		connections = append(connections, connectionFromForward(record))
	}
	sort.Slice(connections, func(i, j int) bool { return connections[i].ID < connections[j].ID })
	return connections, nil
}

func (p *Provider) PrepareSSH(ctx context.Context, ref string, req core.SSHAccessRequest) (core.ClientConnection, error) {
	return p.PrepareSSHAccess(ctx, ref, req)
}

func (p *Provider) PrepareSSHAccess(ctx context.Context, ref string, req core.SSHAccessRequest) (core.ClientConnection, error) {
	if req.HostPort < 1 || req.HostPort > 65535 || strings.TrimSpace(req.PublicKey) == "" || strings.ContainsAny(req.PublicKey, "\r\n\x00") {
		return core.ClientConnection{}, core.ErrInvalidArgument
	}
	if _, err := p.validateOwnedRef(ctx, ref); err != nil {
		return core.ClientConnection{}, err
	}
	id := fmt.Sprintf("ssh-%d", req.HostPort)
	marker := "haco:" + id
	if _, err := p.exec(ctx, ref, []string{"sh", "-ceu", managedKubeSSHProvisionScript, "haco-ssh", req.PublicKey, marker}); err != nil {
		return core.ClientConnection{}, fmt.Errorf("prepare SSH in Kubernetes Environment %s: %w", ref, err)
	}
	connection, err := p.createClientForward(ctx, ref, id, "ssh", req.HostPort, 22)
	if err == nil {
		return connection, nil
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	_, revokeErr := p.exec(cleanupCtx, ref, []string{"sh", "-ceu", managedKubeSSHRevokeScript, "haco-ssh-revoke", marker})
	if revokeErr != nil {
		return core.ClientConnection{}, errors.Join(err, fmt.Errorf("rollback SSH key %s in %s: %w", id, ref, revokeErr), core.ErrRecoveryRequired)
	}
	return core.ClientConnection{}, err
}

func (p *Provider) RevokeSSHAccess(ctx context.Context, ref, connectionID string) error {
	if _, err := p.validateOwnedRef(ctx, ref); err != nil {
		return err
	}
	if !strings.HasPrefix(connectionID, "ssh-") || validateForwardID(connectionID) != nil {
		return fmt.Errorf("SSH connection id %q: %w", connectionID, core.ErrInvalidArgument)
	}
	marker := "haco:" + connectionID
	if _, err := p.exec(ctx, ref, []string{"sh", "-ceu", managedKubeSSHRevokeScript, "haco-ssh-revoke", marker}); err != nil {
		return fmt.Errorf("revoke SSH key %s in Kubernetes Environment %s: %w", connectionID, ref, err)
	}
	if err := p.stopClientForward(ctx, ref, connectionID); err != nil {
		return fmt.Errorf("remove Kubernetes SSH forward %s: %w", connectionID, err)
	}
	return nil
}

func (p *Provider) createClientForward(ctx context.Context, ref, id, kind string, hostPort, targetPort int) (core.ClientConnection, error) {
	if p == nil || p.kubectl == "" {
		return core.ClientConnection{}, core.ErrRuntimeUnavailable
	}
	if err := validateForwardID(id); err != nil {
		return core.ClientConnection{}, err
	}
	dir, err := kubeForwardStateDir()
	if err != nil {
		return core.ClientConnection{}, err
	}
	if err := ensurePrivateStateDir(dir); err != nil {
		return core.ClientConnection{}, err
	}
	token, err := randomForwardToken()
	if err != nil {
		return core.ClientConnection{}, err
	}
	record := forwardRecord{
		Version:        forwardStateVersion,
		EnvironmentRef: ref,
		ID:             id,
		Kind:           kind,
		HostPort:       hostPort,
		TargetPort:     targetPort,
		Token:          token,
		State:          "starting",
	}
	statePath := forwardRecordPath(dir, ref, id)
	if err := createForwardRecord(statePath, record); err != nil {
		if errors.Is(err, os.ErrExist) {
			return core.ClientConnection{}, fmt.Errorf("client connection %q: %w", id, core.ErrAlreadyExists)
		}
		return core.ClientConnection{}, err
	}
	cleanupRecord := true
	defer func() {
		if cleanupRecord {
			_ = removeForwardFiles(dir, record)
		}
	}()

	logPath := forwardLogPath(dir, ref, id)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return core.ClientConnection{}, err
	}
	args := []string{
		"-n", ref,
		"port-forward",
		"--address=127.0.0.1",
		"pod/" + podName,
		fmt.Sprintf("%d:%d", hostPort, targetPort),
	}
	cmd := exec.Command(p.kubectl, args...)
	cmd.Env = cleanPortForwardEnvironment(token)
	cmd.Stdin = nil
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := ctx.Err(); err != nil {
		logFile.Close()
		return core.ClientConnection{}, err
	}
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return core.ClientConnection{}, fmt.Errorf("start Kubernetes port-forward %s: %w", id, err)
	}
	_ = logFile.Close()
	go func() { _ = cmd.Wait() }()

	record.PID = cmd.Process.Pid
	startTicks, err := procStartTicks(record.PID)
	if err != nil {
		_ = terminateForwardProcess(record, forwardStopTimeout)
		return core.ClientConnection{}, fmt.Errorf("record Kubernetes port-forward process %s: %w", id, err)
	}
	record.ProcStartTicks = startTicks
	record.State = "active"
	if err := replaceForwardRecord(statePath, record); err != nil {
		_ = terminateForwardProcess(record, forwardStopTimeout)
		return core.ClientConnection{}, fmt.Errorf("persist Kubernetes port-forward %s: %w", id, err)
	}

	readyCtx, cancel := context.WithTimeout(ctx, forwardReadyTimeout)
	defer cancel()
	if err := waitForLoopbackForward(readyCtx, record); err != nil {
		_ = terminateForwardProcess(record, forwardStopTimeout)
		return core.ClientConnection{}, fmt.Errorf("Kubernetes port-forward %s did not become ready: %w", id, err)
	}
	cleanupRecord = false
	return connectionFromForward(record), nil
}

func (p *Provider) stopClientForward(ctx context.Context, ref, id string) error {
	dir, err := kubeForwardStateDir()
	if err != nil {
		return err
	}
	if err := ensurePrivateStateDir(dir); err != nil {
		return err
	}
	path := forwardRecordPath(dir, ref, id)
	record, err := readForwardRecord(path)
	if errors.Is(err, os.ErrNotExist) {
		return core.ErrNotFound
	}
	if err != nil {
		return err
	}
	if record.EnvironmentRef != ref || record.ID != id {
		return fmt.Errorf("client connection %q ownership mismatch: %w", id, core.ErrIncompatibleState)
	}
	if err := terminateForwardProcessContext(ctx, record, forwardStopTimeout); err != nil {
		return errors.Join(err, core.ErrRecoveryRequired)
	}
	return removeForwardFiles(dir, record)
}

func connectionFromForward(record forwardRecord) core.ClientConnection {
	connection := core.ClientConnection{
		ID:         record.ID,
		Kind:       record.Kind,
		Host:       "127.0.0.1",
		Port:       record.HostPort,
		TargetPort: record.TargetPort,
	}
	if record.Kind == "ssh" {
		connection.User = "root"
		connection.Command = fmt.Sprintf("ssh -p %d root@127.0.0.1", record.HostPort)
	}
	return connection
}

func kubeForwardStateDir() (string, error) {
	root := os.Getenv("HACO_ROOT")
	if root == "" {
		root = "/var/lib/hacocoon"
	}
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || strings.ContainsAny(root, "\r\n\x00") {
		return "", fmt.Errorf("invalid HACO_ROOT for Kubernetes forward state: %w", core.ErrInvalidArgument)
	}
	return filepath.Join(root, "state", "kubernetes-forwards"), nil
}

func ensurePrivateStateDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("Kubernetes forward state path %q is not a directory: %w", dir, core.ErrIncompatibleState)
	}
	if info.Mode().Perm()&0o077 != 0 {
		if err := os.Chmod(dir, 0o700); err != nil {
			return err
		}
	}
	return nil
}

func validateForwardID(id string) error {
	if id == "" || len(id) > 64 || strings.ContainsAny(id, "/\\\r\n\x00") {
		return core.ErrInvalidArgument
	}
	for _, r := range id {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return core.ErrInvalidArgument
		}
	}
	return nil
}

func forwardRecordPath(dir, ref, id string) string {
	return filepath.Join(dir, ref+"--"+id+".json")
}

func forwardLogPath(dir, ref, id string) string {
	return filepath.Join(dir, ref+"--"+id+".log")
}

func createForwardRecord(path string, record forwardRecord) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		_ = os.Remove(path)
		return err
	}
	return file.Close()
}

func replaceForwardRecord(path string, record forwardRecord) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	file, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		file.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func readForwardRecord(path string) (forwardRecord, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return forwardRecord{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return forwardRecord{}, fmt.Errorf("Kubernetes forward state %q is not a regular file: %w", path, core.ErrIncompatibleState)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return forwardRecord{}, fmt.Errorf("Kubernetes forward state %q permissions are too broad: %w", path, core.ErrIncompatibleState)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return forwardRecord{}, err
	}
	var record forwardRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return forwardRecord{}, fmt.Errorf("decode Kubernetes forward state %q: %w", path, core.ErrIncompatibleState)
	}
	if record.Version != forwardStateVersion || record.EnvironmentRef == "" || validateForwardID(record.ID) != nil || record.Token == "" || record.HostPort < 1 || record.HostPort > 65535 || record.TargetPort < 1 || record.TargetPort > 65535 || (record.Kind != "tcp" && record.Kind != "ssh") {
		return forwardRecord{}, fmt.Errorf("invalid Kubernetes forward state %q: %w", path, core.ErrIncompatibleState)
	}
	return record, nil
}

func removeForwardFiles(dir string, record forwardRecord) error {
	var errs []error
	for _, path := range []string{
		forwardRecordPath(dir, record.EnvironmentRef, record.ID),
		forwardLogPath(dir, record.EnvironmentRef, record.ID),
	} {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func randomForwardToken() (string, error) {
	var bytes [24]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}

func cleanPortForwardEnvironment(token string) []string {
	environment := []string{forwardTokenEnv + "=" + token}
	for _, name := range []string{"HOME", "KUBECONFIG", "PATH", "LANG", "LC_ALL"} {
		if value, ok := os.LookupEnv(name); ok {
			environment = append(environment, name+"="+value)
		}
	}
	if _, ok := os.LookupEnv("PATH"); !ok {
		environment = append(environment, "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")
	}
	return environment
}

func waitForLoopbackForward(ctx context.Context, record forwardRecord) error {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(record.HostPort))
	for {
		alive, err := exactForwardProcessAlive(record)
		if err != nil {
			return err
		}
		if !alive {
			return errors.New("port-forward process exited")
		}
		conn, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func procStartTicks(pid int) (uint64, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, err
	}
	text := string(data)
	cut := strings.LastIndex(text, ")")
	if cut < 0 || cut+2 >= len(text) {
		return 0, core.ErrIncompatibleState
	}
	fields := strings.Fields(text[cut+2:])
	if len(fields) <= 19 {
		return 0, core.ErrIncompatibleState
	}
	if fields[0] == "Z" {
		return 0, os.ErrProcessDone
	}
	return strconv.ParseUint(fields[19], 10, 64)
}

func exactForwardProcessAlive(record forwardRecord) (bool, error) {
	if record.PID <= 0 || record.ProcStartTicks == 0 {
		pids, err := findForwardProcessesByToken(record.Token)
		if err != nil {
			return false, err
		}
		if len(pids) == 0 {
			return false, nil
		}
		if len(pids) != 1 {
			return false, fmt.Errorf("multiple processes claim Kubernetes forward token for %s: %w", record.ID, core.ErrIncompatibleState)
		}
		return true, nil
	}
	startTicks, err := procStartTicks(record.PID)
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrProcessDone) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if startTicks != record.ProcStartTicks {
		return false, fmt.Errorf("PID %d was reused for Kubernetes forward %s: %w", record.PID, record.ID, core.ErrIncompatibleState)
	}
	match, err := processHasForwardToken(record.PID, record.Token)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if !match {
		return false, fmt.Errorf("Kubernetes forward process %d identity mismatch for %s: %w", record.PID, record.ID, core.ErrIncompatibleState)
	}
	return true, nil
}

func processHasForwardToken(pid int, token string) (bool, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/environ", pid))
	if err != nil {
		return false, err
	}
	needle := []byte(forwardTokenEnv + "=" + token)
	for _, item := range strings.Split(string(data), "\x00") {
		if item == string(needle) {
			return true, nil
		}
	}
	return false, nil
}

func findForwardProcessesByToken(token string) ([]int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	matches := make([]int, 0, 1)
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		match, err := processHasForwardToken(pid, token)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM) {
				continue
			}
			continue
		}
		if match {
			matches = append(matches, pid)
		}
	}
	sort.Ints(matches)
	return matches, nil
}

func terminateForwardProcess(record forwardRecord, timeout time.Duration) error {
	return terminateForwardProcessContext(context.Background(), record, timeout)
}

func terminateForwardProcessContext(ctx context.Context, record forwardRecord, timeout time.Duration) error {
	pid := record.PID
	if pid <= 0 {
		pids, err := findForwardProcessesByToken(record.Token)
		if err != nil {
			return err
		}
		if len(pids) == 0 {
			return nil
		}
		if len(pids) != 1 {
			return fmt.Errorf("ambiguous Kubernetes forward process for %s: %w", record.ID, core.ErrIncompatibleState)
		}
		pid = pids[0]
	} else {
		alive, err := exactForwardProcessAlive(record)
		if err != nil {
			return err
		}
		if !alive {
			return nil
		}
	}
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(fmt.Sprintf("/proc/%d", pid)); errors.Is(err, os.ErrNotExist) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
				return err
			}
			return nil
		case <-ticker.C:
		}
	}
}
