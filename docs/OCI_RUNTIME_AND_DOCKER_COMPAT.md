# Optional OCI Plugin and Docker Compatibility

Status: **plugin boundary/driver composition implemented; v0.17 Docker Engine/Base integration and real-host acceptance remain pending.**

See [`17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.md`](17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.md) for the Docker milestone contract and [`19_v0.19_OCI_SEED_AND_COW.md`](19_v0.19_OCI_SEED_AND_COW.md) for cross-Environment Seed/COW storage optimization.

## Core decision

Hacocoon Core has no canonical OCI runtime and does not require `containerd`, `nerdctl`, Docker CLI, or Docker Engine.

A valid Core installation may run without any OCI tooling. Core still owns Environment lifecycle, isolation, execution, connection management, policy/approval boundaries, and events.

OCI/container-specific behavior belongs to `modules/plugin/oci` and is explicitly enabled:

```text
HACO_PLUGIN_OCI=nerdctl
HACO_PLUGIN_OCI=docker
```

If `HACO_PLUGIN_OCI` is unset, the OCI plugin is not composed and Core must not probe for or require container tooling.

## Project-maintained OCI profile

For users who opt into the project-maintained OCI workflow, the preferred profile is:

```text
containerd  (Environment-local runtime/content service)
    ^
    |
nerdctl     (normal CLI for this profile)
```

This is a **profile choice**, not a Core invariant. Another Base/Seed may provide a different OCI stack or no OCI stack at all.

Docker compatibility is optional and additive for profiles that need it:

```text
Docker CLI / Docker API client
          |
          | unix:///run/docker.sock
          v
hacocoon-docker.socket
          |
          | socket activation
          v
       dockerd
          |
          | existing Environment-local containerd where supported
          v
     containerd
```

## Plugin driver selection

`HACO_PLUGIN_OCI` selects the optional plugin's OCI inventory driver:

- `nerdctl`: inventory using `nerdctl images ...` inside managed Environments;
- `docker`: inventory using the genuine Docker CLI.

Driver selection does not install the binary, grant registry credentials, or authorize an arbitrary Host Docker daemon.

Base-image lifecycle and OCI workload-image lifecycle remain separate:

```text
haco base ...                    Hacocoon Environment starting points
haco plugin oci ...              optional OCI/container operations
```

## v0.15 / v0.16 plugin ownership

OCI Seed telemetry/recommendation and OCI deletion/tombstone state are plugin-owned under `modules/plugin/oci`.

- v0.15 decides which immutable OCI identities are candidates for future Seed inclusion;
- v0.16 performs explicit deletion/tombstone behavior;
- v0.19 owns physical immutable Seed build/publication/GC and COW integration.

With the plugin disabled, those optional OCI commands are unavailable while Core Environment lifecycle remains valid.

## v0.17 Docker compatibility rules

When a Base/Seed opts into Docker Engine compatibility:

1. use the genuine Docker CLI rather than a Hacocoon imitation;
2. keep `dockerd` stopped unless the Engine API is needed;
3. use an Environment-local `/run/docker.sock`;
4. prefer socket activation/on-demand startup where supported;
5. do not start a second Hacocoon-managed containerd solely for compatibility;
6. never mount a Host Docker socket into an Environment;
7. never turn Docker compatibility into GitHub/cloud/registry/Host authority.

The plugin-owned unit templates live under:

```text
modules/plugin/oci/packaging/systemd/
```

The service is not enabled as a normal boot target; the socket provides on-demand start. Socket activation does not imply automatic idle shutdown.

## Security boundary

A Docker daemon is high-authority **inside its Environment**. Access to that Environment's Docker socket is effectively root-equivalent for Docker-managed workloads there.

Required rules:

- `/run/docker.sock` is Environment-local;
- no Host Docker/containerd/Incus/Hacocoon control socket passthrough;
- no TCP Docker API listener by default;
- socket/group membership is explicit;
- OCI plugin enablement does not grant reusable Host credentials;
- the Hacocoon Environment remains the outer security boundary.

## containerd namespaces and storage

For the project-maintained profile, Docker and nerdctl may use different containerd namespaces while sharing content-addressed blobs in one daemon's content store where supported. This can reduce duplicate content but does **not** promise zero duplicate storage: metadata, snapshots, unpacked filesystems, writable layers, and build caches may still differ.

Cross-Environment storage savings belong to v0.19. Hacocoon must never save space by sharing one writable `/var/lib/containerd` across Environments.

## Acceptance

Core acceptance must include the plugin-disabled case:

```text
HACO_PLUGIN_OCI unset
containerd absent
nerdctl absent
Docker absent

-> Core Environment lifecycle still composes and operates
```

Plugin acceptance covers explicit driver selection. Full v0.17 acceptance additionally requires a supported Base/Seed proving an Environment-local on-demand Docker Engine lifecycle.

> **OCI tooling is optional. `containerd + nerdctl` is a project-maintained profile, not a Hacocoon Core requirement.**
