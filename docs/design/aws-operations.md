# Approved AWS operations

[日本語](aws-operations.ja.md) | English

Status: **partial D3 implementation**. Trusted Host S3 listing is implemented.
Object download, guest-scoped transport, friendly account-name configuration and
real AWS/desktop acceptance remain planned. This does not reintroduce the deferred
EC2 runtime.

## Ordinary use

In the trusted Host, after configuring its optional AWS integration:

```sh
haco aws s3 ls s3://example-bucket/project/
```

A single Environment is inferred. Use `--env dev` when several exist.
`--profile` defaults to `default`; `--region` defaults to that Host profile.
Options precede the S3 URL. The initial command returns a JSON array of exact
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
operation results. Account name is explicitly unavailable, not inferred from a
profile label or guessed.

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
