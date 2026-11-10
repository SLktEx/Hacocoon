package networkrelay

import capability "github.com/SLktEx/Hacocoon/internal/policy"

// PolicyDescriptor describes canonical relay authority without resolving targets.
func (Provider) PolicyDescriptor() capability.CapabilityDescriptor {
	return capability.CapabilityDescriptor{
		Capability: Capability,
		Actions: []capability.ActionDescriptor{{
			Action:      Action,
			Description: "Authorize one bounded TCP connection or UDP association.",
			Resource: capability.ValueDescriptor{
				Description: "External canonical hostname or numeric address, registered Host service name, or destination Environment name.",
			},
			Attributes: []capability.AttributeDescriptor{
				{Name: "addresses", Description: "Sorted, comma-separated canonical pinned IP addresses; 1 to 32 unique unmapped addresses."},
				{Name: "destination_instance", Description: "Empty for external targets; exact Host service registration identity or destination Environment creation identity otherwise."},
				{Name: "destination_kind", Description: "Destination authority type.", Values: []string{"environment", "external", "host"}},
				{Name: "duration_seconds", Description: "Canonical decimal maximum lifetime: 1 to 3600 seconds for TCP, 1 to 300 seconds for UDP."},
				{Name: "port", Description: "Resolved destination port as a canonical decimal integer from 1 to 65535."},
				{Name: "protocol", Description: "Transport protocol.", Values: []string{"tcp", "udp"}},
			},
			Notes: []string{
				"External hostnames require separate network.resolve authorization before the pinned-address connect request.",
				"Host services require administrator registration; Environment targets bind the destination creation identity.",
				"Policy, source and destination checks continue during the session; authorization does not grant application credentials or management access.",
			},
		}},
	}
}
