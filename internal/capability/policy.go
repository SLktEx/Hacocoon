package capability

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type PolicyRule struct {
	Capability string              `json:"capability"`
	Action     string              `json:"action"`
	Resource   string              `json:"resource,omitempty"`
	Attributes map[string]string   `json:"attributes,omitempty"`
	Decision   core.PolicyDecision `json:"decision"`
	Reason     string              `json:"reason,omitempty"`
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
		if rule.Capability != req.Capability || rule.Action != req.Action {
			continue
		}
		if rule.Resource != "" && rule.Resource != req.Resource {
			continue
		}
		if !matchAttributes(rule.Attributes, req.Attributes) {
			continue
		}
		if !validDecision(rule.Decision) {
			return core.PolicyEvaluation{}, fmt.Errorf("invalid policy decision %q", rule.Decision)
		}
		return core.PolicyEvaluation{Decision: rule.Decision, Reason: rule.Reason}, nil
	}
	decision := policy.Default
	if decision == "" {
		decision = core.PolicyDeny
	}
	if !validDecision(decision) {
		return core.PolicyEvaluation{}, fmt.Errorf("invalid default policy decision %q", decision)
	}
	return core.PolicyEvaluation{Decision: decision, Reason: "default policy"}, nil
}

func (e *FilePolicyEvaluator) load() (PolicyFile, error) {
	content, err := os.ReadFile(e.path)
	if errors.Is(err, os.ErrNotExist) {
		return PolicyFile{Default: core.PolicyDeny}, nil
	}
	if err != nil {
		return PolicyFile{}, fmt.Errorf("read policy %s: %w", filepath.Clean(e.path), err)
	}
	var policy PolicyFile
	if err := json.Unmarshal(content, &policy); err != nil {
		return PolicyFile{}, fmt.Errorf("parse policy %s: %w", filepath.Clean(e.path), err)
	}
	return policy, nil
}

func matchAttributes(required, actual map[string]string) bool {
	for key, value := range required {
		if actual[key] != value {
			return false
		}
	}
	return true
}
