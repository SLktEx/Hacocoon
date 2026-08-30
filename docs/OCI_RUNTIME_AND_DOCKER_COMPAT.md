# OCI Runtime and Docker Compatibility

Status: **v0.17 packaging foundation implemented; complete plugin lifecycle, Base/Seed bake-in, and real-host acceptance remain pending.**

This document defines how Hacocoon supports Docker-oriented developer tooling without making Docker Engine the canonical runtime or a Core concept. The versioned feature contract is [`17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.md`](17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.md).

## Decision

Hacocoon's standard OCI runtime is:

```text
containerd  (always-on runtime/content service)
    ^
    |
nerdctl     (normal CLI)
```

Docker compatibility is optional:

```text
Docker CLI / Docker API client
          |
          | Environment-local /run/docker.sock
          v
hacocoon-docker.socket
          |
          | socket activation
          v
       dockerd
          |
          | --containerd=/run/containerd/containerd.sock
          v
     existing containerd
```

The genuine Docker CLI may be installed for ecosystem compatibility. `dockerd` may also be present where Engine API compatibility is required, but it must not become Hacocoon's default always-on runtime.

Normal documentation/automation should prefer `nerdctl`. Docker Engine exists only for software that actually requires Docker CLI/Engine API or `/var/run/docker.sock` semantics.

## Plugin boundary

Docker/nerdctl compatibility is a **plugin/adapter concern**, not Hacocoon Core.

Core continues to own provider-neutral Environment lifecycle/execution contracts. Docker-specific package installation, socket activation, daemon lifecycle, namespace handling, and compatibility checks belong in the optional integration layer and immutable Base/Seed provisioning.

## Runtime rules

1. `containerd` is the long-lived OCI runtime/content service.
2. `nerdctl` is the normal container CLI.
3. genuine `docker` CLI is a compatibility tool, not a Hacocoon wrapper.
4. `dockerd` is stopped by default.
5. access to `/run/docker.sock` may activate `hacocoon-docker.service` through systemd.
6. activated `dockerd` connects to the already-running `/run/containerd/containerd.sock` where the supported integration permits it.
7. do not start a second Hacocoon-managed containerd merely for Docker compatibility.
8. expose Docker Engine API only through the Environment-local Unix socket by default.
9. never mount a Host Docker/containerd socket into an Environment.

The repository ships initial unit templates under `packaging/systemd/`. They are the v0.17 **foundation**, not proof that the complete plugin works in every current Environment.

## Socket activation

`hacocoon-docker.socket` owns `/run/docker.sock` and can start `hacocoon-docker.service` on first access. The service itself is not intended to become a normal boot-time second control plane.

Socket activation is an on-demand start mechanism, not automatic idle shutdown. A future lifecycle policy may stop an idle daemon, but documentation must not claim systemd does that automatically after clients disconnect.

Before enabling the Hacocoon socket, provisioning must ensure no vendor `docker.socket` or other process already owns `/run/docker.sock`.

## containerd namespaces and storage

Docker Engine may use its own containerd namespace (commonly `moby`). Hacocoon/nerdctl metadata does not need to share that namespace merely to save content-store space.

Using one containerd daemon can allow content-addressed OCI blobs to be shared while higher-level metadata remains namespace-specific. This does **not** prove every byte is deduplicated: snapshots, writable layers, build cache, and Docker-owned state may consume additional space.

For cross-Environment savings, v0.19 is authoritative: immutable Seed filesystem state is cloned through Incus/storage-driver semantics. Multiple Environments must never share one writable `/var/lib/containerd`.

## Base and Seed integration

Docker compatibility should be provisioned into immutable development Bases/Seeds rather than installed ad hoc on every Environment startup.

A completed integration should:

1. install supported containerd + nerdctl;
2. install genuine Docker CLI;
3. install `dockerd` only when Engine compatibility is part of that Base/Seed contract;
4. configure Environment-local Docker group/socket policy where needed;
5. install Hacocoon socket/service units;
6. disable conflicting vendor auto-started Docker units;
7. enable the Hacocoon socket, not a permanent Hacocoon Docker service;
8. verify `dockerd` is stopped before an immutable Base/Seed is published;
9. verify the intended containerd endpoint;
10. verify no registry credentials or Host control sockets are captured in the image.

The repository does not yet have the complete Base build / v0.19 Seed publisher needed to bake this automatically into official images, so current unit packaging remains foundation-only.

## Security boundary

A Docker daemon is high-authority **inside its Environment**. Access to the Environment-local Docker socket is effectively root-equivalent for that daemon's scope.

Required rules:

- Environment-local Docker socket only;
- no Host Docker/containerd socket passthrough;
- no Incus/Hacocoon control socket forwarding;
- no TCP Docker API listener by default;
- socket/group membership is explicit;
- Docker compatibility grants no GitHub/cloud/registry/Host credentials;
- the Incus Environment remains the outer security boundary.

## Telemetry interaction

v0.15 OCI Seed telemetry currently samples ordinary `nerdctl images`. When Docker compatibility is fully integrated, telemetry must account for Docker/moby namespace usage without double-counting the same immutable OCI content.

v0.16 deletion/tombstone semantics remain based on exact immutable identity and must remain safe regardless of which compatible CLI surfaced the image.

## Acceptance

Repository-level foundation acceptance includes the unit definitions and static/systemd verification. Complete v0.17 acceptance still requires a supported Base/Environment proving:

```text
boot
  containerd: active
  dockerd: inactive
  hacocoon-docker.socket: active

first Docker API request
  -> dockerd starts on demand
  -> docker info succeeds
  -> ordinary nerdctl still works
```

Real-host acceptance remains pending.

> **Docker is an optional compatibility plugin. containerd + nerdctl remains the standard Hacocoon runtime direction.**
