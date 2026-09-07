# v0.4 — Policy & Capability Foundation

Status: **roadmap contract implemented on `main`.** The fail-closed policy/approval/audit boundary exists; Hacocoon remains pre-1.0 and concrete policy/capability schemas may still change incompatibly.

## Goal

Replace ambient privileged credentials with an explicit host-side capability boundary.

## Core concepts

```text
CapabilityRequest
PolicyDecision = allow | deny | require-approval
ApprovalRequest
CapabilityResult = request identity + execution state + audit state
```

## Fail-closed policy contract

Policy JSON is decoded strictly. Unknown fields, malformed rules, invalid decisions, and rules without an explicit `resource` are rejected.

Wildcards are never inferred from omission. Use `"*"` explicitly when broad authority is intentional.

```json
{
  "default": "deny",
  "rules": [
    {
      "capability": "example.capability",
      "action": "operate",
      "resource": "example://resource",
      "environment": "demo",
      "attributes": {
        "target": "exact-target",
        "revision": "*"
      },
      "decision": "require-approval",
      "reason": "interactive example"
    }
  ]
}
```

`resource:"*"` matches any resource. `environment:"*"` matches any environment. An omitted environment matches only a request whose environment is also empty.

Policy attributes describe authority-sensitive request inputs. Every request attribute must be represented by the matching rule; a rule value may be exact or the explicit `"*"` wildcard. Extra request attributes therefore fail to match instead of silently bypassing policy review.

Opaque `Parameters` are allowed only when the selected provider explicitly declares each key as non-authority data. Any input that can change authority, scope, target, credential selection, or security meaning must be represented in `Resource`, `Environment`, or `Attributes` instead.

## Human-in-the-loop

Hacocoon human approval exists for **security-sensitive authority**, not ordinary code review. The approval prompt includes capability, action, resource, environment, policy reason, and sorted policy-visible attributes. If the prompt cannot be displayed, approval fails closed.

Examples:

- issue a short-lived credential;
- expose a sensitive port;
- perform a privileged host operation;
- cross a network/service boundary that policy marks as approval-required.

## Audit and retry semantics

Every accepted capability request receives a unique request ID before its first audit event. All events for that operation carry the same ID so concurrent requests can be correlated.

A successful provider call and the final audit write are separate outcomes. `CapabilityResult` records the execution state and whether audit completion succeeded. If the provider succeeded but final audit recording failed, Hacocoon returns `ErrAuditIncomplete` with `execution_state=succeeded` and `audit_complete=false`. Callers must reconcile that request ID instead of blindly retrying a non-idempotent operation.

Audit directories and files are tightened to private permissions before security events are appended (`0700` directory, `0600` file).

## Provider registration

Capability names are unique identities. Duplicate or invalid provider names fail service construction instead of using last-registration-wins behavior.

## In scope

- Capability-provider interface.
- Policy evaluation boundary.
- `allow`, `deny`, and `require-approval` outcomes.
- CLI human approval provider.
- Audit/event record for privileged requests and decisions.
- Credential lifetime/scope abstraction where needed.
- Dummy/local capability used to prove the flow before provider-specific behavior.

## Not in scope for the v0.4 gate

- Full GitHub integration (introduced by v0.5).
- Full AWS/EC2 integration (introduced by v0.7).
- Agent task approval or merge approval.

## Compatibility note

The fail-closed security invariants matter more than preserving an accidental pre-1.0 schema. Policy, request, audit, or capability formats may break when needed to close bypasses or clarify authority; such changes must remain explicit and auditable.

## Matching-rule precedence

All matching explicit rules participate: `deny` > `require-approval` > `allow`.
Rule ordering never bypasses a stronger restriction. The default applies only
when no explicit rule matches. This replaces historical first-match behavior.
See [ADR 0023](../adr/0023-policy-restriction-precedence.md).

## Saved decisions

Status: **storage, service, controller stream and terminal component implemented;
ordinary Git saved decisions implemented; notification integration pending**.
The optional `saved_decisions` array uses the same rule shape, allow, deny or require-approval. It participates alongside `rules` without replacing administrator rules.
Persistent allow, deny and ask choices have explicit Environment or global scope.
Persistence keeps every attribute name. A trusted provider may explicitly wildcard
changing values in the displayed saved scope; opaque parameters are never stored.
See [ADR 0024](../adr/0024-saved-approval-decisions.md) for durability, audit failure
and manual editing constraints. The capability stream advertises saved-choice support before
a client may send a persistent decision. Unsupported peers cannot silently
downgrade it to one-shot approval. The terminal component offers y/N plus the
six explicitly labeled scope/decision combinations. Saving ask collects a separate
y/N answer for this request and keeps later requests subject to approval. The
ordinary Git queue supports optional approve/deny --save; notifications remain pending.

The maintained Capability E2E now drives the terminal/controller saved choice,
replays it without a prompt in the same Environment, requires approval in another
Environment, preserves administrator rules and checks audit parameter redaction.
This covers the shared approval path. Ordinary Git queue tests also use real local
Git and a bare remote. See [ADR 0026](../adr/0026-reusable-git-approval-scope.md)
for reusable scope, persistence receipts and fixed execution.

Every request is reevaluated immediately before provider execution, including one-shot approvals and initially allowed requests. A new deny, unreadable Policy, or newly required approval blocks execution. An in-flight request that already obtained explicit approval may proceed if current Policy still requires approval. This is a boundary recheck, not a transaction with arbitrary manual editors; already-established connections are not revoked by this change.

Saved Environment-specific decisions require and bind to a trusted creation identity. The production service resolves it from the catalog for every named request before Policy evaluation and rechecks the exact snapshot before execution. Unidentified requests expose only one-shot and explicit global choices. A saved name-only rule does not match an identified request; explicit global scope remains global. The Git broker obtains and rechecks the ID from canonical state. General client request payloads cannot assert it. See [ADR 0025](../adr/0025-environment-approval-identity.md); ordinary saved Git scope/UI and real network/provider acceptance remain partial. Repository CLI/controller E2E verifies that replacing the catalog creation identity for the same name requires a new approval; this fixture does not create a real provider resource.

## Trusted configuration editing

Status: **implemented repository slice; installed acceptance pending**.
`haco config` inspects the same rules used by saved approval and evaluation.
`--edit` or `--file` replaces a reviewed, revision-bound snapshot through the
trusted controller. Configuration editing and approval saving share the canonical
private atomic writer; stale snapshots fail instead of erasing another change.
See [configuration usage](../reference/configuration.md) and
[ADR 0027](../adr/0027-revision-bound-policy-editing.md). This adds no Environment
permission or notification mutation endpoint. D2 notification approval remains
pending.
