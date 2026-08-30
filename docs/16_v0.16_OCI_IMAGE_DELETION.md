# v0.16 — OCI Image Deletion

Status: **first deletion slice implemented on `main`; physical immutable-Seed rebuild/GC remains v0.17.**

v0.16 adds explicit OCI image deletion semantics under the optional OCI plugin for the trusted Host Seed cache and, when requested, currently managed Environments.

## CLI

```text
haco plugin oci image delete <reference[@sha256:...]> [--all-environments] [--json]
```

The removed pre-1.0 `haco image ...` namespace is not retained as an alias.

The default command affects the Host Seed cache and future Seed selection. It does not rewrite existing Environments.

`--all-environments` additionally attempts to remove the same immutable image identity from every managed Environment.

## Immutable deletion identity

Deletion identity is `reference + immutable digest`.

For an implicit mutable reference, Hacocoon resolves one digest from trusted telemetry. If multiple digests are observed, deletion fails closed and requires an explicit `reference@sha256:...` target.

Immediately before local removal, the mutable reference is revalidated. If the tag moved to another digest, Hacocoon refuses to delete the newer image as though it were the older target.

## Host Seed-cache deletion

The current first slice operates on the dedicated Host nerdctl namespace/cache (`hacocoon-seed`). It:

1. validates the target identity;
2. removes the matching local reference with ordinary `nerdctl rmi` when present;
3. records a trusted deletion tombstone;
4. keeps that exact identity out of v0.15 recommendation/automatic promotion;
5. leaves physical immutable-Seed replacement and old-Seed GC to v0.17.

Published immutable Seeds are never edited in place.

## All-Environment deletion

`--all-environments` preflights every managed Environment before destructive changes. Hacocoon uses the provider-neutral execution path and does not use `nerdctl rmi --force`.

An image referenced by a container may therefore fail removal rather than silently breaking the container. Partial cross-Environment completion is surfaced as recovery-required and retry treats already-absent images as no-ops.

## Tombstone semantics

A deletion tombstone is an explicit Seed-selection override, not a runtime/network denylist.

While the exact tombstone exists:

- v0.15 telemetry may still observe the image;
- recommendation/automatic promotion must not re-add the exact identity;
- an Environment may still pull/use the image through its ordinary runtime/network authority;
- re-adding the exact identity to a future Seed requires an explicit operator override.

## Security requirements

- mutable-tag ambiguity fails closed;
- moved tags are never deleted as stale identities;
- explicit old-digest deletion never removes a newer image behind the same mutable tag;
- Host credentials are never copied into Environments;
- Environment credentials are never harvested;
- all-Environment deletion is explicit;
- `--force` is not used;
- immutable published Seeds are not mutated in place;
- partial destructive work is reported as recovery-required.
