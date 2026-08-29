# Versioning and Release Status

**Status:** authoritative numbering/status note  
**Date:** 2026-08-30  
**Compatibility:** Hacocoon is pre-1.0; versioned design gates and concrete interfaces may still change incompatibly.

## Purpose

This document is the source of truth for Hacocoon roadmap version numbers. It exists to keep the public version sequence understandable when implementation and design work happen in parallel.

Use it together with:

- `00_REBASELINE_AND_ROADMAP.md` for product-boundary and roadmap intent;
- `IMPLEMENTATION_STATUS.md` for code that is actually present on `main`;
- each versioned specification for the detailed acceptance contract.

## Numbering policy

For pre-1.0 Hacocoon, version numbers should primarily describe the order in which product gates become real, user-visible implementation milestones.

The rules are:

1. implemented milestones should remain contiguous whenever renumbering is still cheap;
2. an implementation-pending design gate must not occupy a lower number while an already-implemented independent gate is forced above it;
3. active unmerged integration work follows the last implemented milestone;
4. design-only gates follow active implementation work;
5. security/hardening work normally does not consume a product version number;
6. `IMPLEMENTATION_STATUS.md` remains authoritative for implementation reality;
7. tags/releases remain separate from roadmap-gate numbering.

This intentionally replaces the earlier rule that treated every design assignment committed to `main` as permanently fixed. While Hacocoon is pre-1.0, clarity is more valuable than preserving an awkward numbering accident.

## Current authoritative numbering

| Version | Gate | Main status | Notes |
|---|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | implemented | external Workspace -> Incus Environment -> exec/shell -> delete |
| v0.2 | Workspace Abstraction & Lease | implemented | canonical Workspace identity, RO/RW leases, concurrency safety |
| v0.3 | Client & Interactive Access | implemented | status, loopback forwarding, SSH lifecycle |
| v0.4 | Policy & Capability Foundation | implemented | fail-closed policy, approval, audit |
| v0.5 | Git / GitHub Capability | implemented | brokered GitHub authority without exporting host credentials |
| v0.6 | Agent & Orchestrator Integration | implemented | `haco run`, machine output, external security events |
| v0.7 | Remote / Cloud Runtime & External Capabilities | implemented experimentally | EC2 remains explicit opt-in; real AWS acceptance pending |
| v0.8 | Client Adapters & VS Code Integration | implemented | `haco-vscode`, dedicated Windows/WSL path; real client acceptance pending |
| v0.9 | Per-Agent Sandbox & Agent Host Integration | broker foundation implemented | trusted session -> Environment binding; real Agent Host/AHP routing acceptance pending |
| v0.10 | VS Code Remote Agent Host Adapter | active integration candidate | PR #111; not part of `main` code reality until merged |
| v0.11 | Base Images & Custom Environments | design contract only | implementation pending |
| v0.12 | Sandbox Resource Limits | design contract only | CPU/memory/PID/root-storage budget contract; implementation pending |

The implemented progression is therefore contiguous through **v0.9**. v0.10 is the next active implementation gate, while v0.11 and v0.12 are intentionally kept behind it until their implementations land.

## Renumbering applied on 2026-08-30

The previous assignment was:

```text
v0.9   Base Images & Custom Environments             design only
v0.10  Per-Agent Sandbox & Agent Host Integration    implemented
v0.11  Sandbox Resource Limits                       design only
v0.12  VS Code Remote Agent Host Adapter             active PR
```

That ordering made the version sequence misleading. It is replaced by:

```text
v0.9   Per-Agent Sandbox & Agent Host Integration    implemented
v0.10  VS Code Remote Agent Host Adapter             active PR
v0.11  Base Images & Custom Environments             design only
v0.12  Sandbox Resource Limits                       design only
```

This renumbering changes roadmap/documentation labels only. It does not rewrite Git history or claim new implementation work.

## Migration notes

The maintained documentation surface must use the new numbers consistently:

- former `v0.10 Per-Agent Sandbox` references become **v0.9**;
- PR #111 / former `v0.12 Agent Host Adapter` references become **v0.10**;
- former `v0.9 Base Images` references become **v0.11**;
- former `v0.11 Resource Limits` references become **v0.12**.

Historical commit messages, closed PR titles, and already-existing temporary candidate branch names may retain their original text as history. They are not authoritative for current numbering.

## Current integration watch list

- PR #111: Agent Host adapter — **v0.10** under the current numbering; real Windows/WSL + Incus + VS Code Agents-window acceptance remains environment-dependent.
- PR #113: minimum patched Go toolchain hardening — no roadmap version assignment required.
- PR #114: Incus delete/absence verification hardening — no roadmap version assignment required.

## One-sentence rule

> **Keep implemented Hacocoon milestones contiguous, put active implementation next, and move design-only gates behind them while `IMPLEMENTATION_STATUS.md` separately records what is actually implemented.**
