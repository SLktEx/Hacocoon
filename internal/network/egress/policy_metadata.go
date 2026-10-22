package egress

import (
	"github.com/SLktEx/Hacocoon/internal/core"
	capability "github.com/SLktEx/Hacocoon/internal/policy"
)

// PolicyDescriptor describes request authority without preparing a connection.
func (Provider) PolicyDescriptor() capability.CapabilityDescriptor {
	return capability.CapabilityDescriptor{
		Capability: Capability,
		Actions: []capability.ActionDescriptor{{
			Action:      ActionConnect,
			Description: "Authorize one HTTP or HTTPS proxy connection.",
			Resource: capability.ValueDescriptor{
				Description: "Canonical lowercase DNS hostname without a trailing dot; IP literals are rejected.",
			},
			Attributes: []capability.AttributeDescriptor{
				{Name: "port", Description: "Destination port as a canonical decimal integer from 1 to 65535."},
				{Name: "protocol", Description: "Proxy connection protocol.", Values: []string{string(core.EgressHTTP), string(core.EgressHTTPS)}},
			},
			Notes: []string{
				"The HTTP/HTTPS proxy uses trusted Host DNS after authorization; it does not request a separate network.resolve approval.",
				"A grant is connection-scoped, not reusable IP authority; destination and HTTPS SNI checks still apply.",
			},
		}},
	}
}
