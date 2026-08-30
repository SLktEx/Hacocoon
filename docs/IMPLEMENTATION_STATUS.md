# Implementation Status

Status date: 2026-08-30, after the v0.13-v0.16 feature rebaseline and the initial v0.17 Docker compatibility foundation.

This file reports **current code reality**, not desired architecture and not a compatibility guarantee. Hacocoon is still **pre-1.0**.

The fully implemented product progression is currently contiguous through **v0.16**. v0.17 has a foundation but is not yet a complete feature gate. v0.18-v0.19 are planned.

| Area | Current repository reality | Milestone | Validation status |
|---|---|---:|---|
| Secure Workspace Runtime | `haco create --workspace`, `haco exec`, `haco shell`, `haco delete` over the Environment service | v0.1 | unit/process coverage; real Incus acceptance host-dependent |
| Workspace model / leases | canonical Workspace identity, RO/RW leases, conflict prevention, stale recovery, process serialization | v0.1-v0.2 | unit/persistence/concurrency/process coverage |
| Client access | status, loopback forwarding, SSH preparation/revocation | v0.3 | unit/process coverage; real Incus SSH acceptance pending |
| Policy / Capability | fail-closed policy, approval, request correlation, audit | v0.4 | unit/process/CLI E2E |
| Git / GitHub push capability | brokered push/force-with-lease without exporting reusable host credentials | v0.5 | unit/adversarial/real-git/CLI E2E |
| Agent / orchestrator integration | `haco run`, stable machine JSON, external security events | v0.6 | unit/race/process/CLI E2E |
| EC2 / AWS path | experimental EC2 Environment provider, AWS capability, EBS replacement/recovery | v0.7 | fake-AWS/process/E2E; real AWS acceptance pending |
| VS Code Client Adapter | `haco-vscode` + standard Remote-SSH; Windows/WSL bridge/bootstrap | v0.8 | helper/static coverage; real Windows/WSL acceptance pending |
| Per-agent sandbox broker | opaque session identity → persisted dedicated Environment binding | v0.9 | ownership/persistence/collision/release-proof tests |
| VS Code Remote Agent Host Adapter | `haco-agent-host prepare/release`, hashed aliases, loopback SSH, client-side private key, optional `code --agents` | v0.10 | implemented via PR #137; real Agent Host acceptance pending |
| Base identity / selection | provider-neutral Base identity, immutable Incus fingerprint pinning, `haco image list/inspect`, `create --base` | v0.11 | unit/routing/fake-Incus E2E; real image-source acceptance pending |
| Resource budgets | CPU/memory/PID/root finite-or-unlimited model, CLI flags, Incus pre-start apply/read-back | v0.12 | unit/fake-Incus E2E; real enforcement pending |
| Managed sandbox network | managed `haco-sandbox0` bridge, `haco-sandbox-egress` ACL substrate, `haco-sandbox` profile; drift/broad fallback fail closed | v0.13 | unit/static coverage; real Incus network acceptance pending |
| Git fetch plugin | `haco plugin git fetch`; host `gh auth git-credential`; fixed refspec; hostile repository Git config rejected | v0.14 | CLI/provider/security coverage; real private-repo combinations acceptance-sensitive |
| OCI Seed telemetry | latest image snapshot per Environment, 6h freshness guard, 30d window, immutable recommendation identity | v0.15 | repository tests cover persistence/sampling/recommendation |
| OCI Seed automatic promotion | deterministic top-10% `auto_promote` selection, minimum one eligible candidate | v0.15 | deterministic candidate-count/ranking tests |
| OCI image deletion | `haco image delete`, immutable target revalidation, Host cache deletion, tombstones, optional `--all-environments`, no forced removal | v0.16 | focused deletion/tombstone/retry tests |
| Docker compatibility | Docker compatibility design plus systemd socket/service packaging foundation | v0.17 | partial foundation only; full plugin lifecycle/integration pending |
| Optional Local OCI Registry | optional registry/proxy remains a design choice; normal direct upstream pulls remain valid | v0.18 | planned; not implemented as a feature gate |
| OCI Seed Builder / COW | offline builder, immutable Seed publication, current-Seed pointer, Incus/Btrfs COW integration | v0.19 | planned; physical build/publish not implemented |
| CI / release hardening | Go tests/vet/race, docs checks, bootstrap/release/workflow trust checks, maintained local CI runner | cross-cutting | repository checks; real-provider acceptance separate |

## Current implementation progression

```text
v0.1  secure workspace runtime
  -> v0.2 workspace leases
  -> v0.3 client access
  -> v0.4 policy/capability
  -> v0.5 GitHub push broker
  -> v0.6 agent/orchestrator surface
  -> v0.7 experimental remote/cloud runtime
  -> v0.8 VS Code client adapter
  -> v0.9 per-agent binding broker
  -> v0.10 VS Code Agent Host adapter
  -> v0.11 immutable Base selection
  -> v0.12 ResourceBudget
  -> v0.13 managed sandbox networking
  -> v0.14 Git fetch plugin
  -> v0.15 OCI Seed recommendation
  -> v0.16 OCI image deletion
  -> v0.17 Docker compatibility foundation (partial)
  -> v0.18 optional Local Registry (planned)
  -> v0.19 Seed Builder/COW (planned)
```

## Important boundaries

### v0.9 / v0.10 agent ownership

A deterministic Environment name is not ownership proof. `internal/agenthost` requires a matching persisted binding for acquire/release. Parallel write-capable sessions still need distinct canonical Workspaces, normally Git worktrees.

### v0.11 Bases

A logical Base resolves once at create time to an immutable revision. Existing Environments retain their exact persisted revision when an alias moves. Custom build/import/history/rollback/GC are separate follow-up capabilities.

### v0.12 ResourceBudget

Omitted dimensions resolve to explicit `unlimited`; finite requests are bounded and persisted. Incus applies and reads back finite limits before start. Unsupported providers fail before side effects rather than silently ignoring a requested finite limit.

### v0.13 network

Hacocoon owns a managed sandbox network/profile substrate and refuses silent fallback to broad/default Incus networking. Domain-aware authorization is not provided by Incus ACLs and remains a higher-layer broker/proxy/plugin concern.

### v0.14 Git fetch

Git fetch stays under `haco plugin git`. Reusable GitHub authentication remains on the trusted Host through `gh auth git-credential`; repository-controlled Git configuration is not allowed to redefine the privileged transport path.

### v0.15 / v0.16 OCI selection state

`haco image seed sample|recommend` observes immutable OCI identities and marks the top 10% for automatic future Seed inclusion. `haco image delete` can tombstone an exact identity so normal telemetry cannot silently re-promote it. These features do **not** imply that physical Seed build/publish exists yet.

### v0.17 Docker compatibility

The standard runtime direction remains containerd + nerdctl. Docker compatibility is optional and belongs behind a plugin/adapter boundary. The current repository has the systemd packaging/foundation, but full on-demand Engine/plugin lifecycle is not yet complete.

### v0.18 / v0.19 planned OCI infrastructure

A Local Registry is optional, not a prerequisite for ordinary `nerdctl pull` or OCI Seed. v0.19 will own trusted Host acquisition, offline Seed Builder import, immutable Incus Seed publication, revision pinning, and storage-driver COW behavior. Sharing one writable `/var/lib/containerd` across Environments is forbidden.

## Real-host acceptance

Unit tests, fake-provider E2Es, race checks, vet, build, script syntax, and repository CI are not substitutes for real-host acceptance.

Still host-dependent include real Incus lifecycle/network/resource enforcement, Windows/WSL + VS Code, Agent Host routing, real image sources, private-registry combinations, and AWS/EC2/SSM/EBS.

## Compatibility status

No milestone through v0.19 should be read as a promise that current CLI/API/state/config surfaces are frozen. Until an explicit stable compatibility milestone is declared, breaking changes may correct unsafe authority boundaries, ambiguous ownership, accidental provider coupling, or unnecessary complexity.
