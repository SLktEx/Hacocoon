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
| Environment lifecycle | create, inspect/status, exec, interactive shell, delete, exact ownership, collision refusal | partial unit-level implementation |
| `systemd` / sudo / root | PID 1 systemd and Environment-local root behavior without Host root authority | partial; real runtime untested |
| Workspace leases | canonical Workspace identity, RO/RW semantics, conflicting lease refusal, `/workspace` behavior | untested / incomplete |
| Whole-Environment copy | copy durable machine/root/runtime state with fresh trust identity and COW behavior | unimplemented; #322 |
| Resource budgets | CPU, memory, PID, root-storage semantics are enforced or explicitly rejected identically | partial; PID parity gap currently known |
| Client status/access | existing status, SSH, loopback TCP/forwarding, preparation/revocation behavior | untested / incomplete |
| `haco run` / ephemeral execution | same lifecycle, cleanup, lock/recovery behavior | untested |
| Base lifecycle | list/inspect/select/create-from-Base semantics | untested / incomplete |
| Policy / Approval / Capability | same fail-closed decisions, approval semantics, stale-state handling, audit | existing trusted implementation retained; Kube interaction untested |
| Git push Broker | no reusable write credential in Environment; exact repo/remote/ref/SHA binding; stale-state refusal | trusted Broker retained; end-to-end parity untested |
| Git fetch | trusted Host authority and private-repository behavior | untested |
| Network isolation | equivalent default isolation, DNS behavior, drift detection, no accidental bypass | partial manifest only; real CNI behavior untested |
| Domain-aware egress | same authorization semantics and destination protections without silently broadening access | untested / redesign required |
| OCI / nested runtime | Environment-local container runtime behavior and isolation | untested; current composition rejects OCI plugin |
| Docker compatibility | same Docker status/prepare behavior where supported | untested |
| Seed / image behavior | same Base/Seed semantics, credential separation, immutable identity and recovery | untested |
| Btrfs/COW storage | equivalent capacity behavior, compression intent, COW and recovery properties where observable | untested / storage design open |
| Trusted `haco-host` | same logical trusted Host behavior and isolation from Environment | retained on Incus during experiment; parity of an all-Kube form is not currently required |
| Notifications / interaction events | same client-visible event semantics and no approval authority in clients | existing implementation retained; integration untested |
| Structured logging | same operation fields, redaction, trust-boundary behavior | untested |
| Failure recovery | interrupted create/delete/run, ownership drift, stale state, cleanup-required semantics | partial unit coverage only |
| Ubuntu 26.04 | real substrate works on the project target | blocked/untested; #323 |

The matrix should become stricter as tests land. A feature may only move to `parity proven` after the relevant real or repository-level acceptance is strong enough to support that claim.

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
