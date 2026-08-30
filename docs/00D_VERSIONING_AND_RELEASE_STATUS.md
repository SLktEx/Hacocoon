# Versioning & Release Status

> **Authoritative milestone numbering · Updated 2026-08-30**

Hacocoon is **pre-1.0**. Milestone numbers describe product/implementation progression; they are not compatibility guarantees, release tags, or proof of production support.

Use this document for **which version number belongs to which gate**. Use [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md) for exact code reality and host-dependent acceptance.

## Numbering policy

1. Implemented milestones should remain contiguous while renumbering is still cheap.
2. A design-only gate must not force already-implemented independent work to appear later in the sequence.
3. Security/hardening work normally does not consume a product version number by itself.
4. A planned specification may reserve the next milestone, but it must be labeled **planned / not implemented** until code lands.
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
| v0.10 | VS Code Remote Agent Host Adapter | ✅ implemented | merged in PR #137; real Windows/WSL + Incus + VS Code Agents-window acceptance remains host-dependent |
| v0.11 | Base Images & Custom Environments | ✅ implemented first slice | logical Base selection, immutable revision pinning, persisted identity, list/inspect; build/import/history/GC remain follow-up work |
| v0.12 | Sandbox Resource Limits | ✅ implemented first slice | provider-neutral finite/unlimited budgets and Incus pre-start enforcement; real workload enforcement acceptance remains host-dependent |
| v0.13 | Local OCI Registry | 🚧 planned | design contract exists; not implemented on `main` |
| v0.13A | OCI Seed & Btrfs/COW Optimization | 🚧 planned second slice | companion optimization after the v0.13 registry path; not implemented on `main` |

The implemented progression is therefore contiguous through **v0.12**. **v0.13 is the next planned milestone**, not current repository implementation.

## Implemented vs planned

```text
implemented on main
v0.1 ───────────────────────────── v0.12
                                      |
                                      v
                                next planned
                                   v0.13
                                      |
                                      v
                              planned second slice
                                  v0.13A
```

The existence of `13_v0.13_LOCAL_OCI_REGISTRY.md` or `13A_v0.13_OCI_SEED_AND_COW.md` does not mean those features are already present.

## Renumbering history

The 2026-08-30 cleanup replaced a temporary ordering that placed design-only Base work ahead of already-implemented per-agent work. The current sequence is:

```text
v0.9   Per-Agent Sandbox & Agent Host Integration    implemented
v0.10  VS Code Remote Agent Host Adapter             implemented
v0.11  Base Images & Custom Environments             implemented first slice
v0.12  Sandbox Resource Limits                       implemented first slice
v0.13  Local OCI Registry                            planned
```

Historical commit messages, closed PR titles, candidate branches, or superseded planning notes may keep older labels. They are historical records, not current numbering.

## Acceptance watch list

Implementation and real-host acceptance are tracked separately.

- **v0.7:** real AWS/EC2/SSM/EBS acceptance remains pending; EC2 stays experimental/default-off.
- **v0.8:** real Windows/WSL + Incus + VS Code Remote-SSH acceptance remains pending.
- **v0.9/v0.10:** real VS Code Agent Host/AHP routing and Incus SSH acceptance remain host-dependent.
- **v0.11:** real Incus image-source/custom-Base acceptance remains host-dependent; build/import/history/rollback/GC are not part of the first slice.
- **v0.12:** real supported-Incus CPU/memory/PID/root-storage enforcement remains host-dependent.
- **v0.13/v0.13A:** planned only; implementation and acceptance have not started merely because the specifications exist.

## Rule of thumb

> **Use this file for numbering, `IMPLEMENTATION_STATUS.md` for code reality, and the roadmap/specifications for intent.**
