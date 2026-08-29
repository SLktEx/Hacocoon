# Capability boundary

Hacocoon v0.4 mediates privileged operations on the host side. An Environment or client requests an operation; it does not receive ambient parent credentials.

```text
CapabilityRequest
  -> PolicyEvaluator
     -> allow
     -> deny
     -> require-approval -> human approval
  -> CapabilityProvider
  -> audit
```

## Policy file

The local CLI reads `$HACO_ROOT/policy.json`. Missing policy defaults to `deny`. Malformed policy fails closed.

```json
{
  "default": "deny",
  "rules": [
    {
      "capability": "local.echo",
      "action": "echo",
      "resource": "safe",
      "decision": "allow"
    },
    {
      "capability": "local.echo",
      "action": "echo",
      "resource": "sensitive",
      "decision": "require-approval",
      "reason": "human confirmation required"
    }
  ]
}
```

Rules are evaluated in order. A rule with no `resource` matches any resource for that capability/action pair.

## Requesting a capability

`local.echo` is an intentionally harmless proof provider for v0.4.

```bash
haco capability request local.echo echo \
  --resource safe \
  --param message=hello
```

Parameters are provider inputs and are intentionally excluded from capability audit events. Do not use CLI parameters as a transport for long-lived credentials; provider credentials remain host-side.

## Approval

A `require-approval` policy decision prompts on the CLI and defaults to deny:

```text
Approve capability local.echo action=echo resource=sensitive? [y/N]
```

Approval is for security-sensitive authority. It is not code-review, task, PR, or merge approval.

## Audit

Events are appended to `$HACO_ROOT/audit/capabilities.jsonl` with private file permissions. Events record request metadata, policy decisions, approval outcomes, and completion status, but not request parameters or provider output.

GitHub-specific authority is introduced in v0.5; AWS/EC2 remains v0.7.
