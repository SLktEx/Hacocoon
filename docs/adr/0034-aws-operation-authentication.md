# ADR 0034: Keep AWS authentication inside the trusted Host

Status: accepted; partial S3 listing implementation
Date: 2026-09-08

## Decision

AWS operation code belongs in an optional capability plugin. The Physical Host
controller owns Policy, request identity, approval and audit; the logical Host
resolves AWS credentials and executes the specific approved operation. Environment
workloads receive neither parent credentials nor a management socket.

Preparation authenticates only to discover the actual STS account/principal and
region. Waiting for approval does not hold reusable credentials in the controller.
Execution resolves a new credential set inside Host, verifies its identity equals
the reviewed identity, and uses that same frozen set for S3. A profile label is
not account identity. Changed account/principal requires a new request.

All authority-bearing fields are visible in the capability scope. S3 listing
binds API action, IAM action, account, principal, profile, region, bucket ARN,
expected bucket owner and prefix. Saved decisions and manual edits use the
existing Policy evaluator, including restrictive precedence and Environment
creation identity. No plugin-specific allow database exists.

The SDK supplies SigV4 signing. The adapter checks the context's signing region,
the final Authorization credential scope, and the HTTPS destination before
sending. S3 redirects can change signing region while retaining a custom endpoint;
checking only the URL or the before-sign event argument is insufficient. The
regression exercises this with the real SDK and intercepted transport. Implicit
HeadBucket discovery is not an approved ListObjectsV2 operation and is refused.

The Host adapter accepts only shipped program text from composition and opaque
request stdin. Ownership is checked before execution. A transient systemd unit
bounds the whole process group; credential subprocesses and output are bounded.
No credentials travel in argv, response JSON, logs or temporary credential files.
AWS authentication remains distinct from granting Hacocoon operation permission.

## Rejected alternatives

- Running arbitrary AWS CLI arguments would hide or expand authority.
- Comparing STS identity and then resolving credentials again for S3 creates a
  profile/credential race.
- Exporting resolved credentials to the controller or guest widens the trust boundary.
- Trusting endpoint configuration alone misses SDK signing-region redirection.
- Treating AWS denial as an empty successful listing loses the operation result.

## Remaining scope

Object download is now implemented through [ADR 0035](0035-streamed-aws-downloads.md).
Optional Host profile labels are now bound to an explicit expected account ID,
verified against STS before review and again at execution. Labels are not identity
authority; their changes invalidate the reviewed/saved scope. No untrusted request
can supply a new label. Guest-scoped requests and additional actual-use operations
are future slices. The first slice supports
same-account ordinary commercial S3 buckets and bounded complete listings.
Real AWS/SSO and installed desktop acceptance remain separate from repository
and intercepted-SDK validation. See [AWS operations](../design/aws-operations.md).
