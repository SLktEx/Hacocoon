# Documentation Map

This file defines how to read Hacocoon documentation after the 2026-08-29 architecture rebaseline.

## Source-of-truth order

When documents appear to disagree, use this order:

1. `00_REBASELINE_AND_ROADMAP.md` — product boundary and release order.
2. `00C_TERMINOLOGY_AND_BOUNDARIES.md` — canonical architecture vocabulary.
3. `00B_SECURITY_ARCHITECTURE.md` — cross-cutting trust and security rules.
4. The **current release specification** — currently `01_v0.1_SECURE_WORKSPACE_RUNTIME.md`.
5. `IMPLEMENTATION_STATUS.md` — what the repository actually implements today.
6. Future release documents (`02_...` through `07_...`) — planning only; they must not expand the current release gate.
7. `00A_PLUGIN_ARCHITECTURE.md` — extension/adaptor guidance, not a requirement to create interfaces or plugins now.
8. `90_CODEX_IMPLEMENTATION_HANDOFF.md` — implementation workflow derived from the sources above.
9. `91_IMPLEMENTATION_REFERENCE_NOTES.md` — non-normative external references and historical notes.
10. ADRs under `adr/` — scoped decisions. If an older ADR uses superseded architecture terms, the rebaseline documents above win unless the ADR is explicitly updated.

`README.md`, `CODEX_START_HERE.md`, and `Hacocoon_v0.1-v0.7_MASTER.md` are entry points/indexes; they should summarize, not redefine, the architecture.

## Specification vs implementation

The release specification describes the **target gate**. `IMPLEMENTATION_STATUS.md` describes the **current code reality**.

For example, historical code may still expose `Session`, `haco new`, `haco rm`, or advanced storage commands while the v0.1 target uses Workspace/Environment terminology and `haco create` / `haco delete`. Existing code is not automatically current product scope.

## Historical material

- Historical `Session`, Runtime/Storage-centric, or plugin-heavy code may remain during migration.
- Deleted/superseded documents can still appear in Git history or a stale GitHub search index. Do not use them as current specifications.
- Historical experiments such as advanced storage backing formats are not roadmap commitments unless a current release document or ADR explicitly reintroduces them.

## Editing rule

When changing architecture documentation:

1. update the authoritative document first;
2. update summaries/status documents only as needed;
3. run `python tools/check_docs.py`;
4. keep future-release ideas from leaking into the current implementation gate.
