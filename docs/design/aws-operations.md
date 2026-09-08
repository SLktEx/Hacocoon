# Approved AWS operations

[日本語](aws-operations.ja.md) | English

Status: **partial D3 implementation**. Trusted Host S3 listing and streamed downloads are implemented.
ID-bound Host account labels are implemented. Guest server transport and ordinary guest CLI are implemented; installed guest
and real AWS/desktop acceptance remain pending. This does not reintroduce the deferred
EC2 runtime.

## Ordinary use

In the trusted Host, after configuring its optional AWS integration:

```sh
haco aws s3 ls s3://example-bucket/project/
```

A single Environment is inferred. Use `--env dev` when several exist.
`--profile` defaults to `default`; `--region` defaults to that Host profile.
Options precede the S3 URL. The listing command returns a JSON array of exact
object keys and sizes. Unicode is escaped for safe terminal display without
changing the decoded keys. An oversized or incomplete listing fails rather than
returning a partial listing as success; narrow the prefix when necessary.

If Policy requires approval, use the existing notification/details route or
`haco approve` in another trusted terminal. Existing Environment/global allow,
deny and ask choices use the same `haco config`, audit and request ID. Saving ask
does not decide the current operation. A missing UI never permits execution.
Environment creation identity is captured before AWS authentication and rechecked
by the capability service. Credentials never go to the Environment.

## Optional Host preparation

AWS use requires AWS CLI v2 at `/usr/local/bin/aws` and `botocore` importable by
the Host's `/usr/bin/python3`. These are optional integration dependencies; ordinary
Hacocoon setup and Core do not require them. Install AWS CLI v2 using the
[official installer](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html)
and the Host distribution's `python3-botocore` package. Authenticate as the trusted
Host user whose home is `/root`, for example with
`aws configure sso --profile default` followed by
`aws sso login --profile default`. Configure the profile's ordinary AWS region
as well as its SSO region. Authentication and operation permission are separate.

Existing Policy is not silently changed. A default-deny configuration must have
an explicit require-approval rule for the intended operation before it can prompt.
For example, add this rule through `haco config`, replacing the example target and
region. Wildcard identity fields here require review for every identity; a saved
decision retains the actual identity, not these wildcards.

```json
{
  "capability": "aws.s3",
  "action": "ListObjectsV2",
  "resource": "arn:aws:s3:::example-bucket",
  "environment": "*",
  "attributes": {
    "account": "*",
    "account_name": "unavailable",
    "principal": "*",
    "profile": "default",
    "region": "ap-northeast-1",
    "service": "s3",
    "description": "List object names and sizes",
    "iam_action": "s3:ListBucket",
    "bucket": "example-bucket",
    "bucket_owner": "*",
    "prefix": "project/"
  },
  "decision": "require-approval"
}
```

An explicit require-approval rule continues to require review even after an allow
is saved: restrictive-rule precedence remains unchanged. Remove or narrow that
explicit rule when intentionally relying on the saved Policy, or use an existing
require-approval default. See [Policy semantics](policy-and-capability-foundation.md)
and [configuration editing](../reference/configuration.md).

## Bound execution

The plugin uses the Host's AWS CLI credential resolver, including supported SSO
or credential-process profiles. Resolved credentials remain in Host process
memory. A single resolved credential set performs STS identity verification and
the approved S3 operation; identity is checked again after waiting for approval.
The Physical Host controller receives only account ID, principal ARN, region and
operation results. Account name is either explicitly unavailable or an operator
label configured in Host and bound to the actual account ID; it is not an AWS
identity assertion and is never inferred from the profile name.

The review includes Environment, account, principal, profile, region, S3 bucket
ARN, prefix, API action and IAM action. ListObjectsV2 requires
[s3:ListBucket](https://docs.aws.amazon.com/AmazonS3/latest/API/API_ListObjectsV2.html).
The initial adapter supports ordinary commercial-partition buckets owned by the
executing account and sets ExpectedBucketOwner on every page. Directory buckets,
access-point aliases, other partitions and cross-account buckets are not enabled.
It never vends a credential or invokes an arbitrary AWS CLI command.

SDK signing scope and HTTPS destination are checked before sending. Region
redirects and implicit HeadBucket discovery are refused, including a redirect
that keeps the URL but changes the signing region. List pagination preserves
bucket, owner and prefix; repeated tokens or response limits fail closed. Each
Host invocation has a process-group lifetime limit. No raw credentials, SDK
errors or subprocess output enter the audit log.

A Hacocoon denial is not executed. AWS AccessDenied is an executed provider failure,
distinct from Hacocoon permission; other AWS failures are not successful empty
listings. See [ADR 0034](../adr/0034-aws-operation-authentication.md).

## Verification scope

Repository checks cover input/response validation, shared saved Policy and
revocation, controller review receipts, Host ownership and SDK signing/redirect
behavior using synthetic credentials with intercepted HTTP transport.
`HACO_AWS_TEST_PYTHON=/path/to/venv/bin/python bash tools/ci-local.sh aws` runs the
SDK checks; the maintained GHA aws-plugin job prepares its isolated SDK dependency.

Real AWS, installed Host authentication, SSO renewal, and actual native/VS Code
AWS review have not run. They are separate acceptance requirements, not inferred
from the intercepted SDK or existing Git/network desktop checks.

Dedicated WSL Hacocoon-Review-6771f2f verified the current Host adapter, transient
unit and fail-closed not-configured response. The owned Host has no AWS CLI,
botocore or AWS config; real AWS was SKIP for these missing prerequisites. No
AWS operation or login was attempted. This is not authenticated AWS acceptance.

## Download an object

On the Linux/WSL client in the trusted Host:

```sh
haco aws s3 cp s3://example-bucket/project/config.json ./config.json
```

The optional env/profile/region flags behave like listing and precede the URL.
The destination is a local file on the machine running that client. GetObject
requires a separate Policy scope: API action `GetObject`, IAM action
`s3:GetObject`, the complete object ARN as resource and a `key` attribute in place
of `prefix`. The description is `Download current object at execution`; other
identity attributes are unchanged. ListBucket permission does not imply download
permission. The current version at execution is fetched in one request; there is
no automatic archive restore, upload, range reconstruction or SSE-C key input.

Data is streamed in 64 KiB frames, with no small whole-object JSON limit.
ContentLength, final byte count/SHA-256, complete transport, execution success and
completed audit must all agree before publication. Hashes verify transfer
consistency, not a user-supplied expected object version. Binary and empty objects
are supported. Keys containing dot path segments are refused rather than risking
path normalization to a different object.

The client keeps provisional data in a private directory beside the destination.
Only after verification does it atomically replace an existing regular file;
symlinks and other file types are refused. The resulting file is private to the
client user. Renaming the parent or staging directory cannot redirect publication.
Failure or cancellation before publication leaves the previous file intact.
Cleanup identity drift is reported and unexpected contents are retained, never
recursively removed. Filesystems that cannot enforce private permissions fail
closed; native Windows filesystem acceptance remains separate.

See [ADR 0035](../adr/0035-streamed-aws-downloads.md). Repository verification
includes 20 MiB through the actual controller wire and the ordinary review,
saved-policy and revocation path. Eleven intercepted real-SDK tests cover listing
and downloads. Authenticated S3 downloads remain SKIP because the dedicated Host
has no AWS CLI, botocore or AWS config; those tests have not become real AWS proof.

The current verified Host streaming adapter also transferred 20 MiB successfully
in dedicated WSL without contacting AWS. Maintained local CI, the eleven SDK
tests and documentation checks passed. This does not verify authenticated S3.


## Account names in review

Optionally add a readable label and its expected account ID to the existing
trusted Host AWS profile at /root/.aws/config:

```ini
[profile development]
region = ap-northeast-1
haco_account_id = 123456789012
haco_account_name = Development
```

For the default profile, use [default]. Keep existing authentication settings.
Then use the ordinary --profile development option. The displayed name is a
Host operator label, not an account name discovered from AWS. No IAM lookup or
additional credential permission is required. The actual STS account ID and
principal remain visible and determine the execution identity.

Both name and expected ID must be configured together. A mismatch with the STS
account refuses the operation before S3 access. Labels are re-read at execution;
a label changed while waiting requires a new request. The label is part of the
saved scope, so changing it also invalidates matching old saved permissions.
An explicit rule matching account_name = unavailable must be adjusted through
ordinary haco config when adding a label; permission is never silently widened.

The file must be a regular file owned by the Host user, not writable by group
or others, and at most 1 MiB; symlinks are refused. Invalid or duplicate INI,
control/format characters and labels longer than 256 UTF-8 bytes fail closed.
Profiles with neither field still display unavailable. Labels do not come from
the Environment or its repository.

Focused race tests and fifteen intercepted SDK/config tests passed, including
profile isolation, identity mismatch, label changes and unsafe config. Actual
authenticated AWS and desktop label rendering remain unverified for the same
missing Host AWS prerequisites.

## Guest request boundary

The Standard listener now exposes an optional AWS-only origin-form endpoint at
/_haco/operations/aws. It accepts list/get, URL, profile and region; it does not
accept Environment identities or management/approval methods. Trusted runtime
source evidence and the exact persisted Environment creation ID select the caller.
The plugin rejects recreation before authentication and retains that ID through
ordinary approval and execution. Forwarding headers do not select the source.

The normal guest haco client is now connected. Installed guest acceptance remains
pending. The client publishes downloads only after the final verified receipt. See [ADR 0036](../adr/0036-guest-aws-source-identity.md).


## Use AWS inside an Environment

Standard Environment creation and start now install the ordinary haco entry point
from the verified guest companion. Use the same commands inside the Environment:

```sh
haco aws s3 ls s3://example-bucket/project/
haco aws s3 cp s3://example-bucket/project/config.json ./config.json
```

The caller Environment is automatic; --env is refused inside it. Profile and
region options retain their existing meaning in the trusted Host. Approval is
reviewed from the trusted Host/desktop, not granted by the guest. Credentials and
management sockets remain absent from this route.

The managed guest executable selects the fixed guarded endpoint automatically.
This executable path is client routing, not proof of caller identity: the server
still derives identity from runtime/state evidence. HTTP proxy environment
variables and redirects cannot replace the destination. An existing unrelated
/usr/local/bin/haco is refused instead of overwritten.

Downloads retain private staging, atomic verified publication and preservation
of existing files on incomplete transfer. Guest HTTP checks require EOF after
the receipt, bounded frames, byte count/hash and successful execution/audit.
The short body deadline is cleared after input is complete so it cannot cancel
a valid human approval wait; the 15-minute operation limit remains.

Local tests passed the actual HTTP socket through the ordinary approval queue,
saved Policy, revocation and audit using synthetic source/AWS evidence, plus
redirect/truncation/receipt refusal and guest setup idempotence/conflict refusal.
The first setup test failed because the new link path was not isolated into its
temporary root; after fixing that fixture all focused tests passed. These are
not real Incus guest or authenticated AWS acceptance. Installed guest E2E remains
pending; real AWS remains SKIP for absent Host authentication/dependencies.

## Installed guest acceptance

Dedicated WSL Hacocoon-Review-6771f2f passed guest acceptance with haco/controller
built from 093ed159b80e: ordinary user/API creation of m1-egress-093ed15020260908,
automatic haco companion link, refusal of --env, source-bound controller/Host
refusal of a unique unconfigured AWS profile, failed-download file preservation
and canonical deletion. Provider inventory and the temporary Workspace were
confirmed absent afterward; controller remained active. This was a local binary
update, not a fresh Windows installer run. Previous binaries remain in the
root-only /root/hacocoon-validation-093ed15 backup on that dedicated WSL.

The maintained installed-egress-check now exercises these cases in the existing
Windows installer GHA. Its check-aws mode uses the same create/delete path without
requiring an unrelated external-network allow rule. Local CI and acceptance
verifier regressions passed. The initial new test source had a string-literal
newline error; it was fixed before the successful run. New-head GHA is pending.

Authenticated AWS remains SKIP: Host authentication and optional dependencies are
absent. Negative unconfigured-profile acceptance does not prove S3 success,
positive guest file download, SSO renewal or native desktop AWS decisions.
