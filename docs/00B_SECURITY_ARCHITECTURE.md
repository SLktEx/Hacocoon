# Security Architecture

Status: authoritative cross-cutting security baseline.

## Trust model

Commands, developer tools, and coding agents executed inside a Hacocoon Environment are **untrusted with respect to host authority**.

The host-side Hacocoon control path owns environment lifecycle, policy decisions, capability mediation, and privileged credentials. Convenience must not collapse that boundary.

```text
untrusted workspace process
          |
          | request operation
          v
     Hacocoon boundary
      /      |       \
 policy   approval   capability
      \      |       /
       privileged host/service action
```

## v0.1 security baseline

v0.1 must at minimum:

- mount only the requested workspace rather than the host HOME;
- avoid mounting `~/.ssh`, `~/.aws`, GitHub tokens, Incus control sockets, or Hacocoon state into the Environment;
- make cleanup explicit and testable;
- keep Incus lifecycle authority outside the Environment;
- report command exit status without silently elevating privileges.

v0.1 does **not** need the full Policy/Capability engine.

## Capability model (v0.4+)

Privileged external operations are represented as requests rather than ambient credentials.

```text
CapabilityRequest
    -> PolicyEvaluator
       -> allow
       -> deny
       -> require-approval
```

`require-approval` is a first-class policy result, not a UI afterthought.

## Human-in-the-loop split

Hacocoon owns **security approval**:

- credential issuance;
- GitHub push or other privileged repository operation;
- AWS/API privilege use;
- sensitive port exposure;
- runtime privilege changes;
- policy exceptions.

Development approval belongs above Hacocoon: task scope, code review, PR acceptance, and merge decisions can be handled by a human, GitHub, Daintree, Rookery, or another orchestrator.

## Credentials

Long-lived parent credentials should not become files or environment variables broadly readable by the executed agent.

Where possible, later capability adapters should use provider-native short-lived credentials or a brokered operation with narrow scope, short lifetime, and audit records.

## Fail closed

Failure to evaluate policy, obtain required approval, or acquire a scoped credential must deny the privileged operation. Cleanup failures must be surfaced rather than hidden.
