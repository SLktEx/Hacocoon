# Architecture Rebaseline and Roadmap

**Status:** authoritative baseline  
**Date:** 2026-08-29

## Decision

Hacocoon is a **Secure Workspace Runtime**.

It receives a Workspace from a human, IDE, shell, or external orchestrator; places that Workspace inside an isolated Environment; executes tools there; and later mediates privileged capabilities such as GitHub or cloud access.

It is **not** the owner of IDE workflow, Git branch/worktree orchestration, AI task DAGs, model routing, retry strategy, or model budgets.

```text
VS Code / Shell / Daintree / Rookery / other clients
                         |
                    Workspace
                         v
                +----------------+
                |    Hacocoon    |
                | Secure         |
                | Workspace      |
                | Runtime        |
                +--+----------+--+
                   |          |
              Environment   Policy/Capability
                   |
                 Incus
```

## Hacocoon responsibilities

Hacocoon owns:

- accepting/resolving a Workspace;
- binding a Workspace to an isolated Environment;
- Environment lifecycle and cleanup;
- command/interactive execution;
- later, Workspace lease/ownership safety;
- later, policy evaluation and security approval;
- later, scoped external-service capabilities;
- stable boundaries that clients/orchestrators can call.

Hacocoon does not own:

- Codex vs Claude selection;
- task decomposition or agent DAGs;
- model/token budgets;
- development-review queues;
- branch strategy or worktree orchestration as a Core concern;
- a proprietary IDE/editor/chat UI;
- cloud/provider/storage logic inside Core.

## Core concepts

The long-term Core vocabulary is intentionally small:

```text
Workspace
WorkspaceLease
Environment
Execution
CapabilityRequest
PolicyDecision
ApprovalRequest
```

Concrete technologies stay behind adapters/ports. Core must not depend directly on Incus, Git, GitHub, AWS, VS Code, Daintree, Rookery, Btrfs, QCOW2, or EC2.

## Workspace and worktree boundary

A Workspace is opaque to Hacocoon Core. It may be:

- an ordinary directory;
- a Git repository;
- a Git worktree produced by Daintree/Rookery;
- a worktree produced by an optional Hacocoon WorkspaceProvider.

External ownership:

```text
Daintree/Rookery -> create worktree -> path -> Hacocoon
```

Standalone ownership:

```text
Human/CLI -> optional GitWorktreeWorkspace -> Hacocoon
```

Both paths must converge on the same Workspace/Environment runtime path.

## Human-in-the-loop split

There are two different approval responsibilities:

```text
Development approval -> Human / GitHub / Daintree / Rookery
Security approval    -> Hacocoon Policy/Capability boundary
```

Hacocoon security approval covers privileged authority such as credential issuance, protected GitHub operations, sensitive port exposure, AWS/API access, or runtime privilege changes.

## New release order

The previous v0.1-v0.7 sequence is superseded by this order:

| Version | Gate | Purpose |
|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | external path -> Incus -> exec/shell -> delete |
| v0.2 | Workspace Abstraction & Lease | formal workspace providers/leases; optional worktree provider |
| v0.3 | Client & Interactive Access | VS Code/SSH/code-server/ports without IDE ownership |
| v0.4 | Policy & Capability Foundation | allow/deny/require-approval and audit boundary |
| v0.5 | Git / GitHub Capability | scoped Git/GitHub authority without broad ambient credentials |
| v0.6 | Agent & Orchestrator Integration | Codex/Claude/Daintree/Rookery/MCP integration above generic execution |
| v0.7 | Remote / Cloud Runtime & External Capabilities | AWS, EC2 EnvironmentProvider, EBS and remote-runtime concerns |

## v0.1 scope freeze

v0.1 proves one vertical slice only:

```text
host directory
  -> haco create --workspace
  -> Incus system container
  -> haco exec / haco shell
  -> haco delete
```

v0.1 includes:

- Go CLI;
- external path Workspace;
- Incus Environment lifecycle;
- read/write workspace mount;
- command execution;
- interactive shell;
- minimal state/cleanup;
- unit tests plus a real-Incus integration path.

v0.1 explicitly excludes:

- Hacocoon-owned worktree orchestration;
- Policy/Capability engine;
- GitHub/AWS;
- AI-specific agent management;
- MCP;
- VS Code extension/Web UI;
- advanced Btrfs/loop/QCOW2 storage lifecycle;
- EC2/EBS;
- speculative plugin frameworks.

Historical code for later areas may remain in the repository but must not expand the v0.1 acceptance gate.

## v0.1 implementation order

1. Minimal `Workspace`, `Environment`, and `ExecutionResult` concepts.
2. Thin Incus CLI adapter.
3. `haco create --workspace`.
4. `haco exec`.
5. `haco shell`.
6. `haco delete` and cleanup.
7. Real Incus integration acceptance.
8. Scope freeze and v0.1 alpha tag.

## Cooperation with external orchestrators

### Daintree

Daintree may own task/worktree/agent supervision. Hacocoon receives the resulting workspace path and executes the chosen agent/tool inside an isolated Environment.

### Rookery

Rookery may own workers, budgets, worktrees, and development attention queues. Hacocoon can later expose a CLI/MCP surface for secure Environment and Capability operations.

### VS Code

VS Code can open an ordinary repository or worktree and use Hacocoon as the underlying Environment runtime. VS Code remains a client, not a Core dependency.

## Old-to-new placement

| Historical idea | New placement |
|---|---|
| worktree management | optional WorkspaceProvider, v0.2 |
| VS Code / code-server | Client integration, v0.3 |
| authorization / approval | Policy + Capability, v0.4 |
| Git push / GitHub authority | GitHub Capability, v0.5 |
| AI agent integration | generic execution + external orchestrator integration, v0.6 |
| model routing / task DAG / budgets | outside Hacocoon |
| AWS / EC2 / EBS | v0.7 provider/capability work |
| Btrfs / QCOW2 / storage mechanics | adapter detail when actually needed |

## Design principles

- Keep Core small.
- Prefer clear responsibility boundaries over premature common abstractions.
- Treat the Workspace as opaque to the runtime.
- Keep OS/Incus/process side effects in a narrow imperative shell.
- Human approval is a first-class security decision, not a UI patch.
- Do not hand long-lived parent credentials to untrusted executed tools.
- Hacocoon must remain usable from multiple clients and orchestrators.
- Finish the current release gate before designing the next one into the implementation.

## One-sentence definition

> **Hacocoon is a secure workspace runtime that runs developer tools and AI agents inside isolated environments without owning the IDE, Git workflow, or AI orchestration layer.**
