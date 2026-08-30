package awscap

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

const testAccountID = "123456789012"

type captureCapabilities struct{ req core.CapabilityRequest }

func (c *captureCapabilities) Request(_ context.Context, req core.CapabilityRequest) (core.CapabilityResult, error) {
	c.req = req
	return core.CapabilityResult{Provider: Capability}, nil
}

func TestBrokerMakesAWSAccountAndRegionFullyPolicyVisible(t *testing.T) {
	capture := &captureCapabilities{}
	broker := NewBroker(capture)
	_, err := broker.Query(context.Background(), QuerySpec{AccountID: testAccountID, Region: "ap-northeast-1", Kind: QueryInstance, ID: "i-0123456789abcdef0"})
	if err != nil {
		t.Fatal(err)
	}
	req := capture.req
	if req.Capability != Capability || req.Action != ActionDescribeInstance || req.Resource != "aws://123456789012/ap-northeast-1/ec2/instance/i-0123456789abcdef0" {
		t.Fatalf("req=%#v", req)
	}
	if len(req.Attributes) != 0 || len(req.Parameters) != 0 {
		t.Fatalf("attrs=%#v params=%#v", req.Attributes, req.Parameters)
	}
}

func TestBrokerRejectsImplicitOrMalformedAuthority(t *testing.T) {
	broker := NewBroker(&captureCapabilities{})
	for _, spec := range []QuerySpec{
		{Kind: QueryInstance, AccountID: "", Region: "ap-northeast-1", ID: "i-0123456789abcdef0"},
		{Kind: QueryInstance, AccountID: "1234", Region: "ap-northeast-1", ID: "i-0123456789abcdef0"},
		{Kind: QueryInstance, AccountID: testAccountID, Region: "", ID: "i-0123456789abcdef0"},
		{Kind: QueryInstance, AccountID: testAccountID, Region: "ap-northeast-1", ID: "../../bad"},
		{Kind: "mutate", AccountID: testAccountID, Region: "ap-northeast-1"},
	} {
		if _, err := broker.Query(context.Background(), spec); err == nil {
			t.Fatalf("accepted %#v", spec)
		}
	}
}

type runner struct {
	calls          []string
	accountID      string
	identityOutput string
	identityErr    error
	result         host.Result
	err            error
}

func (r *runner) Run(_ context.Context, name string, args ...string) (host.Result, error) {
	call := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, call)
	if strings.Contains(call, " sts get-caller-identity ") {
		if r.identityErr != nil {
			return host.Result{ExitCode: 1}, r.identityErr
		}
		if r.identityOutput != "" {
			return host.Result{Stdout: r.identityOutput}, nil
		}
		accountID := r.accountID
		if accountID == "" {
			accountID = testAccountID
		}
		return host.Result{Stdout: `{"Account":"` + accountID + `","Arn":"arn:aws:iam::` + accountID + `:role/test"}`}, nil
	}
	return r.result, r.err
}

func TestProviderVerifiesAccountBeforeNormalizedReadOperation(t *testing.T) {
	r := &runner{result: host.Result{Stdout: `{"Reservations":[]}`}}
	p := NewProvider(r)
	req, _ := requestFor(QuerySpec{AccountID: testAccountID, Region: "ap-northeast-1", Kind: QueryInstance, ID: "i-0123456789abcdef0"})
	result, err := p.Execute(context.Background(), req)
	if err != nil || result.Output == "" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if len(r.calls) != 2 {
		t.Fatalf("calls=%v", r.calls)
	}
	if !strings.Contains(r.calls[0], "aws --region ap-northeast-1 --no-cli-pager sts get-caller-identity --output json") {
		t.Fatalf("identity call=%s", r.calls[0])
	}
	if !strings.Contains(r.calls[1], "aws --region ap-northeast-1 --no-cli-pager ec2 describe-instances --instance-ids i-0123456789abcdef0 --output json") {
		t.Fatalf("resource call=%s", r.calls[1])
	}
}

func TestProviderRejectsChangedAccountBeforeResourceOperation(t *testing.T) {
	r := &runner{accountID: "210987654321", result: host.Result{Stdout: `{"Reservations":[]}`}}
	p := NewProvider(r)
	req, _ := requestFor(QuerySpec{AccountID: testAccountID, Region: "ap-northeast-1", Kind: QueryInstance, ID: "i-0123456789abcdef0"})
	_, err := p.Execute(context.Background(), req)
	if !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatalf("err=%v", err)
	}
	if len(r.calls) != 1 || !strings.Contains(r.calls[0], "sts get-caller-identity") {
		t.Fatalf("account mismatch reached resource call: %v", r.calls)
	}
}

func TestProviderRejectsMalformedCallerIdentityBeforeResourceOperation(t *testing.T) {
	r := &runner{identityOutput: `{"Account":"not-an-account"}`}
	p := NewProvider(r)
	req, _ := requestFor(QuerySpec{AccountID: testAccountID, Region: "ap-northeast-1", Kind: QueryVolume, ID: "vol-0123456789abcdef0"})
	_, err := p.Execute(context.Background(), req)
	if !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatalf("err=%v", err)
	}
	if len(r.calls) != 1 {
		t.Fatalf("invalid identity reached resource call: %v", r.calls)
	}
}

func TestProviderCallerIdentityRequiresExpectedAccountAndReturnsVerifiedIdentity(t *testing.T) {
	r := &runner{}
	p := NewProvider(r)
	req, _ := requestFor(QuerySpec{AccountID: testAccountID, Region: "ap-northeast-1", Kind: QueryCallerIdentity})
	result, err := p.Execute(context.Background(), req)
	if err != nil || !strings.Contains(result.Output, testAccountID) {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if len(r.calls) != 1 || !strings.Contains(r.calls[0], "sts get-caller-identity") {
		t.Fatalf("calls=%v", r.calls)
	}
}

func TestProviderRejectsActionResourceMutationBeforeAWSCalls(t *testing.T) {
	r := &runner{}
	p := NewProvider(r)
	req, _ := requestFor(QuerySpec{AccountID: testAccountID, Region: "ap-northeast-1", Kind: QueryInstance, ID: "i-0123456789abcdef0"})
	for _, mutate := range []func(*core.CapabilityRequest){
		func(r *core.CapabilityRequest) { r.Resource = "aws://123456789012/ap-northeast-1/ec2/volume/vol-0123456789abcdef0" },
		func(r *core.CapabilityRequest) { r.Action = ActionDescribeVolume },
	} {
		stale := req
		mutate(&stale)
		before := len(r.calls)
		if _, err := p.Execute(context.Background(), stale); err == nil || !(errors.Is(err, core.ErrCapabilityStale) || errors.Is(err, core.ErrUnsupported)) {
			t.Fatalf("mutated request err=%v", err)
		}
		if len(r.calls) != before {
			t.Fatalf("mutated request reached AWS: %v", r.calls[before:])
		}
	}
}
