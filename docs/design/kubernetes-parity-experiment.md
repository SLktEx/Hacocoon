# Kubernetes parity experiment

[**日本語**](kubernetes-parity-experiment.ja.md) | English

Status: experimental branch-only evaluation.

## Scope

`main-kube` is a disposable experiment. It is **not intended to be merged into `main`**, and this work does not make a migration or replacement recommendation.

The experiment answers only two questions:

1. Can a Kubernetes-based implementation reproduce the current Hacocoon behavior and security invariants with full feature parity?
2. If full parity is possible, is the Hacocoon-owned implementation and operation materially simpler than the current Incus-based implementation?

A Kubernetes design does not pass by being more idiomatic, more fashionable, or easier to explain. It passes only if it reproduces the required behavior without weakening semantics, and its simplification is measured.

## Evaluation rule

The Incus implementation on `main` is the behavioral baseline.

When Kubernetes behaves differently, choose one of these outcomes:

- implement the missing behavior;
- record an explicit parity gap;
- record the Kubernetes approach as non-viable for that feature.

Do **not** change the Hacocoon requirement merely to make Kubernetes fit.

Security invariants count as features. A solution that implements the user-visible command but exposes credentials, weakens approval binding, broadens Host authority, loses fail-closed drift handling, or changes isolation semantics is not feature parity.

## Feature-parity matrix

The table is a working checklist. `untested` means no parity claim is made yet.

| Area | Baseline behavior to reproduce | `main-kube` status |
|---|---|---|
| Environment lifecycle | create, inspect/status, exec, interactive shell, delete, exact ownership, collision refusal | repository integration covered for create/exec/delete and ownership; inspect/shell unit-covered; real runtime untested |
| `systemd` / sudo / root | PID 1 systemd and Environment-local root behavior without Host root authority | manifest + PID 1 verification implemented; real runtime untested |
| Workspace leases | canonical Workspace identity, RO/RW semantics, conflicting lease refusal, `/workspace` behavior | Core/state parity covered through real provider routing; RW/RO rejection and RO/RO sharing tested before cluster mutation; real mount semantics untested |
| Whole-Environment copy | copy durable machine/root/runtime state with fresh trust identity and COW behavior | unimplemented; #322 |
| Resource budgets | CPU, memory, PID, root-storage semantics are enforced or explicitly rejected identically | finite CPU/memory/root values survive Core/provider routing; finite per-Environment PID remains an explicit parity gap |
| Client status/access | existing status, SSH, loopback TCP/forwarding, preparation/revocation behavior | durable loopback-only detached `kubectl port-forward` state, list/reconcile/remove, and SSH prepare/revoke are repository-tested; real Kubernetes behavior, delete-time cleanup and crash/reboot recovery remain unproven |
| `haco run` / ephemeral execution | same lifecycle, cleanup, lock/recovery behavior | repository integration proves create/exec/delete cleanup and durable marker removal through Kubernetes provider; crash/restart recovery real-host acceptance untested |
| Base lifecycle | list/inspect/select/create-from-Base semantics | untested / incomplete; current Kubernetes provider rejects explicit Base selection |
| Policy / Approval / Capability | same fail-closed decisions, approval semantics, stale-state handling, audit | existing trusted implementation retained unchanged; normal Kubernetes Environment push path now crosses the same capability service in repository integration |
| Git push Broker | no reusable write credential in Environment; exact repo/remote/ref/SHA binding; stale-state refusal | repository integration proves exact SHA/ref push binding, stale remote refusal and ambient GitHub/askpass state exclusion for a Kubernetes-backed Environment; real authenticated GitHub push untested |
| Git fetch | trusted Host authority and private-repository behavior | untested |
| Network isolation | equivalent default isolation, DNS behavior, drift detection, no accidental bypass | ingress/egress default-deny manifest and provider-routed source identity are unit-covered; real CNI enforcement, DNS and SNAT behavior untested |
| Domain-aware egress | same authorization semantics and destination protections without silently broadening access | Policy/Approval/Broker remains reusable and source identity is provider-neutral; proxy/listener transport remains unresolved; NetworkPolicy alone is not equivalent to current hostname approval + DNS pinning + SNI validation semantics |
| OCI / nested runtime | Environment-local container runtime behavior and isolation | untested; current composition rejects OCI plugin |
| Docker compatibility | same Docker status/prepare behavior where supported | untested |
| Seed / image behavior | same Base/Seed semantics, credential separation, immutable identity and recovery | untested |
| Btrfs/COW storage | equivalent capacity behavior, compression intent, COW and recovery properties where observable | untested / storage design open |
| Trusted `haco-host` | same logical trusted Host behavior and isolation from Environment | retained on Incus during experiment; parity of an all-Kube form is not currently required |
| Notifications / interaction events | same client-visible event semantics and no approval authority in clients | existing implementation retained; integration untested |
| Structured logging | same operation fields, redaction, trust-boundary behavior | existing Core logging reused for tested create/exec/delete paths; provider-specific coverage incomplete |
| Failure recovery | interrupted create/delete/run, ownership drift, stale state, cleanup-required semantics | provider cleanup/ownership fail-closed unit coverage plus normal `haco run` cleanup integration; client-forward PID reuse fails closed; crash/node-restart failure injection untested |
| Ubuntu 26.04 | real substrate works on the project target | blocked/untested; #323 |

The matrix should become stricter as tests land. A feature may only move to `parity proven` after the relevant real or repository-level acceptance is strong enough to support that claim.

## First findings

The experiment already separates several kinds of complexity that looked similar at the architecture-diagram level.

### Core behavior that carries over cheaply

Workspace lease ownership, RO/RW conflict rules, Environment metadata, ephemeral `haco run` lifecycle, cleanup markers, Policy / Approval / Capability, brokered Git authority, audit state, and client-neutral interaction semantics are not Incus features. They sit above the provider seam and can be reused with little or no Kubernetes-specific code.

Repository integration now exercises Workspace lease conflict behavior and normal ephemeral-run cleanup through the Kubernetes provider rather than only through fake Core runtimes. A conflicting Workspace request is rejected before Kubernetes is touched.

The same is now true for the security-sensitive Git push path. A Kubernetes-backed Environment is persisted through the normal Workspace service and then passed to the unchanged trusted Git Broker. Repository tests prove that the Broker still pushes the exact approved source SHA to the approved ref, fails stale if the GitHub remote identity changes after policy evaluation, and does not inherit ambient `GH_TOKEN`, `GITHUB_TOKEN`, or caller-controlled `GIT_ASKPASS` state. This does not yet replace real authenticated-GitHub acceptance, but it demonstrates that the runtime swap itself does not require weakening the Broker contract.

### Network plumbing can shrink, but the security Broker is not automatically removable

Kubernetes `NetworkPolicy` can plausibly replace a meaningful part of the Incus-specific bridge/ACL isolation machinery. `main-kube` now creates explicit default-deny policies and routes provider-trusted source identity through the Environment router rather than coupling egress identity directly to Incus.

However, the current domain-aware egress feature is stronger than a static packet allowlist. It combines Hacocoon Policy / Approval, Host-side DNS resolution and public-address validation, per-connection DNS pinning, HTTPS CONNECT/SNI validation, and audit. Standard `NetworkPolicy` does not reproduce those semantics by itself.

Therefore the parity experiment must distinguish:

- **Incus-specific network transport/proxy plumbing**, which Kubernetes/CNI may remove; from
- **Hacocoon's authorization/enforcement proxy semantics**, which remain required unless an alternative reproduces the same behavior with less machinery.

Deleting the proxy by broadening outbound access is a parity failure, not simplification.

### Client loopback access can be reproduced, but currently costs more Hacocoon machinery

A repository-level parity candidate now exists for the persistent/reconcilable client-connection contract. Instead of treating a foreground `kubectl port-forward` command as equivalent, the Kubernetes provider starts a detached loopback-only port-forward on the trusted side and writes a private durable record under `HACO_ROOT`. The record includes a random process token, PID, Linux `/proc` start-time identity, Environment ref and exact ports.

The provider can rediscover the connection after a fresh Provider instance is constructed, list it, revoke it, and refuse to signal a reused or identity-mismatched PID. The port-forward subprocess receives a deliberately minimized environment containing Kubernetes authority inputs such as `KUBECONFIG` but not ambient GitHub tokens, Git askpass state, or unrelated process credentials. SSH uses the same durable forwarding path and manages only a marker-scoped public key inside the Environment.

This proves the client contract is not fundamentally impossible on Kubernetes, but it is **not evidence of simplification**. Before this slice, the Kubernetes provider implementation measured 568 physical lines. After adding durable client forwarding and its process/state reconciliation, the provider implementation measures 1,226 physical lines. The Incus provider baseline is still larger at 3,382 lines, but Kubernetes still lacks major parity areas such as whole-Environment clone, Base/Seed, OCI/storage semantics, and real network/runtime acceptance. The line-count delta is therefore evidence to track, not a winner.

There are also remaining client-access acceptance gaps: real `kubectl port-forward` behavior, placement of the supervisor in the trusted `haco-host` boundary, explicit cleanup during Environment deletion, and restart/reboot recovery have not yet been proven. If making those exact requires more daemons or reconciliation state, that cost belongs in the Kubernetes result.

### Runtime/storage remain the largest parity risks

Whole-Environment COW clone, immutable Base identity, nested runtime state, Btrfs/storage behavior, and Ubuntu 26.04 system-container compatibility are still open. These are precisely the areas where Incus currently supplies high-level machine/container semantics rather than merely orchestration.

## Simplicity comparison

Feature parity is evaluated first. Simplicity is meaningful only after equivalent behavior exists.

Measure the Incus baseline and Kubernetes experiment using the same dimensions.

### Hacocoon-owned code

Count and compare code required specifically for runtime, networking, storage integration, bootstrap, reconciliation, and recovery:

- Go LOC;
- shell/PowerShell/Python LOC;
- provider-specific tests;
- E2E harness code.

Generated files, vendored dependencies, and upstream Kubernetes manifests that Hacocoon does not own should not be counted as if complexity disappeared. Record them separately as external operational dependency.

### Hacocoon-owned mechanisms

Count the number of custom mechanisms Hacocoon must understand, create, reconcile, and recover:

- daemons and helpers;
- proxies and forwarding layers;
- bridges / ACLs / custom network resources;
- storage pools / loop devices / mounts / snapshot glue;
- privileged helpers;
- locks and persistent reconciliation state;
- custom ownership markers and drift checks.

### External operational machinery

Also record complexity moved out of Hacocoon rather than removed:

- Kubernetes distribution / control plane;
- CNI;
- CSI or local storage driver;
- system-container RuntimeClass such as Sysbox;
- container runtime;
- required privileged node components;
- compatibility constraints with Ubuntu 26.04 / WSL.

A reduction in Hacocoon LOC with a large new mandatory platform stack is not automatically a simpler total system. Report both Hacocoon complexity and total operational complexity.

### Performance / UX

Where the implementations expose the same operation, compare at least:

- Environment cold start;
- repeated start/reuse where applicable;
- exec latency;
- interactive shell latency;
- network throughput/latency;
- large-file I/O, including roughly 100 GB-class workloads where practical;
- whole-Environment copy time and physical space amplification;
- cleanup time;
- CI/E2E runtime.

The experiment is allowed to conclude that Kubernetes is simpler but slower, faster but more complex, or unable to reach parity.

## Security parity

The following are non-negotiable parity requirements:

- Environment receives no reusable Host/GitHub write credential;
- Environment receives no Kubernetes control-plane authority merely because it runs on Kubernetes;
- raw provider/control sockets are not exposed to the Environment;
- Policy / Approval / Capability remains authoritative for privileged external operations;
- approved Git state cannot be swapped before execution without stale-state refusal;
- provider ownership ambiguity fails closed;
- network-policy drift or unsupported enforcement is not treated as secure success;
- cleanup uncertainty surfaces recovery-required state rather than silently adopting leftovers;
- trust state is not copied as part of Environment clone/snapshot operations.

## Experiment result

The final result should be a factual comparison, not a merge proposal.

Use one of these summaries:

- **full parity + materially simpler**;
- **full parity + roughly equal complexity**;
- **full parity + more complex**;
- **partial parity**, with exact missing behaviors;
- **not viable on the target substrate**.

For the simplicity conclusion, include measured code/mechanism/dependency differences rather than a subjective architecture preference.

## Related documents

- [`kubernetes-runtime.md`](kubernetes-runtime.md) — current experimental provider mechanics and security boundary
- [`../IMPLEMENTATION_STATUS.md`](../IMPLEMENTATION_STATUS.md) — current repository baseline
- GitHub issue #322 — whole-Environment clone parity
- GitHub issue #323 — Ubuntu 26.04 real-runtime and broad feature-parity measurement
