# ADR 0081: Separate Git branch reads and push authority

[日本語](0081-git-read-and-push-authority.ja.md) | English

Status: accepted for implementation; native acceptance is separate.
Date: 2026-09-15

## Context

Developers need to fetch other branches and push a new working branch with ordinary
Git. The registered checkout branch is provenance, not external write permission.
This main integration reuses #585 and #587 in the Standard Git integration.

## Decision

All-head discovery requires a fetch decision for `refs/heads/*`. The trusted
agent returns a bounded list of literal head names and commit OIDs; the broker
checks each exact ref before exposing the list and again before fetching objects.
An exact-ref deny cannot be bypassed through wildcard discovery. The fetched
commit must still match the advertised head. Workspace copies use a wildcard
fetch mapping and independent Git data.

Push preparation requests fetch authority for the exact target, imports only Git
object bytes and fixes the new commit. Execution requests separate push authority
bound to repository, upstream, Environment creation, target ref and old/new OIDs
through the existing Policy/Approval service. Clone/fetch never saves push permission.

A new target has an all-zero old OID and must remain absent. The trusted agent
uses Git's explicit empty lease expectation. An existing target requires a
fast-forward and the exact observed old OID. The reviewed commit is sent as an OID,
not a branch name that could move during approval. Saved choices bind the exact
ref and distinguish `create` from `fast-forward`; neither grants the other,
another branch, or main. Main's require-approval rule retains precedence.

Git's porcelain result must identify one exact ref, commit and expected mutation
status. An identical branch created concurrently may report up-to-date despite
a successful process exit; it does not prove our creation occurred. Unknown
external results require inspection and never trigger automatic push replay.
See the upstream [Git push lease and output contract](https://git-scm.com/docs/git-push)
and [remote-helper protocol](https://git-scm.com/docs/gitremote-helpers).

Core gains no Git-specific authorization rules. Registered Host source ownership,
Workspace leases and Environment generation are rechecked by existing common
services. Guest paths, configuration, remotes, hooks and credentials cannot replace
the trusted source. Reusable Host credentials stay outside ordinary Environments.

## Rejected alternatives and limits

Deriving push permission from clone, widening an exact-ref saved choice, using an
implicit/wildcard lease, accepting arbitrary object IDs for reads, trusting exit
status alone, and forwarding guest Git configuration are rejected.

The initial transport bounds a batch at 1024 heads and 32 MiB total pack data.
Per-head transfers can repeat history; large-repository transport and measurement
remain separate work. Force push, deletion, multi-ref push, LFS and submodules
remain deferred. Real Git component tests do not establish authenticated GitHub,
installed Environment, GUI or large-repository acceptance. Old-version migration
is outside the current user-authorized scope.
