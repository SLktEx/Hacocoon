# ADR 0102: Reuse existing Git history without granting read authority

[日本語](0102-incremental-git-history.ja.md) | English

Status: accepted for implementation; installed acceptance remains separate.

## Context

Sending a complete history for each fetch or small push makes an already cloned
repository exceed the bounded transport even when its new data is small.
The Standard integration owns this behavior; Core authority remains unchanged.

## Decision

The helper supplies up to 32 distinct SHA-1 local branch tips as fetch hints.
The broker and trusted agent accept them only for fetch. After the existing
exact-ref Policy decision and fresh remote head check, the agent excludes only
hints proven ancestors of that head. Missing and unrelated hints are ignored;
they never choose a hidden Host object or another remote ref. Revision input
contains validated OIDs only. Hints never confer push authority.

For an existing-target push, omit the listed old commit's history when available
locally. Existing preparation fetches and rechecks that exact target before strict
object import. Approval still fixes the same repository, ref and old/new OIDs;
execution retains its lease and separate decision. New-target pushes keep complete
packs until a separately reviewed negotiation exists.

Use ordinary [Git revision packing](https://git-scm.com/docs/git-pack-objects)
and [strict object import](https://git-scm.com/docs/git-index-pack). Do not enable
thin packs or shared writable Git directories. The output buffer exposes only
bounded writes, so subprocess pipe copies cannot inherit an unbounded `ReadFrom`.
Existing 32 MiB pack, message, ref-count and operation-time limits remain.

## Rejected alternatives and limits

Do not raise a memory limit to the size of an arbitrary repository, use guest
paths/configuration, fetch arbitrary have OIDs, or treat clone/fetch as approval.
Hints are a bounded optimization rather than complete Git negotiation. Unrelated
branches, unavailable common tips and large new packs can still hit the limit.
Partial/shallow clone, LFS, submodules and full large-repository transport are not
implemented by this change. A 33 MiB regression is not representative performance
acceptance. See [read and push authority](0081-git-read-and-push-authority.md).
