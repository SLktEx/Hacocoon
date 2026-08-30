# OCI Runtime and Docker Compatibility

Status: **v0.18 repository integration implemented ahead of roadmap order; real-host acceptance remains environment-dependent.**

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

Docker/containerd/nerdctl-specific behavior belongs behind the optional plugin/adapter boundary under `modules/plugin/oci`. The repository includes plugin-owned systemd socket/service packaging plus Environment lifecycle inspection/preparation:

```text
HACO_PLUGIN_OCI=docker haco plugin oci docker status <environment>
HACO_PLUGIN_OCI=docker haco plugin oci docker prepare <environment>
```

`status` does not start Docker. `prepare` requires a Base/Seed that already contains the genuine Docker CLI, dockerd, containerd, systemd, the docker group, and the Hacocoon-pinned unit files. It verifies the units before enabling socket activation, never installs packages, and refuses to silently stop an already-active vendor Docker daemon/socket.

The code originally landed while Docker Compatibility was numbered v0.17. The authoritative roadmap now assigns the feature to v0.18; no runtime behavior is rolled back by the renumbering.

Rules:

- Docker compatibility remains Environment-local.
- Never mount the Host Docker socket or Host containerd/Incus/Hacocoon control sockets.
- Do not expose a TCP Docker API listener by default.
- Selecting `HACO_PLUGIN_OCI=docker` selects the plugin inventory/compatibility driver; it does not authorize an arbitrary Host Docker daemon.
- `hacocoon-docker.service` may remain inactive until `/run/docker.sock` is used; that is the intended on-demand state.
- OCI plugin state and commands live under `haco plugin oci ...`; Base identity remains under `haco base ...`.

## Storage

Docker and nerdctl can use different containerd namespaces while sharing content-addressed blobs where supported, but complete byte deduplication is not guaranteed because snapshots, unpacked filesystems, writable layers, namespace metadata, and build caches differ.

Cross-Environment savings belong to **v0.17 OCI Seed Builder & Btrfs/COW**. The intended path is trusted Host acquisition/cache -> offline immutable Seed build -> normal Incus/storage-driver clone -> Environment-private writable state. Never share one writable `/var/lib/containerd` among Environments.

## Registry

A Local OCI Registry is deferred optional infrastructure and has no reserved roadmap milestone. Ordinary direct upstream pulls remain valid when network policy and credentials allow them; Seed construction does not require a Hacocoon-managed registry.

See [`design/oci-seed-and-cow.md`](design/oci-seed-and-cow.md), [`design/docker-compatibility-plugin.md`](design/docker-compatibility-plugin.md), and [`OPTIONAL_LOCAL_OCI_REGISTRY.md`](OPTIONAL_LOCAL_OCI_REGISTRY.md).
