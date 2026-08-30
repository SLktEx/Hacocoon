# Versioning & Release Status

> **Authoritative milestone numbering · Updated 2026-08-30**

Hacocoon is **pre-1.0**. Milestone numbers describe product/implementation progression; they are not compatibility guarantees, release tags, or proof of production support.

Use this document for **which version number belongs to which gate**. Use [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md) for exact code reality and host-dependent acceptance.

## Numbering policy

1. Prefer one independently useful product slice per milestone while renumbering remains cheap.
2. Keep implemented milestones contiguous where practical.
3. Security/hardening normally does not consume a product version by itself.
4. Optional integrations remain optional; shipping a project-maintained profile does not make its tools Core dependencies.
5. Planned specifications are labeled planned until implementation lands.
6. Release tags and roadmap milestone numbers are separate.

## Current authoritative numbering

**Status legend:** ✅ implemented · 🧪 experimental · 🚧 planned

| Version | Gate | `main` status | Notes |
|---|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | ✅ implemented | Workspace → isolated Environment → exec/shell/delete |
| v0.2 | Workspace Abstraction & Lease | ✅ implemented | canonical identity, RO/RW leases, concurrency safety |
| v0.3 | Client & Interactive Access | ✅ implemented | status, loopback forwarding, SSH lifecycle |
| v0.4 | Policy & Capability Foundation | ✅ implemented | fail-closed policy, approval, audit |
| v0.5 | Git / GitHub Push Capability | ✅ implemented | brokered push without exporting host credentials |
| v0.6 | Agent & Orchestrator Integration | ✅ implemented | `haco run`, machine output, security events |
| v0.7 | Remote / Cloud Runtime | 🧪 experimental | EC2 is explicit opt-in; real AWS acceptance pending |
| v0.8 | Client Adapters & VS Code | ✅ implemented | `haco-vscode`; real client acceptance pending |
| v0.9 | Per-Agent Sandbox | ✅ implemented | trusted session → dedicated Environment binding |
| v0.10 | VS Code Remote Agent Host Adapter | ✅ implemented | `haco-agent-host`; real Agent Host acceptance pending |
| v0.11 | Base Images & Custom Environments | ✅ first slice | logical Base → immutable revision pinning |
| v0.12 | Sandbox Resource Limits | ✅ first slice | CPU/memory/PID/root-storage budgets |
| v0.13 | Managed Sandbox Network | ✅ implemented | Hacocoon-managed Incus network/profile, fail-closed drift behavior |
| v0.14 | Git Fetch Plugin | ✅ implemented | `haco plugin git fetch`; host `gh auth git-credential` for HTTPS GitHub auth |
| v0.15 | OCI Seed Usage & Recommendation | ✅ first slice | optional OCI plugin telemetry, recommendation and top-10% selection policy |
| v0.16 | OCI Image Deletion | ✅ first slice | optional OCI plugin deletion, immutable identity and tombstones |
| v0.17 | Docker Compatibility | ✅ packaging foundation | optional genuine Docker CLI / on-demand Engine compatibility; Base/Seed bake and real-host acceptance pending |
| v0.18 | Optional Local OCI Registry | 🚧 planned | optional infrastructure only; normal upstream pulls remain valid |
| v0.19 | OCI Seed Builder & Btrfs/COW | 🚧 planned | offline builder, immutable Seed publish, normal Incus clone/COW |

The implemented milestone progression is therefore contiguous through **v0.17**. v0.18 is the next planned product slice.

## Optional OCI boundary

OCI/container tooling is not a Hacocoon Core requirement. The optional plugin is selected explicitly:

```text
HACO_PLUGIN_OCI=nerdctl
HACO_PLUGIN_OCI=docker

haco plugin oci status
haco plugin oci seed sample
haco plugin oci seed recommend
haco plugin oci image delete <reference>
```

With `HACO_PLUGIN_OCI` unset, Core must not require nerdctl, Docker CLI, dockerd, a Host OCI cache, or a Local Registry merely to manage Hacocoon Environments.

`haco image list` / `haco image inspect` remain Core-facing **Base image identity** commands; workload OCI inventory/Seed operations live under `haco plugin oci`.

## Historical numbering

Older commits and documents may refer to Local OCI Registry as v0.13 or to Seed/COW as v0.13A/B/C. Those labels are superseded by this sequence. Historical commit messages and closed PR titles are not rewritten.

## Acceptance watch list

- v0.7: real AWS/EC2/SSM/EBS acceptance pending.
- v0.8-v0.10: real Windows/WSL + Incus + VS Code/Agent Host acceptance pending.
- v0.11-v0.13: real supported Incus image/resource/network acceptance remains host-dependent.
- v0.14: brokered Git fetch/push has repository tests; host credential behavior depends on configured `gh`/SSH auth.
- v0.15-v0.17: optional OCI plugin repository logic/packaging exists; real container-tool/Base integration remains separate acceptance.
- v0.18-v0.19: planned, not implementation claims.

## Rule of thumb

> **Numbering lives here, code reality lives in `IMPLEMENTATION_STATUS.md`, and optional tooling stays outside Core.**
