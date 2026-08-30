# v0.17 — OCI Seed Builder & Btrfs/COW Optimization

Status: **first repository slice implemented / partial. v0.15 recommendation and v0.16 deletion policy are implemented prerequisites. Seed build/publish, exact-parent resolution, and explicit operator Seed pin/re-enable selection now exist; real-host and physical COW acceptance remain pending.**

v0.17 owns the physical OCI Seed pipeline: trusted Host-side image acquisition/cache, offline Seed construction, immutable publication, revision pinning, and storage-driver COW benefits.

A Local Registry is not required.

The implementation originally landed while this feature was numbered v0.18. The authoritative roadmap now assigns Seed Builder/COW to v0.17; historical commits and PRs may retain the earlier number.

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

## Implemented first slice

- `haco plugin oci seed build [--base <base>] [--json]`
- `haco plugin oci seed current [--base <base>] [--json]`
- `haco plugin oci seed pin <reference@sha256:...> [--re-enable] [--json]`
- `haco plugin oci seed unpin <reference@sha256:...> [--json]`
- `haco plugin oci seed re-enable <reference@sha256:...> [--json]`
- persisted operator Seed-selection state kept separate from usage telemetry
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

## Mandatory isolation rule

Hacocoon must never obtain storage savings by sharing one writable `/var/lib/containerd` across Environments.

Each Environment must remain independently mutable, deletable, corruptible, and recoverable.

## Inputs

- immutable parent Base revision from the v0.11 Base model;
- image identities selected by v0.15 recommendation/automatic-promotion policy plus explicit operator input;
- v0.16 deletion tombstones/overrides;
- trusted Host-side upstream credentials where authentication is required.

Mutable OCI tags are convenience input only. Seed manifests persist immutable digests. Explicit operator Seed selection requires the full immutable `reference@sha256:...` identity.

## Operator selection precedence

Operator pin/re-enable state is persisted separately from sampled usage state so an explicit selection decision cannot rewrite telemetry or erase deletion history.

- A normal `seed pin` forces an immutable identity into future Seed selection even when it falls outside the automatic top-10% promotion set. `pinned=true` remains distinct from `auto_promote=true`.
- A currently tombstoned identity cannot be pinned implicitly. The operator must use `seed re-enable` or `seed pin ... --re-enable`, and re-enable is valid only for an identity that already has a deletion tombstone.
- Re-enable does **not** delete the tombstone. It records a later operator decision. If a new deletion is recorded after that re-enable, the later deletion wins again and suppresses the identity even if an older pin still exists.
- `seed unpin` removes only the pin. It does not erase a re-enable decision or deletion history.
- Pin/re-enable changes only future Seed selection. It grants no registry, network, Host credential, provider, or Environment authority. Trusted Host acquisition and credential policy still apply independently, and reusable credentials must never be copied into the Seed Builder or Seed.

The implementation reads permissive selection state before destructive deletion state. A concurrent cross-file update can therefore only make one observation stricter: a new deletion is visible immediately, while a new re-enable may wait until the next read. This keeps the effective selection fail closed without pretending the two files form one atomic transaction.

## Build lifecycle

1. resolve the logical Base to an immutable Base revision;
2. resolve the effective Seed image set from OCI recommendations plus explicit operator selection;
3. acquire selected OCI content on the trusted Host and pin digests;
4. export/stream content into a temporary Seed Builder;
5. create the builder from the pinned Tooling Base with no general network access and no NIC;
6. import/unpack through supported containerd/nerdctl interfaces;
7. verify every requested digest;
8. stop containerd/Docker compatibility services cleanly;
9. stop the builder;
10. publish an immutable Incus Seed revision;
11. persist a manifest binding parent Base revision, Tooling revision, Seed revision, and OCI digests;
12. move the logical current-Seed pointer only after publication/validation succeeds;
13. surface recovery-required state when publication/state persistence becomes ambiguous.

## Plugin boundary

Seed observation/deletion/build/current and operator selection remain under `haco plugin oci ...`. The physical v0.17 builder/publisher stays outside Core behind the OCI/provider adapter boundary; it does not turn containerd, nerdctl, OCI manifests, Incus images, or Btrfs subvolumes into Core vocabulary.

## Btrfs/COW boundary

Hacocoon relies on Incus/storage-driver cloning. It must not directly manipulate Incus-owned Btrfs subvolumes from Core.

On Btrfs, unchanged Seed-derived blocks should be physically shared until copy-on-write. Non-COW backends may still use the same logical Seed feature but must not claim equivalent storage savings.

## Security requirements

- no shared writable containerd root;
- no Host containerd/Docker socket inside a coding Environment or Seed Builder;
- no Incus/Hacocoon control socket exposed to the builder workload;
- Seed Builder has no arbitrary upstream network path;
- networked acquisition happens on the trusted Host side;
- reusable upstream credentials are not embedded in the Seed;
- operator pin/re-enable does not grant acquisition/network/credential authority;
- a partial build never becomes current;
- cleanup ambiguity becomes recovery-required;
- old Seed GC prefers retention while an active/recoverable Environment may still depend on a revision.

## Remaining acceptance / follow-up

The first repository slice does not make v0.17 complete. Remaining work includes:

- real supported-host Incus + containerd + nerdctl acceptance;
- real Docker Engine compatibility acceptance for the Tooling Base path;
- conservative old Tooling/Seed revision GC and restart/crash recovery;
- authenticated/private-registry combinations without credential leakage;
- physical Btrfs COW/block-sharing measurement;
- broader failure-injection coverage around publish/cleanup/state persistence.

## Relationship to other milestones

- v0.15 selects/recommends image identities through the OCI plugin.
- v0.16 can tombstone/delete identities from future Seed selection through the OCI plugin.
- Optional Local Registry infrastructure is not a prerequisite and has no reserved milestone.
- v0.17 owns the actual immutable Seed build/publish/COW lifecycle, including explicit immutable operator pin/re-enable selection.
- v0.18 Docker Compatibility repository integration is implemented; its CLI/Engine compatibility remains optional and outside Core.
