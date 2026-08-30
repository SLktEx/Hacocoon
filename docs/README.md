# Documentation

[**日本語**](README.ja.md) | English

Hacocoon is **pre-1.0**. These documents distinguish three different things that are easy to confuse:

- **architecture intent** — what Hacocoon owns and where boundaries belong;
- **repository reality** — what is actually implemented on `main`;
- **acceptance** — what has been proven on real Incus / Windows / AWS environments.

> [!TIP]
> If you only want to know **what works in the current repository**, start with [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md).

## Start here

| I want to know… | Read |
|---|---|
| What Hacocoon is | [`../README.md`](../README.md) |
| What is implemented right now | [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md) |
| What the architecture owns / does not own | [`00_REBASELINE_AND_ROADMAP.md`](00_REBASELINE_AND_ROADMAP.md) |
| Which milestone number means what | [`00D_VERSIONING_AND_RELEASE_STATUS.md`](00D_VERSIONING_AND_RELEASE_STATUS.md) |
| Security/trust boundaries | [`00B_SECURITY_ARCHITECTURE.md`](00B_SECURITY_ARCHITECTURE.md) |
| Canonical terminology | [`00C_TERMINOLOGY_AND_BOUNDARIES.md`](00C_TERMINOLOGY_AND_BOUNDARIES.md) |
| How to integrate VS Code / clients | [`CLIENT_ACCESS.md`](CLIENT_ACCESS.md), [`08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md`](08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md) |
| Per-agent sandboxes | [`09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md`](09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md) |
| VS Code Agent Host integration | [`10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md`](10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md) |
| Base images | [`11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md`](11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md), [`BASE_IMAGES.md`](BASE_IMAGES.md) |
| Resource limits | [`12_v0.12_SANDBOX_RESOURCE_LIMITS.md`](12_v0.12_SANDBOX_RESOURCE_LIMITS.md) |
| Planned local OCI registry | [`13_v0.13_LOCAL_OCI_REGISTRY.md`](13_v0.13_LOCAL_OCI_REGISTRY.md) |
| Planned OCI Seed / COW optimization | [`13A_v0.13_OCI_SEED_AND_COW.md`](13A_v0.13_OCI_SEED_AND_COW.md) |

## Source-of-truth order

There is no single document that should override every other document for every question. Use the source that owns the question:

1. **Current code reality:** `IMPLEMENTATION_STATUS.md`
2. **Milestone numbering/status:** `00D_VERSIONING_AND_RELEASE_STATUS.md`
3. **Product boundary / roadmap intent:** `00_REBASELINE_AND_ROADMAP.md`
4. **Canonical vocabulary:** `00C_TERMINOLOGY_AND_BOUNDARIES.md`
5. **Cross-cutting security rules:** `00B_SECURITY_ARCHITECTURE.md`
6. **Feature contract:** the relevant versioned specification
7. **Operational/detail documents:** `CLIENT_ACCESS.md`, `REMOTE_CLOUD_PROVISIONING.md`, `BASE_IMAGES.md`, etc.
8. **Extension guidance:** `00A_PLUGIN_ARCHITECTURE.md`
9. **Implementation workflow:** `90_CODEX_IMPLEMENTATION_HANDOFF.md`
10. **Non-normative historical/reference notes:** `91_IMPLEMENTATION_REFERENCE_NOTES.md` and `adr/`

`README.md`, `README.ja.md`, `CODEX_START_HERE.md`, and generated master/index documents are entry points. They summarize; they do not redefine current code reality.

## Current milestone status

**Status legend:** ✅ implemented · 🧪 experimental · 🚧 planned

| Version | Gate | State |
|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | ✅ implemented |
| v0.2 | Workspace Abstraction & Lease | ✅ implemented |
| v0.3 | Client & Interactive Access | ✅ implemented |
| v0.4 | Policy & Capability Foundation | ✅ implemented |
| v0.5 | Git / GitHub Capability | ✅ implemented |
| v0.6 | Agent & Orchestrator Integration | ✅ implemented |
| v0.7 | Remote / Cloud Runtime & External Capabilities | 🧪 experimental implementation; real AWS acceptance pending |
| v0.8 | Client Adapters & VS Code Integration | ✅ implemented; real-client acceptance pending |
| v0.9 | Per-Agent Sandbox & Agent Host Integration | ✅ broker foundation implemented |
| v0.10 | VS Code Remote Agent Host Adapter | ✅ implemented; real-host acceptance pending |
| v0.11 | Base Images & Custom Environments | ✅ first implementation slice |
| v0.12 | Sandbox Resource Limits | ✅ first implementation slice |
| v0.13 | Local OCI Registry | 🚧 planned; not implemented on `main` |

The implemented progression is contiguous through **v0.12**. See [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md) for exact repository reality and acceptance gaps.

## Specification vs implementation

A specification is a design/acceptance contract. It is **not** proof that the feature exists on `main`.

Examples:

- v0.7 EC2 is implemented experimentally while real AWS acceptance remains pending.
- v0.8 `haco-vscode` exists while real Windows/WSL + Incus + VS Code acceptance remains host-dependent.
- v0.9 contains the persisted per-session Environment broker while real Agent Host/AHP routing remains host-dependent.
- v0.10 `haco-agent-host` is implemented while real Agent Host acceptance remains host-dependent.
- v0.11 Base selection and immutable pinning are implemented; custom build/import/history/GC are follow-up work.
- v0.12 ResourceBudget enforcement is implemented for Incus; real workload enforcement remains host-dependent.
- v0.13 Local OCI Registry and OCI Seed/COW are design documents only until implementation lands.

Do not infer tag/release readiness, compatibility, production support, or real-host acceptance from a roadmap number alone.

## Core reading paths

### Client access / VS Code

1. [`08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md`](08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md)
2. [`CLIENT_ACCESS.md`](CLIENT_ACCESS.md)
3. [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md)

The first adapter is `haco-vscode`. VS Code owns the editor, terminal, debugger, Git UI, and coding-agent UI. Hacocoon owns the Environment and authority boundary.

### Per-agent sandbox / Agent Host

1. [`09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md`](09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md)
2. [`10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md`](10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md)
3. [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md)

`haco-agent-host` prepares trusted, loopback-only access to a bound Environment. The coding agent itself is deliberately absent from the Hacocoon management path.

### Base images

1. [`11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md`](11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md)
2. [`BASE_IMAGES.md`](BASE_IMAGES.md)
3. [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md)

Implemented first-slice commands include:

```text
haco image list
haco image inspect <base>
haco create --base <base> --workspace <path> <environment>
```

Mutable provider sources resolve once to an immutable Base revision before Environment creation.

### Resource limits

1. [`12_v0.12_SANDBOX_RESOURCE_LIMITS.md`](12_v0.12_SANDBOX_RESOURCE_LIMITS.md)
2. [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md)

ResourceBudget covers CPU, memory, PIDs, and root storage. Incus finite limits are applied and verified before `start`; unsupported finite requests fail closed.

### v0.13 OCI work

1. [`13_v0.13_LOCAL_OCI_REGISTRY.md`](13_v0.13_LOCAL_OCI_REGISTRY.md)
2. [`13A_v0.13_OCI_SEED_AND_COW.md`](13A_v0.13_OCI_SEED_AND_COW.md)

These are **planned**. The registry/cache gateway is the first slice; OCI Seed/COW is a second optimization slice. Neither should be described as implemented until `IMPLEMENTATION_STATUS.md` says so.

## Compatibility and breaking changes

Hacocoon is pre-1.0 and actively being simplified and hardened. CLI/helper binaries, state, adapters, policy/capability contracts, Base/image behavior, client/agent integration, providers, and documentation structure may change incompatibly.

Prefer an explicit incompatible correction over preserving accidental compatibility that weakens a security or ownership boundary.

## Editing rule

When changing docs:

1. update the document that owns the fact;
2. update `IMPLEMENTATION_STATUS.md` whenever code reality changes;
3. update `00D_VERSIONING_AND_RELEASE_STATUS.md` whenever numbering/status changes;
4. update English authoritative docs before companions;
5. keep implementation claims separate from real-host acceptance claims;
6. keep experimental/default-off boundaries explicit;
7. run `python tools/check_docs.py`.
