# Hacocoon

**Pronounced: ha-kōn**

[**日本語**](README.ja.md) | English

Hacocoon is an OSS **secure workspace runtime** for humans, developer tools, and coding agents.

It takes an existing Workspace, places it behind an isolated Environment boundary, and provides a small host-side control plane for lifecycle, execution, access, policy, approvals, capabilities, audit, client integration, reproducible Base selection, and resource budgets.

> [!WARNING]
> **Hacocoon is pre-1.0 and under active development. Breaking changes are expected.**
>
> CLI behavior, helper binaries, state formats, APIs, capability contracts, provider interfaces, Base/image configuration, resource-budget behavior, client configuration, and roadmap numbering may still change incompatibly.

## What Hacocoon is

Hacocoon is intentionally not an IDE, Git worktree manager, AI-agent scheduler, or AI chat product. Those tools remain outside Hacocoon and can use it as an execution and security boundary.

```text
VS Code / Shell / coding agents / orchestrators / other clients
                              |
                    optional Client Adapter
                              |
                         Workspace
                              v
                    +-------------------+
                    |     Hacocoon      |
                    | Environment       |
                    | execution         |
                    | policy / approval |
                    | capabilities      |
                    | audit             |
                    +---------+---------+
                              |
                   Environment provider
                    /                 \
            runtime.incus         runtime.ec2
            local default       experimental only
```

The trusted host owns Hacocoon state, policy, credentials, resource ceilings, and privileged capability execution. The Environment receives only the Workspace and authority it actually needs.

## Current state

The implemented roadmap is now contiguous through **v0.12**.

| Version | Gate | Current state |
|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | implemented |
| v0.2 | Workspace Abstraction & Lease | implemented |
| v0.3 | Client & Interactive Access | implemented |
| v0.4 | Policy & Capability Foundation | implemented |
| v0.5 | Git / GitHub Capability | implemented |
| v0.6 | Agent & Orchestrator Integration | implemented |
| v0.7 | Remote / Cloud Runtime & External Capabilities | experimentally implemented; real AWS acceptance pending |
| v0.8 | Client Adapters & VS Code Integration | implemented; real client acceptance pending |
| v0.9 | Per-Agent Sandbox & Agent Host Integration | broker foundation implemented |
| v0.10 | VS Code Remote Agent Host Adapter | implemented; real Agent Host acceptance pending |
| v0.11 | Base Images & Custom Environments | first implementation slice implemented; richer image lifecycle pending |
| v0.12 | Sandbox Resource Limits | first implementation slice implemented; real Incus enforcement acceptance pending |

Real-provider/client acceptance is tracked separately. Real Incus, Windows/WSL + VS Code, Agent Host/AHP routing, real Base/image sources, real resource enforcement, and AWS/EC2/SSM/EBS require suitable external environments; repository CI is not a substitute for those checks.

See [`docs/IMPLEMENTATION_STATUS.md`](docs/IMPLEMENTATION_STATUS.md), [`docs/00D_VERSIONING_AND_RELEASE_STATUS.md`](docs/00D_VERSIONING_AND_RELEASE_STATUS.md), and [`docs/README.md`](docs/README.md).

## VS Code interactive workflow

VS Code is the first supported convenience client, not a Core dependency.

Build from source:

```bash
git clone https://github.com/SLktEx/Hacocoon.git
cd Hacocoon

go build -o ./bin/haco ./cmd/haco
go build -o ./bin/haco-vscode ./cmd/haco-vscode
go build -o ./bin/haco-agent-host ./cmd/haco-agent-host
./bin/haco doctor
```

Open a Workspace with the normal Remote-SSH adapter:

```bash
./bin/haco-vscode open .
```

```text
Workspace
  -> create/reuse Hacocoon Environment
  -> prepare loopback-only SSH access
  -> write adapter-owned SSH host entry
  -> code --remote ssh-remote+<alias> /workspace
```

Cleanup:

```bash
./bin/haco-vscode delete .
```

See [`docs/08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md`](docs/08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md).

## v0.9: per-agent sandbox broker

v0.9 binds an opaque trusted external agent-session identity to a dedicated Environment.

```text
trusted client / integration
        |
 opaque session identity
        |
 internal/agenthost broker
        |
 persisted ownership proof
        |
 Environment
```

Coding agents do not receive Hacocoon/Incus management authority. Raw external session IDs are not used directly as runtime names, reacquire is idempotent only for the same binding, and release requires persisted ownership proof.

Parallel read/write agents normally use separate Git worktrees / canonical Workspace paths.

See [`docs/09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md`](docs/09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md).

## v0.10: VS Code Remote Agent Host Adapter

v0.10 is implemented by the separate `haco-agent-host` helper:

```bash
haco-agent-host prepare --session <opaque-id> [workspace]
haco-agent-host release --session <opaque-id>
```

```text
VS Code Agents window
        |
    Remote SSH
        |
Hacocoon-managed loopback alias
        |
  haco-agent-host
        |
 v0.9-bound Environment
        |
    /workspace
```

The SSH private key stays with the client. Hacocoon owns Environment allocation and safe connection preparation; VS Code owns Agent Host / Agent Host Protocol behavior.

See [`docs/10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md`](docs/10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md).

## v0.11: selectable Base images

v0.11 implements the first provider-neutral Base-selection slice:

```bash
haco image list
haco image inspect <base>
haco create --base <base> --workspace <path> <environment>
```

```text
logical Base name
        |
 provider-owned source
        |
 resolve once at create
        v
immutable Base revision
        |
        v
Environment
```

For the Incus provider, a mutable alias/source is resolved to a validated immutable fingerprint before `incus init`. The resulting `BaseRef` is persisted on the Environment, so moving the logical Base later affects future Environment creation only.

```text
my-dev -> revision A -> Environment 1
my-dev -> revision B -> Environment 2
Environment 1 remains recorded on revision A
```

Host/operator custom logical mappings can currently be supplied through `HACO_INCUS_BASES_JSON`; the `haco/` namespace is reserved. Custom image build/import, history, rollback, physical deletion, and GC are intentionally not part of this first slice.

See [`docs/11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md`](docs/11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md) and [`docs/BASE_IMAGES.md`](docs/BASE_IMAGES.md).

## v0.12: sandbox resource limits

v0.12 implements explicit creation-time resource budgets:

```bash
haco create \
  --cpu 4 \
  --memory 8GiB \
  --pids 1024 \
  --root-size 40GiB \
  --workspace . dev

haco run --cpu 2 --memory 4GiB --workspace . -- go test ./...
```

The provider-neutral budget tracks CPU, memory bytes, process/PID count, and Environment root-storage bytes. Omitted dimensions are resolved by Hacocoon to an explicit `unlimited` effective value and persisted with the Environment instead of silently inheriting an unspecified provider default.

For Incus, finite limits are applied and read back **before the Environment starts**. A failed apply or verification aborts creation and follows normal cleanup/recovery handling. Providers that cannot enforce a requested finite limit must fail closed rather than silently ignore it; the experimental EC2 provider currently rejects finite budgets before AWS-side creation activity.

Byte-sized CLI values use explicit binary units such as `512MiB`, `8GiB`, or `40GiB` (or `unlimited`). Live resizing, aggregate host scheduling, and host Workspace quota management are not part of this first slice.

See [`docs/12_v0.12_SANDBOX_RESOURCE_LIMITS.md`](docs/12_v0.12_SANDBOX_RESOURCE_LIMITS.md).

## AI agents: permissive inside, mediated outside

```text
VS Code AI / Codex / Copilot / Claude / other agent
                         |
                         v
                 Incus Environment
                 broad local freedom
                 within resource budget
                         |
              ---- trust boundary ----
                         |
                     Hacocoon
             Policy / Capability / Audit
                         |
              GitHub / AWS / Host / etc.
```

Package installation, builds, tests, source edits, and destructive changes can be intentionally permissive inside a disposable Environment. That does **not** grant the agent host credentials, broad external authority, or authority to raise its own host-enforced resource ceiling.

## Low-level CLI quick start

```bash
./bin/haco create --workspace "$PWD" dev
./bin/haco exec dev -- uname -a
./bin/haco status dev
./bin/haco shell dev
./bin/haco delete dev

./bin/haco run --workspace "$PWD" -- go test ./...
```

Base selection:

```bash
./bin/haco image list
./bin/haco image inspect haco/ubuntu-26.04
./bin/haco create --base haco/ubuntu-26.04 --workspace "$PWD" dev
```

Resource-limited creation:

```bash
./bin/haco create --cpu 4 --memory 8GiB --pids 1024 --root-size 40GiB --workspace "$PWD" dev
```

## Current CLI surface

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
haco git push
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

## External orchestrators

Hacocoon deliberately does not own worktree orchestration, agent DAGs, model selection, retries, development-review queues, or model budgets.

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

Core rules include:

- long-lived host credentials are not mounted into Environments for convenience;
- privileged external actions go through narrow capabilities;
- policy evaluation fails closed;
- capability requests and decisions are auditable;
- Workspace write access is protected by persisted leases;
- local port exposure is loopback-oriented by default;
- provider-specific and client-specific concepts stay outside Core;
- custom Base/image contents do not grant host-side authority;
- requested finite resource limits are not silently ignored;
- cleanup and recovery failures are surfaced instead of silently converted into success.

Read [`docs/00B_SECURITY_ARCHITECTURE.md`](docs/00B_SECURITY_ARCHITECTURE.md) before extending security-sensitive behavior.

## Experimental EC2 provider

The EC2 provider remains **experimental and disabled by default**.

Both settings are required:

```bash
export HACO_RUNTIME_PROVIDER=runtime.ec2
export HACO_EXPERIMENTAL_EC2=1
```

Without explicit opt-in, Hacocoon must fail before constructing the real EC2 provider or making AWS calls. In the current v0.12 slice, a finite resource budget on EC2 is also rejected before provider creation because that path does not yet prove equivalent enforcement.

## Development and testing

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/haco
go build ./cmd/haco-vscode
go build ./cmd/haco-agent-host
python tools/check_docs.py
```

Some acceptance paths require external infrastructure and are intentionally not represented as passing unless they actually ran against that provider/client.

## Roadmap documents

1. [`v0.1 Secure Workspace Runtime`](docs/01_v0.1_SECURE_WORKSPACE_RUNTIME.md)
2. [`v0.2 Workspace Abstraction & Lease`](docs/02_v0.2_WORKSPACE_ABSTRACTION_AND_LEASE.md)
3. [`v0.3 Client & Interactive Access`](docs/03_v0.3_CLIENT_AND_INTERACTIVE_ACCESS.md)
4. [`v0.4 Policy & Capability Foundation`](docs/04_v0.4_POLICY_AND_CAPABILITY_FOUNDATION.md)
5. [`v0.5 Git / GitHub Capability`](docs/05_v0.5_GIT_AND_GITHUB_CAPABILITY.md)
6. [`v0.6 Agent & Orchestrator Integration`](docs/06_v0.6_AGENT_AND_ORCHESTRATOR_INTEGRATION.md)
7. [`v0.7 Remote / Cloud Runtime`](docs/07_v0.7_REMOTE_AND_CLOUD_RUNTIME.md)
8. [`v0.8 Client Adapters & VS Code`](docs/08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md)
9. [`v0.9 Per-Agent Sandbox & Agent Host`](docs/09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md)
10. [`v0.10 VS Code Remote Agent Host Adapter`](docs/10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md)
11. [`v0.11 Base Images & Custom Environments`](docs/11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md)
12. [`v0.12 Sandbox Resource Limits`](docs/12_v0.12_SANDBOX_RESOURCE_LIMITS.md)

For architecture and documentation rules, start with [`docs/README.md`](docs/README.md) and [`docs/00_REBASELINE_AND_ROADMAP.md`](docs/00_REBASELINE_AND_ROADMAP.md).

## Compatibility policy

Until Hacocoon reaches a stable compatibility milestone, assume that **breaking changes can happen between any revisions**.

Do not infer compatibility guarantees, production support, or real-host acceptance merely because a roadmap gate or command is implemented today.
