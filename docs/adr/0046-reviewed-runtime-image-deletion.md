# ADR 0046: Review native OCI image identity before removal

Status: accepted

## Decision

OCI runtimes own image inventory, references and deletion. Keep runtime-specific
ID conversion in the OCI plugin. The Core lifecycle service only provides exact
Env-generation/attached-resource execution under its existing lifecycle lock.
The wire requires that reviewed identity; it cannot select Host executables,
sockets, namespaces or credentials. Clearing the CLI environment avoids inherited
runtime connection overrides. Never forward arbitrary image metadata to logs.

Refuse known container users, including stopped containers, and use native removal
without force. Resolve tags in the preview and send the immutable runtime ID to
removal. Native failure or unproven post-removal absence is failure. Do not create
an image registry, tombstones, backup or rollback to model native runtime behavior.
Independent Store/snapshot copies retain their content. Host-source and detached
Store operations require their existing ownership/Host-copy boundaries and remain
separate work, rather than pretending attached-Env execution covers them.

## Alternatives rejected

- Treat Docker config IDs and containerd manifest IDs as interchangeable.
- Execute a queued operation by Env name alone after recreation.
- Force-delete images to make all runtimes appear to have identical semantics.
- Delete runtime layer files or resurrect legacy Seed selection state.

## Compatibility and validation

No persistent schema changes or data migration. Existing Core/native ownership and
network/credential guards remain. Focused tests exercise generation and Store
replacement, malformed observations, container users, confirmation and absence.
The maintained isolated COW fixture exercises real Docker/nerdctl formats and
removal separately from installed-controller/CLI acceptance.
