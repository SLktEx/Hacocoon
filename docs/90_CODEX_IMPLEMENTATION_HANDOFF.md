# Codex Implementation Handoff

> Maintenance guide for the current pre-1.0 architecture.

Fully implemented product milestones are contiguous through **v0.16**. v0.17 Docker Compatibility Plugin is a partial foundation. v0.18 Optional Local OCI Registry and v0.19 OCI Seed Builder & Btrfs/COW are planned.

Use [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md) for current repository reality and [`00D_VERSIONING_AND_RELEASE_STATUS.md`](00D_VERSIONING_AND_RELEASE_STATUS.md) for numbering.

## Versioning rule

One independently useful product feature is approximately one minor milestone. Feature PRs assign the next `v0.N` and update status/versioning in the same PR. Fixes, hardening, refactors, CLI namespace cleanup, CI and docs normally do not consume another product version.

## Architecture placement

```text
Workspace identity / leases      -> Core
Environment lifecycle            -> Core contract + provider adapter
Incus lifecycle/networking       -> runtime.incus adapter
Client / VS Code                 -> client adapter
Per-agent binding                -> trusted integration outside Core
Base identity / haco base        -> provider-neutral contract
ResourceBudget                   -> provider-neutral contract
Policy / Approval / Audit        -> trusted boundary
Git push/fetch                    -> Git/GitHub plugin/capability boundary
OCI/container lifecycle          -> optional haco plugin oci boundary
Docker compatibility             -> optional OCI plugin, Environment-local
Local Registry / Seed mechanics  -> optional plugin/host infrastructure
Cloud provider implementation    -> deferred; v0.7 routing seam retained
```

## Required rules

- Do not give coding agents Hacocoon/Incus management authority.
- Do not expose reusable Host credentials or control sockets to arbitrary Environments.
- Keep privileged external operations behind Policy/Capability/plugin boundaries.
- Keep Base lifecycle separate from OCI workload images.
- With `HACO_PLUGIN_OCI` unset, Core must not require containerd, nerdctl, Docker, or a Registry.
- The maintained OCI plugin may use containerd + nerdctl; this is not a Core invariant.
- Docker compatibility uses genuine Docker CLI and optional/on-demand Engine; never mount the Host Docker socket.
- Local Registry is optional.
- Seed build uses trusted Host acquisition and an offline builder; never share one writable `/var/lib/containerd` between Environments.
- Managed network drift and unenforceable finite security/resource controls fail closed.
- Concrete EC2/AWS/EBS support is currently deferred; do not silently restore it while local/provider contracts are changing.

## Newer gates

### v0.13 — Managed Sandbox Network
Read `13_v0.13_MANAGED_SANDBOX_NETWORK.md`. Hacocoon owns/verifies the managed bridge, ACL substrate and profile and must not silently fall back to broad/default networking.

### v0.14 — Git Fetch Plugin
Read `14_v0.14_GIT_FETCH_PLUGIN.md`. `haco plugin git fetch` keeps reusable GitHub credentials on the Host and uses the trusted `gh auth git-credential` path.

### v0.15 — OCI Seed Recommendation
Read `15_v0.15_OCI_SEED_RECOMMENDATION.md`. Sampling/recommendation records immutable OCI identities and marks deterministic top-10% candidates; it does not publish a Seed.

### v0.16 — OCI Image Deletion
Read `16_v0.16_OCI_IMAGE_DELETION.md`. Deletion records an immutable-identity tombstone and exposes explicit optional all-Environment removal with recovery semantics.

### v0.17 — Docker Compatibility Plugin
Read `17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.md`. Foundation only; keep it optional and outside Core.

### v0.18 / v0.19
Local Registry is optional and planned. Seed Builder/COW is planned and must use immutable Seed publication rather than writable containerd sharing.

## Validation

Run the maintained local CI entry point where possible:

```bash
bash tools/ci-local.sh
```

Keep real Incus, Windows/WSL + VS Code, private-registry, Docker compatibility and future cloud-adapter acceptance distinct from repository tests.
