# Documentation Map

[**日本語**](README.ja.md) | English

This file defines how to read Hacocoon documentation after the 2026-08-29 architecture rebaseline, the implementation progression through v0.8, and the explicit v0.9 Base-image roadmap decision.

Hacocoon remains **pre-1.0**. The documents describe the current architecture, implemented roadmap contracts, and scheduled design contracts, but they do not imply API, CLI, state-format, provider, client-adapter, image, or configuration compatibility guarantees.

For a Japanese overview, start with [`../README.ja.md`](../README.ja.md), [`README.ja.md`](README.ja.md), and [`ARCHITECTURE_GUIDE.ja.md`](ARCHITECTURE_GUIDE.ja.md).

## Source-of-truth order

When documents appear to disagree, use this order:

1. `00_REBASELINE_AND_ROADMAP.md` — product boundary and roadmap progression.
2. `00C_TERMINOLOGY_AND_BOUNDARIES.md` — canonical architecture vocabulary.
3. `00B_SECURITY_ARCHITECTURE.md` — cross-cutting trust and security rules.
4. `IMPLEMENTATION_STATUS.md` — what `main` actually implements and what acceptance is still pending.
5. The relevant versioned release specification (`01_...` through `09_...`) — the contract for that roadmap gate.
6. Specialized design documents such as `CLIENT_ACCESS.md`, `REMOTE_CLOUD_PROVISIONING.md`, and `BASE_IMAGES.md` — detailed contracts or explicitly labeled design proposals for their subject area.
7. `00A_PLUGIN_ARCHITECTURE.md` — extension/adaptor guidance; it does not require speculative interfaces or a plugin marketplace.
8. `90_CODEX_IMPLEMENTATION_HANDOFF.md` — current implementation and maintenance workflow derived from the sources above.
9. `91_IMPLEMENTATION_REFERENCE_NOTES.md` — non-normative external references and historical notes.
10. ADRs under `adr/` — scoped decisions. If an older ADR uses superseded architecture terms, the rebaseline documents above win unless the ADR is explicitly updated.

`README.md`, `CODEX_START_HERE.md`, and `Hacocoon_v0.1-v0.7_MASTER.md` are entry points/indexes; they should summarize, not redefine, the architecture. The master filename is historical; v0.9 is authoritative through the roadmap/specification documents above even though the implementation currently remains at v0.8.

## Current documentation state

The original rebaseline documents were written while v0.1 was the active implementation gate. The repository has since progressed through the v0.8 implementation pass, and the next explicit roadmap gate is now v0.9.

Therefore:

- v0.1-v0.9 specifications are **versioned design contracts**;
- v0.1-v0.8 have implementation passes represented on `main`;
- v0.9 Base Images & Custom Environments is scheduled as a design/roadmap gate but is not yet an implementation claim;
- `IMPLEMENTATION_STATUS.md` is the source for current repository reality;
- an implemented roadmap gate does not mean its public surface is frozen;
- real-provider/client acceptance remains separate from unit, integration, fake-provider E2E, race, vet, build, and CI checks;
- EC2 remains experimental and disabled by default even though its v0.7 implementation exists;
- v0.8 explicitly introduces thin Client Adapters, starting with `haco-vscode`, without moving IDE or AI UI ownership into Core;
- v0.9 introduces selectable logical Bases that resolve to immutable Environment starting revisions while keeping Incus aliases/fingerprints behind the adapter boundary;
- `BASE_IMAGES.md` remains the detailed companion proposal; the authoritative minimum v0.9 gate is `09_v0.9_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md`;
- post-v0.9 work must not be invented merely because the numbered roadmap has progressed.

## Specification vs implementation

A release specification describes the design and acceptance contract for that roadmap stage. `IMPLEMENTATION_STATUS.md` describes the **current code reality**.

These are deliberately different claims.

For example, v0.7 may have an implemented EC2 adapter while real AWS acceptance is still pending. Likewise, v0.8 may have a `haco-vscode` implementation while real Windows/WSL + Incus + VS Code Remote-SSH acceptance is still pending. v0.9 may exist as a design contract before `haco image` or `haco create --base` exists in code.

Do not infer release/tag readiness, compatibility guarantees, production support, or implementation presence solely from a roadmap document.

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

## v0.9 reading path

For Base-image/custom-Environment work, read these together:

1. `09_v0.9_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md` — authoritative minimum v0.9 roadmap/acceptance contract.
2. `BASE_IMAGES.md` — detailed design companion for identity, import/build, deletion, GC, and concurrency.
3. `00_REBASELINE_AND_ROADMAP.md` — placement of Base selection inside the Environment boundary.
4. `00B_SECURITY_ARCHITECTURE.md` — custom images do not grant host/external authority.
5. `IMPLEMENTATION_STATUS.md` — confirms that v0.9 is not yet implemented until code actually lands.

The intended identity model is:

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

For Incus, the provider-native starting point may be an image fingerprint, but that fingerprint is not the public Core identity.

## Compatibility and breaking changes

Hacocoon is still pre-1.0 and actively being simplified and hardened.

Breaking changes may affect:

- CLI commands, helper binaries, flags, and output;
- persisted state and migration behavior;
- Core and adapter APIs;
- capability and policy contracts;
- provider configuration;
- client-adapter configuration;
- Base/image configuration and lifecycle;
- experimental backends;
- documentation structure itself.

When making a breaking change, prefer an explicit deletion/replacement over preserving accidental compatibility that weakens the architecture. Document material operator impact and add migration guidance when a safe migration path is actually supported.

## Historical material

- Historical `Session`, Runtime/Storage-centric, or plugin-heavy code may remain as migration inventory.
- Deleted or superseded documents can still appear in Git history or a stale GitHub search index. Do not use them as current specifications.
- Historical experiments such as advanced storage backing formats are not roadmap commitments unless a current specification or ADR explicitly reintroduces them.
- The old instruction to stop implementation at v0.1 is historical and no longer describes `main`.
- `Hacocoon_v0.1-v0.7_MASTER.md` retains its old filename as an entry-point artifact; it must not override the v0.9 authoritative documents.

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
