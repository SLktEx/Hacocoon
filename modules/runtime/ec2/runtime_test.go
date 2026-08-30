package ec2

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

const testAWSAccountID = "123456789012"

type fakeRunner struct {
	calls             []string
	failContains      string
	invocation        string
	instanceState     string
	accountID         string
	lastSSMParameters string
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (host.Result, error) {
	call := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, call)
	if f.failContains != "" && strings.Contains(call, f.failContains) {
		return host.Result{ExitCode: 1}, errors.New("forced failure")
	}
	if name == "tar" {
		if len(args) >= 2 && args[0] == "-czf" {
			if err := os.WriteFile(args[1], []byte("archive"), 0o600); err != nil {
				return host.Result{}, err
			}
		}
		if len(args) >= 4 && args[0] == "-xzf" && args[2] == "-C" {
			if err := os.WriteFile(filepath.Join(args[3], "remote.txt"), []byte("from-ec2\n"), 0o644); err != nil {
				return host.Result{}, err
			}
		}
		return host.Result{}, nil
	}
	switch {
	case strings.Contains(call, " sts get-caller-identity "):
		accountID := f.accountID
		if accountID == "" {
			accountID = testAWSAccountID
		}
		return host.Result{Stdout: accountID + "\n"}, nil
	case strings.Contains(call, " ec2 run-instances "):
		return host.Result{Stdout: "i-0123456789abcdef0\n"}, nil
	case strings.Contains(call, " ssm describe-instance-information "):
		return host.Result{Stdout: "Online\n"}, nil
	case strings.Contains(call, " ssm send-command "):
		for i, arg := range args {
			if arg == "--parameters" && i+1 < len(args) {
				f.lastSSMParameters = args[i+1]
			}
		}
		return host.Result{Stdout: "11111111-1111-1111-1111-111111111111\n"}, nil
	case strings.Contains(call, " ssm get-command-invocation "):
		payload := f.invocation
		if payload == "" {
			payload = `{"Status":"Success","ResponseCode":0,"StandardOutputContent":"","StandardErrorContent":""}`
		}
		return host.Result{Stdout: payload}, nil
	case strings.Contains(call, " ec2 describe-instances "):
		state := f.instanceState
		if state == "" {
			state = "running"
		}
		return host.Result{Stdout: state + "\n"}, nil
	case strings.Contains(call, " s3 cp s3://"):
		for i, arg := range args {
			if strings.HasPrefix(arg, "s3://") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "s3://") {
				_ = os.WriteFile(args[i+1], []byte("remote-archive"), 0o600)
			}
		}
		return host.Result{}, nil
	default:
		return host.Result{}, nil
	}
}

func testConfig() Config {
	return Config{Region: "ap-northeast-1", ImageID: "ami-0123456789abcdef0", InstanceType: "t3.large", SubnetID: "subnet-0123456789abcdef0", SecurityGroupIDs: []string{"sg-0123456789abcdef0"}, InstanceProfile: "hacocoon-remote", WorkspaceBucket: "hacocoon-workspaces-example", WorkspacePrefix: "tests"}
}

func testRuntimeRef(instanceID, workspace, bucket, prefix string, readOnly bool) runtimeRef {
	ref := runtimeRef{AccountID: testAWSAccountID, Region: "ap-northeast-1", InstanceID: instanceID, WorkspacePath: workspace, Bucket: bucket, Prefix: prefix, ReadOnly: readOnly}
	if !readOnly {
		if digest, err := digestWorkspace(context.Background(), workspace); err == nil {
			ref.BaseDigest = digest
		}
	}
	return ref
}

func newTestRuntime(runner host.Runner) *Runtime {
	runtime := New(runner, testConfig())
	runtime.pollAttempts = 1
	runtime.pollDelay = 0
	return runtime
}

func TestCreateStagesWorkspaceAndCreatesSSMManagedInstance(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "host.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}
	runtime := newTestRuntime(runner)
	created, err := runtime.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: workspace})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(created.Ref, "ec2v2.") {
		t.Fatalf("created=%#v", created)
	}
	ref, err := decodeRef(created.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if ref.AccountID != testAWSAccountID || ref.Region != "ap-northeast-1" || ref.InstanceID != "i-0123456789abcdef0" || ref.WorkspacePath != workspace || ref.ReadOnly || !validWorkspaceDigest(ref.BaseDigest) {
		t.Fatalf("ref=%#v", ref)
	}
	joined := strings.Join(runner.calls, "\n")
	for _, want := range []string{"sts get-caller-identity --query Account --output text", "tar -czf", "s3 cp", "ec2 run-instances", "--metadata-options HttpTokens=required,HttpEndpoint=enabled", "ec2 wait instance-status-ok", "ssm describe-instance-information", "ssm send-command"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in calls:\n%s", want, joined)
		}
	}
	if !strings.Contains(runner.lastSSMParameters, "mount --bind") || strings.Contains(runner.lastSSMParameters, "remount,bind,ro") {
		t.Fatalf("bootstrap params=%s", runner.lastSSMParameters)
	}
	if strings.Contains(joined, "AWS_SECRET_ACCESS_KEY") || strings.Contains(joined, "AWS_SESSION_TOKEN") {
		t.Fatalf("credential material appeared in argv:\n%s", joined)
	}
}

func TestCreateRefusesUnprovableAWSAccountBeforeSideEffects(t *testing.T) {
	runner := &fakeRunner{failContains: "sts get-caller-identity"}
	runtime := newTestRuntime(runner)
	_, err := runtime.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: t.TempDir()})
	if err == nil {
		t.Fatal("expected failure")
	}
	joined := strings.Join(runner.calls, "\n")
	if strings.Contains(joined, "tar -czf") || strings.Contains(joined, "s3 cp") || strings.Contains(joined, "ec2 run-instances") {
		t.Fatalf("AWS/account identity failure happened after side effects:\n%s", joined)
	}
}

func TestCreateReadOnlyUsesReadOnlyBindMount(t *testing.T) {
	runner := &fakeRunner{}
	runtime := newTestRuntime(runner)
	created, err := runtime.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: t.TempDir(), ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	ref, _ := decodeRef(created.Ref)
	if !ref.ReadOnly || !strings.Contains(runner.lastSSMParameters, "remount,bind,ro") {
		t.Fatalf("ref=%#v params=%s", ref, runner.lastSSMParameters)
	}
}

func TestCreateFailureCleansInstanceAndStaging(t *testing.T) {
	runner := &fakeRunner{failContains: "ssm describe-instance-information"}
	runtime := newTestRuntime(runner)
	_, err := runtime.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: t.TempDir()})
	if err == nil {
		t.Fatal("expected failure")
	}
	joined := strings.Join(runner.calls, "\n")
	if !strings.Contains(joined, "ec2 terminate-instances --instance-ids i-0123456789abcdef0") || !strings.Contains(joined, "s3 rm s3://hacocoon-workspaces-example/tests/demo --recursive") {
		t.Fatalf("partial cleanup missing:\n%s", joined)
	}
}

func TestExecUsesSSMAndPreservesArgvMeaning(t *testing.T) {
	runner := &fakeRunner{invocation: `{"Status":"Failed","ResponseCode":17,"StandardOutputContent":"out\n","StandardErrorContent":"err\n"}`}
	runtime := newTestRuntime(runner)
	ref, _ := encodeRef(testRuntimeRef("i-0123456789abcdef0", "/tmp/work", "bucket-example", "p", true))
	result, err := runtime.ExecEnvironment(context.Background(), ref, core.ExecutionRequest{Argv: []string{"printf", "%s", "hello world; $(touch nope)", "it's-safe"}})
	if err != nil || result.ExitCode != 17 || result.Stdout != "out\n" || result.Stderr != "err\n" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	for _, want := range []string{`'hello world; $(touch nope)'`, `'it'\\''s-safe'`} {
		if !strings.Contains(runner.lastSSMParameters, want) {
			t.Fatalf("missing shell quote %q in %s", want, runner.lastSSMParameters)
		}
	}
}

func TestLifecycleRejectsRegionChangeBeforeAWSCalls(t *testing.T) {
	runner := &fakeRunner{}
	cfg := testConfig()
	cfg.Region = "us-west-2"
	runtime := New(runner, cfg)
	raw, _ := encodeRef(testRuntimeRef("i-0123456789abcdef0", "/tmp/work", "bucket-example", "p", true))
	_, err := runtime.InspectEnvironment(context.Background(), raw)
	if !errors.Is(err, core.ErrCapabilityStale) || !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("err=%v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("region mismatch reached AWS authority: %v", runner.calls)
	}
}

func TestLifecycleRejectsAccountChangeBeforeResourceCalls(t *testing.T) {
	runner := &fakeRunner{accountID: "210987654321"}
	runtime := newTestRuntime(runner)
	raw, _ := encodeRef(testRuntimeRef("i-0123456789abcdef0", "/tmp/work", "bucket-example", "p", true))
	_, err := runtime.InspectEnvironment(context.Background(), raw)
	if !errors.Is(err, core.ErrCapabilityStale) || !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("err=%v", err)
	}
	joined := strings.Join(runner.calls, "\n")
	if !strings.Contains(joined, "sts get-caller-identity") || strings.Contains(joined, "ec2 describe-instances") {
		t.Fatalf("account mismatch reached resource authority:\n%s", joined)
	}
}

func TestDeleteReadWriteSynchronizesBeforeTerminate(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "old.txt"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}
	runtime := newTestRuntime(runner)
	raw, _ := encodeRef(testRuntimeRef("i-0123456789abcdef0", workspace, "hacocoon-workspaces-example", "tests/demo", false))
	if err := runtime.DeleteEnvironment(context.Background(), raw); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "old.txt")); !os.IsNotExist(err) {
		t.Fatalf("old workspace file survived atomic restore: %v", err)
	}
	if content, err := os.ReadFile(filepath.Join(workspace, "remote.txt")); err != nil || string(content) != "from-ec2\n" {
		t.Fatalf("content=%q err=%v", content, err)
	}
	joined := strings.Join(runner.calls, "\n")
	syncAt := strings.Index(joined, "s3 cp /tmp/haco-output.tgz")
	terminateAt := strings.Index(joined, "ec2 terminate-instances")
	if syncAt < 0 || terminateAt < 0 || syncAt > terminateAt {
		t.Fatalf("sync must precede terminate:\n%s", joined)
	}
}

func TestDeletePreservesUnrelatedLegacyBackupPath(t *testing.T) {
	parent := t.TempDir()
	workspace := filepath.Join(parent, "workspace")
	if err := os.Mkdir(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	legacyBackup := workspace + ".haco-backup"
	if err := os.Mkdir(legacyBackup, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyBackup, "keep.txt"), []byte("keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	runner := &fakeRunner{}
	runtime := newTestRuntime(runner)
	raw, _ := encodeRef(testRuntimeRef("i-0123456789abcdef0", workspace, "hacocoon-workspaces-example", "tests/demo", false))
	if err := runtime.DeleteEnvironment(context.Background(), raw); err != nil {
		t.Fatal(err)
	}
	if content, err := os.ReadFile(filepath.Join(legacyBackup, "keep.txt")); err != nil || string(content) != "keep\n" {
		t.Fatalf("unrelated backup was modified: content=%q err=%v", content, err)
	}
}

func TestDeleteFailsClosedBeforeTerminateWhenSyncCannotBeProven(t *testing.T) {
	runner := &fakeRunner{instanceState: "stopped"}
	runtime := newTestRuntime(runner)
	raw, _ := encodeRef(testRuntimeRef("i-0123456789abcdef0", t.TempDir(), "hacocoon-workspaces-example", "tests/demo", false))
	err := runtime.DeleteEnvironment(context.Background(), raw)
	if !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("err=%v", err)
	}
	if strings.Contains(strings.Join(runner.calls, "\n"), "terminate-instances") {
		t.Fatalf("instance terminated before workspace recovery: %v", runner.calls)
	}
}

func TestDeleteTerminatedReadWriteRetainsStagingAndRequiresRecovery(t *testing.T) {
	runner := &fakeRunner{instanceState: "terminated"}
	runtime := newTestRuntime(runner)
	raw, _ := encodeRef(testRuntimeRef("i-0123456789abcdef0", t.TempDir(), "hacocoon-workspaces-example", "tests/demo", false))
	err := runtime.DeleteEnvironment(context.Background(), raw)
	if !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("err=%v", err)
	}
	joined := strings.Join(runner.calls, "\n")
	if strings.Contains(joined, "ssm send-command") || strings.Contains(joined, "terminate-instances") || strings.Contains(joined, "s3 rm s3://hacocoon-workspaces-example/tests/demo --recursive") {
		t.Fatalf("terminated RW recovery destroyed evidence:\n%s", joined)
	}
}

func TestDeleteTerminatedReadOnlyCleansStaging(t *testing.T) {
	runner := &fakeRunner{instanceState: "terminated"}
	runtime := newTestRuntime(runner)
	raw, _ := encodeRef(testRuntimeRef("i-0123456789abcdef0", t.TempDir(), "hacocoon-workspaces-example", "tests/demo", true))
	if err := runtime.DeleteEnvironment(context.Background(), raw); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(runner.calls, "\n")
	if strings.Contains(joined, "ssm send-command") || strings.Contains(joined, "terminate-instances") || !strings.Contains(joined, "s3 rm s3://hacocoon-workspaces-example/tests/demo --recursive") {
		t.Fatalf("terminated RO cleanup flow:\n%s", joined)
	}
}

func TestInspectMapsEC2State(t *testing.T) {
	runner := &fakeRunner{instanceState: "running"}
	runtime := newTestRuntime(runner)
	raw, _ := encodeRef(testRuntimeRef("i-0123456789abcdef0", "/tmp/work", "bucket-example", "p", true))
	status, err := runtime.InspectEnvironment(context.Background(), raw)
	if err != nil || status.State != core.EnvironmentRunning {
		t.Fatalf("status=%#v err=%v", status, err)
	}
}

func TestConfigFailsClosedWhenRemoteOwnershipIsIncomplete(t *testing.T) {
	cases := []Config{{}, func() Config { c := testConfig(); c.InstanceProfile = ""; return c }(), func() Config { c := testConfig(); c.WorkspaceBucket = "Bad_Bucket"; return c }()}
	for _, cfg := range cases {
		if _, err := cfg.normalized(); !errors.Is(err, core.ErrRuntimeUnavailable) {
			t.Fatalf("config=%#v err=%v", cfg, err)
		}
	}
}

func TestWaitSSMHonorsContext(t *testing.T) {
	runner := &fakeRunner{failContains: "describe-instance-information"}
	runtime := newTestRuntime(runner)
	runtime.pollAttempts = 2
	runtime.pollDelay = time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runtime.waitSSM(ctx, "i-0123456789abcdef0"); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}
