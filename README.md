<div align="center">

<img src="docs/assets/readme/hacocoon-logo.webp" alt="Hacocoon — Secure Workspace Runtime" width="520">

# Hacocoon

### Run coding agents freely inside. Keep host authority outside.

**An open-source secure workspace runtime for humans, developer tools, and coding agents.**

[日本語](README.ja.md) · [Documentation](docs/README.md) · [Security](docs/00B_SECURITY_ARCHITECTURE.md) · [Implementation status](docs/IMPLEMENTATION_STATUS.md) · [Roadmap](docs/00_REBASELINE_AND_ROADMAP.md)

[![CI](https://github.com/SLktEx/Hacocoon/actions/workflows/test.yml/badge.svg)](https://github.com/SLktEx/Hacocoon/actions/workflows/test.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

</div>

<p align="center">
  <img src="docs/assets/readme/hacocoon-hero.webp" alt="Hacocoon secure workspace runtime overview: AI runs inside, authority stays outside" width="100%">
</p>

Hacocoon puts a Workspace behind an isolated execution boundary while keeping privileged authority on the trusted Host.

Coding agents can install packages, edit files, build, test, debug, and make destructive changes inside a disposable Environment without automatically receiving Host credentials, Incus management authority, unrestricted external authority, or permission to raise their own resource limits.

```text
VS Code / Shell / Coding Agent / Orchestrator
                     |
                  Workspace
                     |
          +----------v------------+
          |       Hacocoon        |
          | isolated Environment  |
          | resource budgets      |
          | workspace leases      |
          | policy / approvals    |
          | capabilities / audit  |
          +----------+------------+
                     |
            Environment provider
             /                \
      Incus (default)      EC2 (experimental)

optional integrations:
  haco plugin git ...
  haco plugin oci ...
```

> [!WARNING]
> **Hacocoon is pre-1.0 and under active development. Breaking changes are expected.**
>
> Product milestones are implemented contiguously through **v0.17**. Real-provider/client/tool acceptance remains pending for several Incus, Windows/WSL, VS Code Agent Host, AWS and OCI paths. See [Implementation status](docs/IMPLEMENTATION_STATUS.md).

## Why Hacocoon?

Coding agents are most useful when they can actually work. **Freedom inside a development environment** and **authority over the Host** are not the same thing.

- **Broad local freedom** — tools and agents can work normally inside an isolated Environment.
- **Narrow external authority** — privileged Host and external actions cross explicit Capability / Policy boundaries.
- **Host-owned credentials** — long-lived credentials do not need to be mounted into the Environment.
- **Auditable approvals** — sensitive actions leave Host-side decision and event records.
- **Resource ceilings** — CPU, memory, PID, and root-storage budgets can be Provider-enforced.
- **Use your existing UI** — VS Code is the first convenience client; Hacocoon does not require its own AI chat UI.
- **Optional integrations stay optional** — GitHub Git, nerdctl and Docker-specific workflows do not become universal Core requirements.

## Quick Start

<p align="center">
  <img src="docs/assets/readme/hacocoon-quickstart.webp" alt="Hacocoon VS Code quick start: open a workspace, create or reuse an isolated environment, connect through loopback-only Remote-SSH, and run tests" width="100%">
</p>

### Build

```bash
git clone https://github.com/SLktEx/Hacocoon.git
cd Hacocoon

go build -o ./bin/haco ./cmd/haco
go build -o ./bin/haco-vscode ./cmd/haco-vscode
go build -o ./bin/haco-agent-host ./cmd/haco-agent-host

./bin/haco doctor
```

For Windows + WSL host setup, start with [Windows / WSL bootstrap](docs/WINDOWS_WSL_BOOTSTRAP.md).

### Run a command in an isolated Workspace

```bash
./bin/haco run --workspace "$PWD" -- go test ./...
```

Or manage an Environment explicitly:

```bash
./bin/haco create --workspace "$PWD" dev
./bin/haco exec dev -- uname -a
./bin/haco shell dev
./bin/haco status dev
./bin/haco delete dev
```

### Open it in VS Code

```bash
./bin/haco-vscode open .
```

Hacocoon creates or reuses the Environment, prepares loopback-oriented SSH access, writes adapter-owned SSH configuration, and opens `/workspace` through normal VS Code Remote-SSH.

Cleanup:

```bash
./bin/haco-vscode delete .
```

## What you get

| Area | Hacocoon provides |
|---|---|
| **Isolation** | Provider-backed Environments with Incus as the local default |
| **Workspace safety** | Canonical Workspace identity and persisted write leases |
| **Execution** | `create`, `exec`, `shell`, `run`, lifecycle and recovery operations |
| **Interactive access** | Loopback-oriented SSH, forwarding, and VS Code Remote-SSH integration |
| **Agent isolation** | Per-agent Environment broker with persisted ownership proof |
| **Policy** | Fail-closed Host-side policy and explicit approval decisions |
| **Capabilities** | Narrow privileged operations instead of broad Sandbox-owned Host credentials |
| **Git / GitHub plugin** | Brokered fetch/push with Host-owned credentials kept outside the Environment |
| **Base images** | Provider-neutral logical Base selection resolved to immutable revisions |
| **Resource limits** | CPU, memory, PID, and Environment root-storage budgets |
| **Managed network** | Hacocoon-managed Incus sandbox network/profile with fail-closed drift behavior |
| **Optional OCI plugin** | nerdctl/Docker inventory, Seed recommendation, image deletion, Docker compatibility foundation |
| **Audit** | Events for lifecycle, capability, approval, and recovery-sensitive operations |
| **Providers** | Incus by default; EC2 behind explicit experimental opt-in |

## AI agents: permissive inside, mediated outside

```text
VS Code AI / Codex / Copilot / Claude / other agent
                         |
                         v
                 Incus Environment
                  broad local freedom
               within ResourceBudget
                         |
              ---- trust boundary ----
                         |
                     Hacocoon
              Policy / Capability / Audit
                         |
                 GitHub / AWS / Host
```

An agent can be powerful inside the Sandbox without becoming the authority that manages it. Coding agents do **not** need Hacocoon or Incus management credentials merely to edit, build, test, or debug a project.

## VS Code and per-agent sandboxes

VS Code is the first supported convenience client, not a Core dependency.

```bash
haco-vscode open .

haco-agent-host prepare --session <opaque-id> [workspace]
haco-agent-host release --session <opaque-id>
```

The client keeps the SSH private key. Hacocoon owns Environment allocation and safe connection preparation; the external client owns its Agent Host behavior.

- **v0.9**: [Per-Agent Sandbox & Agent Host](docs/09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md)
- **v0.10**: [VS Code Remote Agent Host Adapter](docs/10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md)

## Base images

```bash
haco image list
haco image inspect haco/ubuntu-26.04
haco create --base haco/ubuntu-26.04 --workspace "$PWD" dev
```

Top-level `haco image` describes **Hacocoon Base identity**. It is intentionally separate from workload container-image operations.

- **v0.11**: [Base Images & Custom Environments](docs/11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md)

## Resource budgets

```bash
haco create --cpu 4 --memory 8GiB --pids 1024 --root-size 40GiB --workspace . dev
haco run --cpu 2 --memory 4GiB --workspace . -- go test ./...
```

Finite limits must be enforced or rejected; a Provider may not silently ignore a requested finite budget.

- **v0.12**: [Sandbox Resource Limits](docs/12_v0.12_SANDBOX_RESOURCE_LIMITS.md)
- **v0.13**: [Managed Sandbox Network](docs/13_v0.13_MANAGED_SANDBOX_NETWORK.md)

## Privileged GitHub Git is a plugin

```bash
haco plugin git fetch <environment>
haco plugin git push <environment> --branch feature/x
```

The plugin crosses the Host-owned Policy / Capability boundary. GitHub HTTPS can use the Host `gh auth git-credential` provider without handing reusable credentials to the Environment. Ordinary Git UX remains Git's responsibility.

- **v0.14**: [Git Fetch Plugin](docs/14_v0.14_GIT_FETCH_PLUGIN.md)

## Optional OCI / Docker tooling

Hacocoon Core does not require a universal container runtime or CLI. Enable workload OCI tooling only when the deployment wants it:

```bash
export HACO_PLUGIN_OCI=nerdctl
# or: export HACO_PLUGIN_OCI=docker

haco plugin oci status
haco plugin oci seed sample
haco plugin oci seed recommend
haco plugin oci image delete docker.io/library/node:24@sha256:<digest>
```

The project-maintained nerdctl profile may use Environment-local `containerd + nerdctl`. Docker compatibility may add genuine Docker CLI and an on-demand Environment-local Engine path. Neither is a Core requirement.

- **v0.15**: [OCI Seed Usage & Recommendation](docs/15_v0.15_OCI_SEED_RECOMMENDATION.md)
- **v0.16**: [OCI Image Deletion](docs/16_v0.16_OCI_IMAGE_DELETION.md)
- **v0.17**: [Docker Compatibility](docs/17_v0.17_DOCKER_COMPATIBILITY.md)
- Detailed boundary: [Optional OCI Plugin and Docker Compatibility](docs/OCI_RUNTIME_AND_DOCKER_COMPAT.md)

## Next milestones

- **v0.18**: [Optional Local OCI Registry](docs/18_v0.18_OPTIONAL_LOCAL_OCI_REGISTRY.md) — optional infrastructure, not the default pull path.
- **v0.19**: [OCI Seed Builder & Btrfs/COW](docs/19_v0.19_OCI_SEED_AND_COW.md) — offline immutable Seed build/publish; never share writable containerd state between Environments.

## What Hacocoon is not

Hacocoon deliberately does **not** try to be an IDE, AI chat UI, Git worktree manager, agent scheduler, DAG engine, model router, retry engine, container CLI/runtime manager, or model-budget manager.

Those tools can sit above or beside Hacocoon and use it as an execution/security boundary.

## Security model

The trusted Host owns Hacocoon state, Policy, credentials, resource ceilings, and privileged Capability execution.

Core rules include:

- long-lived Host credentials are not mounted into Environments for convenience;
- privileged external actions go through narrow Capabilities;
- Policy evaluation fails closed;
- Workspace write access is protected by persisted leases;
- local exposure is loopback-oriented by default;
- Provider/client/tool-specific concepts stay outside Core;
- custom Base contents do not grant Host-side authority;
- requested finite limits are not silently ignored;
- Host Docker/Incus/Hacocoon control sockets are not passed into Environments as shortcuts;
- cleanup and recovery failures are surfaced instead of converted into success.

Read [Security Architecture](docs/00B_SECURITY_ARCHITECTURE.md) before changing security-sensitive behavior.

## Current maturity

`v0.1 Runtime` → `v0.2 Workspace & Lease` → `v0.3 Access` → `v0.4 Policy & Capability` → `v0.5 Git Push` → `v0.6 Agent Integration` → `v0.7 EC2` → `v0.8 VS Code` → `v0.9 Per-Agent Sandbox` → `v0.10 Agent Host` → `v0.11 Base Images` → `v0.12 Resource Limits` → `v0.13 Managed Network` → `v0.14 Git Fetch` → `v0.15 OCI Recommendation` → `v0.16 OCI Delete` → `v0.17 Docker Compatibility` → `v0.18 Registry (planned)` → `v0.19 Seed/COW (planned)`

Authoritative status:

- [Implementation status](docs/IMPLEMENTATION_STATUS.md)
- [Versioning and release status](docs/00D_VERSIONING_AND_RELEASE_STATUS.md)
- [Documentation index](docs/README.md)

## CLI at a glance

```text
haco create
haco image list
haco image inspect
haco exec
haco shell
haco delete
haco status
haco connections
haco forward
haco unforward
haco ssh
haco plugin git fetch
haco plugin git push
haco plugin oci status
haco plugin oci seed sample
haco plugin oci seed recommend
haco plugin oci image delete
haco capability request
haco run
haco events
haco doctor

haco-vscode open
haco-vscode delete

haco-agent-host prepare
haco-agent-host release
```

All surfaces remain pre-1.0 and may change.

## Experimental EC2 provider

The EC2 Provider is **experimental and disabled by default**.

```bash
export HACO_RUNTIME_PROVIDER=runtime.ec2
export HACO_EXPERIMENTAL_EC2=1
```

Both settings are required. Real AWS / EC2 / SSM / EBS acceptance remains tracked separately.

## Development

Use the maintained local entry point:

```bash
bash tools/ci-local.sh
```

Individual jobs are documented in [CONTRIBUTING.md](CONTRIBUTING.md).

## Documentation

- [Documentation index](docs/README.md)
- [Architecture guide](docs/ARCHITECTURE_GUIDE.ja.md)
- [Security architecture](docs/00B_SECURITY_ARCHITECTURE.md)
- [Windows / WSL bootstrap](docs/WINDOWS_WSL_BOOTSTRAP.md)
- [Implementation status](docs/IMPLEMENTATION_STATUS.md)
- [Release security](docs/RELEASE_SECURITY.md)

## License

Hacocoon is licensed under the [Apache License 2.0](LICENSE).

## Compatibility policy

Until Hacocoon reaches a stable compatibility milestone, assume that **breaking changes can happen between any revisions**.
