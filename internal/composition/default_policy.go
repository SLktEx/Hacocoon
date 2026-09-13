package composition

import (
	"strconv"

	capabilityapp "github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
	egressapp "github.com/SLktEx/Hacocoon/internal/egress"
)

const packageRepositoryBaselineReason = "official Ubuntu package repository"

func defaultPolicyBaseline() []capabilityapp.PolicyRule {
	endpoints := egressapp.DefaultPackageRepositoryEndpoints()
	rules := make([]capabilityapp.PolicyRule, 0, len(endpoints))
	for _, endpoint := range endpoints {
		rules = append(rules, capabilityapp.PolicyRule{
			Capability:  egressapp.Capability,
			Action:      egressapp.ActionConnect,
			Resource:    endpoint.Host,
			Environment: "*",
			Attributes: map[string]string{
				"protocol": string(endpoint.Protocol),
				"port":     strconv.Itoa(endpoint.Port),
			},
			Decision: core.PolicyAllow,
			Reason:   packageRepositoryBaselineReason,
		})
	}
	return rules
}
