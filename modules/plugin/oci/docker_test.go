package oci

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type dockerExecResponse struct {
	result core.ExecutionResult
	err    error
}

type dockerQueueExecutor struct {
	responses []dockerExecResponse
	calls     []dockerCall
}

type dockerCall struct {
	ref  string
	argv []string
}

func (f *dockerQueueExecutor) ExecEnvironment(_ context.Context, ref string, req core.ExecutionRequest) (core.ExecutionResult, error) {
	f.calls = append(f.calls, dockerCall{ref: ref, argv: append([]string(nil), req.Argv...)})
	if len(f.responses) == 0 {
		return core.ExecutionResult{ExitCode: 1, Stderr: "unexpected call"}, errors.New("unexpected call")
	}
	response := f.responses[0]
	f.responses = f.responses[1:]
	return response.result, response.err
}

func TestDockerStatusTreatsInactiveEngineAsReady(t *testing.T) {
	dir := t.TempDir()
	environmentPath := dir + "/environments.json"
	writeEnvironmentState(t, environmentPath, map[string]core.Environment{
		"dev": {Name: "dev", RuntimeRef: "ref-dev"},
	})
	runtime := &dockerQueueExecutor{responses: []dockerExecResponse{{result: core.ExecutionResult{Stdout: dockerProbeFixture(true, false, false, false)}}}}
	service, err := New(runtime, environmentPath, NewStore(dir+"/oci.json"), DriverDocker)
	if err != nil {
		t.Fatal(err)
	}

	status, err := service.DockerStatus(context.Background(), "dev")
	if err != nil {
		t.Fatal(err)
	}
	if !status.Ready || status.EngineActive {
		t.Fatalf("status=%#v", status)
	}
	if status.ContainerdActive {
		t.Fatalf("containerd need not already be active before socket activation: %#v", status)
	}
	want := []string{"/bin/sh", "-c", dockerProbeScript}
	if len(runtime.calls) != 1 || runtime.calls[0].ref != "ref-dev" || !reflect.DeepEqual(runtime.calls[0].argv, want) {
		t.Fatalf("calls=%#v", runtime.calls)
	}
}

func TestPrepareDockerEnablesOnlyHacocoonSocket(t *testing.T) {
	dir := t.TempDir()
	environmentPath := dir + "/environments.json"
	writeEnvironmentState(t, environmentPath, map[string]core.Environment{
		"dev": {Name: "dev", RuntimeRef: "ref-dev"},
	})
	runtime := &dockerQueueExecutor{responses: []dockerExecResponse{
		{result: core.ExecutionResult{Stdout: dockerProbeFixture(false, false, true, false)}},
		{result: core.ExecutionResult{}},
		{result: core.ExecutionResult{Stdout: dockerProbeFixture(true, false, false, false)}},
	}}
	service, err := New(runtime, environmentPath, NewStore(dir+"/oci.json"), DriverDocker)
	if err != nil {
		t.Fatal(err)
	}

	status, err := service.PrepareDocker(context.Background(), "dev")
	if err != nil {
		t.Fatal(err)
	}
	if !status.Ready {
		t.Fatalf("status=%#v", status)
	}
	if len(runtime.calls) != 3 {
		t.Fatalf("calls=%#v", runtime.calls)
	}
	if runtime.calls[1].ref != "ref-dev" || !reflect.DeepEqual(runtime.calls[1].argv, []string{"/bin/sh", "-c", dockerPrepareScript}) {
		t.Fatalf("prepare call=%#v", runtime.calls[1])
	}
}

func TestPrepareDockerRefusesToStopActiveVendorDaemon(t *testing.T) {
	dir := t.TempDir()
	environmentPath := dir + "/environments.json"
	writeEnvironmentState(t, environmentPath, map[string]core.Environment{
		"dev": {Name: "dev", RuntimeRef: "ref-dev"},
	})
	runtime := &dockerQueueExecutor{responses: []dockerExecResponse{{result: core.ExecutionResult{Stdout: dockerProbeFixture(false, false, true, true)}}}}
	service, err := New(runtime, environmentPath, NewStore(dir+"/oci.json"), DriverDocker)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.PrepareDocker(context.Background(), "dev")
	if !errors.Is(err, core.ErrRuntimeUnavailable) {
		t.Fatalf("err=%v", err)
	}
	if len(runtime.calls) != 1 {
		t.Fatalf("active vendor daemon must fail before mutation, calls=%#v", runtime.calls)
	}
}

func TestPrepareDockerFailsClosedOnUnitDrift(t *testing.T) {
	dir := t.TempDir()
	environmentPath := dir + "/environments.json"
	writeEnvironmentState(t, environmentPath, map[string]core.Environment{
		"dev": {Name: "dev", RuntimeRef: "ref-dev"},
	})
	output := dockerProbeFixture(false, false, false, false)
	output = replaceDockerProbeValue(output, "service_unit_sha256", "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")
	runtime := &dockerQueueExecutor{responses: []dockerExecResponse{{result: core.ExecutionResult{Stdout: output}}}}
	service, err := New(runtime, environmentPath, NewStore(dir+"/oci.json"), DriverDocker)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.PrepareDocker(context.Background(), "dev")
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("err=%v", err)
	}
	if len(runtime.calls) != 1 {
		t.Fatalf("unit drift must fail before mutation, calls=%#v", runtime.calls)
	}
}

func TestDockerCompatibilityRequiresDockerDriver(t *testing.T) {
	dir := t.TempDir()
	environmentPath := dir + "/environments.json"
	writeEnvironmentState(t, environmentPath, map[string]core.Environment{
		"dev": {Name: "dev", RuntimeRef: "ref-dev"},
	})
	runtime := &dockerQueueExecutor{}
	service, err := New(runtime, environmentPath, NewStore(dir+"/oci.json"), DriverNerdctl)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.DockerStatus(context.Background(), "dev"); !errors.Is(err, core.ErrRuntimeUnavailable) {
		t.Fatalf("err=%v", err)
	}
	if len(runtime.calls) != 0 {
		t.Fatalf("driver rejection must happen before Environment exec: %#v", runtime.calls)
	}
}

func TestParseDockerProbeFailsClosedOnMissingField(t *testing.T) {
	output := replaceDockerProbeValue(dockerProbeFixture(true, false, false, false), "docker_cli", "")
	_, err := parseDockerProbe(output)
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("err=%v", err)
	}
}

func dockerProbeFixture(socketReady, engineActive, vendorEnabled, vendorActive bool) string {
	boolValue := func(value bool) string {
		if value {
			return "1"
		}
		return "0"
	}
	return "docker_cli\t1\n" +
		"dockerd\t1\n" +
		"containerd\t1\n" +
		"systemctl\t1\n" +
		"docker_group\t1\n" +
		"socket_unit_sha256\t" + dockerSocketUnitDigest() + "\n" +
		"service_unit_sha256\t" + dockerServiceUnitDigest() + "\n" +
		"socket_enabled\t" + boolValue(socketReady) + "\n" +
		"socket_active\t" + boolValue(socketReady) + "\n" +
		"engine_active\t" + boolValue(engineActive) + "\n" +
		"containerd_active\t0\n" +
		"vendor_docker_enabled\t" + boolValue(vendorEnabled) + "\n" +
		"vendor_docker_active\t" + boolValue(vendorActive) + "\n"
}

func replaceDockerProbeValue(output, key, value string) string {
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, key+"\t") {
			if value == "" {
				lines = append(lines[:i], lines[i+1:]...)
			} else {
				lines[i] = key + "\t" + value
			}
			break
		}
	}
	return strings.Join(lines, "\n")
}
