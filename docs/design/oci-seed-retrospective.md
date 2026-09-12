# OCI Seed rationale and superseded direction

[日本語](oci-seed-retrospective.ja.md) | English

Status: **historical**. The 2026-08-31 proposal to make OCI application containers
siblings under Incus is not the current default or a completed migration.
Current B4 uses [persistent OCI Stores](persistent-oci-store.md) and the actual
[Host-area copy boundary](../adr/0031-host-oci-area-copy.md).
The retained [Seed implementation](oci-seed-and-cow.md) is a legacy optional path.

Seed addressed duplicated OCI data when every Env had an independent containerd
runtime. Sharing writable `/var/lib/containerd` would break isolation and independent
deletion. Instead, a credential-free offline builder imported pinned images into
an immutable rootfs, which Incus/Btrfs cloned into private writable Envs.
Build/publication, pin/deletion policy, harvest, recovery and GC protected that model.

The historical native-OCI proposal aimed to remove the bridge between Incus lifecycle
and nested runtime storage. It proposed one Incus daemon owning sibling workloads,
controller-mediated operations and operation-scoped registry credentials.
Neither those capabilities nor private ECR authentication were established by the
proposal. They must not be described as current runtime behavior.

The lasting constraints are independent of that proposal:

- Resolve mutable image input to immutable identity where policy/reproducibility requires it.
- Never share one writable runtime root across independent active workloads.
- Keep registry credentials, helper output and Host sessions out of images/snapshots.
- Delegate COW and image/volume lifecycle to Incus rather than manipulating its subvolumes in Core.
- Retain resources when ownership, aliases, users or cleanup completion are uncertain.
- Do not publish partially completed pull/build/copy data as current.

A future native-OCI integration would need its own lifecycle/network/exposure,
private-registry credential-retention, physical-storage and failure/restart evidence.
It is not a prerequisite for today's Store workflow. Full obsolete migration recipes
remain in Git history; this record retains the rationale without scheduling that migration.
