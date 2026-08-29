# Architecture Rebaseline and Roadmap

**Status:** authoritative architecture baseline  
**Date:** 2026-08-30  
**Implementation note:** `main` has implemented milestones continuously through v0.9. v0.10 is the active VS Code Remote Agent Host Adapter integration candidate. v0.11 Base Images & Custom Environments and v0.12 Sandbox Resource Limits remain design-only. See `IMPLEMENTATION_STATUS.md` for current code reality and pending real-provider/client acceptance.

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

The v0.9 agent-session identity is an integration-layer concern. It does not add `Agent`, `VS Code Session`, or AHP to Core vocabulary.

The v0.11 Base concept is an Environment starting-point identity. It must not turn Core into an Incus image manager: backend-native aliases, remotes, fingerprints, and image lifecycle mechanics remain adapter details.

The v0.12 ResourceBudget concept describes a provider-neutral Environment resource ceiling. It must not expose Incus-native resource keys as Core/public architecture, and it remains distinct from Capability authority.

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

The 2026-08-29 rebaseline established v0.1-v0.7. v0.8 added client adapters. The 2026-08-30 numbering cleanup keeps the already-implemented per-agent broker as v0.9, puts the active Remote Agent Host Adapter integration at v0.10, and moves implementation-pending Base Images and Resource Limits to v0.11 and v0.12.

| Version | Gate | Purpose | Repository state |
|---|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | external path -> Incus -> exec/shell -> delete | implemented; real Incus acceptance remains environment-dependent |
| v0.2 | Workspace Abstraction & Lease | formal workspace identity/leases and concurrency safety | implemented |
| v0.3 | Client & Interactive Access | status/SSH/ports/client integration without IDE ownership | implemented |
| v0.4 | Policy & Capability Foundation | allow/deny/require-approval and audit boundary | implemented |
| v0.5 | Git / GitHub Capability | scoped Git/GitHub authority without broad ambient credentials | implemented |
| v0.6 | Agent & Orchestrator Integration | generic machine/agent integration above secure execution | implemented |
| v0.7 | Remote / Cloud Runtime & External Capabilities | AWS capability plus remote provider work | implemented experimentally; real AWS acceptance pending |
| v0.8 | Client Adapters & VS Code Integration | thin client adapter layer; VS Code Remote-SSH first | implemented; real VS Code + Incus acceptance remains environment-dependent |
| v0.9 | Per-Agent Sandbox & Agent Host Integration | bind independently routable coding-agent sessions to dedicated Environments without giving agents Hacocoon/Incus control authority | broker foundation implemented; real VS Code Agent Host/AHP + Incus routing acceptance pending |
| v0.10 | VS Code Remote Agent Host Adapter | connect the VS Code Agents window to the v0.9-bound Environment through the trusted client adapter boundary | active PR #111; not yet on `main` |
| v0.11 | Base Images & Custom Environments | selectable logical Bases resolved to immutable Environment starting revisions | design contract established; implementation pending |
| v0.12 | Sandbox Resource Limits | explicit host-enforced CPU/memory/PID/root-storage budgets for each Environment | design contract established; implementation pending |

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

## Per-agent sandbox integration

v0.9 introduces a trusted integration-layer broker that maps an opaque external session identity to one Environment while reusing the normal Environment/WorkspaceLease lifecycle.

```text
VS Code Agents window / trusted client
              |
      trusted integration
              |
       session broker
          /       \
 Environment A   Environment B
     |                |
   Incus A          Incus B
```

The coding agent is intentionally absent from the management path. It is not expected to invoke `haco`, and must not receive Incus administrator authority, Hacocoon state/control access, or broad host credentials merely because an Environment was allocated for it.

Session-to-Environment binding is persisted in trusted control-plane state. A deterministic Environment name alone is not ownership proof: the broker refuses to adopt or release an Environment without a matching persisted binding.

For VS Code, the preferred direction is to place the Agent Host next to the assigned Workspace inside the Environment and keep Agent Host Protocol details at the client-integration boundary. AHP-specific types/versioning remain outside Core.

See `09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md` for the detailed contract.

## VS Code Remote Agent Host Adapter

v0.10 is the trusted adapter layer currently developed in PR #111.

Its intended flow is:

```text
VS Code Agents window
      |
standard Remote SSH
      |
Hacocoon-managed loopback alias
      |
v0.9-bound Environment
      |
/workspace + Agent Host
```

The client retains the SSH private key. Hacocoon receives only the public key through the existing hardened SSH-access boundary. VS Code owns Agent Host/AHP behavior; Hacocoon owns Environment selection and safe connection preparation.

This gate is **not implemented on `main` until PR #111 is merged**. Real Windows/WSL + Incus + current VS Code Agents-window acceptance remains environment-dependent.

## Base images and custom Environment starting points

v0.11 introduces a Hacocoon-level **Base** as the selectable starting point for a newly created Environment.

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

A Base controls guest filesystem/runtime contents. It does not grant host mounts, devices, privileged mode, Linux capabilities, credentials, network authority, or external-service authority. Those remain governed by the Environment / Policy / Capability boundary.

v0.11 composes with the implemented v0.9 per-agent broker and the v0.10 adapter when those paths request Environment creation; Base selection must still resolve through the normal immutable-revision contract.

See `11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md` for the v0.11 gate and `BASE_IMAGES.md` for the detailed companion design.

## Sandbox resource limits

v0.12 gives each Environment an explicit resource budget.

```text
Environment
  +-- CPU ceiling
  +-- memory ceiling
  +-- process/PID ceiling
  +-- root-storage ceiling where safely enforceable
```

The resource budget describes how much capacity an Environment may consume; it is not a Hacocoon Capability and does not grant external authority.

For Incus, provider-neutral resource concepts are mapped to Incus-native controls inside the adapter. If a caller explicitly requests a limit the selected provider cannot enforce, creation must fail closed rather than silently ignore it.

The effective creation-time budget is persisted with the Environment. Changing defaults later affects only future Environment creation. The first gate prefers creation-time budgets and does not require arbitrary live resize or aggregate host scheduling.

v0.12 composes with v0.11 Bases and v0.9/v0.10 agent integration. A Base must not raise resource limits, and an agent must not gain control-plane authority to raise its own host-enforced limits.

See `12_v0.12_SANDBOX_RESOURCE_LIMITS.md` for the detailed contract.

## Responsibility placement

| Concern | Placement |
|---|---|
| worktree management | outside Core; optional WorkspaceProvider boundary |
| VS Code / code-server / IDE access | client integration / Client Adapter |
| client-native SSH configuration and launch | adapter-owned, outside Core |
| per-agent session -> Environment binding | trusted integration layer outside Core |
| VS Code Agent Host / AHP details | client integration boundary outside Core |
| AI chat UI / model selection | IDE/agent/orchestrator, outside Hacocoon |
| authorization / approval | Policy + Capability |
| Git push / GitHub authority | Git/GitHub capability boundary |
| AI agent integration | generic execution + external/trusted integration |
| model routing / task DAG / budgets | outside Hacocoon |
| AWS / EC2 / EBS | provider/capability adapters; EC2 remains experimental |
| Base selection / immutable Environment starting revision | Hacocoon domain contract; provider-native image mapping stays in adapter |
| Incus image alias/fingerprint/import mechanics | Incus adapter / explicit Incus administration |
| per-Environment CPU/memory/PID/root-storage budget | Hacocoon Environment contract; provider-native enforcement stays in adapter |
| aggregate host scheduling/admission | future separate concern; not v0.12 Core scheduler ownership |
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
- Bound permissive Environment execution with explicit host-enforced resource budgets.
- Client convenience adapters must translate protocols, not absorb IDE or orchestration ownership.
- Resolve mutable Base names to immutable revisions before Environment creation depends on them.
- Keep provider-native image mechanics outside Core.
- Keep provider-native resource controls outside Core while preserving provider-neutral effective limits.
- Keep agent-session routing outside Core and require persisted proof before destructive agent-session cleanup.
- Do not create abstractions solely for hypothetical future backends.
- Treat cleanup, retry, cancellation, concurrency, and partial failure as part of the feature.
- Keep implementation claims separate from real-provider/client acceptance claims.

## One-sentence definition

> **Hacocoon is a secure workspace runtime that runs developer tools and AI agents inside isolated, resource-bounded environments without owning the IDE, Git workflow, or AI orchestration layer.**
