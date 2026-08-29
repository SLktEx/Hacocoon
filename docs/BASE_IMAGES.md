# Base Image Architecture

Status: **design proposal; not yet an implementation or roadmap commitment.**

This document defines how Hacocoon should represent reusable Environment starting points without leaking Incus-specific image mechanics into Core.

It is subordinate to `00_REBASELINE_AND_ROADMAP.md`, `00C_TERMINOLOGY_AND_BOUNDARIES.md`, and `00B_SECURITY_ARCHITECTURE.md`. If this document conflicts with those authoritative documents, they win.

## Goal

For the Incus Environment implementation, users need to be able to choose a standard image or a reusable custom image when creating an Environment.

Hacocoon should expose that concept as a **Base** rather than exposing Incus aliases, remotes, or fingerprints as its public architecture.

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

The layers have different responsibilities:

- **Official Base** — a Hacocoon-maintained or recommended starting point.
- **User Base** — a reusable user-defined starting point derived from an allowed source.
- **Project Setup** — repository/workspace-specific setup applied after the Base is selected.
- **Environment** — the isolated runtime created from one immutable Base revision.

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

Example persisted concept:

```yaml
base:
  name: my-dev
  revision: sha256:bbb...
  backend: incus
```

For the Incus implementation, that revision ultimately maps to an immutable Incus image fingerprint. An Incus alias must not be the final identity of an already-created Environment because aliases are mutable.

```text
Hacocoon Base name
        |
        v
immutable Base revision
        |
        v
Incus image fingerprint
```

## Base types

### Official Base

An Official Base is maintained or recommended by Hacocoon.

Example logical name:

```text
haco/ubuntu-26.04
```

The concrete Incus source used to obtain that Base is an adapter/detail concern and is not a compatibility promise.

### User Base

A User Base is a reusable custom starting point owned by the operator/user.

Examples:

```text
my-dev
go-dev
cuda-dev
```

A User Base may eventually be created from an Official Base, another User Base, a supported imported archive, or an explicitly selected existing Incus image. Supporting a source does not imply that the source is trusted.

## Project Setup

Project-specific dependencies should not require rebuilding a Base for every repository revision.

Conceptually:

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
  agent/developer execution
```

A future project configuration may select a Base and a setup action, for example:

```yaml
base: my-dev
setup:
  script: .hacocoon/setup.sh
```

The exact configuration schema is intentionally not frozen by this proposal.

## Proposed CLI surface

The following commands describe the intended interaction model. They are **not implementation claims** and remain pre-1.0 design surface.

```text
haco image list
haco image inspect <name>
haco image build <name>
haco image remove <name>
haco create --base <name> --workspace <path> <environment>
```

Possible examples:

```text
haco image list
haco image inspect my-dev
haco image build my-dev
haco create --base my-dev --workspace . dev-1
```

Normal output should use Hacocoon concepts. Backend-native identifiers may appear only in an explicit diagnostic section or backend-specific administrative command.

## Base selection precedence

If Base selection is supported at multiple levels, resolution should be deterministic:

```text
CLI --base
    > project configuration
    > user/global default
    > Hacocoon default
```

The resolved immutable revision is recorded when the Environment is created.

Changing any default later affects only future Environment creation.

## Updating a Base

Updating a logical Base creates a new immutable revision rather than rewriting an existing revision.

```text
my-dev
  revision A
  revision B
  revision C  <- current
```

Existing Environments remain attached to the revision they recorded. New Environments resolve `my-dev` to revision C.

A future rollback operation should move the logical name to a previous valid revision rather than mutate revision contents.

## Importing an existing Incus image

Incus interoperability may justify an explicitly backend-specific administrative command, for example:

```text
haco image import-incus <incus-image> --as my-dev
```

Such a command should resolve the Incus source to an immutable fingerprint and register that fingerprint as a Hacocoon Base revision.

After registration, normal Environment creation uses the Hacocoon name rather than the Incus alias.

The exact command name is not frozen.

## Importing local image files

If local archive import is added, the input must be treated as hostile.

At minimum, the implementation should:

- validate supported formats before import;
- enforce practical size/resource limits;
- reject unsafe archive paths and traversal entries;
- avoid following unexpected host symlinks;
- validate metadata rather than trusting archive-provided values;
- calculate content identity itself;
- avoid executing image contents directly on the host;
- clean up partial imports after failure without deleting unrelated data.

Import is a data-ingestion operation, not a trust grant.

## Build isolation

Building a custom Base can execute arbitrary commands. Those commands must not run directly with Hacocoon host authority.

Preferred shape:

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

The builder receives the minimum resources needed for the build. Host credentials are not implicitly injected.

## Security boundary

A custom Base may be malicious and may contain hostile services, modified binaries, credential collectors, or deliberately malformed filesystem state.

Selecting a Base must never implicitly grant additional:

- host mounts;
- Incus devices;
- privileged-container mode;
- Linux capabilities;
- host network reachability;
- GitHub/AWS/cloud credentials;
- SSH private keys;
- registry credentials;
- Hacocoon daemon authority.

Those privileges remain governed outside the Base by the Environment/Capability/Policy boundary.

A Base controls guest filesystem contents. It does **not** control the host-side security policy.

## Credentials and snapshots

Reusable credentials must not be baked into a Base.

Any future feature that creates a Base from an existing Environment must account for runtime secrets, mounted credentials, shell history, caches, generated tokens, and other sensitive state before snapshotting.

Blindly snapshotting an arbitrary Environment into a reusable Base is therefore out of scope for the first implementation.

## Lifecycle and deletion

Deleting a logical Base name and deleting its physical revisions are separate operations.

```text
remove logical name
        |
        v
name is no longer selectable
        |
        v
referenced revisions remain
        |
        v
unreferenced revisions may later be garbage-collected
```

A revision must not be physically removed while required by a protected reference, including a running or recoverable Environment or another retention rule.

When uncertain, cleanup should prefer leaking storage over silently destroying a referenced Environment dependency.

## Garbage collection

Old revisions consume storage, but an old revision can still be required for reproducibility or recovery.

A future GC operation may look like:

```text
haco image gc --dry-run
haco image gc
```

GC eligibility must be derived from explicit reference tracking, not merely from whether a revision is the current logical target.

## Failure states

Image operations can fail because of download interruption, disk exhaustion, failed build commands, Incus restart, invalid metadata, duplicate names, or process crashes.

An implementation should use explicit states such as:

```text
creating
downloading
building
ready
failed
deleting
```

Only `ready` revisions are selectable for new Environments. Partial resources must remain identifiable enough for safe cleanup.

## Concurrency

Image lifecycle operations must assume concurrent clients and agents.

Relevant races include:

```text
build my-dev     vs build my-dev
remove my-dev    vs create --base my-dev
gc               vs create --base my-dev
update my-dev    vs create --base my-dev
```

Environment creation must acquire/record the immutable revision before that revision can become GC-eligible. Logical-name updates and deletion should use lock-protected or transactional state transitions rather than timing assumptions.

## Incus adapter mapping

Core/domain code should model Base identity without requiring Incus vocabulary, for example conceptually:

```go
type BaseName string
type BaseRevision string
```

The Incus adapter may internally resolve that to something like:

```go
type IncusImageRef struct {
    Fingerprint string
}
```

Avoid introducing public types such as `IncusImageAlias` into Core merely because Incus is the first Environment implementation.

## Future Environment implementations

This design deliberately does not require every Environment implementation to use an Incus-style image.

A future adapter could map the same high-level Base concept to a backend-native immutable starting point, for example an AMI or another runtime image/profile. Such adapters are not committed by this document; they only explain why Core should not equate Base with an Incus image.

## Initial implementation slice

When this design is scheduled for implementation, prefer a narrow first slice:

1. resolve a configured Hacocoon Base to an existing Incus image;
2. allow `haco create --base <name>`;
3. persist the immutable resolved revision/fingerprint association;
4. support list/inspect sufficient for diagnosis;
5. make default selection deterministic;
6. add unit/integration tests for resolution, pinning, deletion safety, and races.

Custom build/import, revision history, rollback, and GC should follow only after reference tracking and failure semantics are reliable.

## Non-goals for the first slice

- replacing the Base of an existing Environment;
- transparent migration of existing Environments to a newer image;
- executing custom build steps on the host;
- snapshotting arbitrary live Environments into reusable Bases;
- exposing all Incus image functionality through Hacocoon;
- freezing the proposed CLI or configuration schema before pre-1.0 validation.

## Contract summary

The central contract is:

> A Hacocoon Environment is created from one immutable Base revision. Updating a logical Base affects future Environment creation only.

And the security contract is:

> A Base defines guest filesystem/runtime contents; it does not grant host-side privileges, credentials, devices, mounts, or external authority.

This gives Hacocoon selectable standard/custom starting environments while keeping Incus implementation details behind the Environment boundary and preserving room for later adapters.