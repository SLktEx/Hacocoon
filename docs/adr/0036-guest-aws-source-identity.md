# ADR 0036: Bind guest AWS requests to persisted creation identity

Status: accepted; server and product client implemented; installed guest acceptance pending
Date: 2026-09-08

## Decision

Optional operation plugins reuse the guarded Standard egress listener through
origin-form /_haco/operations/ paths. The Standard proxy owns listener lifecycle,
connection bounds and isolation; it has no AWS dependency. The AWS handler exposes
only listing and download requests, not management, approval decisions, generic
capability invocation or credential vending. Absolute proxy URLs with identical
paths remain ordinary egress requests.

The handler ignores forwarding headers and refuses caller-supplied Environment
or instance fields. Runtime source evidence must match exactly one persisted
Environment snapshot. The state store resolves the durable creation ID for that
exact snapshot under its canonical lock. Missing, ambiguous or stale evidence
fails closed. The plugin verifies that instance before AWS authentication and
carries it in the ordinary capability request through approval and execution.
A newly created Environment with the same name cannot inherit the request.

Requests are limited to 8 KiB, 32 concurrent operations and 15 minutes. Header/body
and output deadlines apply. Downloads use bounded frames and a final ordinary
execution/audit receipt. Data before that receipt is provisional. No new generic
management socket or network exception is installed in a workload.

## Rejected alternatives

- Accepting an Environment name/header lets workloads select another identity.
- Resolving only a name can adopt a later creation while preparing approval.
- Exposing the management socket gives workloads unrelated lifecycle authority.
- Treating a matching absolute URL as a local operation bypasses egress policy.
- Sharing listener admission does not replace plugin authorization.

## Evidence and remaining work

Regression tests cover persisted source ambiguity and stale/invalid identities,
recreated Environment refusal before authentication, caller identity injection,
forwarded-source spoofing, bounded download frames and external proxy URL routing.
The endpoint and automatic guest client are wired in Standard composition. The
managed companion selects the fixed endpoint; that path grants no authority.
Client frame/EOF/receipt verification precedes ordinary private file publication.
HTTP proxies and redirects are disabled. The body deadline is cleared before
waiting for approval, retaining the total operation deadline. A real HTTP socket
through ordinary queue/Policy/audit is tested with synthetic source/AWS evidence.
Installed guest/Incus acceptance and authenticated AWS remain unverified. See [AWS operations](../design/aws-operations.md).
