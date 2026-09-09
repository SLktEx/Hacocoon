# Environment copy

Status: implemented; provider acceptance is recorded with the change.

## Use

```bash
haco env stop dev
haco env copy dev experiment
```

Omit the destination to use `<source>-copy` (source prefix at most 52 characters).
The source must already be stopped. An existing destination or incomplete
creation lease is refused. The new Environment starts automatically with fresh
identity and current security settings. The original stays stopped. Use ordinary
SSH setup/open for the new name. `--json` before the source returns public names.

Copy preserves independent rootfs, managed Workspace files and Git state, and
attached OCI Store data. It does not copy running processes, saved snapshot
history, old approvals, management devices or Hacocoon-managed connection identity. Ordinary
Env deletion preserves both its own persistent data and other independent copies.
Base is provenance; original Base/image material is unnecessary for the copy.
External-path Workspaces, other storage drivers, live databases and cross-host
migration are unsupported by this aggregate.

## Incus operations and ownership

Incus instance copy does not include independently managed custom volumes, so
Hacocoon must coordinate rootfs and Work/OCI copies. See [Incus instance backup
scope](https://linuxcontainers.org/incus/docs/main/howto/instances_backup/) and
[volume copy](https://linuxcontainers.org/incus/docs/main/howto/storage_move_volume/).

`internal/environmentcopy` composes the existing stopped aggregate capture and
`internal/snapshotrestore` service. Both stages use native Incus Btrfs COW, not
file export/import or a new storage engine. The intermediate aggregate costs an
extra native COW copy; this reuses the existing exact-owned receipts and source
write exclusion without inventing another multi-resource transaction. It is
visible through `haco snapshot list` while present and is deleted after the copy
attempt. It is not a backup of a restore destination: the destination must be new.
No user snapshot is selected for deletion, and no rollback copy is created.

The capture checks stopped state under canonical Environment and Workspace locks.
Once complete, the intermediate data is independent, allowing source locks to be
released before destination creation. Canonical creation repeats destination
exclusivity checks and assigns new generation, network guards and SSH identity.
The catalog, native provider and Workspace/Store interfaces retain their existing
ownership responsibilities. No schema, new recovery state or backend is added.

## Failure and cleanup

Failed creation uses the existing bounded exact-owned cleanup. Failed start keeps
the published destination and data for normal `haco env start`. Intermediate
cleanup runs with a bounded cancellation-independent context even after failure.
An uncertain cleanup remains a failure and returns `temporary_snapshot`; inspect
`haco snapshot list`, then explicitly delete that ID once its source reservations
allow it. A crashed client/controller may leave that same visible owned save;
there is no automatic crash-resume promise. Never infer that a failed command
means that no destination exists: its result includes remaining public names.

Existing catalogs and saved data are unchanged. There is no manual migration.
