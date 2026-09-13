package capability

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type PolicyRule struct {
	ExpiresAt           *time.Time          `json:"expires_at,omitempty"`
	EnvironmentInstance string              `json:"environment_instance,omitempty"`
	Capability          string              `json:"capability"`
	Action              string              `json:"action"`
	Resource            string              `json:"resource"`
	Environment         string              `json:"environment,omitempty"`
	Attributes          map[string]string   `json:"attributes,omitempty"`
	Decision            core.PolicyDecision `json:"decision"`
	Reason              string              `json:"reason,omitempty"`
}

type PolicyFile struct {
	NetworkServices []NetworkService    `json:"network_services,omitempty"`
	SavedDecisions  []PolicyRule        `json:"saved_decisions,omitempty"`
	Default         core.PolicyDecision `json:"default"`
	Rules           []PolicyRule        `json:"rules"`
}

type FilePolicyEvaluator struct {
	path               string
	now                func() time.Time
	baselineRules      []PolicyRule
	baselineConfigured bool
}

func NewFilePolicyEvaluator(path string) *FilePolicyEvaluator {
	return &FilePolicyEvaluator{path: path, now: time.Now}
}

// ConfigureBaselineRules installs product-owned narrow grants that are checked
// only after all matching operator and saved Policy rules. They are not written
// to policy.json and therefore cannot be widened by guest-controlled state.
// Configuration belongs to trusted composition and must finish before the
// evaluator is served concurrently.
func (e *FilePolicyEvaluator) ConfigureBaselineRules(rules []PolicyRule) error {
	if e == nil {
		return core.ErrInvalidArgument
	}
	if e.baselineConfigured {
		return core.ErrIncompatibleState
	}
	cloned := make([]PolicyRule, 0, len(rules))
	for index, rule := range rules {
		if rule.Decision != core.PolicyAllow || rule.ExpiresAt != nil || rule.EnvironmentInstance != "" ||
			rule.Environment != "*" || rule.Resource == "*" || strings.TrimSpace(rule.Reason) == "" {
			return fmt.Errorf("baseline rule %d is not a narrow static allow: %w", index, core.ErrInvalidArgument)
		}
		for key, value := range rule.Attributes {
			if key == "*" || value == "*" {
				return fmt.Errorf("baseline rule %d contains a wildcard attribute: %w", index, core.ErrInvalidArgument)
			}
		}
		clone := rule
		if rule.Attributes != nil {
			clone.Attributes = make(map[string]string, len(rule.Attributes))
			for key, value := range rule.Attributes {
				clone.Attributes[key] = value
			}
		}
		cloned = append(cloned, clone)
	}
	if err := validatePolicy(PolicyFile{Default: core.PolicyDeny, Rules: cloned}); err != nil {
		return fmt.Errorf("validate baseline policy: %w", err)
	}
	e.baselineRules = cloned
	e.baselineConfigured = true
	return nil
}

func (e *FilePolicyEvaluator) Evaluate(_ context.Context, req core.CapabilityRequest) (core.PolicyEvaluation, error) {
	policy, err := e.load()
	if err != nil {
		return core.PolicyEvaluation{}, err
	}
	now := time.Now()
	if e.now != nil {
		now = e.now()
	}
	var selected core.PolicyEvaluation
	for index, rule := range append(append([]PolicyRule(nil), policy.Rules...), policy.SavedDecisions...) {
		if rule.ExpiresAt != nil && !now.Before(*rule.ExpiresAt) {
			continue
		}
		if index >= len(policy.Rules) && rule.Environment != "*" && rule.EnvironmentInstance != req.EnvironmentInstance {
			continue
		}
		if !ruleMatches(rule, req) {
			continue
		}
		if decisionPriority(rule.Decision) > decisionPriority(selected.Decision) {
			selected = core.PolicyEvaluation{Decision: rule.Decision, Reason: rule.Reason}
		}
	}
	if selected.Decision != "" {
		return selected, nil
	}
	for _, rule := range e.baselineRules {
		if !ruleMatches(rule, req) {
			continue
		}
		if decisionPriority(rule.Decision) > decisionPriority(selected.Decision) {
			selected = core.PolicyEvaluation{Decision: rule.Decision, Reason: rule.Reason}
		}
	}
	if selected.Decision != "" {
		return selected, nil
	}
	decision := policy.Default
	if decision == "" {
		decision = core.PolicyDeny
	}
	return core.PolicyEvaluation{Decision: decision, Reason: "default policy"}, nil
}

func ruleMatches(rule PolicyRule, req core.CapabilityRequest) bool {
	if rule.EnvironmentInstance != "" && rule.EnvironmentInstance != req.EnvironmentInstance {
		return false
	}
	if rule.Capability != req.Capability || rule.Action != req.Action {
		return false
	}
	if rule.Resource != "*" && rule.Resource != req.Resource {
		return false
	}
	if rule.Environment != "*" && rule.Environment != req.Environment {
		return false
	}
	return matchAttributes(rule.Attributes, req.Attributes)
}

func matchAttributes(required, actual map[string]string) bool {
	if len(required) != len(actual) {
		return false
	}
	for key, actualValue := range actual {
		requiredValue, ok := required[key]
		if !ok || (requiredValue != "*" && requiredValue != actualValue) {
			return false
		}
	}
	return true
}

func (e *FilePolicyEvaluator) load() (PolicyFile, error) {
	content, err := os.ReadFile(e.path)
	if errors.Is(err, os.ErrNotExist) {
		return PolicyFile{Default: core.PolicyDeny}, nil
	}
	if err != nil {
		return PolicyFile{}, fmt.Errorf("read policy %s: %w", filepath.Clean(e.path), err)
	}
	return decodePolicy(content)
}

func decodePolicy(content []byte) (PolicyFile, error) {
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var policy PolicyFile
	if err := decoder.Decode(&policy); err != nil {
		return PolicyFile{}, fmt.Errorf("parse policy: %w", err)
	}
	if err := rejectTrailingJSON(decoder); err != nil {
		return PolicyFile{}, fmt.Errorf("parse policy: %w", err)
	}
	if err := validatePolicy(policy); err != nil {
		return PolicyFile{}, fmt.Errorf("validate policy: %w", err)
	}
	return policy, nil
}

func rejectTrailingJSON(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err == io.EOF {
		return nil
	} else if err != nil {
		return err
	}
	return fmt.Errorf("multiple JSON values are not allowed")
}

func validatePolicy(policy PolicyFile) error {
	services := make(map[string]bool)
	identities := make(map[string]bool)
	for _, service := range policy.NetworkServices {
		if ValidateNetworkService(service) != nil || services[service.Name] || identities[service.Instance] {
			return fmt.Errorf("invalid or duplicate network service")
		}
		services[service.Name], identities[service.Instance] = true, true
	}

	if policy.Default != "" && !validDecision(policy.Default) {
		return fmt.Errorf("invalid default policy decision %q", policy.Default)
	}
	for _, rule := range policy.SavedDecisions {
		if rule.EnvironmentInstance != "" && !core.ValidEnvironmentInstanceID(rule.EnvironmentInstance) {
			return fmt.Errorf("invalid Environment instance identity")
		}
		if !validDecision(rule.Decision) {
			return fmt.Errorf("saved decisions must allow, deny or require approval")
		}
	}
	for index, rule := range append(append([]PolicyRule(nil), policy.Rules...), policy.SavedDecisions...) {
		if strings.TrimSpace(rule.Capability) == "" || strings.TrimSpace(rule.Action) == "" || strings.TrimSpace(rule.Resource) == "" {
			return fmt.Errorf("rule %d requires capability, action, and explicit resource", index)
		}
		if rule.EnvironmentInstance != "" && !core.ValidEnvironmentInstanceID(rule.EnvironmentInstance) {
			return fmt.Errorf("invalid Environment instance identity")
		}
		if !validDecision(rule.Decision) {
			return fmt.Errorf("rule %d has invalid policy decision %q", index, rule.Decision)
		}
		for _, value := range []string{rule.Capability, rule.Action, rule.Resource, rule.Environment, rule.Reason} {
			if strings.ContainsAny(value, "\r\n\x00") {
				return fmt.Errorf("rule %d contains invalid control characters", index)
			}
		}
		for key, value := range rule.Attributes {
			if strings.TrimSpace(key) == "" || strings.ContainsAny(key+value, "\r\n\x00") {
				return fmt.Errorf("rule %d contains invalid attribute scope", index)
			}
		}
	}
	return nil
}

// Matching explicit restrictions cannot be bypassed by reordering saved rules.
// The default is a fallback only; it does not override a matching explicit rule.
func decisionPriority(decision core.PolicyDecision) int {
	switch decision {
	case core.PolicyDeny:
		return 3
	case core.PolicyRequireApproval:
		return 2
	case core.PolicyAllow:
		return 1
	default:
		return 0
	}
}
