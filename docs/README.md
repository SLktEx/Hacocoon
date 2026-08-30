# Documentation

[**日本語**](README.ja.md) | English

Hacocoon is **pre-1.0**. Keep three questions separate: architecture intent, current repository reality, and real-host acceptance.

> If you only want to know what exists on `main`, start with [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md).

## Source-of-truth order

1. Current code reality: [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md)
2. Milestone numbering/status: [`00D_VERSIONING_AND_RELEASE_STATUS.md`](00D_VERSIONING_AND_RELEASE_STATUS.md)
3. Product boundary/roadmap: [`00_REBASELINE_AND_ROADMAP.md`](00_REBASELINE_AND_ROADMAP.md)
4. Security architecture: [`00B_SECURITY_ARCHITECTURE.md`](00B_SECURITY_ARCHITECTURE.md)
5. Terminology: [`00C_TERMINOLOGY_AND_BOUNDARIES.md`](00C_TERMINOLOGY_AND_BOUNDARIES.md)
6. Adapter/plugin architecture: [`00A_PLUGIN_ARCHITECTURE.md`](00A_PLUGIN_ARCHITECTURE.md)
7. Feature contracts: versioned milestone documents below

## Current milestone map

**Legend:** ✅ implemented · 🧪 experimental · 🚧 planned

| Version | Gate | State | Contract |
|---|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | ✅ | [`01_v0.1_SECURE_WORKSPACE_RUNTIME.md`](01_v0.1_SECURE_WORKSPACE_RUNTIME.md) |
| v0.2 | Workspace Abstraction & Lease | ✅ | roadmap / implementation status |
| v0.3 | Client & Interactive Access | ✅ | [`CLIENT_ACCESS.md`](CLIENT_ACCESS.md) |
| v0.4 | Policy & Capability Foundation | ✅ | security architecture |
| v0.5 | Git / GitHub Push Capability | ✅ | [`GIT_GITHUB_CAPABILITY.md`](GIT_GITHUB_CAPABILITY.md) |
| v0.6 | Agent & Orchestrator Integration | ✅ | roadmap |
| v0.7 | Remote / Cloud Runtime | 🧪 | [`REMOTE_CLOUD_PROVISIONING.md`](REMOTE_CLOUD_PROVISIONING.md) |
| v0.8 | Client Adapters & VS Code | ✅ | [`08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md`](08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md) |
| v0.9 | Per-Agent Sandbox | ✅ | [`09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md`](09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md) |
| v0.10 | VS Code Remote Agent Host Adapter | ✅ | [`10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md`](10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md) |
| v0.11 | Base Images & Custom Environments | ✅ first slice | [`11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md`](11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md) |
| v0.12 | Sandbox Resource Limits | ✅ first slice | [`12_v0.12_SANDBOX_RESOURCE_LIMITS.md`](12_v0.12_SANDBOX_RESOURCE_LIMITS.md) |
| v0.13 | Managed Sandbox Network | ✅ | [`13_v0.13_MANAGED_SANDBOX_NETWORK.md`](13_v0.13_MANAGED_SANDBOX_NETWORK.md) |
| v0.14 | Git Fetch Plugin | ✅ | [`14_v0.14_GIT_FETCH_PLUGIN.md`](14_v0.14_GIT_FETCH_PLUGIN.md) |
| v0.15 | OCI Seed Usage & Recommendation | ✅ first slice | [`15_v0.15_OCI_SEED_RECOMMENDATION.md`](15_v0.15_OCI_SEED_RECOMMENDATION.md) |
| v0.16 | OCI Image Deletion | ✅ first slice | [`16_v0.16_OCI_IMAGE_DELETION.md`](16_v0.16_OCI_IMAGE_DELETION.md) |
| v0.17 | Docker Compatibility | ✅ packaging foundation | [`17_v0.17_DOCKER_COMPATIBILITY.md`](17_v0.17_DOCKER_COMPATIBILITY.md) |
| v0.18 | Optional Local OCI Registry | 🚧 | [`18_v0.18_OPTIONAL_LOCAL_OCI_REGISTRY.md`](18_v0.18_OPTIONAL_LOCAL_OCI_REGISTRY.md) |
| v0.19 | OCI Seed Builder & Btrfs/COW | 🚧 | [`19_v0.19_OCI_SEED_AND_COW.md`](19_v0.19_OCI_SEED_AND_COW.md) |

Implemented milestones are contiguous through **v0.17**. v0.18 is the next planned product slice.

## Important boundaries

### Base images vs workload OCI images

Top-level commands remain Hacocoon Base identity operations:

```text
haco image list
haco image inspect <base>
haco create --base <base> --workspace <path> <environment>
```

Workload OCI tooling is optional and lives under:

```text
HACO_PLUGIN_OCI=nerdctl  # or docker
haco plugin oci ...
```

See [`OCI_RUNTIME_AND_DOCKER_COMPAT.md`](OCI_RUNTIME_AND_DOCKER_COMPAT.md).

### Git

Privileged GitHub fetch/push is under `haco plugin git`. Ordinary Git UX remains Git's responsibility. See [`GIT_GITHUB_CAPABILITY.md`](GIT_GITHUB_CAPABILITY.md).

### Client UI / notifications

VS Code is the first client adapter, not Core. Browser/Web notifications and richer Interaction API work remain future client/adapter functionality; a VS Code extension is optional.

## Specification vs implementation

A specification is a design/acceptance contract, not proof that a feature exists. `IMPLEMENTATION_STATUS.md` is authoritative for code reality, and real Incus/Windows/AWS/container-tool/Btrfs acceptance is tracked separately from repository tests.

## Editing rule

When changing docs, update the source that owns the fact, keep implementation and acceptance claims separate, and run:

```text
python tools/check_docs.py
```
