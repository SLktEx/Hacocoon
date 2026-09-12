# Open and branch a Workspace

Status: implemented Standard workflow on the development branch; scoped native
acceptance is recorded below.

A Workspace is one retained working set: the existing managed repository or
immutable repository collection, including each independent Git index and dirty
files, plus its associated OCI Store. An Environment is its disposable runtime.
This workflow composes the existing catalogs and canonical lifecycle API; it
does not introduce a second ownership catalog.

## Path entry

A local directory can hold a small Workspace reference. Preparing/opening that
reference copies explicitly selected Host-managed repositories into the existing
Incus Btrfs Workspace volumes. The directory's existing files are not moved,
replaced, mounted into the Environment or used as an implicit source. The new local files are the explicit Workspace reference
`.haco-workspace.json` and its owner-only coordination lock
`.haco-workspace.lock`.

`haco open --repo first,second --client none .` prepares that reference and opens
the isolated work. Later `haco open .` reuses its exact Workspace ownership
identity and the existing Environment, including its changes. A directory
without a reference requires explicit repository selection; guessing a Git remote
or copying ambient Host credentials is not part of path discovery.

The reference pins the Workspace owner and retains Base/OCI preferences. The
chosen directory must be owned by the client user and not group/world writable.
Reference/lock symlinks, hardlinks, non-regular files and broad permissions are
refused; a pinned directory descriptor prevents a rename from redirecting writes. It
contains no authority token or authentication data. Successful opens also pin
the selected OCI Store owner; a recycled Store name is refused. An explicit
`--oci auto` or `--oci oci:ID` chooses current Store selection again. A stale reference never
adopts a different Workspace with a recycled name. An existing Env with different
requested Base/OCI settings is refused; delete only the Env explicitly before
reopening with a different Base. The reference never grants simultaneous RW
access. Canonical leases remain authoritative across concurrent opens and crashes.

## Independent work

Fork captures a stopped Environment belonging to the selected Workspace under
the canonical lifecycle and Workspace locks. It uses the existing native
snapshot-to-Workspace and snapshot-to-Store copy operations. The new work receives
fresh volume owners and independent Git metadata; an Env root filesystem is not
restored into the new work. Open the new work with its chosen Base when needed.

The fork's OCI initial state is the source's stopped, guest-modified Store, when
present. A fork without OCI remains a no-OCI choice. New work without a retained
Store can instead initialize from the trusted Host publication on normal open.
Existing retained Stores win over newer Host content; ambiguous Store selection
requires an explicit choice. No guest Store is published back to trusted Host.

The source must have one stopped Env for aggregate capture; the command refuses
running, ambiguous or incomplete ownership. Stop it through the ordinary CLI
before forking. Detached retained work can be reopened and stopped first.
The destination remains creating until repo and optional OCI restoration finish.
Capture/delete/copy failures retain the existing exact ownership and source
reservations where absence/publication is not confirmed. CLI results expose
retained objects for recovery instead of hiding them with a fresh retry.

A fork inherits neither live connections nor one-shot approvals. Environment
creation identities are new. Administrator Policy retains its declared global,
name or creation scope; name/creation rules for the source do not silently become
rules for the fork.

## Retention and validation

`haco env delete` removes only the Env runtime. Managed repository changes and
OCI persist. `haco workspace delete` explicitly destroys the work's repository
data and retains independent Stores/snapshots. OCI deletion remains the existing
separate plugin operation. Listing work projects repository, Env, OCI and
snapshot references from the current catalogs, including incomplete states.

In a collection, repository mounts under `/workspace/<repository>` are retained.
Unrelated files in the Env root filesystem, including paths outside those mounts,
are disposable. Put additional retained working files inside a repository mount
or the explicitly attached Store.

Acceptance must distinguish component fixtures, actual Incus/Btrfs, installed
CLI and desktop clients. Record preparation, fork and reopen duration and
incremental allocation. Small fixtures do not establish Linux-kernel repository
performance. See [Workspace/lease](workspace-abstraction-and-lease.md),
[managed repository workflow](../guides/git-workflow.md),
[persistent OCI](persistent-oci-store.md), and
[the workflow decision](../adr/0062-workspace-entry-and-data-fork.md).


## Commands

Run the Linux/WSL client against its existing controller. Host sources are prepared
with the existing repository command; private repositories use the existing
trusted Host authentication setup.

~~~bash
haco repo clone --branch main first https://github.com/OWNER/FIRST.git
haco repo clone --branch main second https://github.com/OWNER/SECOND.git
mkdir task task-fork
haco open --repo first,second --name task --client none ./task
haco open --client ssh ./task
# Or use the existing VS Code client:
haco open ./task

haco env stop work-task
haco workspace fork --path ./task-fork --name task-fork ./task
haco open --client none ./task-fork

haco env delete work-task
haco open --base haco/ubuntu-24.04 --client none ./task
haco workspace list --json
haco plugin oci store list --json
~~~

Preparation without an Env is
`haco workspace prepare --repo first,second --name task --path ./task`.
A new reference defaults to automatic OCI selection; use `--oci none` or
`--oci oci:existing` explicitly. Reopen retains the reference's choice.
An existing Env is reused only when requested Base/OCI options agree; deleting
an Env is an explicit operation. Existing Environment-name desktop/preview
commands retain their behavior.

A failed preparation preserves its name for idempotent retry; an incomplete
provider copy fails closed. A failed fork reference and its result expose the
Workspace and temporary snapshot IDs. Inspect them with existing list/status
commands; a recovery-required reference does not trigger another fork or open.

## Dedicated native acceptance

See [commit-bound acceptance, measurements and Windows limitations](../status/acceptance-evidence.md#development-branch-integration).
