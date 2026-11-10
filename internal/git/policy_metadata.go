package gitrepo

import (
	"github.com/SLktEx/Hacocoon/internal/adapters/git"
	capability "github.com/SLktEx/Hacocoon/internal/policy"
)

// PolicyDescriptor is independent of registered repositories and prepared work.
func (*Broker) PolicyDescriptor() capability.CapabilityDescriptor {
	fetch := gitPolicyAttributes()
	fetch = append(fetch, capability.AttributeDescriptor{
		Name: "target_ref", Description: "Exact refs/heads/ branch ref, or the literal " + gitadapter.AllHeadsRef + " discovery scope.",
	})
	push := gitPolicyAttributes()
	push = append(push,
		capability.AttributeDescriptor{Name: "target_ref", Description: "Exact refs/heads/ branch ref being created or fast-forwarded."},
		capability.AttributeDescriptor{Name: "update_kind", Description: "Prepared branch update type.", Values: []string{"create", "fast-forward"}},
	)
	return capability.CapabilityDescriptor{
		Capability: Capability,
		Actions: []capability.ActionDescriptor{
			{
				Action: "fetch", Description: "Read branch refs or commits, including observation for push preparation.",
				Resource:   capability.ValueDescriptor{Description: "Registered credential-free remote URL: github.com HTTPS or a trusted Host file URL."},
				Attributes: fetch,
				Notes: append(gitPolicyNotes(),
					"Branch discovery also checks each exact advertised ref; fetch permission never grants push permission.",
					"Object IDs may be empty for discovery; fetch and push preparation bind the available exact object IDs.",
				),
			},
			{
				Action: "push", Description: "Create one branch or fast-forward one existing branch with fixed content.",
				Resource:   capability.ValueDescriptor{Description: "Registered credential-free remote URL: github.com HTTPS or a trusted Host file URL."},
				Attributes: push,
				Notes: append(gitPolicyNotes(),
					"old_oid is 40 zeroes for branch creation; new_oid is a nonzero exact commit ID. Force, deletion and multi-ref pushes are unsupported.",
				),
			},
		},
	}
}

func gitPolicyAttributes() []capability.AttributeDescriptor {
	return []capability.AttributeDescriptor{
		{Name: "new_oid", Description: "Prepared new object ID in 40 lowercase hexadecimal characters, or empty for read-only discovery."},
		{Name: "old_oid", Description: "Prepared previous object ID in 40 lowercase hexadecimal characters, or empty for read-only discovery/fetch."},
		{Name: "operation_id", Description: "Opaque identifier of the exact operation prepared inside the trusted Git broker."},
		{Name: "remote", Description: "Exact registered remote URL, equal to the resource."},
		{Name: "repository", Description: "Registered repository identifier: 1 to 48 lowercase letters, digits or hyphens, starting with a letter or digit."},
	}
}

func gitPolicyNotes() []string {
	return []string{
		"Only an exact context-bound operation prepared inside the Git broker can execute; a matching Policy rule cannot submit arbitrary Git work.",
		"Saved Git approval scopes widen only operation_id, old_oid and new_oid to '*'; repository, remote, target_ref and push update_kind remain exact.",
	}
}
