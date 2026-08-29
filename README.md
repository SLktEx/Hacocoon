# Hacocoon

**Pronounced: ha-kōn**

Hacocoon is an OSS **secure workspace runtime** for humans, developer tools, and coding agents.

It takes an existing workspace, places it behind an isolated execution boundary, and provides a small host-side control plane for environment lifecycle, execution, access, policy, approvals, capabilities, and audit.

> [!WARNING]
> **Hacocoon is pre-1.0 and under active development. Breaking changes are expected.**
>
> CLI behavior, state formats, APIs, capability contracts, provider interfaces, and configuration may change incompatibly. Pin the version or commit you depend on and review changes before upgrading.

## What Hacocoon is

Hacocoon is intentionally not an IDE, Git worktree manager, or AI-agent scheduler. Those tools remain outside Hacocoon and can use it as an execution and security boundary.

```text
VS Code / Shell / coding agents / orchestrators / other clients
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

## Current state

`main` currently contains the implementation progression through the **v0.7 roadmap**. That describes repository implementation state, not a promise of release or API stability.

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

Real-provider acceptance is deliberately tracked separately. Real Incus host acceptance and real AWS / EC2 / SSM / EBS acceptance still require suitable external environments; unit, integration, fake-provider E2E, race, vet, build, and CI results are not substitutes for those checks.

See [`docs/IMPLEMENTATION_STATUS.md`](docs/IMPLEMENTATION_STATUS.md) for detailed repository reality and [`docs/README.md`](docs/README.md) for documentation precedence.

## Quick start

### Requirements

For the default local runtime:

- Go **1.26**
- a working Incus installation usable by the current user

Build Hacocoon from source:

```bash
git clone https://github.com/SLktEx/Hacocoon.git
cd Hacocoon

go build -o ./bin/haco ./cmd/haco
./bin/haco doctor
```

Create an Environment from a workspace and execute inside it:

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

For one-shot tool or agent execution, `haco run` creates the execution path from a workspace directly:

```bash
./bin/haco run --workspace "$PWD" -- go test ./...
```

Machine consumers can request JSON output:

```bash
./bin/haco run --workspace "$PWD" --json -- go test ./...
```

## CLI surface

The current CLI includes:

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

The CLI is still pre-1.0. Command names, flags, output, and semantics may change as the architecture is refined.

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
- provider-specific concepts stay outside the Core domain;
- cleanup and recovery failures are surfaced instead of silently converted into success.

Security still depends on the host, provider, and deployment configuration. Hacocoon does not turn an incorrectly configured Incus or cloud environment into a safe one by itself.

Read [`docs/00B_SECURITY_ARCHITECTURE.md`](docs/00B_SECURITY_ARCHITECTURE.md) before extending security-sensitive behavior.

## Experimental EC2 provider

The EC2 provider exists for v0.7 experimentation, but it is **not enabled by selecting AWS credentials or installing the AWS CLI**.

Both of these settings are currently required:

```bash
export HACO_RUNTIME_PROVIDER=runtime.ec2
export HACO_EXPERIMENTAL_EC2=1
```

Without the explicit experimental opt-in, Hacocoon must fail before constructing the real EC2 provider or making AWS calls.

The current remote path uses S3 staging and SSM. Real AWS acceptance remains pending; see [`docs/REMOTE_CLOUD_PROVISIONING.md`](docs/REMOTE_CLOUD_PROVISIONING.md).

## Design boundaries

Hacocoon keeps Core deliberately small.

Core does not own:

- IDE/editor UX;
- Git branch or worktree orchestration;
- model selection;
- agent DAGs and retries;
- model/token budgets;
- provider-specific storage mechanics;
- provider-specific cloud APIs.

Concrete integrations such as Incus, Git/GitHub, AWS/EC2/EBS, VS Code, or external orchestrators live at explicit boundaries around the common Workspace / Environment / Execution model.

## Development and testing

Run the maintained local checks with:

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/haco
python tools/check_docs.py
```

Some integration and acceptance paths require external infrastructure and are intentionally not represented as passing unless they actually ran against that provider.

## Roadmap documents

The v0.1-v0.7 documents are **versioned design contracts**, not promises that their public interfaces are frozen:

1. [`v0.1 Secure Workspace Runtime`](docs/01_v0.1_SECURE_WORKSPACE_RUNTIME.md)
2. [`v0.2 Workspace Abstraction & Lease`](docs/02_v0.2_WORKSPACE_ABSTRACTION_AND_LEASE.md)
3. [`v0.3 Client & Interactive Access`](docs/03_v0.3_CLIENT_AND_INTERACTIVE_ACCESS.md)
4. [`v0.4 Policy & Capability Foundation`](docs/04_v0.4_POLICY_AND_CAPABILITY_FOUNDATION.md)
5. [`v0.5 Git / GitHub Capability`](docs/05_v0.5_GIT_AND_GITHUB_CAPABILITY.md)
6. [`v0.6 Agent & Orchestrator Integration`](docs/06_v0.6_AGENT_AND_ORCHESTRATOR_INTEGRATION.md)
7. [`v0.7 Remote / Cloud Runtime & External Capabilities`](docs/07_v0.7_REMOTE_AND_CLOUD_RUNTIME.md)

For architecture and documentation rules, start with [`docs/README.md`](docs/README.md) and [`docs/00_REBASELINE_AND_ROADMAP.md`](docs/00_REBASELINE_AND_ROADMAP.md).

## Compatibility policy

Until Hacocoon reaches a stable compatibility milestone, assume that **breaking changes can happen between any revisions**.

That freedom is intentional: the project is still hardening its boundaries and may delete, rename, replace, or redesign behavior when doing so produces a smaller and safer system.

Do not infer compatibility guarantees from an implemented roadmap version, an existing command, or persisted state merely because it exists on `main` today.
