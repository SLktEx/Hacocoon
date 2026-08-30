# Implementation Status

Status date: 2026-08-30.

This file reports **current code reality**. It is not a compatibility guarantee and it does not turn host-dependent acceptance into an implementation claim.

Hacocoon is still **pre-1.0**. The implemented milestone progression is contiguous through **v0.17**; v0.18 and v0.19 are planned.

## Current repository reality

| Area | Current repository reality | Release | Validation status |
|---|---|---:|---|
| Secure Workspace Runtime | `haco create --workspace`, `haco exec`, `haco shell`, `haco delete` | v0.1 | unit/process integration; real Incus acceptance host-dependent |
| Workspace model / Lease | canonical Workspace identity, RO/RW leases, conflict prevention, process serialization | v0.1-v0.2 | unit/persistence/concurrency coverage |
| Client access | status, loopback forwarding, SSH prepare/revoke | v0.3 | unit/process coverage; real SSH acceptance pending |
| Policy / Capability | fail-closed policy, approval, audit | v0.4 | unit/process/CLI E2E coverage |
| Git push plugin | `haco plugin git push`; exact SHA/ref authority and force-with-lease without exporting host credentials | v0.5 | unit/adversarial/real-git/CLI E2E coverage |
| Agent / Orchestrator | `haco run`, stable JSON, security event export | v0.6 | unit/race/process/CLI E2E coverage |
| EC2 provider | experimental, disabled unless explicitly enabled | v0.7 | fake-AWS tests; real AWS acceptance pending |
| VS Code adapter | `haco-vscode` → loopback SSH → standard Remote-SSH | v0.8 | helper tests; real Windows/WSL acceptance pending |
| Per-agent sandbox | trusted opaque session → dedicated Environment binding with persisted ownership proof | v0.9 | ownership/persistence/collision coverage |
| Agent Host adapter | `haco-agent-host prepare/release` | v0.10 | repository coverage; real Agent Host acceptance pending |
| Base identity | `BaseName` / immutable `BaseRevision`; `haco image list`, `haco image inspect`, `haco create --base` | v0.11 | unit/fake-Incus coverage |
| Resource budgets | CPU / memory / PID / root-storage finite or explicit unlimited; Incus applies finite limits before start and verifies read-back | v0.12 | unit/fake-Incus coverage; real enforcement pending |
| Managed sandbox network | Hacocoon-managed Incus network/profile with broad/default fallback and drift handled fail-closed | v0.13 | unit/static integration; real Incus networking acceptance pending |
| Git fetch plugin | `haco plugin git fetch`; policy-scoped GitHub fetch using verified URL/refspec; HTTPS auth uses Host `gh auth git-credential` without exposing credentials to Sandbox | v0.14 | unit/CLI/real-git coverage |
| Optional OCI plugin | `HACO_PLUGIN_OCI=nerdctl|docker`; no OCI plugin is composed when unset | v0.15+ | driver/service tests; Core remains usable without container CLI dependencies |
| OCI usage / Seed recommendation | `haco plugin oci seed sample|recommend`; latest per-Environment snapshots, immutable digest recommendation, top-10% selection flag | v0.15 | unit/persistence coverage; real tool acceptance pending |
| OCI image deletion | `haco plugin oci image delete`; immutable reference+digest identity, deletion tombstones, optional all-Environment deletion, no force removal | v0.16 | adversarial/deletion tests |
| Docker compatibility | optional plugin-owned systemd socket/service packaging; genuine Docker CLI/Engine compatibility may use existing containerd without making dockerd the Hacocoon runtime | v0.17 | unit packaging verification; Base/Seed bake and real-host lifecycle pending |
| Optional Local OCI Registry | not required for normal pulls or Seed construction | v0.18 | planned |
| OCI Seed Builder / Btrfs COW | trusted Host acquisition → offline builder → immutable Incus Seed → normal clone/COW | v0.19 | planned |
| CI / release hardening | Go test/race/vet, docs checks, workflow/release checks and maintained `tools/ci-local.sh` | cross-cutting | repository/local checks; provider acceptance remains separate |

## Core vs optional plugin boundary

Core owns Workspace, Environment lifecycle, execution, client access primitives, Policy/Capability/Audit, Base identity and generic resource/network safety. It does **not** require nerdctl, Docker CLI, dockerd, a Host OCI cache, or a Local Registry.

OCI workload operations are explicitly opt-in:

```text
HACO_PLUGIN_OCI=nerdctl
# or
HACO_PLUGIN_OCI=docker

haco plugin oci status
haco plugin oci seed sample
haco plugin oci seed recommend
haco plugin oci image delete <reference>
```

Top-level `haco image list|inspect` continues to mean Hacocoon **Base image identity**, not workload container image management.

## Git credential boundary

`haco plugin git fetch` and `haco plugin git push` execute privileged Git operations on the trusted Host side. For GitHub HTTPS, the broker explicitly uses the Host-owned `gh auth git-credential` provider. PATs, helper plaintext, SSH private keys, and authorization headers are not copied into the Environment or Hacocoon audit state.

## Planned work

### v0.18 — Optional Local OCI Registry

A registry/proxy may be integrated where repeated upstream downloads, rate limits, or centralized policy justify it. It is not required for ordinary Environment pulls and is not required for Seed construction.

### v0.19 — OCI Seed Builder & Btrfs/COW

Planned flow:

```text
trusted Host image acquisition
        -> immutable digest
        -> OCI export/stream
        -> offline Seed Builder
        -> clean containerd stop
        -> immutable Incus Seed
        -> normal Incus clone
```

Writable `/var/lib/containerd` state is never shared between Environments; physical block sharing is left to Incus/storage-driver COW semantics.

## Future client interaction

Browser/Web notification and richer Interaction API work remain future client/adapter functionality. A VS Code extension may surface notifications, but it is optional and must not become a Core transport dependency.

## Acceptance boundary

Unit tests, fake-provider E2E, race/vet/build, repository CI and local CI do not substitute for real Incus, Windows/WSL, VS Code, AWS, containerd/nerdctl/Docker or Btrfs acceptance. Those host-dependent checks are reported separately.
