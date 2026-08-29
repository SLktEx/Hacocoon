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
