# Hacocoon

**Pronounced: ha-kōn**

[**日本語**](README.ja.md) | English

Hacocoon is an OSS **secure workspace runtime** for humans, developer tools, and coding agents.

It takes an existing workspace, places it behind an isolated execution boundary, and provides a small host-side control plane for Environment lifecycle, execution, access, policy, approvals, capabilities, and audit.

> [!WARNING]
> **Hacocoon is pre-1.0 and under active development. Breaking changes are expected.**
>
> CLI behavior, helper binaries, state formats, APIs, capability contracts, provider interfaces, Base/image configuration, client configuration, and roadmap numbering may still change incompatibly.

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

The trusted host owns Hacocoon state, policy, credentials, and privileged capability execution. The Environment receives only the workspace and the authority it actually needs.

## Current state

The implemented roadmap is now contiguous through **v0.9**.

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
| v0.10 | VS Code Remote Agent Host Adapter | active integration candidate in PR #111; not on `main` yet |
| v0.11 | Base Images & Custom Environments | design only; implementation pending |
| v0.12 | Sandbox Resource Limits | design only; implementation pending |

The numbering was intentionally cleaned up while Hacocoon is pre-1.0: implementation-pending design gates no longer sit in front of already-implemented independent milestones. See [`docs/00D_VERSIONING_AND_RELEASE_STATUS.md`](docs/00D_VERSIONING_AND_RELEASE_STATUS.md).

Real-provider/client acceptance is deliberately tracked separately. Real Incus host acceptance, real Windows/WSL + VS Code acceptance, Agent Host/AHP routing, and real AWS / EC2 / SSM / EBS acceptance require suitable external environments; unit, integration, fake-provider E2E, race, vet, build, and CI results are not substitutes for those checks.

See [`docs/IMPLEMENTATION_STATUS.md`](docs/IMPLEMENTATION_STATUS.md) for detailed repository reality and [`docs/README.md`](docs/README.md) for documentation precedence.

## VS Code: intended interactive workflow

VS Code is the first supported convenience client, not a Core dependency.

Build from source:

```bash
git clone https://github.com/SLktEx/Hacocoon.git
cd Hacocoon

go build -o ./bin/haco ./cmd/haco
go build -o ./bin/haco-vscode ./cmd/haco-vscode
./bin/haco doctor
```

From the workspace you want to use:

```bash
./bin/haco-vscode open .
```

The adapter performs roughly:

```text
Workspace
  -> create/reuse Hacocoon Environment
  -> prepare loopback-only SSH access
  -> write an adapter-owned SSH host entry
  -> code --remote ssh-remote+<alias> /workspace
```

Once VS Code reconnects, normal VS Code features run against the Environment: terminal, Git UI, debugger, tests, and coding-agent UI. Hacocoon does not provide a second AI UI.

Cleanup:

```bash
./bin/haco-vscode delete .
```

When Hacocoon runs inside WSL and desktop VS Code runs on Windows, `haco-vscode` targets the Windows user's SSH configuration rather than only the WSL user's Linux SSH config.

See [`docs/08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md`](docs/08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md).

## v0.9: per-agent sandbox broker

v0.9 is the implemented trusted bridge from an opaque external agent-session identity to a dedicated Hacocoon Environment.

```text
trusted VS Code / client integration
              |
       opaque session identity
              |
       internal/agenthost broker
              |
       persisted binding proof
              |
          Environment
              |
            Incus
```

Important properties:

- coding agents do not receive Hacocoon/Incus management authority;
- raw external session IDs are not used directly as runtime instance names;
- exact reacquire is idempotent;
- rebinding the same session to a different Workspace/access mode fails closed;
- release requires persisted ownership proof;
- parallel read/write agents normally use separate Git worktrees / canonical Workspace paths.

Real VS Code Agent Host/AHP routing remains a host-dependent acceptance path.

See [`docs/09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md`](docs/09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md).

## v0.10: VS Code Remote Agent Host Adapter

v0.10 is the **active integration candidate** in PR #111. It is not part of current `main` until merged.

The intended path is:

```text
VS Code Agents window
        |
    Remote SSH
        |
Hacocoon-managed loopback alias
        |
 v0.9-bound Environment
        |
    /workspace + Agent Host
```

The private SSH key stays with the client. Hacocoon prepares the Environment and safe connection; VS Code owns Agent Host / Agent Host Protocol behavior.

## v0.11: selectable Base images

v0.11 defines how an Environment may start from a Hacocoon **Base**.

```text
logical Base name
        |
        v
immutable Base revision
        |
        v
provider-native starting point
        |
        v
Environment
```

A logical Base can move to a new revision without changing an existing Environment:

```text
my-dev -> revision A -> Environment 1
my-dev -> revision B -> Environment 2
Environment 1 stays on revision A
```

Custom Base contents are untrusted and cannot grant host mounts, privileged mode, devices, credentials, or external-service authority merely through image metadata.

The likely interaction shape includes commands such as:

```text
haco image list
haco image inspect <base>
haco create --base <base> --workspace <path> <environment>
```

These commands are **planned, not implemented or frozen yet**.

Read [`docs/11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md`](docs/11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md) and [`docs/BASE_IMAGES.md`](docs/BASE_IMAGES.md).

## v0.12: sandbox resource limits

v0.12 defines provider-neutral creation-time budgets for:

- CPU;
- memory;
- process/PID count;
- Environment root storage where the selected provider can enforce it safely.

Resource budgets limit consumption inside the Environment; they are not Capabilities and do not grant authority across the sandbox boundary.

Requested limits that a provider cannot enforce must fail closed rather than be silently ignored.

See [`docs/12_v0.12_SANDBOX_RESOURCE_LIMITS.md`](docs/12_v0.12_SANDBOX_RESOURCE_LIMITS.md).

## AI agents: permissive inside, mediated outside

```text
VS Code AI / Codex / Copilot / Claude / other agent
                         |
                         v
                 Incus Environment
                 broad local freedom
                         |
              ---- trust boundary ----
                         |
                     Hacocoon
             Policy / Capability / Audit
                         |
              GitHub / AWS / Host / etc.
```

Package installation, builds, tests, source edits, and destructive changes can be intentionally permissive inside a disposable Environment.

That does **not** grant the agent host credentials or broad external authority. GitHub, AWS, host access, and other privileged operations remain behind explicit Hacocoon capabilities and policy.

## Low-level CLI quick start

```bash
./bin/haco create --workspace "$PWD" dev
./bin/haco exec dev -- uname -a
./bin/haco status dev
./bin/haco shell dev
./bin/haco delete dev
```

One-shot tool/agent execution:

```bash
./bin/haco run --workspace "$PWD" -- go test ./...
```

Machine consumers can request JSON:

```bash
./bin/haco run --workspace "$PWD" --json -- go test ./...
```

## Current CLI surface

```text
haco create
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
```

All surfaces remain pre-1.0 and may change.

## External orchestrators

Hacocoon deliberately does not own worktree orchestration, agent DAGs, model selection, retries, budgets, or development-review queues.

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
- privileged external actions go through narrow capabilities instead of exporting broad credentials;
- policy evaluation fails closed;
- capability requests and decisions are auditable;
- workspace write access is protected by persisted leases;
- local port exposure is loopback-oriented by default;
- provider-specific and client-specific concepts stay outside Core;
- custom Base/image contents do not grant host-side authority;
- cleanup and recovery failures are surfaced instead of silently converted into success.

Read [`docs/00B_SECURITY_ARCHITECTURE.md`](docs/00B_SECURITY_ARCHITECTURE.md) before extending security-sensitive behavior.

## Experimental EC2 provider

The EC2 provider remains **experimental and disabled by default**.

Both settings are required:

```bash
export HACO_RUNTIME_PROVIDER=runtime.ec2
export HACO_EXPERIMENTAL_EC2=1
```

Without explicit opt-in, Hacocoon must fail before constructing the real EC2 provider or making AWS calls.

## Development and testing

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/haco
go build ./cmd/haco-vscode
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
10. v0.10 VS Code Remote Agent Host Adapter — active PR #111
11. [`v0.11 Base Images & Custom Environments`](docs/11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md)
12. [`v0.12 Sandbox Resource Limits`](docs/12_v0.12_SANDBOX_RESOURCE_LIMITS.md)

For architecture and documentation rules, start with [`docs/README.md`](docs/README.md) and [`docs/00_REBASELINE_AND_ROADMAP.md`](docs/00_REBASELINE_AND_ROADMAP.md).

## Compatibility policy

Until Hacocoon reaches a stable compatibility milestone, assume that **breaking changes can happen between any revisions**.

Do not infer compatibility guarantees or implementation presence from a roadmap version, command, helper binary, or persisted state merely because it appears in documentation or on `main` today.
