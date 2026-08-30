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

Coding agents can install packages, edit files, build, test, debug, and make destructive changes inside a disposable Environment without automatically receiving Host credentials, Incus management authority, unrestricted external access, or permission to raise their own resource limits.

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
```

> [!WARNING]
> **Hacocoon is pre-1.0 and under active development. Breaking changes are expected.**
>
> The roadmap is implemented contiguously through **v0.12**, but real-provider/client acceptance remains pending for several Incus, Windows/WSL, VS Code Agent Host, Base-image, resource-enforcement, and AWS paths. See [Implementation status](docs/IMPLEMENTATION_STATUS.md).

## Why Hacocoon?

Coding agents are most useful when they can actually work. The problem is that **freedom inside a development environment** and **authority over the Host** are not the same thing.

Hacocoon separates them:

- **Broad local freedom** — tools and agents can work normally inside an isolated Environment.
- **Narrow external authority** — privileged Host and external actions cross explicit Capability / Policy boundaries.
- **Host-owned credentials** — long-lived credentials do not need to be mounted into the Environment for convenience.
- **Auditable approvals** — sensitive actions leave Host-side decision and event records.
- **Resource ceilings** — CPU, memory, PID, and root-storage budgets can be enforced by the Provider.
- **Use your existing UI** — VS Code is the first convenience client; Hacocoon does not require its own AI chat UI.

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

```text
Workspace -> Hacocoon Environment -> loopback SSH alias -> VS Code Remote-SSH
```

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
| **Capabilities** | Narrow privileged operations instead of broad sandbox-owned Host credentials |
| **Git / GitHub** | Privileged Git push exposed as a plugin capability |
| **Base images** | Provider-neutral logical Base selection resolved to immutable revisions |
| **OCI tooling** | Optional containerd/nerdctl Seed telemetry and image lifecycle under `haco plugin oci` |
| **Resource limits** | CPU, memory, PID, and Environment root-storage budgets |
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

An agent can be powerful inside the sandbox without becoming the authority that manages the sandbox. Coding agents do **not** need Hacocoon or Incus management credentials merely to edit, build, test, or debug a project.

## VS Code and per-agent sandboxes

VS Code is the first supported convenience client, not a Core dependency.

For normal interactive development:

```bash
haco-vscode open .
```

For a trusted integration that maps an opaque external agent session to its own Environment:

```bash
haco-agent-host prepare --session <opaque-id> [workspace]
haco-agent-host release --session <opaque-id>
```

The client keeps the SSH private key. Hacocoon owns Environment allocation and safe connection preparation; the external client owns its Agent Host behavior.

- **v0.9**: [Per-Agent Sandbox & Agent Host](docs/09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md)
- **v0.10**: [VS Code Remote Agent Host Adapter](docs/10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md)

## Base images

Choose a logical Base when creating an Environment:

```bash
haco base list
haco base inspect haco/ubuntu-26.04
haco create --base haco/ubuntu-26.04 --workspace "$PWD" dev
```

`haco base` is for Hacocoon Environment starting points. OCI/container images are deliberately kept out of that namespace and live under `haco plugin oci`.

For Incus, Hacocoon resolves a mutable source to a validated immutable fingerprint before creation and persists that revision on the Environment.

```text
my-dev -> revision A -> Environment 1
my-dev -> revision B -> Environment 2
Environment 1 remains recorded on revision A
```

- **v0.11**: [Base Images & Custom Environments](docs/11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md)
- More details: [Base Images](docs/BASE_IMAGES.md)

## OCI plugin

OCI/containerd/nerdctl-specific operations are grouped under the optional OCI plugin surface:

```bash
haco plugin oci seed sample
haco plugin oci seed recommend
haco plugin oci image delete docker.io/library/node:24
```

This keeps container-image lifecycle separate from Hacocoon/Incus Base-image lifecycle.

## Resource budgets

```bash
haco create \
  --cpu 4 \
  --memory 8GiB \
  --pids 1024 \
  --root-size 40GiB \
  --workspace . dev

haco run --cpu 2 --memory 4GiB --workspace . -- go test ./...
```

Finite limits must be enforced or rejected. A Provider is not allowed to silently ignore a requested finite budget.

- **v0.12**: [Sandbox Resource Limits](docs/12_v0.12_SANDBOX_RESOURCE_LIMITS.md)

## Privileged Git push is a plugin

Git push deliberately sits outside the Core CLI surface:

```bash
haco plugin git push ...
```

The plugin still crosses the Host-owned Policy / Capability boundary. It does not make the operation trusted by default and does not hand Host credentials to the Environment.

## What Hacocoon is not

Hacocoon deliberately does **not** try to be an IDE, AI chat UI, Git worktree manager, agent scheduler, DAG engine, model router, retry engine, development-review queue, or model-budget manager.

Those tools can sit above Hacocoon and use it as an execution and security boundary.

```text
Daintree / other orchestrator
          |
 task / worktree / agent ownership
          |
       Workspace
          |
      Hacocoon
          |
      Environment
```

## Security model

The trusted Host owns Hacocoon state, Policy, credentials, resource ceilings, and privileged Capability execution.

Core rules include:

- long-lived Host credentials are not mounted into Environments for convenience;
- privileged external actions go through narrow Capabilities;
- Policy evaluation fails closed;
- Capability requests and decisions are auditable;
- Workspace write access is protected by persisted leases;
- local exposure is loopback-oriented by default;
- Provider-specific and Client-specific concepts stay outside Core;
- custom Base contents do not grant Host-side authority;
- requested finite resource limits are not silently ignored;
- cleanup and recovery failures are surfaced instead of converted into success.

Read [Security Architecture](docs/00B_SECURITY_ARCHITECTURE.md) before changing security-sensitive behavior.

## Current maturity

`v0.1 Runtime` → `v0.2 Workspace & Lease` → `v0.3 Access` → `v0.4 Policy & Capability` → `v0.5 Git/GitHub` → `v0.6 Agent Integration` → `v0.7 EC2` → `v0.8 VS Code` → `v0.9 Per-Agent Sandbox` → `v0.10 Agent Host Adapter` → `v0.11 Base Images` → `v0.12 Resource Limits`

Implemented does not automatically mean production-accepted on every real Provider or Client. Authoritative status:

- [Implementation status](docs/IMPLEMENTATION_STATUS.md)
- [Versioning and release status](docs/00D_VERSIONING_AND_RELEASE_STATUS.md)
- [Documentation index](docs/README.md)

## CLI at a glance

```text
haco create
haco base list
haco base inspect
haco exec
haco shell
haco delete
haco status
haco connections
haco forward
haco unforward
haco ssh
haco plugin git push
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

Both settings are required. Real AWS / EC2 / SSM / EBS acceptance remains tracked separately, and finite v0.12 resource budgets are currently rejected before AWS-side creation because equivalent enforcement has not yet been proven.

## Development

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/haco
go build ./cmd/haco-vscode
go build ./cmd/haco-agent-host
python tools/check_docs.py
```

## Documentation

- [Documentation index](docs/README.md)
- [Architecture guide](docs/ARCHITECTURE_GUIDE.md)
- [Security architecture](docs/00B_SECURITY_ARCHITECTURE.md)
- [Windows / WSL bootstrap](docs/WINDOWS_WSL_BOOTSTRAP.md)
- [Implementation status](docs/IMPLEMENTATION_STATUS.md)
- [Release security](docs/RELEASE_SECURITY.md)

## License

Hacocoon is licensed under the [Apache License 2.0](LICENSE).

## Compatibility policy

Until Hacocoon reaches a stable compatibility milestone, assume that **breaking changes can happen between any revisions**.
