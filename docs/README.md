# Documentation

[**日本語**](README.ja.md) | English

Hacocoon is pre-1.0. Keep architecture intent, current repository reality, and real-host acceptance separate.

## Start here

- Current code reality: [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md)
- Design principles: [`DESIGN_PRINCIPLES.md`](DESIGN_PRINCIPLES.md)
- Architecture / roadmap: [`00_REBASELINE_AND_ROADMAP.md`](00_REBASELINE_AND_ROADMAP.md)
- Milestone numbering: [`00D_VERSIONING_AND_RELEASE_STATUS.md`](00D_VERSIONING_AND_RELEASE_STATUS.md)
- Security: [`00B_SECURITY_ARCHITECTURE.md`](00B_SECURITY_ARCHITECTURE.md)
- Core / Standard / Plugin boundaries: [`00A_PLUGIN_ARCHITECTURE.md`](00A_PLUGIN_ARCHITECTURE.md)
- Client interaction events: [`INTERACTION_EVENTS.md`](INTERACTION_EVENTS.md)

## Core / Standard / Plugin rule

Hacocoon separates product semantics from default implementations and optional integrations:

- **Core** defines stable Environment, Policy / Approval / Capability, and boundary-control contracts.
- **Standard** provides project-maintained, replaceable default implementations expected in normal installations, such as the current Incus backend and the future default egress proxy/enforcer.
- **Plugin** contains optional or specialized integrations whose absence still leaves a generally useful Hacocoon installation, including nerdctl / Docker / OCI tooling.

For outbound access, the egress request/policy/controller contract belongs to Core while the concrete default proxy/enforcement implementation belongs to Standard. This classification is architecture intent; v0.13 currently provides the default-deny network substrate, not completed domain-aware egress authorization.

## Source-of-truth order

1. `IMPLEMENTATION_STATUS.md` for current code reality
2. `00D_VERSIONING_AND_RELEASE_STATUS.md` for milestone numbering/status
3. `DESIGN_PRINCIPLES.md` for cross-cutting product/architecture constraints
4. `00_REBASELINE_AND_ROADMAP.md` for product boundary and roadmap intent
5. security/terminology docs
6. the relevant versioned specification

## Numbering rule

> One independently useful product feature is approximately one minor milestone.

Fixes, hardening, refactors, CLI namespace cleanup, CI and docs normally do not consume another product version.

## Current milestones

| Version | Gate | State |
|---|---|---|
| v0.13 | Managed Sandbox Network | implemented |
| v0.14 | Git Fetch Plugin | implemented |
| v0.15 | OCI Seed Recommendation | implemented |
| v0.16 | OCI Image Deletion | implemented first slice |
| v0.17 | Docker Compatibility Plugin | implemented; host acceptance separate |
| v0.18 | OCI Seed Builder & Btrfs/COW | first repository slice / partial |

The fully implemented product progression is contiguous through **v0.17**. v0.18 has a first repository implementation slice; real-host/COW acceptance remains pending.

Local OCI Registry is deferred optional infrastructure, not a reserved roadmap milestone.

Specifications:
- [`13_v0.13_MANAGED_SANDBOX_NETWORK.md`](13_v0.13_MANAGED_SANDBOX_NETWORK.md)
- [`14_v0.14_GIT_FETCH_PLUGIN.md`](14_v0.14_GIT_FETCH_PLUGIN.md)
- [`15_v0.15_OCI_SEED_RECOMMENDATION.md`](15_v0.15_OCI_SEED_RECOMMENDATION.md)
- [`16_v0.16_OCI_IMAGE_DELETION.md`](16_v0.16_OCI_IMAGE_DELETION.md)
- [`17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.md`](17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.md)
- [`18_v0.18_OCI_SEED_AND_COW.md`](18_v0.18_OCI_SEED_AND_COW.md)
- [`OPTIONAL_LOCAL_OCI_REGISTRY.md`](OPTIONAL_LOCAL_OCI_REGISTRY.md) — deferred optional direction

## Client interaction boundary

`pkg/interaction` exposes a client-neutral, read-only event projection over the capability audit stream. Clients get stable IDs, resumable byte cursors, bounded batches, attention/recovery flags, and a deliberately minimized schema without raw resources, attributes, provider output, approval tokens, or free-form audit reasons.

This boundary is observation only. Approval and execution remain inside the trusted Policy/Capability path, so browser, VS Code, code-server, JetBrains, and other adapters may observe the same event without the observation itself becoming authorization.

See [`INTERACTION_EVENTS.md`](INTERACTION_EVENTS.md).

## Base vs OCI CLI

```text
haco base list
haco base inspect <base>

HACO_PLUGIN_OCI=nerdctl  haco plugin oci seed sample
HACO_PLUGIN_OCI=nerdctl  haco plugin oci seed recommend
HACO_PLUGIN_OCI=nerdctl  haco plugin oci seed build
HACO_PLUGIN_OCI=nerdctl  haco plugin oci seed current
HACO_PLUGIN_OCI=nerdctl  haco plugin oci image delete <reference>
HACO_PLUGIN_OCI=docker   haco plugin oci docker status <environment>
HACO_PLUGIN_OCI=docker   haco plugin oci docker prepare <environment>
```

`haco base` describes Hacocoon Environment starting points. `haco plugin oci` is optional developer-workload/container tooling. Core has no mandatory OCI runtime.

The Docker `prepare` command validates a Base-provided compatibility profile and the pinned Hacocoon systemd units, then enables Environment-local socket activation. It does not install Docker packages or silently stop an already-active vendor daemon.

## OCI storage direction

v0.18 has a first repository slice for trusted Host acquisition/cache, offline no-NIC immutable Seed construction/publication, current-Seed resolution, and normal Incus/storage-driver cloning. Physical Btrfs COW measurement and broader real-host acceptance remain pending. A Local Registry is not a prerequisite and normal direct upstream pulls remain valid when policy allows.

## Cloud

The v0.7 provider-neutral routing seam remains. Concrete EC2/AWS/EBS implementation has been removed from the active tree and cloud implementation is currently deferred.

## Editing rule

Update the owning specification, `IMPLEMENTATION_STATUS.md`, and—when a new independent feature consumes a milestone—`00D_VERSIONING_AND_RELEASE_STATUS.md` in the same PR. Run `python tools/check_docs.py`.
