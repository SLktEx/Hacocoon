# v0.11 — Base Images & Custom Environments

Status: **first implementation slice present on `main`; real-Incus acceptance and richer Base lifecycle remain pending.**

This document defines the v0.11 Base-image contract. `BASE_IMAGES.md` remains the broader design companion. `IMPLEMENTATION_STATUS.md` is authoritative for current code reality.

## Goal

The product CLI now exposes `haco base list`, `haco base inspect <base>`,
`haco env create --base <base> --workspace managed:<workspace> <environment>`
and `haco env switch-base --base <base> <environment>`. Switch retains every
managed repository volume, including uncommitted, untracked and unpushed work.
It discards the Environment root filesystem and connections; reconnect Git
and SSH afterwards. It uses canonical stop/delete/create transitions and
fails closed at each step. See [ADR 0011](../adr/0011-managed-workspace-base-switch.md).
Real-host acceptance is recorded in [implementation status](../IMPLEMENTATION_STATUS.md).

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
