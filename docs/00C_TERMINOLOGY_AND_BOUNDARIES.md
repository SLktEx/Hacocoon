# Hacocoon Canonical Terminology and Boundary Glossary

Status: Authoritative naming guide for v0.1-v0.7
Date: 2026-08-29

This file exists to prevent historical terms from silently re-entering the design. When another current document uses a conflicting name, this glossary wins for terminology; the version specification still wins for release scope.

## Canonical architecture names

| Canonical term | Meaning | Do not use as synonym |
|---|---|---|
| **Core** | Hacocoon-generic Session concepts, lifecycle orchestration, composition and generic state | Incus wrapper, Btrfs manager |
| **System Module** | replaceable infrastructure implementation such as Runtime/Storage/Host/Access | security sandbox |
| **Security Framework** | authoritative Session capability authorization | Hacocoon IAM |
| **Capability Profile / Capability Policy** | reusable posture/rules evaluated by Security Framework | human IAM/RBAC |
| **CapabilityRequest** | normalized request for a cross-boundary capability | raw argv |
| **Grant / Lease** | exact authorization / runtime realization with scope and lifecycle | credential as the primary model |
| **Feature Plugin** | optional provider/tool/product realization such as Git/GitHub/AWS/IDE | authorization authority |
| **Capability Broker** | optional v0.4 routing/materialization mechanism after/beside Security authorization | Security Framework |
| **Access Layer** | deployment-owned authentication/Internet-edge controls such as TLS/SSO/VPN/WAF | Hacocoon Security Framework |
| **Gateway/access adapter** | Hacocoon routing adapter that consumes verified identity context and Session access decisions | identity provider / Internet edge |
| **Interaction API** | client-neutral events/actions for approval/notification UX | authorization state machine |

## Runtime and storage names

Canonical implementation IDs for the current plan:

```text
runtime.incus
storage.btrfs
block.local-qcow2   # private implementation seam under local storage
block.local-raw     # private implementation seam under local storage
host.local-wsl
host.remote-linux
runtime.ec2         # v0.7
```

`storage.ebs` is **not frozen as a cross-version contract**. EC2/EBS attachment/provisioning order differs enough from the v0.1 Incus path that v0.7 has an explicit design gate. EBS-specific code must remain outside Core, but its exact module boundary is chosen only after the concrete EC2 composition proves the right seam.

## Security naming rules

Use:

```text
Security Framework
Capability Profile
Capability Policy evaluator
ALLOW / ASK / DENY
Approval
Grant / Lease
```

Do not introduce a Hacocoon component called `Hacocoon IAM`. `AWS IAM` remains the correct name for AWS's provider-side permission system.

## Authority rule

```text
Access Layer: who is connecting?
Security Framework: what capability/access may this principal exercise?
Plugin/Module: how is the approved capability realized?
Provider-native IAM/API: what does the external provider ultimately permit?
```

A UI, Gateway, plugin or provider adapter never becomes authoritative merely because it can execute the operation.
