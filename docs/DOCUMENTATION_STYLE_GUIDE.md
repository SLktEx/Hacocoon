# Documentation ownership and writing guide

This is the single place for documentation roles, ownership and update rules.
It governs writing, not product behavior.

## Choose an owner

| Reader need / fact | Owner |
|---|---|
| First successful installation and development session | [Getting started](guides/getting-started.md) |
| Complete a specific operation | `docs/guides/` |
| Exact commands, configuration, vocabulary and interfaces | `docs/reference/` |
| Feature semantics, invariants and failure behavior | `docs/design/` |
| Cross-cutting trust boundaries | [Security architecture](security/security-architecture.md) |
| Product constraints | [Design principles](DESIGN_PRINCIPLES.md) |
| Current code reality and feature limits | [Implementation status](IMPLEMENTATION_STATUS.md) |
| Commit-bound real-host results, unresolved failures and skips | [Acceptance evidence](status/acceptance-evidence.md) |
| Remaining work and future direction | [Roadmap](status/architecture-and-roadmap.md) |
| Checkpoint numbering/current/Gate identity | [checkpoints.yaml](status/checkpoints.yaml) |
| Numbering policy and human-readable checkpoint history | [Versioning and release status](status/versioning-and-release-status.md) |
| Why a consequential decision was made; rejected alternatives | `docs/adr/` |
| Discovery and routing | Repository, documentation and module README files |

A fact has one detailed owner. Summaries link to it. README files are entry points,
not competing specifications. A module README explains its module's use and
contribution entry, then links to the owning feature contract.

## Resolve disagreements

Verify observable behavior against current main code, CLI help, tests and configuration
readers. The implementation-status page summarizes that evidence; it cannot override
code. Design intent, roadmap plans and real-host acceptance are separate claims.
Do not silently choose whichever conflicting sentence sounds convenient.

Use [canonical terminology](reference/terminology-and-boundaries.md) and the
cross-cutting security/product constraints when interpreting feature designs.
Read the relevant ADR for rationale. Older ADRs describe decisions at their date;
mark superseded decisions and link to the current contract without renumbering them.
README prose never overrides these owners.

## Structure and paths

Use stable semantic paths. Normal filenames must not encode a version, milestone,
or arbitrary reading order. ADR sequence numbers are part of identity and are the
exception. Keep a page focused on one reader question; omit empty template sections.

Do not create generated master documents, handoff notes or duplicate “start here”
pages. Remove obsolete operating instructions and intermediate routing pages.
Git history is the archive; moving whole obsolete documents to an archive directory
does not solve duplication.

## Update rules

1. Update the owning contract/reference first, then any summaries made false.
2. Change implementation status when availability or a material limit changes.
3. Keep failed and skipped acceptance visible. Record decisive evidence with an exact
   commit/run, scope and remaining gaps; do not append a daily execution diary.
4. Put open work in the roadmap. Preserve unique rationale, rejected unsafe alternatives
   and unresolved failures when merging or shortening documents.
5. Follow [checkpoint policy](status/versioning-and-release-status.md) for meaningful
   progress. Documentation cleanup alone does not advance a version/checkpoint.
6. Update English/Japanese companions together for commands, prerequisites, data lifetime,
   permissions, status, limitations and links. Natural translation is preferred.
7. On moves/deletions, update inbound links, heading anchors, images, tool/CI references
   and required verification targets in the same change.

## Status words

- **implemented**: the repository implements the stated slice; it does not prove every Host.
- **partial**: a useful part exists; explicitly name missing behavior and acceptance.
- **planned**: intended but not implemented.
- **deferred**: deliberately postponed.
- **historical**: previous behavior or evidence, not a current operating instruction.

## Write for the reader

Lead with the result or concept. Use short, direct paragraphs, descriptive headings
and tables for actual comparisons. Explain permissions and what survives stop/delete
near the relevant operation. Avoid repeated preambles and unsupported security promises.

Use natural Japanese. Keep product names, commands, identifiers and necessary terms;
translate ordinary prose instead of embedding English phrases throughout a sentence.
Retain precise Host, Workspace, Environment, Base and OCI Store distinctions.

Command examples specify where they run, prerequisites, placeholders, expected results
and cleanup/data consequences. Verify flags/defaults against their actual command parser.
Separate product `haco` from temporary `hacoq` in [migration information](reference/cli-migration.md).
Do not imply that a design API or development fixture is a shipped product command.

Use one H1, logical heading depth, sentence-case English headings, fenced code with
language tags, and blank lines around blocks. Prefer descriptive repository-relative
links. Explicit HTML anchors are useful only for a stable cross-language target;
otherwise use the actual GitHub heading anchor. Diagrams should clarify relationships
or trust boundaries, not decorate a technical page.

## Verify changes

Run from the repository root:

```bash
python tools/check_docs.py
bash tools/ci-local.sh docs
```

The checker covers semantic paths, local links/anchors/images, tool references and
checkpoint mirrors. Its regressions must continue rejecting real defects.
Also walk the beginner journey, check examples against code/help and review translation
parity. Automated link checks cannot establish product correctness or real-host acceptance.
