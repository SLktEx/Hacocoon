# Documentation

[**日本語**](README.ja.md) | English

Hacocoon is **pre-1.0**. Keep architecture intent, repository reality, and real-host acceptance separate.

> [!TIP]
> If you only want to know **what works in the current repository**, start with [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md).

## Start here

| I want to know… | Read |
|---|---|
| What Hacocoon is | [`../README.md`](../README.md) |
| What is implemented right now | [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md) |
| Architecture / roadmap | [`00_REBASELINE_AND_ROADMAP.md`](00_REBASELINE_AND_ROADMAP.md) |
| Authoritative milestone numbering | [`00D_VERSIONING_AND_RELEASE_STATUS.md`](00D_VERSIONING_AND_RELEASE_STATUS.md) |
| Security/trust boundaries | [`00B_SECURITY_ARCHITECTURE.md`](00B_SECURITY_ARCHITECTURE.md) |
| Canonical terminology | [`00C_TERMINOLOGY_AND_BOUNDARIES.md`](00C_TERMINOLOGY_AND_BOUNDARIES.md) |
| Client / VS Code integration | [`08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md`](08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md) |
| Per-agent sandbox | [`09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md`](09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md) |
| VS Code Agent Host | [`10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md`](10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md) |
| Base images / `haco base` | [`11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md`](11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md), [`BASE_IMAGES.md`](BASE_IMAGES.md) |
| Resource limits | [`12_v0.12_SANDBOX_RESOURCE_LIMITS.md`](12_v0.12_SANDBOX_RESOURCE_LIMITS.md) |
| Managed sandbox network | [`13_v0.13_MANAGED_SANDBOX_NETWORK.md`](13_v0.13_MANAGED_SANDBOX_NETWORK.md) |
| Git fetch plugin | [`14_v0.14_GIT_FETCH_PLUGIN.md`](14_v0.14_GIT_FETCH_PLUGIN.md) |
| OCI Seed recommendation | [`15_v0.15_OCI_SEED_RECOMMENDATION.md`](15_v0.15_OCI_SEED_RECOMMENDATION.md) |
| OCI image deletion | [`16_v0.16_OCI_IMAGE_DELETION.md`](16_v0.16_OCI_IMAGE_DELETION.md) |
| Docker compatibility plugin | [`17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.md`](17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.md), [`OCI_RUNTIME_AND_DOCKER_COMPAT.md`](OCI_RUNTIME_AND_DOCKER_COMPAT.md) |
| Optional Local OCI Registry | [`18_v0.18_LOCAL_OCI_REGISTRY.md`](18_v0.18_LOCAL_OCI_REGISTRY.md) |
| OCI Seed Builder / COW | [`19_v0.19_OCI_SEED_AND_COW.md`](19_v0.19_OCI_SEED_AND_COW.md) |

## Source-of-truth order

1. **Current code reality:** `IMPLEMENTATION_STATUS.md`
2. **Milestone numbering/status:** `00D_VERSIONING_AND_RELEASE_STATUS.md`
3. **Product boundary / roadmap intent:** `00_REBASELINE_AND_ROADMAP.md`
4. **Canonical vocabulary:** `00C_TERMINOLOGY_AND_BOUNDARIES.md`
5. **Cross-cutting security rules:** `00B_SECURITY_ARCHITECTURE.md`
6. **Feature contract:** the relevant versioned specification
7. **Operational/detail documents:** `CLIENT_ACCESS.md`, `REMOTE_CLOUD_PROVISIONING.md`, `BASE_IMAGES.md`, `OCI_RUNTIME_AND_DOCKER_COMPAT.md`, etc.
8. **Extension guidance:** `00A_PLUGIN_ARCHITECTURE.md`
9. **Implementation workflow:** `90_CODEX_IMPLEMENTATION_HANDOFF.md`

Entry-point READMEs summarize; they do not redefine implementation reality.

## Numbering rule

Hacocoon uses a deliberately simple pre-1.0 convention:

> **One independently useful product feature is approximately one minor milestone.**

Feature implementation PRs update the numbering/status docs when they land. Security fixes, bug fixes, hardening, refactors, CLI namespace cleanup, CI, docs, and test-only changes normally do not consume another product version.

## Current milestone status

**Status legend:** ✅ implemented · 🧪 partial/historical · 🚧 planned

| Version | Gate | State |
|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | ✅ |
| v0.2 | Workspace Abstraction & Lease | ✅ |
| v0.3 | Client & Interactive Access | ✅ |
| v0.4 | Policy & Capability Foundation | ✅ |
| v0.5 | Git / GitHub Capability | ✅ |
| v0.6 | Agent & Orchestrator Integration | ✅ |
| v0.7 | Remote / Cloud Runtime & External Capabilities | 🧪 routing seam retained; cloud implementation deferred |
| v0.8 | Client Adapters & VS Code Integration | ✅ |
| v0.9 | Per-Agent Sandbox & Agent Host Integration | ✅ foundation |
| v0.10 | VS Code Remote Agent Host Adapter | ✅ |
| v0.11 | Base Images & Custom Environments | ✅ first slice |
| v0.12 | Sandbox Resource Limits | ✅ first slice |
| v0.13 | Managed Sandbox Network | ✅ |
| v0.14 | Git Fetch Plugin | ✅ |
| v0.15 | OCI Seed Recommendation | ✅ |
| v0.16 | OCI Image Deletion | ✅ first slice |
| v0.17 | Docker Compatibility Plugin | 🧪 foundation |
| v0.18 | Optional Local OCI Registry | 🚧 |
| v0.19 | OCI Seed Builder & Btrfs/COW | 🚧 |

The fully implemented progression is contiguous through **v0.16**. See [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md) for exact repository reality and acceptance gaps.

v0.7 remains numbered because its provider-neutral routing seam is current. The former concrete EC2/AWS/EBS implementation is intentionally deferred and is not part of the active tree.

## Base vs OCI CLI

```text
haco base list
haco base inspect <base>

haco plugin oci seed sample
haco plugin oci seed recommend
haco plugin oci image delete <reference> [--all-environments]
```

`haco base` is for Hacocoon Environment starting points. OCI/containerd/nerdctl-specific lifecycle is under the optional `haco plugin oci` surface. This namespace cleanup is a refactor/boundary correction, not an extra product milestone.

## v0.13-v0.19 reading path

```text
v0.13 managed Incus sandbox network
  -> v0.14 brokered Git fetch plugin
  -> v0.15 OCI Seed recommendation
  -> v0.16 OCI identity deletion/tombstone
  -> v0.17 optional Docker compatibility plugin (partial)
  -> v0.18 optional Local Registry (planned)
  -> v0.19 offline immutable Seed Builder + COW (planned)
```

Important distinctions:

- v0.15 recommendation is implemented, but physical Seed build/publish is not.
- v0.16 deletion changes Host cache/future Seed selection and optionally current Environments; it does not mutate published immutable Seeds in place.
- v0.17 does not replace containerd + nerdctl as the standard runtime.
- v0.18 is optional; normal `nerdctl pull` may still go directly to configured upstreams when policy allows.
- v0.19 must not share one writable `/var/lib/containerd` across Environments.

## Specification vs implementation

A specification is a design/acceptance contract, not proof that a feature exists. `IMPLEMENTATION_STATUS.md` is the deciding source for current code reality.

Real-host acceptance is also separate: unit/fake-provider/repository checks do not prove real Incus, Windows/VS Code, private-registry, Docker compatibility, or future cloud behavior. Cloud acceptance remains deferred until a concrete adapter returns.

## Compatibility and breaking changes

Hacocoon is pre-1.0 and actively being simplified/hardened. CLI/helper binaries, state, adapters/plugins, policy/capability contracts, Base/image behavior, client/agent integration, providers, and documentation may change incompatibly.

Prefer an explicit incompatible correction over preserving accidental compatibility that weakens a security or ownership boundary.

## Editing rule

When changing docs or landing a feature:

1. update the document that owns the fact;
2. update `IMPLEMENTATION_STATUS.md` whenever code reality changes;
3. update `00D_VERSIONING_AND_RELEASE_STATUS.md` in the same feature PR when a new independent feature consumes a milestone;
4. do not bump product versions for fix/hardening/refactor/CLI-namespace/CI/docs-only work;
5. keep implementation claims separate from real-host acceptance claims;
6. keep deferred/partial boundaries explicit;
7. run `python tools/check_docs.py`.
