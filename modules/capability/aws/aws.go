// Package aws implements explicitly approved AWS operations using logical Host authentication.
package aws

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const Capability = "aws.s3"
const ListAction = "ListObjectsV2"

//go:embed host_agent.py
var HostAgent string

// Host executes the shipped agent with opaque stdin in the verified logical Host.
// Its output must be bounded. Neither this interface nor responses carry credentials.
type Host func(context.Context, string, []byte) ([]byte, error)
type Requester interface {
	Request(context.Context, core.CapabilityRequest) (core.CapabilityResult, error)
}
type Environments interface {
	GetEnvironment(context.Context, string) (core.Environment, error)
	EnvironmentInstance(context.Context, core.Environment) (string, error)
}

type ListSpec struct {
	Environment string `json:"environment"`
	URL         string `json:"url"`
	Profile     string `json:"profile,omitempty"`
	Region      string `json:"region,omitempty"`
}
type identity struct {
	AccountName string `json:"account_name,omitempty"`
	Account     string `json:"account"`
	Principal   string `json:"principal"`
	Region      string `json:"region"`
}
type agentRequest struct {
	AccountName string `json:"account_name,omitempty"`
	Mode        string `json:"mode"`
	Profile     string `json:"profile"`
	Region      string `json:"region"`
	Account     string `json:"account,omitempty"`
	Principal   string `json:"principal,omitempty"`
	Bucket      string `json:"bucket,omitempty"`
	Prefix      string `json:"prefix,omitempty"`
	Key         string `json:"key,omitempty"`
}
type agentResponse struct {
	Error    string    `json:"error,omitempty"`
	Identity *identity `json:"identity,omitempty"`
	Objects  []Object  `json:"objects,omitempty"`
}
type Object struct {
	Key  string `json:"key"`
	Size int64  `json:"size"`
}

var (
	profilePattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)
	regionPattern     = regexp.MustCompile(`^(af|ap|ca|eu|il|me|mx|sa|us)-(central|east|west|north|south|northeast|northwest|southeast|southwest)-[1-9][0-9]?$`)
	bucketPattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)
	accountPattern    = regexp.MustCompile(`^[0-9]{12}$`)
	principalPattern  = regexp.MustCompile(`^arn:aws:(iam|sts)::[0-9]{12}:(user|role|assumed-role)/[A-Za-z0-9+=,.@_/-]+$`)
	ErrAWSRejected    = errors.New("AWS rejected the operation; check AWS IAM permissions")
	ErrAWSUnavailable = errors.New("AWS operation failed; no successful result was returned")
)

func validRegion(s string) bool { return regionPattern.MatchString(s) }
func safeText(s string, limit int) bool {
	if len(s) > limit || !utf8.ValidString(s) || strings.TrimSpace(s) != s || s == "*" {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return false
		}
	}
	return true
}
func validIdentity(i identity) bool {
	return safeText(i.AccountName, 256) && i.AccountName != "unavailable" && accountPattern.MatchString(i.Account) && principalPattern.MatchString(i.Principal) &&
		strings.Contains(i.Principal, "::"+i.Account+":") && len(i.Principal) <= 256 && validRegion(i.Region)
}
func parse(s ListSpec) (ListSpec, string, string, error) {
	if s.Profile == "" {
		s.Profile = "default"
	}
	if s.Environment == "" || !safeText(s.Environment, 128) || !profilePattern.MatchString(s.Profile) || s.Region != "" && !validRegion(s.Region) {
		return s, "", "", core.ErrInvalidArgument
	}
	u, err := url.Parse(s.URL)
	if err != nil || u.Scheme != "s3" || u.Opaque != "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Port() != "" || !bucketPattern.MatchString(u.Host) || strings.Contains(u.Host, "..") ||
		strings.HasSuffix(u.Host, "--x-s3") || strings.HasSuffix(u.Host, "-s3alias") || strings.HasSuffix(u.Host, "--ol-s3") || strings.HasPrefix(u.Host, "xn--") {
		return s, "", "", core.ErrInvalidArgument
	}
	prefix := strings.TrimPrefix(u.Path, "/")
	if !safeText(prefix, 1024) {
		return s, "", "", core.ErrInvalidArgument
	}
	return s, u.Host, prefix, nil
}
func request(s ListSpec, bucket, prefix string, i identity) core.CapabilityRequest {
	return core.CapabilityRequest{Capability: Capability, Action: ListAction, Resource: "arn:aws:s3:::" + bucket, Environment: s.Environment, Attributes: map[string]string{
		"account": i.Account, "account_name": displayAccountName(i.AccountName), "principal": i.Principal, "profile": s.Profile, "region": i.Region,
		"service": "s3", "description": "List object names and sizes", "iam_action": "s3:ListBucket", "bucket": bucket, "bucket_owner": i.Account, "prefix": prefix,
	}}
}

type Broker struct {
	Host         Host
	Capabilities Requester
	Environments Environments
}

func (b *Broker) List(ctx context.Context, s ListSpec) (core.CapabilityResult, error) {
	req, err := b.prepare(ctx, s, false)
	if err != nil {
		return core.CapabilityResult{}, err
	}
	return b.Capabilities.Request(ctx, req)
}

func (b *Broker) prepare(ctx context.Context, s ListSpec, get bool) (core.CapabilityRequest, error) {
	s, bucket, prefix, err := parse(s)
	if err != nil || get && !validObjectKey(prefix) {
		if err == nil {
			err = core.ErrInvalidArgument
		}
		return core.CapabilityRequest{}, err
	}
	if b == nil || b.Host == nil || b.Capabilities == nil || b.Environments == nil {
		return core.CapabilityRequest{}, core.ErrUnsupported
	}
	environment, err := b.Environments.GetEnvironment(ctx, s.Environment)
	if err != nil {
		return core.CapabilityRequest{}, err
	}
	instance, err := b.Environments.EnvironmentInstance(ctx, environment)
	if err != nil {
		return core.CapabilityRequest{}, err
	}
	out, err := call(ctx, b.Host, agentRequest{Mode: "identity", Profile: s.Profile, Region: s.Region})
	if err != nil {
		return core.CapabilityRequest{}, err
	}
	if out.Identity == nil || !validIdentity(*out.Identity) || len(out.Objects) != 0 || s.Region != "" && out.Identity.Region != s.Region {
		return core.CapabilityRequest{}, core.ErrIncompatibleState
	}
	req := request(s, bucket, prefix, *out.Identity)
	if get {
		req = getRequest(s, bucket, prefix, *out.Identity)
	}
	req.EnvironmentInstance = instance
	return req, nil
}

type Provider struct {
	Host   Host
	Stream HostStream
}

func (*Provider) Capability() string { return Capability }
func (p *Provider) Execute(ctx context.Context, r core.CapabilityRequest) (core.CapabilityResult, error) {
	if p != nil && r.Action == GetAction {
		return p.download(ctx, r)
	}
	if p == nil || p.Host == nil {
		return core.CapabilityResult{}, core.ErrUnsupported
	}
	a := r.Attributes
	i := identity{Account: a["account"], Principal: a["principal"], Region: a["region"], AccountName: rawAccountName(a["account_name"])}
	s := ListSpec{Environment: r.Environment, Profile: a["profile"], Region: i.Region, URL: (&url.URL{Scheme: "s3", Host: a["bucket"], Path: "/" + a["prefix"]}).String()}
	_, bucket, prefix, err := parse(s)
	if err != nil || !validIdentity(i) || len(r.Parameters) != 0 {
		return core.CapabilityResult{}, core.ErrInvalidArgument
	}
	expected := request(s, bucket, prefix, i)
	if r.Capability != Capability || r.Action != ListAction || r.Resource != expected.Resource || !maps.Equal(a, expected.Attributes) {
		return core.CapabilityResult{}, core.ErrInvalidArgument
	}
	out, err := call(ctx, p.Host, agentRequest{Mode: "list", Profile: s.Profile, Region: i.Region, Account: i.Account, AccountName: i.AccountName, Principal: i.Principal, Bucket: bucket, Prefix: prefix})
	if err != nil {
		return core.CapabilityResult{}, err
	}
	if out.Identity == nil || *out.Identity != i {
		return core.CapabilityResult{}, core.ErrCapabilityStale
	}
	for _, o := range out.Objects {
		// Keys are data, never filesystem paths or terminal escape sequences.
		if !utf8.ValidString(o.Key) || len(o.Key) > 1024 || !strings.HasPrefix(o.Key, prefix) || o.Size < 0 {
			return core.CapabilityResult{}, core.ErrIncompatibleState
		}
	}
	if out.Objects == nil {
		out.Objects = []Object{}
	}
	encoded, err := json.Marshal(out.Objects)
	if err != nil {
		return core.CapabilityResult{}, core.ErrIncompatibleState
	}
	return core.CapabilityResult{Provider: Capability, Output: asciiJSON(encoded)}, nil
}
func call(ctx context.Context, h Host, r agentRequest) (agentResponse, error) {
	input, _ := json.Marshal(r)
	output, err := h(ctx, HostAgent, input)
	if err != nil {
		return agentResponse{}, fmt.Errorf("trusted AWS agent unavailable: %w", core.ErrRuntimeUnavailable)
	}
	if len(output) > 2<<20 {
		return agentResponse{}, core.ErrIncompatibleState
	}
	var out agentResponse
	d := json.NewDecoder(bytes.NewReader(output))
	d.DisallowUnknownFields()
	if d.Decode(&out) != nil || d.Decode(new(any)) != io.EOF {
		return out, core.ErrIncompatibleState
	}
	if out.Error != "" {
		if out.Identity != nil || len(out.Objects) != 0 {
			return agentResponse{}, core.ErrIncompatibleState
		}
		switch out.Error {
		case "identity_changed":
			return out, core.ErrCapabilityStale
		case "aws_denied":
			return out, ErrAWSRejected
		case "not_configured":
			return out, fmt.Errorf("configure AWS CLI v2, python3-botocore, authentication and region in the trusted Host: %w", core.ErrUnsupported)
		case "too_large":
			return out, errors.New("S3 listing exceeds the response limit; use a narrower prefix")
		case "invalid":
			return out, core.ErrInvalidArgument
		default:
			return out, ErrAWSUnavailable
		}
	}
	return out, nil
}

// Preserve exact Unicode keys while keeping invisible formatting out of terminals.
func asciiJSON(encoded []byte) string {
	var out strings.Builder
	for _, r := range string(encoded) {
		if r < 128 {
			out.WriteRune(r)
			continue
		}
		for _, unit := range utf16.Encode([]rune{r}) {
			fmt.Fprintf(&out, "\\u%04x", unit)
		}
	}
	return out.String()
}

// Account names are trusted Host labels tied to an actual STS account ID.
func displayAccountName(name string) string {
	if name == "" {
		return "unavailable"
	}
	return name
}
func rawAccountName(name string) string {
	if name == "unavailable" {
		return ""
	}
	return name
}
