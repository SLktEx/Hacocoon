# ADR 0008: Keep repository clones and Git authority separate

Status: accepted for implementation  
Date: 2026-09-06

## Context

The WSL PoC requires ordinary SSH and Git fetch/pull/push on an independent
Workspace. The historical external-path Workspace and Host Git capability do
not provide that workflow: they can share writable Git metadata and assume
that the controller can read a local checkout. The trusted logical Host must
own the upstream repository and credentials while the Physical Host remains
the sole controller and Incus owner.

## Decision

- A maintained Git integration prepares a registered repository on an
  Incus-owned Btrfs custom volume mounted only in trusted `haco-host`.
- Each managed Workspace is a separate Incus volume copy, including its own
  `.git`. Core receives an opaque Workspace source and stable identity through
  its WorkspaceProvider; Incus resolves the storage reference in its adapter.
- Volume copy carries Incus's existing `volatile.idmap.last/next` with the
  already shifted filesystem. Dropping that bookkeeping shifts owners twice
  on the next mount. Do not compensate with recursive chown or a privileged
  Environment; preserve Incus's copy contract instead.
- Volume ownership is recorded by the controller before creation and included
  in the provider creation request. Uncertain creation retains that record as
  recovery-required. No automatic deletion or adoption is part of this PoC.
- Environment creation continues through the canonical Workspace/Environment
  transition API. Graceful stop retains its metadata, lease and Workspace.
- Each Environment receives a Git-only Unix endpoint bound by the controller
  to that Environment and registered repository. It never receives the
  management endpoint, upstream credentials or a general trusted-host shell.
- `git-remote-haco` implements ordinary Git helper fetch/push. The trusted side
  derives the upstream URL and branch from registration, validates transferred
  Git objects and binds approval to repository, remote, ref, old/new OIDs and
  operation. Only the pinned commit is pushed, with an exact remote-ref lease.
- Approval is collected through the trusted controller client. Guest bytes,
  guest Git configuration and helper URLs cannot grant approval or select a
  different repository. Force, deletion and multiple-ref operations are
  explicitly unsupported in the initial workflow.

## Rejected alternatives

Sharing the trusted `.git`, Git alternates or a worktree common directory would
give the Environment control over trusted repository metadata. Mounting the
controller socket or forwarding a general SSH shell would grant unrelated Host
authority. Running authenticated Git against a guest-controlled directory
would import guest hooks/configuration into the trusted process. Directly
managing Btrfs subvolumes would add another storage lifecycle owner. None of
these shortcuts is required by the PoC.

## Consequences

The first workflow uses explicit repository and Workspace preparation commands
and a separate trusted terminal for approval. Small, bounded Git object
transfers are sufficient; large repositories and generic recovery are deferred.
Workspace volume retention does not depend on the runtime's lifetime. Provider
and end-to-end acceptance must establish actual COW copying and the SSH/Git
journey separately from unit tests.
