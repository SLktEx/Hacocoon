package main

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	"reflect"
	"testing"
)

type switchClient struct {
	steps   []string
	fail    string
	request controlapi.EnvironmentCreateRequest
}

func (c *switchClient) step(s string) error {
	c.steps = append(c.steps, s)
	if c.fail == s {
		return core.ErrRecoveryRequired
	}
	return nil
}
func (c *switchClient) EnvironmentStatus(context.Context, string) (core.EnvironmentStatus, error) {
	return core.EnvironmentStatus{Environment: core.Environment{Name: "dev", Workspace: core.Workspace{Path: "managed:both"}, AccessMode: core.WorkspaceReadOnly}}, c.step("status")
}
func (c *switchClient) InspectBase(context.Context, core.BaseName) (core.BaseInfo, error) {
	return core.BaseInfo{}, c.step("inspect")
}
func (c *switchClient) StopEnvironment(context.Context, string) error   { return c.step("stop") }
func (c *switchClient) DeleteEnvironment(context.Context, string) error { return c.step("delete") }
func (c *switchClient) CreateEnvironment(_ context.Context, r controlapi.EnvironmentCreateRequest) (core.Environment, error) {
	c.request = r
	return core.Environment{}, c.step("create")
}
func TestBaseSwitchUsesCanonicalLifecycleAndStopsAtFailure(t *testing.T) {
	steps := []string{"status", "inspect", "stop", "delete", "create"}
	for i, fail := range append(append([]string{}, steps...), "") {
		c := &switchClient{fail: fail}
		_, err := switchBase(context.Background(), c, "dev", "haco/ubuntu-24.04")
		want := steps
		if fail != "" {
			want = steps[:i+1]
			if !errors.Is(err, core.ErrRecoveryRequired) {
				t.Fatal(err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(c.steps, want) {
			t.Fatalf("steps=%v want=%v", c.steps, want)
		}
		if fail == "" && (c.request.WorkspacePath != "managed:both" || c.request.AccessMode != core.WorkspaceReadOnly || c.request.Base != "haco/ubuntu-24.04") {
			t.Fatalf("lost workspace or access: %+v", c.request)
		}
	}
}
