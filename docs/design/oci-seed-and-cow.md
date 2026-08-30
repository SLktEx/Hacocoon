# v0.17 — OCI Seed Builder & Btrfs/COW Optimization

Status: **repository build/publish, operations-hardening, and credential-free managed-Environment harvest slices implemented / partial. v0.15 recommendation and v0.16 deletion policy are implemented prerequisites. Real-host, authenticated/private-registry combination, and physical COW acceptance remain pending.**

v0.17 owns the physical OCI Seed pipeline: trusted Host-side image acquisition/cache, offline Seed construction, immutable publication, revision pinning, storage-driver COW benefits, and conservative lifecycle maintenance.

A Local Registry is not required.

The implementation originally began landing while this feature was numbered v0.18. The authoritative roadmap now assigns Seed Builder/COW to v0.17; historical commits and PRs may retain the earlier number.

## Goal

Preload common OCI images into an immutable Incus-derived Seed so future Environments can reuse unchanged filesystem blocks through normal Incus/storage-driver clone semantics while keeping each Environment's writable containerd state independent.

```text
upstream OCI registry
        |
        | trusted Host acquisition/cache
        v
Host seed namespace/cache
        |
   OCI export / stream
        v
Offline Seed Builder
(no general network; no NIC)
        |
 containerd import/unpack
        |
 clean containerd stop
        v
immutable Incus Seed revision
        |
      Incus clone
   (Btrfs COW when available)
      /    |    \
   Env A Env B Env C
```

## Implemented first repository slice

- `haco plugin oci seed build [--base <base>] [--json]`
- `haco plugin oci seed current [--base <base>] [--json]`
- persisted Tooling Base and current Seed manifests with process-safe build locking
- immutable parent Base resolution before build
- trusted Host-side OCI acquisition through the dedicated `hacocoon-seed` namespace
- offline Seed Builder created with no profiles and no NIC
- import through supported nerdctl/containerd interfaces rather than copying a live containerd state directory
- verification of every selected immutable digest before publication
- clean service stop before publishing an immutable Incus image
- current-Seed pointer advancement only after publication and manifest persistence succeed
- exact-parent Seed resolution: a Seed is used only while its recorded parent Base revision still matches the currently resolved immutable parent
- Tooling Base support for containerd + nerdctl and genuine Docker CLI/Engine compatibility without forwarding a Host Docker/containerd socket

## Implemented operations-hardening slice

- `haco plugin oci seed pin <reference@sha256:...> [--base <base>] [--json]`
- `haco plugin oci seed unpin <reference@sha256:...> [--base <base>] [--json]`
- `haco plugin oci seed pins [--base <base>] [--json]`
- `haco plugin oci seed gc [--json]`
- `haco plugin oci seed recover [--json]`
- `haco plugin oci image reenable <reference@sha256:...> [--json]`
- persistent per-Base explicit immutable pins merged with automatic recommendations
- deletion tombstones override recommendations and existing pins until the exact immutable identity is explicitly re-enabled
- build-time deletion state is re-checked after Seed publication so a deletion racing a long build cannot silently advance the current pointer
- interrupted exact Hacocoon Seed/Tooling builders are reconciled before a new build while the process-safe Seed build lock is held
- conservative old Tooling/Seed image GC is scoped to the Hacocoon Incus project and Hacocoon-owned image aliases
- current manifest revisions/aliases, instance `volatile.base_image` dependencies, Incus `used_by` references, and externally aliased images are retained
- malformed Incus/provider inventory fails closed before destructive image deletion
- GC uses supported Incus image lifecycle operations and does not manipulate Incus-owned Btrfs subvolumes directly

## Implemented credential-free Environment harvest slice

When an exact immutable OCI identity is already present in a running Hacocoon-managed Environment, Seed acquisition can reuse that local content without copying registry credentials or requiring the trusted Host to authenticate to the registry first.

- newly created managed Incus Environments are marked with `user.hacocoon.kind=environment`;
- only running instances carrying that exact marker are eligible harvest sources;
- only exact `reference@sha256:...` Seed pulls are eligible for harvest;
- the Environment runs `nerdctl save` for that exact immutable identity into a random temporary archive under `/tmp`;
- the trusted Host copies only that OCI archive with `incus file pull`, immediately removes the temporary guest archive, and loads it into the dedicated `hacocoon-seed` namespace;
- the exact immutable identity is verified in the Host cache after load;
- unmarked/legacy Environments are not inspected for harvest;
- if no safe harvest succeeds, the existing trusted Host `nerdctl pull` path remains the fallback, including Host-owned registry credential handling.

The harvest path never copies registry login files, credential-helper output, workspace data, arbitrary Environment files, or a live `/var/lib/containerd` tree. The only transferred artifact is a temporary OCI image archive produced from already-local content.

## Mandatory isolation rule

Hacocoon must never obtain storage savings by sharing one writable `/var/lib/containerd` across Environments.

Each Environment must remain independently mutable, deletable, corruptible, and recoverable.

## Inputs

- immutable parent Base revision from the v0.11 Base model;
- image identities selected by v0.15 recommendation/automatic-promotion policy plus explicit operator pins;
- v0.16 deletion tombstones/overrides;
- exact immutable OCI content already present in a marked managed Environment where available;
- trusted Host-side upstream credentials where registry authentication is still required.

Mutable OCI tags are convenience input only. Seed manifests and explicit pins persist immutable digests.

## Build lifecycle

1. acquire the process-safe Seed build lock and reconcile interrupted Hacocoon builders when the provider supports maintenance;
2. resolve the logical Base to an immutable Base revision;
3. resolve the effective Seed image set from OCI recommendations plus explicit per-Base pins;
4. reject any selected immutable identity blocked by an OCI deletion tombstone;
5. acquire each exact immutable OCI identity into the trusted Host Seed cache: first attempt credential-free harvest from an eligible marked managed Environment, then fall back to trusted Host registry acquisition when needed;
6. export/stream content from the trusted Host cache into a temporary Seed Builder;
7. create the builder from the pinned Tooling Base with no general network access and no NIC;
8. import/unpack through supported containerd/nerdctl interfaces;
9. verify every requested digest;
10. stop containerd/Docker compatibility services cleanly;
11. stop the builder;
12. publish an immutable Incus Seed revision;
13. re-check deletion state for the selected immutable identities;
14. persist a manifest binding parent Base revision, Tooling revision, Seed revision, and OCI digests;
15. move the logical current-Seed pointer only after publication/validation succeeds;
16. surface recovery-required state when publication/state persistence or maintenance cleanup becomes ambiguous.

## Pin, deletion, and re-enable precedence

An explicit pin is an operator request to include one exact immutable OCI identity in future Seeds for a logical Base. It does not override an explicit deletion.

A v0.16 deletion tombstone wins over both automatic recommendation and an existing pin. A tombstoned identity must be removed with `haco plugin oci image reenable <reference@sha256:...>` before it can be pinned or selected again. Re-enable is exact-identity only so a mutable tag move cannot silently re-enable another digest.

## Recovery and GC

`haco plugin oci seed recover` reconciles exact Hacocoon temporary Seed/Tooling builders and then performs the same conservative image-retention analysis used by `seed gc`. `seed build` invokes interrupted-builder recovery before starting another build when the configured backend supports it.

An image is not deleted if Hacocoon cannot prove it is owned and unused. Current Seed/Tooling revisions, protected aliases, instance base-image fingerprints, Incus `used_by` references, and any external alias cause retention. Malformed inventory is a fail-closed error rather than permission to delete.

## Plugin boundary

Seed observation/deletion/build/current/pin/maintenance remain under `haco plugin oci ...`. The physical v0.17 builder/publisher stays outside Core behind the OCI/provider adapter boundary; it does not turn containerd, nerdctl, OCI manifests, Incus images, or Btrfs subvolumes into Core vocabulary. Incus-specific harvest mechanics remain in the Incus provider/runner boundary rather than the OCI plugin or Core.

## Btrfs/COW boundary

Hacocoon relies on Incus/storage-driver cloning. It must not directly manipulate Incus-owned Btrfs subvolumes from Core or from Seed GC.

On Btrfs, unchanged Seed-derived blocks should be physically shared until copy-on-write. Non-COW backends may still use the same logical Seed feature but must not claim equivalent storage savings.

## Security requirements

- no shared writable containerd root;
- no Host containerd/Docker socket inside a coding Environment or Seed Builder;
- no Incus/Hacocoon control socket exposed to the builder workload;
- Seed Builder has no arbitrary upstream network path;
- networked registry acquisition happens on the trusted Host side;
- credential-free Environment harvest is limited to exact immutable identities from explicitly marked running managed Environments;
- harvest transfers only a temporary OCI archive and never credential files, credential-helper output, workspace data, arbitrary Environment files, or live containerd state;
- reusable upstream credentials are not embedded in the Seed;
- explicit OCI identities reject option-like references and require immutable `sha256` digests;
- deletion tombstones have precedence over pins and recommendations;
- a partial or deletion-raced build never becomes current;
- cleanup ambiguity becomes recovery-required;
- old Seed GC prefers retention whenever ownership or dependency evidence is ambiguous.

## Remaining acceptance / follow-up

The repository slices do not make v0.17 complete. Remaining work includes:

- real supported-host Incus + containerd + nerdctl acceptance;
- real Docker Engine compatibility acceptance for the Tooling Base path;
- authenticated/private-registry combinations using Host-owned credentials without leaking credentials into the Seed Builder or coding Environments;
- physical Btrfs COW/block-sharing measurement;
- broader real-host failure-injection coverage around publication, restart, cleanup, harvest, and storage behavior;
- evaluate whether an Incus/Btrfs-backed trusted acquisition/cache materially improves cache -> builder -> Seed block reuse; keep it optional unless measurement justifies it.

## Relationship to other milestones

- v0.15 selects/recommends image identities through the OCI plugin.
- v0.16 can tombstone/delete identities from future Seed selection through the OCI plugin.
- Optional Local Registry infrastructure is not a prerequisite and has no reserved milestone.
- v0.17 owns the actual immutable Seed build/publish/COW lifecycle and its repository-side maintenance/acquisition semantics.
- v0.18 Docker Compatibility repository integration is implemented; its CLI/Engine compatibility remains optional and outside Core.
