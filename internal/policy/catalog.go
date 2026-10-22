package capability

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// PolicyDescriptorProvider describes static, policy-visible authority. It must
// not inspect credentials, configuration, prepared requests or opaque parameters.
// Descriptors are copied once at registration, never obtained during discovery.
type PolicyDescriptorProvider interface {
	PolicyDescriptor() CapabilityDescriptor
}

type CapabilityCatalog struct {
	Version      int                    `json:"version"`
	Matching     PolicyMatching         `json:"matching"`
	Capabilities []CapabilityDescriptor `json:"capabilities"`
}

type PolicyMatching struct {
	Capability  string `json:"capability"`
	Action      string `json:"action"`
	Resource    string `json:"resource"`
	Environment string `json:"environment"`
	Attributes  string `json:"attributes"`
}

type CapabilityDescriptor struct {
	Capability string             `json:"capability"`
	Actions    []ActionDescriptor `json:"actions"`
}

type ActionDescriptor struct {
	Action      string                `json:"action"`
	Description string                `json:"description"`
	Resource    ValueDescriptor       `json:"resource"`
	Attributes  []AttributeDescriptor `json:"attributes"`
	Notes       []string              `json:"notes,omitempty"`
}

type ValueDescriptor struct {
	Description string   `json:"description"`
	Values      []string `json:"values,omitempty"`
}

// Every listed attribute is required; matching uses the complete key set.
type AttributeDescriptor struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Values      []string `json:"values,omitempty"`
}

func policyMatching() PolicyMatching {
	return PolicyMatching{
		Capability: "exact", Action: "exact",
		Resource: "exact or explicit *", Environment: "exact or explicit *; omitted matches only empty",
		Attributes: "exact key set; each value is exact or explicit * (not a glob)",
	}
}

func buildCapabilityCatalog(providers map[string]Provider) (CapabilityCatalog, error) {
	catalog := CapabilityCatalog{Version: 1, Matching: policyMatching(), Capabilities: []CapabilityDescriptor{}}
	for name, provider := range providers {
		// Exclude only the concrete local demonstration, not other providers
		// that share a name or embed LocalEcho. They must describe their schema.
		switch provider.(type) {
		case LocalEcho, *LocalEcho:
			continue
		}
		described, ok := provider.(PolicyDescriptorProvider)
		if !ok {
			return CapabilityCatalog{}, fmt.Errorf("capability metadata is incomplete: %w", core.ErrUnsupported)
		}
		descriptor := cloneCapabilityDescriptor(described.PolicyDescriptor())
		if descriptor.Capability != name {
			return CapabilityCatalog{}, core.ErrInvalidArgument
		}
		slices.SortFunc(descriptor.Actions, func(a, b ActionDescriptor) int { return strings.Compare(a.Action, b.Action) })
		for i := range descriptor.Actions {
			slices.SortFunc(descriptor.Actions[i].Attributes, func(a, b AttributeDescriptor) int { return strings.Compare(a.Name, b.Name) })
		}
		catalog.Capabilities = append(catalog.Capabilities, descriptor)
	}
	slices.SortFunc(catalog.Capabilities, func(a, b CapabilityDescriptor) int { return strings.Compare(a.Capability, b.Capability) })
	if err := ValidateCapabilityCatalog(catalog); err != nil {
		return CapabilityCatalog{}, err
	}
	return catalog, nil
}

// CapabilityCatalog is metadata-only: it never evaluates Policy, records audit,
// asks for approval, resolves a target, or calls a provider's Execute method.
func (s *Service) CapabilityCatalog(ctx context.Context) (CapabilityCatalog, error) {
	if err := ctx.Err(); err != nil {
		return CapabilityCatalog{}, err
	}
	if s == nil {
		return CapabilityCatalog{}, core.ErrUnsupported
	}
	if s.catalogErr != nil {
		return CapabilityCatalog{}, s.catalogErr
	}
	if s.catalog.Version != 1 {
		return CapabilityCatalog{}, core.ErrUnsupported
	}
	result := s.catalog
	result.Capabilities = make([]CapabilityDescriptor, len(s.catalog.Capabilities))
	for i, descriptor := range s.catalog.Capabilities {
		result.Capabilities[i] = cloneCapabilityDescriptor(descriptor)
	}
	return result, nil
}

func cloneCapabilityDescriptor(in CapabilityDescriptor) CapabilityDescriptor {
	out := in
	out.Actions = make([]ActionDescriptor, len(in.Actions))
	for i, action := range in.Actions {
		out.Actions[i] = action
		out.Actions[i].Resource.Values = slices.Clone(action.Resource.Values)
		out.Actions[i].Notes = slices.Clone(action.Notes)
		out.Actions[i].Attributes = make([]AttributeDescriptor, len(action.Attributes))
		for j, attribute := range action.Attributes {
			out.Actions[i].Attributes[j] = attribute
			out.Actions[i].Attributes[j].Values = slices.Clone(attribute.Values)
		}
	}
	return out
}

// ValidateCapabilityCatalog also protects presentation from malformed controller
// responses. Version 1 is bounded, unique, sorted and free of terminal controls.
func ValidateCapabilityCatalog(catalog CapabilityCatalog) error {
	if catalog.Version != 1 || catalog.Matching != policyMatching() || catalog.Capabilities == nil || len(catalog.Capabilities) > 64 {
		return core.ErrInvalidArgument
	}
	previous := ""
	for _, descriptor := range catalog.Capabilities {
		if !catalogIdentifier(descriptor.Capability) || descriptor.Capability <= previous || len(descriptor.Actions) < 1 || len(descriptor.Actions) > 32 {
			return core.ErrInvalidArgument
		}
		previous = descriptor.Capability
		previousAction := ""
		for _, action := range descriptor.Actions {
			if !catalogIdentifier(action.Action) || action.Action <= previousAction || !catalogText(action.Description) || !catalogText(action.Resource.Description) || !catalogValues(action.Resource.Values) || action.Attributes == nil || len(action.Attributes) > 64 || len(action.Notes) > 16 {
				return core.ErrInvalidArgument
			}
			previousAction = action.Action
			previousAttribute := ""
			for _, attribute := range action.Attributes {
				if !catalogIdentifier(attribute.Name) || attribute.Name <= previousAttribute || !catalogText(attribute.Description) || !catalogValues(attribute.Values) {
					return core.ErrInvalidArgument
				}
				previousAttribute = attribute.Name
			}
			for _, note := range action.Notes {
				if !catalogText(note) {
					return core.ErrInvalidArgument
				}
			}
		}
	}
	return nil
}

func catalogIdentifier(value string) bool {
	if len(value) < 1 || len(value) > 128 {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}

func catalogText(value string) bool {
	if value == "" || len(value) > 2048 || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return false
		}
	}
	return true
}

func catalogValues(values []string) bool {
	if len(values) > 64 {
		return false
	}
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if !catalogText(value) || seen[value] {
			return false
		}
		seen[value] = true
	}
	return true
}
