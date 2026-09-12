# ADR 0025: Bind saved Environment decisions to a creation identity

Status: accepted; identity binding and ordinary saved Git CLI implemented; external acceptance partial.

## Decision

A reusable Environment name is not an approval identity. Canonical creation now
allocates a random 128-bit `env-...` instance ID before reserving its Workspace
lease. The lease stores this ID before provider creation and preserves it through
runtime recording and ready commit. Lifecycle transitions reject replacement.

The catalog exposes an identity resolver for an exact ready Environment snapshot.
It checks metadata and the active lease under the catalog lock. For a legacy
aggregate with no instance ID, a clearly marked migration assigns one random ID
and persists it once under that lock. Missing, stale or inconsistent aggregates
fail closed. This does not recreate or change provider resources.

Capability requests and audit events can carry `environment_instance` alongside
the human-facing Environment name. Saved Environment-specific rules match that
exact instance; name-only legacy saved rules do not match identified requests.
Explicit global choices omit the instance condition. Administrator rules retain
their existing name scopes and may explicitly constrain an instance.

The production Git broker resolves this identity from trusted catalog state and
checks it again before its prepared operation executes. The general client
request payload cannot assert an instance ID; approval payloads preserve the
trusted identifier for display. The production Capability service also resolves every named request from the catalog before Policy evaluation and rechecks the exact snapshot immediately before provider execution. Environment-scoped saving requires a valid creation identity; unidentified requests offer only one-shot and explicit global choices.

## Rejected alternatives and limits

- Lease Owner is currently a name label, not a random creation identity.
- Names, provider references and timestamps can be reused.
- Requesting an instance ID as a new mandatory CLI argument burdens users and
  mistakes caller-supplied identity for proof.
- An unknown legacy identity must not be inferred from an old saved name grant.

The identity is not a credential or a provider ownership token. It does not by
itself make a request authorized. Git commit/ref binding, Policy, audit and
canonical cleanup remain authoritative. Reusable Git scope and CLI saving are
implemented under [ADR 0026](0026-reusable-git-approval-scope.md); notifications
and real network/provider acceptance remain unfinished.

Identity resolution reads persisted lease evidence directly. The general legacy state reader can synthesize missing leases; that compatibility result is not accepted as proof for identity assignment.
