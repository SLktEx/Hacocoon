package awscap

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

type captureCapabilities struct{ req core.CapabilityRequest }

func (c *captureCapabilities) Request(_ context.Context, req core.CapabilityRequest) (core.CapabilityResult, error) {
	c.req = req
	return core.CapabilityResult{Provider: Capability}, nil
}

func TestBrokerMakesAWSAuthorityFullyPolicyVisible(t *testing.T) {
	capture := &captureCapabilities{}
	broker := NewBroker(capture)
	_, err := broker.Query(context.Background(), QuerySpec{AccountID: "123456789012", Region: "ap-northeast-1", Kind: QueryInstance, ID: "i-0123456789abcdef0"})
	if err != nil {
		t.Fatal(err)
	}
	req := capture.req
	if req.Capability != Capability || req.Action != ActionDescribeInstance || req.Resource != "aws://123456789012/ap-northeast-1/ec2/instance/i-0123456789abcdef0" {
		t.Fatalf("req=%#v", req)
	}
	if req.Attributes["aws.account_id"] != "123456789012" || req.Attributes["aws.region"] != "ap-northeast-1" || len(req.Attributes) != 2 || len(req.Parameters) != 0 {
		t.Fatalf("attrs=%#v params=%#v", req.Attributes, req.Parameters)
	}
}

func TestBrokerRejectsImplicitOrMalformedAuthority(t *testing.T) {
	broker := NewBroker(&captureCapabilities{})
	for _, spec := range []QuerySpec{
		{AccountID: "", Kind: QueryInstance, Region: "ap-northeast-1", ID: "i-0123456789abcdef0"},
		{AccountID: "123", Kind: QueryInstance, Region: "ap-northeast-1", ID: "i-0123456789abcdef0"},
		{AccountID: "123456789012", Kind: QueryInstance, Region: "", ID: "i-0123456789abcdef0"},
		{AccountID: "123456789012", Kind: QueryInstance, Region: "ap-northeast-1", ID: "../../bad"},
		{AccountID: "123456789012", Kind: "mutate", Region: "ap-northeast-1"},
	} {
		if _, err := broker.Query(context.Background(), spec); err == nil {
			t.Fatalf("accepted %#v", spec)
		}
	}
}

type runner struct {
	calls   []string
	results []host.Result
	errs    []error
}

func (r *runner) Run(_ context.Context, name string, args ...string) (host.Result, error) {
	r.calls = append(r.calls, name+" "+strings.Join(args, " "))
	idx := len(r.calls) - 1
	var result host.Result
	if idx < len(r.results) {
		result = r.results[idx]
	}
	var err error
	if idx < len(r.errs) {
		err = r.errs[idx]
	}
	return result, err
}

func TestProviderExecutesOnlyNormalizedReadOperationsInPinnedAccount(t *testing.T) {
	r := &runner{results: []host.Result{{Stdout: "123456789012\n"}, {Stdout: `{"Reservations":[]}`}}}
	p := NewProvider(r)
	req, _ := requestFor(QuerySpec{AccountID: "123456789012", Region: "ap-northeast-1", Kind: QueryInstance, ID: "i-0123456789abcdef0"})
	result, err := p.Execute(context.Background(), req)
	if err != nil || result.Output == "" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if len(r.calls) != 2 || !strings.Contains(r.calls[0], "sts get-caller-identity --query Account --output text") || !strings.Contains(r.calls[1], "aws --region ap-northeast-1 --no-cli-pager ec2 describe-instances --instance-ids i-0123456789abcdef0 --output json") {
		t.Fatalf("calls=%#v", r.calls)
	}
}

func TestProviderRejectsCredentialAccountChangeBeforeTargetOperation(t *testing.T) {
	r := &runner{results: []host.Result{{Stdout: "999999999999\n"}}}
	p := NewProvider(r)
	req, _ := requestFor(QuerySpec{AccountID: "123456789012", Region: "ap-northeast-1", Kind: QueryInstance, ID: "i-0123456789abcdef0"})
	if _, err := p.Execute(context.Background(), req); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatalf("account mismatch err=%v", err)
	}
	if len(r.calls) != 1 || strings.Contains(strings.Join(r.calls, "\n"), "describe-instances") {
		t.Fatalf("account mismatch reached target AWS operation: %#v", r.calls)
	}
}

func TestProviderRejectsTamperedResourceOrAttributes(t *testing.T) {
	base, _ := requestFor(QuerySpec{AccountID: "123456789012", Region: "ap-northeast-1", Kind: QueryInstance, ID: "i-0123456789abcdef0"})
	cases := []func(*core.CapabilityRequest){
		func(req *core.CapabilityRequest) { req.Resource = "aws://999999999999/ap-northeast-1/ec2/instance/i-0123456789abcdef0" },
		func(req *core.CapabilityRequest) { req.Resource = "aws://123456789012/ap-northeast-1/ec2/volume/vol-0123456789abcdef0" },
		func(req *core.CapabilityRequest) { req.Action = ActionDescribeVolume },
		func(req *core.CapabilityRequest) { req.Attributes = map[string]string{"aws.account_id": "999999999999", "aws.region": "ap-northeast-1"} },
		func(req *core.CapabilityRequest) { req.Attributes = map[string]string{"aws.account_id": "123456789012", "aws.region": "us-east-1"} },
	}
	for _, mutate := range cases {
		req := base
		req.Attributes = map[string]string{"aws.account_id": base.Attributes["aws.account_id"], "aws.region": base.Attributes["aws.region"]}
		mutate(&req)
		r := &runner{results: []host.Result{{Stdout: "123456789012\n"}}}
		_, err := NewProvider(r).Execute(context.Background(), req)
		if err == nil || !(errors.Is(err, core.ErrCapabilityStale) || errors.Is(err, core.ErrUnsupported)) {
			t.Fatalf("mutated request %#v err=%v", req, err)
		}
		if strings.Contains(strings.Join(r.calls, "\n"), "describe-instances") || strings.Contains(strings.Join(r.calls, "\n"), "describe-volumes") {
			t.Fatalf("mutated request reached target operation: %#v", r.calls)
		}
	}
}
