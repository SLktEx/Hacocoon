# Versioning & Release Status

> **Authoritative milestone numbering · Updated 2026-08-30**

Hacocoon is **pre-1.0**. Milestone numbers describe product/implementation progression; they are not compatibility guarantees, release tags, or proof of production support.

Use this document for **which version number belongs to which feature gate**. Use [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md) for exact code reality and host-dependent acceptance.

## Numbering policy

> **One independently useful product feature is approximately one minor milestone.**

1. A new independently useful feature normally consumes the next `v0.N` milestone.
2. Multiple PRs that are slices of one coherent feature may share that milestone.
3. Security/hardening, bug fixes, refactors, CLI namespace cleanup, CI, docs, release engineering, and test-only work normally do not consume a product version by themselves.
4. The feature implementation PR must update this file and `IMPLEMENTATION_STATUS.md`; do not defer the version bump to later cleanup.
5. Design-only specifications may reserve future numbers but remain **planned** until implementation lands.
6. Historical commit messages, PR titles, candidate branches, and superseded filenames are not authoritative numbering.
7. Tags/releases are separate from roadmap milestone numbering.

## Current authoritative numbering

**Status legend:** ✅ implemented · 🧪 partial/experimental/historical · 🚧 planned

| Version | Gate | `main` status |
|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | ✅ implemented |
| v0.2 | Workspace Abstraction & Lease | ✅ implemented |
| v0.3 | Client & Interactive Access | ✅ implemented |
| v0.4 | Policy & Capability Foundation | ✅ implemented |
| v0.5 | Git / GitHub Capability | ✅ implemented |
| v0.6 | Agent & Orchestrator Integration | ✅ implemented |
| v0.7 | Remote / Cloud Runtime & External Capabilities | 🧪 provider routing seam retained; concrete EC2/AWS/EBS implementation deferred |
| v0.8 | Client Adapters & VS Code Integration | ✅ implemented |
| v0.9 | Per-Agent Sandbox & Agent Host Integration | ✅ broker foundation implemented |
| v0.10 | VS Code Remote Agent Host Adapter | ✅ implemented |
| v0.11 | Base Images & Custom Environments | ✅ first slice implemented |
| v0.12 | Sandbox Resource Limits | ✅ first slice implemented |
| v0.13 | Managed Sandbox Network | ✅ implemented |
| v0.14 | Git Fetch Plugin | ✅ implemented |
| v0.15 | OCI Seed Recommendation | ✅ implemented |
| v0.16 | OCI Image Deletion | ✅ first slice implemented |
| v0.17 | Docker Compatibility Plugin | ✅ repository implementation complete; real-host acceptance tracked separately |
| v0.18 | OCI Seed Builder & Btrfs/COW | 🧪 build/publish + operations-hardening repository slices implemented; real-host/private-registry/COW acceptance pending |

The fully implemented product progression is contiguous through **v0.17**. v0.18 has multiple repository implementation slices but is not yet a complete feature gate.

v0.7 keeps its milestone number because the provider-neutral routing seam introduced by that gate remains implemented. The previous concrete EC2/AWS/EBS slice is intentionally absent from the active tree and **cloud implementation is currently deferred** while local/provider contracts stabilize.

## v0.12 → v0.18 rebaseline

Independent work had accumulated under one OCI milestone and letter-suffixed sub-milestones, which no longer matched the original one-feature-per-version intent. The authoritative replacement is:

```text
v0.12  Sandbox Resource Limits                 implemented
v0.13  Managed Sandbox Network                 implemented
v0.14  Git Fetch Plugin                        implemented
v0.15  OCI Seed Recommendation                 implemented
v0.16  OCI Image Deletion                      implemented
v0.17  Docker Compatibility Plugin             implemented
v0.18  OCI Seed Builder & Btrfs/COW            partial / repository slices
```

A short-lived intermediate rebaseline reserved v0.18 for Optional Local OCI Registry and v0.19 for Seed Builder/COW. That reservation is superseded: Local Registry infrastructure is deferred and unversioned because it is not required by the default architecture, and Seed Builder/COW therefore occupies v0.18.

Historical commits, closed PRs, and old branches may retain superseded OCI milestone labels. They are historical records only.

The later CLI namespace cleanup that separated `haco base ...` from `haco plugin oci ...` is a boundary/refactor correction and does not consume another product milestone.

## Specification map

- [`13_v0.13_MANAGED_SANDBOX_NETWORK.md`](13_v0.13_MANAGED_SANDBOX_NETWORK.md)
- [`14_v0.14_GIT_FETCH_PLUGIN.md`](14_v0.14_GIT_FETCH_PLUGIN.md)
- [`15_v0.15_OCI_SEED_RECOMMENDATION.md`](15_v0.15_OCI_SEED_RECOMMENDATION.md)
- [`16_v0.16_OCI_IMAGE_DELETION.md`](16_v0.16_OCI_IMAGE_DELETION.md)
- [`17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.md`](17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.md)
- [`18_v0.18_OCI_SEED_AND_COW.md`](18_v0.18_OCI_SEED_AND_COW.md)
- [`OPTIONAL_LOCAL_OCI_REGISTRY.md`](OPTIONAL_LOCAL_OCI_REGISTRY.md) — deferred optional infrastructure, not a reserved milestone

## Acceptance watch list

- **v0.7:** cloud implementation is currently deferred; acceptance will be redefined when a concrete cloud adapter returns.
- **v0.8:** real Windows/WSL + Incus + VS Code acceptance remains pending.
- **v0.9/v0.10:** real Agent Host/AHP routing remains host-dependent.
- **v0.11/v0.12:** real Base/image and resource-enforcement acceptance remains host-dependent.
- **v0.13:** real supported-Incus network/profile/ACL acceptance remains host-dependent.
- **v0.14:** brokered fetch is implemented; real private-repository combinations remain acceptance-sensitive.
- **v0.15/v0.16:** OCI plugin repository behavior is implemented.
- **v0.17:** repository lifecycle/CLI integration is implemented; real Base + Incus/systemd socket-activation acceptance remains host-dependent.
- **v0.18:** build/publish plus explicit pin/re-enable, conservative old-revision GC, and restart/crash recovery are implemented at the repository gate; real Incus/containerd/Docker acceptance, authenticated/private-registry combinations including credential-free Environment harvesting where supported, and physical Btrfs COW measurement remain pending.

## Rule of thumb

> **New independent feature? Take the next minor number in the same PR. Fix/hardening/refactor? Keep the current product number.**

Use this file for numbering, `IMPLEMENTATION_STATUS.md` for code reality, and roadmap/specifications for intent.
