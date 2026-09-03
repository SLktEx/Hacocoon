<div align="center">

<img src="docs/assets/readme/hacocoon-logo.webp" alt="Hacocoon — Secure Workspace Runtime" width="520">

# Hacocoon

### Run coding agents freely inside. Keep host authority outside.

**An open-source secure workspace runtime for humans, developer tools, and coding agents.**

[日本語](README.ja.md) · [Documentation](docs/README.md) · [Security](docs/security/security-architecture.md) · [Implementation status](docs/IMPLEMENTATION_STATUS.md) · [Roadmap](docs/status/architecture-and-roadmap.md)

[![CI](https://github.com/SLktEx/Hacocoon/actions/workflows/test.yml/badge.svg)](https://github.com/SLktEx/Hacocoon/actions/workflows/test.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

</div>

<p align="center">
  <img src="docs/assets/readme/hacocoon-hero.webp" alt="Hacocoon secure workspace runtime overview: AI runs inside, authority stays outside" width="100%">
</p>

Hacocoon puts a Workspace behind an isolated execution boundary while keeping privileged authority on the trusted Host. Coding agents can install packages, edit files, build, test, debug, and make destructive changes inside a disposable Environment without automatically receiving Host credentials, Incus management authority, unrestricted external access, or permission to raise their own resource limits.

> [!WARNING]
> **Hacocoon is pre-1.0 and under active development. Breaking changes are expected.**
>
> The product-facing `haco` CLI is currently being rebuilt from the basic user workflow outward. The previous CLI is temporarily available as `hacoq` for migration only and will be deleted. See [CLI migration](docs/CLI_MIGRATION.md).
>
> Incus is the active Environment backend today. The provider seam remains generic, while the previous concrete EC2/AWS/EBS implementation is deferred. See [Implementation status](docs/IMPLEMENTATION_STATUS.md) for current repository reality and real-host acceptance gaps, and [Versioning and release status](docs/status/versioning-and-release-status.md) for the authoritative fast-moving development checkpoint.

## Why Hacocoon?

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
                     |
             Incus (current)
```

Hacocoon separates **freedom inside a development Environment** from **authority over the Host and external services**:

- broad local freedom for build/test/debug work;
- fail-closed Host-side Policy and narrow Capabilities for privileged operations;
- Host-owned long-lived credentials instead of mounting them into arbitrary Environments;
- persisted Workspace write leases and Provider-enforced resource ceilings;
- loopback-oriented client access and reusable client adapters;
- client-neutral interaction events without making observation an authorization path.

## Quick start

<p align="center">
  <img src="docs/assets/readme/hacocoon-quickstart.webp" alt="Hacocoon VS Code quick start: open a workspace, create or reuse an isolated environment, connect through loopback-only Remote-SSH, and run tests" width="100%">
</p>

### Build

```bash
git clone https://github.com/SLktEx/Hacocoon.git
cd Hacocoon

go build -o ./bin/haco ./cmd/haco-product
go build -o ./bin/hacoq ./cmd/haco
go build -o ./bin/haco-vscode ./cmd/haco-vscode
go build -o ./bin/haco-agent-host ./cmd/haco-agent-host
go build -o ./bin/haco-notify ./cmd/haco-notify

./bin/haco --version
./bin/haco help
```

The reset product CLI intentionally starts small. Current low-level behavior remains available temporarily through `hacoq`; do not build new integrations against it.

For Windows + WSL host setup, see [Windows / WSL bootstrap](docs/WINDOWS_WSL_BOOTSTRAP.md). On the supported local path, normal interactive WSL entry opens the persistent trusted `haco-host`; Physical Host root remains the explicit recovery path.

### Temporary legacy workspace commands

The following commands are migration-only examples while their product-facing replacements are rebuilt:

```bash
./bin/hacoq run --workspace "$PWD" -- go test ./...
./bin/hacoq create --workspace "$PWD" dev
./bin/hacoq exec dev -- go test ./...
./bin/hacoq shell dev
./bin/hacoq status dev
./bin/hacoq delete dev
```

### Open in VS Code

```bash
./bin/haco-vscode open .
```

Hacocoon creates or reuses the Environment, prepares loopback-oriented SSH access, and opens `/workspace` through normal VS Code Remote-SSH. VS Code is the first convenience client, not a Core dependency.

## Core, Standard, and Plugin

Hacocoon separates product contracts from default implementations and optional integrations.

| Layer | Role | Examples |
|---|---|---|
| **Core** | Stable product semantics and security boundaries | Environment, Workspace lease, Policy, Capability, interaction contracts |
| **Standard** | Project-maintained replaceable defaults used by normal installations | current Incus backend; hostname-aware egress enforcement |
| **Plugin** | Optional/specialized integrations | Git helpers, nerdctl/Docker/OCI tooling |

See [Adapter and extension architecture](docs/design/plugin-architecture.md) and [Design principles](docs/DESIGN_PRINCIPLES.md).

## Bases and optional OCI tooling

During the CLI migration, these existing low-level operations are exposed under temporary `hacoq`:

```bash
hacoq base list
hacoq base inspect haco/ubuntu-26.04
hacoq create --base haco/ubuntu-26.04 --workspace "$PWD" dev

HACO_PLUGIN_OCI=nerdctl hacoq plugin oci seed sample
HACO_PLUGIN_OCI=nerdctl hacoq plugin oci seed recommend
HACO_PLUGIN_OCI=nerdctl hacoq plugin oci seed build
HACO_PLUGIN_OCI=nerdctl hacoq plugin oci seed current
HACO_PLUGIN_OCI=docker  hacoq plugin oci docker status dev
HACO_PLUGIN_OCI=docker  hacoq plugin oci docker prepare dev
```

Core does not require containerd, nerdctl, Docker, or a local Registry. Current implementation reality lives in [Implementation status](docs/IMPLEMENTATION_STATUS.md); the intentionally fast-moving pre-1.0 checkpoint number and history live in [Versioning and release status](docs/status/versioning-and-release-status.md). README deliberately does not duplicate the checkpoint table. The Local OCI Registry direction remains deferred/unversioned optional infrastructure.

## Reusable clients

`pkg/clientadapter` provides a client-neutral contract for exact Environment ensure/reuse, status, `/workspace` discovery, loopback SSH/TCP connections, revoke/delete, and interaction batches. The client keeps its SSH private key and IDE configuration; Hacocoon receives only public-key material.

Notification clients consume the same read-only interaction stream for browser, native OS, and optional VS Code notifications without turning observation into an approval path.

See [Reusable client adapter contract](docs/CLIENT_ADAPTER_CONTRACT.md) and [Interaction events](docs/INTERACTION_EVENTS.md).

## Security model

The trusted Host owns Hacocoon state, Policy, credentials, resource ceilings, and privileged Capability execution. Coding agents do not need Hacocoon or Incus management authority merely to edit, build, test, or debug code inside an Environment. On the local Incus path, the persistent `haco-host` is part of the TCB and remains distinct from untrusted Environments; raw Incus control stays on the Physical Host.

Key rules include:

- long-lived Host credentials are not mounted into Environments for convenience;
- privileged external actions cross narrow Policy / Capability boundaries;
- requested finite resource limits are enforced or rejected, not silently ignored;
- local client exposure is loopback-oriented by default;
- custom Base contents do not grant Host-side authority;
- cleanup and recovery ambiguity is surfaced rather than converted into success.

Read [Security architecture](docs/security/security-architecture.md), [Trusted logical Host](docs/design/trusted-host.md), and the [adversarial audit guide](.github/security/ADVERSARIAL_AUDIT.md) before changing security-sensitive behavior.

## Documentation

- [Documentation index](docs/README.md)
- [CLI migration](docs/CLI_MIGRATION.md)
- [Documentation style guide](docs/DOCUMENTATION_STYLE_GUIDE.md)
- [Implementation status](docs/IMPLEMENTATION_STATUS.md)
- [Architecture and roadmap](docs/status/architecture-and-roadmap.md)
- [Versioning and release status](docs/status/versioning-and-release-status.md)
- [Trusted logical Host](docs/design/trusted-host.md)
- [Canonical terminology](docs/reference/terminology-and-boundaries.md)
- [Windows / WSL bootstrap](docs/WINDOWS_WSL_BOOTSTRAP.md)
- [Release security](docs/RELEASE_SECURITY.md)

Documentation uses semantic paths: feature addresses do not contain release/milestone numbers. ADR sequence numbers are the deliberate exception.

## Development

The primary supported Host baseline is **Ubuntu 26.04+**. GitHub-hosted Linux CI is pinned explicitly to **`ubuntu-26.04`** so CI exercises that baseline rather than a floating `ubuntu-latest` image or an older Ubuntu generation. Real Incus, managed-storage privilege, and trusted-host acceptance are tracked in the implementation/status authorities instead of being copied into this introduction.

```bash
go test ./...
go test -race ./...
go vet ./...
python tools/check_docs.py
```

For the maintained local CI entry point:

```bash
bash tools/ci-local.sh
```

## License

Hacocoon is licensed under the [Apache License 2.0](LICENSE).
