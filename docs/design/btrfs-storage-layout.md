# Hacocoon-managed Btrfs storage layout

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

## Runtime selection rule

The local composition creates/ensures the Hacocoon Btrfs storage first, then prepares the Incus runtime with that storage attachment. Environment, Tooling Base, and Seed creation use the prepared pool instead of inheriting the Host's Incus default profile pool.

A low-level Incus runtime used directly without the Hacocoon local composition retains its legacy default-profile behavior for compatibility. That compatibility path is not the local Hacocoon storage architecture.

## Workspace boundary

Host Workspaces remain bind-mounted into Environments and are not required to live inside the Hacocoon Btrfs pool. This storage layout applies to Hacocoon-owned Incus rootfs/image-volume data, not arbitrary user source trees.

## Multiple pools

The rule is one shared Btrfs filesystem **per configured Hacocoon storage pool**, not one hard global filesystem for every possible Hacocoon deployment. Runtime preparation records the selected `incus_pool` from the storage attachment, so another configured storage ID can map to another `haco-<storage-id>` pool without falling back to the Host default pool.
