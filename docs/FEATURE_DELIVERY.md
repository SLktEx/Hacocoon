# Feature Delivery Contract

> **One independently useful product feature = one numeric implementation milestone.**

Hacocoon is pre-1.0. Its `v0.x` numbers are implementation/product milestones, not compatibility promises or Git release tags. A feature is complete only when implementation, version state, documentation, specification, and tests move together.

The repository-root `VERSION` is the machine-readable highest milestone with implementation in the repository. It is currently `0.17`, matching the v0.17 Docker Compatibility Plugin foundation already present on `main`.

## Required feature delivery

When a PR makes a new independently useful product capability available, it must in the same change:

1. use a PR title beginning with `feat:` or `feat(scope):`;
2. advance root `VERSION` exactly one numeric milestone;
3. update the owning versioned specification;
4. update authoritative numbering and implementation-status documents;
5. update roadmap, docs indexes, and root READMEs that describe current reality;
6. update maintainer/coding-agent handoff documents;
7. add/update tests and acceptance notes appropriate to the capability.

If `VERSION` is `0.17`, the next independently useful feature is `0.18`.

Multiple implementation PRs may contribute to one coherent already-numbered feature, but supporting slices that do not yet create another independently useful capability are not separate feature deliveries and must not consume another milestone merely because they are separate PRs.

## What normally does not consume a milestone

The following normally keep the current `VERSION`:

- bug fixes that restore an already documented contract;
- security hardening without a new capability;
- refactors, deletions, and CLI namespace cleanup;
- docs/test/CI/release-engineering-only changes;
- acceptance or completion work that remains inside an already-numbered feature contract.

If one of those changes also adds a new independently useful capability, that capability is feature delivery and must advance the milestone.

## Required documentation set

A non-bootstrap `VERSION` change must update all of these in the same PR:

- `README.md`
- `README.ja.md`
- `CODEX_START_HERE.md`
- `docs/README.md`
- `docs/README.ja.md`
- `docs/00_REBASELINE_AND_ROADMAP.md`
- `docs/00D_VERSIONING_AND_RELEASE_STATUS.md`
- `docs/00D_VERSIONING_AND_RELEASE_STATUS.ja.md`
- `docs/IMPLEMENTATION_STATUS.md`
- `docs/IMPLEMENTATION_STATUS.ja.md`
- `docs/90_CODEX_IMPLEMENTATION_HANDOFF.md`
- the owning `docs/NN_v0.x_*.md` specification

Subsystem-specific operational docs also change when their contract or usage changes.

## Local validation

Normal docs validation:

```bash
bash tools/ci-local.sh docs
```

Feature delivery validation against the branch/ref the work started from:

```bash
HACO_FEATURE_BASE=main bash tools/ci-local.sh feature
```

Equivalent direct invocation:

```bash
python3 tools/check_feature_delivery.py --base main --feature
```

GitHub Actions reads the pull-request title and base SHA automatically. A `feat:` PR with no version advance fails. A non-bootstrap version advance with a non-`feat:` title fails. Skipped numeric milestones, missing required docs, and missing owning specifications fail as well.

## Bootstrap note

`VERSION` is introduced at `0.17` after v0.13-v0.17 had already been rebaselined in documentation. Adding the marker itself is maintenance work and does not consume v0.18.

## Sources of truth

- `VERSION` — machine-readable current numeric implementation milestone.
- `00D_VERSIONING_AND_RELEASE_STATUS.md` — human-readable milestone numbering/status authority.
- `IMPLEMENTATION_STATUS.md` — exact repository implementation reality.
- versioned specification — feature contract and acceptance intent.
- roadmap/README/handoff documents — current discovery and implementation context.

For future feature delivery these sources move together.
