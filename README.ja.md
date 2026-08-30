<div align="center">

<img src="docs/assets/readme/hacocoon-logo.webp" alt="Hacocoon — Secure Workspace Runtime" width="520">

# Hacocoon

**読み方: はこーん**

### AI は中で自由に。Host の権限は外で守る。

**人間・developer tool・coding agentのためのOSS Secure Workspace Runtime。**

[English](README.md) · [ドキュメント](docs/README.ja.md) · [Security](docs/00B_SECURITY_ARCHITECTURE.md) · [実装状況](docs/IMPLEMENTATION_STATUS.ja.md) · [Roadmap](docs/00_REBASELINE_AND_ROADMAP.md)

[![CI](https://github.com/SLktEx/Hacocoon/actions/workflows/test.yml/badge.svg)](https://github.com/SLktEx/Hacocoon/actions/workflows/test.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

</div>

<p align="center">
  <img src="docs/assets/readme/hacocoon-hero.webp" alt="Hacocoon secure workspace runtime overview" width="100%">
</p>

HacocoonはWorkspaceをisolated Environmentの内側に置き、HostやGitHub/AWSなどのprivileged authorityをtrusted Host側に残します。

coding agentはEnvironment内でpackage install、source edit、build、test、debug、破壊的操作まで自由にできます。しかし、その自由だけでHost credential、Incus管理権限、外部authority、resource limit変更権限まで得ることはありません。

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
             /                \
      Incus (default)      EC2 (experimental)

optional integrations:
  haco plugin git ...
  haco plugin oci ...
```

> [!WARNING]
> **Hacocoonはpre-1.0で活発に開発中です。Breaking changeがあります。**
>
> product milestoneは **v0.17まで連続して実装済み**。real Incus / Windows/WSL / VS Code Agent Host / AWS / OCI tooling acceptanceにはpending領域があります。詳細は[実装状況](docs/IMPLEMENTATION_STATUS.ja.md)。

## なんでHacocoon？

coding agentを便利にするには自由に作業させたい。一方で、**Environment内の自由**と**Hostの権限**は同じものではありません。

- **中では自由** — build/test/edit/package installなどをisolated Environment内で実行
- **外は狭く** — privileged Host/external operationはPolicy/Capability boundaryを通す
- **credentialはHost側** — long-lived credentialをSandboxへ雑にmountしない
- **approval/audit** — sensitive operationの判断をHost側に残す
- **resource ceiling** — CPU/memory/PID/root storageをProvider側で制限
- **既存UIを使う** — VS Codeを最初のclient adapterとして使い、独自AI chat UIを必須にしない
- **optionalはoptionalのまま** — GitHub Git、nerdctl、Docker固有workflowをCore必須機能にしない

## Quick Start

<p align="center">
  <img src="docs/assets/readme/hacocoon-quickstart.webp" alt="Hacocoon VS Code quick start" width="100%">
</p>

```bash
git clone https://github.com/SLktEx/Hacocoon.git
cd Hacocoon

go build -o ./bin/haco ./cmd/haco
go build -o ./bin/haco-vscode ./cmd/haco-vscode
go build -o ./bin/haco-agent-host ./cmd/haco-agent-host

./bin/haco doctor
```

Windows + WSLは [Windows / WSL bootstrap](docs/WINDOWS_WSL_BOOTSTRAP.md) を参照。

### isolated Workspaceで実行

```bash
./bin/haco run --workspace "$PWD" -- go test ./...
```

またはEnvironmentを明示管理:

```bash
./bin/haco create --workspace "$PWD" dev
./bin/haco exec dev -- uname -a
./bin/haco shell dev
./bin/haco status dev
./bin/haco delete dev
```

### VS Codeで開く

```bash
./bin/haco-vscode open .
```

standard VS Code Remote-SSHを使って`/workspace`へ接続します。cleanupは:

```bash
./bin/haco-vscode delete .
```

## 主な機能

| Area | Hacocoonが提供するもの |
|---|---|
| Isolation | Incusをlocal defaultとするProvider-backed Environment |
| Workspace safety | canonical identity + persisted write lease |
| Execution | `create` / `exec` / `shell` / `run` / lifecycle / recovery |
| Client access | loopback-oriented SSH / forwarding / VS Code Remote-SSH |
| Agent isolation | persisted ownership proof付きper-agent Environment broker |
| Policy/Capability | fail-closed policy / approval / narrow privileged operation |
| Git plugin | Host credentialをSandboxへ渡さないbrokered fetch/push |
| Base images | logical Base → immutable revision |
| Resource limits | CPU / memory / PID / root storage |
| Managed network | Hacocoon-managed Incus sandbox network/profile |
| Optional OCI plugin | nerdctl/Docker inventory、Seed recommendation、image deletion、Docker compatibility foundation |
| Providers | Incus default、EC2はexperimental opt-in |

## VS Code / Per-Agent Sandbox

```bash
haco-vscode open .

haco-agent-host prepare --session <opaque-id> [workspace]
haco-agent-host release --session <opaque-id>
```

private SSH keyはclient側に残します。HacocoonはEnvironment allocationとsafe connection preparationを所有し、VS Code/Agent Host behaviorはclient側の責任です。

## Base image

```bash
haco image list
haco image inspect haco/ubuntu-26.04
haco create --base haco/ubuntu-26.04 --workspace "$PWD" dev
```

Top-level `haco image` は**Hacocoon Base identity**を扱います。workload container image操作とは分離しています。

## Resource / Network

```bash
haco create --cpu 4 --memory 8GiB --pids 1024 --root-size 40GiB --workspace . dev
```

finite limitはenforceするかrejectします。silent ignoreしません。

- **v0.12**: [Sandbox Resource Limits](docs/12_v0.12_SANDBOX_RESOURCE_LIMITS.ja.md)
- **v0.13**: [Managed Sandbox Network](docs/13_v0.13_MANAGED_SANDBOX_NETWORK.ja.md)

## GitHub Gitはplugin

```bash
haco plugin git fetch <environment>
haco plugin git push <environment> --branch feature/x
```

GitHub HTTPSではHostの`gh auth git-credential`をbroker経路から使えます。credential自体はEnvironmentへコピーしません。ordinary Git UXはGit自身の責任です。

- **v0.14**: [Git Fetch Plugin](docs/14_v0.14_GIT_FETCH_PLUGIN.ja.md)

## OCI / Dockerはoptional plugin

Hacocoon Coreはuniversal container runtime/CLIを要求しません。必要なdeploymentだけ有効化します。

```bash
export HACO_PLUGIN_OCI=nerdctl
# または: export HACO_PLUGIN_OCI=docker

haco plugin oci status
haco plugin oci seed sample
haco plugin oci seed recommend
haco plugin oci image delete docker.io/library/node:24@sha256:<digest>
```

project-maintained nerdctl profileはEnvironment-local `containerd + nerdctl`を使えます。Docker profileはgenuine Docker CLIとon-demand Environment-local Docker Engine compatibilityを提供できます。どちらもCore requirementではありません。

- **v0.15**: [OCI Seed Usage & Recommendation](docs/15_v0.15_OCI_SEED_RECOMMENDATION.ja.md)
- **v0.16**: [OCI Image Deletion](docs/16_v0.16_OCI_IMAGE_DELETION.ja.md)
- **v0.17**: [Docker Compatibility](docs/17_v0.17_DOCKER_COMPATIBILITY.ja.md)
- 詳細: [Optional OCI Plugin と Docker Compatibility](docs/OCI_RUNTIME_AND_DOCKER_COMPAT.ja.md)

## 次のmilestone

- **v0.18**: [Optional Local OCI Registry](docs/18_v0.18_OPTIONAL_LOCAL_OCI_REGISTRY.ja.md) — 必要なinstallationだけ使うoptional infrastructure
- **v0.19**: [OCI Seed Builder & Btrfs/COW](docs/19_v0.19_OCI_SEED_AND_COW.ja.md) — offline immutable Seed build/publish。writable containerd stateをEnvironment間共有しない

## Hacocoonがやらないこと

HacocoonはIDE、AI chat UI、Git worktree manager、agent scheduler、DAG engine、model router、retry engine、container CLI/runtime managerにはなりません。そうしたtoolはHacocoonの上や横からWorkspace/Environment boundaryを利用できます。

## Security model

- long-lived Host credentialをEnvironmentへ便利だからという理由でmountしない
- privileged external actionはnarrow Capabilityを通す
- Policyはfail closed
- Workspace write accessはpersisted leaseで守る
- local exposureはloopback-oriented
- Provider/client/tool-specific conceptをCoreへ持ち込まない
- requested finite limitをsilent ignoreしない
- Host Docker/Incus/Hacocoon control socketをEnvironmentへshortcutで渡さない
- cleanup/recovery failureを成功扱いにしない

security-sensitiveな変更前に [Security Architecture](docs/00B_SECURITY_ARCHITECTURE.md) を読んでください。

## 現在のmaturity

`v0.1 Runtime` → `v0.2 Workspace & Lease` → `v0.3 Access` → `v0.4 Policy & Capability` → `v0.5 Git Push` → `v0.6 Agent` → `v0.7 EC2` → `v0.8 VS Code` → `v0.9 Per-Agent` → `v0.10 Agent Host` → `v0.11 Base` → `v0.12 Resource` → `v0.13 Network` → `v0.14 Git Fetch` → `v0.15 OCI Recommendation` → `v0.16 OCI Delete` → `v0.17 Docker Compatibility` → `v0.18 Registry (planned)` → `v0.19 Seed/COW (planned)`

正本:

- [実装状況](docs/IMPLEMENTATION_STATUS.ja.md)
- [Versioning](docs/00D_VERSIONING_AND_RELEASE_STATUS.ja.md)
- [ドキュメントindex](docs/README.ja.md)

## CLI一覧

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
haco plugin git fetch
haco plugin git push
haco plugin oci status
haco plugin oci seed sample
haco plugin oci seed recommend
haco plugin oci image delete
haco capability request
haco run
haco events
haco doctor

haco-vscode open / delete
haco-agent-host prepare / release
```

## Experimental EC2

```bash
export HACO_RUNTIME_PROVIDER=runtime.ec2
export HACO_EXPERIMENTAL_EC2=1
```

EC2はexperimental/default-off。real AWS acceptanceは別に追跡します。

## Development

```bash
bash tools/ci-local.sh
```

個別jobは [CONTRIBUTING.md](CONTRIBUTING.md) を参照。

## License

[Apache License 2.0](LICENSE)
