# Kubernetes Environment runtime

[**日本語**](kubernetes-runtime.ja.md) | English

Status: **partial experimental implementation on `main-kube`; not a supported replacement for the Incus path.**

## Summary

The `main-kube` experiment adds Kubernetes as an Environment provider without moving Hacocoon's trusted security boundary into Kubernetes workloads. Ordinary Environments may run as Sysbox-backed Pods while `haco-host`, Policy / Approval / Capability, Git push brokering, credentials, audit state, and the current recovery path remain trusted Host responsibilities.

The experiment exists to determine whether Kubernetes can replace most Environment lifecycle and network plumbing without weakening Hacocoon's brokered-authority model or losing whole-Environment copy semantics.

## Goals

- Reuse the existing provider-neutral Environment routing seam rather than adding Kubernetes conditionals to Core.
- Run an Environment with `systemd` as PID 1 through a Sysbox RuntimeClass.
- Permit Environment-local root while withholding Kubernetes control-plane authority and reusable Host credentials.
- Make Kubernetes networking default deny at Environment creation time.
- Fail closed on namespace ownership ambiguity, unsupported resource guarantees, or incomplete cleanup.
- Keep the existing Hacocoon Broker / Approval path authoritative for privileged external operations such as Git push.
- Preserve `haco-host` as trusted infrastructure while the Environment backend is evaluated independently.

## Non-goals of the current slice

The current repository slice does not claim:

- real Kubernetes + Sysbox host acceptance;
- safe multi-node Workspace placement;
- whole-Environment snapshot or clone support;
- Kubernetes-native Base / Seed support;
- OCI plugin compatibility on the Kubernetes Environment provider;
- SSH or local-port client adapters for Kubernetes Environments;
- production-ready outbound network policy;
- that a Kubernetes PVC snapshot alone satisfies Hacocoon's whole-Environment copy requirement.

These are acceptance or design gates, not implied follow-ups that may silently weaken the current security model.

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

`hostUsers: false` requests a Kubernetes Pod user namespace. The selected Sysbox RuntimeClass is expected to support the system-container behavior required by Hacocoon. A cluster that cannot provide these guarantees is not an accepted Hacocoon Kubernetes backend.

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

The long-term Kubernetes direction is to let the cluster networking layer own ordinary packet isolation, DNS, and approved direct egress where the selected CNI can enforce the required policy. Hacocoon should not retain a mandatory byte-forwarding proxy merely to duplicate CNI behavior.

This does **not** remove the privileged-operation Broker. Git push and other operations that require protected authority remain structured requests through Hacocoon Policy / Approval / Capability and execute with credentials only on the trusted side.

The current slice intentionally has no broad egress allow policy. A future egress implementation must define the exact CNI capability required before enabling direct outbound access and must fail closed when that capability is unavailable or drifted.

## Workspace transport

The first local experiment mounts the explicitly selected Workspace into the Pod with `hostPath`.

This is **not the final Kubernetes storage/security contract**. A writable `hostPath` gives the Pod authority over that exact host path and is not compatible with Kubernetes Pod Security Baseline/Restricted policy. It also creates node-placement coupling on a multi-node cluster.

The experiment therefore treats `hostPath` only as a local proof path. Before the Kubernetes backend can be considered supported, Workspace transport must either:

- move behind a storage mechanism with equivalent explicit lease/blast-radius semantics; or
- prove and enforce a local single-node placement model whose remaining `hostPath` authority is intentionally accepted and bounded.

Hacocoon must not mount Host HOME, credential stores, Kubernetes configuration, Hacocoon state, or runtime sockets as a convenience workaround.

## Whole-Environment copy gate

Whole-Environment copy is a required Hacocoon property, not an optional optimization.

The target semantic is conceptually:

```text
haco clone source target
```

where `target` begins from the source Environment's durable machine state, including changes under normal root filesystem paths and Environment-local runtime data, while receiving a fresh Hacocoon identity and no copied credentials, approvals, capability leases, or trusted control authority.

A Kubernetes PVC clone or `VolumeSnapshot` is insufficient by itself when relevant state still lives in the OCI writable root filesystem or Sysbox runtime-local data. The Kubernetes backend is therefore **not eligible to replace the Incus backend** until it demonstrates a storage layout and clone operation that captures the required Environment state atomically enough for Hacocoon's contract.

The preferred direction is to make durable Environment root state live on a snapshot-capable COW storage boundary and keep trust state outside it. The exact CSI/Btrfs implementation remains undecided in this branch.

## Resource guarantees

CPU, memory, and root-storage limits are projected into Kubernetes container limits in the initial experiment.

A finite per-Environment PID budget is rejected before cluster mutation because the portable Pod resource API does not provide the same per-Environment PID guarantee currently modeled by Hacocoon. The provider must not pretend that a node-wide kubelet setting is equivalent.

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

Any Kubernetes implementation that requires exposing the push credential to the Environment is incompatible with this design.

## `haco-host`

The experimental provider does not replace `haco-host`.

On `main-kube`, the existing local Incus trusted-host lifecycle remains the normal trusted logical Host and recovery path. This deliberately allows the Environment runtime question to be tested independently from a later decision about how `haco-host` itself should be hosted.

## Current configuration

The experimental provider is selected with:

```text
HACO_RUNTIME_PROVIDER=runtime.kubernetes
HACO_KUBERNETES_IMAGE=<systemd-capable-image>
```

Optional experiment settings are:

```text
HACO_KUBERNETES_RUNTIME_CLASS=sysbox-runc
HACO_KUBECTL=kubectl
```

The Kubernetes Environment provider currently rejects `HACO_PLUGIN_OCI` composition because that integration has not yet been verified on this backend.

## Acceptance gates before replacement

Do not describe Kubernetes as a replacement for Incus until all of these are demonstrated on the intended supported host:

1. real Kubernetes + Sysbox lifecycle with `systemd`, Environment-local root, and nested container-runtime workloads;
2. no ServiceAccount token, Host credential, Hacocoon control socket, or `haco-host` authority exposed to the Environment;
3. effective ingress/egress isolation and the selected CNI's fail-closed policy behavior;
4. brokered Git push regression coverage proving credentials remain outside the Environment and approvals cannot be reused for changed state;
5. whole-Environment snapshot/clone with fresh trust identity;
6. Workspace semantics without an accidental multi-node `hostPath` authority/placement bug;
7. cleanup, crash, retry, node restart, and drift failure injection;
8. performance measurements showing that ordinary network and large-file paths do not regress by reintroducing an unnecessary Hacocoon forwarding proxy.
