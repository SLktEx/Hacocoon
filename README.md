<div align="center">

<img src="docs/assets/readme/hacocoon-logo.webp" alt="Hacocoon" width="520">

# Hacocoon

A secure workspace runtime for people, developer tools and coding agents.

[日本語](README.ja.md) · [Get started](docs/guides/getting-started.md) · [Documentation](docs/README.md)

[![CI](https://github.com/SLktEx/Hacocoon/actions/workflows/test.yml/badge.svg)](https://github.com/SLktEx/Hacocoon/actions/workflows/test.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

</div>

Hacocoon runs development tools in isolated **Environments**, keeps project files in
retained **Workspaces**, and mediates access to Host credentials and external services.
An agent can edit, build and test without receiving Incus management authority.

> [!WARNING]
> Hacocoon is **pre-1.0**. Breaking changes are expected. The current local backend
> is Incus on Ubuntu 26.04+ (including a dedicated WSL 2 distribution on Windows).
> Implementation and tested platform coverage are separate; see [current status](docs/IMPLEMENTATION_STATUS.md).

## Get started

Follow [installation → create → connect → develop → stop and resume](docs/guides/getting-started.md).
It identifies which terminal to use, required permissions and what data remains.

After installing and entering trusted `haco-host`, the core sequence is:

```bash
haco doctor
haco repo clone --branch main sample https://github.com/SLktEx/Hacocoon.git
haco workspace create --repo sample sample-work
haco env create --workspace managed:sample-work sample-dev
haco open --client ssh sample-dev
```

Creating an Environment from a managed Git Workspace automatically connects the
Git broker. Configure the narrow Git/package permissions described in the guide before
network operations. Exit the development shell, then run `haco env stop sample-dev`
in the Host. Later, `haco open --client ssh sample-dev` resumes it.
Use `haco open sample-dev` for VS Code with Remote-SSH installed.

The current product CLI is `haco`. Remaining legacy-only operations are isolated
in [CLI migration](docs/reference/cli-migration.md).

## Know what persists

| Object | Role | After Environment deletion |
|---|---|---|
| Host | Trusted management, credentials and controller access | Remains |
| Workspace | Project files and independent Git metadata | Remains, including unpushed and untracked work |
| Environment | Running tools, packages and root filesystem | Deleted |
| Base | Starting image selected at creation | Remains independently; it does not capture later edits |
| OCI Store | Optional container images, metadata and build cache | Remains; running containers are not resumed automatically |

Stop preserves the Environment too. Delete discards its root filesystem.
See [data lifetime](docs/guides/data-lifetime.md) before cleanup.
Writable Workspace data remains writable by the agent; isolation is not a backup.

## Next steps

- [Task guides and references](docs/README.md): Git approvals, SSH, recipes, preview, snapshots and cleanup.
- [Bases](docs/design/base-images-and-custom-environments.md): `haco base list`, inspect and build.
- [Optional OCI Stores](docs/design/persistent-oci-store.md): `haco plugin oci`; Core does not require Docker or containerd.
- [Security architecture](docs/security/security-architecture.md): shared-kernel limits and trust boundaries.
- [Implementation status](docs/IMPLEMENTATION_STATUS.md), [roadmap](docs/status/architecture-and-roadmap.md), and [Versioning and release status](docs/status/versioning-and-release-status.md).
- [Contributing and local CI](CONTRIBUTING.md): builds, validation and the current collaborator-only PR policy.

Licensed under [Apache License 2.0](LICENSE).
