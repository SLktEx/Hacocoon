# Base Image Architecture

Status: **v0.11 first slice implemented; this document also describes future Base lifecycle work.** The minimum current contract is [`design/base-images-and-custom-environments.md`](design/base-images-and-custom-environments.md), and `IMPLEMENTATION_STATUS.md` is authoritative for code reality.

This document describes reusable Environment starting points without leaking Incus-specific image mechanics into Core.

## Goal

Hacocoon exposes reusable starting points as **Bases** rather than exposing Incus aliases, remotes, or fingerprints as Core architecture.

```text
logical Base name
        |
 provider-owned source
        |
 resolve once at create
        v
immutable Base revision
        |
        v
Environment
```

A running or existing Environment does not change its Base when a logical Base moves.

## Current identity model

Core uses provider-neutral values:

```text
BaseName
BaseRevision
BaseRef{Name, Revision}
```

Current commands:

```text
haco base list [--json]
haco base inspect <base> [--json]
haco create --base <base> --workspace <path> <environment>
```

`haco base` is deliberately reserved for Hacocoon Environment starting points. OCI/container images belong to the separate optional `haco plugin oci ...` namespace.

At Environment creation time Hacocoon resolves the selected logical name once and persists the immutable `BaseRef` on the Environment.

For Incus, the adapter resolves its configured source with `incus image info`, validates the returned full fingerprint, initializes from the pinned fingerprint, and exposes only the provider-neutral revision to Core.

A mutable alias is therefore never the final identity of an already-created Environment.

## Official and custom Bases

The current Incus adapter provides logical official Bases including:

```text
haco/ubuntu-26.04
haco/ubuntu-24.04
```

Host/operator custom mappings can currently be supplied through `HACO_INCUS_BASES_JSON`, for example:

```json
{"my-dev":"images:my-moving-alias"}
```

The `haco/` namespace is reserved and custom mappings cannot override it.

This environment variable is an Incus adapter configuration detail and is not a frozen pre-1.0 public schema.

## Immutable revision rule

```text
my-dev -> revision A -> Environment 1
my-dev -> revision B -> Environment 2
Environment 1 remains revision A
```

Changing a logical Base affects future Environment creation only. Hacocoon does not replace the Base underneath an existing Environment.

## Security boundary

A custom Base is untrusted guest content. Selecting one must never implicitly grant additional:

- host mounts;
- Incus devices;
- privileged-container mode;
- Linux capabilities;
- host network authority;
- GitHub/AWS/cloud credentials;
- SSH private keys;
- registry credentials;
- Hacocoon/Incus control-plane authority.

```text
Base = guest contents
Policy / Capability = authority
```

The current mapping parser rejects malformed logical names, control characters, leading-option-shaped sources, reserved namespace overrides, oversized configuration, and malformed fingerprints. Source values are passed as argv values rather than shell-interpolated strings.

## Project Setup

Repository-specific setup remains separate from a reusable Base.

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

The exact Project Setup schema is not frozen.

## Future build/import work

Custom Base build/import is **not implemented in the first v0.11 slice**.

When added, build/import inputs must be treated as hostile data/code. The implementation must consider archive traversal, unsafe symlinks, malformed metadata, resource exhaustion, partial-import cleanup, and accidental credential capture.

Preferred future shape:

```text
Host
  |
  +-- Hacocoon / Incus authority
  |
  +-- isolated builder Environment
          |
          +-- build/import processing
          +-- produce immutable image
          +-- register Base revision
```

Host credentials must not be injected implicitly.

## Future history, rollback, deletion, and GC

These operations are also **not implemented in the first slice**.

Logical-name removal and physical revision deletion must remain separate:

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
provably unreferenced revisions may later be GC'd
```

A revision must not be physically removed while a running or recoverable Environment depends on it. If safety cannot be proven, Hacocoon should retain storage rather than destroy a dependency.

Future lifecycle code must deliberately handle races such as:

```text
create vs update
create vs remove
gc     vs create
build  vs build
```

Environment creation must obtain a stable immutable revision before that revision can become deletion/GC eligible.

## Selection precedence

Only explicit CLI selection and the Hacocoon default are implemented in the current first slice. If project/user defaults are added later, precedence should remain deterministic:

```text
CLI --base
    > project configuration
    > user/global default
    > Hacocoon default
```

Omission and explicit selection must remain diagnosable.

## Provider boundary

Incus is the first/default implementation, but Core does not require Incus aliases, remotes, fingerprints, or publish/import mechanics.

A future provider may map the same Hacocoon Base concepts to a different immutable starting-point mechanism without changing Core vocabulary.

## Acceptance split

Repository CI covers the current first slice with unit/adversarial tests and fake-Incus E2E for list, inspect, explicit selection, alias-to-fingerprint resolution, pinned initialization, and persisted revision identity.

Real Incus image-remote/custom-image acceptance remains host-dependent. Build/import/history/rollback/deletion/GC acceptance remains future work because those APIs are not exposed yet.

> **A Base chooses guest contents. Its immutable revision anchors reproducibility. It never grants host-side authority.**
