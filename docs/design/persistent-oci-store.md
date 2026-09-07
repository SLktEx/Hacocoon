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

The former `haco plugin oci distribute` CLI, RPC, archive service and save/load
adapter have been removed. [ADR 0012](../adr/0012-one-way-oci-distribution.md) and
its commit-bound acceptance remain historical; image delivery is not current B4.
