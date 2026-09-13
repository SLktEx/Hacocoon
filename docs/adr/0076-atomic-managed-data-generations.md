# Atomic managed-data generation selection

Status: accepted; implementation candidate. [日本語](0076-atomic-managed-data-generations.ja.md)

## Decision

Represent current generation selection in the same durable catalog transaction
as managed-resource readiness, ownership and deletion exclusions. A producer
supplies the complete expected generation, including a random reset epoch and
opaque compatibility identity. Publish a whole verified source through canonical
resource creation, then atomically compare and select. Stale producers skip;
ambiguous persistence retains resources. Current selections pin deletion.

Core owns the provider-neutral identity and lifecycle contract. Standard owns
cache configuration, compatibility meaning and ordinary-Env collection. Incus
owns custom-volume mechanics and Btrfs copying. Guest cache contents never become
trusted Host tools/configuration; cache volumes cannot enter OCI Host control paths.

Reset changes the epoch and clears only the selection. It does not delete old
resources or overwrite Env-local copies. Successful stale-candidate cleanup uses
the same positive-absence finalizer as other managed-resource deletions.

## Rejected alternatives

A separate cache index with an independent lock would race catalog deletion and
leave a source pointing at a deleted volume. Comparing only the generation number
would let old producers pass after reset. File-by-file merging would violate the
agreed whole-generation semantics and assume tool-specific consistency rules.
Deleting on every selection error could remove a successfully selected source
after an ambiguous write. A separate cleanup implementation would duplicate the
ownership-release decision. Sharing the OCI attachment path would expose cache
contents through a tool-specific mount and nesting policy.

This is a foundation for the [cache contract](../design/cache-generations.md).
Host settings, path collection and disposable Env attachment are still required;
the decision does not claim that the complete user workflow is implemented.
