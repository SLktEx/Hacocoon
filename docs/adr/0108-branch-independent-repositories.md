# ADR 0108: Register repositories independently of checkout branches

[日本語](0108-branch-independent-repositories.ja.md) | English

Status: accepted. Refs #709.

## Decision

Repository registration names a Git remote through `haco repo add <id> <remote>`
and `repository.add`. A branch belongs to a Workspace's initial checkout or an
individual Git operation, never to source identity or broker source matching.
This supersedes the branch-from-registration portion of
[ADR 0008](0008-managed-repository-workspaces.md).

Keep the existing owned source volume and independent Workspace volume copies.
The source is a normal all-heads clone without checkout. A fresh copy refreshes
upstream heads and checks out an explicitly selected branch or the current
remote symbolic HEAD, then receives only the non-authorizing helper config.
Never run this authenticated preparation against imported or restored guest
data. Source owner, registered remote, Environment identity and exact-ref Policy
remain mandatory. Missing default HEAD does not invalidate other heads; an
explicit branch can prepare a Workspace in that case.

The shared persisted object uses `branch` only as optional Workspace provenance.
Source records with a branch fail closed as incompatible. This change does not
automatically rewrite old registrations or delete their owned volumes. Saved
Workspace branches remain valid provenance and need not match registration.
Snapshot/transfer routing permits a registered remote without branch metadata;
an absent remote remains offline and cannot acquire authority by a matching name.

## Alternatives and consequences

A rename retaining required/default branch metadata would preserve the wrong
identity. Separate registrations per branch would duplicate ownership and Git
authority. Shared worktrees or alternates would expose trusted Git metadata to
guest writes. A bare/mirror layout would require unnecessary changes to the
current volume-copy contract; an ordinary clone already retains all heads.

The pre-1.0 [compatibility policy](../../CONTRIBUTING.md) permits deliberate
removal. The old CLI and API are removed rather than keeping an alias whose
`--branch` is ignored or wrongly interpreted as registration identity. Initial
selection is the optional `workspace create --branch` flag for one source;
collections and `workspace prepare` use each current remote default. Reopening,
forking and importing preserve existing work and never reset its checkout.

The [Git design](../design/git-and-github-capability.md#branch-independent-registration)
owns detailed semantics and observation/refusal rules. Tests cover branchless
CLI/API/storage, independent multi-branch copies, changes after registration,
missing heads and preserved broker ownership checks. Local Git and repository
tests do not establish native Incus or installed Windows acceptance.
