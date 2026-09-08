# ADR 0038: Retain Base material independently of image caches

Status: accepted design; catalog/coordinator and Incus storage implemented; normal creation integration implemented; installed acceptance pending
Date: 2026-09-08

## Decision

A recorded Base name and immutable revision do not preserve its material. Incus
may remove cached images while an Environment created from them remains. The
snapshot acceptance fixture encountered an absent original fingerprint. Ordinary
Environment creation must eventually retain its exact Base before publishing a
usable Environment, without another user command or required argument.

Base assets are independent immutable resources, separate from runnable
Environments, writable Workspace/OCI attachments and saved snapshot components.
Reuse is scoped by exact Base name/revision, provider and storage scope. A provider
supplies a read-only plan with fresh native identity and a bounded opaque binding;
the catalog records the full plan before create, the creation receipt before
verification, and ready only after successful verification. Reuse re-verifies the
owned material. Failed or interrupted work retains its exact ownership.

The catalog shares the canonical Environment state transaction. Schema 7 prevents
older controllers from silently dropping Base ownership. Schemas through 6 remain
readable; existing snapshot bindings are preserved. Old schemas claiming new Base
ownership, duplicate logical assets and duplicate provider-native targets fail
closed. Ready ownership is immutable. Removal is deliberately unavailable until
reference reservations and proven-absence collection can exclude Environment,
snapshot and concurrent creation users atomically.

## Rejected alternatives

- Relying on a mutable image alias or revision string as preserved material.
- Disabling a shared image cache's update or expiry policy as a retention contract.
- Recreating a missing old Base from a newer alias or modified Environment rootfs.
- Calling ordinary retained Bases snapshots or reviving the removed Seed workflow.
- Publishing ready before persisting the created resource's exact ownership.
- Releasing a planned resource after an ambiguous create or failed verification.

## Current scope

The provider-neutral catalog/coordinator and Incus retained-material adapter are
implemented internally. Incus initializes a stopped, profile-free Base instance
from the exact pinned source and records fresh asset ownership at creation.
Verification checks exact root storage and metadata without consulting the image
cache. Snapshot and retained-Base validation share the same ownership checks but
keep distinct namespaces and unchanged persisted snapshot bindings.

The local composition uses retention during ordinary creation; the snapshot
planner's cached-image requirement remains unchanged. Ambiguous planned-create
recovery and referenced-asset collection remain required.
There is no new public command. Dedicated WSL proved retained material survives
explicit source-image deletion and catalog reload, then verified owned cleanup.
Ordinary-create and restore acceptance remain separate.

See [Base contract](../design/base-images-and-custom-environments.md) and
[snapshot contract](../design/environment-snapshots.md).

Recovery from a durable `created` receipt is now automatic on the next Ensure:
verify the exact existing material again, then atomically publish ready. Failed
verification or publication preserves ownership. A `planned` row still cannot be
adopted from a stopped-instance observation alone: the original provider operation
may be incomplete even if a same-name instance appears. Resolving ambiguous creates
requires stronger provider completion evidence or proven-safe cleanup.

Ordinary creation now invokes the composition-owned retention coordinator before
any Environment init, using the already-resolved pinned source. Failures belong
to the independent Base reservation, not a nonexistent Environment resource;
therefore they must not strand an unused Workspace lease as Environment recovery.
The Incus create boundary still requires an exact ready Base receipt. Assets
remain retained after Environment deletion; no implicit garbage collection is
introduced. Installed acceptance and snapshot lookup integration remain pending.

Base assets use the same canonical `runtime.incus` provider identity as runtime
routing. The initial adapter-only fixtures used `incus`, which disagreed with
production composition and blocked ordinary creation; the adapter and fixtures
now use the shared constant. No automatic adoption or rewriting of old asset
receipts is performed.
