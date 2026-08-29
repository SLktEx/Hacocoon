# Hacocoon Security Boundary / Trust Domain Architecture

Status: Authoritative draft for implementation  
Date: 2026-08-29  
Applies to: Hacocoon v0.1-v0.7

## 1. One-sentence definition

> Hacocoon does not try to become an Access/IAM platform for people; it isolates a disposable Session from its Host, keeps parent credentials and host authority outside the Session, and grants only explicitly authorized capabilities across that boundary.

This document defines the security model that must remain valid even when Runtime, Storage, GitHub, AWS, Web IDE, or GUI implementations are scrapped and rebuilt.


## 1.1 Release staging note

The security model is normative across v0.1-v0.7, but implementation is staged: external Capability authorization begins in v0.3; AWS delegated access is v0.4; local GUI host-resource leases are v0.5; Web/interaction clients are v0.6; remote/EC2 deployment concerns are v0.7. This staging does not weaken the trust boundary in earlier releases.

## 2. Security is a boundary, not a collection of product features

Hacocoon has two different kinds of boundaries and they must not be confused.

### 2.1 Code replacement boundary

Examples:

```text
Core -> Runtime contract -> runtime.incus
Core -> Storage contract -> storage.btrfs (local profile)
Plugin registry -> plugin.aws
```

This boundary exists so code can be deleted and replaced.

It is **not automatically a security boundary**. A Go package in another directory, or a built-in plugin loaded into the Manager process, still runs with the Manager process' privileges.

### 2.2 Security / trust boundary

The real security boundaries are:

```text
Session <-> Host / Manager
Session A <-> Session B
Session <-> parent credentials
Session <-> external privileged capability
untrusted remote client <-> deployment Access Layer
one host trust domain <-> another host trust domain
```

Security architecture reviews must reason about these boundaries separately from repository/package structure.

## 3. Trust domains

### 3.1 One trust domain per host by default

The default deployment principle is:

> **1 trust domain = 1 Hacocoon host. For EC2, 1 trust domain = 1 EC2 instance by default.**

A trust domain may contain multiple disposable Sessions owned by the same trusted user or team context.

Hacocoon does not claim that mutually distrusting human users can safely share one host merely because their Sessions are different Incus instances.

For users or groups that require a different host-level trust boundary, use a separate host/EC2 instance.

### 3.2 Why this is explicit

Incus provides useful namespace/mount/device isolation, but the Hacocoon Manager, host kernel, Incus daemon, host networking, and host credentials remain privileged components.

Therefore Hacocoon is not a strong multi-tenant cloud hypervisor or a SaaS tenant isolation layer.

The product should make this deployment assumption visible rather than hide it behind a large RBAC system.

## 4. Access Layer is outside Hacocoon

Hacocoon does **not** own the user-facing Access Layer.

The deployment/user supplies or chooses mechanisms such as:

```text
TLS termination
SSO / OIDC
VPN
bastion
reverse proxy
WAF
source-IP policy
organization authentication
```

Hacocoon may expose integration hooks and consume an authenticated identity, but it does not attempt to become the identity provider, enterprise SSO server, or Internet edge security product.

### 4.1 Authentication vs authorization

Keep these distinct:

```text
Access Layer
  -> "who is connecting?"

Hacocoon Security Framework
  -> "what capability may this authenticated/local principal exercise for this Session?"
```

External authentication success must never imply GitHub/AWS/Registry capability authorization.

### 4.2 Gateway/Web UI role

A Hacocoon Gateway or Web UI may provide:

- Session routing;
- verified identity-context handoff from the deployment Access Layer;
- approval UI;
- interaction/notification UI;
- access audit.

It is not the final authority for provider capabilities. Security state in the Manager/Security Framework is authoritative.

Browser notifications are the default notification path when a Web UI exists. VS Code/code-server notification integration remains optional and must not become a security authority.

## 5. Security Framework position

Security is intentionally separated from the Microcore and from Feature Plugins.

```text
+-------------------+
|  Hacocoon Core    |
| Session lifecycle |
+---------+---------+
          |
          | contracts / identity
          v
+----------------------------+
|   Security Framework       |
| capability/profile policy  |
| ALLOW / ASK / DENY         |
| approval / grant / audit   |
+-------------+--------------+
              ^
              |
      capability requests
              |
+-------------+-------------------------------+
| Feature Plugins / privileged integrations   |
| Git / GitHub / AWS / Registry / Port / GUI  |
+---------------------------------------------+
```

The Security Framework may be omitted from a minimal local runtime installation that has no privileged external plugins. Any plugin declaring a security dependency must fail closed when the Security Framework is absent.

## 6. Capability / Profile model

Do not introduce a product subsystem named "Hacocoon IAM" that resembles AWS IAM or general human identity management. The canonical Hacocoon terms are Security Framework, Capability Profile/Policy, CapabilityRequest, Grant and Lease.

The central abstraction is the **capability granted to a Session/actor**, optionally collected into a reusable **Capability Profile**.

Examples:

```text
filesystem.workspace.rw
filesystem.inputs.ro
network.package-mirror
network.aws-build
port.local-preview
device.gpu
device.gui
git.push
github.pr.create
aws.identity.image-builder
registry.push
```

A profile can describe the intended sandbox posture:

```yaml
profile: dev-default
filesystem:
  workspace: rw
  inputs: ro
network:
  allow: [package-mirror, public-docs]
ports:
  localhost_preview: allow
external:
  git.feature_push: allow
  git.main_push: ask
  aws.identity: deny
host_devices:
  gui: deny
  gpu: deny
```

The profile is provider-neutral intent. Concrete enforcement is delegated to Modules/Plugins.

## 7. Capability realization belongs outside Security

Security decides **whether** a capability may be exercised. It should not know vendor implementation details.

Examples:

```text
Capability/Profile intent
        |
        v
Security Framework decision
        |
        +--> network module -> firewall/egress rules
        +--> runtime module -> Incus device/mount configuration
        +--> Git plugin -> brokered Git protocol
        +--> AWS plugin -> STS delegated identity
        +--> access plugin -> localhost/private route
```

This keeps the Security Framework stable even if Incus, Btrfs, AWS integration, or the Gateway implementation is replaced.

## 8. Inside Session = free; crossing the boundary = controlled

The default Hacocoon model is:

```text
Inside Session
  shell
  edit
  compile
  unit/integration test
  package install
  systemd
  nerdctl
  local DB
  IDE indexing
  coding agent
        => normally not individually authorized by Hacocoon

Boundary crossing
  git push
  GitHub mutation
  cloud identity acquisition
  registry push
  deployment
  external secret mutation
  public/private port exposure
  host filesystem/device integration
        => capability/security decision
```

Do not build a generic command firewall that attempts to understand every CLI command.

## 9. Credential invariant

### 9.1 Parent credentials stay outside the Session

The following must never be mounted or copied into a Session as a convenience mechanism:

```text
Manager HOME
host ~/.ssh
host ~/.aws
GitHub PAT / host gh token
AWS SSO cache / refresh credential
host SSH private keys
cloud parent access keys
Manager state DB
Incus administrative socket
```

### 9.2 Short-lived delegated credentials are allowed only when provider-native

The old blanket phrase "credential never enters Session" is too coarse.

The precise invariant is:

> **Long-lived and parent credentials never enter the Session. A provider may materialize a short-lived, scope-limited delegated credential when that provider has a safe native delegation model.**

Example:

```text
AWS parent identity (outside Session)
        |
        v
AWS plugin + Security decision
        |
        v
STS AssumeRole
        |
        v
short-lived role credential
        |
        v
Session process / AWS standard credential chain
```

GitHub may use brokered protocol/operations instead of raw token delegation when safe delegation is not available or administratively practical.

## 10. EC2 security model

### 10.1 Host instance authority is privileged

If Hacocoon runs on EC2, the instance profile/role belongs to the **Host trust domain**, not to Sessions.

A Session must not obtain EC2 instance credentials by reaching IMDS directly.

Required posture:

```text
Session -> cloud metadata endpoint      DENY
Session -> Manager private endpoints    DENY unless explicit IPC
Manager/plugin -> host credential source ALLOW as configured
Session -> delegated short-lived AWS identity only through approved capability
```

### 10.2 Hacocoon does not manage human AWS IAM

Hacocoon does not require users to repeatedly edit AWS IAM users/roles just to express local Session permissions.

Hacocoon Capability/Profile expresses sandbox intent. Provider-specific enforcement is mapped underneath it:

```text
Local host
  -> Incus mounts/devices
  -> firewall/network module
  -> Credential/Capability broker

AWS/provider
  -> IAM Role / STS
  -> Security Groups / network controls where relevant
  -> provider-native authorization
```

Provider-native IAM remains a last line of defense; Hacocoon policy does not replace it.

## 11. Session-to-Session isolation

Every Session receives an identity bound by the Manager/runtime, not a caller-supplied string.

Required rules:

- Session A cannot use Session B's Manager IPC endpoint;
- Session A cannot read Session B workspace/mounts by default;
- Session A cannot consume Session B grants/leases;
- Session A cannot use Session B Web/GUI routes without explicit deployment authorization;
- deleting a Session revokes/cleans its active leases, IPC endpoints, routes, and device attachments.

This is required even inside one host trust domain.

## 12. Network policy and capability authorization are separate axes

Do not merge packet reachability and authorization into one engine.

```text
Network policy
  "can a packet reach this destination?"

Hacocoon Security Framework
  "may this Session acquire/exercise this capability?"

Provider-native IAM
  "what can the resulting provider identity actually do?"
```

All three may independently deny an action.

Example: Packer may require AWS API, plugin downloads, package mirrors, and sometimes EC2 SSH/SSM. A reusable network profile may permit only those destinations, while the AWS capability separately controls which role can be acquired.

## 13. Plugin trust model

### 13.1 Plugin boundary is not automatically an isolation boundary

A built-in Go plugin/module compiled into the Manager process is trusted code from the OS point of view.

Therefore:

> Directory separation and interfaces improve scrap/rebuild safety, but do not make a malicious plugin safe.

### 13.2 Security authority remains centralized

Feature Plugins may:

- normalize provider-specific requests;
- request a capability decision;
- materialize an already authorized Grant;
- use provider-specific credential sources needed for that realization;
- return a provider result.

They may not:

- decide final `ALLOW / ASK / DENY` for themselves;
- mint authoritative Grants;
- consume another Session's Grant;
- silently bypass approval;
- fall back to mounting/exporting a parent credential.

### 13.3 Out-of-process plugins

When third-party/untrusted extension support becomes real, prefer a versioned out-of-process protocol over Go `plugin.so`.

The external process model should support:

- declared plugin identity/version;
- declared capability namespaces;
- per-plugin OS/process identity where practical;
- explicit file/socket access;
- scoped secret/materialization interfaces rather than broad Manager HOME access;
- crash/restart without corrupting Core state.

Do not build this framework in v0.1 solely for hypothetical extensibility.

## 14. System Module trust model

Runtime/Storage/Network/Host modules can affect the actual sandbox boundary and are therefore privileged implementation components.

Examples:

```text
runtime.incus
network.default
host.remote-linux
```

They are replaceable code, but they are part of the deployment trusted computing base when enabled.

Storage implementations such as Btrfs are less likely to handle external credentials, but bugs can still affect availability, isolation, or deletion semantics. They must not be treated as untrusted plugins merely because they are modular.

## 15. Approval and UI semantics

An approval is bound to the exact normalized request, including the relevant principal/resource/payload conditions.

```text
explicit DENY
  > ASK requiring exact approval
  > ALLOW
  > default DENY
```

Approval clients may include:

```text
CLI
Hacocoon Web UI
browser notification + Web UI action
optional VS Code/code-server integration
```

These clients are replaceable UX. They do not own authorization state.

## 16. Audit model

Security audit is provider-neutral and secret-free.

Record:

```text
Session/principal
capability/action/resource
normalized non-secret conditions
decision/policy rule
request/grant/lease correlation
approval actor/decision
provider/module realization result
route/device/network lease lifecycle where security-relevant
```

Never record raw tokens, secret keys, authorization headers, AWS secret access keys, or private key material.

## 17. Security profile examples

### 17.1 Local disposable development

```yaml
profile: local-dev
network:
  allow: [public-docs, package-mirror]
ports:
  localhost_preview: allow
external:
  git.feature_push: allow
  git.main_push: ask
  github.mutation: ask
  aws.identity: deny
host_devices:
  gui: deny
```

### 17.2 Image build Session

```yaml
profile: image-build
network:
  allow: [aws-build]
external:
  aws.identity.image-builder: allow
  git.push: deny
  github.mutation: deny
host_devices:
  gui: deny
```

The AWS plugin realizes the role; the Network module realizes reachability. Security only evaluates the declared capability/profile.

## 18. Failure policy

Security uncertainty fails closed.

Examples:

```text
unknown plugin namespace          -> DENY / unsupported
cannot bind Session identity      -> DENY
approval payload changed          -> DENY
provider credential materialize failed -> fail; no parent credential fallback
network profile missing           -> no implicit Internet widening
remote authenticated identity missing when required -> access rejected by deployment layer
```

## 19. Security acceptance invariants

These invariants apply when the corresponding components exist:

1. Session cannot read Manager HOME, host `~/.ssh`, host `~/.aws`, or Manager state.
2. Session cannot obtain EC2 metadata credentials.
3. Session A cannot use Session B IPC/grants/routes.
4. Feature Plugin cannot turn an `ASK`/`DENY` into execution without Security authorization.
5. Approval is exact and cannot be reused after payload/resource mutation.
6. Parent credentials are not logged or stored in generic Grant/Audit state.
7. Removing `plugin.aws` removes AWS capability without changing Core lifecycle code.
8. Replacing `runtime.incus` does not require rewriting Git/AWS security policy semantics.
9. External authentication success never automatically grants provider capabilities.
10. Different mutually distrusting human trust domains are deployed on separate hosts/EC2 instances by default.

## 20. Final security invariants

> **Core describes the Session. Modules realize infrastructure. Security decides capability authority. Plugins perform an authorized feature. The Access Layer authenticates/reaches Hacocoon but does not own capability authority.**

And:

> **Hacocoon protects the Host/Manager and external authority from an untrusted Session. It does not pretend that one shared host is a strong isolation boundary between mutually distrusting human tenants.**
