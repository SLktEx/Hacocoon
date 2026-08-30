# Architecture & Roadmap

> **Architecture baseline · Updated 2026-08-30**
>
> Hacocoon is a **Secure Workspace Runtime**.
> Current code reality: [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md).
> Authoritative milestone numbering: [`00D_VERSIONING_AND_RELEASE_STATUS.md`](00D_VERSIONING_AND_RELEASE_STATUS.md).

Hacocoon runs developer tools and coding agents inside isolated Environments while keeping Host, GitHub, cloud and other privileged authority behind trusted boundaries.

Hacocoon is **pre-1.0**. Architecture simplicity and security boundaries take priority over accidental compatibility.

## Product boundary

Hacocoon owns:

- Workspace identity and lease safety;
- isolated Environment lifecycle and cleanup;
- command/interactive execution;
- client access primitives;
- Policy / Approval / Capability / Audit;
- trusted agent-session → Environment binding;
- provider-neutral Base identity and ResourceBudget;
- provider safety such as managed Incus networking.

Hacocoon Core does not own:

- IDE/editor/chat UX;
- AI model choice, task DAG, retries or token budgets;
- Git branch/worktree strategy;
- ordinary Git UX;
- nerdctl/Docker/containerd workflow as a universal requirement;
- OCI Registry infrastructure;
- Btrfs-specific mechanics.

## Layering

```text
Client / IDE / Agent / Orchestrator
                |
         Workspace / request
                v
        Hacocoon Core
 Environment / Execution
 Policy / Capability / Audit
                |
       provider / adapter
          /           \
       Incus        EC2 (experimental)

optional feature surfaces:
  haco plugin git ...
  haco plugin oci ...
```

Provider mechanics and optional developer-tool integrations stay outside Core domain vocabulary.

## Project status at a glance

**Legend:** ✅ implemented · 🧪 experimental · 🚧 planned

| Version | Gate | Current code reality |
|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | ✅ |
| v0.2 | Workspace Abstraction & Lease | ✅ |
| v0.3 | Client & Interactive Access | ✅ |
| v0.4 | Policy & Capability Foundation | ✅ |
| v0.5 | Git / GitHub Push Capability | ✅ |
| v0.6 | Agent & Orchestrator Integration | ✅ |
| v0.7 | Remote / Cloud Runtime | 🧪 experimental and disabled by default |
| v0.8 | Client Adapters & VS Code | ✅ |
| v0.9 | Per-Agent Sandbox | ✅ |
| v0.10 | VS Code Remote Agent Host Adapter | ✅ |
| v0.11 | Base Images & Custom Environments | ✅ first slice |
| v0.12 | Sandbox Resource Limits | ✅ first slice |
| v0.13 | Managed Sandbox Network | ✅ |
| v0.14 | Git Fetch Plugin | ✅ |
| v0.15 | OCI Seed Usage & Recommendation | ✅ optional-plugin first slice |
| v0.16 | OCI Image Deletion | ✅ optional-plugin first slice |
| v0.17 | Docker Compatibility | ✅ optional-plugin packaging foundation |
| v0.18 | Optional Local OCI Registry | 🚧 planned |
| v0.19 | OCI Seed Builder & Btrfs/COW | 🚧 planned |

Implemented milestones are contiguous through **v0.17**. v0.18 is the next planned product gate.

## Workspace and agent model

Workspace is opaque to Core. A human, IDE or external orchestrator may create a directory/worktree and pass it to Hacocoon. Parallel RW work should use distinct canonical Workspaces, normally distinct Git worktrees.

A coding agent may have broad freedom **inside** its Environment. It does not receive Hacocoon/Incus management authority merely because it can install packages, build, test or modify source.

```text
Coding Agent
    |
Environment      <- broad local freedom
    |
---- trust boundary ----
    |
Hacocoon Policy / Capability
    |
GitHub / AWS / Host
```

## Client adapters

VS Code is the first supported client adapter through `haco-vscode` and standard Remote-SSH. Hacocoon does not become the editor or AI chat product.

Future browser/Web notification and richer Interaction API work belongs to the client/adapter layer. A VS Code extension may expose notifications/approval UX, but remains optional.

## Git extension

Ordinary Git remains Git's responsibility. Security-sensitive Host-side GitHub operations are extension commands:

```text
haco plugin git fetch <environment>
haco plugin git push <environment> --branch <branch>
```

GitHub HTTPS authentication uses the Host-owned `gh auth git-credential` provider inside the hardened broker without copying credentials into the Sandbox.

## Optional OCI extension

Hacocoon Core does not choose a universal container runtime. Deployments that want OCI workflow enable a plugin explicitly:

```text
HACO_PLUGIN_OCI=nerdctl
# or
HACO_PLUGIN_OCI=docker
```

The project-maintained nerdctl profile may use Environment-local `containerd + nerdctl`. Docker compatibility may add genuine Docker CLI and an on-demand Environment-local Engine path. Neither profile is required by Core.

Current plugin capabilities include usage sampling/recommendation and image deletion. A Local Registry is optional future infrastructure, not the default data path.

## Seed and storage direction

Planned v0.19 uses trusted Host acquisition and an offline immutable Seed Builder. Multiple Environments never share one writable `/var/lib/containerd`; Incus/storage-driver clone semantics provide physical COW sharing when available.

```text
trusted Host OCI acquisition
  -> offline Seed Builder
  -> immutable Incus Seed
  -> normal clone
       -> Env A independent state
       -> Env B independent state
```

## EC2

EC2 remains experimental/default-off. Disabling it must prevent provider construction from causing AWS credential lookup or AWS API activity. Real AWS acceptance is separate from fake-provider/repository testing.

## Security invariant

Do not shortcut the Host boundary by injecting broad reusable credentials or control sockets into an Environment. In particular, do not pass Host SSH/AWS/registry credential stores, Incus control sockets, Hacocoon control state or Host Docker sockets merely for convenience.

## Historical note

Older planning documents/commits may call Local OCI Registry “v0.13” or Seed work “v0.13A/B/C”. Those labels are historical. The current authoritative sequence is v0.13 Managed Network, v0.14 Git Fetch, v0.15 Recommendation, v0.16 Deletion, v0.17 Docker Compatibility, v0.18 Optional Registry and v0.19 Seed/COW.

## Breaking changes

Before 1.0, rename/delete/replace/CLI/state/adapter changes are allowed when they simplify the architecture or improve safety. Compatibility freedom does not permit silent data loss, ambiguous ownership or security-boundary regression.
