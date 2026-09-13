# Cache generations

[日本語](cache-generations.ja.md) | English

Status: **partial**. Atomic selection of complete managed sources and detached
Incus cache-volume copying are implemented on this development candidate.
Host configuration, selected-path collection from ordinary Environments,
Environment enrollment/placement, generation history, batch clearing and real
cache-workflow acceptance remain planned. There is no public cache command yet.
See [remaining work](../status/architecture-and-roadmap.md).

## Intended daily use

Host settings select only reproducible cache directories, their placement and
sharing/compatibility scope. Normal development in an ordinary Environment
produces the data. A stopped, enrolled producer may publish one complete new
generation. A compatible new Environment chooses its own Base and receives an
independent CoW copy. It never shares a writable common cache with other Envs.
Host does not build the cache or execute guest-origin data as trusted tooling.

Unconfigured files, source code, irreplaceable outputs and credentials are outside
the selection. HOME, repository-relative and multi-repository resolution must
reject escapes through links and preserve unconfigured data. These collection
and placement boundaries are not implemented by the generation foundation.

## Atomic publication contract

Core represents a named source, opaque compatibility digest, random reset epoch,
monotonic generation number and exact owned resource reference. Standard chooses
the meaning of compatibility; Core does not inspect tools, files or Btrfs.
Number zero represents an initialized source without published data.

Publication uses the existing persistent-resource creation protocol: record exact
ownership, create the provider resource, prepare all content, verify it and commit
it ready. A catalog transaction then compares the complete expected source and
selects the ready source-only candidate. No partially prepared candidate is current.

If A and B started from generation 1 and A publishes generation 2, B is skipped.
A stale producer detected before preparation allocates nothing. A conflict detected
after preparation cleans only that producer's exact candidate, through the common
delete/positive-absence/finalization boundary. Neither path merges individual
files or replaces another Environment's local cache.

A preparation failure retains its creating resource for recovery. An uncertain
selection write retains the complete candidate, because it might be current.
An uncertain delete retains the deleting resource and ownership. Skipped, failed,
recovery-required and cleanup-required results remain distinct.

The current source blocks ordinary, reviewed and exact-owner deletion in the
shared catalog transaction. Reset clears only the selection, creates a new epoch
and retains old resources. Producers from before reset remain stale even when
the generation number is zero again. Reset does not erase CoW copies, Workspace,
OCI data or snapshot references, and is not proof of physical space reclamation.

## Provider and persistence boundaries

The Incus adapter recognizes `build-cache` volumes through the existing owned
custom-volume create, verify, copy and delete mechanisms. Copies require Btrfs,
fresh ownership, matching resource kinds and detached sources. Source config is
not copied wholesale. Attached cache sources are refused; they cannot use trusted
Host OCI pause/resume or OCI attachment/maintenance/import paths.

Catalog format 15 adds generation selections. Supported earlier formats remain
readable without writes during observation; the first mutation upgrades the
catalog. Format 9 remains unsupported. Generation fields under earlier formats,
invalid ownership or incomplete selected resources fail closed. Older binaries
that do not understand format 15 must refuse it; do not replace an installed
controller merely to run these tests.

## Completion still required

The foundation does not yet define the public Host configuration format, securely
resolve configured paths, enroll pre-existing Envs, attach multiple cache areas,
collect stopped writers, track Env origin/history or clear selected/all Env copies.
Those changes must extend canonical lifecycle ownership rather than assemble
independent resource/lease operations in orchestration code. Env-local cache copies
must be disposable while contracted Workspace and OCI data remain retained.

Real Incus/Btrfs acceptance must measure shared extents and independent mutation,
then the complete ordinary-Env flow across compatible different Bases. Large-repo
capacity and time claims require large-repo measurements. Small catalog/provider
fixtures prove neither cache collection nor giant-repository performance.

The ownership decision and rejected alternatives are recorded in
[ADR 0076](../adr/0076-atomic-managed-data-generations.md).
