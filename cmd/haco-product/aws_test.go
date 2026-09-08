package main

import (
	"bytes"
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	awsplugin "github.com/SLktEx/Hacocoon/modules/capability/aws"
	"testing"
)

type fakeAWSClient struct {
	envs  []core.Environment
	spec  awsplugin.ListSpec
	err   error
	calls int
}

func (f *fakeAWSClient) ListEnvironments(context.Context) ([]core.Environment, error) {
	return f.envs, nil
}
func (f *fakeAWSClient) ListS3(_ context.Context, s awsplugin.ListSpec) (core.CapabilityResult, error) {
	f.spec = s
	f.calls++
	return core.CapabilityResult{Output: "[]", ExecutionState: core.CapabilitySucceeded, AuditComplete: true}, f.err
}
func TestAWSUsesOneEnvironmentAndReportsFailure(t *testing.T) {
	f := &fakeAWSClient{envs: []core.Environment{{Name: "dev"}}}
	var out, diag bytes.Buffer
	if code := awsCommand(context.Background(), f, []string{"s3", "ls", "s3://example-bucket/a/"}, &out, &diag); code != 0 || f.spec.Environment != "dev" || f.spec.Profile != "default" || out.String() != "[]\n" {
		t.Fatal(code, f.spec, out.String())
	}
	f.envs = append(f.envs, core.Environment{Name: "other"})
	if code := awsCommand(context.Background(), f, []string{"s3", "ls", "s3://example-bucket"}, &out, &diag); code != 2 || f.calls != 1 {
		t.Fatal("ambiguous environment executed")
	}
	f.err = errors.New("AWS rejected")
	if code := awsCommand(context.Background(), f, []string{"s3", "ls", "--env", "dev", "s3://example-bucket"}, &out, &diag); code != 1 {
		t.Fatal("failure hidden")
	}
}

type guestAWSFixture struct{ fakeAWSClient }

func (*guestAWSFixture) GuestSource() bool { return true }
func TestGuestAWSDoesNotRequireOrSendEnvironmentSelection(t *testing.T) {
	f := &guestAWSFixture{}
	var out, diag bytes.Buffer
	if code := awsCommand(context.Background(), f, []string{"s3", "ls", "s3://example-bucket/a/"}, &out, &diag); code != 0 || f.spec.Environment != "" || f.calls != 1 {
		t.Fatal(code, f.spec, diag.String())
	}
	if code := awsCommand(context.Background(), f, []string{"s3", "ls", "--env", "other", "s3://example-bucket/a/"}, &out, &diag); code != 2 || f.calls != 1 {
		t.Fatal("guest selected another environment", code)
	}
}
