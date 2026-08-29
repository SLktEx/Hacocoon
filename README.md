# Hacocoon

**Pronounced: ha-kōn**

[**日本語**](README.ja.md) | English

Hacocoon is an OSS **Secure Workspace Runtime** for humans, developer tools, and coding agents.

It places an existing Workspace inside an isolated Environment and keeps environment lifecycle, execution, client access, policy, approvals, capabilities, and audit on the trusted host side.

> [!WARNING]
> **Hacocoon is pre-1.0 and under active development. Breaking changes are expected.**
>
> CLI behavior, helper binaries, persisted state, APIs, providers, Base/image configuration, Client Adapters, and agent integration may change incompatibly.

## What Hacocoon owns

```text
VS Code / Shell / coding agents / orchestrators / other clients
                              |
                    thin/trusted integration
                              |
                         Workspace
                              v
                    +-------------------+
                    |     Hacocoon      |
                    | Environment       |
                    | execution/access  |
                    | policy/approval   |
                    | capabilities      |
                    | audit             |
                    +---------+---------+
                              |
                   Environment provider
                       /              \
              runtime.incus      runtime.ec2
               local default     experimental
```

Hacocoon is intentionally **not** an IDE, Git worktree manager, model router, agent scheduler, or AI chat product. Coding agents can be permissive inside an isolated Environment while GitHub, AWS, host credentials, and other external authority remain mediated outside it.

## Current state

The repository currently contains:

- **v0.1-v0.8 implementation progression** — Secure Workspace Runtime, leases, client access, policy/capabilities, Git/GitHub, orchestrator access, experimental cloud runtime, and VS Code Client Adapter.
- **v0.9 Base Images & Custom Environments** — design contract only; implementation is still pending.
- **v0.10 Per-Agent Sandbox** — implemented trusted session-to-Environment binding with persisted ownership proof.
- **v0.11 VS Code Remote Agent Host Adapter** — implemented `haco-agent-host` adapter that prepares a v0.10 Environment as a standard remote-SSH target for the VS Code Agents window.

Implementation presence and real-provider/client acceptance are separate claims. Real Incus, Windows/WSL + VS Code, current VS Code Agents-window/Agent-Host behavior, and AWS acceptance require suitable external environments.

See [`docs/IMPLEMENTATION_STATUS.md`](docs/IMPLEMENTATION_STATUS.md) for current code reality and [`docs/README.md`](docs/README.md) for documentation precedence.

## Normal VS Code workflow — v0.8

Build from source:

```bash
git clone https://github.com/SLktEx/Hacocoon.git
cd Hacocoon

go build -o ./bin/haco ./cmd/haco
go build -o ./bin/haco-vscode ./cmd/haco-vscode
go build -o ./bin/haco-agent-host ./cmd/haco-agent-host
./bin/haco doctor
```

Open a Workspace with standard VS Code Remote-SSH:

```bash
./bin/haco-vscode open .
```

Conceptually:

```text
Workspace
  -> Hacocoon Environment
  -> loopback-only SSH
  -> VS Code Remote-SSH
  -> /workspace
```

Cleanup:

```bash
./bin/haco-vscode delete .
```

Hacocoon does not replace the VS Code editor, terminal, debugger, Git UI, or coding-agent UI.

See [`docs/08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md`](docs/08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md).

## Base images — v0.9 design contract

v0.9 defines a provider-neutral **Base** as the immutable starting point of a newly created Environment:

```text
logical Base name
       -> immutable Base revision
       -> provider-native starting point
       -> Environment
```

For Incus, provider-native image aliases/remotes/fingerprints remain adapter details. Updating a logical Base affects future Environments only; already-created Environments remain bound to their original revision.

The expected CLI shape includes ideas such as:

```text
haco image list
haco image inspect <base>
haco create --base <base> --workspace <path> <environment>
```

These commands are **not implemented or frozen yet**.

See [`docs/09_v0.9_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md`](docs/09_v0.9_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md) and [`docs/BASE_IMAGES.md`](docs/BASE_IMAGES.md).

## Per-agent sandbox — v0.10

v0.10 adds trusted session ownership outside Core:

```text
opaque trusted session ID
          |
          v
 internal/agenthost Broker
          |
 persisted ownership proof
          |
 dedicated Environment
```

Important properties:

- exact reacquire is idempotent;
- rebinding the same session to another Workspace/access mode fails closed;
- raw external session IDs are not persisted or used directly as runtime names;
- a deterministic Environment name alone is not ownership proof;
- release requires the persisted binding and cannot delete an arbitrary Environment;
- coding agents do not receive the Incus socket or Hacocoon management authority.

Parallel write-capable sessions should normally use separate Git worktrees/Workspace paths.

See [`docs/10_v0.10_PER_AGENT_SANDBOX_AND_AGENT_HOST.md`](docs/10_v0.10_PER_AGENT_SANDBOX_AND_AGENT_HOST.md).

## VS Code Agents window — v0.11

Prepare one isolated Hacocoon remote slot:

```bash
haco-agent-host prepare --session task-a /path/to/worktree-a
```

The adapter creates/reuses the v0.10 binding, prepares loopback-only SSH, writes a hashed Hacocoon SSH alias, and opens the VS Code Agents window unless `--no-launch` is supplied.

Then choose the printed target in the Agents window:

```text
New -> Remote -> SSH -> haco-agent-...
```

VS Code owns its remote CLI, Agent Host, and Agent Host Protocol (AHP). Hacocoon intentionally does not reimplement AHP.

To prepare another independent write-capable slot:

```bash
haco-agent-host prepare --session task-b /path/to/worktree-b
```

Explicit cleanup:

```bash
haco-agent-host release --session task-a
```

The isolation unit is **one Hacocoon `--session` slot = one v0.10 Environment**. Hacocoon does not currently receive VS Code's internal top-level agent-session UUID automatically. Multiple VS Code sessions deliberately created through the same prepared SSH alias may therefore share one Environment.

The client SSH private key remains client-side; only the public key enters the existing Hacocoon SSH-access path.

See [`docs/11_v0.11_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md`](docs/11_v0.11_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md).

## Windows + WSL

The intended Windows layout is:

```text
Windows VS Code / OpenSSH
        |
   127.0.0.1:<port>
        |
 dedicated Hacocoon WSL 2
        |
      systemd
        |
      Incus
```

Bootstrap uses a dedicated `Hacocoon` WSL 2 instance, requires systemd as PID 1, leaves unrelated WSL distributions/global defaults alone, and does not silently grant `incus-admin`.

When adapters run under WSL for a Windows desktop VS Code client, they target the Windows user's SSH configuration rather than WSL-only SSH configuration.

## Low-level CLI and trusted automation

The lower-level `haco` CLI remains available for humans, debugging, trusted automation, and other clients:

```bash
haco create --workspace "$PWD" dev
haco exec dev -- go test ./...
haco shell dev
haco status dev
haco delete dev
```

Trusted one-shot execution:

```bash
haco run --workspace "$PWD" -- go test ./...
haco run --workspace "$PWD" --json -- go test ./...
```

Current helper surfaces include:

```text
haco-vscode open
haco-vscode delete

haco-agent-host prepare
haco-agent-host release
```

## External orchestrators

Daintree and similar systems remain above Hacocoon:

```text
Orchestrator
  -> task / model / worktree / retry / budget ownership
  -> Workspace
  -> Hacocoon Environment
```

Hacocoon owns the secure execution and external-authority boundary, not the orchestration policy.

## Experimental EC2 provider

EC2 remains **experimental and disabled by default**. Both settings are required:

```bash
export HACO_RUNTIME_PROVIDER=runtime.ec2
export HACO_EXPERIMENTAL_EC2=1
```

Real AWS / EC2 / SSM / EBS acceptance remains separate from fake-provider and repository CI.

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

Release CI also validates workflow trust boundaries and GoReleaser packaging for `haco`, `haco-vscode`, and `haco-agent-host`.

## Roadmap documents

The v0.1-v0.11 documents are **versioned design contracts**, not compatibility promises:

1. [`v0.1 Secure Workspace Runtime`](docs/01_v0.1_SECURE_WORKSPACE_RUNTIME.md)
2. [`v0.2 Workspace Abstraction & Lease`](docs/02_v0.2_WORKSPACE_ABSTRACTION_AND_LEASE.md)
3. [`v0.3 Client & Interactive Access`](docs/03_v0.3_CLIENT_AND_INTERACTIVE_ACCESS.md)
4. [`v0.4 Policy & Capability Foundation`](docs/04_v0.4_POLICY_AND_CAPABILITY_FOUNDATION.md)
5. [`v0.5 Git / GitHub Capability`](docs/05_v0.5_GIT_AND_GITHUB_CAPABILITY.md)
6. [`v0.6 Agent & Orchestrator Integration`](docs/06_v0.6_AGENT_AND_ORCHESTRATOR_INTEGRATION.md)
7. [`v0.7 Remote / Cloud Runtime & External Capabilities`](docs/07_v0.7_REMOTE_AND_CLOUD_RUNTIME.md)
8. [`v0.8 Client Adapters & VS Code Integration`](docs/08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md)
9. [`v0.9 Base Images & Custom Environments`](docs/09_v0.9_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md)
10. [`v0.10 Per-Agent Sandbox & Agent Host Integration`](docs/10_v0.10_PER_AGENT_SANDBOX_AND_AGENT_HOST.md)
11. [`v0.11 VS Code Remote Agent Host Adapter`](docs/11_v0.11_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md)

## Compatibility policy

Until Hacocoon reaches an explicit stable compatibility milestone, assume **Breaking Change** can happen between revisions. Prefer smaller, safer boundaries over preserving accidental behavior that leaks authority, makes ownership ambiguous, or creates unsafe cleanup semantics.
