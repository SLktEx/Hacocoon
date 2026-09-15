# Cache generations

[日本語](cache-generations.ja.md) | English

Status: **partial**. Host settings, creation-time enrollment, stopped whole-area
collection and independent generation reuse are implemented on main.
Use the public commands below on the trusted Host. Existing-Env enrollment,
unknown-copy recovery remain incomplete.
Real-host results and large-repository performance are separate acceptance claims.

## Intended daily use

Host settings select only reproducible cache directories, their placement and
sharing/compatibility scope. Normal development in an ordinary Environment
produces the data. A stopped, enrolled producer may publish one complete new
generation. A compatible new Environment chooses its own Base and receives an
independent CoW copy. It never shares a writable common cache with other Envs.
Host does not build the cache or execute guest-origin data as trusted tooling.

Unconfigured files, source code, irreplaceable outputs and credentials are outside
the selection. Placement supports explicit HOME and managed-repository targets,
including individual collection members, and rejects links or existing content.
Only creation-time enrollment is supported; existing content is not adopted.

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

Status: **implemented lifecycle and Linux placement slices**. A trusted
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

Incus creation with a durable receipt can place these areas in the rootfs or a managed Workspace.
Snapshot and independent copy retain every enrolled area through the saved aggregate.
Portable archive transfer includes every added area with fresh destination ownership. The Standard selector reads the trusted Host settings for each new Environment.
Unconfigured installations select no areas.

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
program attests to safety and no guessed daemon storage path is opened. Rootfs
placement refuses overlapping disks. Managed-repository placement uses the
separate volume observation below; external Workspace enrollment remains unsupported.

Manual and client-triggered resume share the canonical lease-based dispatch.
The current catalog supplies the complete resources under the Env lifecycle lock.
Incus verifies creation identity, binding, exact devices and exclusive native use;
stopped resume repeats the path checks. Reference-only resume refuses a data-bound
Env. Added/missing devices, owner drift or path drift never trigger automatic
repair, deletion or a fallback start. See [ADR 0088](../adr/0088-environment-owned-disposable-data.md).

## Managed-repository placement

The current lease supplies the managed Workspace path and access mode to the same
provider boundary for creation and manual/client resume. The trusted repository
catalog resolves exact owned Workspace volumes, including collection members.
A cache destination must be strictly inside exactly one writable member; covering
a member root, using an external Workspace or inheriting its device is refused.
The binding includes selected native volumes and owners. Ordinary Git branch or
remote changes do not change that storage identity.

Incus must provide `file_storage_volume`, present in the supported 7.0.1 API.
The rootfs file API checks only mountpoint ancestry; the custom-volume file API
checks actual Workspace ancestors and the absent/empty destination. Missing API,
links, non-directories, existing content and uncertain metadata fail closed.
Exact explicit and expanded devices plus exclusive native use are rechecked;
other overlapping mounts, source substitutions and shifted/read-only parents
are refused. No guest helper or Host storage-path guess replaces these checks.

Canonical deletion removes only Env-owned disposable children and retains the
Workspace volumes and unconfigured data. Native acceptance for this new placement
slice is separate from the earlier rootfs fixture. The required Incus test covers
two owned members, resume, parent substitution, deletion/reuse and a guest-created
link. It does not establish Host configuration, collection or large-repo performance.

## Provider and persistence boundaries

The Incus adapter recognizes `build-cache` volumes through the existing owned
custom-volume create, verify, copy and delete mechanisms. Copies require Btrfs,
fresh ownership and matching resource kinds. Ordinary copies require detached
sources. Collection has a separate canonical reservation for one exact stopped
Env-owned child; Incus checks its sole consumer, explicit instance identity, data
device, stopped state and disabled autostart. Source config is not copied wholesale.
Cache collection never uses trusted Host OCI pause/resume or OCI maintenance.

The current catalog validates generation ownership, immutable publication origin,
producer receipts, selected ready sources and complete Env/lease bindings. In-flight
copy reservations survive process exit and block parent start, client access and
deletion. Origin receipts alone do not pin a deleted producer. Old-version
compatibility and migration are outside this development scope.

## Completion still required

Existing-Env enrollment, unknown-copy recovery and orphan-source
operations remain incomplete. Unknown copy outcomes retain ownership;
there is no automatic replay or inference of success from an existing destination.
These remaining operations must use canonical lifecycle ownership. Workspace and
OCI data remain retained when an enrolled Environment is deleted.

Real Incus/Btrfs acceptance must measure shared extents and independent mutation,
then the complete ordinary-Env flow across compatible different Bases. Large-repo
capacity and time claims require large-repo measurements. Small catalog/provider
fixtures prove neither cache collection nor giant-repository performance.

The ownership decision and rejected alternatives are recorded in
[ADR 0087](../adr/0087-atomic-managed-data-generations.md).

Small synthetic Incus/Btrfs provider measurements are recorded in [acceptance evidence](../status/acceptance-evidence.md#cache-generation-foundation); they do not establish the normal-Env workflow.


## Host target and compatibility selection

Status: **implemented Standard component; enabled when Host settings select areas**. The cache
selector accepts a trusted Host configuration of up to 32 named areas. Its bounded
JSON decoder rejects duplicate keys (including case aliases), unknown fields,
trailing documents, invalid UTF-8 and documents over 64 KiB. Parsing does not authorize edits. The public settings API is registered only on
the existing trusted management transport. Complete added-data transfer remains open.

An area specifies `name`, `path`, `compatibility`, optional `repository`, `scope`
and `group`. Without `repository`, `path` is an absolute path inside the Env.
With a repository name, it is a strict relative path within that managed repository;
a collection resolves it to the named member. Absolute `/workspace` paths are refused
so a rule cannot evade repository matching. Paths are not shell-expanded. Provider
checks still own protected destinations, symlink/content inspection and native ownership.
An unrelated repository rule does not apply, and external Workspaces are not adopted
as managed repository data. Read-only Envs receive no selected areas.

The default `workspace` scope isolates generations by the exact Workspace identity.
Explicit `shared` scope requires a group. `compatibility` must explicitly identify
the relevant tool/platform/data format; it is not inferred from Base names or guest
output. Matching shared rules may reuse a source across compatible Bases and
Workspaces. Rule name, path expression, repository, scope/group and compatibility
are encoded unambiguously into the full source digest. A changed contract selects
a different source and retains old data. Generated catalog names are internal;
attachments preserve the configured name for presentation. The catalog verifies
the full digest even if shortened generated names collide.

The selector freezes its configuration, validates the whole placement set before
initializing sources, rejects overlaps and verifies exact managed Workspace ownership.
It uses the existing generation catalog and Env lifecycle API, without another
cache index, resource creator or cleanup path. An error may leave harmless empty
source entries but returns no partial selection and never creates provider data.
The native rootfs fixture now uses this Standard selector. That does not establish
stopped-writer publication, public configuration, data-bearing cross-Base reuse,
large-repository performance or new Windows acceptance.

## Configure and collect

Run on the trusted `haco-host` or Linux/WSL Physical Host. Save `cache.json`:

```json
{"areas":[{"name":"go-build","path":"/root/.cache/go-build","compatibility":"go-linux-amd64"}]}
```

Select a compatibility value matching your toolchain/platform. The default scope
is one Workspace. Use `scope: "shared"` and an explicit `group` only for data safe
to share across the intended Workspaces. `repository` selects a managed repository
and makes `path` relative to that repository. Paths do not expand shell variables.

```sh
haco cache configure cache.json
haco cache settings
haco env create --workspace managed:work development
# Build normally inside development, then stop it.
haco env stop development
haco cache status development
haco cache collect development
# Or collect one named area:
haco cache collect development go-build
```

New compatible Environments receive independent CoW copies. The producer keeps its
local data. Another producer based on an older generation is skipped; files are
never automatically merged. `--json` before the target preserves named result
states for scripts. Plain output uses area names, paths and generation numbers.

Settings are persisted by the trusted controller under its private state directory,
using revision comparison, an exclusive file lock and an atomic, synced replacement.
They are re-read for new Environments; no restart is needed. Existing Environments
keep their original placements and data. An empty `areas` array disables enrollment
for new Environments without deleting existing data. Never select credentials or
irreplaceable files as cache contents.

If collection reports `recovery-required`, source and candidate stay owned and the
producer cannot resume or be deleted through Hacocoon. Keep it stopped. Use `haco cache recover <env> <area>` for copies with durable completion; retry
cannot convert an unknown native copy into success. See [the lifecycle decision](../adr/0090-stopped-cache-collection.md).

## Inspect and clear collected data

`haco cache history <env> <area>` lists retained collection attempts by configured
name, path, creation time, original generation and state. `retained` means not
currently selected; it does not assert that an interrupted candidate was ever
published. An existing enrolled Environment identifies the source, including old
epochs. The --all operations below also cover sources after removing all
identifying Environments.

`haco cache clear [--yes] [--json] <env> <area>` displays that exact scope and uses
the common client deletion confirmation. A revision binds the Environment, source
and full candidate ownership. A changed review is refused. The canonical generation
CAS resets the source epoch before common exact-owner deletion removes only the
reviewed source candidates. It never deletes Env children, Workspace or OCI data.
New candidates outside the review are not swept. Current selections, snapshots and
in-flight copies retain their existing catalog/provider fences. Shared sources
affect new Environments throughout the configured group.

Existing Env copies remain usable. New Environments start empty until a producer
created after reset collects again. Creating/uncertain candidates stay owned and
report recovery-required; busy or failed cleanup reports cleanup-required. The
result preserves a successful reset separately from partial cleanup. Inspect
history and repeat clear after in-flight copies finish; unknown copy completion
still needs recovery. This is logical source cleanup, not physical disk reclamation.

## Recover completed collection

`haco cache recover [--json] <env> <area>` resolves exact candidates from the
enrolled name. It resumes only copies with a durable positive completion receipt,
rechecks the exact target owner and provider, then uses the common create commit
and generation CAS. It never reissues the copy request or uses destination
existence as completion evidence. Successful recovery releases the original
in-flight source pin; ordinary lifecycle checks still control Env resume/delete.

If the source is still the producer's original generation, recovery selects the
ready candidate. An already selected candidate is idempotent. If another producer
or reset advanced the source, complete data remains unselected; recovery does not
delete it or replay an old publication. Use reviewed clear for retained data.
Deletion-incomplete candidates report cleanup-required. Unknown copies and failed
provider checks remain recovery-required and owned, with partial results preserved.
Unknown native-copy cancellation remains unsupported; --all recovery includes
positively completed orphan-source copies.

Named-data snapshot/copy support is implemented through the canonical saved aggregate;
saved bytes do not grant publication into a newer common generation. Portable
added-data transfer is implemented; supported-host acceptance remains separate. See [snapshot semantics](environment-snapshots.md#named-disposable-data).


## Retained sources after Environment deletion

`haco cache history --all` lists every retained cache generation group, including
sources with no remaining producer. Existing Env names and configured area names
are shown when they can be observed; a missing producer is explicitly unnamed,
not inferred from an opaque ID. Ordinary output needs no native resource identity.

`haco cache recover --all` resumes only positively completed copies through the
same recovery transition. `haco cache clear --all` displays the complete scope
and uses the common confirmation; `--yes` still requires successful display.
A revision binds all groups, selections, exact owners and displayed Env names.
Changed review is refused before mutation. Each group then uses the existing
source CAS, exact-owner cleanup and positive-absence fences. Env data, Workspace,
OCI and protected snapshots remain. Groups report complete, failed or not_started;
a partial batch requires fresh inspection. Logical cleanup does not imply physical
space recovery. Unknown copies are retained, never canceled by destination guess.

## Empty an Environment's cache

Implemented candidate: `haco cache empty --preview <env> [<area>]` displays enrolled
cache directories, their maintenance state and retained snapshot count. Stop the
Env, then use `haco cache empty <env> [<area>]` to confirm and empty those contents.
`--all` selects all enrolled Envs; `--yes` skips the prompt only after successful
scope display. `--json` preserves per-area results. Options precede names.
Names and configured paths are sufficient; native volume identities are not inputs.

Unlike source `clear`, `empty` preserves common generations and removes only the
selected Env's cache contents. The Env, rootfs, Workspace, OCI data, independent
copies and saved snapshots remain. A later new Env may still reuse a common source;
use source clear separately when intended. This is logical emptying, not a claim
that snapshot/shared storage or the Windows disk has shrunk.

Standard binds a revision to the complete displayed selection and passes each exact
attachment to the common Env lifecycle. Core holds Env/Workspace locks, verifies the
owned stopped runtime and durably changes the child to `clearing` before any provider
mutation. Normal start/access/snapshot/copy/delete remain fenced. The provider
independently verifies the owned volume and sole stopped consumer, empties it,
verifies empty contents and ownership again, then Core commits `ready`. Neither
resource nor parent lease is released or replaced.

An interrupted/failed area stays `clearing` and blocks that Env's use. Keep it stopped,
inspect a fresh preview and explicitly repeat emptying. This retry only removes
remaining contents from the same owner; it cannot adopt a replacement Env/volume.
Other Envs may continue. Batches retain successful, failed and not_started results;
another area of the same interrupted Env remains blocked until recovery.

Linux Incus requires the volume file API (`file_storage_volume`). It enumerates
directory entries before deleting children in postorder, never follows symbolic
links or deletes the volume root, and never runs a guest executable or guesses Host
mount paths. Traversal is bounded to 100,000 entries, depth 64, 4096-byte paths and
4 MiB directory responses under a five-minute operation deadline. Unsupported or
malformed observations fail closed. These are bounded initial operations, not
large-repository performance acceptance. See [ADR 0101](../adr/0101-environment-cache-emptying.md).
