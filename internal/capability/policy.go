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

	"github.com/SLktEx/Hacocoon/internal/core"
)

type PolicyRule struct {
	Capability  string              `json:"capability"`
	Action      string              `json:"action"`
	Resource    string              `json:"resource"`
	Environment string              `json:"environment,omitempty"`
	Parameters  map[string]string   `json:"parameters,omitempty"`
	Decision    core.PolicyDecision `json:"decision"`
	Reason      string              `json:"reason,omitempty"`
}

type PolicyFile struct {
	Default core.PolicyDecision `json:"default"`
	Rules   []PolicyRule        `json:"rules"`
}

type FilePolicyEvaluator struct {
	path string
}

func NewFilePolicyEvaluator(path string) *FilePolicyEvaluator {
	return &FilePolicyEvaluator{path: path}
}

func (e *FilePolicyEvaluator) Evaluate(_ context.Context, req core.CapabilityRequest) (core.PolicyEvaluation, error) {
	policy, err := e.load()
	if err != nil {
		return core.PolicyEvaluation{}, err
	}
	for _, rule := range policy.Rules {
		if !ruleMatches(rule, req) {
			continue
		}
		return core.PolicyEvaluation{Decision: rule.Decision, Reason: rule.Reason}, nil
	}
	decision := policy.Default
	if decision == "" {
		decision = core.PolicyDeny
	}
	return core.PolicyEvaluation{Decision: decision, Reason: "default policy"}, nil
}

func ruleMatches(rule PolicyRule, req core.CapabilityRequest) bool {
	if rule.Capability != req.Capability || rule.Action != req.Action {
		return false
	}
	if rule.Resource != "*" && rule.Resource != req.Resource {
		return false
	}
	if rule.Environment != "*" && rule.Environment != req.Environment {
		return false
	}
	return parametersMatch(rule.Parameters, req.Parameters)
}

func parametersMatch(rule, request map[string]string) bool {
	if len(rule) != len(request) {
		return false
	}
	for key, value := range request {
		expected, ok := rule[key]
		if !ok || (expected != "*" && expected != value) {
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
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var policy PolicyFile
	if err := decoder.Decode(&policy); err != nil {
		return PolicyFile{}, fmt.Errorf("parse policy %s: %w", filepath.Clean(e.path), err)
	}
	if err := rejectTrailingJSON(decoder); err != nil {
		return PolicyFile{}, fmt.Errorf("parse policy %s: %w", filepath.Clean(e.path), err)
	}
	if err := validatePolicy(policy); err != nil {
		return PolicyFile{}, fmt.Errorf("validate policy %s: %w", filepath.Clean(e.path), err)
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
	if policy.Default != "" && !validDecision(policy.Default) {
		return fmt.Errorf("invalid default policy decision %q", policy.Default)
	}
	for index, rule := range policy.Rules {
		if strings.TrimSpace(rule.Capability) == "" || strings.TrimSpace(rule.Action) == "" || strings.TrimSpace(rule.Resource) == "" {
			return fmt.Errorf("rule %d requires capability, action, and explicit resource", index)
		}
		if !validDecision(rule.Decision) {
			return fmt.Errorf("rule %d has invalid policy decision %q", index, rule.Decision)
		}
		for _, value := range []string{rule.Capability, rule.Action, rule.Resource, rule.Environment, rule.Reason} {
			if strings.ContainsAny(value, "\r\n\x00") {
				return fmt.Errorf("rule %d contains invalid control characters", index)
			}
		}
		for key, value := range rule.Parameters {
			if strings.TrimSpace(key) == "" || strings.ContainsAny(key+value, "\r\n\x00") {
				return fmt.Errorf("rule %d contains invalid parameter scope", index)
			}
		}
	}
	return nil
}
