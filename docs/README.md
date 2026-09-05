# Documentation

[**日本語**](README.ja.md) | English

Hacocoon is pre-1.0. Keep architecture intent, current repository reality, development-checkpoint numbering, published releases, and real-host acceptance separate.

## Start here

- Current code reality: [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md)
- Development-checkpoint source: [`status/checkpoints.yaml`](status/checkpoints.yaml)
- Development-checkpoint policy/status/history: [`status/versioning-and-release-status.md`](status/versioning-and-release-status.md)
- Design principles: [`DESIGN_PRINCIPLES.md`](DESIGN_PRINCIPLES.md)
- Architecture / roadmap: [`status/architecture-and-roadmap.md`](status/architecture-and-roadmap.md)
- Security architecture: [`security/security-architecture.md`](security/security-architecture.md)
- Trusted logical Host: [`design/trusted-host.md`](design/trusted-host.md)
- Windows / WSL bootstrap and default `haco-host` entry: [`WINDOWS_WSL_BOOTSTRAP.md`](WINDOWS_WSL_BOOTSTRAP.md)
- Core / Standard / Plugin boundaries: [`design/plugin-architecture.md`](design/plugin-architecture.md)
- Canonical terminology: [`reference/terminology-and-boundaries.md`](reference/terminology-and-boundaries.md)
- Logging policy: [`reference/logging.md`](reference/logging.md)
- Domain-aware egress: [`EGRESS_AUTHORIZATION.md`](EGRESS_AUTHORIZATION.md)
- Managed Btrfs storage: [`design/btrfs-storage-layout.md`](design/btrfs-storage-layout.md)
- Controller/client transport: [`design/controller-client-transport.md`](design/controller-client-transport.md)
- Reusable client adapters: [`CLIENT_ADAPTER_CONTRACT.md`](CLIENT_ADAPTER_CONTRACT.md)
- Client interaction events and notification delivery: [`INTERACTION_EVENTS.md`](INTERACTION_EVENTS.md)
- Documentation style: [`DOCUMENTATION_STYLE_GUIDE.md`](DOCUMENTATION_STYLE_GUIDE.md)

## Documentation layout

Long-lived paths describe the subject, not the release that introduced it.

```text
docs/design/      feature and architecture contracts
docs/security/    security and trust-boundary documentation
docs/reference/   terminology and reference material
docs/status/      roadmap and version/status authority
docs/adr/         architecture decision records; ADR sequence numbers are intentional
```

Normal docs must not use milestone/version or arbitrary reading-order prefixes in filenames.

## Source-of-truth order

1. `IMPLEMENTATION_STATUS.md` for current code reality
2. `status/checkpoints.yaml` for development-checkpoint numbering, current checkpoint, and Gate identity
3. `status/versioning-and-release-status.md` for checkpoint policy, human-readable status, and history/context
4. `DESIGN_PRINCIPLES.md` for cross-cutting product/architecture constraints
5. `status/architecture-and-roadmap.md` for product boundary and future direction
6. `reference/terminology-and-boundaries.md` and `security/security-architecture.md`
7. the relevant design specification under `design/`
8. focused operational/reference documents
9. README files and indexes

README/index files intentionally do **not** copy the checkpoint table. The current `v0.N` value and version/Gate mapping belong in [`status/checkpoints.yaml`](status/checkpoints.yaml); human-readable status belongs in [`status/versioning-and-release-status.md`](status/versioning-and-release-status.md); implementation detail belongs in [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md).

## Core / Standard / Plugin rule

- **Core** defines stable Environment, Policy / Approval / Capability, interaction, and boundary-control contracts.
- **Standard** provides project-maintained, replaceable default implementations expected in normal installations, including the current Incus backend and the default hostname-aware HTTP/HTTPS egress proxy/enforcer.
- **Plugin** contains optional or specialized integrations whose absence still leaves a generally useful Hacocoon installation, including nerdctl / Docker / OCI tooling.

For outbound access, the egress request/policy/controller contract belongs to Core while the concrete default proxy/enforcement implementation belongs to Standard. The Incus adapter supplies the proxy-only lower-layer transport guard, bridge DNS disablement and trusted source mapping; repository implementation is complete while real supported-Incus acceptance remains host-dependent.

`haco-host` is trusted infrastructure supplied by the local Incus integration, not an Environment and not an OCI-plugin requirement. The current lifecycle/default-entry slice is implemented; broader Git/OCI/credential/control-channel migration remains follow-up work.

## Current checkpoint

Do not infer the current checkpoint from individual feature pages, README prose, or commit messages. Use:

- [`status/checkpoints.yaml`](status/checkpoints.yaml) for the authoritative current `v0.N` checkpoint and ordered version/Gate mapping;
- [`status/versioning-and-release-status.md`](status/versioning-and-release-status.md) for policy and human-readable checkpoint status/history;
- [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md) for what is actually implemented and which acceptance gaps remain.

Pre-1.0 checkpoint numbers are deliberately cheap. Meaningful product, implementation, operator-experience, observability, or acceptance slices may advance the next minor without waiting for every earlier real-host acceptance item. README/index pages link to the authority instead of copying the evolving table.

Current design/reference documents include:

- [`design/trusted-host.md`](design/trusted-host.md)
- [`design/managed-sandbox-network.md`](design/managed-sandbox-network.md)
- [`design/git-fetch-plugin.md`](design/git-fetch-plugin.md)
- [`design/oci-seed-recommendation.md`](design/oci-seed-recommendation.md)
- [`design/oci-image-deletion.md`](design/oci-image-deletion.md)
- [`design/oci-seed-and-cow.md`](design/oci-seed-and-cow.md)
- [`design/docker-compatibility-plugin.md`](design/docker-compatibility-plugin.md)
- [`EGRESS_AUTHORIZATION.md`](EGRESS_AUTHORIZATION.md)
- [`INTERACTION_EVENTS.md`](INTERACTION_EVENTS.md)
- [`reference/logging.md`](reference/logging.md)
- [`design/btrfs-storage-layout.md`](design/btrfs-storage-layout.md)
- [`design/controller-client-transport.md`](design/controller-client-transport.md)
- [`OPTIONAL_LOCAL_OCI_REGISTRY.md`](OPTIONAL_LOCAL_OCI_REGISTRY.md) — deferred optional direction

## Trusted Host / WSL entry

On the supported local Incus path, Hacocoon distinguishes the actual Linux/WSL **Physical Host** from the persistent trusted logical **`haco-host`**. The Physical Host retains Incus, loop/Btrfs, and other platform authority. `haco-host` is part of the TCB and must not be confused with an untrusted Environment.

The current repository slice provides controller-backed `haco setup` / temporary `hacoq host shell`, exact ownership marking, collision refusal, managed-storage placement, and a dedicated WSL login entry so a completed Windows install can treat `wsl -d Hacocoon` as “open my Hacocoon Host.” Raw Incus control is not mounted into `haco-host`; the root Physical Host shell remains the explicit recovery path. See [`design/trusted-host.md`](design/trusted-host.md) and [`WINDOWS_WSL_BOOTSTRAP.md`](WINDOWS_WSL_BOOTSTRAP.md).

## Reusable client adapter boundary

`pkg/clientadapter` exposes a VS Code-independent client contract for exact Environment ensure/reuse, state inspection, `/workspace` discovery, loopback SSH/TCP connections, revoke/delete, and `pkg/interaction` batches.

The client keeps its private key and IDE configuration. Hacocoon receives only SSH public-key material, revalidates returned connections as loopback-only, and refuses reuse when the canonical Workspace or requested access mode differs. The ordinary `haco create` + `haco ssh` + `ssh` flow is the non-VS-Code proof; code-server, JetBrains, and future clients can reuse the same boundary.

See [`CLIENT_ADAPTER_CONTRACT.md`](CLIENT_ADAPTER_CONTRACT.md).

## Client interaction and notification delivery

`pkg/interaction` exposes a client-neutral, read-only event projection over the capability audit stream. Clients get stable IDs, resumable byte cursors, bounded batches, attention/recovery flags, and a deliberately minimized schema without raw resources, authority attributes, provider output, approval tokens, or free-form audit reasons.

`haco-notify` and the optional VS Code notification extension consume this client-safe stream. Observation or notification never authorizes or executes a Capability. See [`INTERACTION_EVENTS.md`](INTERACTION_EVENTS.md).

## Base vs OCI CLI

```text
haco base list
haco base inspect <base>

HACO_PLUGIN_OCI=nerdctl  haco plugin oci seed sample
HACO_PLUGIN_OCI=nerdctl  haco plugin oci seed recommend
HACO_PLUGIN_OCI=nerdctl  haco plugin oci seed build
HACO_PLUGIN_OCI=nerdctl  haco plugin oci seed current
HACO_PLUGIN_OCI=docker   haco plugin oci docker status <environment>
HACO_PLUGIN_OCI=docker   haco plugin oci docker prepare <environment>
```

`haco base` describes Hacocoon Environment starting points. `haco plugin oci` is optional developer-workload/container tooling. Core has no mandatory OCI runtime.

## Cloud

The provider-neutral remote/cloud routing seam remains. Concrete EC2/AWS/EBS implementation has been removed from the active tree and cloud implementation is currently deferred. See [`design/remote-and-cloud-runtime.md`](design/remote-and-cloud-runtime.md).

## Editing rule

Follow [`DOCUMENTATION_STYLE_GUIDE.md`](DOCUMENTATION_STYLE_GUIDE.md). Update the owning document first, then `IMPLEMENTATION_STATUS.md`. When a development checkpoint consumes or changes a minor number, use `tools/bump-milestone` so `status/checkpoints.yaml`, the status mirrors, and generated build identity stay synchronized. Keep English/Japanese companions aligned and run:

```bash
python tools/check_docs.py
```
