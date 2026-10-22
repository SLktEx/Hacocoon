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
| `haco env delete dev` | Removed (running requires `-f`) | Automatic Workspace removed; explicit Volume retained | Automatic owned data removed; Volume data retained | Released only after positive runtime absence |
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

## Keep data independently of an Environment

```bash
haco volume create saved-work --container dev
haco env stop dev
haco env delete dev
haco open --new --volume saved-work
```

The Volume is an independent copy. It survives deletion of either Environment.
Automatic Workspace data is deleted with its Environment, including uncommitted,
unpushed and untracked work. Existing Volume bindings cannot be changed. Legacy
explicit managed Workspaces retain their existing independent lifetime.
See [creation and ownership](../design/environment-creation.md).

## Save or copy the whole development state

```bash
haco snapshot create dev
haco snapshot list
haco open --new --snapshot <snapshot-id> --name restored-dev
haco env stop dev
haco env copy dev independent-dev
```

Select a ready Snapshot ID from `haco snapshot list`. New creation uses its Image,
Workspace and OCI metadata, ignoring current defaults and Repository registrations.
Capture leaves the source running/stopped state unchanged. Copy still requires a
stopped source. Neither operation overwrites an existing Environment.

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
haco image list --all
```

Environment deletion does not prompt. Other advanced storage deletion commands retain their target review. Repository deletion unregisters future input while preserving existing Git routes. Never delete valuable work just to clear a dependency.
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

Deletion warnings and retained-data results follow the selected language.
Empty input, EOF or an answer other than yes cancels. Failed warning/prompt output
refuses deletion even with `--yes`; the controller rechecks references and ownership.
