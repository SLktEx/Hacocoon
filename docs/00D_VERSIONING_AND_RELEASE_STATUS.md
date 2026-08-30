# Versioning & Release Status

> **Authoritative milestone numbering · Updated 2026-08-30**

Hacocoon is **pre-1.0**. Milestone numbers describe product/implementation progression; they are not compatibility guarantees, release tags, or proof of production support.

Use this document for **which version number belongs to which gate**. Use [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md) for exact code reality and host-dependent acceptance.

## Numbering policy

1. Implemented milestones should remain contiguous while renumbering is still cheap.
2. A design-only gate must not force already-implemented independent work to appear later in the sequence.
3. Security/hardening work normally does not consume a product version number by itself.
4. Optional integrations must not be promoted into Core requirements merely because the project ships a preferred profile.
5. Tags/releases are separate from roadmap-gate numbering.
6. `IMPLEMENTATION_STATUS.md` remains authoritative for repository implementation reality.

## Current authoritative numbering

**Status legend:** ✅ implemented · 🧪 experimental · 🚧 planned

| Version | Gate | Main status | Notes |
|---|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | ✅ implemented | external Workspace → Incus Environment → exec/shell → delete |
| v0.2 | Workspace Abstraction & Lease | ✅ implemented | canonical Workspace identity, RO/RW leases, concurrency safety |
| v0.3 | Client & Interactive Access | ✅ implemented | status, loopback forwarding, SSH lifecycle |
| v0.4 | Policy & Capability Foundation | ✅ implemented | fail-closed policy, approval, audit |
| v0.5 | Git / GitHub Capability | ✅ implemented | brokered GitHub authority without exporting host credentials |
| v0.6 | Agent & Orchestrator Integration | ✅ implemented | `haco run`, machine output, external security events |
| v0.7 | Remote / Cloud Runtime & External Capabilities | 🧪 implemented experimentally | EC2 stays explicit opt-in; real AWS acceptance pending |
| v0.8 | Client Adapters & VS Code Integration | ✅ implemented | `haco-vscode`; real client acceptance pending |
| v0.9 | Per-Agent Sandbox & Agent Host Integration | ✅ broker foundation implemented | trusted session → Environment binding; real Agent Host/AHP routing acceptance pending |
| v0.10 | VS Code Remote Agent Host Adapter | ✅ implemented | real Windows/WSL + Incus + VS Code Agents-window acceptance remains host-dependent |
| v0.11 | Base Images & Custom Environments | ✅ implemented first slice | logical Base selection, immutable revision pinning, persisted identity, list/inspect; build/import/history/GC remain follow-up work |
| v0.12 | Sandbox Resource Limits | ✅ implemented first slice | provider-neutral finite/unlimited budgets and Incus pre-start enforcement; real workload enforcement acceptance remains host-dependent |
| v0.13 | Optional OCI Plugin | ✅ implemented first slice | opt-in nerdctl/Docker inventory drivers and Seed recommendation foundation; no container-tool Core dependency |
| v0.13A | OCI Seed Build & Btrfs/COW Optimization | 🚧 planned follow-up | immutable Seed build/publish and real storage acceptance remain pending |
| v0.13B | OCI Seed Automatic Promotion Policy | ✅ selection policy implemented | top eligible recommendations are marked for next-Seed inclusion; build consumption remains pending |
| v0.13C | OCI Image Deletion | ✅ implemented first slice | plugin-owned deletion/tombstones; replacement Seed publish and old-Seed GC remain pending |

The implemented progression is contiguous through the **v0.13 optional-plugin first slice**. The next work is not to make OCI mandatory; it is to complete optional Seed build/publish and real-host acceptance behind the plugin boundary.

## Implemented vs planned

```text
implemented Core/product progression
v0.1 ───────────────────────────── v0.12
                                      |
                                      v
                         optional OCI plugin first slice
                                   v0.13
                              /       |       \
                             v        v        v
                 Seed build/COW   promotion   deletion
                    v0.13A         v0.13B      v0.13C
```

The existence of OCI specifications does not make OCI tooling part of Core. With `HACO_PLUGIN_OCI` unset, Hacocoon must not require `containerd`, `nerdctl`, Docker CLI, or Docker Engine.

## v0.13 plugin contract

The user-facing plugin boundary is:

```text
HACO_PLUGIN_OCI=nerdctl
HACO_PLUGIN_OCI=docker

haco plugin oci status
haco plugin oci seed sample
haco plugin oci seed recommend
haco plugin oci image delete <reference[@digest]>
```

The project-maintained `containerd + nerdctl` arrangement is an optional profile. Docker CLI/Engine compatibility is also optional. A Hacocoon Core installation with no OCI tooling remains valid.

## Renumbering history

The 2026-08-30 cleanup replaced a temporary ordering that placed design-only Base work ahead of already-implemented per-agent work. The current sequence is:

```text
v0.9   Per-Agent Sandbox & Agent Host Integration    implemented
v0.10  VS Code Remote Agent Host Adapter             implemented
v0.11  Base Images & Custom Environments             implemented first slice
v0.12  Sandbox Resource Limits                       implemented first slice
v0.13  Optional OCI Plugin                           implemented first slice
```

Earlier documents and commits that describe v0.13 as a mandatory Local OCI Registry or describe `containerd + nerdctl` as the Hacocoon runtime are superseded by the plugin boundary documented in `00A_PLUGIN_ARCHITECTURE.md` and `OCI_RUNTIME_AND_DOCKER_COMPAT.md`.

Historical commit messages, closed PR titles, candidate branches, or superseded planning notes may keep older labels. They are historical records, not current numbering.

## Acceptance watch list

Implementation and real-host acceptance are tracked separately.

- **v0.7:** real AWS/EC2/SSM/EBS acceptance remains pending; EC2 stays experimental/default-off.
- **v0.8:** real Windows/WSL + Incus + VS Code Remote-SSH acceptance remains pending.
- **v0.9/v0.10:** real VS Code Agent Host/AHP routing and Incus SSH acceptance remain host-dependent.
- **v0.11:** real Incus image-source/custom-Base acceptance remains host-dependent; build/import/history/rollback/GC are not part of the first slice.
- **v0.12:** real supported-Incus CPU/memory/PID/root-storage enforcement remains host-dependent.
- **v0.13:** repository plugin boundary, driver selection, telemetry, and recommendation logic are implemented; real OCI-profile/container-tool acceptance remains pending.
- **v0.13A:** Seed build/publish and real Btrfs/COW acceptance remain planned.
- **v0.13B:** selection policy is implemented; actual consumption by the future Seed Builder remains planned.
- **v0.13C:** deletion/tombstone logic is implemented; replacement Seed publication and old-Seed GC remain pending.

## Rule of thumb

> **Use this file for numbering, `IMPLEMENTATION_STATUS.md` for code reality, and the roadmap/specifications for intent. Optional tooling stays optional even when Hacocoon ships a preferred profile.**
