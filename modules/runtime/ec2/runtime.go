package ec2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

const providerID = "runtime.ec2"

type Runtime struct {
	runner       host.Runner
	config       Config
	stdin        io.Reader
	stdout       io.Writer
	stderr       io.Writer
	pollAttempts int
	pollDelay    time.Duration
}

func New(runner host.Runner, config Config) *Runtime {
	return &Runtime{runner: runner, config: config, stdin: os.Stdin, stdout: os.Stdout, stderr: os.Stderr, pollAttempts: 24, pollDelay: 5 * time.Second}
}

func (*Runtime) ID() string { return providerID }

func (r *Runtime) CreateEnvironment(ctx context.Context, spec core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	if r == nil || r.runner == nil || strings.TrimSpace(spec.Name) == "" || strings.TrimSpace(spec.WorkspacePath) == "" {
		return core.EnvironmentRuntime{}, core.ErrInvalidArgument
	}
	cfg, err := r.config.normalized()
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}

	archive, cleanupArchive, err := createWorkspaceArchive(ctx, r.runner, spec.WorkspacePath)
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}
	defer cleanupArchive()

	prefix := cfg.WorkspacePrefix + "/" + spec.Name
	inputURI := s3URI(cfg.WorkspaceBucket, prefix+"/input.tgz")
	if _, err := r.aws(ctx, "s3", "cp", archive, inputURI, "--only-show-errors"); err != nil {
		return core.EnvironmentRuntime{}, fmt.Errorf("stage workspace: %w", err)
	}
	staged := true
	var instanceID string
	cleanup := func(cause error) (core.EnvironmentRuntime, error) {
		cleanupCtx := context.WithoutCancel(ctx)
		var cleanupErrs []error
		if instanceID != "" {
			if _, err := r.aws(cleanupCtx, "ec2", "terminate-instances", "--instance-ids", instanceID); err != nil {
				cleanupErrs = append(cleanupErrs, fmt.Errorf("terminate partial EC2 instance %s: %w", instanceID, err))
			}
		}
		if staged {
			if _, err := r.aws(cleanupCtx, "s3", "rm", "s3://"+cfg.WorkspaceBucket+"/"+prefix, "--recursive", "--only-show-errors"); err != nil {
				cleanupErrs = append(cleanupErrs, fmt.Errorf("remove staged workspace: %w", err))
			}
		}
		return core.EnvironmentRuntime{}, errors.Join(append([]error{cause}, cleanupErrs...)...)
	}

	args := []string{"ec2", "run-instances", "--image-id", cfg.ImageID, "--instance-type", cfg.InstanceType, "--subnet-id", cfg.SubnetID, "--security-group-ids"}
	args = append(args, cfg.SecurityGroupIDs...)
	args = append(args,
		"--iam-instance-profile", "Name="+cfg.InstanceProfile,
		"--metadata-options", "HttpTokens=required,HttpEndpoint=enabled",
		"--tag-specifications", fmt.Sprintf("ResourceType=instance,Tags=[{Key=Name,Value=hacocoon-%s},{Key=HacocoonEnvironment,Value=%s}]", spec.Name, spec.Name),
		"--query", "Instances[0].InstanceId", "--output", "text",
	)
	result, err := r.aws(ctx, args...)
	if err != nil {
		return cleanup(fmt.Errorf("create EC2 environment: %w", err))
	}
	instanceID = strings.TrimSpace(result.Stdout)
	if !validInstanceID(instanceID) {
		return cleanup(fmt.Errorf("invalid EC2 instance id %q: %w", instanceID, core.ErrIncompatibleState))
	}
	if _, err := r.aws(ctx, "ec2", "wait", "instance-status-ok", "--instance-ids", instanceID); err != nil {
		return cleanup(fmt.Errorf("wait for EC2 health: %w", err))
	}
	if err := r.waitSSM(ctx, instanceID); err != nil {
		return cleanup(err)
	}

	bootstrap := bootstrapCommand(inputURI, spec.ReadOnly)
	if _, err := r.runSSM(ctx, instanceID, bootstrap); err != nil {
		return cleanup(fmt.Errorf("materialize remote workspace: %w", err))
	}

	ref, err := encodeRef(runtimeRef{InstanceID: instanceID, WorkspacePath: spec.WorkspacePath, Bucket: cfg.WorkspaceBucket, Prefix: prefix, ReadOnly: spec.ReadOnly})
	if err != nil {
		return cleanup(err)
	}
	staged = false
	return core.EnvironmentRuntime{Ref: ref}, nil
}

func (r *Runtime) ExecEnvironment(ctx context.Context, rawRef string, req core.ExecutionRequest) (core.ExecutionResult, error) {
	if len(req.Argv) == 0 {
		return core.ExecutionResult{}, core.ErrInvalidArgument
	}
	ref, err := decodeRef(rawRef)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	command := "cd /workspace && exec " + shellJoin(req.Argv)
	return r.runSSM(ctx, ref.InstanceID, command)
}

func (r *Runtime) ShellEnvironment(ctx context.Context, rawRef string) error {
	ref, err := decodeRef(rawRef)
	if err != nil {
		return err
	}
	cfg, err := r.config.normalized()
	if err != nil {
		return err
	}
	args := []string{"--region", cfg.Region, "--no-cli-pager", "ssm", "start-session", "--target", ref.InstanceID, "--document-name", "AWS-StartInteractiveCommand", "--parameters", `command=cd /workspace && exec /bin/bash`}
	cmd := exec.CommandContext(ctx, "aws", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = r.stdin, r.stdout, r.stderr
	return cmd.Run()
}

func (r *Runtime) DeleteEnvironment(ctx context.Context, rawRef string) error {
	ref, err := decodeRef(rawRef)
	if err != nil {
		return err
	}
	state, err := r.instanceState(ctx, ref.InstanceID)
	if err != nil {
		return err
	}
	if state != "terminated" {
		if !ref.ReadOnly {
			if state != "running" {
				return fmt.Errorf("EC2 environment %s state=%s cannot be synchronized safely: %w", ref.InstanceID, state, core.ErrRecoveryRequired)
			}
			if err := r.syncBack(ctx, ref); err != nil {
				return err
			}
		}
		if _, err := r.aws(ctx, "ec2", "terminate-instances", "--instance-ids", ref.InstanceID); err != nil {
			return fmt.Errorf("terminate EC2 environment %s: %w", ref.InstanceID, err)
		}
		if _, err := r.aws(ctx, "ec2", "wait", "instance-terminated", "--instance-ids", ref.InstanceID); err != nil {
			return fmt.Errorf("wait for EC2 termination %s: %w", ref.InstanceID, err)
		}
	}
	if _, err := r.aws(ctx, "s3", "rm", "s3://"+ref.Bucket+"/"+ref.Prefix, "--recursive", "--only-show-errors"); err != nil {
		return fmt.Errorf("cleanup EC2 workspace staging: %w", err)
	}
	return nil
}

func (r *Runtime) InspectEnvironment(ctx context.Context, rawRef string) (core.EnvironmentRuntimeStatus, error) {
	ref, err := decodeRef(rawRef)
	if err != nil {
		return core.EnvironmentRuntimeStatus{}, err
	}
	state, err := r.instanceState(ctx, ref.InstanceID)
	if err != nil {
		return core.EnvironmentRuntimeStatus{}, err
	}
	mapped := map[string]core.EnvironmentState{"running": core.EnvironmentRunning, "stopped": core.EnvironmentStopped}
	observed, ok := mapped[state]
	if !ok {
		observed = core.EnvironmentUnknown
	}
	return core.EnvironmentRuntimeStatus{State: observed}, nil
}

func (r *Runtime) syncBack(ctx context.Context, ref runtimeRef) error {
	outputURI := s3URI(ref.Bucket, ref.Prefix+"/output.tgz")
	command := "set -eu; tar -czf /tmp/haco-output.tgz -C /workspace .; aws s3 cp /tmp/haco-output.tgz " + shellQuote(outputURI) + " --only-show-errors; rm -f /tmp/haco-output.tgz"
	if _, err := r.runSSM(ctx, ref.InstanceID, command); err != nil {
		return fmt.Errorf("stage remote workspace changes: %w", err)
	}
	archive, err := os.CreateTemp(filepath.Dir(ref.WorkspacePath), ".haco-remote-*.tgz")
	if err != nil {
		return fmt.Errorf("create remote workspace download: %w", err)
	}
	archivePath := archive.Name()
	_ = archive.Close()
	defer os.Remove(archivePath)
	if _, err := r.aws(ctx, "s3", "cp", outputURI, archivePath, "--only-show-errors"); err != nil {
		return fmt.Errorf("download remote workspace changes: %w", err)
	}
	if err := restoreWorkspaceArchive(ctx, r.runner, archivePath, ref.WorkspacePath); err != nil {
		return fmt.Errorf("restore remote workspace changes: %w", err)
	}
	return nil
}

func (r *Runtime) waitSSM(ctx context.Context, instanceID string) error {
	attempts := r.pollAttempts
	if attempts <= 0 {
		attempts = 1
	}
	for i := 0; i < attempts; i++ {
		result, err := r.aws(ctx, "ssm", "describe-instance-information", "--filters", "Key=InstanceIds,Values="+instanceID, "--query", "InstanceInformationList[0].PingStatus", "--output", "text")
		if err == nil && strings.TrimSpace(result.Stdout) == "Online" {
			return nil
		}
		if i+1 < attempts && r.pollDelay > 0 {
			timer := time.NewTimer(r.pollDelay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	return fmt.Errorf("EC2 environment %s did not become SSM-managed: %w", instanceID, core.ErrRuntimeUnavailable)
}

type commandInvocation struct {
	Status       string `json:"Status"`
	ResponseCode int    `json:"ResponseCode"`
	Stdout       string `json:"StandardOutputContent"`
	Stderr       string `json:"StandardErrorContent"`
}

func (r *Runtime) runSSM(ctx context.Context, instanceID, command string) (core.ExecutionResult, error) {
	params, err := json.Marshal(map[string][]string{"commands": {command}})
	if err != nil {
		return core.ExecutionResult{}, err
	}
	sent, err := r.aws(ctx, "ssm", "send-command", "--instance-ids", instanceID, "--document-name", "AWS-RunShellScript", "--parameters", string(params), "--query", "Command.CommandId", "--output", "text")
	if err != nil {
		return core.ExecutionResult{}, err
	}
	commandID := strings.TrimSpace(sent.Stdout)
	if commandID == "" || unsafeToken(commandID) {
		return core.ExecutionResult{}, fmt.Errorf("invalid SSM command id: %w", core.ErrIncompatibleState)
	}
	_, _ = r.aws(ctx, "ssm", "wait", "command-executed", "--command-id", commandID, "--instance-id", instanceID)
	got, err := r.aws(ctx, "ssm", "get-command-invocation", "--command-id", commandID, "--instance-id", instanceID, "--output", "json")
	if err != nil {
		return core.ExecutionResult{}, err
	}
	var invocation commandInvocation
	if err := json.Unmarshal([]byte(got.Stdout), &invocation); err != nil {
		return core.ExecutionResult{}, fmt.Errorf("decode SSM invocation: %w", err)
	}
	if invocation.ResponseCode < 0 {
		return core.ExecutionResult{}, fmt.Errorf("SSM command status %s: %w", invocation.Status, core.ErrRuntimeUnavailable)
	}
	return core.ExecutionResult{ExitCode: invocation.ResponseCode, Stdout: invocation.Stdout, Stderr: invocation.Stderr}, nil
}

func (r *Runtime) instanceState(ctx context.Context, instanceID string) (string, error) {
	result, err := r.aws(ctx, "ec2", "describe-instances", "--instance-ids", instanceID, "--query", "Reservations[0].Instances[0].State.Name", "--output", "text")
	if err != nil {
		return "", err
	}
	state := strings.TrimSpace(result.Stdout)
	if state == "" || state == "None" {
		return "terminated", nil
	}
	return state, nil
}

func (r *Runtime) aws(ctx context.Context, args ...string) (host.Result, error) {
	cfg, err := r.config.normalized()
	if err != nil {
		return host.Result{}, err
	}
	all := append([]string{"--region", cfg.Region, "--no-cli-pager"}, args...)
	return r.runner.Run(ctx, "aws", all...)
}

func createWorkspaceArchive(ctx context.Context, runner host.Runner, workspace string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "haco-workspace-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	path := filepath.Join(dir, "workspace.tgz")
	if _, err := runner.Run(ctx, "tar", "-czf", path, "-C", workspace, "."); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("archive workspace: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("secure workspace archive: %w", err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("inspect workspace archive: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		cleanup()
		return "", nil, fmt.Errorf("workspace archive mode %s: %w", info.Mode(), core.ErrIncompatibleState)
	}
	return path, cleanup, nil
}

func restoreWorkspaceArchive(ctx context.Context, runner host.Runner, archive, workspace string) error {
	parent := filepath.Dir(workspace)
	extracted, err := os.MkdirTemp(parent, ".haco-restore-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(extracted)
	if _, err := runner.Run(ctx, "tar", "-xzf", archive, "-C", extracted); err != nil {
		return err
	}

	backupDir, err := os.MkdirTemp(parent, ".haco-backup-*")
	if err != nil {
		return fmt.Errorf("create workspace backup directory: %w", err)
	}
	backup := filepath.Join(backupDir, "workspace")
	if err := os.Rename(workspace, backup); err != nil {
		_ = os.Remove(backupDir)
		return err
	}
	if err := os.Rename(extracted, workspace); err != nil {
		rollbackErr := os.Rename(backup, workspace)
		if rollbackErr == nil {
			_ = os.Remove(backupDir)
			return err
		}
		return errors.Join(
			err,
			fmt.Errorf("restore original workspace from %s: %w", backup, rollbackErr),
			core.ErrRecoveryRequired,
		)
	}
	if err := os.RemoveAll(backupDir); err != nil {
		return fmt.Errorf("remove workspace backup %s: %w", backupDir, err)
	}
	return nil
}

func bootstrapCommand(inputURI string, readOnly bool) string {
	cmd := "set -eu; rm -rf /opt/hacocoon/workspace /workspace; mkdir -p /opt/hacocoon/workspace /workspace; aws s3 cp " + shellQuote(inputURI) + " /tmp/haco-workspace.tgz --only-show-errors; tar -xzf /tmp/haco-workspace.tgz -C /opt/hacocoon/workspace; rm -f /tmp/haco-workspace.tgz; mount --bind /opt/hacocoon/workspace /workspace"
	if readOnly {
		cmd += "; mount -o remount,bind,ro /workspace"
	}
	return cmd
}

func s3URI(bucket, key string) string { return "s3://" + bucket + "/" + key }
func validInstanceID(id string) bool {
	return strings.HasPrefix(id, "i-") && len(id) >= 10 && !unsafeToken(id)
}
func shellJoin(argv []string) string {
	parts := make([]string, len(argv))
	for i, v := range argv {
		parts[i] = shellQuote(v)
	}
	return strings.Join(parts, " ")
}
func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
