# Optional OCI Plugin and Docker Compatibility

Status: **OCI plugin boundary and driver selection implemented; Base/Seed bake-in and real-host acceptance remain pending.**

This document defines an optional developer-tooling plugin. It does **not** define a mandatory Hacocoon Core runtime.

## Core decision

Hacocoon Core has no canonical OCI runtime and does not require `containerd`, `nerdctl`, Docker CLI, or Docker Engine.

A valid Hacocoon installation may run without any of those tools. Core still owns Environment lifecycle, isolation, execution, connection management, policy/approval boundaries, and events.

Container tooling belongs to `modules/plugin/oci` and is enabled explicitly:

```text
HACO_PLUGIN_OCI=nerdctl
HACO_PLUGIN_OCI=docker
```

If `HACO_PLUGIN_OCI` is unset, the OCI plugin is not composed and Core must not probe for or require container tooling.

Plugin commands live under:

```text
haco plugin oci status
haco plugin oci seed sample
haco plugin oci seed recommend
```

The `haco base` command remains a Core command because it describes Hacocoon Environment Bases, not OCI workload images.

## Project-maintained OCI profile

For users who opt into the project-maintained OCI workflow, the preferred profile is:

```text
containerd  (runtime/content service inside the Environment)
    ^
    |
nerdctl     (normal CLI for this profile)
```

That is a **plugin/profile choice**, not a Hacocoon Core invariant. A different Base/Seed may choose another OCI stack, or no OCI stack at all.

The genuine Docker CLI may be provided by the same plugin profile for ecosystem compatibility. Software that actually requires the Docker Engine API can use the optional socket-activated compatibility path:

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
          | --containerd=/run/containerd/containerd.sock
          v
     same containerd
```

The unit templates are plugin-owned and live under `modules/plugin/oci/packaging/systemd/`.

## Plugin driver selection

`HACO_PLUGIN_OCI` currently selects how the optional plugin inventories OCI images for Seed telemetry:

- `nerdctl`: run `nerdctl images ...` inside each Environment;
- `docker`: run the genuine `docker images --digests ...` CLI inside each Environment.

Driver selection does not grant credentials and does not imply that Hacocoon Core installed the selected binary. The Base/Seed or operator must provide the requested tool.

## Docker compatibility rules

When the OCI plugin's Docker Engine compatibility profile is used:

1. `containerd` is the long-lived OCI service for that profile;
2. the genuine `docker` CLI is used, not a Hacocoon wrapper;
3. `dockerd` is stopped by default;
4. access to `/run/docker.sock` activates `hacocoon-docker.service` through systemd;
5. activated `dockerd` connects to `/run/containerd/containerd.sock`;
6. the profile must not start a second private containerd merely for Docker compatibility;
7. the Docker API is Environment-local by default;
8. a Host Docker socket must never be mounted into an Environment.

The service itself is deliberately not enabled by a boot target; only the socket is enabled. Socket activation starts the daemon on demand but does not by itself stop an idle daemon later.

Before enabling the plugin socket, provisioning must ensure that a vendor `docker.socket` or another process is not already listening on `/run/docker.sock`.

## containerd namespaces and storage

When the project-maintained profile uses Docker Engine with the same containerd daemon, Docker may use a separate namespace such as `moby`. Higher-level metadata can remain separate while content-addressed OCI blobs are shared by containerd's content store.

This does **not** guarantee zero duplicate storage. Namespace metadata, unpacked snapshots, writable layers, build cache, and Docker-owned state may consume additional space.

Cross-Environment savings remain a Seed/COW concern: immutable Seed filesystem state may be cloned through the Environment/storage implementation, but multiple Environments must never share one writable `/var/lib/containerd`.

## Base / Seed integration

An OCI-enabled Base/Seed decides which plugin profile it provides. Hacocoon Core does not install these packages during Environment startup.

A project-maintained `nerdctl` profile may install:

- standalone `containerd`;
- `nerdctl`;
- optional genuine Docker CLI for command compatibility.

A Docker Engine compatibility profile may additionally install:

- `dockerd`;
- an Environment-local `docker` group when non-root API access is desired;
- `modules/plugin/oci/packaging/systemd/hacocoon-docker.socket`;
- `modules/plugin/oci/packaging/systemd/hacocoon-docker.service`.

Provisioning must disable conflicting vendor Docker auto-start, enable only the Hacocoon plugin socket, verify `dockerd` is stopped before publishing an immutable Base/Seed, and never capture registry credentials or Host control sockets in the image.

## Security boundary

A Docker daemon is high-authority **inside its Environment**. Access to the Docker socket effectively grants control over the daemon and its managed containers in that Environment.

Required rules:

- `/run/docker.sock` is Environment-local;
- no Host `/var/run/docker.sock` bind mount;
- no Host containerd, Incus, or Hacocoon control socket passthrough;
- no TCP Docker API listener by default;
- socket mode is `0660` and group membership is explicit;
- Environment `docker` group membership is treated as root-equivalent within that Environment;
- OCI plugin enablement does not grant GitHub, cloud, registry, or Host credentials;
- the Hacocoon Environment remains the outer security boundary.

## Telemetry and Seed recommendations

OCI Seed usage telemetry is plugin-owned. The selected driver samples workload-image usage and stores plugin state separately from Core Environment state.

The same immutable digest must be deduplicated before recommendation weighting. Using both Docker-oriented and nerdctl-oriented workflows must not double-count identical OCI content.

## Acceptance

Core acceptance must include a configuration with `HACO_PLUGIN_OCI` unset and no `nerdctl`, Docker, or containerd dependency introduced by composition.

OCI plugin acceptance additionally covers both explicit drivers:

```text
HACO_PLUGIN_OCI=nerdctl
  -> haco plugin oci status reports nerdctl
  -> seed sampling invokes nerdctl only

HACO_PLUGIN_OCI=docker
  -> haco plugin oci status reports docker
  -> seed sampling invokes Docker CLI only
```

Docker Engine compatibility acceptance for a plugin-enabled Base/Seed remains:

```text
boot
  containerd: active
  dockerd: inactive
  hacocoon-docker.socket: active

first Docker API request
  -> dockerd becomes active
  -> docker info succeeds
```

Real-host acceptance remains pending until the Base/Seed build path exercises this lifecycle on supported Environment images.

> **OCI tooling is optional. `containerd + nerdctl` is a project-maintained plugin profile, not a Hacocoon Core requirement.**
