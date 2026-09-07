package capability

import (
	"github.com/SLktEx/Hacocoon/internal/core"
	"testing"
)

func TestSavedChoicesPreserveObservedAuthority(t *testing.T) {
	request := core.CapabilityRequest{Capability: "local.echo", Action: "echo", Resource: "target", Environment: "dev", Attributes: map[string]string{"branch": "main"}, Parameters: map[string]string{"secret": "never-store"}}
	for _, choice := range []SavedChoice{AllowEnvironment, DenyEnvironment, AllowGlobal, DenyGlobal} {
		rule, err := RuleForSavedChoice(request, choice)
		if err != nil {
			t.Fatal(err)
		}
		wantEnv := "dev"
		if choice == AllowGlobal || choice == DenyGlobal {
			wantEnv = "*"
		}
		wantDecision := core.PolicyAllow
		if choice == DenyEnvironment || choice == DenyGlobal {
			wantDecision = core.PolicyDeny
		}
		if rule.Environment != wantEnv || rule.Decision != wantDecision || rule.Resource != "target" || rule.Attributes["branch"] != "main" {
			t.Fatalf("authority changed: %#v", rule)
		}
		rule.Attributes["branch"] = "changed"
		if request.Attributes["branch"] != "main" {
			t.Fatal("request map shared")
		}
	}
}
func TestSavedChoiceRejectsImplicitOrWildcardAuthority(t *testing.T) {
	for _, request := range []core.CapabilityRequest{
		{Capability: "local.echo", Action: "echo", Resource: "*", Environment: "dev"},
		{Capability: "local.echo", Action: "echo", Resource: "target"},
		{Capability: "local.echo", Action: "echo", Resource: "target", Environment: "*"},
		{Capability: "local.echo", Action: "echo", Resource: "target", Environment: "dev", Attributes: map[string]string{"branch": "*"}},
	} {
		if _, err := RuleForSavedChoice(request, AllowEnvironment); err == nil {
			t.Fatalf("broad authority accepted: %#v", request)
		}
	}
}
