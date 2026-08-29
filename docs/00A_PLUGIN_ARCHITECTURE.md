# Hacocoon Microcore / Module / Security / Plugin Architecture Specification

Status: Authoritative draft for implementation  
Date: 2026-08-29  
Applies to: Hacocoon v0.1-v0.7  
CLI: `haco`

## 1. One-sentence definition

> Hacocoon keeps Core limited to product concepts, lifecycle orchestration, contracts, and composition; concrete infrastructure is implemented as replaceable System Modules, security authority is isolated in a dedicated Security Framework, and user-facing/provider-specific capabilities are implemented as Feature Plugins.

The architecture is optimized not only for extension but for **deletion and replacement**.

A feature boundary is considered good when its implementation can be removed or replaced without a broad Core rewrite.

Security boundaries are defined separately in `00B_SECURITY_ARCHITECTURE.md`. **A module/plugin boundary is not automatically a trust boundary.** In particular, built-in Manager-side modules/plugins are trusted code even when physically separated into different packages.

## 2. Why this architecture exists

Hacocoon is expected to be repeatedly scrapped and rebuilt while its product shape is still evolving.

The main architectural risk is therefore not only "how do we add features?" but also:

- how do we delete a storage strategy without touching Git/AWS code;
- how do we replace Incus interaction without rewriting security;
- how do we remove a Web IDE without changing Session lifecycle;
- how do we replace the Security Framework/policy implementation without changing Git normalization;
- how do we experiment with a new host/network implementation without contaminating Core;
- how do we keep old experimental code from silently becoming a permanent dependency.

The governing rule is:

> **Optimize for replaceability and deletion before optimizing for abstraction completeness.**

A useful review question for every new component is:

> If this entire feature is deleted six months from now, which directory should be removable?

If the answer is "many Core packages must be rewritten", the boundary is probably wrong.

## 3. The four architectural classes

Hacocoon uses four classes of code.

```text
+--------------------------------------------------------------+
|                         Hacocoon Core                         |
| Session model / lifecycle orchestration / contracts / state  |
| module composition / plugin routing                          |
+---------------------------+----------------------------------+
                            |
                            | contracts
                            v
          +-----------------+-----------------+
          |                                   |
+---------v-----------+             +---------v-----------+
|   System Modules    |             |  Security Framework |
| runtime / storage   |             | policy / grant      |
| network / host/...  |             | approval / audit    |
+---------+-----------+             +---------+-----------+
          |                                   ^
          |                                   |
          +----------------+------------------+
                           |
                           v
                 +---------+----------+
                 |   Feature Plugins  |
                 | Git / AWS / GitHub |
                 | VS Code / Codex... |
                 +--------------------+
```

### 3.1 Core

Core owns only Hacocoon-generic concepts and orchestration.

Core does **not** own concrete Incus, Btrfs, AWS, GitHub, VS Code, WSLg, or Codex behavior.

### 3.2 System Modules

System Modules implement replaceable infrastructure slots used to construct a Session or its host environment.

Examples:

```text
runtime.incus
storage.btrfs
network.default
host.local-wsl
host.remote-linux
access.ssh
```

System Modules are generally privileged and may affect isolation or host resources.

### 3.3 Security Framework

Security is neither ordinary Core business logic nor a normal Feature Plugin.

The Security Framework is about Session capability authorization, not human authentication. TLS/SSO/VPN/WAF and other deployment Access Layer concerns remain outside Hacocoon. The normative trust-domain and credential rules are in `00B_SECURITY_ARCHITECTURE.md`.

It is a dedicated authority responsible for:

```text
Principal
CapabilityRequest
Policy
ALLOW / ASK / DENY
Approval
Grant / Lease
Audit authority
```

Feature Plugins request authority from Security; they do not define authority themselves.

### 3.4 Feature Plugins

Feature Plugins add optional product capabilities.

Examples:

```text
plugin.git
plugin.github
plugin.aws
plugin.registry
plugin.codex
plugin.vscode
plugin.webide.code-server
plugin.gui.wslg
plugin.ide.intellij
```

A Feature Plugin should be removable without changing the meaning of a Session.

## 4. Core boundary

### 4.1 Core owns

Core owns the smallest stable Hacocoon concepts:

- `SessionID`;
- `SessionSpec`;
- desired Session lifecycle state;
- generic Session metadata;
- lifecycle orchestration;
- composition of selected modules;
- registration/routing of plugins;
- generic configuration loading;
- minimal state persistence needed to recover orchestration;
- generic error/correlation identifiers;
- contract version compatibility.

Core may say:

```text
create this Session using the selected Runtime
attach the selected Storage
start the Session
invoke plugin X for capability namespace Y
clean up all resources associated with Session Z
```

Core must not know how those concrete operations are performed.

### 4.2 Core does not own

The following must not become Core implementation details:

```text
Incus CLI/API calls
Incus project/profile syntax
Btrfs loop image creation
mkfs.btrfs / mount / losetup
ZFS/EBS/filesystem-specific logic
Git transport
GitHub REST/GraphQL calls
AWS STS / credential formats
OCI registry authentication
VS Code command-line behavior
code-server lifecycle details
WSLg socket paths
Wayland/X11 details
Codex-specific command flags
IntelliJ installation/startup details
cloud metadata firewall rules
```

### 4.3 Core must remain useful with fake implementations

A Core lifecycle test must be able to run with in-memory/fake contracts and without:

- Incus;
- Btrfs;
- GitHub;
- AWS;
- VS Code;
- WSLg.

If a Core unit test requires one of those products to exist, the dependency boundary should be reviewed.

## 5. Contracts package

Contracts are the stable seam between Core and replaceable implementations.

Recommended repository shape:

```text
contracts/
  runtime/
  storage/
  security/
  network/
  host/
  access/
  capability/
  agent/
```

Contracts contain:

- small interfaces;
- provider-neutral request/response types;
- capability namespaces;
- typed lifecycle results;
- version information;
- error categories.

Contracts do not contain concrete provider clients.

Avoid giant interfaces. A contract should be introduced from an actual use case, not from hypothetical completeness.

## 6. Runtime Module

### 6.1 Purpose

The Runtime Module realizes a Hacocoon Session as a concrete system environment.

Initial implementation:

```text
modules/runtime-incus
```

This makes the distribution "Hacocoon + Incus" without making Incus a permanent Core dependency.

### 6.2 Conceptual contract

```go
type Runtime interface {
    Probe(ctx context.Context) (RuntimeCapabilities, error)
    Create(ctx context.Context, spec RuntimeSessionSpec) (RuntimeSession, error)
    Start(ctx context.Context, id RuntimeSessionID) error
    Stop(ctx context.Context, id RuntimeSessionID) error
    Delete(ctx context.Context, id RuntimeSessionID) error
    Exec(ctx context.Context, id RuntimeSessionID, cmd ExecRequest) (ExecResult, error)
    Inspect(ctx context.Context, id RuntimeSessionID) (RuntimeState, error)
}
```

The exact Go shape may evolve. The semantic boundary is normative; the literal interface is illustrative.

### 6.3 Incus-specific code location

Incus operations belong under the Incus module, for example:

```text
modules/runtime-incus/
  client.go
  instance.go
  profile.go
  project.go
  devices.go
  errors.go
  contract_test.go
```

No unrelated Core package should import the Incus implementation package.

## 7. Storage Module

### 7.1 Btrfs and local block images are explicitly not Core

Btrfs is a shipping Storage implementation, not a Hacocoon concept. The initial implementation is:

```text
modules/storage-btrfs
```

For the supported local WSL profile, `storage.btrfs` may compose a **private implementation seam** for its backing image:

```text
block.local-qcow2   # preferred when host probe proves NBD/QEMU support
block.local-raw     # sparse raw/loop fallback
```

This inner block-image seam exists because QCOW2 and sparse raw are real local alternatives. It is not a second Core-facing storage API. Do not generalize it to every future cloud volume until a real second composition proves that boundary useful.

`storage.btrfs` owns Btrfs formatting/mount/resize/health semantics. The local block implementation owns image creation, attachment, outer-capacity resize and backend-specific verification.

### 7.2 Why Storage is separate from Runtime

Do not hide Btrfs implementation inside `runtime-incus` merely because v0.1 ships them together.

The separation exists so future experiments can use combinations such as:

```text
runtime.incus + storage.btrfs
runtime.incus + storage.zfs
runtime.incus + storage.native-existing-pool
runtime.future + storage.future
```

Not every combination must be supported. The point is that replacing storage must not require modifying Git/AWS/Core code.

### 7.3 Conceptual contract

```go
type Storage interface {
    Probe(ctx context.Context) (StorageCapabilities, error)
    Ensure(ctx context.Context, spec StorageSpec) (StorageHandle, error)
    Inspect(ctx context.Context, handle StorageHandle) (StorageState, error)
    Snapshot(ctx context.Context, req SnapshotRequest) (SnapshotRef, error)
    Restore(ctx context.Context, req RestoreRequest) error
    Clone(ctx context.Context, req CloneRequest) (StorageHandle, error)
    Delete(ctx context.Context, handle StorageHandle) error
}
```

Only methods actually needed by the current release should be implemented.

### 7.4 Runtime/Storage coordination

Core coordinates references; it does not translate Btrfs commands.

Example:

```text
Core
  |
  +-- Storage.Ensure(session storage spec)
  |       -> StorageHandle
  |
  +-- Runtime.Create(session spec + StorageHandle)
```

The Runtime implementation may consume a provider-neutral handle or a declared implementation-specific opaque payload negotiated through capability metadata.

Avoid Core parsing provider-specific storage fields.

## 8. Security Framework

### 8.1 Position

Security is deliberately outside the minimal runtime Core.

A minimal local Hacocoon installation may conceptually run:

```text
Core + Runtime + Storage
```

without external privileged Feature Plugins.

When a plugin can create external side effects, access privileged host resources, expose ports, or acquire identity, the configured Security Framework becomes mandatory for that operation.

### 8.2 Security owns authority

Security owns:

```text
Principal resolution
CapabilityRequest evaluation
policy matching
ALLOW / ASK / DENY
Approval state
Grant creation
Lease expiry/revoke
security audit truth
```

Security may be implemented in a separate package/service/module boundary so it can itself be rebuilt, but it is a **privileged framework slot**, not an ordinary plugin.

### 8.3 Security does not implement providers

Security must not contain:

```text
Git branch parsing
AWS STS client details
GitHub PR API payload construction
OCI auth flow
VS Code launching
WSLg socket attachment
```

Plugins normalize provider-specific meaning into generic capability requests.

### 8.4 Plugin/Security flow

Canonical flow:

```text
Session / user
     |
     v
Feature Plugin
  normalize provider-specific request
     |
     v
CapabilityRequest
     |
     v
Security Framework
  ALLOW / ASK / DENY
     |
     v
Grant / Lease
     |
     v
Feature Plugin
  realize exactly the approved capability
```

Forbidden flow:

```text
Feature Plugin
  decides allowed=true
  performs side effect
  sends audit event afterward
```

## 9. Feature Plugin model

### 9.1 Git Plugin

The Git Plugin owns Git-specific behavior:

- clone/fetch/push transport;
- repository/ref normalization;
- source SHA, force/delete interpretation;
- Manager-side credential integration needed to perform Git transport.

It does not own authorization.

```text
git push
   -> Git Plugin
   -> CapabilityRequest(git.push, repo/ref/SHA/...)
   -> Security
   -> Grant
   -> Git Plugin
   -> remote Git host
```

Deleting `plugins/git` must not require modifying AWS, Storage, or Core lifecycle logic.

### 9.2 GitHub Plugin

The GitHub Plugin owns safe GitHub operation normalization and execution.

Examples:

```text
github.pr.create
github.issue.create
github.workflow.run
```

Raw parent tokens are not returned to the Session.

### 9.3 AWS Plugin

The AWS Plugin owns:

- AWS identity/capability mapping;
- STS/AssumeRole integration;
- provider-native temporary credential materialization;
- AWS-specific expiry/refresh behavior;
- optional AWS network-profile hints.

Security decides whether the capability may be acquired. AWS IAM remains the final provider-side authorization boundary.

Packer/Terraform/AWS CLI/SDK are consumers of the AWS capability and are not separate Core features.

### 9.4 Access and UX plugins

VS Code, Web IDE, GUI, and Agent integrations should remain separate feature packages unless they are proven to require a shared System Module contract.

Examples:

```text
plugins/vscode
plugins/webide-code-server
plugins/codex
plugins/intellij
plugins/gui-wslg
```

This makes it possible to delete an IDE integration without changing runtime lifecycle.

## 10. Module versus Plugin

The distinction is semantic.

| Class | Meaning | Cardinality | Typical authority |
|---|---|---:|---|
| System Module | replaceable infrastructure slot | usually one selected implementation per slot | high |
| Security Framework | shared authorization authority | one configured authority | highest for capability decisions |
| Feature Plugin | optional product capability | zero or many | scoped |

Examples:

```text
Runtime = System Module
Storage = System Module
Security = Framework
Git = Feature Plugin
AWS = Feature Plugin
VS Code = Feature Plugin
```

Do not call every package a plugin simply because it has an interface.

## 11. Composition root

Concrete implementations are wired together only at a narrow bootstrap/composition boundary.

Recommended shape:

```text
cmd/haco/
  main.go
  bootstrap.go

internal/composition/
  defaults.go
  registry.go
```

Conceptual composition:

```text
Core
  runtime  = runtime-incus
  storage  = storage-btrfs
  security = security-default

plugins:
  git      = enabled
  github   = optional
  aws      = optional
  vscode   = optional
```

Core packages must not import the composition package.

## 12. Repository layout

Recommended starting layout:

```text
hacocoon/
├─ cmd/
│  └─ haco/
│
├─ core/
│  ├─ session/
│  ├─ lifecycle/
│  ├─ state/
│  ├─ config/
│  └─ registry/
│
├─ contracts/
│  ├─ runtime/
│  ├─ storage/
│  ├─ security/
│  ├─ capability/
│  ├─ network/
│  ├─ host/
│  ├─ access/
│  └─ agent/
│
├─ modules/
│  ├─ runtime-incus/
│  ├─ storage-btrfs/
│  ├─ security-default/
│  ├─ network-default/
│  ├─ host-local-wsl/
│  └─ host-remote-linux/
│
├─ plugins/
│  ├─ git/
│  ├─ github/
│  ├─ aws/
│  ├─ registry/
│  ├─ codex/
│  ├─ vscode/
│  ├─ webide-code-server/
│  ├─ gui-wslg/
│  └─ intellij/
│
└─ internal/
   └─ composition/
```

This is a dependency layout, not a requirement to create empty directories for every future feature.

Create a directory when the implementation begins.

## 13. Dependency rules

### 13.1 Allowed dependency direction

```text
Core ---------> Contracts
Modules ------> Contracts
Security -----> Contracts
Plugins ------> Contracts
Composition --> Core + Modules + Security + Plugins
```

Feature Plugins may use public Core services and Security contracts through explicit facades, but must not import concrete sibling plugin implementations.

### 13.2 Forbidden dependencies

Examples:

```text
core/session -> modules/storage-btrfs
core/lifecycle -> plugins/aws
plugins/aws -> plugins/git
plugins/vscode -> modules/runtime-incus internals
modules/storage-btrfs -> core private state DB
security-default -> plugins/github concrete client
```

### 13.3 Cross-plugin collaboration

If GitHub needs Git functionality, use a stable capability/service contract rather than importing `plugins/git` internals.

Prefer:

```text
GitHub Plugin -> GitService contract -> selected Git implementation
```

not:

```text
GitHub Plugin -> import plugins/git/internal/...
```

## 14. Trust and process boundaries

Architectural modularity does not automatically create security isolation.

Initial releases may compile trusted built-in Modules/Plugins into one binary for implementation simplicity.

Do not use Go `plugin.so` as the public extension model.

Reasons:

- loaded code has Manager process authority;
- Go plugin compatibility is fragile;
- crashes/panics affect Manager;
- it creates a false impression that the plugin is isolated.

When third-party or less-trusted plugins are supported, prefer:

```text
Manager
   |
   | versioned IPC/RPC
   v
separate plugin process
```

The external-process protocol is a later implementation concern and must not block v0.1.

## 15. Deletion-first rules

Every Module/Plugin should have an explicit removal story.

### 15.1 Directory deletion test

A feature is well bounded when most of its implementation can be removed by deleting one directory and removing one composition registration.

Examples:

```text
rm -rf modules/storage-btrfs
rm -rf plugins/aws
rm -rf plugins/webide-code-server
```

Additional expected changes:

- remove configuration schema specific to the feature;
- remove composition registration;
- remove feature-specific acceptance tests;
- optionally remove an unused contract if no implementation remains.

Broad Core lifecycle rewrites should not be required.

### 15.2 No shared junk drawer

Do not move provider-specific code into generic folders such as:

```text
utils/
common/
helpers/
manager_misc/
```

merely to avoid an apparent dependency.

Shared code must have a clear provider-neutral semantic purpose.

### 15.3 Second implementation rule

Do not over-generalize a contract for imagined future implementations.

But when a subsystem is intentionally expected to be replaced during early development, a narrow contract may be introduced before a second implementation **specifically to enforce deletion boundaries**.

Runtime and Storage qualify because they are high-churn infrastructure choices.

## 16. State ownership

State must be divided by authority.

### Core state

Examples:

```text
Session ID
selected module IDs
runtime resource reference
storage resource reference
desired lifecycle state
created_at / updated_at
```

### Security state

Examples:

```text
policy version
approval request
Grant / Lease
revocation / expiry
security audit correlation
```

### Module/Plugin state

Only implementation-specific opaque state that cannot be represented generically.

Prefer storing a stable reference rather than serializing provider secrets or entire provider objects into Core state.

## 17. Configuration ownership

Generic configuration chooses implementations:

```yaml
runtime: incus
storage: btrfs
security: default
plugins:
  git: enabled
  aws: disabled
```

Implementation-specific configuration belongs to the selected implementation namespace:

```yaml
modules:
  runtime-incus:
    project: hacocoon
  storage-btrfs:
    mode: loop
    size: 64GiB
```

Core must not grow top-level fields such as:

```text
btrfs_loop_size
aws_role_arn
wslg_socket_path
```

## 18. Failure semantics

### Core

Core handles orchestration failure and cleanup ordering.

### Runtime/Storage Module

Failures must return typed errors and enough resource references for cleanup/reconciliation.

### Security

Security-sensitive uncertainty fails closed.

### Feature Plugin

A plugin failure must not silently widen authority or fall back to raw parent credentials.

Example:

```text
AWS temporary credential materialization failed
-> fail request
-> do not mount ~/.aws as fallback
```

## 19. Testing strategy

### 19.1 Core tests

Use fakes for Runtime, Storage, Security, and plugin registry.

Test:

- lifecycle orchestration;
- cleanup ordering;
- state transitions;
- module selection;
- failure reconciliation.

### 19.2 Module contract tests

Each System Module gets a contract suite.

Examples:

```text
runtime-incus contract tests
storage-btrfs contract tests
```

### 19.3 Security tests

Security gets independent tests for:

- ALLOW / ASK / DENY;
- exact approval binding;
- expiry/revoke;
- explicit deny precedence;
- no-secret audit.

### 19.4 Plugin tests

Plugins test provider-specific normalization and realization separately from policy evaluation.

## 20. v0.1-v0.7 rollout

The architecture is introduced incrementally.

### v0.1 Local Foundation

Ship the minimal Core plus real local infrastructure boundaries:

```text
Core
contracts/runtime
contracts/storage
(private local block-image seam; not Core-facing)
modules/runtime-incus
modules/storage-btrfs
modules/block-local-qcow2 and/or modules/block-local-raw
```

Security and external Feature Plugins are not required for v0.1. Incus/Btrfs/QCOW2 are implementations, not Core concepts.

### v0.2 Developer Workspace

Add workspace and convenience integrations behind boundaries:

```text
plugins/codex
plugins/vscode
access.ssh
localhost port access
workspace/repository packages
```

### v0.3 Security Framework + Git

Introduce the dedicated Security Framework and first privileged external Feature Plugin:

```text
security.default
plugin.git
```

This proves `Plugin -> Security -> Grant -> Plugin`.

### v0.4 External Capabilities

Generalize capability plugins:

```text
plugin.github
plugin.aws
plugin.registry
```

AWS delegated identity is implemented here even though EC2 runtime/deployment waits until v0.7.

### v0.5 Local GUI + IDE

Add replaceable local host integration:

```text
access/gui contract
plugin.gui.wslg
IDE profile: IntelliJ
optional GPU/audio leases
```

### v0.6 Local Web + Interaction

Add local web clients without changing authorization authority:

```text
interaction API
Hacocoon Web UI
Browser Notification
approval UI
plugin.webide.code-server
```

VS Code/code-server notification integration remains optional; Web UI + Browser Notification is the standard notification path when the web client is running.

### v0.7 Remote + EC2

Add remote implementations behind existing seams:

```text
host.remote-linux
runtime.ec2
runtime.ec2 EBS lifecycle component (exact extraction boundary decided in v0.7)
remote Gateway/access integration
remote Web IDE/preview routing
```

Remote/EC2 code must not become a prerequisite for the local v0.1-v0.6 composition.

## 21. Naming conventions

Suggested IDs:

```text
runtime.incus
storage.btrfs
block.local-qcow2
block.local-raw
runtime.ec2
security.default
network.default
host.local-wsl
host.remote-linux

plugin.git
plugin.github
plugin.aws
plugin.registry
plugin.codex
plugin.vscode
plugin.webide.code-server
plugin.gui.wslg
plugin.ide.intellij
```

Names describe implementation identity, not security authority.

## 22. Architecture decision checklist

Before adding code, ask:

1. Is this a stable Hacocoon concept? If yes, consider Core/Contracts.
2. Is this replaceable infrastructure used to construct the environment? Use a System Module.
3. Is this authorization authority shared by privileged capabilities? Use Security Framework.
4. Is this an optional external/product/tool capability? Use a Feature Plugin.
5. Can the feature be deleted mostly by deleting one directory and one registration?
6. Does Core import any vendor-specific package because of this change?
7. Does a Feature Plugin make its own final authorization decision?
8. Does the implementation require exposing a parent credential as a shortcut?
9. Are two plugins directly importing each other's internals?
10. Is an abstraction being added without a real deletion/replacement reason?

Any `yes` to 6-9 requires redesign before merge.

## 23. Non-goals

This architecture does not require:

- every module to be dynamically loaded;
- every module to be untrusted;
- hot reload;
- a marketplace in early versions;
- a universal plugin SDK in v0.1;
- every internal Go package to implement an interface;
- arbitrary combinations of every Runtime and Storage implementation;
- microservices for their own sake.

The primary objective is clean replacement boundaries inside a still-small OSS codebase.

## 24. Implementation principle for Codex

When refactoring historical v0.1, do not rewrite everything into a plugin framework.

Perform dependency extraction in this order:

```text
1. identify concrete Incus calls
2. move them into modules/runtime-incus
3. make Core depend on the Runtime contract
4. identify Btrfs/loop/mount logic
5. move it into modules/storage-btrfs
6. make orchestration depend on Storage contract
7. preserve working behavior with contract/integration tests
8. leave future Git/AWS/GUI abstractions unimplemented until their version
```

The target is **less coupling**, not more framework code.

## 25. Final invariant

> Core should describe **what a Hacocoon Session is and how its lifecycle is orchestrated**. System Modules decide **how infrastructure realizes it**. Security decides **whether privileged capabilities are allowed**. Feature Plugins decide **how an allowed feature is performed**.

And the practical deletion rule is:

> If replacing Btrfs, AWS, GitHub, VS Code, or WSLg requires widespread Core changes, Hacocoon has violated its architecture boundary.

The corresponding security rule is:

> If a Session can cross into Host authority, another Session, a parent credential, or an external privileged capability merely because a module/plugin was swapped, Hacocoon has violated its security boundary.
