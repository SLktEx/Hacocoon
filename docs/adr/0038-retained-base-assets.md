# ADR 0038: Retain Base material independently of image caches

Status: accepted design; catalog/coordinator implemented; provider integration planned
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

The provider-neutral catalog and retention coordinator are implemented internally.
They do not yet change ordinary creation or remove the snapshot planner's cached
image requirement. The Incus retained-material adapter, normal-create wiring,
interrupted-create recovery and referenced-asset collection remain required.
There is no new public command and no claim of real-provider retention acceptance.

See [Base contract](../design/base-images-and-custom-environments.md) and
[snapshot contract](../design/environment-snapshots.md).
