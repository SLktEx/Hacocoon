# ADR 0080: Remove Seed implementation without a compatibility layer

Status: accepted, implementation candidate. [日本語](0080-seed-runtime-retirement.ja.md)

Date: 2026-09-15

## Decision

Resolve the explicit Base and revision when creating an Environment. Remove Seed
composition, resolver options, builder/publication, harvest, maintenance and CLI.
An OCI plugin toggle must never silently replace that Base or grant additional
nested-container permissions. Current Base and persistent OCI contracts remain.

The user explicitly excluded old-version compatibility and migration because the
product is still under test. Remove sampling, recommendation and shared historical
usage/deletion catalogs completely, including the temporary image delete/re-enable
commands. Do not retain a reader, data converter or inactive resolver for future
callers. The current managed-image path is the canonical image-deletion workflow.
Keep optional Docker behavior independent of the removed Seed state and Host runner.

Remove tests and the manual private-registry job that only exercised the retired
production path. Keep historical evidence distinguished from current acceptance.
Normal current-version tests must continue proving the actual product path.

## Rejected alternatives

A disabled resolver or legacy observation reader preserves accidental complexity
and an alternate state path without a current product requirement. A plugin toggle
is not authorization for image substitution. `switch-base` is not revived.

Code retirement is not a deletion operation. Do not automatically remove user
files, Workspaces, OCI Stores or native objects, release uncertain ownership, or
weaken current lifecycle/authority checks to make removal pass.

See [Seed retirement](../design/oci-seed-and-cow.md) and
[data lifetime](../guides/data-lifetime.md).
