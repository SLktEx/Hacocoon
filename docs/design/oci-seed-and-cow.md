# OCI Seed retirement

[日本語](oci-seed-and-cow.ja.md) | English

Status: **implemented retirement candidate**. Legacy version compatibility and
migration are out of scope for the current M0–M5 work, by the user's 2026-09-15
scope correction. No legacy catalog reader or conversion shim remains.

New Environments resolve the explicitly selected Base and pinned revision. OCI
plugin configuration no longer substitutes a Seed or changes nested-container
permissions. Seed composition, resolver, commands, harvest, build/publication,
maintenance, catalog, sampling, recommendation, tombstones and re-enable code are
removed. Current Base builds, independent Workspaces, snapshots, export/import,
managed image deletion and persistent OCI Stores keep their existing contracts.
See [ADR 0080](../adr/0080-seed-runtime-retirement.md).

## Current boundary

No resource deletion is triggered by code retirement. Setup and ordinary creation
neither read nor migrate `seeds.json` or `oci-usage.json`. There is no automatic
cleanup of old native images, namespaces, files or installations. Existing resource
ownership and ordinary retained-data lifecycle checks remain in their canonical
implementations. Shared managed-kind markers retain their values in Environment
identity code and do not replace ownership checks.

The current `haco plugin oci image ...` path queries its owning runtime and uses
reviewed deletion; it does not share the removed Seed image-deletion machinery.
Optional Docker integration remains separate. `switch-base` is not being restored.
Use [data lifetime](../guides/data-lifetime.md) and
[current image deletion](oci-image-deletion.md) for current operations.

## Evidence and continuing constraints

The old pipeline and its [historical evidence](../status/seed-private-registry-acceptance.md)
remain in Git history and the evidence record. Its Seed-only private-registry
fixture/manual job is retired with the removed production path. That evidence is
not current Store credential acceptance, and old-version migration is not a gate.

Never share one writable `/var/lib/containerd` across independent Environments,
copy reusable Host credentials into images, manipulate Incus-owned Btrfs subvolumes
in Core, or release ownership after ambiguous cleanup. Current Base and OCI Store
operations retain their own lifecycle, authority and CoW contracts. Performance
measurement and additional acceptance follow the usable implementation.
