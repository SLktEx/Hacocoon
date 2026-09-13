# Data lifetime and cleanup

[日本語](data-lifetime.ja.md) | English

Run lifecycle commands from the trusted management terminal. Ordinary development
belongs inside an Environment. The Physical Host owns Incus and protected Hacocoon
state; trusted `haco-host` owns management tools and authenticated Git operations.
Neither Host is an untrusted development Environment.

## Objects and lifetime

```text
Physical Host: controller, Policy, Incus, storage authority
  ├─ trusted haco-host: credentials and registered source repositories
  ├─ Base image ──create──> Environment root filesystem
  ├─ managed Workspace ──exclusive lease──> /workspace
  └─ optional OCI Store ──exclusive lease──> /var/lib/hacocoon-oci
```

| Operation | Environment rootfs | Workspace and Git | OCI Store | Lease/access |
|---|---|---|---|---|
| Exit SSH/editor | Retained, runtime may keep running | Retained | Retained | Retained |
| `haco env stop dev` | Retained; processes stop | Retained | Retained | Lease retained |
| `haco env start dev` / `haco open dev` | Same runtime resumes | Same data | Same data | Ownership/network rechecked |
| `haco env delete dev` | Removed | Retained | Retained | Released only after positive runtime absence |
| `haco workspace delete work` | Refused while referenced by an Env | All managed files and Git metadata removed | Retained | Active/intermediate leases block deletion |
| `haco plugin oci store delete store` | Refused while Store is reserved | Retained | Entire selected Store removed | Stopped Envs also block deletion |

A Base is an immutable starting revision, not a backup of a modified Environment.
Changing a Base alias affects later creation only. OCI Stores retain images, build
cache and persistent runtime metadata; binaries and process/socket state belong to
the Environment. Reattachment needs compatible tooling and does not resume tasks.

For a single repository, `/workspace` is the persistent mount. For a collection,
only `/workspace/<member>` mounts persist; files beside them are Environment-only.
External Workspaces remain at their explicit **controller-side** paths.
Guest `/tmp` may be cleared at boot. None of these mechanisms protects writable
Workspace files from an agent's edits.

## Recreate while keeping project files

First save needed Environment-only files into the Workspace or create a snapshot.
To deliberately discard packages/rootfs changes:

```bash
haco env stop dev
haco env delete dev
haco env create --workspace managed:work --base haco/ubuntu-26.04 dev
haco git connect dev
haco open dev
```

Use the original Workspace ID. Its associated default Store is reused; if you
previously selected an independent Store explicitly, pass the same
`--resource oci:<store>`. Repeat `--no-oci` if you intend no OCI attachment.
The new Env gets a new creation/SSH identity and does not inherit old
Environment-specific approvals. `switch-base` is disabled.
See [Workspace ownership](../design/workspace-abstraction-and-lease.md).

## Save or copy the whole development state

```bash
haco snapshot create dev
haco snapshot list
haco snapshot restore <snapshot-id> restored-dev
haco env stop dev
haco env copy dev independent-dev
```

Snapshots include independent rootfs, all managed Workspace members, optional OCI
and metadata. Capture stops a running source and restarts it only after a ready save.
Copy requires a stopped source. Restore/copy create new resources and security
identities; they do not overwrite an existing Environment. A default restore name
is `<source>-restored`; the copy default is documented in the [copy contract](../design/environment-copy.md).
Stop application writers first when consistency matters. Byte retention is not
proof of arbitrary database consistency.

Saved snapshots survive source deletion. Explicit `haco snapshot delete <id>`
deletes the selected save, retaining current independent work. Failures retain
identities for inspection. See [snapshot semantics](../design/environment-snapshots.md).

## Delete retained data explicitly

Inspect before deleting:

```bash
haco workspace list
haco plugin oci store list
haco repo list
haco base list --all
```

Each delete previews and confirms its exact managed target; `--yes` is for
intentional automation. The selected target and consequences are shown in the
chosen CLI language. Failed warning/prompt output stops before deletion; a failed
completion display does not undo deletion, so inspect the inventory before a
retry. Delete a source repository only after no Workspace record
needs its Git route. Never delete valuable work just to clear a dependency.
Remote repositories, credentials and independent saves are not deleted by
`haco repo delete <id>`.

Native Incus child snapshots, backups and schedules block parent volume deletion.
Unknown ownership, partial creation or unfinished copy also blocks unsafe removal.
A retained `deleting` record can be retried through the same explicit command;
incomplete creation has no general repair command.
[Base cleanup](../design/base-images-and-custom-environments.md#explicit-built-image-cleanup)
and [individual OCI images](../design/oci-image-deletion.md) have their own reference checks.

Deleting data and recovering Windows disk allocation are different operations.
[Storage reclamation](../design/storage-reclamation.md) documents `haco reclaim`,
status/review, interruption handling and its tested scope.
[Environment transfer](../design/environment-transfer.md) moves a completed bundle;
[evacuation](../guides/data-evacuation.md) is partial maintenance work, not whole-installation backup.
