# Documentation Map

[**日本語**](README.ja.md) | English

This file defines how to read Hacocoon documentation after the 2026-08-29 architecture rebaseline, the v0.8 implementation progression, the v0.9 Base-image roadmap contract, and the v0.10 per-agent sandbox broker foundation.

Hacocoon remains **pre-1.0**. Documentation describes architecture, implementation state, and roadmap contracts; it does not imply API, CLI, state-format, provider, client-adapter, image, agent-integration, or configuration compatibility guarantees.

For a Japanese overview, start with [`../README.ja.md`](../README.ja.md), [`README.ja.md`](README.ja.md), and [`ARCHITECTURE_GUIDE.ja.md`](ARCHITECTURE_GUIDE.ja.md).

## Source-of-truth order

When documents appear to disagree, use this order:

1. `00_REBASELINE_AND_ROADMAP.md` — product boundary and roadmap progression.
2. `00C_TERMINOLOGY_AND_BOUNDARIES.md` — canonical architecture vocabulary.
3. `00B_SECURITY_ARCHITECTURE.md` — cross-cutting trust and security rules.
4. `IMPLEMENTATION_STATUS.md` — current code reality and pending acceptance.
5. The relevant versioned release specification (`01_...` through `10_...`).
6. Specialized documents such as `CLIENT_ACCESS.md`, `REMOTE_CLOUD_PROVISIONING.md`, and `BASE_IMAGES.md`.
7. `00A_PLUGIN_ARCHITECTURE.md` — extension/adapter guidance.
8. `90_CODEX_IMPLEMENTATION_HANDOFF.md` — implementation and maintenance workflow.
9. `91_IMPLEMENTATION_REFERENCE_NOTES.md` — non-normative references and historical notes.
10. ADRs under `adr/` — scoped decisions.

`README.md`, `CODEX_START_HERE.md`, and `Hacocoon_v0.1-v0.7_MASTER.md` are entry points/indexes; they summarize rather than redefine the architecture.

## Current documentation state

- v0.1-v0.10 specifications are **versioned design contracts**.
- v0.1-v0.8 have implementation passes represented in the repository.
- v0.9 Base Images & Custom Environments is a design/roadmap contract; its CLI/provider implementation remains pending.
- v0.10 adds an implemented trusted session-to-Environment broker foundation outside Core.
- `IMPLEMENTATION_STATUS.md` is the source for current repository reality.
- real-provider/client acceptance remains separate from unit, integration, fake-provider E2E, race, vet, build, and CI checks.
- EC2 remains experimental and disabled by default.

## Specification vs implementation

A release specification describes a roadmap/design/acceptance contract. `IMPLEMENTATION_STATUS.md` describes **current code reality**.

Examples:

- v0.7 EC2 code can exist while real AWS acceptance remains pending.
- v0.8 `haco-vscode` can exist while real Windows/WSL + Incus + VS Code Remote-SSH acceptance remains pending.
- v0.9 can exist as a Base-image design contract before `haco image` or `haco create --base` is implemented.
- v0.10 can contain a session broker while real VS Code Agent Host/AHP + Incus per-session routing remains pending.

Do not infer release/tag readiness, production support, or implementation presence from a roadmap document alone.

## v0.8 reading path

1. `08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md`
2. `CLIENT_ACCESS.md`
3. `00_REBASELINE_AND_ROADMAP.md`
4. `IMPLEMENTATION_STATUS.md`

VS Code owns its editor, terminal, debugger, Git UI, and coding-agent UI. Hacocoon owns the Environment and external-authority boundary.

## v0.9 reading path

1. `09_v0.9_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md`
2. `BASE_IMAGES.md`
3. `00_REBASELINE_AND_ROADMAP.md`
4. `00B_SECURITY_ARCHITECTURE.md`
5. `IMPLEMENTATION_STATUS.md`

```text
logical Base name
        |
        v
immutable Base revision
        |
        v
provider-native starting point
        |
        v
Environment
```

For Incus, a provider-native starting point may be an image fingerprint, but that fingerprint is not the public Core identity. v0.9 remains implementation-pending until code actually lands.

## v0.10 reading path

1. `10_v0.10_PER_AGENT_SANDBOX_AND_AGENT_HOST.md` — authoritative v0.10 contract.
2. `10_v0.10_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md` — maintained Japanese explanation.
3. `02_v0.2_WORKSPACE_ABSTRACTION_AND_LEASE.md` — Workspace lease/concurrency rules.
4. `08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md` — existing VS Code/client boundary.
5. `09_v0.9_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md` — Base contract that v0.10 composes with.
6. `IMPLEMENTATION_STATUS.md` — implementation versus real-client acceptance.

```text
VS Code Agents window / trusted client
                 |
       trusted integration
                 |
      per-session broker
          /           \
 Environment A     Environment B
      |                 |
    Incus A           Incus B
```

The coding agent is deliberately absent from the Hacocoon management path. Session-to-Environment ownership proof is persisted in trusted state, and a deterministic Environment name alone is insufficient to authorize adoption or deletion.

Parallel write-capable sessions normally use separate Git worktrees/Workspace paths. Worktrees isolate code changes; Incus/Environment isolation is the security boundary.

VS Code Agent Host/AHP is the preferred client-integration direction, but real Agent Host/AHP + Incus per-session routing remains environment-dependent acceptance work.

## Compatibility and breaking changes

Hacocoon is pre-1.0 and actively being simplified and hardened. Breaking changes may affect CLI/helper binaries, persisted state, Core/adapter APIs, capability/policy contracts, provider configuration, Base/image lifecycle, client/agent integration, experimental backends, and documentation structure.

Prefer an explicit incompatible correction over preserving accidental compatibility that weakens the architecture. Material operator impact should remain documented and tested.

## Historical material

Historical `Session`, Runtime/Storage-centric, plugin-heavy, or advanced storage experiments may remain in Git history or as migration inventory without becoming current architecture commitments.

## Editing rule

1. update the authoritative document first;
2. update `IMPLEMENTATION_STATUS.md` when code reality changes;
3. update entry points/handoffs when their summary becomes stale;
4. keep implementation claims distinct from real-provider/client acceptance claims;
5. preserve explicit experimental/default-off provider rules;
6. update Japanese summaries when user-facing descriptions change;
7. run `python tools/check_docs.py`;
8. do not turn an implementation detail into an accidental compatibility promise.
