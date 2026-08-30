# Versioning & Release Status

> **Authoritative milestone numbering · Updated 2026-08-30**

Hacocoon is **pre-1.0**. Milestone numbers describe product/implementation progression; they are not compatibility guarantees, release tags, or proof of production support.

Use this document for **which version number belongs to which feature gate**. Use [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md) for exact code reality and host-dependent acceptance.

## Numbering policy

> **One independently useful product feature is approximately one minor milestone.**

1. A new independently useful feature normally consumes the next `v0.N` milestone.
2. Multiple PRs that are slices of one coherent feature may share that milestone.
3. Security/hardening, bug fixes, refactors, CI, docs, release engineering, and test-only work normally do not consume a product version by themselves.
4. The feature implementation PR must update this file and `IMPLEMENTATION_STATUS.md`; do not defer the version bump to later cleanup.
5. Design-only specifications may reserve future numbers but remain **planned** until implementation lands.
6. Historical commit messages, PR titles, candidate branches, and superseded filenames are not authoritative numbering.
7. Tags/releases are separate from roadmap milestone numbering.

## Current authoritative numbering

**Status legend:** ✅ implemented · 🧪 partial/experimental · 🚧 planned

| Version | Gate | `main` status |
|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | ✅ implemented |
| v0.2 | Workspace Abstraction & Lease | ✅ implemented |
| v0.3 | Client & Interactive Access | ✅ implemented |
| v0.4 | Policy & Capability Foundation | ✅ implemented |
| v0.5 | Git / GitHub Capability | ✅ implemented |
| v0.6 | Agent & Orchestrator Integration | ✅ implemented |
| v0.7 | Remote / Cloud Runtime & External Capabilities | 🧪 experimental implementation; EC2 remains default-off |
| v0.8 | Client Adapters & VS Code Integration | ✅ implemented |
| v0.9 | Per-Agent Sandbox & Agent Host Integration | ✅ broker foundation implemented |
| v0.10 | VS Code Remote Agent Host Adapter | ✅ implemented |
| v0.11 | Base Images & Custom Environments | ✅ first slice implemented |
| v0.12 | Sandbox Resource Limits | ✅ first slice implemented |
| v0.13 | Managed Sandbox Network | ✅ implemented |
| v0.14 | Git Fetch Plugin | ✅ implemented |
| v0.15 | OCI Seed Recommendation | ✅ implemented |
| v0.16 | OCI Image Deletion | ✅ first slice implemented |
| v0.17 | Docker Compatibility Plugin | 🧪 foundation implemented; full plugin integration pending |
| v0.18 | Optional Local OCI Registry | 🚧 planned |
| v0.19 | OCI Seed Builder & Btrfs/COW | 🚧 planned |

The fully implemented product progression is contiguous through **v0.16**. v0.17 has a foundation but is not a complete feature gate. v0.18 and v0.19 remain planned.

## v0.12 → v0.19 rebaseline

Independent work had accumulated under `v0.13` and `v0.13A/B/C`, which no longer matched the original one-feature-per-version intent. The authoritative replacement is:

```text
v0.12  Sandbox Resource Limits                 implemented
v0.13  Managed Sandbox Network                 implemented
v0.14  Git Fetch Plugin                        implemented
v0.15  OCI Seed Recommendation                 implemented
v0.16  OCI Image Deletion                      implemented
v0.17  Docker Compatibility Plugin             partial/foundation
v0.18  Optional Local OCI Registry             planned
v0.19  OCI Seed Builder & Btrfs/COW            planned
```

Old `v0.13` Local Registry and `v0.13A/B/C` assignments are historical only.

## Specification map

- [`13_v0.13_MANAGED_SANDBOX_NETWORK.md`](13_v0.13_MANAGED_SANDBOX_NETWORK.md)
- [`14_v0.14_GIT_FETCH_PLUGIN.md`](14_v0.14_GIT_FETCH_PLUGIN.md)
- [`15_v0.15_OCI_SEED_RECOMMENDATION.md`](15_v0.15_OCI_SEED_RECOMMENDATION.md)
- [`16_v0.16_OCI_IMAGE_DELETION.md`](16_v0.16_OCI_IMAGE_DELETION.md)
- [`17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.md`](17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.md)
- [`18_v0.18_LOCAL_OCI_REGISTRY.md`](18_v0.18_LOCAL_OCI_REGISTRY.md)
- [`19_v0.19_OCI_SEED_AND_COW.md`](19_v0.19_OCI_SEED_AND_COW.md)

## Acceptance watch list

- **v0.7:** real AWS/EC2/SSM/EBS acceptance remains pending.
- **v0.8:** real Windows/WSL + Incus + VS Code acceptance remains pending.
- **v0.9/v0.10:** real Agent Host/AHP routing remains host-dependent.
- **v0.11/v0.12:** real Base/image and resource-enforcement acceptance remains host-dependent.
- **v0.13:** real supported-Incus network/profile/ACL acceptance remains host-dependent.
- **v0.14:** brokered fetch is implemented; real private-repository combinations remain acceptance-sensitive.
- **v0.15/v0.16:** repository behavior is implemented; physical Seed publication/GC belongs to v0.19.
- **v0.17:** do not claim complete until plugin lifecycle/integration is landed and validated.
- **v0.18/v0.19:** planned only.

## Rule of thumb

> **New independent feature? Take the next minor number in the same PR. Fix/hardening/refactor? Keep the current product number.**

Use this file for numbering, `IMPLEMENTATION_STATUS.md` for code reality, and roadmap/specifications for intent.
