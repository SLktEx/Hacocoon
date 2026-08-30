# OCI Runtime and Docker Compatibility

Status: **v0.17 packaging foundation implemented; complete plugin integration and real-host acceptance remain pending.**

OCI/container tooling is an optional plugin/adapter concern, not a Hacocoon Core runtime requirement.

## Maintained plugin profile

The project-maintained OCI plugin profile may use:

```text
containerd
    ^
    |
nerdctl
```

This is the preferred maintained profile, not a Core invariant. With `HACO_PLUGIN_OCI` unset, Core must not require or probe for containerd, nerdctl, Docker CLI, or Docker Engine.

Docker compatibility is optional:

```text
Docker CLI / API client
        |
Environment-local docker.sock
        |
socket-activated dockerd
        |
existing containerd where supported
```

Use the genuine Docker CLI. Do not make `dockerd` always-on merely because some packages require the Engine API, and do not start a second Hacocoon-managed containerd solely for Docker compatibility.

## v0.17 plugin boundary

Docker/containerd/nerdctl-specific behavior belongs behind the optional plugin/adapter boundary under `modules/plugin/oci`. The current repository contains the Docker compatibility design and Hacocoon-specific systemd socket/service packaging foundation. This is **not yet a complete v0.17 feature**.

Rules:

- Docker compatibility remains Environment-local.
- Never mount the Host Docker socket or Host containerd/Incus/Hacocoon control sockets.
- Do not expose a TCP Docker API listener by default.
- Selecting `HACO_PLUGIN_OCI=docker` selects the plugin inventory/compatibility driver; it does not authorize an arbitrary Host Docker daemon.
- OCI plugin state and commands live under `haco plugin oci ...`; Base identity remains under `haco base ...`.

## Storage

Docker and nerdctl can use different containerd namespaces while sharing content-addressed blobs where supported, but complete byte deduplication is not guaranteed because snapshots, unpacked filesystems, writable layers, namespace metadata, and build caches differ.

Cross-Environment savings belong to **v0.19 OCI Seed Builder & Btrfs/COW**. The intended path is immutable Seed filesystem -> normal Incus/storage-driver clone -> Environment-private writable state. Never share one writable `/var/lib/containerd` among Environments.

## Registry

v0.18 Local OCI Registry is optional. Ordinary direct upstream pulls remain valid when network policy and credentials allow them; Seed construction does not require a Hacocoon-managed registry.

See [`17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.md`](17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.md) and [`19_v0.19_OCI_SEED_AND_COW.md`](19_v0.19_OCI_SEED_AND_COW.md).
