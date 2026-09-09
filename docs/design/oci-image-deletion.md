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
source images are addressed below. Detached Stores, candidate-selected GC and
automatic startup of a maintenance Env remain planned. No Seed/tombstone path, hidden backup, new catalog
state or schema migration is introduced. Existing saved snapshots are independent
copies, so image deletion does not modify them. Unit and CLI regressions and the
existing optional real-runtime COW fixture cover different scopes; exact executed
results are recorded in implementation status and the PR.

## Managed Host source

Partial implementation; dedicated native runtime acceptance passed, installed-controller acceptance pending:

```bash
haco plugin oci image list --host
haco plugin oci image delete --host --runtime docker example.local/app:dev
```

`--host` and an Env name are mutually exclusive. The preview identifies the exact
managed source owner and explains that removal changes the source of future Store
copies. Existing independent Store/snapshot copies are unaffected. This does not
resurrect the historical Seed namespace, tombstones or all-Environment deletion.
The current managed source is `oci-source:host`; callers cannot substitute a guest
Store as a Host attachment. No automatic setup, migration or recovery runs here.

The plugin requires the current ready source owner. The Incus adapter independently
accepts only fixed image/container inventory, limited inspect templates and
immutable-ID removal without force. It supplies fixed executable search paths and
local daemon sockets with a cleared CLI environment. No caller-controlled shell,
program, daemon option or arbitrary inspect template crosses this Host boundary.

Each command holds the existing Host-operation lock, rejects pending copy journals,
and verifies the native volume owner, sole consumer, exact mount, local Host/source
markers, unprivileged container type, empty profiles, running state and managed
daemon layout. It neither resumes a paused Host nor clears recovery evidence.
Unknown ownership/layout or failed/truncated observations refuse the operation.
The native command is bounded to two minutes; the full request remains bounded to
five minutes. Container references and post-removal absence use the same runtime
checks as attached Stores. Existing copy/cleanup state remains unchanged.

## Historical Seed deletion

The old v0.16 Host Seed-cache/tombstone and all-Environment operation belongs to
the quarantined legacy implementation. It is not the current product command or
a current Store retention model. Historical behavior is recoverable from Git;
existing records are not silently deleted by this change. The release table's
v0.16 link identifies that historical checkpoint, not this new partial slice.
See [ADR 0046](../adr/0046-reviewed-runtime-image-deletion.md).

## Detached Store implementation in progress

Not exposed through the public CLI yet. A scratch-run reservation must match the
exact Store owner and a durable run identity; normal Workspace associations,
exclusive Store leases and source-only refusal remain required. Catalog read-time
validation preserves admitted leases through active execution and cleanup while
requiring the exact scratch identity. Run evidence cannot be deleted or replaced
until the associated lease is released after confirmed native absence. The independent Incus preparation primitive
passed a dedicated systemd/Btrfs fixture; actual detached Docker/nerdctl image
operations remain unverified. See [ADR 0047](../adr/0047-detached-store-maintenance.md).

Receipt-based SandboxProvider creation now prepares before attachment and starts
metadata services after current network validation. Receipt-free creation and
snapshot restore refuse maintenance. The existing run service pins the reviewed
owner and holds its marker/lock across the whole operation and canonical cleanup.
Public image routing is still not connected. Focused run and adapter race tests
passed; full native acceptance of the composed creation path remains pending.

## Detached containerd metadata service

An internal startup primitive now checks the exact native Store owner, single
consumer, current Env generation and unprivileged mount before starting a
containerd 2.3.3 metadata service. Its private guest socket/configuration/state
are independent of retained configuration and ordinary daemon startup. Task,
restart, CRI, NRI and sandbox controllers are disabled; persisted container
records and restart labels are preserved. Its dedicated Incus 6.0.5/Btrfs test
passed in 179.66s, checking task API refusal, unchanged restart-marked metadata,
used-image retention, unused-alias deletion and retained Store cleanup. The test
is also wired into existing Incus/Btrfs GHA; this is primitive acceptance. This
primitive does not enable public maintenance creation, Docker support or a
caller-selected socket. See [ADR 0047](../adr/0047-detached-store-maintenance.md#containerd-metadata-only-startup).
