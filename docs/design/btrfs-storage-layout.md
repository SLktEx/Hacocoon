# Btrfs storage layout

Status: **the supported local storage path is Incus-owned loop-backed Btrfs.**

Milestones: **v0.20 Managed Btrfs Rootfs Storage**, **v0.21 Managed Btrfs Transparent Compression**, and **v0.25 Incus-owned Btrfs Storage Acceptance**.

## Default local layout

The local composition lazily asks Incus to create the default pool without a `source=` override:

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
  |- trusted haco-host rootfs
  |- Environment rootfs volumes
  `- Incus snapshots / clones
```

Incus owns creation of the backing image, loop attachment, Btrfs formatting, mount/unmount lifecycle, and supported loop-pool growth. Hacocoon owns only the desired pool identity and policy. There is no second Host-managed block or mount lifecycle in Hacocoon.

## Sparse file versus WSL sparse VHD

Incus' loop-backed Btrfs pool uses a sparse **Linux file**. The logical 128 GiB pool size is not eagerly allocated in full. This is separate from WSL's `sparseVhd` / sparse-VHDX mode. Hacocoon does not enable WSL sparse-VHD mode as part of this storage design; Windows-host VHDX reclamation remains an explicit Host/operator concern.

## Why rootfs objects share one pool

Base, Tooling, Seed, trusted-host, and Environment rootfs data share the Hacocoon Btrfs pool so Incus can apply its Btrfs storage-driver behavior across their lifecycle:

- transparent Btrfs compression reduces physical bytes where data is compressible;
- Incus Btrfs snapshots and clones can preserve copy-on-write sharing;
- Seed-derived Environments can share unchanged extents where supported;
- storage maintenance stays scoped to Hacocoon rootfs data rather than arbitrary Host data.

Hacocoon does not create a separate Btrfs filesystem or loop image per Environment or Seed merely for isolation. Incus volumes/subvolumes provide logical isolation inside the shared pool.

## Managed mount policy

The desired mount policy is:

```text
compress=zstd:3,noatime,nodiscard
```

`compress=zstd:3` enables transparent compression without `compress-force`; normal Btrfs heuristics may leave incompressible data uncompressed. `noatime` avoids read-triggered access-time metadata writes and unnecessary COW churn. `nodiscard` disables continuous discard so reclamation can remain an explicit batch operation. `autodefrag` is intentionally not enabled because automatic defragmentation can rewrite extents and reduce reflink/COW sharing in a snapshot/clone-heavy rootfs pool.

Mount options mainly affect newly written extents. Hacocoon does not automatically rewrite all existing data merely to recompress it.

## Runtime selection rule

The local composition configures a lazy storage provider. Opening a command that does not need Incus root storage does not create the pool.

Before the first Environment, Tooling Base builder, Seed builder, or trusted host needs root storage, the provider checks for `haco-local-default`. If it does not exist, Hacocoon asks Incus to create the Btrfs loop pool with the desired size and mount options. Subsequent Hacocoon-owned rootfs operations reuse that pool rather than the Host's unrelated Incus default-profile pool.

When `haco-local-default` already exists, Hacocoon reconciles `btrfs.mount_options` to `compress=zstd:3,noatime,nodiscard` before reuse. The populated pool is not destructively recreated; Incus remains the lifecycle and remount owner.

The runtime expects this Incus-owned pool shape and carries no alternate Host-managed storage ownership path.

## Acceptance coverage

Repository CI drives the temporary legacy runtime CLI (`cmd/haco`, packaged as `hacoq` during the CLI migration) as an ordinary user against real Incus. It verifies that Incus creates its loop-backed Btrfs pool, the backing image is sparse at the Linux-file level, the configured desired state is `compress=zstd:3,noatime,nodiscard`, and the live filesystem has zstd compression and `noatime` with no active discard mode or autodefrag. It also verifies create/exec/delete/run lifecycle operations reuse the pool and that an old compression-only pool setting is reconciled back to the desired policy.

`findmnt` can omit the negative/default `nodiscard` token. Acceptance therefore requires `nodiscard` in the Incus pool configuration and rejects active `discard` / `discard=async` modes on the live mount.

These checks establish lifecycle and policy behavior on the hosted environment. They do not by themselves establish compression ratio, COW efficiency, Windows-host VHDX compaction effectiveness, or every supported Host configuration.

## Workspace boundary

Host Workspaces remain bind-mounted into Environments and are not required to live inside the Hacocoon Btrfs pool. This layout applies to Hacocoon-owned Incus rootfs/image-volume data, not arbitrary user source trees.

## Multiple pools

The default local pool is `haco-local-default`. Future explicitly configured pools may use their own Incus-managed storage identity while preserving the same single-owner lifecycle.
