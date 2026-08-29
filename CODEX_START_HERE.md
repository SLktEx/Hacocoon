# CODEX START HERE

Hacocoon is built around one product boundary:

> **Hacocoon is a secure workspace runtime, not an AI orchestrator.**

Hacocoon is still **pre-1.0**. Breaking changes are allowed when they simplify the system, strengthen the trust boundary, remove accidental complexity, or correct an unsafe contract.

The implemented milestone sequence is contiguous through **v0.9**. v0.10 is the active VS Code Remote Agent Host Adapter integration candidate. v0.11 Base Images and v0.12 Resource Limits remain design-only.

## Read first

1. `docs/README.md` — documentation precedence and current-vs-historical rules.
2. `docs/00_REBASELINE_AND_ROADMAP.md` — product boundary and roadmap progression.
3. `docs/00D_VERSIONING_AND_RELEASE_STATUS.md` — authoritative version numbering.
4. `docs/00C_TERMINOLOGY_AND_BOUNDARIES.md` — canonical terminology.
5. `docs/00B_SECURITY_ARCHITECTURE.md` — trust boundary.
6. `docs/IMPLEMENTATION_STATUS.md` — current repository reality and pending acceptance.
7. the versioned specification relevant to the code being changed.
8. `docs/90_CODEX_IMPLEMENTATION_HANDOFF.md` — implementation and maintenance workflow.
9. `.github/security/ADVERSARIAL_AUDIT.md` for security-sensitive or hostile review.

`docs/00A_PLUGIN_ARCHITECTURE.md` is extension/adaptor guidance, not an instruction to create speculative plugin interfaces.

## Roadmap progression

```text
0.1  Secure Workspace Runtime MVP                         implemented
0.2  Workspace Abstraction & Lease                        implemented
0.3  Client & Interactive Access                          implemented
0.4  Policy & Capability Foundation                       implemented
0.5  Git / GitHub Capability                              implemented
0.6  Agent & Orchestrator Integration                     implemented
0.7  Remote / Cloud Runtime & External Capabilities       experimental implementation
0.8  Client Adapters & VS Code Integration                implemented
0.9  Per-Agent Sandbox & Agent Host Integration           broker foundation implemented
0.10 VS Code Remote Agent Host Adapter                    active PR #111
0.11 Base Images & Custom Environments                    design only
0.12 Sandbox Resource Limits                              design only
```

Do not infer implementation, release readiness, or compatibility from a version number alone. `docs/IMPLEMENTATION_STATUS.md` owns implementation truth.

## Current work rule

Treat the existing v0.1-v0.9 implementation as a system to **harden, simplify, verify, and evolve**.

When working on numbered roadmap gates:

- v0.10 work belongs to the trusted VS Code Remote Agent Host Adapter boundary;
- v0.11 work belongs to Base Images & Custom Environments;
- v0.12 work belongs to provider-neutral Sandbox Resource Limits.

Before changing behavior:

1. check `docs/IMPLEMENTATION_STATUS.md` for current reality;
2. identify which architecture boundary and versioned contract the change belongs to;
3. inspect actual code and tests rather than trusting documentation claims;
4. preserve fail-closed security behavior;
5. add or strengthen tests for the changed contract;
6. update status/documentation when reality changes.

Do not assign a new roadmap number without checking `docs/00D_VERSIONING_AND_RELEASE_STATUS.md` and active PRs.

## v0.8 client-adapter rule

VS Code is the first convenience client, not a Core dependency.

`haco-vscode` may translate generic Hacocoon Environment/client-access state into client-native SSH configuration and launch standard Remote-SSH. It must not absorb editor, terminal, Git UI, AI chat, model routing, task orchestration, or worktree ownership.

```text
VS Code AI / coding agent
        |
 isolated Environment
 broad local freedom
        |
--- trust boundary ---
        |
Hacocoon Policy / Capability / Audit
        |
GitHub / AWS / Host
```

When Hacocoon runs in WSL while desktop VS Code runs on Windows, treat the Windows SSH client configuration as client-side state.

## v0.9 per-agent sandbox rule

`internal/agenthost` binds an opaque external session identity to a dedicated Environment while reusing the normal WorkspaceLease and Environment lifecycle.

Rules:

- the coding agent must not receive Hacocoon/Incus management authority;
- raw session IDs must not become runtime names or trusted ownership proof;
- exact reacquire is idempotent;
- Workspace/access-mode rebinding fails closed;
- release requires persisted ownership proof;
- parallel RW sessions normally use distinct canonical Workspace paths/worktrees;
- real Agent Host/AHP routing acceptance remains separate from the broker foundation.

Read `docs/09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md`.

## v0.10 Remote Agent Host Adapter rule

v0.10 is the active integration candidate in PR #111 and is not part of `main` until merged.

The adapter may prepare a v0.9-bound Environment as a loopback-only SSH target for the VS Code Agents window. The client keeps the private SSH key; only the public key crosses the existing Hacocoon SSH-access boundary.

VS Code owns Agent Host/AHP behavior. Hacocoon owns trusted Environment selection and safe connection preparation.

## v0.11 Base-image rule

Hacocoon exposes a **Base**, not an Incus image alias, as the Environment starting-point concept.

```text
logical Base name
        |
        v
immutable Base revision
        |
        v
provider-native image identity
        |
        v
Environment
```

Rules:

- resolve the logical Base to an immutable revision before Environment creation depends on it;
- persist the resolved immutable revision with Environment state;
- updating a Base affects future Environments only;
- custom images and image archives are untrusted input;
- image metadata must not grant host mounts, devices, privileged mode, credentials, network authority, or external-service authority;
- arbitrary custom-image build steps must not execute directly with Hacocoon host authority;
- deletion/GC must not remove a revision still referenced by a running or recoverable Environment;
- handle create/update/remove/GC races deliberately.

Read `docs/11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md` first and `docs/BASE_IMAGES.md` for the detailed companion design.

## v0.12 resource-budget rule

Resource limits describe Environment consumption, not external authority.

The first design gate covers CPU, memory, PIDs/process count, and root storage where safely enforceable.

Rules:

- keep provider-neutral concepts in the Hacocoon contract;
- keep Incus-native resource keys inside the adapter;
- requested-but-unsupported limits fail closed;
- apply requested limits before client/agent access;
- persist the effective creation-time budget;
- Base metadata cannot raise host-selected limits;
- coding agents cannot receive control-plane authority to raise their own limits.

Read `docs/12_v0.12_SANDBOX_RESOURCE_LIMITS.md`.

## Hard rules

- Do not make Git worktree a Core concept.
- Do not move agent scheduling, model routing, retry strategy, task DAGs, or model budgets into Hacocoon.
- Do not build a Hacocoon-specific AI UI when the client already owns that UX.
- Do not move VS Code, Daintree, JetBrains, or another client product into Core.
- Do not mount host HOME, `~/.ssh`, `~/.aws`, GitHub tokens, the Incus socket, or Hacocoon control state into an Environment as a shortcut.
- Treat commands running inside an Environment as untrusted with respect to host authority.
- Keep provider-specific concepts such as Incus, GitHub, AWS, EC2, EBS, Btrfs, and QCOW2 outside the Core domain.
- Keep Incus image aliases/remotes/fingerprints outside the Base/Core contract.
- EC2 remains experimental and disabled by default. Never activate it by implicit AWS credential discovery.
- Prefer standard CLI/protocol boundaries over Hacocoon-specific wrappers.
- Do not create an interface only because a hypothetical future backend might need one.
- Do not preserve accidental compatibility when it conflicts with a smaller or safer pre-1.0 design.
- Destructive and authority-sensitive operations must be reviewed adversarially, including retry, partial failure, concurrency, and cleanup.

## Compatibility rule

Until an explicit stable compatibility milestone is declared, existing behavior is not automatically a permanent public contract.

A breaking change should still be deliberate: explain the replacement, avoid silent data loss, preserve recoverability where possible, update affected docs/tests, and provide migration guidance when a supported migration exists.

## Work method

1. Reproduce or define the behavior being changed.
2. Map it to Workspace / Environment / Execution / Client Adapter / agent binding / Base / ResourceBudget / Policy / Capability / provider boundaries.
3. Read the relevant implementation and tests.
4. Consider hostile input, concurrent operations, cancellation, retry, and partial failure.
5. Implement the smallest coherent change.
6. Add unit tests and the strongest practical integration/E2E coverage.
7. Run maintained checks:

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/haco
go build ./cmd/haco-vscode
python tools/check_docs.py
```

8. Keep real Incus, real Windows/WSL + VS Code, Agent Host/AHP, and real AWS acceptance claims separate from fake/process-boundary tests.
9. Update `docs/IMPLEMENTATION_STATUS.md` whenever repository reality materially changes.

When uncertain, choose the smallest design that preserves the trust boundary and keeps Workspace, Client, Orchestrator, Environment, agent binding, Base, ResourceBudget, and external authority responsibilities separate.
