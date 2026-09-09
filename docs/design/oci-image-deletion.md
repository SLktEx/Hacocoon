# OCI image inventory and deletion

[日本語](oci-image-deletion.ja.md) | English

## Current attached-Store commands

Partial implementation; dedicated native runtime acceptance passed, installed-controller acceptance pending: use the existing plugin namespace:

```bash
haco plugin oci image list dev
haco plugin oci image list --runtime docker --json dev
haco plugin oci image delete dev example.local/app:dev
```

`nerdctl` is the default; Docker is an explicit runtime selection. List shows the
Env generation, exact Store owner, image IDs/tags/digests, container users and
independent snapshot provenance. Delete accepts an exact displayed ID or tag,
resolves it to the runtime's immutable ID, previews it and asks for confirmation.
`--yes` skips the prompt, never identity or reference checks. This deletes the
whole selected image; it is not a tag-only untag operation. Runtime refusal for
multiple tags, containers or other references is returned without force.

The OCI plugin queries the runtime instead of maintaining another image catalog.
Docker uses config IDs; nerdctl uses manifest/index digests for selection/removal,
while its Docker-compatible inspect exposes config IDs and container image names.
The plugin resolves these native differences explicitly. It invokes fixed local
sockets with a cleared CLI environment and fixed containerd namespace/snapshotter.
It neither accesses layer files nor accepts a caller-selected socket/executable.
Unparseable, incomplete or failed runtime observations fail closed. A successful
remove must be followed by an inventory proving absence; otherwise report failure.

`internal/workspace.ExecForResource` holds the canonical Env lifecycle lock while
comparing both the reviewed generation and attached Store identity and invoking
normal execution. Each runtime call repeats that check. The plugin separately
checks current ready Store ownership. Recreating a name cannot redirect a pending
delete into its replacement. Ordinary guest work may change runtime inventory;
the runtime's non-force deletion remains the final reference check.

This slice handles a Store attached to an Env whose runtime is available. Host
source images, detached Stores, candidate-selected GC and automatic startup of a
maintenance Env remain planned. No Seed/tombstone path, hidden backup, new catalog
state or schema migration is introduced. Existing saved snapshots are independent
copies, so image deletion does not modify them. Unit and CLI regressions and the
existing optional real-runtime COW fixture cover different scopes; exact executed
results are recorded in implementation status and the PR.

## Historical Seed deletion

The old v0.16 Host Seed-cache/tombstone and all-Environment operation belongs to
the quarantined legacy implementation. It is not the current product command or
a current Store retention model. Historical behavior is recoverable from Git;
existing records are not silently deleted by this change. The release table's
v0.16 link identifies that historical checkpoint, not this new partial slice.
See [ADR 0046](../adr/0046-reviewed-runtime-image-deletion.md).
