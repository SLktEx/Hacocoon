# Persistent OCI Store

Status: implemented for containerd/nerdctl; packaged acceptance is recorded in
[implementation status](../IMPLEMENTATION_STATUS.md). Docker Store compatibility
is deferred. Container tooling remains optional and is not installed by Core.

An Environment has a disposable root filesystem, a persistent Workspace and an
optional persistent OCI Store. The controller owns the Store catalog and its
exclusive attachment reservation. Incus owns its Btrfs custom volume. See
[ADR 0014](../adr/0014-persistent-managed-resources.md) and the
[Workspace lifecycle](workspace-abstraction-and-lease.md).

```bash
haco plugin oci store create dev
haco plugin oci store list
haco plugin oci store inspect dev
haco env create --workspace managed:work --resource oci:dev first
# Install the optional runtime, then pull/build inside first.
haco env delete first
haco plugin oci store inspect dev
haco env create --workspace managed:work --resource oci:dev second
# Install the same runtime version; previously stored images/cache are available.
haco env delete second
haco plugin oci store delete dev
```

Create records an exact provider identity and random ownership token before any
provider mutation. Only a verified volume becomes ready. Environment creation
reserves the Workspace and Store atomically; concurrent RW attachment and Store
deletion while reserved fail closed. Stop retains the reservation. Environment
delete removes only the runtime and releases its reservations after confirmed
absence. Explicit Store delete excludes new attachment, verifies ownership,
deletes the volume, confirms absence and then removes its catalog entry.
Uncertain create/delete results retain ownership for inspection and explicit
retry. No background collection deletes unused Stores.

The instance receives only this volume at `/var/lib/hacocoon-oci`. The optional
Incus attachment enables unprivileged nesting without changing its network or
anti-spoofing rules. Its containerd configuration uses:

| Data | Location | Lifetime |
|---|---|---|
| Images, content/layers, snapshots and metadata | `/var/lib/hacocoon-oci/containerd` | Store |
| Local BuildKit build cache | `/var/lib/hacocoon-oci/buildkit` | Store |
| containerd process state and socket | `/run/containerd` | Environment |
| BuildKit process/socket | `/run/buildkit` | Environment |
| Runtime binaries, service process and rootfs | Environment rootfs | Environment |

This follows [containerd's root/state separation](https://github.com/containerd/containerd/blob/main/docs/ops.md).
Stored container metadata may remain, but tasks and running containers are not
migrated or automatically resumed. Select compatible runtime versions when
reattaching. Runtime version upgrades, broad image compatibility and cache
compaction are outside this PoC.

Install containerd, runc, nerdctl and (for builds) BuildKit in the Environment
using its permitted package/download proxy or a prepared Base. No custom Base
builder is required. The attachment writes `/etc/containerd/config.toml` and an
Environment-local `buildkit.service` using the native snapshotter. It waits a bounded
period for the guest systemd manager before configuring services; a readiness
failure fails creation through the normal ownership-preserving cleanup. Start the
optional services when installed. Daemon-side registry operations also need the
Environment's existing credential-free Standard proxy. Configure this inside the
Environment (and retain exact registry/download Policy on the Physical Host):

```bash
for service in containerd buildkit; do
  mkdir -p /etc/systemd/system/$service.service.d
  printf '[Service]\nEnvironment="HTTP_PROXY=%s" "HTTPS_PROXY=%s" "NO_PROXY=%s"\n' \
    "$HTTP_PROXY" "$HTTPS_PROXY" "$NO_PROXY" \
    > /etc/systemd/system/$service.service.d/proxy.conf
done
systemctl daemon-reload
systemctl restart containerd buildkit
nerdctl --snapshotter native pull --unpack=false docker.io/library/busybox:latest
nerdctl --snapshotter native build --network none -t example:local .
nerdctl --snapshotter native run --rm --network none example:local
```

The validated containerd 2.2 / nerdctl 2.3 combination uses `pull --unpack=false`:
content is stored first and native snapshots are prepared by `run`. Its transfer
service default unpack configuration does not select the native snapshotter.
Use `--snapshotter native` consistently for run, build and image inspection.

Pulled and built images in the same containerd namespace remain after a later
reattachment. A different Store starts with separate image/cache data. No Host
runtime process, credential, Docker/containerd management socket, writable Host
runtime directory, `/run`, or Windows authority is shared. Registry credentials
are not provisioned into a Store by Hacocoon. Treat Store contents as untrusted
Environment data and do not attach them to the trusted Host.

## Default Environment creation flow

Status: planned, required by the current user workflow. The explicit Store
commands above are implemented advanced/recovery operations, not the intended
ordinary create sequence. Environment creation must automatically publish the
locally prepared Docker/nerdctl images from trusted Host, make an independent
Btrfs COW copy and attach the resulting persistent data. Users must not normally
create, copy and name a Store separately. A single optional `--no-oci` opt-out
is the intended product surface; it is not implemented yet.

The maintained optional OCI integration supplies this default. Core keeps a
provider-neutral initialization contract and does not require either runtime.
If the integration/runtime is absent, ordinary non-OCI Environment creation
must remain usable. If a configured publication/copy operation fails, report
that failure and retain exact ownership for recovery; do not silently omit the
requested content. Do not fetch new registry content as a side effect of copying
already prepared local images.

Publication must contain image data only, quiesced before COW. Do not copy a live
Host daemon directory, credentials, management sockets, process state or arbitrary
Host volumes. Preserve separate Docker/containerd formats when required; verify
both runtime paths independently. A guest-used Store is never reattached to Host.
Reuse a Workspace's retained Store on recreation without overwriting guest edits;
explicit resource selection remains available for advanced use. Creation failure
must leave every automatic resource identifiable and recoverable. This default
workflow is incomplete until Host publication, automatic copy/attachment, opt-out,
repeat/recreate behavior and both runtime acceptance have actually run.

## Independent offline copies

Status: implemented at the repository and real-Incus storage boundary. End-to-end
OCI image distribution from trusted Host remains **partial**. See
[ADR 0015](../adr/0015-offline-persistent-resource-copy.md).

Reuse the existing create operation when a new Environment needs an independent
copy of a prepared Store:

```bash
# Gracefully stop workloads and delete the old Environment to release its Store.
# Environment deletion retains its Workspace and Store.
haco env delete first
haco plugin oci store create dev --from shared
haco env create --workspace managed:work --resource oci:dev second
```

Here `shared` is an existing Store previously prepared in an Environment, not a
Host daemon directory. `--from shared` may precede or follow `dev`. Omitting it
still creates an empty Store. Copying requires an unleased source: stopping an
Environment alone retains its reservation. Runtime/tool installation is still
required in the new Environment. Copies preserve Store data; they do not start
containers or copy the old Environment rootfs, `/run`, sockets or credentials
from trusted Host. Hacocoon does not provision registry credentials into Stores;
any credentials a user manually placed in source data would also be copied.

The Incus adapter requires the same Btrfs pool, independently validates ownership
and `used_by`, supplies new ownership markers and preserves idmap metadata. Incus
performs the volume-only COW copy. Source and copy can later be used by separate
Environments, changed or deleted independently. Core never parses image contents
or accesses the Btrfs mount directly.

During copying, source attachment/deletion and target attachment/deletion are
blocked by the durable catalog reservation. On success the target is `ready` and
the source is released. A failed/timed-out copy stays `creating` with an exact
`copy_source`; `inspect`/`list` show these recovery details. Automatic recovery is
not yet implemented. Do not retry by editing state or deleting provider objects:
an asynchronous Incus operation may still be running even if its destination is
not yet visible. An explicit future recovery path must prove operation quiescence.

Repository regressions cover malformed/foreign/busy source observations, idmap
preservation, CLI/RPC validation, reservations, restart, duplicate operations and
failure retention. `TestRealIncusPersistentCopyE2E` uses a dedicated test-owned
pool/project and synthetic data to verify Btrfs parent UUID, independent writes,
source deletion and exact cleanup. It is included in the existing real-Incus GHA
workflow. This is storage acceptance, **not** image/runtime or installed CLI
acceptance. Reproduce on a root Linux/WSL host with Incus and Btrfs:

```bash
HACO_E2E_INCUS_PERSISTENT_COPY=1 go test -count=1 \
  -run '^TestRealIncusPersistentCopyE2E$' -v ./modules/runtime/incus
```

The former `haco plugin oci distribute` CLI, RPC, archive service and save/load
adapter remain removed. [ADR 0012](../adr/0012-one-way-oci-distribution.md) is
historical. Revised B4 requires both persistent Stores and independent COW image
delivery; Store reattachment alone does not complete that request. Trusted Host
acquisition/publication without Host credential/live-state sharing, complete
containerd/nerdctl and Docker image acceptance, and interrupted-copy recovery
remain follow-up work. Never attach guest-populated Stores to trusted Host.
