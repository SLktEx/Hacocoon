# OCI Runtime and Docker Compatibility

Status: **packaging foundation implemented; Base/Seed bake-in and real-host acceptance remain pending.**

This document defines how Hacocoon supports Docker-oriented developer tooling without making Docker Engine the canonical runtime.

## Decision

Hacocoon's canonical OCI runtime is:

```text
containerd  (always-on runtime/content service)
    ^
    |
nerdctl     (normal CLI)
```

Docker compatibility is an additional interface:

```text
Docker CLI / Docker API client
          |
          | unix:///var/run/docker.sock
          v
hacocoon-docker.socket
          |
          | socket activation
          v
       dockerd
          |
          | --containerd=/run/containerd/containerd.sock
          v
     same containerd
```

The genuine Docker CLI may be installed in Hacocoon development Bases/Seeds for ecosystem compatibility. `dockerd` may also be installed, but it must not be the default always-on runtime.

Normal Hacocoon documentation and automation should prefer `nerdctl`. Docker Engine exists for software that actually requires the Docker Engine API or `/var/run/docker.sock`.

## Runtime rules

1. `containerd` is the long-lived OCI runtime service.
2. `nerdctl` is the normal container CLI.
3. the genuine `docker` CLI is a compatibility tool, not a wrapper implemented by Hacocoon;
4. `dockerd` is stopped by default;
5. access to `/run/docker.sock` activates `hacocoon-docker.service` through systemd;
6. activated `dockerd` connects to the already-running `/run/containerd/containerd.sock`;
7. Hacocoon must not start a second private/managed containerd merely for Docker compatibility;
8. the Docker Engine API is exposed only through the Environment-local Unix socket by default;
9. Hacocoon must never mount a Host Docker socket into an Environment.

The repository ships unit templates under `packaging/systemd/`. They intentionally use Hacocoon-specific unit names so Base provisioning can disable vendor `docker.service` / `docker.socket` without replacing package-owned files.

## Socket activation

`hacocoon-docker.socket` owns `/run/docker.sock` and starts `hacocoon-docker.service` on first access.

The service is deliberately not installable into `multi-user.target`; only the socket is enabled. This prevents an ordinary boot from turning Docker Engine into a second always-running control plane.

Socket activation is an **on-demand start mechanism**, not an idle shutdown mechanism. A future lifecycle policy may stop an idle daemon, but Hacocoon must not claim that systemd automatically stops `dockerd` after clients disconnect.

Before enabling the Hacocoon socket, Base/Seed provisioning must ensure that no vendor `docker.socket` or other process is already listening on `/run/docker.sock`.

## containerd namespaces and storage

Docker Engine normally owns its own containerd namespace (commonly `moby`). Hacocoon/nerdctl metadata does not need to share that namespace merely to save image-content space.

When both clients use the same containerd daemon, content-addressed OCI blobs can be shared by the daemon's content store even when higher-level image/container metadata is isolated by namespace.

This does **not** mean every byte is guaranteed to be deduplicated. Namespace-specific metadata, snapshots/unpacked filesystem state, writable layers, build cache, and Docker-owned state can consume additional storage. Hacocoon should therefore describe this optimization as **shared containerd content**, not as zero-cost duplicate images.

For cross-Environment savings, the v0.13A Seed/COW design remains authoritative: immutable Seed filesystem state is cloned through Incus/storage-driver semantics. Multiple Environments must never share one writable `/var/lib/containerd`.

## Base and Seed integration

Docker compatibility belongs in immutable development Bases/Seeds, not in per-Environment startup package installation.

A Base/Seed integration that enables this feature must:

1. install a supported standalone `containerd` service and `nerdctl`;
2. install the genuine Docker CLI;
3. install `dockerd` only when Engine compatibility is part of that Base/Seed contract;
4. create the Environment-local `docker` group when non-root Docker API access is desired;
5. install the Hacocoon socket/service units;
6. disable vendor auto-started `docker.service` and `docker.socket` units;
7. enable `hacocoon-docker.socket`, not `hacocoon-docker.service`;
8. verify that `dockerd` is stopped before the immutable Base/Seed is published;
9. verify that `/run/containerd/containerd.sock` is the containerd endpoint used by the activated daemon;
10. verify that no registry credentials or Host control sockets are captured in the image.

The current repository does not yet have the v0.11 custom Base builder / v0.13A Seed publisher needed to bake these packages and units into official Hacocoon images automatically. The shipped units are therefore a packaging foundation, not a claim that `docker` already works in every Environment created from the current vanilla Ubuntu Bases.

## Security boundary

A Docker daemon is a high-authority service **inside its Environment**. Access to the Docker socket effectively grants control over that Environment's Docker daemon and the containers it manages.

Required rules:

- `/run/docker.sock` is Environment-local;
- no Host `/var/run/docker.sock` bind mount;
- no Host/containerd/Incus/Hacocoon control socket is forwarded through Docker compatibility;
- no TCP Docker API listener by default;
- socket mode is `0660` and group membership is explicit;
- membership in the Environment's `docker` group must be treated as root-equivalent authority within that Environment;
- Docker compatibility does not grant GitHub, cloud, registry, or Host credentials;
- the Incus Environment boundary remains the outer security boundary.

## Telemetry interaction

OCI Seed usage telemetry currently samples ordinary `nerdctl images` output. Once Docker Engine compatibility is baked into Environments, Seed telemetry must account for images used through the Docker/moby namespace as well, or collect usage through a lower-level containerd view.

When the same immutable digest is visible through both paths, telemetry should deduplicate it before calculating Seed recommendations. Docker compatibility must not cause the same OCI content to receive double recommendation weight.

## Acceptance

Repository-level acceptance for the packaging foundation requires:

- a socket unit listening on `/run/docker.sock` with `0660` permissions;
- a service unit using `dockerd -H fd://` and the external `/run/containerd/containerd.sock`;
- no boot target that directly enables `hacocoon-docker.service`;
- documentation that distinguishes shared content from total-storage deduplication;
- documentation that forbids Host Docker socket passthrough.

Real Base/Seed acceptance will additionally require a Hacocoon-built image where:

```text
boot
  containerd: active
  dockerd: inactive
  hacocoon-docker.socket: active

first Docker API request
  -> dockerd becomes active
  -> docker info succeeds
  -> ordinary nerdctl still works
```

Real-host acceptance remains pending until the Base/Seed build path exists and exercises this lifecycle on supported Incus images.

> **Docker is a compatibility interface. containerd + nerdctl remains the Hacocoon runtime.**
