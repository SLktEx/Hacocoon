# Docker Compatibility Plugin

> Legacy optional integration. Commands below use temporary `hacoq` on the Physical Host. See the [current CLI](../reference/cli.md) and [migration boundary](../reference/cli-migration.md) for ordinary product use.

Status: **repository implementation complete ahead of milestone order; real-host acceptance remains environment-dependent.**

v0.18 defines Docker compatibility as an optional OCI plugin feature. Hacocoon Core models Environments and execution; it does not require Docker Engine, containerd, or nerdctl.

## Maintained OCI profile

The project-maintained OCI plugin profile may use:

```text
containerd + nerdctl
```

That is a supported project profile, not a Core invariant. Docker compatibility is additive:

```text
maintained profile -> nerdctl -> containerd
compatibility      -> genuine Docker CLI -> optional/on-demand dockerd -> existing containerd where supported
```

The standard Tooling Base does not preinstall Docker Engine. By default, `docker` is a compatibility symlink from `/usr/bin/docker` to `/usr/local/bin/nerdctl`; a Docker package installed later may naturally replace `/usr/bin/docker` with the genuine CLI.

The standard Tooling Base retains `hacocoon-docker.socket` / `.service` while masking vendor `docker.service` / `docker.socket` to avoid competing for `/run/docker.sock`. An enabled `hacocoon-docker-autostart.path` watches for `/usr/bin/dockerd`; when Docker Engine is installed later it automatically enables and starts `hacocoon-docker.socket`. `dockerd` itself remains on-demand and starts only when an Environment-local client opens `/run/docker.sock`.

The goal is to support tools that insist on Docker CLI/Engine APIs without forcing every Hacocoon installation to run a permanent Docker daemon or install Docker Engine.

## Commands

Docker lifecycle integration is exposed only when `HACO_PLUGIN_OCI=docker` is selected:

```text
hacoq plugin oci docker status <environment> [--json]
hacoq plugin oci docker prepare <environment> [--json]
```

`status` is observational. It checks the Environment-local Docker profile without starting `dockerd`.

`prepare` is deliberately narrow and idempotent. If standard-Base autostart already prepared the socket after Docker was installed, `prepare` simply validates and accepts that ready state.

1. resolve the managed Environment through trusted Hacocoon state;
2. verify the genuine `docker` CLI, `dockerd`, `containerd`, systemd, and the `docker` group are present;
3. verify the installed `hacocoon-docker.socket` and `hacocoon-docker.service` exactly match the plugin-pinned unit files;
4. refuse to stop an already-active vendor Docker daemon/socket;
5. disable inactive vendor Docker autostart units;
6. enable/start `hacocoon-docker.socket` if needed;
7. re-probe and fail closed unless the expected socket-activated state is reached.

`prepare` does **not** install packages, pull images, mount Host sockets, or silently terminate an existing guest Docker daemon. The selected Base/Seed is responsible for providing the Docker compatibility profile and the pinned unit files.

An inactive `hacocoon-docker.service` is healthy: the Engine is expected to start on demand when an Environment-local client opens `/run/docker.sock`.

## Plugin boundary

- Docker/nerdctl-specific orchestration belongs outside Core under `modules/plugin/oci` / `hacoq plugin oci`.
- `HACO_PLUGIN_OCI=nerdctl|docker` remains explicit opt-in for plugin CLI integration. Installing Docker Engine into the standard Tooling Base is itself treated as explicit intent to activate the compatibility socket automatically.
- `dockerd` is never an always-on Hacocoon requirement.
- Engine startup is Environment-local and socket-activated.
- Never mount the Host Docker socket or Host containerd/Incus/Hacocoon control sockets.
- Do not expose a TCP Docker API listener by default.
- Avoid unnecessary duplicate OCI content without promising complete byte-level deduplication.

## Repository gate

The repository gate is implemented by the plugin-owned systemd packaging, lifecycle/status service, CLI integration, fail-closed unit-drift checks, and unit tests. Supported-host acceptance remains a separate concern: a real Base must provide the required binaries/units and the target Incus/systemd environment must support the socket-activation path.


## Non-goals

- making containerd/nerdctl a Core requirement;
- making Docker Engine mandatory;
- running dockerd permanently when no workload needs it;
- exposing Host Docker/containerd sockets;
- installing or upgrading Docker packages from `hacoq plugin oci docker prepare`;
- placing Docker-specific concepts into Core.