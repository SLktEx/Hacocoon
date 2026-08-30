# Documentation

[**日本語**](README.ja.md) | English

Hacocoon is pre-1.0. Keep architecture intent, current repository reality, and real-host acceptance separate.

## Start here

- Current code reality: [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md)
- Architecture / roadmap: [`00_REBASELINE_AND_ROADMAP.md`](00_REBASELINE_AND_ROADMAP.md)
- Milestone numbering: [`00D_VERSIONING_AND_RELEASE_STATUS.md`](00D_VERSIONING_AND_RELEASE_STATUS.md)
- Security: [`00B_SECURITY_ARCHITECTURE.md`](00B_SECURITY_ARCHITECTURE.md)
- Plugin boundaries: [`00A_PLUGIN_ARCHITECTURE.md`](00A_PLUGIN_ARCHITECTURE.md)

## Source-of-truth order

1. `IMPLEMENTATION_STATUS.md` for current code reality
2. `00D_VERSIONING_AND_RELEASE_STATUS.md` for milestone numbering/status
3. `00_REBASELINE_AND_ROADMAP.md` for product boundary and roadmap intent
4. security/terminology docs
5. the relevant versioned specification

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
| v0.17 | OCI Seed Builder & Btrfs/COW | planned |
| v0.18 | Docker Compatibility Plugin | foundation / partial |

The fully implemented product progression is contiguous through **v0.16**. v0.17 is the next planned implementation gate; v0.18 has foundation code already landed early.

Local OCI Registry is deferred optional infrastructure, not a reserved roadmap milestone.

Specifications:
- [`13_v0.13_MANAGED_SANDBOX_NETWORK.md`](13_v0.13_MANAGED_SANDBOX_NETWORK.md)
- [`14_v0.14_GIT_FETCH_PLUGIN.md`](14_v0.14_GIT_FETCH_PLUGIN.md)
- [`15_v0.15_OCI_SEED_RECOMMENDATION.md`](15_v0.15_OCI_SEED_RECOMMENDATION.md)
- [`16_v0.16_OCI_IMAGE_DELETION.md`](16_v0.16_OCI_IMAGE_DELETION.md)
- [`17_v0.17_OCI_SEED_AND_COW.md`](17_v0.17_OCI_SEED_AND_COW.md)
- [`18_v0.18_DOCKER_COMPATIBILITY_PLUGIN.md`](18_v0.18_DOCKER_COMPATIBILITY_PLUGIN.md)
- [`OPTIONAL_LOCAL_OCI_REGISTRY.md`](OPTIONAL_LOCAL_OCI_REGISTRY.md) — deferred optional direction

## Base vs OCI CLI

```text
haco base list
haco base inspect <base>

HACO_PLUGIN_OCI=nerdctl  haco plugin oci seed sample
HACO_PLUGIN_OCI=nerdctl  haco plugin oci seed recommend
HACO_PLUGIN_OCI=nerdctl  haco plugin oci image delete <reference>
```

`haco base` describes Hacocoon Environment starting points. `haco plugin oci` is optional developer-workload/container tooling. Core has no mandatory OCI runtime.

## OCI storage direction

v0.17 owns trusted Host acquisition/cache, offline immutable Seed construction/publication, and Incus/storage-driver COW. A Local Registry is not a prerequisite and normal direct upstream pulls remain valid when policy allows.

Docker compatibility is v0.18. The repository already contains early Docker CLI/Engine and systemd packaging foundation, but complete Environment/Base lifecycle integration follows the v0.17 Seed/Base pipeline.

## Cloud

The v0.7 provider-neutral routing seam remains. Concrete EC2/AWS/EBS implementation has been removed from the active tree and cloud implementation is currently deferred.

## Editing rule

Update the owning specification, `IMPLEMENTATION_STATUS.md`, and—when a new independent feature consumes a milestone—`00D_VERSIONING_AND_RELEASE_STATUS.md` in the same PR. Run `python tools/check_docs.py`.
