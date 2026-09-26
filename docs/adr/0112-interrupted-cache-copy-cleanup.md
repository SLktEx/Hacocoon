# ADR 0112: Clean interrupted cache collection through recorded operations

Status: accepted. [日本語](0112-interrupted-cache-copy-cleanup.ja.md)

## Decision

Record the exact Incus asynchronous operation on the reserved collection candidate
before waiting. On failure, atomically fence completion, positively observe the
saved operation terminate, then enter common exact-owner deletion. Keep CopySource
until the provider positively proves destination absence and catalog finalization
succeeds. A persisted deleting state proves the stop check already passed.
Existing cache recover retries these transitions after process exit.

Incus volume copies may not support cancellation. A client timeout, missing volume,
missing operation, malformed reply or resource existence does not prove stop or
completion. Missing/expired operation receipts remain recovery-required. A tracked
copy that completed without a durable completion receipt is discarded during
failure cleanup; it is never adopted from existence alone. Completed-copy receipts
and ambiguous generation-selection writes retain their existing recovery semantics.

Scope is stopped ordinary-Env cache collection. Generic resource copies, OCI,
creation-time cache placement and lifecycle authority remain unchanged. No guest
management credentials, Host paths or file-by-file merging are introduced.

## Rejected alternatives

Deleting immediately on CLI failure races a still-running daemon copy. Releasing
the source before destination absence permits concurrent writers and deletion.
Reconstructing resource removal outside the shared deletion transition bypasses
ownership and generation-selection fences. Retaining every tracked failure forever
prevents ordinary Env reuse despite available positive terminal evidence.
