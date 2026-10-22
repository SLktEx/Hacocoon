package dnsproxy

import (
	"github.com/SLktEx/Hacocoon/internal/network/dns"
	capability "github.com/SLktEx/Hacocoon/internal/policy"
)

// PolicyDescriptor never consults the Environment catalog or a DNS resolver.
func (Provider) PolicyDescriptor() capability.CapabilityDescriptor {
	return capability.CapabilityDescriptor{
		Capability: nameresolution.Capability,
		Actions: []capability.ActionDescriptor{{
			Action:      nameresolution.Action,
			Description: "Resolve a hostname using the source Environment's configured resolver.",
			Resource: capability.ValueDescriptor{
				Description: "Canonical lowercase DNS hostname without a trailing dot; IP literals are rejected.",
			},
			Attributes: []capability.AttributeDescriptor{},
			Notes: []string{
				"There are no request attributes. Resolution returns addresses, not connection permission.",
				"The current Environment selects Host, backend or disabled DNS; listing this capability does not imply DNS is enabled.",
			},
		}},
	}
}
