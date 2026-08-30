package awscap

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

const Capability = "aws.api"

const (
	ActionCallerIdentity   = "sts.get-caller-identity"
	ActionDescribeInstance = "ec2.describe-instance"
	ActionDescribeVolume   = "ec2.describe-volume"
)

type QueryKind string

const (
	QueryCallerIdentity QueryKind = "caller-identity"
	QueryInstance       QueryKind = "instance"
	QueryVolume         QueryKind = "volume"
)

type QuerySpec struct {
	AccountID string
	Region    string
	Kind      QueryKind
	ID        string
}

type capabilityRequester interface {
	Request(context.Context, core.CapabilityRequest) (core.CapabilityResult, error)
}

type Broker struct{ capabilities capabilityRequester }

func NewBroker(capabilities capabilityRequester) *Broker { return &Broker{capabilities: capabilities} }

func (b *Broker) Query(ctx context.Context, spec QuerySpec) (core.CapabilityResult, error) {
	req, err := requestFor(spec)
	if err != nil {
		return core.CapabilityResult{}, err
	}
	if b == nil || b.capabilities == nil {
		return core.CapabilityResult{}, core.ErrPolicyDenied
	}
	return b.capabilities.Request(ctx, req)
}

func requestFor(spec QuerySpec) (core.CapabilityRequest, error) {
	accountID := strings.TrimSpace(spec.AccountID)
	region := strings.TrimSpace(spec.Region)
	id := strings.TrimSpace(spec.ID)
	if !accountPattern.MatchString(accountID) || !regionPattern.MatchString(region) {
		return core.CapabilityRequest{}, core.ErrInvalidArgument
	}

	req := core.CapabilityRequest{Capability: Capability}
	switch spec.Kind {
	case QueryCallerIdentity:
		if id != "" {
			return core.CapabilityRequest{}, core.ErrInvalidArgument
		}
		req.Action = ActionCallerIdentity
		req.Resource = "aws://" + accountID + "/" + region + "/identity"
	case QueryInstance:
		if !instancePattern.MatchString(id) {
			return core.CapabilityRequest{}, core.ErrInvalidArgument
		}
		req.Action = ActionDescribeInstance
		req.Resource = "aws://" + accountID + "/" + region + "/ec2/instance/" + id
	case QueryVolume:
		if !volumePattern.MatchString(id) {
			return core.CapabilityRequest{}, core.ErrInvalidArgument
		}
		req.Action = ActionDescribeVolume
		req.Resource = "aws://" + accountID + "/" + region + "/ec2/volume/" + id
	default:
		return core.CapabilityRequest{}, core.ErrUnsupported
	}
	return req, nil
}

type Provider struct{ runner host.Runner }

func NewProvider(runner host.Runner) *Provider { return &Provider{runner: runner} }
func (*Provider) Capability() string            { return Capability }

func (p *Provider) Execute(ctx context.Context, req core.CapabilityRequest) (core.CapabilityResult, error) {
	if p == nil || p.runner == nil {
		return core.CapabilityResult{}, core.ErrRuntimeUnavailable
	}
	if req.Capability != Capability || len(req.Parameters) != 0 || len(req.Attributes) != 0 {
		return core.CapabilityResult{}, core.ErrInvalidArgument
	}

	expectedAccount, region, id, err := parseResource(req.Action, req.Resource)
	if err != nil {
		return core.CapabilityResult{}, err
	}

	identity, runErr := p.runner.Run(ctx, "aws",
		"--region", region,
		"--no-cli-pager",
		"sts", "get-caller-identity",
		"--output", "json",
	)
	if runErr != nil {
		return core.CapabilityResult{}, fmt.Errorf("verify AWS capability caller identity: %w", runErr)
	}
	actualAccount, err := accountFromIdentity(identity.Stdout)
	if err != nil {
		return core.CapabilityResult{}, err
	}
	if actualAccount != expectedAccount {
		return core.CapabilityResult{}, fmt.Errorf(
			"AWS capability authority account changed from %s to %s: %w",
			expectedAccount,
			actualAccount,
			core.ErrCapabilityStale,
		)
	}

	if req.Action == ActionCallerIdentity {
		return core.CapabilityResult{Provider: Capability, Output: identity.Stdout}, nil
	}

	args := []string{"--region", region, "--no-cli-pager"}
	switch req.Action {
	case ActionDescribeInstance:
		args = append(args, "ec2", "describe-instances", "--instance-ids", id, "--output", "json")
	case ActionDescribeVolume:
		args = append(args, "ec2", "describe-volumes", "--volume-ids", id, "--output", "json")
	default:
		return core.CapabilityResult{}, core.ErrUnsupported
	}
	result, runErr := p.runner.Run(ctx, "aws", args...)
	return core.CapabilityResult{Provider: Capability, Output: result.Stdout}, runErr
}

func accountFromIdentity(raw string) (string, error) {
	var identity struct {
		Account string `json:"Account"`
	}
	if err := json.Unmarshal([]byte(raw), &identity); err != nil {
		return "", fmt.Errorf("decode AWS caller identity: %w", core.ErrCapabilityStale)
	}
	accountID := strings.TrimSpace(identity.Account)
	if !accountPattern.MatchString(accountID) {
		return "", fmt.Errorf("invalid AWS caller account identity %q: %w", accountID, core.ErrCapabilityStale)
	}
	return accountID, nil
}

func parseResource(action, resource string) (string, string, string, error) {
	if !strings.HasPrefix(resource, "aws://") || strings.ContainsAny(resource, "\r\n\x00") {
		return "", "", "", core.ErrInvalidArgument
	}
	parts := strings.Split(strings.TrimPrefix(resource, "aws://"), "/")
	if len(parts) < 3 || !accountPattern.MatchString(parts[0]) || !regionPattern.MatchString(parts[1]) {
		return "", "", "", core.ErrInvalidArgument
	}
	accountID, region := parts[0], parts[1]
	switch action {
	case ActionCallerIdentity:
		if len(parts) != 3 || parts[2] != "identity" {
			return "", "", "", core.ErrCapabilityStale
		}
		return accountID, region, "", nil
	case ActionDescribeInstance:
		if len(parts) != 5 || parts[2] != "ec2" || parts[3] != "instance" || !instancePattern.MatchString(parts[4]) {
			return "", "", "", core.ErrCapabilityStale
		}
		return accountID, region, parts[4], nil
	case ActionDescribeVolume:
		if len(parts) != 5 || parts[2] != "ec2" || parts[3] != "volume" || !volumePattern.MatchString(parts[4]) {
			return "", "", "", core.ErrCapabilityStale
		}
		return accountID, region, parts[4], nil
	default:
		return "", "", "", fmt.Errorf("AWS action %q: %w", action, core.ErrUnsupported)
	}
}

var (
	accountPattern  = regexp.MustCompile(`^[0-9]{12}$`)
	regionPattern   = regexp.MustCompile(`^[a-z]{2}(?:-gov)?-[a-z0-9-]+-\d+$`)
	instancePattern = regexp.MustCompile(`^i-[A-Za-z0-9]{8,32}$`)
	volumePattern   = regexp.MustCompile(`^vol-[A-Za-z0-9]{8,32}$`)
)
