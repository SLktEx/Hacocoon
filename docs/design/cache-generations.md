# Cache generations

[日本語](cache-generations.ja.md) | English

Status: **partial**. Atomic selection of complete managed sources and detached
Incus cache-volume copying are implemented on this development candidate.
Env-owned disposable resource reservation, materialization and cleanup now share
canonical lifecycle transitions. Linux Incus rootfs placement and bound resume
are implemented candidates. Host configuration is not enabled. Selected-path collection, enrollment/placement, history, clearing and real
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

## Env-owned disposable data

Status: **implemented lifecycle and Linux rootfs-placement slices**. A trusted
Host selector supplies named areas and exact origin generations. The common Env
transaction reserves the Workspace, retained OCI reference, all fresh child
identities and any source-copy reservations before a provider request. Ordinary
resource create/copy/delete cannot adopt or release an Env-owned child.

Each Env keeps a bounded, canonically ordered list of independent data references,
placement targets and immutable origin receipts. Origin is provenance after copy
completion: advancing/resetting/deleting a former source does not invalidate an
existing Env. In-flight copying still pins its source through the shared catalog.
The existing exclusive copy reservation is retained; simultaneous uses of one
source may be refused as busy while preparation is unfinished.

Materialization claims each planned child exactly once. An empty volume records
positive creation before verification; a copy uses the existing completed-copy
receipt and recovery. Failed or ambiguous creation remains owned. No runtime is
requested until every selected resource is ready. Unsupported runtimes refuse
selection before reservation instead of silently dropping areas.

Canonical Env deletion first proves complete runtime/network absence, then records
that fact durably and fences child creation. Shared resource deletion verifies
absence of each exact child before releasing its ownership. The Env and Workspace
lease survive until all children are absent. Unstarted children can be cancelled;
unknown in-flight creation cannot. Child cleanup continues independently after a
sibling fails. Retained Workspace and OCI data are never included in child cleanup.
A retry after the durable runtime-absence receipt does not delete that runtime name
again. A missing runtime reference alone is not evidence of absence.

Incus creation with a durable receipt can place these areas in the rootfs.
Restore/archive and snapshot planning still refuse them; snapshot refusal precedes
quiescing so an unsupported capture does not stop the producer. Existing Envs
without added areas keep their snapshot/copy/transfer paths. The Standard selector
is not registered in production and there is no public cache operation.

## Rootfs placement and resume

Linux Incus validates an exact binding of the creation identity, every area,
origin and native resource. Each disposable custom volume also carries its parent
creation identity. The initial instance records the binding before any device is
added. All volumes and paths are verified before the first attachment, and every
device and exclusive native user is checked before starting the guest.

Supported destinations are explicit descendants of `/root`, individual `/home`
directories and `/var/cache`. Home directories themselves, system/control paths
and credential/configuration directories such as `.ssh`, `.aws`, `.config` and
`.git` are refused. These provider limits do not determine which caches are
reproducible or compatible; that remains trusted Host/Standard selection.

Stopped-instance Incus file metadata checks every ancestor without following
links. A destination must be absent or an empty directory. Existing content is
neither hidden nor adopted. Metadata reads are bounded and cancellable; no guest
program attests to safety and no guessed daemon storage path is opened. Another
disk overlapping the destination is refused, because the stopped file API sees
the rootfs rather than the custom disk. Repository-relative placement and external
Workspace enrollment therefore remain required work, not accepted paths.

Manual and client-triggered resume share the canonical lease-based dispatch.
The current catalog supplies the complete resources under the Env lifecycle lock.
Incus verifies creation identity, binding, exact devices and exclusive native use;
stopped resume repeats the path checks. Reference-only resume refuses a data-bound
Env. Added/missing devices, owner drift or path drift never trigger automatic
repair, deletion or a fallback start. See [ADR 0077](../adr/0077-environment-owned-disposable-data.md).

## Provider and persistence boundaries

The Incus adapter recognizes `build-cache` volumes through the existing owned
custom-volume create, verify, copy and delete mechanisms. Copies require Btrfs,
fresh ownership, matching resource kinds and detached sources. Source config is
not copied wholesale. Attached cache sources are refused; they cannot use trusted
Host OCI pause/resume or OCI attachment/maintenance/import paths.

Catalog format 15 introduced generation selections; format 16 adds Env-owned
children and runtime-absence receipts. Supported earlier formats remain
readable without writes during observation; the first mutation upgrades the
catalog. Format 9 remains unsupported. Generation fields under earlier formats,
invalid ownership or incomplete selected resources fail closed. Format-16
attachment fields in older catalogs, orphan children, conflicting identities,
nonmatching Env/lease data and invalid creation/absence states also fail closed. Older binaries
that do not understand format 16 must refuse it; do not replace an installed
controller merely to run these tests.

## Completion still required

The foundation does not yet define the public Host configuration format, securely
resolve repository-relative paths, enroll pre-existing Envs,
collect stopped writers, expose origin/history or clear selected/all Env copies.
Those changes must extend canonical lifecycle ownership rather than assemble
independent resource/lease operations in orchestration code. Env-local cache copies
must be disposable while contracted Workspace and OCI data remain retained.

Real Incus/Btrfs acceptance must measure shared extents and independent mutation,
then the complete ordinary-Env flow across compatible different Bases. Large-repo
capacity and time claims require large-repo measurements. Small catalog/provider
fixtures prove neither cache collection nor giant-repository performance.

The ownership decision and rejected alternatives are recorded in
[ADR 0076](../adr/0076-atomic-managed-data-generations.md).

Small synthetic Incus/Btrfs provider measurements are recorded in [acceptance evidence](../status/acceptance-evidence.md#cache-generation-foundation); they do not establish the normal-Env workflow.
