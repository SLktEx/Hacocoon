# v0.11 — Base Images & Custom Environments

Status: **first implementation slice present on `main`; real-Incus acceptance and richer Base lifecycle remain pending.**

This document defines the v0.11 Base-image contract. `BASE_IMAGES.md` remains the broader design companion. `IMPLEMENTATION_STATUS.md` is authoritative for current code reality.

## Goal

The product CLI exposes `haco base list`, `haco base inspect <base>` and
`haco env create --base <base> --workspace managed:<workspace> <environment>`.
`haco env switch-base` is currently disabled and is not a Stage B requirement.
Its need, semantics and CLI UX are deferred to Stage D or later, without blocking
Stage A-C. The underlying historical composition/tests and
[ADR 0011](../adr/0011-managed-workspace-base-switch.md) remain available.

When necessary, use ordinary Environment deletion and creation with another
Base, then attach the retained Workspace and optional persistent resource.
This lifecycle is available today; it does not prescribe a future helper UX.
See [ADR 0014](../adr/0014-persistent-managed-resources.md).

Allow an Environment to start from a selectable Hacocoon **Base** without exposing Incus image aliases, remotes, or fingerprints as Core/public architecture.

```text
logical Base name
        |
        v
provider-owned mutable source
        |
 resolve once at create
        v
immutable Base revision
        |
        v
Environment
```

An Environment records one immutable Base revision at creation time. Updating a logical Base affects only future Environment creation.

## Public/domain model

The Core/domain vocabulary is provider-neutral:

```text
BaseName
BaseRevision
BaseRef
Environment
```

The Incus adapter may resolve a Base to an Incus image fingerprint internally, but Incus alias/remote/fingerprint names are not required Core concepts.

The current implementation persists `BaseRef{Name, Revision}` on the Environment record.

## Implemented first slice

The current v0.11 implementation includes:

1. deterministic default Base selection for Incus;
2. explicit Base selection through `haco create --base <base> ...`;
3. `haco base list` for configured logical Base names;
4. `haco base inspect <base>` to resolve and display the current immutable revision;
5. immutable revision persistence on the Environment;
6. adapter-side alias/source -> fingerprint resolution before `incus init`;
7. initialization from the pinned fingerprint rather than the mutable alias;
8. official logical Bases plus operator-defined custom logical mappings;
9. adversarial input validation for Base names, adapter sources, and returned fingerprints;
10. tests proving alias movement changes only future resolution, not an already-recorded revision.

The current CLI is pre-1.0 and may change:

```text
haco base list [--json]
haco base inspect <base> [--json]
haco create --base <base> --workspace <path> <environment>
```

The Base namespace is intentionally distinct from OCI/container image operations, which live under the optional `haco plugin oci ...` namespace.

## Official and custom Bases

The Incus adapter currently provides official logical names including:

```text
haco/ubuntu-26.04
haco/ubuntu-24.04
```

Host/operator-defined logical mappings can be supplied through:

```text
HACO_INCUS_BASES_JSON
```

Example:

```json
{"my-dev":"images:my-moving-alias"}
```

The `haco/` namespace is reserved for Hacocoon-owned logical Bases and cannot be overridden by custom mapping input.

`HACO_INCUS_BASES_JSON` is an adapter configuration detail, not a frozen Core/public schema.

## Immutable revision rule

The central contract is:

> A Hacocoon Environment is created from one immutable Base revision. Updating a logical Base affects future Environment creation only.

Example:

```text
my-dev -> revision A
Environment 1 -> revision A

move my-dev -> revision B

Environment 1 -> revision A
Environment 2 -> revision B
```

Creation performs this sequence:

```text
logical Base
  -> adapter source
  -> `incus image info ...`
  -> validate full immutable fingerprint
  -> derive provider-neutral BaseRevision
  -> `incus init` from pinned fingerprint
  -> persist BaseRef on Environment
```

The mutable alias is therefore not used at `incus init` after resolution.

## Incus boundary

Incus remains the first/default Environment implementation.

The adapter may internally keep:

```text
Hacocoon Base name
        |
        v
Incus source/alias
        |
        v
Incus image fingerprint
        |
        v
Hacocoon BaseRevision
```

The following stay outside Core except for explicit Incus-specific diagnostics/configuration:

- Incus image aliases;
- Incus remotes;
- Incus fingerprints;
- native import/publish mechanics.

This leaves room for later providers to map a Hacocoon Base to their own immutable starting-point mechanism.

## Security contract

A Base controls guest filesystem/runtime contents. It does **not** grant host-side authority.

Selecting a Base must not implicitly add:

- host filesystem mounts beyond the normal Workspace contract;
- Incus devices;
- privileged-container mode;
- Linux capabilities;
- host network authority;
- GitHub/AWS/cloud credentials;
- SSH private keys;
- registry credentials;
- Hacocoon/Incus control-plane authority.

Custom Base contents are untrusted.

The first slice does not expose arbitrary Incus config passthrough. Base names/sources/fingerprints are validated and passed as argv values rather than shell-interpolated strings.

## Reference and deletion safety

The first slice intentionally does **not** implement Hacocoon Base deletion, physical image garbage collection, history pruning, or rollback.

That omission is deliberate: Hacocoon therefore cannot currently delete a referenced Base revision through this public Base API.

When deletion/GC is added, a revision must not be physically removed while a running or recoverable Environment depends on it. If Hacocoon cannot prove deletion is safe, it must retain storage rather than destroy a dependency.

Concurrent operations such as these remain future lifecycle work:

```text
create --base my-dev  vs update my-dev
create --base my-dev  vs remove my-dev
gc                    vs create --base my-dev
```

## Build/import trust boundary

Custom image build/import/history/rollback/GC are **not part of the first implemented slice**.

When build/import is introduced, arbitrary image contents and build steps must not execute directly with Hacocoon host authority. Local archives must be treated as untrusted data, including path traversal, unsafe symlink, malformed metadata, resource exhaustion, and partial-import cleanup risks.

## Project Setup boundary

Repository/workspace-specific setup remains distinct from a reusable Base:

```text
Base
  common OS/runtime/tooling
        |
        v
Project Setup
  workspace-specific dependency/setup work
        |
        v
Environment
```

The exact project-setup schema is not frozen by v0.11.

## Acceptance status

Repository-level acceptance for the current first slice covers:

- multiple selectable logical Bases;
- explicit create-time Base selection;
- alias/source resolution to a validated immutable revision;
- pinned fingerprint use for actual Incus initialization;
- persisted `BaseRef` identity;
- list/inspect CLI behavior;
- alias movement not rewriting a previously resolved revision;
- reserved official namespace;
- malformed Base/source/fingerprint rejection;
- provider-neutral Core types;
- existing v0.1-v0.10 lifecycle/security boundaries remaining intact;
- host-independent fake-Incus E2E.

Real-Incus image-remote/custom-image acceptance remains separate because repository CI cannot substitute for a supported real Incus host.

The broader design acceptance for build/import/deletion/history/rollback/GC remains pending because those operations are intentionally not exposed yet.

## Relationship to earlier and later gates

v0.8 introduced the thin `haco-vscode` Client Adapter. v0.9 added the trusted per-agent Environment broker. v0.10 added the `haco-agent-host` bridge for the VS Code Agents window.

v0.11 changes the starting filesystem/runtime Base of an Environment; it does not move Base authority into VS Code or an AI/orchestrator UI.

v0.12 Resource Budgets must compose with v0.11 so a custom Base cannot raise or disable host-selected resource ceilings.

## Explicit non-goals of the first slice

- changing the Base of an existing Environment in place;
- transparent migration of existing Environments to newer Base revisions;
- exposing the full Incus image API through `haco`;
- custom Base build/import;
- revision history and rollback;
- physical image deletion/GC;
- snapshotting arbitrary live Environments into reusable Bases;
- baking reusable credentials into images;
- freezing CLI/configuration compatibility before 1.0.

## Detailed companion

See [`../BASE_IMAGES.md`](../BASE_IMAGES.md) for the broader design and future lifecycle work.

> **v0.11 now gives Environment creation a provider-neutral logical Base that is resolved once to an immutable revision and persisted; mutable Incus image names remain adapter details.**

## Independently retained Base assets

Partial: schema 7 now stores exact Base-asset ownership alongside Environment and
snapshot state. The internal retention coordinator reserves a provider-native
plan, immediately records successful creation, verifies it and publishes ready.
A ready asset can be reused only for the same name/revision, provider and storage
scope and only after provider verification. An ambiguous create, failed receipt or
failed verification leaves recoverable ownership; it does not trigger another
create or forget the resource. Existing snapshot schema 6 bindings survive upgrade.

The local Incus composition now connects retention automatically during ordinary
Environment creation. The snapshot planner now prefers the exact retained Base asset;
using retained material for snapshot capture remains follow-up work. No command or required
argument is added. Asset removal requires future reference-aware collection; this
slice exposes no deletion API. See [ADR 0038](../adr/0038-retained-base-assets.md).

The Incus adapter creates a stopped `haco-base-<owner>` with the exact pinned
image, no profiles, no host devices and no autostart. It rechecks Btrfs placement
before creation. Canonical bindings qualify project/pool, source revision, owner,
asset ID and provider before provider access. Verification checks the independently
retained rootfs rather than re-querying the source image. Existing snapshot Base
validation uses the same storage checks while retaining its own namespace.
Component regressions cover cache-independent verification, lost create replies,
binding drift and inherited authority refusal. Real-provider E2E is included in
the existing Incus workflow; its new execution is pending.

### Retained-material acceptance

Dedicated WSL Incus 6.0.5 passed TestRealIncusBaseAssetE2E in 10.50 seconds.
Asset base-b8b35a1f72ffee3934eb3916ecce88a6 reached ready through the real catalog
and retention coordinator. The fixture proved project image isolation and that
no other Environment used its explicitly selected source image before deleting
1c0521930f3ac10dd5b9e61f236a7f61f8ebb5487a7b44aa4d7d9e75197f81af.
After positive image absence and catalog reload, Ensure reused exactly the same
asset and its Ubuntu rootfs remained readable. Exact asset deletion and private
catalog cleanup then succeeded. Source-image deletion was executed successfully
in this dedicated run; the shared-cache deletion variant remains disabled in GHA.
This does not yet establish ordinary-create wiring or snapshot restore.

An initial local compile/vet attempt failed on a missing BaseRevision conversion
in the new regression test. The type was corrected; focused tests and the real
fixture passed. The full local CI rerun remains pending.

When creation completion was durably recorded but verification/publication was
interrupted, the next retention request re-verifies the same owned material and
finishes ready publication. It issues no new create. A merely planned reservation
still requires recovery; resource presence alone does not prove completed creation.

## Automatic retention during Environment creation

Implemented in the local Incus composition: ordinary `haco create` and temporary
`haco run` retain the resolved Base before the Environment is initialized. No new
command, argument or opt-in is required. The resolved immutable source is passed
into the asset plan, including local effective revisions, without resolving a
moving alias again. The exact ready provider/scope/revision/binding is checked
before Environment creation continues.

The Base catalog owns failed retention independently. If retention fails before
Environment creation starts, creation returns unavailable and its unused
Workspace reservation can be released. Completed asset creation can finish
verification/publication on the next request; ambiguous planned creation remains
reserved for recovery. Deleting an Environment keeps its independent Base asset.
Reference-aware asset collection remains planned.

The existing ordinary-user Incus storage CLI E2E now asserts ready Base ownership
and isolated stopped material after create. Its disposable CI cleanup verifies
the exact unused catalog-owned asset and positive absence. New installed/GHA
execution is reported in implementation status. Snapshot planning/copy now uses
exact ready retained assets, with cached-image fallback only for absent catalog
entries. Restore remains planned.
