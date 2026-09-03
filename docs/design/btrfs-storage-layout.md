# Btrfs storage layout

Status: **the normal local Incus path delegates loop-backed Btrfs lifecycle to Incus; the older Hacocoon-managed raw/helper implementation remains available for focused compatibility tests.**

Milestones: **v0.20 Managed Btrfs Rootfs Storage** and **v0.21 Managed Btrfs Transparent Compression**.

## Default local layout

The normal local composition does not create or mount a Hacocoon-owned raw image. Instead, it lazily asks Incus to create the default pool without a `source=` override:

```text
Incus pool: haco-local-default
  driver=btrfs
  size=128GiB
  btrfs.mount_options=compress=zstd:3
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

That is separate from WSL's `sparseVhd` / sparse-VHDX mode. Hacocoon does not enable WSL sparse-VHD mode as part of this storage design. Windows-host VHDX space reclamation remains an explicit maintenance concern; see the `haco maintenance compact` work tracked separately.

## Why rootfs objects share one pool

The storage boundary is deliberate. Base, Tooling, Seed, and Environment rootfs data should stay on the same Hacocoon Btrfs pool so storage-level optimizations apply across the lifecycle:

- transparent Btrfs compression can reduce physical bytes for managed rootfs data;
- Incus Btrfs snapshots and clones can preserve copy-on-write sharing;
- Seed-derived Environments only need new extents for changed data where the storage driver can share unchanged blocks;
- filesystem-level maintenance and optional out-of-band deduplication can target Hacocoon rootfs data rather than unrelated Host data.

Hacocoon must not create a separate Btrfs filesystem or loop image per Environment or Seed merely for isolation. Incus volumes/subvolumes provide the logical isolation inside the shared pool.

## Compression and defragmentation policy

The default pool uses `compress=zstd:3`. Hacocoon deliberately does **not** request `compress-force`: normal Btrfs compression heuristics may leave incompressible data uncompressed rather than repeatedly spending CPU on forced attempts.

The default policy also leaves `autodefrag` disabled. Automatic defragmentation can rewrite extents and reduce existing reflink/COW sharing, which is a poor default trade-off for an Incus snapshot/clone-heavy rootfs pool. Any future autodefrag use requires an explicit workload-specific decision rather than becoming an implicit mount default.

Compression mount options affect newly written extents. Hacocoon does not automatically rewrite all existing data merely to recompress it.

## Runtime selection rule

The local composition configures a lazy storage provider. Merely opening the local application for a command that does not need an Incus rootfs does not create the default pool.

Before the first Environment, Tooling Base builder, Seed builder, or trusted host needs root storage, the provider checks for `haco-local-default`. If it does not exist, Hacocoon asks Incus to create the Btrfs loop pool with the desired size and mount options. The runtime then reuses that pool for subsequent Hacocoon-owned rootfs operations instead of inheriting the Host's Incus default-profile pool.

An already-existing `haco-local-default` pool is reused. Hacocoon does not destructively replace an existing populated legacy pool during ordinary startup; migration of legacy pool contents must be a separate fail-safe operation.

## Legacy Hacocoon-managed storage path

The repository still contains the earlier `modules/storage/btrfs` implementation and `haco-storage-helper`. That path manages:

```text
HACO_ROOT/images/<storage-id>.raw
  -> loop device
  -> Hacocoon-managed Btrfs mount
  -> Incus pool source=<mountpoint>
```

It remains useful for focused storage-helper, block-backend, shrink/compact, hardening, and compatibility tests. Explicit `HACO_STORAGE_PRIVILEGE_MODE` or `HACO_BLOCK_BACKEND` configuration selects that compatibility path in local composition. Normal installations set neither variable and therefore use the Incus-owned pool.

The helper remains fail-closed and typed: it does not expose arbitrary root command execution. Its dedicated acceptance coverage continues independently of the normal CLI storage path.

## Acceptance coverage

Repository CI uses independent disposable Ubuntu 26.04 acceptance paths:

1. the storage-helper job exercises the retained Hacocoon-managed raw/loop/Btrfs helper boundary and its hardening rules;
2. the normal CLI job runs the actual ordinary-user `haco` binary against real Incus without installing the storage helper for that path. It verifies that Incus creates `/var/lib/incus/disks/haco-local-default.img`, the image is sparse at the Linux-file level, a real loop device backs the pool, the live mount is Btrfs with zstd compression and no autodefrag, no legacy `$HACO_ROOT/images/local-default.raw` or `$HACO_ROOT/mounts/local-default` appears, and `haco create` / `exec` / `delete` / `run` reuse the pool correctly.

These checks establish lifecycle and policy behavior on the hosted environment. They do not by themselves establish compression ratio, COW efficiency, Windows-host VHDX compaction effectiveness, or every supported Host configuration.

## Workspace boundary

Host Workspaces remain bind-mounted into Environments and are not required to live inside the Hacocoon Btrfs pool. This layout applies to Hacocoon-owned Incus rootfs/image-volume data, not arbitrary user source trees.

## Multiple pools

The rule is one shared Btrfs filesystem **per configured Hacocoon storage pool**, not one hard global filesystem for every deployment. The default local pool is `haco-local-default`; future explicitly configured pools can use their own Incus-managed storage identity without falling back to the Host default pool.
