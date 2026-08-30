# Hacocoon-managed Btrfs storage layout

Status: **repository implementation present; physical compression/COW/compaction acceptance remains host-dependent.**

Milestones: **v0.20 Managed Btrfs Rootfs Storage** and **v0.21 Managed Btrfs Transparent Compression**.

Hacocoon local storage uses one sparse raw backing image per configured storage pool and formats that image as Btrfs. Incus uses the mounted Btrfs filesystem as the source for the corresponding Hacocoon-managed storage pool.

```text
HACO_ROOT/images/<storage-id>.raw   (sparse raw)
        |
        v
     loop device
        |
        v
  Btrfs filesystem
        |
        v
Incus pool: haco-<storage-id>
  |- cached Base image volumes
  |- Tooling Base builders
  |- Seed builders / cached Seed image volumes
  |- Environment rootfs volumes
  `- Incus snapshots / clones
```

The default local storage ID is `local-default`, so the default Incus pool is `haco-local-default`.

## Why these objects share one filesystem

The storage boundary is deliberate. Base, Tooling, Seed, and Environment rootfs data should stay on the same Hacocoon-managed Btrfs filesystem so storage-level optimizations apply across the complete lifecycle:

- transparent Btrfs compression can reduce physical bytes for all managed rootfs data;
- Incus Btrfs snapshots and clones can preserve copy-on-write sharing;
- Seed-derived Environments only need new extents for changed data where the storage driver can share unchanged blocks;
- filesystem-level maintenance and optional out-of-band deduplication can target only the Hacocoon filesystem rather than unrelated Host data;
- compaction can return unused extents to the sparse raw backing file.

Hacocoon must not create a separate Btrfs filesystem or sparse image per Environment or Seed merely for isolation. Incus volumes/subvolumes provide the logical isolation inside a shared storage pool.

## Compression policy

Managed Btrfs filesystems use `compress=zstd:3` by default. Hacocoon deliberately does **not** use `compress-force`; normal Btrfs compression heuristics may leave incompressible data uncompressed rather than repeatedly spending CPU on forced attempts.

When Hacocoon finds an already-mounted managed filesystem without the expected compression option, it remounts that managed filesystem with `compress=zstd:3`. A `compress-force` mount is not considered compliant with the managed desired state.

The compression mount option affects newly written extents. Hacocoon does not automatically defragment or recompress existing data because rewriting extents can reduce existing reflink/COW sharing. Physical compression ratio, CPU cost, and supported-host behavior remain acceptance concerns rather than repository-only claims.

## Runtime selection rule

The local composition configures a lazy Hacocoon Btrfs storage provider. Merely opening the local application for a command that does not need an Incus rootfs must not attach a loop image, mount Btrfs, or create an Incus storage pool.

Before the first Environment, Tooling Base builder, or Seed builder needs root storage, the Incus runtime resolves the configured provider, ensures the sparse-raw Btrfs storage and its `haco-<storage-id>` Incus pool, records that pool, and reuses it for subsequent Hacocoon-owned rootfs operations. These paths therefore do not inherit the Host's Incus default profile pool.

A low-level Incus runtime used directly without the Hacocoon local composition retains its legacy default-profile behavior for compatibility. That compatibility path is not the local Hacocoon storage architecture.

## Workspace boundary

Host Workspaces remain bind-mounted into Environments and are not required to live inside the Hacocoon Btrfs pool. This storage layout applies to Hacocoon-owned Incus rootfs/image-volume data, not arbitrary user source trees.

## Multiple pools

The rule is one shared Btrfs filesystem **per configured Hacocoon storage pool**, not one hard global filesystem for every possible Hacocoon deployment. Runtime preparation or a configured storage provider selects the `incus_pool` from the storage attachment, so another configured storage ID can map to another `haco-<storage-id>` pool without falling back to the Host default pool.
