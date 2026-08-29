# Architecture Rebaseline and Roadmap

**Status:** authoritative architecture baseline  
**Date:** 2026-08-29  
**Implementation note:** `main` has progressed through the v0.8 implementation pass. The explicit v0.9 roadmap gate adds selectable immutable Base images/custom Environment starting points and remains implementation-pending. v0.10 adds a trusted per-agent session-to-Environment broker foundation outside Core. v0.11 adds a thin VS Code Agents-window remote-SSH adapter on top of v0.10. See `IMPLEMENTATION_STATUS.md` for current code reality and pending real-provider/client acceptance.

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

Thin client adapters and trusted agent-session integration may live around Hacocoon without changing that ownership. They translate client/session identity and generic Environment/client-access information without turning IDE, agent, or protocol concepts into Core concepts.

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

Concrete technologies stay behind adapters/ports. Core must not depend directly on Incus, Git, GitHub, AWS, VS Code, AHP, Daintree, Rookery, Btrfs, QCOW2, EC2, EBS, or other provider-specific concepts.

The v0.9 Base concept is an Environment starting-point identity. It must not turn Core into an Incus image manager: backend-native aliases, remotes, fingerprints, and image lifecycle mechanics remain adapter details.

The v0.10 agent-session identity is likewise an integration-layer concern. It does not add `Agent`, `VS Code Session`, or AHP to Core vocabulary.

v0.11 reinforces that boundary: Hacocoon prepares a standard SSH target for the VS Code Agents window; VS Code still owns its Agent Host and AHP implementation.

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

Parallel write-capable agent sessions should receive distinct canonical Workspace paths, normally distinct Git worktrees. Existing WorkspaceLease rules continue to reject conflicting read/write use of one canonical Workspace. Git worktrees isolate code changes; the Environment/Incus boundary provides OS/runtime isolation.

## Human-in-the-loop split

There are two different approval responsibilities:

```text
Development approval -> Human / GitHub / external orchestrator
Security approval    -> Hacocoon Policy/Capability boundary
```

Hacocoon security approval covers privileged authority such as protected Git operations, sensitive exposure, AWS/API access, or runtime privilege changes. It is not a general development workflow engine.

## Baseline roadmap progression

The 2026-08-29 rebaseline established v0.1-v0.7. v0.8 added client adapters. v0.9 adds selectable immutable Base images/custom Environment starting points while keeping Incus-native image mechanics behind the Environment adapter boundary. v0.10 adds a trusted per-agent session-to-Environment binding layer outside Core. v0.11 exposes that binding to the VS Code Agents window through standard remote SSH without implementing AHP in Hacocoon.

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
| v0.9 | Base Images & Custom Environments | selectable logical Bases resolved to immutable Environment starting revisions | design contract established; implementation pending |
| v0.10 | Per-Agent Sandbox & Agent Host Integration | bind independently routable coding-agent sessions to dedicated Environments without giving agents Hacocoon/Incus control authority | broker foundation implemented; real VS Code Agent Host/AHP + Incus routing acceptance pending |
| v0.11 | VS Code Remote Agent Host Adapter | prepare v0.10 session Environments as loopback-only SSH targets for the VS Code Agents window while VS Code owns Agent Host/AHP | adapter implemented; real Agents-window + Incus acceptance pending |

These rows describe roadmap/design and implementation progression, **not compatibility guarantees**. Hacocoon remains pre-1.0 and may change CLI, state formats, APIs, capabilities, adapters, and configuration incompatibly.

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

This section is retained as a historical design constraint. It is **not** an instruction to remove later functionality or design contracts now present in the repository.

## Cooperation with external orchestrators

External orchestrators may own tasks, worktrees, workers, budgets, retries, and development review. Hacocoon receives the resulting Workspace and provides the secure Environment and Capability boundary underneath them.

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

The normal VS Code adapter is `haco-vscode`:

```text
VS Code
  -> haco-vscode
  -> Hacocoon Environment + SSH access
  -> standard Remote-SSH
  -> /workspace
```

The v0.11 Agents-window adapter is `haco-agent-host`:

```text
VS Code Agents window
  -> Hacocoon-managed SSH alias
  -> v0.10 bound Environment
  -> VS Code remote CLI / Agent Host
```

Neither adapter implements an editor, terminal, debugger, Git UI, AI chat surface, Agent Host protocol, or model routing. Those remain client responsibilities.

### VS Code and other IDEs

VS Code can use Hacocoon as the underlying Environment runtime while remaining a client, not a Core dependency. Other IDEs should consume the same generic client/environment boundary rather than causing IDE-specific branching inside Core.

## Base images and custom Environment starting points

v0.9 introduces a Hacocoon-level **Base** as the selectable starting point for a newly created Environment.

```text
logical Base name
      |
      v
immutable Base revision
      |
      v
Environment
```

For Incus, the adapter resolves that immutable Base revision to an Incus image fingerprint internally. Mutable Incus aliases must not remain the identity of an already-created Environment.

Updating `my-dev` from revision A to revision B affects only new Environments:

```text
Environment 1 -> revision A
my-dev        -> revision B
Environment 2 -> revision B
```

A Base controls guest filesystem/runtime contents. It does not grant host mounts, devices, privileged mode, Linux capabilities, credentials, network authority, or external-service authority.

See `09_v0.9_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md` and `BASE_IMAGES.md`.

## Per-agent sandbox integration

v0.10 introduces a trusted integration-layer broker that maps an opaque external session identity to one Environment while reusing the normal Environment/WorkspaceLease lifecycle.

```text
trusted client
      |
 session broker
      |
 persisted binding proof
      |
 Environment
```

The coding agent is intentionally absent from the management path. Session-to-Environment binding is persisted in trusted control-plane state. A deterministic Environment name alone is not ownership proof: the broker refuses to adopt or release an Environment without a matching persisted binding.

See `10_v0.10_PER_AGENT_SANDBOX_AND_AGENT_HOST.md`.

## VS Code remote Agent Host adapter

v0.11 turns a v0.10 binding into a standard remote-SSH target for the VS Code Agents window.

```text
haco-agent-host prepare --session <id> <worktree>
          |
       v0.10 binding
          |
 loopback-only SSH target
          |
 VS Code Agents window
          |
 New -> Remote -> SSH
          |
 VS Code-owned Agent Host/AHP
```

The private SSH key remains client-side; only its public key enters Hacocoon's existing SSH-access path. Managed SSH aliases are derived from a hash instead of exposing raw session IDs.

The isolation unit is one prepared Hacocoon `--session` slot. Hacocoon does not currently receive VS Code's internal agent-session UUID automatically. If several VS Code sessions intentionally use one prepared alias, they share that Environment. Independent write-capable sessions therefore need separate Hacocoon session slots and normally separate worktrees.

`prepare` does not implicitly delete a v0.10 binding when client setup fails; explicit `release` is the destructive cleanup path. This keeps concurrent client setup from deleting another trusted caller's binding.

See `11_v0.11_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md`.

## Responsibility placement

| Concern | Placement |
|---|---|
| worktree management | outside Core; optional WorkspaceProvider boundary |
| normal VS Code/code-server/IDE access | client integration / Client Adapter |
| VS Code Agents-window remote host preparation | `haco-agent-host`, outside Core |
| client-native SSH configuration and launch | adapter-owned, outside Core |
| per-agent session -> Environment binding | trusted integration layer outside Core |
| VS Code Agent Host / AHP implementation | VS Code/client boundary, outside Hacocoon |
| AI chat UI / model selection | IDE/agent/orchestrator, outside Hacocoon |
| authorization / approval | Policy + Capability |
| Git push / GitHub authority | Git/GitHub capability boundary |
| model routing / task DAG / budgets | outside Hacocoon |
| AWS / EC2 / EBS | provider/capability adapters; EC2 remains experimental |
| Base selection / immutable Environment starting revision | Hacocoon domain contract; provider-native image mapping stays in adapter |
| Incus image alias/fingerprint/import mechanics | Incus adapter / explicit Incus administration |
| Btrfs / QCOW2 / storage mechanics | provider/adapter detail only when actually required |

## Experimental EC2 rule

The EC2 Environment provider is **experimental and disabled by default**. It must never become active merely because AWS credentials, AWS environment variables, the AWS CLI, or cloud metadata are present. Enabling it requires explicit Hacocoon-owned operator opt-in.

See `07_v0.7_REMOTE_AND_CLOUD_RUNTIME.md` and `REMOTE_CLOUD_PROVISIONING.md`.

## Pre-1.0 compatibility rule

Breaking changes remain allowed while the architecture is being hardened. Prefer a deliberate incompatible correction over preserving accidental behavior when the old behavior leaks authority, makes ownership ambiguous, creates unsafe cleanup/retry semantics, forces provider-specific concepts into Core, or preserves unnecessary complexity only for compatibility.

## Design principles

- Keep Core small.
- Prefer clear responsibility boundaries over premature common abstractions.
- Treat the Workspace as opaque to the runtime.
- Human approval is a first-class security decision, not a UI patch.
- Do not hand long-lived parent credentials to untrusted executed tools.
- Let coding agents be permissive inside an isolated Environment without turning that into host authority.
- Client convenience adapters must translate protocols, not absorb IDE or orchestration ownership.
- Resolve mutable Base names to immutable revisions before Environment creation depends on them.
- Keep provider-native image mechanics outside Core.
- Keep agent-session routing outside Core and require persisted proof before destructive agent-session cleanup.
- Do not implement evolving VS Code Agent Host/AHP details in Hacocoon when standard client transport can own them.
- Treat cleanup, retry, cancellation, concurrency, and partial failure as part of the feature.
- Keep implementation claims separate from real-provider/client acceptance claims.

## One-sentence definition

> **Hacocoon is a secure workspace runtime that runs developer tools and AI agents inside isolated environments without owning the IDE, Git workflow, or AI orchestration layer.**
