# OCI Runtime and Docker Compatibility

Status: **v0.18 packaging foundation implemented early; complete plugin integration and real-host acceptance remain pending after v0.17 Seed/Base provisioning.**

OCI/container tooling is an optional plugin/adapter concern, not a Hacocoon Core runtime requirement.

## Maintained plugin profile

The project-maintained OCI plugin profile may use **containerd + nerdctl**:

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

## v0.18 plugin boundary

Docker/containerd/nerdctl-specific behavior belongs behind the optional plugin/adapter boundary under `modules/plugin/oci`. The current repository contains the Docker compatibility design and Hacocoon-specific systemd socket/service packaging foundation. This is **not yet a complete v0.18 feature**.

Rules:

- Docker compatibility remains Environment-local.
- Never mount the Host Docker socket or Host containerd/Incus/Hacocoon control sockets.
- Do not expose a TCP Docker API listener by default.
- Selecting `HACO_PLUGIN_OCI=docker` selects the plugin inventory/compatibility driver; it does not authorize an arbitrary Host Docker daemon.
- OCI plugin state and commands live under `haco plugin oci ...`; Base identity remains under `haco base ...`.

## Storage

Docker and nerdctl can use different containerd namespaces while sharing content-addressed blobs where supported, but complete byte deduplication is not guaranteed because snapshots, unpacked filesystems, writable layers, namespace metadata, and build caches differ.

Cross-Environment savings belong to **v0.17 OCI Seed Builder & Btrfs/COW**. The intended path is trusted Host acquisition/cache -> offline immutable Seed build -> normal Incus/storage-driver clone -> Environment-private writable state. Never share one writable `/var/lib/containerd` among Environments.

The v0.17 Seed/Base provisioning path is also the prerequisite for completing Environment-local Docker provisioning in v0.18.

## Registry

A Local OCI Registry is deferred optional infrastructure and has no reserved roadmap milestone. Ordinary direct upstream pulls remain valid when network policy and credentials allow them; Seed construction does not require a Hacocoon-managed registry.

See [`17_v0.17_OCI_SEED_AND_COW.md`](17_v0.17_OCI_SEED_AND_COW.md), [`18_v0.18_DOCKER_COMPATIBILITY_PLUGIN.md`](18_v0.18_DOCKER_COMPATIBILITY_PLUGIN.md), and [`OPTIONAL_LOCAL_OCI_REGISTRY.md`](OPTIONAL_LOCAL_OCI_REGISTRY.md).
