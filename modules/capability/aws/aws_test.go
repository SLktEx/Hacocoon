package aws

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"strings"
	"testing"
)

var testIdentity = identity{Account: "123456789012", Principal: "arn:aws:sts::123456789012:assumed-role/Developer/session", Region: "ap-northeast-1"}

type environments struct{}

func (environments) GetEnvironment(_ context.Context, n string) (core.Environment, error) {
	return core.Environment{Name: n}, nil
}
func (environments) EnvironmentInstance(_ context.Context, e core.Environment) (string, error) {
	if e.Name == "other" {
		return "env-22222222222222222222222222222222", nil
	}
	return "env-11111111111111111111111111111111", nil
}

type requester func(context.Context, core.CapabilityRequest) (core.CapabilityResult, error)

func (f requester) Request(c context.Context, r core.CapabilityRequest) (core.CapabilityResult, error) {
	return f(c, r)
}
func fixtureHost(t *testing.T, callCount *int) Host {
	return func(_ context.Context, script string, input []byte) ([]byte, error) {
		if script != HostAgent {
			t.Fatal("untrusted agent")
		}
		var req agentRequest
		if json.Unmarshal(input, &req) != nil {
			t.Fatal("bad request")
		}
		out := agentResponse{Identity: &testIdentity}
		if req.Mode == "list" {
			*callCount++
			if req.Account != testIdentity.Account || req.Principal != testIdentity.Principal || req.Bucket != "example-bucket" {
				t.Fatal("execution not bound")
			}
			out.Objects = []Object{{Key: req.Prefix + "config.json", Size: 20}}
		}
		return json.Marshal(out)
	}
}
func TestBrokerPreparesExactScopeAndProviderRejectsTampering(t *testing.T) {
	calls := 0
	h := fixtureHost(t, &calls)
	p := &Provider{Host: h}
	var prepared core.CapabilityRequest
	b := &Broker{Host: h, Environments: environments{}, Capabilities: requester(func(_ context.Context, r core.CapabilityRequest) (core.CapabilityResult, error) {
		prepared = r
		return core.CapabilityResult{}, nil
	})}
	if _, err := b.List(context.Background(), ListSpec{Environment: "dev", URL: "s3://example-bucket/project/"}); err != nil {
		t.Fatal(err)
	}
	if calls != 0 || prepared.Attributes["prefix"] != "project/" || prepared.Attributes["account"] != testIdentity.Account || prepared.Attributes["iam_action"] != "s3:ListBucket" || prepared.Resource != "arn:aws:s3:::example-bucket" {
		t.Fatal("wrong prepared scope")
	}
	if _, err := p.Execute(context.Background(), prepared); err != nil || calls != 1 {
		t.Fatal(err)
	}
	for _, field := range []string{"account", "account_name", "principal", "region", "profile", "prefix", "bucket_owner", "description", "service", "unknown"} {
		encoded, _ := json.Marshal(prepared)
		var bad core.CapabilityRequest
		json.Unmarshal(encoded, &bad)
		bad.Attributes[field] = "--injected\n"
		if _, err := p.Execute(context.Background(), bad); err == nil {
			t.Fatalf("accepted %s", field)
		}
	}
	bad := prepared
	bad.Parameters = map[string]string{"endpoint": "https://evil.invalid"}
	if _, err := p.Execute(context.Background(), bad); err == nil {
		t.Fatal("opaque authority accepted")
	}
	if calls != 1 {
		t.Fatal("executed invalid request")
	}
}
func TestInvalidInputNeverAuthenticates(t *testing.T) {
	for _, url := range []string{"https://example-bucket", "s3://user:pass@example-bucket/a", "s3://example-bucket/a?token=x", "s3://example-bucket/a#fragment", "s3://example-bucket/%0a", "s3://example-bucket/%1b", "s3://example-bucket/*", "s3://bucket--x-s3/", "s3://bucket-s3alias/", "s3://-option/"} {
		b := &Broker{Host: func(context.Context, string, []byte) ([]byte, error) {
			t.Fatal("authenticated invalid input")
			return nil, nil
		}, Capabilities: requester(func(context.Context, core.CapabilityRequest) (core.CapabilityResult, error) {
			return core.CapabilityResult{}, nil
		}), Environments: environments{}}
		if _, err := b.List(context.Background(), ListSpec{Environment: "dev", URL: url}); err == nil {
			t.Fatal(url)
		}
	}
}
func TestUntrustedAgentResponseNeverLeaksOrSucceeds(t *testing.T) {
	for _, raw := range []string{`{"error":"secret-token"}`, `{"identity":{"account":"secret-token","principal":"secret-token","region":"ap-northeast-1"}}`, `{"error":"aws_denied","objects":[{"key":"secret-token","size":1}]}`, `{"unknown":"secret-token"}`, `not-json-secret-token`} {
		b := &Broker{Host: func(context.Context, string, []byte) ([]byte, error) { return []byte(raw), nil }, Capabilities: requester(func(context.Context, core.CapabilityRequest) (core.CapabilityResult, error) {
			t.Fatal("accepted hostile response")
			return core.CapabilityResult{}, nil
		}), Environments: environments{}}
		if _, err := b.List(context.Background(), ListSpec{Environment: "dev", URL: "s3://example-bucket"}); err == nil || strings.Contains(err.Error(), "secret-token") {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		code string
		want error
	}{{"identity_changed", core.ErrCapabilityStale}, {"aws_denied", ErrAWSRejected}, {"aws_failed", ErrAWSUnavailable}} {
		_, err := call(context.Background(), func(context.Context, string, []byte) ([]byte, error) {
			return []byte(`{"error":"` + tc.code + `"}`), nil
		}, agentRequest{})
		if !errors.Is(err, tc.want) {
			t.Fatal(err)
		}
	}
}

func TestListingEscapesUnicodeWithoutChangingKeys(t *testing.T) {
	original := []Object{{Key: "project/\u202e日本語😀", Size: 1}}
	data, _ := json.Marshal(original)
	text := asciiJSON(data)
	if strings.ContainsRune(text, '\u202e') || strings.Contains(text, "日本") {
		t.Fatal("terminal format control escaped incorrectly")
	}
	var decoded []Object
	if json.Unmarshal([]byte(text), &decoded) != nil || len(decoded) != 1 || decoded[0] != original[0] {
		t.Fatal("key changed", text)
	}
}

func TestAccountLabelBoundToReviewAndExecution(t *testing.T) {
	ctx := context.Background()
	original := testIdentity
	original.AccountName = "Development"
	current := original
	h := func(_ context.Context, _ string, input []byte) ([]byte, error) {
		var r agentRequest
		if err := json.Unmarshal(input, &r); err != nil {
			t.Fatal(err)
		}
		if r.Mode == "list" && r.AccountName != current.AccountName {
			return []byte(`{"error":"identity_changed"}`), nil
		}
		return json.Marshal(agentResponse{Identity: &current})
	}
	var prepared core.CapabilityRequest
	b := &Broker{Host: h, Environments: environments{}, Capabilities: requester(func(_ context.Context, r core.CapabilityRequest) (core.CapabilityResult, error) {
		prepared = r
		return core.CapabilityResult{}, nil
	})}
	if _, err := b.List(ctx, ListSpec{Environment: "dev", URL: "s3://example-bucket/project/"}); err != nil {
		t.Fatal(err)
	}
	if prepared.Attributes["account_name"] != "Development" || prepared.Attributes["account"] != original.Account {
		t.Fatal("label not bound")
	}
	p := &Provider{Host: h}
	if _, err := p.Execute(ctx, prepared); err != nil {
		t.Fatal(err)
	}
	current.AccountName = "Production"
	if _, err := p.Execute(ctx, prepared); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatalf("changed label: %v", err)
	}
}
