# ADR 0017: Initialize default resources inside Environment creation

Status: accepted
Date: 2026-09-07

## Decision

Ordinary Environment creation may obtain its default persistent resource through
an optional provider-neutral initializer. It runs under the canonical Environment
and Workspace locks, after duplicate checks and before the atomic Environment /
lease reservation. Core does not import an OCI implementation. The maintained OCI
integration supplies automatic copies; `haco env create --no-oci` skips this
default, while an explicit resource selects the advanced path. Combining explicit
selection with opt-out is invalid.

The OCI integration derives a stable resource ID from Workspace identity and
persists that Workspace association before provider copying. Existing ready
resources are revalidated and reused, preserving user edits across Environment
deletion/recreation. A same-name resource with another association is never
adopted. A failed copy remains identifiable and recovery-required; creation does
not continue with empty data. Successful resources survive later Environment
creation failure and remain explicitly deletable after leases are released.

Sources carry a generic `source_only` role that excludes workload attachment.
The actual Host area may change outside a reserved copy; the role does not imply
an immutable image publication. See [ADR 0031](0031-host-oci-area-copy.md). The catalog
forbids direct Environment attachment of these resources and attachment of a
Workspace-bound resource to another Workspace. Incus mirrors/verifies the source
role in provider metadata and also rejects direct source attachment. Copying
produces a fresh ordinary resource; it does not propagate the source-only role.
`PublishSource` records the exact identity, creates storage, invokes preparation,
verifies the detached resource and only then commits ready state. Copy and delete
cannot overtake an unfinished publication. Publication or copy failure retains
ownership until an explicit recovery path can prove producer quiescence.

## Rejected alternatives

- Mandatory create/copy/attach commands repeat setup work for every Environment.
- Running preparation outside the canonical locks permits inverse-operation races.
- Recopying on every recreation discards guest modifications.
- Adopting by a deterministic name alone confuses independently owned resources.
- Making a source ready before its producer finishes exposes partial images.
- Reattaching a guest-used Store to Host crosses the intended trust boundary.

## Scope and acceptance

The default resolver currently consumes a ready `oci-source:host` publication.
Absence means there is no published optional content, not a failed copy. The Host
Docker/nerdctl image producer, refresh protocol, Docker persistence and image/runtime
acceptance remain incomplete. The source-only storage API and synthetic fixtures
do not prove these missing product paths. Existing explicit Store operations stay
available for advanced/recovery use. The full B4 requirement remains open.
