# Optional OCI Plugin and Docker Compatibility

Status: **optional-plugin packaging foundation implemented; Base/Seed bake-in and real-host acceptance remain pending.**

This document defines the project-maintained OCI plugin profiles. It does **not** define an OCI runtime dependency for Hacocoon Core.

## Core rule

Hacocoon Core manages Workspaces and isolated Environments without requiring a container CLI/runtime inside those Environments. With `HACO_PLUGIN_OCI` unset, Hacocoon must not require nerdctl, Docker CLI, dockerd, a Host OCI cache, or a Local Registry.

Operators who want OCI developer tooling may explicitly select a profile:

```text
HACO_PLUGIN_OCI=nerdctl
HACO_PLUGIN_OCI=docker
```

## Project-maintained nerdctl profile

The preferred lightweight OCI profile for deployments that want container tooling is:

```text
containerd  (Environment-local service/content store)
    ^
    |
nerdctl     (ordinary CLI)
```

This is a plugin profile, not a Hacocoon Core runtime contract.

## Docker compatibility profile

Some developer tools require genuine Docker CLI or Docker Engine API semantics. The optional Docker profile can provide:

```text
Docker CLI / Docker API client
          |
          | unix:///var/run/docker.sock
          v
plugin-owned hacocoon-docker.socket
          |
          | socket activation
          v
       dockerd
          |
          | --containerd=/run/containerd/containerd.sock
          v
 Environment-local containerd
```

Rules:

1. use the genuine Docker CLI, not a Hacocoon wrapper;
2. keep `dockerd` optional and normally inactive until Docker API access needs it;
3. do not start a second private containerd merely for Docker compatibility;
4. expose Docker API only through the Environment-local Unix socket by default;
5. never mount the Host Docker socket into a Hacocoon Environment;
6. never treat Docker/nerdctl credentials as Core credentials.

The units live under:

```text
modules/plugin/oci/packaging/systemd/
```

They deliberately use Hacocoon-specific unit names rather than replacing vendor `docker.service` / `docker.socket`.

## Socket activation

`hacocoon-docker.socket` listens on `/run/docker.sock`; the first request activates `hacocoon-docker.service`. Socket activation does not by itself stop dockerd again when idle, so idle shutdown is a possible future policy rather than a current claim.

Before enabling the plugin socket, provisioning must ensure another service is not already listening on the same path.

## containerd namespaces and storage

Docker Engine commonly uses its own containerd namespace while nerdctl may use another. Sharing one containerd content service can share content-addressed blobs without requiring high-level metadata to use one namespace.

This does **not** guarantee zero duplicate bytes: namespace metadata, unpacked snapshots, writable layers, build cache and Docker-specific state can add storage.

Cross-Environment storage optimization belongs to planned v0.19 Seed/COW. Never share one writable `/var/lib/containerd` between Environments.

## Base / Seed integration

The intended deployment shape is to bake selected OCI tooling into immutable development Base/Seed images rather than package-installing it at every Environment start. That build/publish pipeline is not yet complete.

A Docker-compatible Base/Seed should:

- install supported containerd/nerdctl only if that optional profile is desired;
- install genuine Docker CLI where desired;
- install dockerd only where Engine compatibility is desired;
- install the plugin-owned socket/service units;
- disable conflicting vendor auto-start;
- enable only the plugin socket, not an always-on Hacocoon dockerd service;
- ensure dockerd is stopped before immutable publish;
- never bake registry credentials or Host control sockets into the image.

## Security boundary

Docker socket access is root-equivalent authority **inside that Environment**. The outer security boundary remains the Hacocoon Environment.

- Host `/var/run/docker.sock` is never bind-mounted.
- Host Incus/Hacocoon/containerd control sockets are not exposed through this compatibility path.
- Docker API is not TCP-listened by default.
- Docker compatibility does not grant GitHub/cloud/registry/Host credentials.

## Telemetry

v0.15 OCI usage telemetry belongs to the optional plugin. It inventories the selected driver (`nerdctl` or `docker`) and deduplicates Seed recommendation by immutable image identity rather than double-counting one OCI digest merely because multiple CLIs can see it.

## Acceptance

Repository/local CI verifies the plugin-owned systemd packaging. Real acceptance still needs a supported Base/Incus host showing the intended lifecycle:

```text
boot: containerd/profile services as configured; dockerd inactive
first Docker API request: plugin socket activates dockerd
docker info succeeds
ordinary selected OCI CLI remains usable
```

> **Docker and nerdctl are optional OCI plugin profiles. Hacocoon Core does not choose a universal container runtime.**
