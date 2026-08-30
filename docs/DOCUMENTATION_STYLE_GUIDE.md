# Documentation style guide

Status: repository documentation guidance.

This guide defines how Hacocoon README files and documentation are written and organized. It does not define product behavior. Product facts remain owned by architecture, status, security, reference, and design documents.

## Goals

Documentation should feel like one project even when edited at different times by different people or agents. Optimize for:

- one clear owner for each fact;
- a short path from the repository front page to first success;
- pages organized around reader intent;
- stable semantic paths that survive release renumbering;
- consistent terminology and heading style;
- explicit implementation/status claims;
- current, copy-pasteable examples;
- English/Japanese semantic parity;
- precise trust-boundary language;
- low maintenance cost during pre-1.0 change.

Do not optimize for exhaustive repetition or making every page explain the whole project.

## Choose the page type first

| Type | Reader question | Typical content |
|---|---|---|
| Concept | How does this work? | Mental model, relationships, architecture concepts |
| Tutorial | Can I get one successful result? | Guided first experience |
| How-to | How do I accomplish this task? | Prerequisites, steps, expected result, diagnostics, cleanup |
| Reference | What exactly is available? | Commands, flags, configuration, schemas, interfaces, vocabulary |
| Design | What contract or boundary defines this feature? | Goals, non-goals, ownership, invariants, behavior, acceptance |
| Status / roadmap | What exists now or comes later? | Repository reality, milestone mapping, planned/deferred work |

A page may link to other types, but it should have one primary job.

## Documentation paths

Use semantic paths. A feature or concept owns its address; a release number does not.

Current semantic areas are:

```text
docs/design/      feature and architecture contracts
docs/security/    security and trust-boundary documentation
docs/reference/   terminology and reference material
docs/status/      roadmap and version/status authority
```

Add `concepts/`, `tutorials/`, `guides/`, or contributor-specific subdirectories when enough pages of that type exist to justify a directory.

### Prohibited path style

Do not encode product version, milestone, or arbitrary reading order in a normal documentation filename.

Avoid version-prefixed names, numeric sort prefixes, and lettered sort prefixes. Prefer semantic names such as:

```text
design/managed-sandbox-network.md
design/plugin-architecture.md
status/versioning-and-release-status.md
```

The version in which a feature first appeared may be stated in the page body or status metadata, but version-to-feature mapping is owned by [`status/versioning-and-release-status.md`](status/versioning-and-release-status.md).

ADR sequence numbers are an exception because sequence is part of ADR identity.

## Source-of-truth order

When statements disagree, resolve them from the owning source:

1. Current code reality: [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md)
2. Version/milestone mapping: [`status/versioning-and-release-status.md`](status/versioning-and-release-status.md)
3. Product boundary and roadmap intent: [`status/architecture-and-roadmap.md`](status/architecture-and-roadmap.md)
4. Cross-cutting product constraints: [`DESIGN_PRINCIPLES.md`](DESIGN_PRINCIPLES.md)
5. Canonical vocabulary: [`reference/terminology-and-boundaries.md`](reference/terminology-and-boundaries.md)
6. Cross-cutting security rules: [`security/security-architecture.md`](security/security-architecture.md)
7. Feature contract: the relevant page under [`design/`](design/)
8. Focused operational/reference documents
9. Entry points: README files and documentation indexes

README files are intentionally last for product truth. They explain and route; they do not override owners.

## README style

The repository README is a product entry point, not a manual.

A good default order is:

1. product identity and one-sentence purpose;
2. prominent pre-1.0 warning;
3. a small set of user-visible highlights;
4. quick start / first success;
5. one short security/trust-boundary explanation;
6. links to documentation, current status, contribution, security reporting, and license.

Keep the README skimmable. Do not paste the full milestone history, complete CLI command map, detailed provider/plugin contracts, implementation-status tables, or long architecture specifications when a maintained owning document can be linked instead.

## Design documents

Design pages should normally cover, where relevant:

```text
# Feature name

Status: implemented | partial | planned | deferred
Milestone: v0.N   # only when useful; never part of the filename

## Summary
## Goals
## Non-goals
## Ownership and boundaries
## User / CLI / API behavior
## Security invariants
## Failure, retry, cancellation, and cleanup
## State and identity
## Acceptance
```

Omit sections that genuinely do not apply. Do not add boilerplate just to fill a template.

For privilege-boundary work, explicitly document who owns authority, what untrusted input can influence, which credentials/tokens/sockets/state are exposed or intentionally withheld, fail-open versus fail-closed behavior, partial-failure cleanup, retry/idempotency expectations, and audit/decision records where relevant.

## Voice and tone

Write in a direct, technical, calm voice.

Prefer concrete nouns and verbs, present tense for current behavior, explicit qualifiers for partial/planned/deferred behavior, short paragraphs, and examples near the concept they demonstrate.

Avoid hype such as “perfect”, “bulletproof”, “fully secure”, or “zero risk”. Avoid vague promises and filler such as “obviously”, “just”, “easy”, “easily”, or “simply” when the operation is not actually trivial.

Security claims should name the mechanism or boundary. Prefer “Host credentials are not mounted into the Environment” over “credentials are safe”.

## Canonical terminology

Use [`reference/terminology-and-boundaries.md`](reference/terminology-and-boundaries.md) as authority.

Important project terms include **Hacocoon**, `haco`, **Host**, **Workspace**, **Environment**, **Execution**, **Client**, **Core**, **Standard**, **Capability**, **Policy**, **Provider**, and **Plugin**.

Use product/tool spelling consistently: **Incus**, **VS Code**, **GitHub**, **containerd**, **nerdctl**, **Docker**, **Windows**, **WSL**.

Do not casually substitute `container`, `sandbox`, `session`, `machine`, or `VM` for `Environment` when the architecture concept is what you mean.

## Status language

Use these meanings consistently:

| Term | Meaning |
|---|---|
| implemented | The intended repository slice exists and maintained repository checks cover it. This does not automatically prove every real-host path. |
| partial | A meaningful slice exists, but the named feature contract is incomplete. |
| planned | Future intent; do not describe it as available. |
| deferred | Intentionally postponed while a boundary/seam may remain. |
| historical | Retained only to explain previous behavior or migration; not current product behavior. |

Keep three kinds of truth separate: architecture intent, repository reality, and real-host acceptance. A unit test, fake provider, process test, or specification is not proof of real-host acceptance.

## English and Japanese parity

When a page has an English/Japanese counterpart, keep these synchronized in the same change:

- commands and flags;
- status and milestone facts;
- warnings and security invariants;
- implemented/partial/planned/deferred meaning;
- links and target documents;
- capability lists and limitations.

Natural translation is preferred over sentence-for-sentence literal translation, but factual meaning must match.

## Markdown and headings

- Use one `#` heading per page.
- Use sentence case for English headings except proper nouns and literal product names.
- Keep heading depth logical.
- Put blank lines around headings, lists, tables, fenced blocks, and admonitions.
- Use backticks for commands, paths, flags, identifiers, and literal values.
- Prefer bullets for independent facts and tables for compact comparisons.
- Avoid HTML unless Markdown cannot express the README presentation.

Use GitHub admonitions sparingly for genuine warnings, important state, or useful tips.

## Commands and examples

Examples are contracts with the reader.

- Use current commands and flags.
- Prefer copy-pasteable examples.
- Mark placeholders clearly, such as `<workspace>` or `<opaque-id>`.
- Use `bash` for commands without a prompt.
- Use `console` or `text` when showing prompts or command output.
- Do not show privileged shortcuts that contradict the security architecture.
- Label conceptual/pseudocode examples explicitly.
- Keep `haco base` separate from optional OCI/container tooling under `haco plugin oci` unless the owning architecture changes.

## Links

Prefer repository-relative links with descriptive text. When a document moves, update inbound links in the same change. Do not keep a duplicate page solely as a redirect unless there is a deliberate compatibility reason.

Before adding a new explanation, search for the same fact elsewhere. Prefer editing the owner and linking to it over creating a third copy.

## Images and diagrams

Use diagrams to explain a boundary, lifecycle, or flow, not as decoration inside technical docs. Keep text diagrams narrow, use canonical terms, and label trust boundaries when security is the point. README images need meaningful alt text and surrounding prose that remains useful if images fail to load.

## What changes together

| Change | Usually update |
|---|---|
| New independent product feature | Design doc, implementation status, version/status authority, relevant roadmap/index/README summary |
| Feature becomes complete/partial/deferred | Implementation status, owning design/status source, summaries that would become false |
| Architecture/trust boundary changes | Owning design/security/reference docs first, then affected summaries |
| CLI command/flag changes | Owning reference/detail docs, README examples if exposed, paired translations |
| Docs-only wording/layout cleanup | Affected docs only; normally no product version bump |
| Bug fix/hardening/refactor/test/CI | Update docs only when observable behavior or documented repository reality changes; normally no product milestone |

## Review checklist

Before finishing a documentation change, verify:

- [ ] The page has one primary reader intent.
- [ ] New filenames are semantic and contain no milestone/order prefix.
- [ ] Current implementation claims agree with `IMPLEMENTATION_STATUS.md`.
- [ ] Version mapping agrees with `status/versioning-and-release-status.md`.
- [ ] Terminology agrees with `reference/terminology-and-boundaries.md`.
- [ ] Security claims preserve `security/security-architecture.md` boundaries.
- [ ] Planned/partial/deferred/historical work is not presented as implemented.
- [ ] Real-host acceptance is not inferred from repository tests/specifications.
- [ ] README remains an entry point rather than a duplicate source of truth.
- [ ] Paired English/Japanese docs remain semantically synchronized.
- [ ] Commands, paths, and links are current.
- [ ] Duplicate/stale claims were removed or linked instead of multiplied.
- [ ] No product version was bumped solely for docs/refactor/hardening/test/CI work.
- [ ] `python tools/check_docs.py` passes.
