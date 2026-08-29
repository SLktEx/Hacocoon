# Hacocoon

**Pronounced: ha-kōn**

[**日本語**](README.ja.md) | English

Hacocoon is an OSS **secure workspace runtime** for humans, developer tools, and coding agents.

It takes an existing workspace, places it behind an isolated execution boundary, and provides a small host-side control plane for environment lifecycle, execution, access, policy, approvals, capabilities, and audit.

> [!WARNING]
> **Hacocoon is pre-1.0 and under active development. Breaking changes are expected.**
>
> CLI behavior, helper binaries, state formats, APIs, capability contracts, provider interfaces, and configuration may change incompatibly. Pin the version or commit you depend on and review changes before upgrading.

## What Hacocoon is

Hacocoon is intentionally not an IDE, Git worktree manager, AI-agent scheduler, or AI chat product. Those tools remain outside Hacocoon and can use it as an execution and security boundary.

```text
VS Code / Shell / coding agents / orchestrators / other clients
                              |
                    optional Client Adapter
                              |
                         Workspace
                              |
                              v
                    +-------------------+
                    |     Hacocoon      |
                    |                   |
                    | Environment       |
                    | execution         |
                    | policy / approval |
                    | capabilities      |
                    | audit             |
                    +---------+---------+
                              |
                   Environment provider
                              |
              +---------------+---------------+
              |                               |
       runtime.incus                  runtime.ec2
        local default              experimental only
```

The trusted host owns Hacocoon state, policy, credentials, and privileged capability execution. The Environment receives only the workspace and the authority it actually needs.

The intended interactive-development model is simple: a normal client such as VS Code connects to the isolated Environment, and coding agents can be intentionally permissive inside that Environment. Crossing the Environment boundary still requires Hacocoon-mediated authority.

## Current state

`main` contains the implementation progression through the **v0.8 roadmap**. That describes repository implementation state, not a promise of release or API stability.

| Area | Current state |
|---|---|
| Secure Workspace Runtime | Environment create / exec / shell / delete implemented |
| Workspace model | canonical Workspace identity plus persisted RO/RW leases implemented |
| Lease safety | RW conflict prevention, stale-lease recovery, and process serialization implemented |
| Local runtime | Incus is the default Environment provider |
| Client access | status, loopback port forwarding, connection management, and SSH preparation/revocation implemented |
| Policy / Capability | fail-closed allow / deny / require-approval flow with audit implemented |
| Git / GitHub | host-side brokered push implemented without exporting host credentials into the Environment |
| Agent / orchestrator access | `haco run`, machine-readable JSON output, and security event export implemented |
| Runtime routing | provider-neutral Environment routing implemented |
| EC2 runtime | implemented as an **experimental, disabled-by-default** provider |
| AWS capability | narrow host-side brokered read capability implemented |
| EBS replacement | replacement/migration flow implemented; no in-place shrink and no automatic source-volume deletion |
| Client adapters | thin adapter layer introduced without adding client-specific concepts to Core |
| VS Code integration | `haco-vscode` prepares standard Remote-SSH access and opens `/workspace` |
| Windows + WSL bridge | WSL execution resolves the Windows desktop client's SSH profile/configuration |

Real-provider/client acceptance is deliberately tracked separately. Real Incus host acceptance, real Windows/WSL + VS Code Remote-SSH acceptance, and real AWS / EC2 / SSM / EBS acceptance require suitable external environments; unit, integration, fake-provider E2E, race, vet, build, and CI results are not substitutes for those checks.

See [`docs/IMPLEMENTATION_STATUS.md`](docs/IMPLEMENTATION_STATUS.md) for detailed repository reality and [`docs/README.md`](docs/README.md) for documentation precedence.

## VS Code: the intended interactive workflow

VS Code is the first supported convenience client, not a Core dependency.

Build both binaries from source:

```bash
git clone https://github.com/SLktEx/Hacocoon.git
cd Hacocoon

go build -o ./bin/haco ./cmd/haco
go build -o ./bin/haco-vscode ./cmd/haco-vscode
./bin/haco doctor
```

Requirements for the default path:

- Go **1.26** when building from source;
- a working Incus installation usable by the current user;
- OpenSSH client support;
- VS Code with Remote-SSH installed;
- an SSH key pair, by default `~/.ssh/id_ed25519` and `~/.ssh/id_ed25519.pub` on the VS Code client side.

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

Once VS Code reconnects, use normal VS Code features inside the Environment:

```text
Terminal
Git UI
Debugger
Tests
Copilot / Codex / Claude / other coding-agent UI
```

Hacocoon does not provide a second AI UI. The IDE/agent owns the conversation and task workflow; Hacocoon owns the isolated execution and authority boundary.

Useful adapter options:

```bash
./bin/haco-vscode open --name dev .
./bin/haco-vscode open --identity /path/to/id_ed25519 .
./bin/haco-vscode open --read-only .
./bin/haco-vscode open --no-launch .
```

Cleanup:

```bash
./bin/haco-vscode delete .
```

The adapter keeps unrelated SSH configuration intact. It adds a single include when needed and stores Hacocoon-owned host entries under `~/.ssh/hacocoon/`.

When Hacocoon runs inside WSL and desktop VS Code runs on Windows, `haco-vscode` targets the Windows user's SSH configuration rather than only the WSL user's Linux SSH config.

See [`docs/08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md`](docs/08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md) for the v0.8 contract.

## AI agents: permissive inside, mediated outside

The motivating model is:

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

Package installation, builds, tests, source edits, and even destructive changes can be intentionally permissive inside a disposable Environment.

That does **not** grant the agent host credentials or broad external authority. GitHub, AWS, host access, and other privileged operations remain behind explicit Hacocoon capabilities and policy.

## Low-level CLI quick start

The lower-level `haco` CLI remains useful for scripting, debugging, adapters, and non-VS-Code clients.

Create an Environment and execute inside it:

```bash
./bin/haco create --workspace "$PWD" dev
./bin/haco exec dev -- uname -a
./bin/haco status dev
./bin/haco shell dev
./bin/haco delete dev
```

Use a read-only workspace lease when mutation is not required:

```bash
./bin/haco create --read-only --workspace "$PWD" review
```

For one-shot tool or agent execution:

```bash
./bin/haco run --workspace "$PWD" -- go test ./...
```

Machine consumers can request JSON output:

```bash
./bin/haco run --workspace "$PWD" --json -- go test ./...
```

## CLI surface

The current low-level CLI includes:

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
```

The first client helper includes:

```text
haco-vscode open
haco-vscode delete
```

All surfaces remain pre-1.0 and may change.

## External orchestrators

Hacocoon deliberately does not own worktree orchestration, agent DAGs, model selection, retries, budgets, or development-review queues.

A tool such as Daintree can sit above Hacocoon:

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

The same generic Environment/client boundary can support future thin adapters for JetBrains, web clients, Daintree, or other tools without making those products Core dependencies.

## Security model

Hacocoon treats the host and the execution Environment as different trust domains.

Core rules include:

- long-lived host credentials are not mounted into Environments for convenience;
- privileged external actions go through narrow capabilities instead of exporting broad credentials;
- policy evaluation fails closed;
- human approval is a security boundary, not an orchestration engine;
- capability requests and decisions are auditable;
- workspace write access is protected by persisted leases;
- local port exposure is loopback-oriented by default;
- provider-specific and client-specific concepts stay outside the Core domain;
- cleanup and recovery failures are surfaced instead of silently converted into success.

Security still depends on the host, provider, client, and deployment configuration. Hacocoon does not turn an incorrectly configured Incus, SSH, VS Code, or cloud environment into a safe one by itself.

Read [`docs/00B_SECURITY_ARCHITECTURE.md`](docs/00B_SECURITY_ARCHITECTURE.md) before extending security-sensitive behavior.

## Experimental EC2 provider

The EC2 provider exists for v0.7 experimentation, but it is **not enabled by selecting AWS credentials or installing the AWS CLI**.

Both settings are currently required:

```bash
export HACO_RUNTIME_PROVIDER=runtime.ec2
export HACO_EXPERIMENTAL_EC2=1
```

Without the explicit experimental opt-in, Hacocoon must fail before constructing the real EC2 provider or making AWS calls.

The current remote path uses S3 staging and SSM. Real AWS acceptance remains pending; see [`docs/REMOTE_CLOUD_PROVISIONING.md`](docs/REMOTE_CLOUD_PROVISIONING.md).

## Design boundaries

Hacocoon keeps Core deliberately small.

Core does not own:

- IDE/editor/chat UX;
- Git branch or worktree orchestration;
- model selection;
- agent DAGs and retries;
- model/token budgets;
- provider-specific storage mechanics;
- provider-specific cloud APIs;
- client-native SSH configuration or launch behavior.

Concrete integrations such as Incus, Git/GitHub, AWS/EC2/EBS, VS Code, Daintree, or external orchestrators live at explicit boundaries around the common Workspace / Environment / Execution model.

## Development and testing

Run the maintained local checks with:

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/haco
go build ./cmd/haco-vscode
python tools/check_docs.py
```

Some integration and acceptance paths require external infrastructure and are intentionally not represented as passing unless they actually ran against that provider/client.

## Roadmap documents

The v0.1-v0.8 documents are **versioned design contracts**, not promises that their public interfaces are frozen:

1. [`v0.1 Secure Workspace Runtime`](docs/01_v0.1_SECURE_WORKSPACE_RUNTIME.md)
2. [`v0.2 Workspace Abstraction & Lease`](docs/02_v0.2_WORKSPACE_ABSTRACTION_AND_LEASE.md)
3. [`v0.3 Client & Interactive Access`](docs/03_v0.3_CLIENT_AND_INTERACTIVE_ACCESS.md)
4. [`v0.4 Policy & Capability Foundation`](docs/04_v0.4_POLICY_AND_CAPABILITY_FOUNDATION.md)
5. [`v0.5 Git / GitHub Capability`](docs/05_v0.5_GIT_AND_GITHUB_CAPABILITY.md)
6. [`v0.6 Agent & Orchestrator Integration`](docs/06_v0.6_AGENT_AND_ORCHESTRATOR_INTEGRATION.md)
7. [`v0.7 Remote / Cloud Runtime & External Capabilities`](docs/07_v0.7_REMOTE_AND_CLOUD_RUNTIME.md)
8. [`v0.8 Client Adapters & VS Code Integration`](docs/08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md)

For architecture and documentation rules, start with [`docs/README.md`](docs/README.md) and [`docs/00_REBASELINE_AND_ROADMAP.md`](docs/00_REBASELINE_AND_ROADMAP.md).

## Compatibility policy

Until Hacocoon reaches a stable compatibility milestone, assume that **breaking changes can happen between any revisions**.

That freedom is intentional: the project is still hardening its boundaries and may delete, rename, replace, or redesign behavior when doing so produces a smaller and safer system.

Do not infer compatibility guarantees from an implemented roadmap version, an existing command, helper binary, or persisted state merely because it exists on `main` today.
