# OCI image inventory and deletion

The GHA native fixture now includes an opt-in shipped controller/product CLI gate. It registers only its freshly created synthetic Store in a private real catalog, then exercises detached inventory, referenced-image refusal, confirmed deletion and temporary lifecycle cleanup. This gate is restricted to disposable GitHub-hosted runners; implementation of the gate is not evidence of a successful run.

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
source images are addressed below. Detached Store routing is partial as described below; candidate-selected GC remains planned. No Seed/tombstone path, hidden backup, new catalog
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

Status: **partial**. The existing image commands accept a retained Store ID in
place of an Environment name; there are no additional commands or required flags:

```bash
haco plugin oci image list oci:store-id
haco plugin oci image delete oci:store-id sha256:<displayed-digest>
```

The review contains the exact Store ID/owner, without a stale scratch Environment
identity. Each list or delete acquires one canonical maintenance run and uses its
current Environment generation for all runtime calls. The run owns cancellation,
exclusive Store reservation and cleanup; it never rebinds the original Workspace
or deletes the borrowed Store. Ambiguous cleanup preserves ownership evidence.
Mixed Host/Environment/Store targets and stale reviews are refused. Docker on a
detached Store is explicitly unsupported.

Receipt-based SandboxProvider creation starts without retained data, preserves
current network guards, masks ordinary daemons, attaches the Store, then starts a
private containerd 2.3.3 metadata service. Task/restart/CRI/NRI and sandbox services
are disabled. No retained configuration, restart labels or authority is adopted.
Receipt-free creation and snapshot restore refuse maintenance. See
[ADR 0047](../adr/0047-detached-store-maintenance.md).

Linux/WSL amd64 composition now supplies compatible tools automatically before
retained attachment. The OCI module downloads the pinned nerdctl 2.3.5 distribution,
verifies its SHA-256, and reuses a private archive cache under the configured Haco
root. It prepares only containerd, ctr and nerdctl in a private temporary directory.
The Incus adapter rechecks ownership, generation and absence of retained mounts,
transfers and hashes these files, then installs them in the disposable rootfs.
Downloaded binaries never execute on the Physical Host. A failed preparation does
not attach the retained Store. No user preparation command is required.

Cache access is serialized; symlinks, hardlinks, unsafe permissions and corrupt
entries are refused without silently replacing them. Archive paths never select
Host output paths. Signed download URLs and response bodies are not included in
transport errors. Non-Linux and non-amd64 tool provisioning are currently unsupported.
Native tool delivery is accepted; complete installed-controller acceptance remains pending.
There is no schema migration, automatic backup or arbitrary executable/socket option.

## Detached containerd metadata service

The independent Incus 6.0.5/Btrfs primitive passed in 179.66s, including task API
refusal, unchanged restart-marked container metadata, retained used image, unused
alias removal, masked restart and exact owned cleanup. Product image operations on
that socket are a separate acceptance test. Its fixture must distinguish displayed
tags from immutable digests and actual container references: a shared tag is not
proof that every corresponding digest is referenced. Whole-controller creation and
tool provisioning are not represented by its fixture catalog/lifecycle adapter.

The expanded native fixture passed in 224.64s: product inventory, actual-container
reference refusal, selected unused digest removal and confirmed absence, retained
container metadata, masked restart, Store survival after Env deletion and exact
owned cleanup. Its lifecycle/catalog adapter remains a fixture.

Automatic tool delivery through the production preparer and Incus adapter passed
in the dedicated native fixture (237.37s), including subsequent image operations,
retained metadata/Store protection and exact cleanup. A separate empty-cache real
HTTPS acquisition and extraction test passed in 70.71s without executing downloaded
binaries on the Host. Full OCI/Incus/composition race suites and vet passed.
