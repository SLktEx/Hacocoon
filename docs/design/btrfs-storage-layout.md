# Btrfs storage layout

Status: **the normal local Incus path delegates loop-backed Btrfs lifecycle to Incus; the older Hacocoon-managed raw/helper implementation remains available for focused compatibility tests.**

Milestones: **v0.20 Managed Btrfs Rootfs Storage** and **v0.21 Managed Btrfs Transparent Compression**.

## Default local layout

The normal local composition does not create or mount a Hacocoon-owned raw image. Instead, it lazily asks Incus to create the default pool without a `source=` override:

```text
Incus pool: haco-local-default
  driver=btrfs
  size=128GiB
  btrfs.mount_options=compress=zstd:3,noatime,nodiscard
        |
        v
/var/lib/incus/disks/haco-local-default.img
  (Incus-owned sparse Linux file)
        |
        v
     loop device
        |
        v
  Btrfs filesystem
        |
        v
/var/lib/incus/storage-pools/haco-local-default
  |- cached Base image volumes
  |- Tooling Base builders
  |- Seed builders / cached Seed image volumes
  |- Environment rootfs volumes
  `- Incus snapshots / clones
```

Incus owns creation of the backing image, loop attachment, Btrfs formatting, mount/unmount lifecycle, and supported loop-pool growth. Hacocoon owns the desired pool identity and policy, but does not duplicate Incus' block-device lifecycle for the normal local path.

This ownership boundary is important on WSL. A Host-managed mount created after `incusd` starts can race Incus storage initialization and leave a pool temporarily unavailable. Letting Incus own the backing image and mount removes that cross-owner startup dependency rather than adding mount-namespace or service-order workarounds.

## Sparse file versus WSL sparse VHD

Incus' loop-backed Btrfs pool uses a sparse **Linux file**. Incus creates the loop image through its sparse-file path and sets its logical size without eagerly allocating every block. Repository acceptance verifies that the image's allocated bytes are smaller than its logical 128 GiB size after creation.

That is separate from WSL's `sparseVhd` / sparse-VHDX mode. Hacocoon does not enable WSL sparse-VHD mode as part of this storage design. Windows-host VHDX space reclamation is not exposed as a Hacocoon product CLI command; when needed it remains an explicit Windows/WSL operational concern.

## Why rootfs objects share one pool

The storage boundary is deliberate. Base, Tooling, Seed, and Environment rootfs data should stay on the same Hacocoon Btrfs pool so storage-level optimizations apply across the lifecycle:

- transparent Btrfs compression can reduce physical bytes for managed rootfs data;
- Incus Btrfs snapshots and clones can preserve copy-on-write sharing;
- Seed-derived Environments only need new extents for changed data where the storage driver can share unchanged blocks;
- filesystem-level maintenance and optional out-of-band deduplication can target Hacocoon rootfs data rather than unrelated Host data.

Hacocoon must not create a separate Btrfs filesystem or loop image per Environment or Seed merely for isolation. Incus volumes/subvolumes provide the logical isolation inside the shared pool.

## Managed mount policy

The desired mount policy is:

```text
compress=zstd:3,noatime,nodiscard
```

`compress=zstd:3` keeps transparent compression enabled without `compress-force`; normal Btrfs compression heuristics may leave incompressible data uncompressed instead of repeatedly spending CPU on forced attempts.

`noatime` avoids access-time metadata updates for read-heavy development rootfs workloads. This reduces metadata writes and avoidable COW churn when tools repeatedly read source trees, packages, compilers, runtimes, and database files. Applications or operator scripts that explicitly depend on `st_atime`, `find -atime`, or equivalent access-time semantics are a compatibility exception and require a different storage policy in the future.

`nodiscard` disables continuous discard, including Btrfs' async-discard default on supporting devices. Hacocoon prefers batch reclamation rather than mixing discard work into ordinary Environment activity. Windows/WSL trim and VHDX compaction are explicit Host/operator operations during the current CLI migration; no product CLI command owns that maintenance flow today. Other Host paths likewise require an explicit Host/operator trim policy until a generic Hacocoon storage-maintenance surface owns the operation.

The default policy also leaves `autodefrag` disabled. Automatic defragmentation can rewrite extents and reduce existing reflink/COW sharing, which is a poor default trade-off for an Incus snapshot/clone-heavy rootfs pool. Any future autodefrag use requires an explicit workload-specific decision rather than becoming an implicit mount default.

Compression mount options affect newly written extents. Hacocoon does not automatically rewrite all existing data merely to recompress it.

## Runtime selection rule

The local composition configures a lazy storage provider. Merely opening the local application for a command that does not need an Incus rootfs does not create the default pool.

Before the first Environment, Tooling Base builder, Seed builder, or trusted host needs root storage, the provider checks for `haco-local-default`. If it does not exist, Hacocoon asks Incus to create the Btrfs loop pool with the desired size and mount options. The runtime then reuses that pool for subsequent Hacocoon-owned rootfs operations instead of inheriting the Host's Incus default-profile pool.

An already-existing `haco-local-default` pool is reused, but its `btrfs.mount_options` value is reconciled to Hacocoon's desired policy before reuse. Hacocoon updates the Incus pool configuration rather than destructively replacing a populated pool; Incus remains the owner of the corresponding remount. Migration of legacy pool contents remains a separate fail-safe operation.

## Legacy Hacocoon-managed storage path

The repository still contains the earlier `modules/storage/btrfs` implementation and `haco-storage-helper`. That historical compatibility path is the **Hacocoon-managed Btrfs storage layout**: it creates a **sparse raw** backing file for a storage ID and maps it to an Incus pool named `haco-<storage-id>`.

It manages:

```text
HACO_ROOT/images/<storage-id>.raw
  -> loop device
  -> Hacocoon-managed Btrfs mount
  -> Incus pool source=<mountpoint>
```

It remains useful for focused storage-helper, block-backend, shrink/compact, hardening, and compatibility tests. Explicit `HACO_STORAGE_PRIVILEGE_MODE` or `HACO_BLOCK_BACKEND` configuration selects that compatibility path in local composition. Normal installations set neither variable and therefore use the Incus-owned pool.

The legacy filesystem path enforces the same `compress=zstd:3,noatime,nodiscard` desired state. The helper remains fail-closed and typed: it accepts only the exact Hacocoon mount/remount policy and does not expose arbitrary root command execution. Its dedicated acceptance coverage continues independently of the normal local Incus path.

## `metadata_ratio` policy

Hacocoon does not set `metadata_ratio` by default. Snapshot/reflink-heavy workloads can become metadata-heavy, but reserving metadata more aggressively also changes allocation behavior and can waste space. Any non-default value must first show a repeatable benefit under a Hacocoon-specific snapshot/clone workload, including metadata ENOSPC behavior and physical-space overhead.

## Acceptance coverage

Repository CI uses independent disposable Ubuntu 26.04 acceptance paths:

1. the storage-helper job exercises the retained Hacocoon-managed raw/loop/Btrfs helper boundary, verifies the exact managed mount policy, and keeps the helper hardening rules fail-closed;
2. the storage CLI acceptance job drives the temporary legacy runtime CLI implementation (`cmd/haco`, packaged as `hacoq` during the CLI migration) as an ordinary user against real Incus. This is compatibility coverage for the shared runtime/storage path, not a claim that the reset product-facing `haco` currently exposes `create` or `run`. The job verifies that Incus creates `/var/lib/incus/disks/haco-local-default.img`, the image is sparse at the Linux-file level, a real loop device backs the pool, the configured desired state is `compress=zstd:3,noatime,nodiscard`, and the live Btrfs mount has zstd compression and `noatime` with no active discard mode or autodefrag. It also verifies that no legacy `$HACO_ROOT/images/local-default.raw` / `$HACO_ROOT/mounts/local-default` appears, that create/exec/delete/run lifecycle operations reuse the pool correctly, and that deliberately installing the old compression-only pool setting is reconciled back to the desired policy on the next rootfs operation.

`findmnt` may omit the negative/default `nodiscard` token from live mount output. Acceptance therefore requires `nodiscard` in the Incus pool configuration and proves live behavior by rejecting active `discard`/`discard=async` modes.

These checks establish lifecycle and policy behavior on the hosted environment. They do not by themselves establish compression ratio, COW efficiency, Windows-host VHDX compaction effectiveness, optimal `metadata_ratio`, or every supported Host configuration.

## Workspace boundary

Host Workspaces remain bind-mounted into Environments and are not required to live inside the Hacocoon Btrfs pool. This layout applies to Hacocoon-owned Incus rootfs/image-volume data, not arbitrary user source trees.

## Multiple pools

The rule is one shared Btrfs filesystem **per configured Hacocoon storage pool**, not one hard global filesystem for every deployment. The default local pool is `haco-local-default`; future explicitly configured pools can use their own Incus-managed storage identity without falling back to the Host default pool.
