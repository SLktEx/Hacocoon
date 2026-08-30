# Versioning and Release Status

**Status:** authoritative numbering/status note  
**Date:** 2026-08-30  
**Compatibility:** Hacocoon is pre-1.0; versioned design gates and concrete interfaces may still change incompatibly.

## Purpose

This document is the source of truth for Hacocoon roadmap version numbers. Use it together with `00_REBASELINE_AND_ROADMAP.md` for product intent and `IMPLEMENTATION_STATUS.md` for code that is actually present on `main`.

## Numbering policy

For pre-1.0 Hacocoon, version numbers primarily describe the order in which product gates become real, user-visible implementation milestones.

1. implemented milestones should remain contiguous whenever renumbering is still cheap;
2. an implementation-pending design gate must not occupy a lower number while an already-implemented independent gate is forced above it;
3. active unmerged integration work follows the last implemented milestone;
4. design-only gates follow active implementation work;
5. security/hardening work normally does not consume a product version number;
6. `IMPLEMENTATION_STATUS.md` remains authoritative for implementation reality;
7. tags/releases remain separate from roadmap-gate numbering.

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
| v0.10 | VS Code Remote Agent Host Adapter | implemented | merged in PR #137; real Windows/WSL + Incus + VS Code Agents-window acceptance remains host-dependent |
| v0.11 | Base Images & Custom Environments | implemented first slice | logical Base selection, immutable revision pinning, persisted identity, list/inspect; broader build/import/GC remains future work |
| v0.12 | Sandbox Resource Limits | implemented first slice | provider-neutral finite/unlimited budgets, strict CLI parsing, Incus pre-start enforcement, persistence/status; real Incus enforcement acceptance remains host-dependent |

The implemented progression is therefore contiguous through **v0.12**.

## Renumbering applied on 2026-08-30

The earlier assignment temporarily placed design-only Base Images before implemented per-agent work. That ordering was replaced with the current sequence:

```text
v0.9   Per-Agent Sandbox & Agent Host Integration    implemented
v0.10  VS Code Remote Agent Host Adapter             implemented
v0.11  Base Images & Custom Environments             implemented first slice
v0.12  Sandbox Resource Limits                       implemented first slice
```

Historical commit messages, closed PR titles, and temporary candidate branch names may retain older labels as history. They are not authoritative for current numbering.

## Current acceptance watch list

- v0.8: real Windows/WSL + Incus + VS Code Remote-SSH acceptance remains pending.
- v0.9/v0.10: real VS Code Agent Host/AHP routing and real Incus SSH acceptance remain host-dependent.
- v0.11: real Incus image-remote/custom-Base acceptance remains host-dependent; custom build/import/history/rollback/GC are not part of the first implemented slice.
- v0.12: real supported-Incus validation of CPU, memory, PID, and root-disk enforcement remains host-dependent. The experimental EC2 provider rejects finite budgets rather than silently ignoring them.
- v0.7 EC2: real AWS acceptance remains pending and the provider stays experimental/default-off.

## One-sentence rule

> **Keep implemented Hacocoon milestones contiguous and use `IMPLEMENTATION_STATUS.md` to distinguish repository implementation from real-host/provider acceptance.**
