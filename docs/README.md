# Documentation

[**日本語**](README.ja.md) | English

Hacocoon is pre-1.0. Keep architecture intent, current repository reality, and real-host acceptance separate.

## Start here

- Current code reality: [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md)
- Design principles: [`DESIGN_PRINCIPLES.md`](DESIGN_PRINCIPLES.md)
- Architecture / roadmap: [`status/architecture-and-roadmap.md`](status/architecture-and-roadmap.md)
- Milestone numbering: [`status/versioning-and-release-status.md`](status/versioning-and-release-status.md)
- Security architecture: [`security/security-architecture.md`](security/security-architecture.md)
- Core / Standard / Plugin boundaries: [`design/plugin-architecture.md`](design/plugin-architecture.md)
- Canonical terminology: [`reference/terminology-and-boundaries.md`](reference/terminology-and-boundaries.md)
- Domain-aware egress: [`EGRESS_AUTHORIZATION.md`](EGRESS_AUTHORIZATION.md)
- Managed Btrfs storage: [`design/btrfs-storage-layout.md`](design/btrfs-storage-layout.md)
- Reusable client adapters: [`CLIENT_ADAPTER_CONTRACT.md`](CLIENT_ADAPTER_CONTRACT.md)
- Client interaction events: [`INTERACTION_EVENTS.md`](INTERACTION_EVENTS.md)
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
2. `status/versioning-and-release-status.md` for milestone numbering/status
3. `DESIGN_PRINCIPLES.md` for cross-cutting product/architecture constraints
4. `status/architecture-and-roadmap.md` for product boundary and roadmap intent
5. `reference/terminology-and-boundaries.md` and `security/security-architecture.md`
6. the relevant design specification under `design/`
7. focused operational/reference documents
8. README files and indexes

## Core / Standard / Plugin rule

- **Core** defines stable Environment, Policy / Approval / Capability, interaction, and boundary-control contracts.
- **Standard** provides project-maintained, replaceable default implementations expected in normal installations, including the current Incus backend and the default hostname-aware HTTP/HTTPS egress proxy/enforcer.
- **Plugin** contains optional or specialized integrations whose absence still leaves a generally useful Hacocoon installation, including nerdctl / Docker / OCI tooling.

For outbound access, the egress request/policy/controller contract belongs to Core while the concrete default proxy/enforcement implementation belongs to Standard. The Incus adapter supplies the proxy-only lower-layer transport guard, bridge DNS disablement and trusted source mapping; repository implementation is complete while real supported-Incus acceptance remains host-dependent.

## Current feature gates

The authoritative table lives in [`status/versioning-and-release-status.md`](status/versioning-and-release-status.md). Current late-stage gates are:

| Version | Gate | State |
|---|---|---|
| v0.13 | Managed Sandbox Network | implemented |
| v0.14 | Git Fetch Plugin | implemented |
| v0.15 | OCI Seed Recommendation | implemented |
| v0.16 | OCI Image Deletion | first slice implemented |
| v0.17 | OCI Seed Builder & Btrfs/COW | repository slices / partial acceptance |
| v0.18 | Docker Compatibility Plugin | repository implementation complete; real-host acceptance separate |
| v0.19 | Domain-aware Egress Authorization | repository implementation complete; real supported-Incus acceptance separate |
| v0.20 | Managed Btrfs Rootfs Storage | first repository slice implemented; physical COW/compaction acceptance separate |
| v0.21 | Managed Btrfs Transparent Compression | `compress=zstd:3` managed default implemented; physical compression/performance acceptance separate |

The current milestone position is **v0.21**. Minor versions are lightweight pre-1.0 progress checkpoints, so a partial earlier milestone does not block later checkpoints. Local OCI Registry is deferred optional infrastructure and does not reserve a milestone.

Current design documents:

- [`design/managed-sandbox-network.md`](design/managed-sandbox-network.md)
- [`design/git-fetch-plugin.md`](design/git-fetch-plugin.md)
- [`design/oci-seed-recommendation.md`](design/oci-seed-recommendation.md)
- [`design/oci-image-deletion.md`](design/oci-image-deletion.md)
- [`design/oci-seed-and-cow.md`](design/oci-seed-and-cow.md)
- [`design/docker-compatibility-plugin.md`](design/docker-compatibility-plugin.md)
- [`EGRESS_AUTHORIZATION.md`](EGRESS_AUTHORIZATION.md)
- [`design/btrfs-storage-layout.md`](design/btrfs-storage-layout.md)
- [`OPTIONAL_LOCAL_OCI_REGISTRY.md`](OPTIONAL_LOCAL_OCI_REGISTRY.md) — deferred optional direction

## Reusable client adapter boundary

`pkg/clientadapter` exposes a VS Code-independent client contract for exact Environment ensure/reuse, state inspection, `/workspace` discovery, loopback SSH/TCP connections, revoke/delete, and `pkg/interaction` batches.

The client keeps its private key and IDE configuration. Hacocoon receives only SSH public-key material, revalidates returned connections as loopback-only, and refuses reuse when the canonical Workspace or requested access mode differs. The ordinary `haco create` + `haco ssh` + `ssh` flow is the non-VS-Code proof; code-server, JetBrains, and future clients can reuse the same boundary.

See [`CLIENT_ADAPTER_CONTRACT.md`](CLIENT_ADAPTER_CONTRACT.md).

## Client interaction boundary

`pkg/interaction` exposes a client-neutral, read-only event projection over the capability audit stream. Clients get stable IDs, resumable byte cursors, bounded batches, attention/recovery flags, and a deliberately minimized schema without raw resources, authority attributes, provider output, approval tokens, or free-form audit reasons.

Observation never authorizes or executes a Capability. See [`INTERACTION_EVENTS.md`](INTERACTION_EVENTS.md).

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

## Numbering rule

> Minor versions are pragmatic pre-1.0 progress checkpoints.

Meaningful implementation slices may take the next minor even when follow-up work or real-host acceptance remains. Fixes, hardening, refactors, CLI namespace cleanup, CI and docs normally do not consume another product version by themselves. Version mapping belongs in status documents and page bodies, never in normal documentation filenames.

## Editing rule

Follow [`DOCUMENTATION_STYLE_GUIDE.md`](DOCUMENTATION_STYLE_GUIDE.md). Update the owning document first, then `IMPLEMENTATION_STATUS.md`, and update `status/versioning-and-release-status.md` whenever a development checkpoint consumes or changes a milestone. Keep English/Japanese companions aligned and run:

```bash
python tools/check_docs.py
```
