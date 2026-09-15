package incus

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

type ownershipRunnerCall struct {
	name string
	args []string
}

type ownershipTestRunner struct {
	owner string
	calls []ownershipRunnerCall
}

func (r *ownershipTestRunner) Run(_ context.Context, name string, args ...string) (host.Result, error) {
	r.calls = append(r.calls, ownershipRunnerCall{name: name, args: append([]string(nil), args...)})
	if len(args) >= 4 && args[0] == "network" && args[1] == "get" && args[3] == environmentNetworkOwnerKey {
		return host.Result{Stdout: r.owner + "\n"}, nil
	}
	return host.Result{}, nil
}

func TestEnvironmentNetworkOwnershipRunnerMarksCreatedBridge(t *testing.T) {
	inner := &ownershipTestRunner{}
	runner := WrapEnvironmentNetworkOwnershipRunner(inner)
	bridge := environmentBridgeName("haco-demo")

	if _, err := runner.Run(context.Background(), "incus", "network", "create", bridge,
		"ipv4.address=auto", "ipv4.nat=false", "--project", sandboxBridgeResourceProject,
	); err != nil {
		t.Fatal(err)
	}
	if len(inner.calls) != 1 {
		t.Fatalf("calls = %#v", inner.calls)
	}
	want := []string{
		"network", "create", bridge,
		"ipv4.address=auto", "ipv4.nat=false",
		environmentNetworkOwnerKey + "=" + environmentNetworkOwnerValue,
		"--project", sandboxBridgeResourceProject,
	}
	if !reflect.DeepEqual(inner.calls[0].args, want) {
		t.Fatalf("create args = %#v, want %#v", inner.calls[0].args, want)
	}
}

func TestEnvironmentNetworkOwnershipRunnerRefusesUnownedAttachment(t *testing.T) {
	inner := &ownershipTestRunner{owner: ""}
	runner := WrapEnvironmentNetworkOwnershipRunner(inner)
	bridge := environmentBridgeName("haco-demo")

	_, err := runner.Run(context.Background(), "incus", "config", "device", "add", "haco-demo", "eth0", "nic",
		"name=eth0", "network="+bridge, "--project", defaultProject,
	)
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("error = %v", err)
	}
	if len(inner.calls) != 1 || len(inner.calls[0].args) < 4 || inner.calls[0].args[0] != "network" || inner.calls[0].args[1] != "get" {
		t.Fatalf("unowned attachment reached Incus mutation: %#v", inner.calls)
	}
}

func TestEnvironmentNetworkOwnershipRunnerAllowsOwnedAttachment(t *testing.T) {
	inner := &ownershipTestRunner{owner: environmentNetworkOwnerValue}
	runner := WrapEnvironmentNetworkOwnershipRunner(inner)
	bridge := environmentBridgeName("haco-demo")
	attach := []string{"config", "device", "add", "haco-demo", "eth0", "nic", "name=eth0", "network=" + bridge, "--project", defaultProject}

	if _, err := runner.Run(context.Background(), "incus", attach...); err != nil {
		t.Fatal(err)
	}
	if len(inner.calls) != 2 {
		t.Fatalf("calls = %#v", inner.calls)
	}
	if !reflect.DeepEqual(inner.calls[1].args, attach) {
		t.Fatalf("forwarded attachment = %#v, want %#v", inner.calls[1].args, attach)
	}
}

func TestEnvironmentNetworkOwnershipRunnerRefusesUnownedDelete(t *testing.T) {
	inner := &ownershipTestRunner{owner: "someone-else"}
	runner := WrapEnvironmentNetworkOwnershipRunner(inner)
	bridge := environmentBridgeName("haco-demo")

	_, err := runner.Run(context.Background(), "incus", "network", "delete", bridge, "--project", sandboxBridgeResourceProject)
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("error = %v", err)
	}
	if len(inner.calls) != 1 || inner.calls[0].args[1] != "get" {
		t.Fatalf("unowned delete reached mutation: %#v", inner.calls)
	}
}

func TestEnvironmentNetworkOwnershipRunnerLeavesUnrelatedIncusNetworkUntouched(t *testing.T) {
	inner := &ownershipTestRunner{}
	runner := WrapEnvironmentNetworkOwnershipRunner(inner)
	args := []string{"network", "delete", "other-network", "--project", "default"}
	if _, err := runner.Run(context.Background(), "/usr/bin/incus", args...); err != nil {
		t.Fatal(err)
	}
	if len(inner.calls) != 1 || !reflect.DeepEqual(inner.calls[0].args, args) {
		t.Fatalf("unrelated command changed: %#v", inner.calls)
	}
}

func TestEnvironmentNetworkOwnershipRunnerRejectsWrongProjectCreate(t *testing.T) {
	inner := &ownershipTestRunner{}
	runner := WrapEnvironmentNetworkOwnershipRunner(inner)
	bridge := environmentBridgeName("haco-demo")
	_, err := runner.Run(context.Background(), "incus", "network", "create", bridge, "ipv4.address=auto", "--project", "other")
	if !errors.Is(err, core.ErrIncompatibleState) || !strings.Contains(err.Error(), "other") {
		t.Fatalf("error = %v", err)
	}
	if len(inner.calls) != 0 {
		t.Fatalf("wrong-project create reached Incus: %#v", inner.calls)
	}
}
