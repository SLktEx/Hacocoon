# Versioning and Release Status

**Status:** authoritative numbering/status note  
**Date:** 2026-08-29  
**Compatibility:** Hacocoon is pre-1.0; versioned design gates and concrete interfaces may still change incompatibly.

## Purpose

This document prevents parallel design/implementation work from accidentally assigning the same Hacocoon version number to different gates.

Use it together with:

- `00_REBASELINE_AND_ROADMAP.md` for product-boundary and roadmap intent;
- `IMPLEMENTATION_STATUS.md` for code that is actually present on `main`;
- each versioned specification for the detailed acceptance contract.

When an open branch or pull request disagrees with this document and `main`, the branch/PR must be renumbered or rebased before merge. An open PR does not reserve or redefine a version number that is already assigned on `main`.

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
| v0.9 | Base Images & Custom Environments | design contract only | implementation remains pending |
| v0.10 | Per-Agent Sandbox & Agent Host Integration | broker foundation implemented | trusted session -> Environment binding; real Agent Host/AHP routing acceptance pending |
| v0.11 | Sandbox Resource Limits | design contract only | CPU/memory/PID/root-storage budget contract; implementation pending |
| v0.12 | VS Code Remote Agent Host Adapter | active integration candidate | reserved for the Agent Host remote-SSH adapter currently developed in PR #111; not part of `main` code reality until merged |

Version numbers describe roadmap gates, not a promise that all lower-numbered gates are fully implemented before higher-numbered additive work lands. The existing v0.9/v0.10 ordering is the explicit example: v0.9 remains design-only while the independent v0.10 broker foundation is already implemented.

## Resolved numbering collision: PR #111

PR #111 was originally opened as **`v0.11 VS Code Remote Agent Host Adapter`** while `main` independently assigned **v0.11** to **Sandbox Resource Limits**.

The authoritative resolution is:

```text
v0.10  Per-Agent Sandbox & Agent Host Integration
v0.11  Sandbox Resource Limits
v0.12  VS Code Remote Agent Host Adapter
```

The reason for keeping Resource Limits as v0.11 is operational rather than architectural preference: its specification and documentation consistency contract are already committed to `main`. The still-open Agent Host adapter branch is therefore the safer place to renumber.

Before PR #111 merges, its user-facing/versioned documentation must use v0.12 names and text. Code/package names such as `haco-agent-host` do not need a version suffix and are unaffected.

## Assignment rules for future parallel work

1. A version number becomes reserved when its roadmap/spec assignment is committed to `main`.
2. Open branches and PR titles do not override an assignment already present on `main`.
3. Before assigning a new version, search `main` documentation, open PRs, and active roadmap work for the proposed number.
4. If two parallel efforts collide, keep the number already committed to `main` unless there is a compelling migration reason; renumber the unmerged work.
5. Renumber the whole documentation surface together: filename, heading, prose references, roadmap/index links, docs consistency checks, PR title/body, and Japanese companion docs.
6. Do not infer implementation completeness from numbering. `IMPLEMENTATION_STATUS.md` remains the source of truth for code reality.
7. Security/hardening PRs that do not introduce a roadmap gate normally do not consume a product version number.
8. Tags/releases are separate from roadmap gate assignment. A design gate named v0.x does not by itself mean a release/tag with that version is ready.

## Current integration watch list

As of 2026-08-29:

- PR #111: Agent Host adapter — treat as **v0.12** before merge; real Windows/WSL + Incus + VS Code Agents-window acceptance remains environment-dependent.
- PR #113: minimum patched Go toolchain hardening — no roadmap version assignment required.
- PR #114: Incus delete/absence verification hardening — no roadmap version assignment required.

These entries are coordination notes, not substitutes for the PRs themselves. If a PR closes, is replaced, or materially changes scope, update this document only when the version/status summary would otherwise become misleading.

## One-sentence rule

> **`main` owns Hacocoon version-number assignments; parallel work must rebase onto that numbering before merge, while `IMPLEMENTATION_STATUS.md` separately owns the truth about what is actually implemented.**
