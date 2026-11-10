package capability

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type catalogTestProvider struct {
	descriptor   CapabilityDescriptor
	descriptions int
	executions   int
}

func (p *catalogTestProvider) Capability() string { return p.descriptor.Capability }
func (p *catalogTestProvider) PolicyDescriptor() CapabilityDescriptor {
	p.descriptions++
	return p.descriptor
}
func (p *catalogTestProvider) Execute(context.Context, core.CapabilityRequest) (core.CapabilityResult, error) {
	p.executions++
	return core.CapabilityResult{}, errors.New("must not execute")
}

func catalogFixture(name string) *catalogTestProvider {
	return &catalogTestProvider{descriptor: CapabilityDescriptor{Capability: name, Actions: []ActionDescriptor{{
		Action: "operate", Description: "Describe an operation", Resource: ValueDescriptor{Description: "An exact target", Values: []string{"target"}},
		Attributes: []AttributeDescriptor{{Name: "protocol", Description: "Protocol", Values: []string{"tcp", "udp"}}}, Notes: []string{"Metadata only"},
	}}}}
}

func TestCapabilityCatalogIsSortedImmutableAndSideEffectFree(t *testing.T) {
	first, last := catalogFixture("a.production"), catalogFixture("z.production")
	audit, approval := &fakeAudit{}, &fakeApproval{}
	s, err := New(fakePolicy{err: errors.New("must not evaluate")}, approval, audit, last, LocalEcho{}, first)
	if err != nil {
		t.Fatal(err)
	}
	first.descriptor.Actions[0].Resource.Values[0] = "changed after registration"
	got, err := s.CapabilityCatalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Capabilities) != 2 || got.Capabilities[0].Capability != "a.production" || got.Capabilities[1].Capability != "z.production" {
		t.Fatal(got)
	}
	baseline, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(baseline), "changed") || strings.Contains(string(baseline), "local.echo") {
		t.Fatal(string(baseline))
	}
	got.Capabilities[0].Actions[0].Resource.Values[0] = "changed resource"
	got.Capabilities[0].Actions[0].Attributes[0].Values[0] = "changed enum"
	got.Capabilities[0].Actions[0].Attributes[0].Name = "changed attribute"
	got.Capabilities[0].Actions[0].Notes[0] = "changed note"
	got.Capabilities[0].Actions[0].Action = "changed action"
	got.Capabilities[0].Capability = "changed capability"
	again, err := s.CapabilityCatalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(again)
	if string(encoded) != string(baseline) {
		t.Fatal("discovery returned mutable registry storage")
	}
	if first.descriptions != 1 || last.descriptions != 1 || first.executions != 0 || last.executions != 0 || approval.calls != 0 || len(audit.events) != 0 {
		t.Fatal("discovery invoked an execution boundary")
	}
}

// Embedding the demo must not make a real provider invisible.
type embeddedEchoCatalogProvider struct{ LocalEcho }

func (embeddedEchoCatalogProvider) Capability() string { return "production.embedded" }

func TestCapabilityCatalogMissingAndInvalidMetadataFailsClosed(t *testing.T) {
	for _, p := range []Provider{namedProvider("production.missing"), namedProvider("local.echo"), embeddedEchoCatalogProvider{}} {
		s, err := New(nil, nil, nil, p)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := s.CapabilityCatalog(context.Background()); err == nil || got.Version != 0 {
			t.Fatal("silently omitted registered provider", got, err)
		}
	}
	for _, mutate := range []func(*CapabilityDescriptor){
		func(d *CapabilityDescriptor) { d.Capability = "other" },
		func(d *CapabilityDescriptor) { d.Actions = nil },
		func(d *CapabilityDescriptor) { d.Actions = append(d.Actions, d.Actions[0]) },
		func(d *CapabilityDescriptor) {
			d.Actions[0].Attributes = append(d.Actions[0].Attributes, d.Actions[0].Attributes[0])
		},
		func(d *CapabilityDescriptor) { d.Actions[0].Description = "escape\x1b[0m" },
		func(d *CapabilityDescriptor) { d.Actions[0].Notes = []string{"bidi\u202econtrol"} },
		func(d *CapabilityDescriptor) { d.Actions[0].Resource.Description = strings.Repeat("x", 2049) },
		func(d *CapabilityDescriptor) { d.Actions[0].Attributes[0].Values = []string{"tcp", "tcp"} },
	} {
		p := catalogFixture("production.test")
		// Capture the registration name independently for the mismatch case.
		d := p.descriptor
		mutate(&d)
		p.descriptor = d
		providers := map[string]Provider{"production.test": p}
		if _, err := buildCapabilityCatalog(providers); err == nil {
			t.Fatal("accepted invalid metadata", d)
		}
	}
}

func TestCapabilityCatalogCancellationAndUnavailable(t *testing.T) {
	s, err := New(nil, nil, nil, LocalEcho{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.CapabilityCatalog(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	for _, service := range []*Service{nil, {}} {
		if _, err := service.CapabilityCatalog(context.Background()); !errors.Is(err, core.ErrUnsupported) {
			t.Fatal(err)
		}
	}
}

func TestCapabilityCatalogMatchingDescribesActualPolicy(t *testing.T) {
	rule := PolicyRule{Capability: "network.egress", Action: "connect", Resource: "example.com", Environment: "dev", Attributes: map[string]string{"protocol": "https", "port": "443"}}
	request := core.CapabilityRequest{Capability: rule.Capability, Action: rule.Action, Resource: rule.Resource, Environment: rule.Environment, Attributes: map[string]string{"protocol": "https", "port": "443"}}
	if !ruleMatches(rule, request) {
		t.Fatal("documented exact rule does not match")
	}
	rule.Resource, rule.Environment, rule.Attributes["port"] = "*", "*", "*"
	if !ruleMatches(rule, request) {
		t.Fatal("explicit wildcard does not match")
	}
	rule.Attributes["port"] = "4*"
	if ruleMatches(rule, request) {
		t.Fatal("partial glob matched")
	}
	rule.Attributes["port"] = "*"
	delete(rule.Attributes, "protocol")
	if ruleMatches(rule, request) {
		t.Fatal("omitted attribute matched")
	}
	rule.Attributes["protocol"] = "https"
	rule.Attributes["extra"] = "*"
	if ruleMatches(rule, request) {
		t.Fatal("extra attribute matched")
	}
}

func TestCapabilityCatalogIdentifierAlphabetAndBounds(t *testing.T) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._-"
	for value := range 128 {
		r := rune(value)
		if got, want := catalogIdentifier(string(r)), strings.ContainsRune(alphabet, r); got != want {
			t.Errorf("catalogIdentifier(%q) = %v, want %v", r, got, want)
		}
	}
	if !catalogIdentifier(strings.Repeat("a", 128)) {
		t.Fatal("rejected maximum-length identifier")
	}
	for _, value := range []string{"", strings.Repeat("a", 129), "é", "\u202e", "a\x00b", "a\xffb"} {
		if catalogIdentifier(value) {
			t.Errorf("accepted invalid identifier %q", value)
		}
	}
}
