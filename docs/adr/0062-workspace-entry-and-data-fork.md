# ADR 0062: Compose Workspace entry and data forks

Status: accepted
Date: 2026-09-13

## Context

Managed Workspaces already own independent Incus Btrfs repository volumes,
immutable collections, canonical aggregate leases and associated persistent OCI
Stores. Environment create/start/delete and stopped capture already have durable
ownership and failure semantics. A work-oriented entry must preserve those
boundaries while making reopen and independent parallel work straightforward.

## Decision

Add a Standard orchestration layer and a local path reference pinned to the
existing managed Workspace owner. Reuse canonical Env creation/resume, existing
collection copies and snapshot-backed repo/OCI restoration. Keep all durable
provider ownership in its current catalog. Fork captures a stopped source and
restores retained data only; new Envs choose their Base independently.

Do not interpret a path as permission to relocate its existing files or share a
Host checkout with untrusted code. Explicit repository selection prepares the
isolated copy; subsequent path opens resolve the saved owner. Capture receives
an expected Workspace identity checked under the lifecycle lock. Creation also
pins the reviewed Workspace before default OCI preparation or provider mutation.

Associated-data restoration runs before the existing Workspace registry publishes
ready state. An unfinished fork remains unavailable and retains its source
reservation and owned data; independently owned OCI resources are never cleaned
up as if they belonged to the failed fork.

## Rejected alternatives

- A replacement Workspace catalog would duplicate existing ownership receipts.
- Reconstructing lifecycle from metadata and lease mutations would reintroduce
  the failures excluded by ADR 0002.
- Sharing Git indexes, worktree metadata or writable OCI volumes would couple
  independent workers.
- Copying data from a running Env without canonical quiescence would bypass the
  existing stopped aggregate consistency contract.
- Resuming saved rootfs by default would conflate persistent work with its Base.

## Consequences

Path references are convenience metadata, not a new authority or storage engine.
A stopped source Env is required for aggregate fork; detached work can be opened
and stopped first. Fork leaves source stopped and starts no destination Env.
Failed operations retain canonical recovery records. Large-repository performance
requires separate native measurement.
