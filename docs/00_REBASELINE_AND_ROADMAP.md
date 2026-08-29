# Architecture Rebaseline and Roadmap

**Status:** authoritative architecture baseline  
**Date:** 2026-08-29  
**Implementation note:** `main` has progressed through the v0.7 implementation pass; v0.8 adds the first thin client adapter for VS Code while preserving the same Core boundary. See `IMPLEMENTATION_STATUS.md` for current code reality and pending real-provider acceptance.

## Decision

Hacocoon is a **Secure Workspace Runtime**.

It receives a Workspace from a human, IDE, shell, or external orchestrator; places that Workspace inside an isolated Environment; executes tools there; and mediates privileged capabilities such as GitHub or cloud access without handing broad parent authority to the Environment.

It is **not** the owner of IDE workflow, Git branch/worktree orchestration, AI task DAGs, model routing, retry strategy, model budgets, or AI chat UX.

```text
VS Code / Shell / coding agents / orchestrators / other clients
                              |
                    optional Client Adapter
                              |
                         Workspace
                              v
                    +-------------------+
                    |     Hacocoon      |
                    | Secure Workspace  |
                    | Runtime           |
                    +---------+---------+
                              |
              +---------------+----------------+
              |                                |
         Environment                    Policy/Capability
              |
      Environment provider
         /           \
 runtime.incus   runtime.ec2
 local default  experimental
```

## Hacocoon responsibilities

Hacocoon owns:

- accepting/resolving a Workspace;
- binding a Workspace to an isolated Environment;
- Environment lifecycle and cleanup;
- command and interactive execution;
- Workspace lease/ownership safety;
- policy evaluation and security approval;
- scoped external-service capabilities;
- stable conceptual boundaries that clients and orchestrators can call;
- audit/recovery behavior for authority-sensitive operations.

Hacocoon does not own:

- Codex vs Claude selection;
- task decomposition or agent DAGs;
- model/token budgets;
- development-review queues;
- branch strategy or worktree orchestration as a Core concern;
- a proprietary IDE/editor/chat UI;
- cloud/provider/storage logic inside Core.

Thin client adapters may live around Hacocoon without changing that ownership. They translate generic Environment/client-access information into client-native configuration and launch/reconnect behavior.

## Core concepts

The Core vocabulary is intentionally small:

```text
Workspace
WorkspaceLease
Environment
Execution
CapabilityRequest
PolicyDecision
ApprovalRequest
```

Concrete technologies stay behind adapters/ports. Core must not depend directly on Incus, Git, GitHub, AWS, VS Code, Daintree, Rookery, Btrfs, QCOW2, EC2, EBS, or other provider-specific concepts.

## Workspace and worktree boundary

A Workspace is opaque to Hacocoon Core. It may be:

- an ordinary directory;
- a Git repository;
- a Git worktree produced by an external tool;
- a workspace produced by an optional Hacocoon WorkspaceProvider.

External ownership:

```text
external orchestrator -> create/select workspace -> path -> Hacocoon
```

Standalone ownership:

```text
Human/CLI -> optional WorkspaceProvider -> Workspace -> Hacocoon
```

Both paths converge on the same Workspace/Environment runtime path.

## Human-in-the-loop split

There are two different approval responsibilities:

```text
Development approval -> Human / GitHub / external orchestrator
Security approval    -> Hacocoon Policy/Capability boundary
```

Hacocoon security approval covers privileged authority such as protected Git operations, sensitive exposure, AWS/API access, or runtime privilege changes. It is not a general development workflow engine.

## Baseline roadmap progression

The 2026-08-29 rebaseline established v0.1-v0.7. The explicit v0.8 decision adds client adapters without moving client-specific behavior into Core.

| Version | Gate | Purpose | Repository state |
|---|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | external path -> Incus -> exec/shell -> delete | implemented; real Incus acceptance remains environment-dependent |
| v0.2 | Workspace Abstraction & Lease | formal workspace identity/leases and concurrency safety | implemented |
| v0.3 | Client & Interactive Access | status/SSH/ports/client integration without IDE ownership | implemented |
| v0.4 | Policy & Capability Foundation | allow/deny/require-approval and audit boundary | implemented |
| v0.5 | Git / GitHub Capability | scoped Git/GitHub authority without broad ambient credentials | implemented |
| v0.6 | Agent & Orchestrator Integration | generic machine/agent integration above secure execution | implemented |
| v0.7 | Remote / Cloud Runtime & External Capabilities | AWS capability plus remote provider work | implemented experimentally; real AWS acceptance pending |
| v0.8 | Client Adapters & VS Code Integration | thin client adapter layer; VS Code Remote-SSH first | implementation introduced; real VS Code + Incus acceptance remains environment-dependent |

These rows describe the implementation progression, **not compatibility guarantees**. Hacocoon remains pre-1.0 and may change CLI, state formats, APIs, capabilities, adapters, and configuration incompatibly.

## v0.1 baseline gate record

v0.1 established the first vertical slice:

```text
host directory
  -> haco create --workspace
  -> Incus system container
  -> haco exec / haco shell
  -> haco delete
```

Its scope intentionally excluded policy, GitHub/AWS authority, agent orchestration, advanced storage, and cloud runtime. Those boundaries were introduced by later roadmap stages instead of being pre-built into v0.1.

This section is retained as a historical design constraint. It is **not** an instruction to remove later v0.2-v0.8 functionality now present on `main`.

## Cooperation with external orchestrators

External orchestrators may own tasks, worktrees, workers, budgets, retries, and development review. Hacocoon receives the resulting Workspace and provides the secure Environment and Capability boundary underneath them.

Daintree and similar tools therefore sit above Hacocoon rather than inside Core:

```text
Daintree / other orchestrator
          |
   task + worktree ownership
          |
       Workspace
          |
      Hacocoon
          |
      Environment
```

## Client adapters

A Client Adapter is a thin integration layer outside Core. It may create/select an Environment, request standard access such as loopback SSH, translate connection information into client configuration, and launch/reconnect the client.

The first adapter is `haco-vscode`.

```text
VS Code
  -> haco-vscode
  -> Hacocoon Environment + SSH access
  -> standard Remote-SSH
  -> /workspace
```

The adapter does not implement an editor, terminal, debugger, Git UI, or AI chat surface. Those remain VS Code responsibilities.

The intended AI workflow is permissive inside the isolated Environment while keeping external authority mediated:

```text
VS Code AI / coding agent
        |
   Incus Environment
    broad local freedom
        |
--- Hacocoon boundary ---
        |
Policy / Capability / Audit
        |
GitHub / AWS / Host
```

### VS Code and other IDEs

VS Code can use Hacocoon as the underlying Environment runtime while remaining a client, not a Core dependency. `haco-vscode` is a convenience adapter around standard Remote-SSH.

Other IDEs should consume the same generic client/environment boundary rather than causing IDE-specific branching inside Core. Future adapters may include JetBrains, web clients, Daintree, or other tools, but those adapters are not Core concepts.

## Responsibility placement

| Concern | Placement |
|---|---|
| worktree management | outside Core; optional WorkspaceProvider boundary |
| VS Code / code-server / IDE access | client integration / Client Adapter |
| client-native SSH configuration and launch | adapter-owned, outside Core |
| AI chat UI / model selection | IDE/agent/orchestrator, outside Hacocoon |
| authorization / approval | Policy + Capability |
| Git push / GitHub authority | Git/GitHub capability boundary |
| AI agent integration | generic execution + external orchestrator integration |
| model routing / task DAG / budgets | outside Hacocoon |
| AWS / EC2 / EBS | provider/capability adapters; EC2 remains experimental |
| Btrfs / QCOW2 / storage mechanics | provider/adapter detail only when actually required |

## Experimental EC2 rule

The EC2 Environment provider is **experimental and disabled by default**.

It must never become active merely because AWS credentials, AWS environment variables, the AWS CLI, or cloud metadata are present. Enabling it requires explicit Hacocoon-owned operator opt-in. Disabling the experimental gate must fail closed before real provider initialization or AWS activity while preserving recoverable remote state.

See `07_v0.7_REMOTE_AND_CLOUD_RUNTIME.md` and `REMOTE_CLOUD_PROVISIONING.md` for the detailed contract.

## Pre-1.0 compatibility rule

Breaking changes remain allowed while the architecture is being hardened.

Prefer a deliberate incompatible correction over preserving accidental behavior when the old behavior:

- leaks authority across a trust boundary;
- makes ownership ambiguous;
- creates unsafe cleanup/retry semantics;
- forces provider-specific concepts into Core;
- preserves unnecessary complexity only for compatibility.

Breaking-change freedom should still be disciplined: document operator-visible impact, preserve data/recovery where possible, update tests, and provide migration guidance when a supported migration exists.

## Design principles

- Keep Core small.
- Prefer clear responsibility boundaries over premature common abstractions.
- Treat the Workspace as opaque to the runtime.
- Keep OS/Incus/process side effects in a narrow imperative shell/adaptor layer.
- Human approval is a first-class security decision, not a UI patch.
- Do not hand long-lived parent credentials to untrusted executed tools.
- Hacocoon must remain usable from multiple clients and orchestrators.
- Let coding agents be permissive inside an isolated Environment without turning that into host authority.
- Client convenience adapters must translate protocols, not absorb IDE or orchestration ownership.
- Do not create abstractions solely for hypothetical future backends.
- Treat cleanup, retry, cancellation, concurrency, and partial failure as part of the feature.
- Keep implementation claims separate from real-provider acceptance claims.

## One-sentence definition

> **Hacocoon is a secure workspace runtime that runs developer tools and AI agents inside isolated environments without owning the IDE, Git workflow, or AI orchestration layer.**
