# CODEX START HERE

> **Hacocoon is a secure workspace runtime, not an AI orchestrator.**

Hacocoon is pre-1.0. Breaking changes are acceptable when they simplify the system, strengthen trust boundaries, or correct accidental contracts.

## Current status

- Fully implemented product milestones: **v0.1 → v0.16**
- v0.17 Docker Compatibility Plugin: foundation / partial
- v0.18 Optional Local OCI Registry: planned
- v0.19 OCI Seed Builder & Btrfs/COW: planned
- v0.7 provider-neutral routing remains; concrete cloud implementation is currently deferred

Read, in order:

1. [`docs/IMPLEMENTATION_STATUS.md`](docs/IMPLEMENTATION_STATUS.md)
2. [`docs/00_REBASELINE_AND_ROADMAP.md`](docs/00_REBASELINE_AND_ROADMAP.md)
3. [`docs/00D_VERSIONING_AND_RELEASE_STATUS.md`](docs/00D_VERSIONING_AND_RELEASE_STATUS.md)
4. [`docs/00B_SECURITY_ARCHITECTURE.md`](docs/00B_SECURITY_ARCHITECTURE.md)
5. the relevant versioned specification
6. [`docs/90_CODEX_IMPLEMENTATION_HANDOFF.md`](docs/90_CODEX_IMPLEMENTATION_HANDOFF.md)

## Versioning

One independently useful product feature is approximately one minor milestone. New feature PRs update versioning and implementation status in the same PR. Security fixes, bug fixes, hardening, refactors, CLI namespace cleanup, CI, docs, release engineering, and test-only work normally do not consume a version.

## Roadmap snapshot

```text
v0.13 Managed Sandbox Network              implemented
v0.14 Git Fetch Plugin                     implemented
v0.15 OCI Seed Recommendation              implemented
v0.16 OCI Image Deletion                   implemented first slice
v0.17 Docker Compatibility Plugin          foundation / partial
v0.18 Optional Local OCI Registry          planned
v0.19 OCI Seed Builder & Btrfs/COW         planned
```

## Hard boundaries

- Coding agents do not receive Hacocoon/Incus management authority.
- Reusable Host credentials are not copied into arbitrary Environments.
- Privileged external operations cross explicit Policy/Capability/plugin boundaries.
- Git push/fetch stays under the Git/GitHub plugin/capability boundary.
- Base identity uses `haco base ...`.
- OCI/container tooling uses optional `haco plugin oci ...`.
- With `HACO_PLUGIN_OCI` unset, Core must not require containerd, nerdctl, Docker CLI, Docker Engine, or a local Registry.
- The maintained OCI plugin may use containerd + nerdctl; Core has no mandatory OCI runtime.
- Docker compatibility is optional and Environment-local; never mount the Host Docker socket.
- Local Registry is optional, not a prerequisite for ordinary pulls or Seed construction.
- Seed construction must not share one writable `/var/lib/containerd` across Environments.
- Requested security/resource controls that cannot be enforced fail closed.
- Managed sandbox networking must not silently fall back to broad/default Incus networking.
- Real-host acceptance is distinct from repository tests.

## Cloud

The v0.7 provider-neutral routing seam remains as architecture. Concrete EC2/AWS/EBS support has been removed from the active tree and is deferred until the local/provider contracts are stable enough for meaningful cloud validation. Do not reintroduce cloud-specific code into Core merely to preserve historical behavior.

## Work method

Inspect code and tests, define intended behavior, implement the smallest coherent change, exercise hostile input/retry/concurrency/cleanup, and run the maintained checks:

```bash
bash tools/ci-local.sh
```

Keep real Incus, Windows/WSL + VS Code, private-registry, Docker compatibility, and any future cloud-adapter acceptance separate from repository CI claims.
