# Hacocoon v0.1-v0.7 Rebaseline and Roadmap

Status: Authoritative draft for implementation  
Date: 2026-08-29  
CLI: `haco`  
Primary first target: Windows 11 + WSL2 + Ubuntu 26.04 + Incus

## 1. Why this rebaseline exists

Hacocoon accumulated valid ideas in an order that no longer matched implementation dependencies. The new order optimizes for a working local product first, then adds remote/EC2 deployment only after the local model is stable.

```text
v0.1  Local Foundation
       ↓
v0.2  Developer Workspace
       ↓
v0.3  Security Framework + Git
       ↓
v0.4  External Capabilities
       ↓
v0.5  Local GUI + IDE
       ↓
v0.6  Local Web + Interaction
       ↓
==============================
local product is feature-complete enough to validate
==============================
       ↓
v0.7  Remote + EC2
```

The architecture underneath every release is:

```text
Core        = stable product concepts + lifecycle orchestration
Modules     = replaceable infrastructure implementations
Security    = privileged authorization authority
Plugins     = optional provider/product/tool integrations
```

## 2. Critical split: AWS access vs EC2 deployment

These are independent:

```text
v0.4 AWS capability
Session -> Security -> short-lived delegated AWS identity -> AWS APIs

v0.7 EC2 deployment/runtime
Hacocoon -> runtime.ec2 / host.remote-linux / storage.ebs -> remote infrastructure
```

Therefore AWS CLI, SDKs, Packer and Terraform integration remain v0.4. EC2 lifecycle, AMI/EBS, remote Gateway deployment and cloud-host operations are v0.7.

## 3. Product definition

Hacocoon is an OSS disposable Linux development workspace runtime for humans and coding agents.

```text
Existing client / IDE / agent
          |
          v
+-----------------------------------------------+
| Hacocoon Session [UNTRUSTED]                  |
| full Linux userspace                          |
| systemd                                       |
| containerd / nerdctl                          |
| build tools / DB / language runtimes          |
| coding agent                                  |
| workspace                                     |
+----------------------+------------------------+
                       |
                       | explicit boundary
                       v
+---------------------------------------------------------------+
| Hacocoon Manager / Controller [TRUSTED]                       |
| Core + Modules + Security + Feature Plugins                   |
+---------------------------+-----------------------------------+
                            |
                            v
                 GitHub / AWS / Registry / Host
```

Hacocoon does not implement a new editor, container runtime, package manager, cloud provider, filesystem, or enterprise identity platform.

## 4. Global invariants

### G-1 Session is a disposable machine
A Session may run systemd services, package managers, build tools, nested nerdctl workloads, local databases, agents and IDE backends.

### G-2 Manager/host is trusted; Session is untrusted
Repository content, dependencies, setup scripts, agents and IDE extensions inside the Session can be hostile.

### G-3 Inside Session = free development
Do not approve ordinary local filesystem/process/build operations one command at a time.

### G-4 Boundary effects = explicit policy
External writes, privileged identities, host devices, public exposure and cross-boundary access are policy-controlled.

### G-5 Parent credentials stay outside
No host SSH private keys, GitHub parent token, AWS parent credential/SSO cache, Manager HOME or Incus control socket enters the Session.

### G-6 Tiny vendor-neutral Core
Core must not import concrete Incus, Btrfs, QCOW2/QEMU, EBS, AWS, GitHub, VS Code, code-server, WSLg or IntelliJ implementation packages.

### G-7 Runtime and storage are replaceable
Initial local composition uses Incus plus local Btrfs-backed managed storage. v0.7 may add EC2/EBS without changing Session semantics.

### G-8 Security is separate authority
Policy, ALLOW/ASK/DENY, Approval, Grant/Lease and authoritative security audit belong to Security Framework.

### G-9 UI is never authorization authority
CLI, Web UI, Browser Notification and optional IDE extensions display/submit decisions; Manager/Security state is authoritative.

### G-10 Prefer standard interfaces
OpenSSH, Git transport, AWS credential chains, OCI, Wayland/X11/WSLg and provider-native APIs are preferred over proprietary wrappers.

### G-11 Deletion/replacement is a design requirement
A concrete feature should be removable mostly by deleting its implementation directory and composition registration.

### G-12 Remote is not allowed to infect local Core
Local mode must remain fully usable without Gateway, EC2, EBS, OIDC or remote-host packages.

## 5. Version themes and gates

| Version | Theme | Release gate |
|---|---|---|
| v0.1 | Local Foundation | local Session lifecycle works with systemd/nerdctl; managed local storage can inspect/grow and safely shrink/compact where backend supports it |
| v0.2 | Developer Workspace | repo/workspace, SSH/VS Code, localhost preview, snapshots and coding agent work without host credential leakage |
| v0.3 | Security + Git | Capability/Grant/Approval works and safe Git read/write uses it |
| v0.4 | External Capabilities | GitHub, AWS delegated access and Registry reuse Security contracts; standard AWS tools work |
| v0.5 | Local GUI + IDE | WSLg/native GUI and IntelliJ work as explicit replaceable host integrations |
| v0.6 | Local Web + Interaction | local Web UI, Browser Notification, approval interaction and code-server work without becoming security authority |
| v0.7 | Remote + EC2 | remote Linux/EC2/EBS compositions work without changing Core Session/Security semantics |

## 6. Feature matrix

Legend: `M` mandatory, `O` optional/experimental, `-` out of scope.

| Capability | 0.1 | 0.2 | 0.3 | 0.4 | 0.5 | 0.6 | 0.7 |
|---|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| Core contracts/orchestration | M | M | M | M | M | M | M |
| runtime.incus local | M | M | M | M | M | M | M |
| local managed storage | M | M | M | M | M | M | M |
| local QCOW2 backend | O/M* | O/M* | O/M* | O/M* | O/M* | O/M* | O/M* |
| Btrfs filesystem/storage implementation | M | M | M | M | M | M | M |
| grow/inspect/health | M | M | M | M | M | M | M |
| shrink/compact orchestration | M | M | M | M | M | M | M |
| repo/workspace | - | M | M | M | M | M | M |
| VS Code/SSH | - | M | M | M | M | M | M |
| local preview | - | M | M | M | M | M | M |
| coding agent | - | M | M | M | M | M | M |
| Security Framework | - | - | M | M | M | M | M |
| Git plugin | - | - | M | M | M | M | M |
| GitHub plugin | - | - | - | M | M | M | M |
| AWS delegated identity | - | - | - | M | M | M | M |
| Packer/Terraform AWS compatibility | - | - | - | M | M | M | M |
| Registry provider | - | O | - | M | M | M | M |
| WSLg / GUI / IntelliJ | - | - | - | - | M | M | M |
| Hacocoon Web UI + Browser Notification | - | - | - | - | - | M | M |
| code-server local Web IDE | - | - | - | - | - | M | M |
| remote-linux host adapter | - | - | - | - | - | - | M |
| remote Gateway integration | - | - | - | - | - | - | M |
| runtime.ec2 | - | - | - | - | - | - | M |
| storage.ebs | - | - | - | - | - | - | M |

`O/M*`: QCOW2 is the target managed-image backend when the supported host can provide the required block attachment mechanism. A sparse raw-loop fallback may remain supported. The mandatory requirement is the provider-neutral managed local storage lifecycle and safe behavior, not that Core knows QCOW2.

## 7. Stable seams introduced early

- `SessionID` exists from v0.1.
- Core depends on `Runtime`, not Incus.
- Core depends on `Storage`, not Btrfs/QCOW2/EBS.
- Storage implementations may internally compose a narrow `BlockStore` seam because local image and EBS are real replacement boundaries.
- Feature Plugins request Security authorization; they do not own policy truth.
- Host/Access/GUI/Web/Remote implementations do not define Session semantics.

## 8. Release discipline

1. Version N+1 starts only after version N acceptance is automated.
2. Future code may exist but current release must not depend on it.
3. Security-sensitive behavior needs negative tests.
4. Destructive storage operations require preflight, explicit state transitions, verification and fail-safe cleanup.
5. `haco doctor` reports missing prerequisites by layer.
6. New vendor-specific imports in Core require architecture review.
7. A new abstraction must preserve a real replacement/deletion boundary, not aesthetic purity.

## 9. Explicit non-goals through v0.7

- proprietary Hacocoon IDE/editor/chat implementation;
- Kubernetes control plane;
- SaaS multi-tenant scheduler/control plane;
- organization-wide GitHub administration;
- universal HTTPS MITM;
- universal CLI argument firewall;
- syscall-level DLP;
- automatic public exposure of Session listeners;
- plugin marketplace before a real external ecosystem exists;
- Go `plugin.so` as the public extension boundary;
- pretending one shared host is a strong isolation boundary for mutually distrusting human tenants.
