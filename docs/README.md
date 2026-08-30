# Documentation Map

[**日本語**](README.ja.md) | English

Hacocoon is **pre-1.0**. Version numbers describe roadmap/implementation milestones, not compatibility guarantees or release readiness.

For a Japanese overview, start with [`../README.ja.md`](../README.ja.md), [`README.ja.md`](README.ja.md), and [`ARCHITECTURE_GUIDE.ja.md`](ARCHITECTURE_GUIDE.ja.md).

## Source-of-truth order

When documents appear to disagree, use this order:

1. `00_REBASELINE_AND_ROADMAP.md` — product boundary and roadmap progression.
2. `00D_VERSIONING_AND_RELEASE_STATUS.md` — authoritative version-number assignment.
3. `00C_TERMINOLOGY_AND_BOUNDARIES.md` — canonical architecture vocabulary.
4. `00B_SECURITY_ARCHITECTURE.md` — cross-cutting trust and security rules.
5. `IMPLEMENTATION_STATUS.md` — current code reality and pending acceptance.
6. The relevant versioned release specification.
7. Specialized documents such as `CLIENT_ACCESS.md`, `REMOTE_CLOUD_PROVISIONING.md`, and `BASE_IMAGES.md`.
8. `00A_PLUGIN_ARCHITECTURE.md` — extension/adapter guidance.
9. `90_CODEX_IMPLEMENTATION_HANDOFF.md` — implementation and maintenance workflow.
10. `91_IMPLEMENTATION_REFERENCE_NOTES.md` — non-normative references and historical notes.
11. ADRs under `adr/` — scoped decisions.

`README.md`, `CODEX_START_HERE.md`, and `Hacocoon_v0.1-v0.7_MASTER.md` are entry points/indexes; they summarize rather than redefine the architecture.

## Current documentation state

| Version | Gate | State |
|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | implemented |
| v0.2 | Workspace Abstraction & Lease | implemented |
| v0.3 | Client & Interactive Access | implemented |
| v0.4 | Policy & Capability Foundation | implemented |
| v0.5 | Git / GitHub Capability | implemented |
| v0.6 | Agent & Orchestrator Integration | implemented |
| v0.7 | Remote / Cloud Runtime & External Capabilities | experimentally implemented; real AWS acceptance pending |
| v0.8 | Client Adapters & VS Code Integration | implemented; real client acceptance pending |
| v0.9 | Per-Agent Sandbox & Agent Host Integration | broker foundation implemented |
| v0.10 | VS Code Remote Agent Host Adapter | implemented; real host acceptance pending |
| v0.11 | Base Images & Custom Environments | first implementation slice present; broader build/import/GC pending |
| v0.12 | Sandbox Resource Limits | design contract only |

The implemented progression is contiguous through **v0.11**. See `00D_VERSIONING_AND_RELEASE_STATUS.md` for the numbering policy and `IMPLEMENTATION_STATUS.md` for exact code/acceptance reality.

## Specification vs implementation

A release specification describes a roadmap/design/acceptance contract. `IMPLEMENTATION_STATUS.md` describes **current code reality**.

Examples:

- v0.7 EC2 code can exist while real AWS acceptance remains pending.
- v0.8 `haco-vscode` can exist while real Windows/WSL + Incus + VS Code Remote-SSH acceptance remains pending.
- v0.9 contains the persisted per-session Environment broker while real Agent Host/AHP routing acceptance remains host-dependent.
- v0.10 `haco-agent-host` is implemented while real VS Code Agent Host acceptance remains host-dependent.
- v0.11 Base selection/pinning can be implemented while image build/import/history/GC remains future work.
- v0.12 can define provider-neutral resource budgets before resource enforcement is implemented.

Do not infer release/tag readiness, production support, or real-host acceptance from a roadmap version alone.

## v0.8 reading path

1. `08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md`
2. `CLIENT_ACCESS.md`
3. `00_REBASELINE_AND_ROADMAP.md`
4. `IMPLEMENTATION_STATUS.md`

VS Code owns its editor, terminal, debugger, Git UI, and coding-agent UI. Hacocoon owns the Environment and external-authority boundary.

## v0.9 reading path — Per-Agent Sandbox

1. `09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md`
2. `09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md`
3. `02_v0.2_WORKSPACE_ABSTRACTION_AND_LEASE.md`
4. `08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md`
5. `IMPLEMENTATION_STATUS.md`

```text
trusted client / integration
        |
 opaque session identity
        |
 internal/agenthost broker
        |
 persisted ownership proof
        |
 Environment
```

The coding agent is deliberately absent from the Hacocoon management path. A deterministic Environment name alone is not enough to authorize adoption or deletion.

## v0.10 reading path — VS Code Remote Agent Host Adapter

1. `10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md`
2. `10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.ja.md`
3. `09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md`
4. `CLIENT_ACCESS.md`
5. `IMPLEMENTATION_STATUS.md`

```text
VS Code Agents window
        |
    Remote SSH
        |
Hacocoon-managed loopback alias
        |
  haco-agent-host
        |
 v0.9-bound Environment
        |
    /workspace
```

Hacocoon prepares the trusted Environment and SSH access; VS Code continues to own the Agent Host and Agent Host Protocol. Real Windows/WSL + Incus + VS Code acceptance remains host-dependent.

## v0.11 reading path — Base Images

1. `11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md`
2. `BASE_IMAGES.md`
3. `BASE_IMAGES.ja.md`
4. `00_REBASELINE_AND_ROADMAP.md`
5. `00B_SECURITY_ARCHITECTURE.md`
6. `IMPLEMENTATION_STATUS.md`

```text
logical Base name
        |
        v
mutable provider source
        |
 resolve once at create
        v
immutable Base revision
        |
        v
Environment
```

Implemented first-slice commands:

```text
haco image list
haco image inspect <base>
haco create --base <base> --workspace <path> <environment>
```

For Incus, provider-native aliases/remotes/fingerprints remain adapter details. Existing Environment metadata records the resolved immutable revision so moving a logical source later affects future creation only.

Custom Base build/import, revision history, rollback, and garbage collection remain outside the first implemented slice.

## v0.12 reading path — Sandbox Resource Limits

1. `12_v0.12_SANDBOX_RESOURCE_LIMITS.md`
2. `12_v0.12_SANDBOX_RESOURCE_LIMITS.ja.md`
3. `00_REBASELINE_AND_ROADMAP.md`
4. `11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md`
5. `09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md`
6. `00B_SECURITY_ARCHITECTURE.md`
7. `IMPLEMENTATION_STATUS.md`

Resource budgets bound CPU, memory, PIDs, and safely enforceable root storage **inside** an Environment. They remain separate from Capabilities, which mediate authority crossing the Environment boundary.

## Compatibility and breaking changes

Hacocoon is pre-1.0 and actively being simplified and hardened. Breaking changes may affect CLI/helper binaries, persisted state, Core/adapter APIs, capability/policy contracts, provider configuration, Base/image lifecycle, client/agent integration, resource-budget behavior, experimental backends, and documentation structure.

Prefer an explicit incompatible correction over preserving accidental compatibility that weakens the architecture. Material operator impact should remain documented and tested.

## Editing rule

1. update the authoritative document first;
2. update `00D_VERSIONING_AND_RELEASE_STATUS.md` when numbering/status changes;
3. update `IMPLEMENTATION_STATUS.md` when code reality changes;
4. update entry points/handoffs when summaries become stale;
5. keep implementation claims distinct from real-provider/client acceptance claims;
6. preserve explicit experimental/default-off provider rules;
7. update Japanese summaries when user-facing descriptions change;
8. run `python tools/check_docs.py`;
9. do not turn an implementation detail into an accidental compatibility promise.
