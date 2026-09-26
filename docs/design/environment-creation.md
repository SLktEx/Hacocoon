# Environment creation and immutable configuration

[日本語](environment-creation.ja.md) | English

`haco open` continues the last opened Environment, starting it if stopped. With
no Environments, it creates one from the configured default Image. An empty
Repository registry produces an empty Workspace. `haco open --new [IMAGE]`
always creates a new Environment; `haco create IMAGE` creates it stopped and
never launches a desktop client. Names are generated unless `--name` is supplied.

## Source selection

The creation service selects exactly one source:

1. `--snapshot SNAPSHOT`: use the saved Image, independent Workspace copies,
   OCI data and saved configuration. Current repositories/default Image are ignored.
2. Positional `IMAGE`: use that Image without changing the default setting.
3. Neither: use the configured default Image.

A Snapshot cannot be combined with an explicit Image or Volume. `create` requires
an explicit Image. Both commands use the canonical Environment lifecycle. Only
`open` follows creation with start and client launch. `--client none` supports
headless use; with it, `--json` returns Environment metadata.

`haco image default` displays the reference; `haco image default IMAGE` validates
and saves an existing Image. Initial Host setup downloads `haco/ubuntu-26.04` and
sets it only when no preference exists. The protected Environment JSON catalog
stores `default_image` and `last_opened`. These are references, not new resource
types. Existing Environment revisions never change when the preference changes.


Catalog version 17 adds these references and ownership fields. Version 16 upgrades
without changing existing Environment lifetimes. Existing installations with no
default can run setup or `haco image default IMAGE`; old binaries reject the new
catalog rather than discarding its fields.

## Workspace ownership and Volumes

Automatic Workspaces contain independent copies of the registered repositories,
starting at each remote default branch. Registration changes affect future
creation only. The Environment records its immutable Image, Workspace identity,
Workspace ownership and optional Volume name. Git owns subsequent branch state.

`haco volume create NAME` creates empty independent data. `--container ENV` copies
that Environment's current Workspace. Select it with `open --new --volume NAME`
or `create IMAGE --volume NAME`. No extra ordinary Workspace is generated. The
fixed mount is `/workspace` (collection members retain their relative layout).
An exclusive Workspace lease prevents simultaneous use, including by stopped
Environments. There are no post-creation attach/detach operations.

`haco env delete ENV` refuses a running Environment; `-f` stops it first. Automatic
Workspace data is deleted with its owner. Explicit Volumes survive; `haco volume rm`
deletes unused data. Legacy explicitly managed Workspaces retain their old lifetime.

## Failure boundaries

A failed new creation removes newly owned data only after canonical cleanup proves
the provider instance absent. Unknown ownership or cleanup retains the reservation
and reports recovery required. A completed Environment survives start/editor
failure; its last-opened reference permits retry with `haco open`. Existing broken
Environments fail without automatic replacement or Workspace rebinding.

Ownership receipts and resource relations remain durable. No new per-step creation
progress state machine is introduced. See [lifecycle ownership](../adr/0002-environment-lifecycle-ownership.md)
and [decision](../adr/0112-unified-environment-creation.md).

## Names, execution and retries

`haco snapshot create --name NAME ENV` selects a Snapshot name. Existing Snapshot,
Image or Volume names cause an error. A shared name lock excludes concurrent
Volume creation; Incus atomically reserves Image aliases during publication.

`exec` and `env exec` run only in running Environments and return the command exit
code. `commit ENV IMAGE` publishes rootfs without Workspace/Volume data and leaves
the source state unchanged. `image tag SOURCE TARGET` adds a name for the same
Image without changing any existing Environment revision.

Failed automatic Workspace deletion retains the ownership relation to the deleted
Environment in `owned_workspace_cleanup`. Repeating `rm` retries cleanup; names
and data cannot be reused before completion. Repository registration fetches the
remote default branch on each retry. Broken Haco-owned source Git metadata is
retained in a recovery directory before reacquisition; network/authentication
failures remain errors. Collections use the existing provider inventory bound of
253 members instead of the previous eight-member ceiling.

The retired in-place preparation API cannot create new staging records. Legacy
staging metadata and exact-owner cleanup remain solely for catalog migration.

Snapshot data can open without its old Repository registration. If that source is
absent, Git broker connection remains unavailable; opening never registers a
replacement source or grants credentials from a different repository.
