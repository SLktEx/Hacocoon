package aws

import capability "github.com/SLktEx/Hacocoon/internal/policy"

// PolicyDescriptor reads no Host authentication or live AWS identity.
func (*Provider) PolicyDescriptor() capability.CapabilityDescriptor {
	return capability.CapabilityDescriptor{
		Capability: Capability,
		Actions: []capability.ActionDescriptor{
			{
				Action: GetAction, Description: "Download the current object at execution with verified transfer completion.",
				Resource: capability.ValueDescriptor{Description: "Complete object ARN: arn:aws:s3:::<bucket>/<key>."},
				Attributes: s3PolicyAttributes("Download current object at execution", "s3:GetObject", capability.AttributeDescriptor{
					Name: "key", Description: "Exact nonempty UTF-8 object key, at most 1024 bytes; '.' and '..' path segments are rejected.",
				}),
				Notes: append(s3PolicyNotes(), "Download permission is separate from listing; the request does not pin an object version."),
			},
			{
				Action: ListAction, Description: "List object names and sizes under an exact prefix.",
				Resource: capability.ValueDescriptor{Description: "Bucket ARN: arn:aws:s3:::<bucket>."},
				Attributes: s3PolicyAttributes("List object names and sizes", "s3:ListBucket", capability.AttributeDescriptor{
					Name: "prefix", Description: "Exact UTF-8 key prefix, at most 1024 bytes; empty means the whole bucket, not a glob.",
				}),
				Notes: append(s3PolicyNotes(), "Listing is bounded and fails rather than reporting an oversized or incomplete result as success."),
			},
		},
	}
}

func s3PolicyAttributes(description, iamAction string, object capability.AttributeDescriptor) []capability.AttributeDescriptor {
	return []capability.AttributeDescriptor{
		{Name: "account", Description: "Verified 12-digit AWS account ID."},
		{Name: "account_name", Description: "Trusted Host label bound to the account ID, or the literal 'unavailable' when no label is configured."},
		{Name: "bucket", Description: "Exact ordinary S3 bucket name: 3 to 63 lowercase letters, digits, dots or hyphens; starts and ends with a letter or digit, with no consecutive dots."},
		{Name: "bucket_owner", Description: "Expected 12-digit bucket owner account ID, equal to account."},
		{Name: "description", Description: "Exact operation description included in the request.", Values: []string{description}},
		{Name: "iam_action", Description: "AWS IAM action required for this operation.", Values: []string{iamAction}},
		object,
		{Name: "principal", Description: "Verified arn:aws IAM user/role or STS assumed-role ARN for account; at most 256 bytes."},
		{Name: "profile", Description: "Trusted Host AWS profile: 1 to 64 letters, digits, underscores, dots or hyphens, starting with a letter or digit; defaults to 'default'."},
		{Name: "region", Description: "Resolved ordinary commercial AWS region, fixed before approval."},
		{Name: "service", Description: "AWS service name.", Values: []string{"s3"}},
	}
}

func s3PolicyNotes() []string {
	return []string{
		"This optional plugin's presence does not imply configured Host credentials, AWS IAM permission or Hacocoon Policy permission.",
		"Only ordinary commercial-partition buckets owned by account are supported; directory buckets, access-point aliases and cross-account buckets are excluded.",
		"Host authentication is rechecked after approval; credentials are never included in discovery or sent to the Environment.",
		"Key and prefix values reject control/format characters, surrounding whitespace and the literal '*'; Policy wildcard matching is separate.",
	}
}
