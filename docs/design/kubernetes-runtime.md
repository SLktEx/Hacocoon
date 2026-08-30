# Kubernetes Environment runtime

[**日本語**](kubernetes-runtime.ja.md) | English

Status: **partial experimental implementation on `main-kube`; parity evaluation only. This branch is not intended to merge into `main`.**

## Summary

The `main-kube` experiment adds Kubernetes as an Environment provider without moving Hacocoon's trusted security boundary into Kubernetes workloads. Ordinary Environments may run as Sysbox-backed Pods while `haco-host`, Policy / Approval / Capability, Git push brokering, credentials, audit state, and the current recovery path remain trusted Host responsibilities.

The experiment exists only to determine whether Kubernetes can reproduce the complete current Hacocoon feature/security behavior and, if it can, whether the Hacocoon-owned implementation and operation are measurably simpler than the Incus baseline. It does not make a migration, adoption, or merge recommendation. See [`kubernetes-parity-experiment.md`](kubernetes-parity-experiment.md) for the evaluation contract.

## Goals

- Reuse the existing provider-neutral Environment routing seam rather than adding Kubernetes conditionals to Core.
- Run an Environment with `systemd` as PID 1 through a Sysbox RuntimeClass.
- Permit Environment-local root while withholding Kubernetes control-plane authority and reusable Host credentials.
- Make Kubernetes networking default deny at Environment creation time.
- Fail closed on namespace ownership ambiguity, unsupported resource guarantees, or incomplete cleanup.
- Keep the existing Hacocoon Broker / Approval path authoritative for privileged external operations such as Git push.
- Preserve `haco-host` as trusted infrastructure while Environment-runtime parity is evaluated independently.
- Measure whether Kubernetes actually removes Hacocoon-owned complexity instead of merely moving it into different glue code or mandatory platform dependencies.

## Non-goals of the current slice

The current repository slice does not claim:

- any intent to merge `main-kube` into `main`;
- any migration or replacement decision;
- real Kubernetes + Sysbox host acceptance;
- safe multi-node Workspace placement;
- whole-Environment snapshot or clone support;
- Kubernetes-native Base / Seed support;
- OCI plugin compatibility on the Kubernetes Environment provider;
- SSH or local-port client adapters for Kubernetes Environments;
- production-ready outbound network policy;
- that a Kubernetes PVC snapshot alone satisfies Hacocoon's whole-Environment copy requirement.

These are parity gaps or open experiment questions. The current Hacocoon requirement is not weakened merely because Kubernetes expresses the mechanism differently.

## Ownership and trust boundary

The intended split is:

```text
Physical Host / WSL                         TRUSTED
  |- Hacocoon controller
  |- Policy / Approval / Capability
  |- Git and external-operation Broker
  |- credentials and audit state
  |- Incus recovery / current haco-host lifecycle
  `- haco-host                              TRUSTED

Kubernetes cluster
  `- haco-<environment> namespace
       `- Sysbox Environment Pod            UNTRUSTED
            |- systemd
            |- Environment-local root
            `- /workspace
```

A Kubernetes Environment must not receive:

- a Kubernetes ServiceAccount token by default;
- Hacocoon controller or provider-control sockets;
- reusable GitHub or other Host credentials;
- `haco-host` filesystem or control authority;
- host PID, IPC, or network namespaces;
- privileged-container authority.

The current Pod manifest therefore sets `automountServiceAccountToken: false`, `hostUsers: false`, `hostNetwork: false`, `hostPID: false`, `hostIPC: false`, and `privileged: false`.

`hostUsers: false` requests a Kubernetes Pod user namespace. The selected Sysbox RuntimeClass is expected to support the system-container behavior required by Hacocoon. A cluster that cannot provide these guarantees has a runtime parity gap for this experiment.

## Identity and ownership

Each Environment currently owns one namespace named:

```text
haco-<environment-name>
```

Hacocoon writes and revalidates all of these labels before mutation:

```text
app.kubernetes.io/managed-by=hacocoon
hacocoon.dev/role=environment
hacocoon.dev/environment=<environment-name>
```

A colliding namespace without the exact expected identity is incompatible state. Hacocoon must not adopt, execute in, or delete it.

Name validation occurs before cluster mutation. Provider refs are not accepted merely because they have a `haco-` prefix; namespace ownership remains authoritative.

## Network model

The current experiment creates an explicit namespace-wide default-deny `NetworkPolicy` for both ingress and egress. Kubernetes otherwise permits traffic when no selecting NetworkPolicy isolates a Pod, so absence of a policy is not treated as a secure default.

For the experiment, ordinary packet isolation, DNS, and approved direct egress may be delegated to the cluster networking layer where the selected CNI can enforce behavior equivalent to the current Hacocoon contract. This is specifically tested to see whether Hacocoon-owned proxy/network plumbing can disappear without changing semantics.

This does **not** remove the privileged-operation Broker. Git push and other operations that require protected authority remain structured requests through Hacocoon Policy / Approval / Capability and execute with credentials only on the trusted side.

The current slice intentionally has no broad egress allow policy. A future experimental egress implementation must define the exact CNI capability required before enabling direct outbound access and must fail closed when that capability is unavailable or drifted. If equivalent domain-aware semantics cannot be reproduced, that is a parity failure.

## Workspace transport

The first local experiment mounts the explicitly selected Workspace into the Pod with `hostPath`.

This is **not a parity-complete Kubernetes storage/security contract**. A writable `hostPath` gives the Pod authority over that exact host path and is not compatible with Kubernetes Pod Security Baseline/Restricted policy. It also creates node-placement coupling on a multi-node cluster.

The experiment therefore treats `hostPath` only as a local proof path. Before Workspace parity can be claimed, the experiment must either:

- move behind a storage mechanism with equivalent explicit lease/blast-radius semantics; or
- prove and enforce a local single-node placement model whose remaining `hostPath` authority is intentionally accepted and behaviorally equivalent to the Incus baseline.

Hacocoon must not mount Host HOME, credential stores, Kubernetes configuration, Hacocoon state, or runtime sockets as a convenience workaround.

## Whole-Environment copy parity gate

Whole-Environment copy is a required Hacocoon property, not an optional optimization.

The target semantic is conceptually:

```text
haco clone source target
```

where `target` begins from the source Environment's durable machine state, including changes under normal root filesystem paths and Environment-local runtime data, while receiving a fresh Hacocoon identity and no copied credentials, approvals, capability leases, or trusted control authority.

A Kubernetes PVC clone or `VolumeSnapshot` is insufficient by itself when relevant state still lives in the OCI writable root filesystem or Sysbox runtime-local data. Until the experiment demonstrates a storage layout and clone operation that captures the required Environment state atomically enough for the same contract, whole-Environment copy remains an explicit parity failure.

The current investigation direction is to make durable Environment root state live on a snapshot-capable COW storage boundary and keep trust state outside it. The exact CSI/Btrfs implementation remains undecided in this branch. See GitHub issue #322.

## Resource guarantees

CPU, memory, and root-storage limits are projected into Kubernetes container limits in the initial experiment.

A finite per-Environment PID budget is rejected before cluster mutation because the portable Pod resource API does not provide the same per-Environment PID guarantee currently modeled by Hacocoon. The provider must not pretend that a node-wide kubelet setting is equivalent. Until an equivalent mechanism exists, finite PID budgets are a recorded parity gap.

## Failure, retry, and cleanup

Creation follows a fail-closed sequence:

1. validate Environment name, Workspace path, requested resources, and provider configuration;
2. inspect the target namespace;
3. refuse a collision unless exact Hacocoon ownership is proven;
4. create the owned namespace;
5. create default-deny networking and the Sysbox Pod;
6. wait for readiness;
7. verify `systemd` is PID 1;
8. verify writable Workspace access when RW was requested.

Failure after namespace creation triggers bounded cleanup using a cancellation-detached context. Cleanup revalidates ownership immediately before namespace deletion. If cleanup cannot be proven complete, the operation reports recovery-required state rather than claiming success.

Exec, shell, inspect, and delete likewise revalidate exact namespace ownership before operating.

## Broker and Git push invariant

Kubernetes does not become Hacocoon's authorization model for privileged external actions.

For Git push in particular:

- the Environment receives no reusable write credential;
- the Environment cannot authorize itself through Kubernetes RBAC;
- the request continues through Hacocoon Policy / Approval / Capability;
- approval must remain bound to the concrete repository/remote/ref/commit/operation state required by the Git broker contract;
- stale or mismatched state must fail closed;
- the trusted Broker constructs and executes the privileged operation and records audit evidence.

Any Kubernetes implementation that requires exposing the push credential to the Environment fails parity.

## `haco-host`

The Environment-runtime experiment keeps the existing `haco-host` implementation unchanged on Incus.

This is deliberate: the experiment isolates the question "can Kubernetes reproduce the untrusted Environment runtime with full parity and less Hacocoon-owned complexity?" from the separate question of how trusted `haco-host` infrastructure might be hosted. An all-Kubernetes `haco-host` is not required for this experiment unless it becomes necessary to reproduce an existing Environment-facing behavior.

## Current configuration

The experimental provider is selected with:

```text
HACO_RUNTIME_PROVIDER=runtime.kubernetes
HACO_KUBERNETES_IMAGE=<systemd-capable-image>
HACO_KUBERNETES_EXPERIMENTAL_HOSTPATH=1
```

`HACO_KUBERNETES_EXPERIMENTAL_HOSTPATH=1` is intentionally mandatory while Workspace transport uses writable `hostPath`. Omitting it fails closed instead of making the experimental transport look supported by default.

Optional experiment settings are:

```text
HACO_KUBERNETES_RUNTIME_CLASS=sysbox-runc
HACO_KUBECTL=kubectl
```

The Kubernetes Environment provider currently rejects `HACO_PLUGIN_OCI` composition because that integration has not yet been reproduced on this backend. That rejection is an explicit feature-parity gap, not an accepted product difference.

## Parity gates

Full parity cannot be claimed until all of these are demonstrated on the same target used by the current Hacocoon baseline:

1. real Kubernetes + selected system-container RuntimeClass lifecycle with `systemd`, Environment-local root, and nested container-runtime workloads;
2. no ServiceAccount token, Host credential, Hacocoon control socket, or `haco-host` authority exposed to the Environment;
3. ingress/egress behavior equivalent to the current isolation and authorization contract, with fail-closed drift behavior;
4. brokered Git push regression coverage proving credentials remain outside the Environment and approvals cannot be reused for changed state;
5. whole-Environment snapshot/clone with fresh trust identity and equivalent COW semantics;
6. Workspace identity, RO/RW lease, mount, and conflict semantics without accidental multi-node authority changes;
7. current client access, Base, OCI/Docker, Git fetch, interaction, logging, resource, run/recovery, and other user-visible/runtime features reproduced or explicitly marked as parity gaps;
8. cleanup, crash, retry, node restart, and drift failure injection;
9. measured complexity and performance comparison against the Incus baseline, including large-file paths.

Passing these gates still does **not** imply a merge into `main`. The result is only one of the factual classifications defined in [`kubernetes-parity-experiment.md`](kubernetes-parity-experiment.md).
