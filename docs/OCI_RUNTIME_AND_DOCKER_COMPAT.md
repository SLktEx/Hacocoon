# OCI Runtime and Docker Compatibility

Status: **v0.17 packaging foundation implemented; complete plugin integration and real-host acceptance remain pending.**

See [`17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.md`](17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.md) for the milestone contract and [`19_v0.19_OCI_SEED_AND_COW.md`](19_v0.19_OCI_SEED_AND_COW.md) for cross-Environment Seed/COW storage optimization.

## Decision

Hacocoon's standard OCI runtime inside development Environments is:

```text
containerd  (long-lived runtime/content service)
    ^
    |
nerdctl     (normal CLI)
```

Docker compatibility is optional and additive:

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
          | existing containerd where supported
          v
     containerd
```

The genuine Docker CLI may be present for ecosystem compatibility. `dockerd` must not become the canonical or always-on Hacocoon runtime merely because some packages expect the Docker Engine API.

## v0.17 plugin boundary

Docker/containerd/nerdctl-specific lifecycle belongs outside Hacocoon Core behind the plugin/adapter boundary.

Required direction:

1. `containerd` remains the standard long-lived OCI service.
2. `nerdctl` remains the normal container CLI.
3. use the genuine `docker` CLI rather than a Hacocoon imitation;
4. keep `dockerd` stopped unless Docker Engine compatibility is actually needed;
5. use Environment-local socket activation/on-demand startup where supported;
6. do not start a second Hacocoon-managed containerd merely for Docker compatibility;
7. never mount the Host Docker socket into an Environment;
8. never turn Docker compatibility into GitHub/cloud/registry/Host authority.

The repository currently contains the design plus Hacocoon-specific systemd socket/service packaging foundation. That is intentionally classified as a **partial v0.17 feature**, not a complete plugin implementation.

## Socket activation

`hacocoon-docker.socket` owns the Environment-local `/run/docker.sock` and activates `hacocoon-docker.service` when a client actually requests the Docker Engine API.

The service itself should not be enabled as a normal boot target. Socket activation is an on-demand **start** mechanism; it does not imply automatic idle shutdown after clients disconnect.

Before enabling the Hacocoon socket, provisioning must ensure vendor `docker.socket` or another process is not already listening on the same path.

## Security boundary

A Docker daemon is high-authority **inside its Environment**. Access to that Environment's Docker socket is effectively root-equivalent for Docker-managed workloads there.

Required rules:

- `/run/docker.sock` is Environment-local;
- no Host `/var/run/docker.sock` bind mount;
- no Host/containerd/Incus/Hacocoon control socket passthrough;
- no TCP Docker API listener by default;
- socket/group membership is explicit;
- Docker compatibility does not provide reusable Host credentials;
- the Incus Environment remains the outer security boundary.

## containerd namespaces and storage

Docker and nerdctl may use different containerd namespaces while still sharing content-addressed blobs in one daemon's content store. This can reduce duplicate content, but it is **not** a promise of zero duplicate storage: namespace metadata, snapshots, unpacked filesystems, writable layers, and build caches may still differ.

Hacocoon should therefore describe this as **shared containerd content where supported**, not total image-store deduplication.

## v0.19 cross-Environment storage

Cross-Environment storage savings belong to **v0.19 OCI Seed Builder & Btrfs/COW**, not to Docker compatibility itself.

Required shape:

```text
immutable Seed filesystem
        |
   Incus/storage clone
        +--------+--------+
        |        |        |
 independent   independent
 containerd    containerd
 state A       state B
```

Hacocoon must never save space by sharing one writable `/var/lib/containerd` across Environments.

When Btrfs is the active Incus storage backend, unchanged Seed-derived blocks may be shared through normal copy-on-write semantics. Hacocoon Core does not directly manage hidden Btrfs subvolume paths.

## OCI plugin interaction

Base-image lifecycle and OCI/container-image lifecycle are separate CLI surfaces:

```text
haco base ...                    Hacocoon Environment starting points
haco plugin oci ...              OCI/containerd/nerdctl operations
```

v0.15 recommendation currently samples OCI usage through the optional OCI plugin. Once Docker Engine compatibility is fully integrated, telemetry should also account for Docker/moby-namespace image usage without double-counting the same immutable digest.

## Completion criteria

v0.17 should not be called complete until supported-host integration proves at least:

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

The packaging foundation exists today; full plugin lifecycle, image/Base integration, and real-host validation remain follow-up work.

> **Docker is an optional compatibility plugin. containerd + nerdctl remains the standard Hacocoon OCI runtime.**
