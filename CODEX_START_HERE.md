# CODEX START HERE

Hacocoon is built around one product boundary:

> **Hacocoon is a secure workspace runtime, not an AI orchestrator.**

The 2026-08-29 architecture rebaseline has now been followed by implementation work through the v0.8 roadmap. Do not use old instructions that describe v0.1 or v0.7 as the only active implementation gate.

Hacocoon is still **pre-1.0**. Breaking changes are allowed when they simplify the system, strengthen the trust boundary, remove accidental complexity, or correct an unsafe contract.

## Read first

1. `docs/README.md` — documentation precedence and current-vs-historical rules.
2. `docs/00_REBASELINE_AND_ROADMAP.md` — product boundary and roadmap progression.
3. `docs/00C_TERMINOLOGY_AND_BOUNDARIES.md` — canonical terminology.
4. `docs/00B_SECURITY_ARCHITECTURE.md` — trust boundary.
5. `docs/IMPLEMENTATION_STATUS.md` — current repository reality and pending acceptance.
6. the release specification relevant to the code being changed (`docs/01_...` through `docs/08_...`).
7. `docs/90_CODEX_IMPLEMENTATION_HANDOFF.md` — implementation and maintenance workflow.
8. `.github/security/ADVERSARIAL_AUDIT.md` for security-sensitive or hostile review.

`docs/00A_PLUGIN_ARCHITECTURE.md` is extension/adaptor guidance, not an instruction to create speculative plugin interfaces.

## Roadmap progression

```text
0.1 Secure Workspace Runtime MVP
0.2 Workspace Abstraction & Lease
0.3 Client & Interactive Access
0.4 Policy & Capability Foundation
0.5 Git / GitHub Capability
0.6 Agent & Orchestrator Integration
0.7 Remote / Cloud Runtime & External Capabilities
0.8 Client Adapters & VS Code Integration
```

The implementation progression currently reaches v0.8. This does **not** mean the interfaces are stable or that every real-provider/client acceptance gate has passed.

## Current job

Treat the existing v0.1-v0.8 implementation as a system to **harden, simplify, verify, and evolve**, not as frozen compatibility surface.

Before changing behavior:

1. check `docs/IMPLEMENTATION_STATUS.md` for current reality;
2. identify which architecture boundary and versioned contract the change belongs to;
3. inspect actual code and tests rather than trusting documentation claims;
4. preserve fail-closed security behavior;
5. add or strengthen tests for the changed contract;
6. update status/documentation when reality changes.

Do not invent a post-v0.8 product direction or broaden Core merely because the numbered implementation exists.

## v0.8 client-adapter rule

VS Code is the first convenience client, not a Core dependency.

`haco-vscode` may translate generic Hacocoon Environment/client-access state into client-native SSH configuration and launch standard Remote-SSH. It must not absorb editor, terminal, Git UI, AI chat, model routing, task orchestration, or worktree ownership.

The intended trust model is:

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

When Hacocoon runs in WSL while desktop VS Code runs on Windows, treat the Windows SSH client configuration as client-side state. Do not assume the WSL user's `~/.ssh/config` is the desktop client's configuration.

## Hard rules

- Do not make Git worktree a Core concept.
- Do not move agent scheduling, model routing, retry strategy, task DAGs, or model budgets into Hacocoon.
- Do not build a Hacocoon-specific AI UI when the client already owns that UX.
- Do not move VS Code, Daintree, JetBrains, or another client product into Core.
- Do not mount host HOME, `~/.ssh`, `~/.aws`, GitHub tokens, the Incus socket, or Hacocoon control state into an Environment as a shortcut.
- Treat commands running inside an Environment as untrusted with respect to host authority.
- Keep provider-specific concepts such as Incus, GitHub, AWS, EC2, EBS, Btrfs, and QCOW2 outside the Core domain.
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
2. Map it to Workspace / Environment / Execution / Client Adapter / Policy / Capability / provider boundaries.
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

8. Keep real Incus, real Windows/WSL + VS Code, and real AWS acceptance claims separate from fake/process-boundary tests.
9. Update `docs/IMPLEMENTATION_STATUS.md` whenever repository reality materially changes.

When uncertain, choose the smallest design that preserves the trust boundary and keeps Workspace, Client, Orchestrator, Environment, and external authority responsibilities separate.
