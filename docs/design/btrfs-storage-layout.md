# Btrfs storage layout

Status: **the supported local storage path is Incus-owned loop-backed Btrfs. Hacocoon does not provide a legacy Host-owned raw/loop/mount compatibility path.**

Milestones: **v0.20 Managed Btrfs Rootfs Storage** and **v0.21 Managed Btrfs Transparent Compression**.

The former **Hacocoon-managed Btrfs storage layout** used a **sparse raw** file, `haco-<storage-id>` external-source pools, and `haco-storage-helper`. Those implementation paths are removed; these names are retained here only to make the unsupported boundary explicit.

## Default local layout

The local composition lazily asks Incus to create the default pool without a `source=` override:

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

Incus owns creation of the backing image, loop attachment, Btrfs formatting, mount/unmount lifecycle, and supported loop-pool growth. Hacocoon owns only the desired pool identity and policy.

There is deliberately no Hacocoon-managed `HACO_ROOT/images/<id>.raw -> loop -> mount -> source=<mountpoint>` path. Pre-existing installations built around that removed layout are unsupported; recreate the installation instead of migrating or recovering that storage shape.

This single-owner boundary is especially useful on WSL because Hacocoon no longer has to race `incusd` while creating or restoring Host mounts.

## Sparse file versus WSL sparse VHD

Incus' loop-backed Btrfs pool uses a sparse **Linux file**. The logical 128 GiB pool size is not eagerly allocated in full.

That is separate from WSL's `sparseVhd` / sparse-VHDX mode. Hacocoon does not enable WSL sparse-VHD mode as part of this storage design. Windows-host VHDX space reclamation remains an explicit maintenance concern tracked separately from pool ownership.

## Why rootfs objects share one pool

Base, Tooling, Seed, trusted-host, and Environment rootfs data share the Hacocoon Btrfs pool so Incus can apply its Btrfs storage-driver behavior across their lifecycle:

- transparent Btrfs compression reduces physical bytes where data is compressible;
- Incus Btrfs snapshots and clones can preserve copy-on-write sharing;
- Seed-derived Environments can share unchanged extents where supported;
- storage maintenance stays scoped to Hacocoon rootfs data rather than arbitrary Host data.

Hacocoon does not create a separate Btrfs filesystem or loop image per Environment or Seed merely for isolation. Incus volumes/subvolumes provide logical isolation inside the shared pool.

## Compression and defragmentation policy

The default pool requests `compress=zstd:3`. `compress-force` is intentionally not desired state; normal Btrfs compression heuristics may leave incompressible data uncompressed.

`autodefrag` is also intentionally disabled by default. Automatic defragmentation can rewrite extents and reduce reflink/COW sharing, which is a poor default for a snapshot/clone-heavy rootfs pool.

Compression mount options mainly affect newly written extents. Hacocoon does not automatically rewrite all existing data merely to recompress it.

## Runtime selection rule

The local composition configures a lazy storage provider. Opening a command that does not need Incus root storage does not create the pool.

Before the first Environment, Tooling Base builder, Seed builder, or trusted host needs root storage, the provider checks for `haco-local-default`. If it does not exist, Hacocoon asks Incus to create the Btrfs loop pool with the desired size and mount options. Subsequent Hacocoon-owned rootfs operations use that pool rather than the Host's unrelated Incus default-profile pool.

The supported state is the current Incus-owned pool shape. Hacocoon does not implement migration, recovery, or startup reconciliation for the removed external-path storage layout.

## Acceptance coverage

Repository CI runs the actual ordinary-user `haco` binary against real Incus and verifies the supported storage path: Incus creates its loop-backed Btrfs pool, the backing image is sparse at the Linux-file level, the live filesystem uses zstd compression without autodefrag, and normal `haco create` / `exec` / `delete` / `run` operations reuse the pool.

These checks establish lifecycle and policy behavior on the hosted environment. They do not by themselves establish compression ratio, COW efficiency, Windows-host VHDX compaction effectiveness, or every supported Host configuration.

## Workspace boundary

Host Workspaces remain bind-mounted into Environments and are not required to live inside the Hacocoon Btrfs pool. This layout applies to Hacocoon-owned Incus rootfs/image-volume data, not arbitrary user source trees.

## Multiple pools

The default local pool is `haco-local-default`. Future explicitly configured pools may use their own Incus-managed storage identity, but Hacocoon does not reintroduce a second Host-owned block/mount lifecycle to support them.
