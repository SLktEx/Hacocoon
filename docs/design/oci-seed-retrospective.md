# OCI Seed Retrospective and Migration to Incus-native OCI

**English** | [日本語](oci-seed-retrospective.ja.md)

Status: **historical design record / migration decision**  
Decision date: **2026-08-31**

This document preserves why Hacocoon's OCI Seed mechanism existed and why the architecture is moving toward Incus-native OCI workloads. The Seed design should not be remembered merely as obsolete complexity: it solved real storage, isolation, reproducibility, credential, and recovery problems created by running `containerd + nerdctl` inside every Environment.

The detailed implementation remains documented in [OCI Seed Builder & Btrfs/COW Optimization](oci-seed-and-cow.md). This document records the rationale, the replacement architecture, and the design principles that must survive the migration.

## 1. What OCI Seed solved

The previous runtime model placed `containerd` and `nerdctl` inside each Hacocoon Environment.

```text
Physical Host
└─ Incus
   └─ Environment
      ├─ workspace
      ├─ systemd
      └─ containerd / nerdctl
         ├─ app
         ├─ db
         └─ redis
```

Each Environment needed independent writable containerd state so it could be modified, deleted, corrupted, and recovered without affecting another Environment. Sharing one writable `/var/lib/containerd` would reduce storage but break that isolation boundary.

OCI Seed therefore shared immutable filesystem blocks instead of writable runtime state. Common OCI content was imported into an offline Seed Builder, published as an immutable Incus Seed, and cloned through Incus/storage-driver semantics so Btrfs could share unchanged blocks through COW.

```text
upstream OCI registry
        |
        v
trusted acquisition/cache
        |
        v
OCI export / stream
        |
        v
Offline Seed Builder
        |
 containerd import/unpack
        |
        v
immutable Incus Seed
        |
        v
Incus clone / Btrfs COW
        |
   independent Environments
```

Seed therefore solved more than image caching:

- physical reuse of common OCI content across Environments;
- no shared writable containerd root;
- immutable digest tracking instead of relying on mutable tags;
- safe publication that did not advance `current` after partial or deletion-raced builds;
- no reusable registry credentials embedded in Seed Builders or coding Environments;
- conservative recovery and GC;
- COW optimization through supported Incus/storage lifecycle operations.

## 2. Why Seed became complex

The complexity came from a two-level runtime model:

```text
Incus lifecycle / storage / network
            |
            v
Environment filesystem
            |
            v
containerd lifecycle / image store / snapshotter / network
```

Hacocoon had to bridge those two worlds with acquisition, export/import, digest verification, builder lifecycle, publication, pinning, deletion, recovery, and GC.

Seed was a reasonable answer to the storage-sharing problem created by selecting `containerd/nerdctl` as the workload runtime inside each Environment. If Incus itself can natively manage OCI application containers, however, the extra runtime layer no longer needs to be the default architecture.

## 3. Runtime direction decided on 2026-08-31

Hacocoon should use **one Incus daemon**. Nested Incus is not part of this model.

```text
Physical Host
└─ Incus daemon
   ├─ Hacocoon Environment
   ├─ app      (OCI application container)
   ├─ db       (OCI application container)
   └─ redis    (OCI application container)
```

The Environment and its OCI workloads become sibling Incus instances. Hacocoon can group them through its Incus project/network policy and delegate instance networking, port exposure, storage, and lifecycle to Incus.

`haco-host` does **not** connect directly to Incus and does not receive the Incus socket.

```text
haco-host
   |
   | Hacocoon control request
   v
Host controller / broker
   |
   | authorized Incus operation
   v
Incus daemon
```

The goal is to make OCI `run/start/stop/exec/network/storage` part of the Incus instance model rather than part of an inner `nerdctl` runtime.

## 4. Private registry and ECR credential boundary

Registry authentication is resolved on the `haco-host` side. The Physical Host should not require the user's long-lived AWS credentials or AWS SSO session.

For ECR the intended conceptual flow is:

```text
haco-host
   |
   | AWS credential / SSO / credential_process
   v
obtain ECR authorization token
   |
   | temporary registry credential
   v
Hacocoon control socket
   |
   v
Host controller / broker
   |
   | authorize request
   | retain credential only for the pull operation
   v
Incus OCI pull
   |
   v
ECR
```

Required boundaries:

- `haco-host` never receives the Incus socket;
- the Physical Host controller does not persist the user's AWS access key or SSO session;
- credentials passed from `haco-host` should be scoped to registry use whenever possible;
- temporary credentials are discarded after the pull and are not persisted to disk;
- every Incus operation passes through Hacocoon controller/broker authorization.

If Incus credential-helper integration is used, Hacocoon should preserve these boundaries through an ephemeral Hacocoon-managed credential provider/helper rather than creating a persistent Host login store.

## 5. Responsibility mapping

```text
Old OCI Seed                        Incus-native OCI direction
──────────────────────────────────────────────────────────────
containerd runtime in Environment -> Incus OCI application instance
per-Environment image store       -> Incus-daemon-side OCI/image management
Seed Builder                      -> normally unnecessary
OCI export/import                 -> normally unnecessary
immutable Seed revision           -> Incus cache/image + immutable OCI identity
Seed clone COW                    -> Incus-native storage/image lifecycle
credential-free harvest           -> normally unnecessary with centralized runtime/cache
seed current/pin                  -> keep digest identity in Hacocoon policy where needed
seed deletion tombstone           -> preserve as image-selection policy if still useful
seed recover                      -> integrate into Incus operation / Hacocoon transaction recovery
seed GC                           -> Hacocoon policy that respects Incus lifecycle
```

The migration should remove the **reason Seed is necessary**, not re-create Seed under a different name.

## 6. Seed design principles that must survive

### Immutable identity

Mutable tags may remain convenient input, but reproducibility and policy decisions should resolve to immutable identities such as `sha256` digests.

### Never share writable runtime state

Storage savings must not come from sharing one writable runtime state between independent Environments or workloads. Every instance must remain independently mutable, deletable, and recoverable.

### Never bake credentials into image/storage state

Registry credentials, credential-helper output, AWS sessions, and login files must not be captured in images or snapshots. Credentials should exist only for the operation that needs them.

### Use supported Incus lifecycle operations

Even on Btrfs, Hacocoon Core should not directly manipulate Incus-owned subvolumes to implement COW or GC. Delegate to supported Incus image/instance/storage lifecycle APIs.

### Destructive GC fails closed

If ownership, dependencies, aliases, or usage are ambiguous, retain the resource. Safety takes precedence over storage optimization.

### Partial operations do not become current

Interrupted pull/publication/metadata/policy operations must not be treated as successful. Ambiguous state should enter an explicit recovery path.

## 7. Role of nerdctl/containerd after migration

Moving the default runtime to Incus-native OCI does not necessarily mean deleting `nerdctl/containerd` from every possible workflow immediately.

Docker/Compose compatibility, BuildKit-based image builds, or specialized containerd tooling can remain as an **optional compatibility/build backend**. They should not return as the center of Hacocoon's default runtime, image-sharing, network, or storage lifecycle.

```text
Hacocoon default runtime
        |
        v
      Incus
     /     \
system     OCI application
container  container

optional compatibility/build path
        |
        v
containerd / nerdctl / BuildKit
```

## 8. Migration acceptance

Do not fully retire Seed until real-host testing confirms at least:

- Incus OCI application containers satisfy Hacocoon's required run/exec/stop/delete semantics;
- multiple OCI instances and the Environment can be safely connected through managed networking;
- required Host/Environment port exposure works;
- ECR/private-registry pulls work safely using credentials obtained by `haco-host`;
- credentials do not remain on Host disk, in Incus images, or in instance root filesystems;
- image/cache duplication and physical Btrfs usage are acceptable compared with Seed;
- partial pulls and orphan resources can be reconciled after failure/restart;
- Environment boundaries remain enforceable through Incus project and controller policy.

Until these conditions are met, the existing Seed implementation and documentation must not be deleted.

## 9. Historical preservation rule

Keep `oci-seed-and-cow(.ja).md` as a historical architecture document even after Seed implementation is removed.

The answer to "why did Hacocoon ever have such a complicated Seed pipeline?" is:

> Hacocoon needed independent containerd runtimes inside each Environment while still reducing physical OCI image duplication and preserving credential isolation, Environment independence, and Btrfs/COW benefits.

The reason for moving to Incus-native OCI is:

> If one Incus daemon can natively manage both Hacocoon Environments and OCI workloads, integrating image/runtime/network/storage lifecycle in Incus is simpler and more coherent than maintaining a Hacocoon-specific bridge between two runtime layers.

Seed was not a failed design. It was a solution to the runtime boundary Hacocoon had at the time; it becomes a retirement candidate because that boundary can now be removed.
