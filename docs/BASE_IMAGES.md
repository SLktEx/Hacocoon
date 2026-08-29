# Base Image Architecture

Status: **detailed design companion for v0.11 Base Images & Custom Environments.** The minimum v0.11 roadmap/acceptance contract is defined by `11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md`; broader ideas in this document are not automatically required in the first v0.11 implementation slice.

This document defines how Hacocoon should represent reusable Environment starting points without leaking Incus-specific image mechanics into Core.

It is subordinate to `00_REBASELINE_AND_ROADMAP.md`, `00C_TERMINOLOGY_AND_BOUNDARIES.md`, `00B_SECURITY_ARCHITECTURE.md`, and the minimum v0.11 contract. If this document conflicts with those authoritative documents, they win.

## Goal

For the Incus Environment implementation, users need to be able to choose a standard image or a reusable custom image when creating an Environment.

Hacocoon exposes that concept as a **Base** rather than exposing Incus aliases, remotes, or fingerprints as its public architecture.

```text
Official Base
    |
    v
User Base (optional)
    |
    v
Project Setup
    |
    v
Environment
```

A running or existing Environment does not change its Base when a logical Base is updated. A different Base revision is used only by newly created Environments.

## Design principles

1. **Core talks about Bases, not Incus images.** Incus aliases, remotes, and fingerprints stay inside the Incus adapter except for explicit diagnostics or Incus-specific administration.
2. **Logical names are convenient; revisions are immutable.** `my-dev` may move to a newer revision, but an Environment records the exact revision it was created from.
3. **Recreate instead of mutating the root starting point.** Hacocoon does not replace the Base underneath an existing Environment.
4. **Image contents are untrusted.** Selecting or importing a custom Base must not expand host-side authority.
5. **Privileges are policy, not image metadata.** Devices, mounts, networking, credentials, capabilities, privileged-container settings, and external authority remain controlled by Hacocoon/Environment policy.
6. **Do not over-generalize early.** The first implementation may be Incus-specific internally while preserving a Base-shaped public/domain boundary.

## Identity model

A Base has a logical name and an immutable revision.

```text
my-dev
  |
  +-- sha256:aaa...   older revision
  +-- sha256:bbb...   current revision
```

Users normally select the logical name:

```text
haco create --base my-dev --workspace <path> <environment>
```

At Environment creation time Hacocoon resolves the logical name once and records the immutable revision.

Conceptually:

```yaml
base:
  name: my-dev
  revision: sha256:bbb...
  backend: incus
```

For Incus, the revision ultimately maps to an immutable image fingerprint. A mutable Incus alias must not be the final identity of an already-created Environment.

## Base types

### Official Base

An Official Base is maintained or recommended by Hacocoon, for example:

```text
haco/ubuntu-26.04
```

The concrete Incus source used to obtain it is an adapter detail and is not a compatibility promise.

### User Base

A User Base is a reusable custom starting point owned by the operator/user, for example:

```text
my-dev
go-dev
cuda-dev
```

A User Base may eventually be derived from an Official Base, another User Base, a supported archive, or an explicitly selected existing Incus image. Supporting a source does not imply that the source is trusted.

## Project Setup

Project-specific dependencies should remain separate from a reusable Base.

```text
Base
  common OS/runtime/tooling
        |
        v
Project Setup
  workspace-specific setup
        |
        v
Environment
```

The exact configuration schema is not frozen.

## Proposed CLI surface

These commands describe the intended interaction model. They are **not implementation claims**.

```text
haco image list
haco image inspect <name>
haco image build <name>
haco image remove <name>
haco create --base <name> --workspace <path> <environment>
```

The minimum v0.11 gate prioritizes selection, immutable revision pinning, persisted identity, list/inspect diagnostics, and safe reference semantics. Build/import/history/rollback/GC may follow once the lifecycle is safe.

## Base selection precedence

If Base selection is supported at multiple levels, resolution should be deterministic:

```text
CLI --base
    > project configuration
    > user/global default
    > Hacocoon default
```

The resolved immutable revision is recorded at creation time. Changing defaults later affects only future Environments.

## Updating a Base

Updating a logical Base creates a new immutable revision rather than rewriting an existing one.

```text
my-dev -> revision A -> Environment 1
my-dev -> revision B -> Environment 2
Environment 1 remains revision A
```

A future rollback should move the logical name to a previous valid revision rather than mutate revision contents.

## Import/build trust boundary

Local archives and custom build inputs are untrusted data.

At minimum, import/build handling must:

- validate supported formats;
- enforce practical size/resource limits;
- reject unsafe archive paths and traversal entries;
- avoid unexpected host symlink following;
- validate metadata instead of trusting archive-provided values;
- calculate content identity itself;
- avoid executing image contents directly with host authority;
- clean partial resources without deleting unrelated data.

Preferred build shape:

```text
Host
  |
  +-- Hacocoon / Incus authority
  |
  +-- isolated builder Environment
          |
          +-- execute build steps
          +-- produce image
          +-- register immutable Base revision
```

Host credentials are not implicitly injected.

## Security boundary

A custom Base may be malicious. Selecting a Base must never implicitly grant additional:

- host mounts;
- Incus devices;
- privileged-container mode;
- Linux capabilities;
- host network reachability;
- GitHub/AWS/cloud credentials;
- SSH private keys;
- registry credentials;
- Hacocoon control-plane authority.

A Base controls guest filesystem/runtime contents. It does **not** control host-side security policy.

## Lifecycle and deletion

Deleting a logical Base name and deleting physical revisions are separate operations.

```text
remove logical name
        |
        v
name no longer selectable
        |
        v
referenced revisions remain
        |
        v
unreferenced revisions may later be GC'd
```

A revision must not be physically removed while required by a running or recoverable Environment or another retention rule. When uncertain, cleanup should prefer retaining storage over silently destroying a dependency.

## Concurrency

Image lifecycle operations must assume concurrent clients and agents.

Relevant races include:

```text
build my-dev     vs build my-dev
remove my-dev    vs create --base my-dev
gc               vs create --base my-dev
update my-dev    vs create --base my-dev
```

Environment creation must acquire/record the immutable revision before that revision can become deletion/GC eligible. Logical-name updates and deletion should use lock-protected or transactional state transitions.

## Incus adapter mapping

Core/domain code should model Base identity without requiring Incus vocabulary, conceptually:

```go
type BaseName string
type BaseRevision string
```

The Incus adapter may internally resolve that to a fingerprint. Avoid introducing public `IncusImageAlias`-style types merely because Incus is the first Environment implementation.

## Relationship to the current roadmap

- **v0.9** supplies the implemented trusted per-agent Environment broker.
- **v0.10** is the active VS Code Remote Agent Host Adapter integration.
- **v0.11** is this Base Images & Custom Environments design gate.
- **v0.12** Resource Limits must compose with Bases without allowing image metadata to raise host-selected limits.

The v0.11 Base implementation must use the normal Environment lifecycle and must not bypass v0.9/v0.10 ownership/security boundaries.

## Initial v0.11 implementation slice

Prefer a narrow first slice:

1. resolve a configured Hacocoon Base to an existing Incus image;
2. allow `haco create --base <name>`;
3. persist the immutable resolved revision/fingerprint association;
4. support list/inspect sufficient for diagnosis;
5. make default selection deterministic;
6. add unit/integration tests for resolution, pinning, deletion safety, and races.

Custom build/import, revision history, rollback, and GC should follow only after reference tracking and failure semantics are reliable.

## Contract summary

> A Hacocoon Environment is created from one immutable Base revision. Updating a logical Base affects future Environment creation only.

> A Base defines guest filesystem/runtime contents; it does not grant host-side privileges, credentials, devices, mounts, or external authority.
