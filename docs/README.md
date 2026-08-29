# Documentation Map

[**日本語**](README.ja.md) | English

This file defines how to read Hacocoon documentation after the 2026-08-29 architecture rebaseline and the subsequent implementation progression through v0.8.

Hacocoon remains **pre-1.0**. The documents describe the current architecture and implemented roadmap contracts, but they do not imply API, CLI, state-format, provider, client-adapter, or configuration compatibility guarantees.

For a Japanese overview, start with [`../README.ja.md`](../README.ja.md), [`README.ja.md`](README.ja.md), and [`ARCHITECTURE_GUIDE.ja.md`](ARCHITECTURE_GUIDE.ja.md).

## Source-of-truth order

When documents appear to disagree, use this order:

1. `00_REBASELINE_AND_ROADMAP.md` — product boundary and roadmap progression.
2. `00C_TERMINOLOGY_AND_BOUNDARIES.md` — canonical architecture vocabulary.
3. `00B_SECURITY_ARCHITECTURE.md` — cross-cutting trust and security rules.
4. `IMPLEMENTATION_STATUS.md` — what `main` actually implements and what acceptance is still pending.
5. The relevant versioned release specification (`01_...` through `08_...`) — the contract for that roadmap gate.
6. Specialized design documents such as `CLIENT_ACCESS.md`, `REMOTE_CLOUD_PROVISIONING.md`, and `BASE_IMAGES.md` — detailed contracts or explicitly labeled design proposals for their subject area.
7. `00A_PLUGIN_ARCHITECTURE.md` — extension/adaptor guidance; it does not require speculative interfaces or a plugin marketplace.
8. `90_CODEX_IMPLEMENTATION_HANDOFF.md` — current implementation and maintenance workflow derived from the sources above.
9. `91_IMPLEMENTATION_REFERENCE_NOTES.md` — non-normative external references and historical notes.
10. ADRs under `adr/` — scoped decisions. If an older ADR uses superseded architecture terms, the rebaseline documents above win unless the ADR is explicitly updated.

`README.md`, `CODEX_START_HERE.md`, and `Hacocoon_v0.1-v0.7_MASTER.md` are entry points/indexes; they should summarize, not redefine, the architecture. The master filename is historical; v0.8 is authoritative through the roadmap/specification documents above.

## Current documentation state

The original rebaseline documents were written while v0.1 was the active implementation gate. The repository has since progressed through the v0.8 implementation pass.

Therefore:

- v0.1-v0.8 specifications are now read as **versioned design contracts**;
- `IMPLEMENTATION_STATUS.md` is the source for current repository reality;
- an implemented roadmap gate does not mean its public surface is frozen;
- real-provider/client acceptance remains separate from unit, integration, fake-provider E2E, race, vet, build, and CI checks;
- EC2 remains experimental and disabled by default even though its v0.7 implementation exists;
- v0.8 explicitly introduces thin Client Adapters, starting with `haco-vscode`, without moving IDE or AI UI ownership into Core;
- `BASE_IMAGES.md` is an explicitly labeled design proposal and does not by itself schedule post-v0.8 implementation;
- post-v0.8 work must not be invented merely because the numbered roadmap has progressed.

## Specification vs implementation

A release specification describes the design and acceptance contract for that roadmap stage. `IMPLEMENTATION_STATUS.md` describes the **current code reality**.

These are deliberately different claims.

For example, v0.7 may have an implemented EC2 adapter while real AWS acceptance is still pending. Likewise, v0.8 may have a `haco-vscode` implementation while real Windows/WSL + Incus + VS Code Remote-SSH acceptance is still pending. Historical packages may remain without becoming part of the supported architecture.

Do not infer release/tag readiness, compatibility guarantees, or production support solely from the presence of implementation code.

## v0.8 reading path

For the client-adapter work, read these together:

1. `08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md` — the v0.8 contract.
2. `CLIENT_ACCESS.md` — generic SSH/port access supplied by Hacocoon.
3. `00_REBASELINE_AND_ROADMAP.md` — why VS Code and orchestrators remain outside Core.
4. `IMPLEMENTATION_STATUS.md` — what is implemented and what real-client acceptance remains pending.

The intended model is:

```text
VS Code / another client
        |
 thin Client Adapter
        |
    Hacocoon
        |
 isolated Environment
```

VS Code owns its editor, terminal, debugger, Git UI, and AI/coding-agent UI. Hacocoon owns the Environment and the security boundary around external authority.

## Compatibility and breaking changes

Hacocoon is still pre-1.0 and actively being simplified and hardened.

Breaking changes may affect:

- CLI commands, helper binaries, flags, and output;
- persisted state and migration behavior;
- Core and adapter APIs;
- capability and policy contracts;
- provider configuration;
- client-adapter configuration;
- experimental backends;
- documentation structure itself.

When making a breaking change, prefer an explicit deletion/replacement over preserving accidental compatibility that weakens the architecture. Document material operator impact and add migration guidance when a safe migration path is actually supported.

## Historical material

- Historical `Session`, Runtime/Storage-centric, or plugin-heavy code may remain as migration inventory.
- Deleted or superseded documents can still appear in Git history or a stale GitHub search index. Do not use them as current specifications.
- Historical experiments such as advanced storage backing formats are not roadmap commitments unless a current specification or ADR explicitly reintroduces them.
- The old instruction to stop implementation at v0.1 is historical and no longer describes `main`.
- `Hacocoon_v0.1-v0.7_MASTER.md` retains its old filename as an entry-point artifact; it must not override the v0.8 authoritative documents.

## Editing rule

When changing architecture documentation:

1. update the authoritative document first;
2. update `IMPLEMENTATION_STATUS.md` when code reality changes;
3. update entry points and handoff documents when their summary becomes stale;
4. keep implementation claims distinct from real-provider/client acceptance claims;
5. preserve the explicit experimental/default-off status of unstable providers;
6. update Japanese summaries when their user-facing description becomes stale;
7. run `python tools/check_docs.py`;
8. do not turn an existing implementation detail into a compatibility promise by accident.
