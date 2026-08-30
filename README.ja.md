<div align="center">

# Hacocoon

### AI は中で自由に。Host の権限は外で守る。

**Hacocoon は、人間・開発ツール・コーディングエージェント向けの OSS Secure Workspace Runtime です。**

[English](README.md) · [ドキュメント](docs/README.ja.md) · [セキュリティ](docs/00B_SECURITY_ARCHITECTURE.ja.md) · [実装状況](docs/IMPLEMENTATION_STATUS.ja.md) · [ロードマップ](docs/00_REBASELINE_AND_ROADMAP.ja.md)

[![CI](https://github.com/SLktEx/Hacocoon/actions/workflows/test.yml/badge.svg)](https://github.com/SLktEx/Hacocoon/actions/workflows/test.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

</div>

Hacocoon は Workspace を隔離された実行境界の中に置き、特権 authority を trusted Host 側に残します。

Coding Agent は disposable な Environment の中で package install、source edit、build、test、debug、破壊的変更まで自由に行えます。一方で、それだけを理由に Host credential、Incus 管理 authority、無制限の外部アクセス、自分自身の resource limit を引き上げる権限までは与えません。

```text
VS Code / Shell / Coding Agent / Orchestrator
                     |
                     v
                 Workspace
                     |
                     v
          +-----------------------+
          |       Hacocoon        |
          |                       |
          | isolated Environment  |
          | resource budgets      |
          | workspace leases      |
          | policy / approvals    |
          | capabilities / audit  |
          +-----------+-----------+
                      |
             Environment provider
              /                \
       Incus (default)      EC2 (experimental)
```

> [!WARNING]
> **Hacocoon はまだ pre-1.0 で、Breaking Change は今後も発生します。**
>
> Roadmap は現在 **v0.12 まで連続して実装済み**ですが、Incus、Windows/WSL、VS Code Agent Host、Base image、resource enforcement、AWS の一部は real environment での acceptance がまだ pending です。正確な状態は [実装状況](docs/IMPLEMENTATION_STATUS.ja.md) を参照してください。

## なぜ Hacocoon?

Coding Agent は、source を編集し、dependency を入れ、test を走らせ、server を起動し、自分の失敗から復旧できるくらい自由な方が役に立ちます。

でも、その自由と **Host まで自由に触れる authority** は別物です。

Hacocoon はそこを分離します。

- **Environment 内では広く自由** — Tool / Agent は普通の開発作業を行えます。
- **外部 authority は狭くする** — Host や外部サービスへの特権操作は明示的な Capability / Policy を通します。
- **Credential は Host 側に置く** — 長期 credential を便利さのために Environment へ mount する前提にしません。
- **Approval を監査できる** — Sensitive operation の decision と event を Host 側に残します。
- **Resource ceiling を Host が持つ** — CPU / memory / PID / root storage の上限を Provider 側で enforce できます。
- **既存 UI を使う** — 最初の convenience client は VS Code。Hacocoon 専用 AI UI は必須ではありません。

## Quick Start

### 1. Source から build

```bash
git clone https://github.com/SLktEx/Hacocoon.git
cd Hacocoon

go build -o ./bin/haco ./cmd/haco
go build -o ./bin/haco-vscode ./cmd/haco-vscode
go build -o ./bin/haco-agent-host ./cmd/haco-agent-host

./bin/haco doctor
```

Windows + WSL host を準備する場合は [Windows / WSL bootstrap](docs/WINDOWS_WSL_BOOTSTRAP.ja.md) から始めてください。

### 2. Workspace を隔離環境で実行

```bash
./bin/haco run --workspace "$PWD" -- go test ./...
```

Environment を明示的に管理する場合:

```bash
./bin/haco create --workspace "$PWD" dev
./bin/haco exec dev -- uname -a
./bin/haco shell dev
./bin/haco status dev
./bin/haco delete dev
```

### 3. VS Code で開く

```bash
./bin/haco-vscode open .
```

Hacocoon が Environment を作成または再利用し、loopback-oriented SSH、adapter-owned SSH config を準備して、通常の VS Code Remote-SSH で `/workspace` を開きます。

```text
Workspace
   |
   v
Hacocoon Environment
   |
loopback SSH alias
   |
   v
VS Code Remote-SSH
```

削除:

```bash
./bin/haco-vscode delete .
```

## できること

| 領域 | Hacocoon が提供するもの |
|---|---|
| **Isolation** | Incus を local default とした Provider-backed Environment |
| **Workspace safety** | Canonical Workspace identity と persisted write lease |
| **Execution** | `create` / `exec` / `shell` / `run` / lifecycle / recovery |
| **Interactive access** | Loopback-oriented SSH、forwarding、VS Code Remote-SSH integration |
| **Agent isolation** | Persisted ownership proof を使う per-agent Environment broker |
| **Policy** | Fail-closed な Host-side policy と explicit approval |
| **Capabilities** | Sandbox に広い Host credential を渡さず narrow privileged operation を実行 |
| **Git / GitHub** | Privileged Git push を Core ではなく plugin capability として提供 |
| **Base images** | Provider-neutral logical Base を immutable revision に解決 |
| **Resource limits** | CPU / memory / PID / Environment root storage budget |
| **Audit** | Lifecycle / Capability / Approval / Recovery-sensitive operation の event |
| **Providers** | Incus が default、EC2 は explicit opt-in の experimental provider |

## AI Agent: 中では自由、外へ出る時は仲介

```text
VS Code AI / Codex / Copilot / Claude / other agent
                         |
                         v
                 Incus Environment
                  broad local freedom
               within ResourceBudget
                         |
              ---- trust boundary ----
                         |
                     Hacocoon
              Policy / Capability / Audit
                         |
                 GitHub / AWS / Host
```

Agent は sandbox の中で強い権限を持てますが、**sandbox 自体を管理する authority** にはなりません。

Source edit / build / test / debug をするためだけに、Coding Agent 自身へ `haco` や Incus の管理 credential を渡す必要はありません。

## VS Code と Agent Host

VS Code は最初の supported convenience client ですが、Core dependency ではありません。

通常の interactive development:

```bash
haco-vscode open .
```

Trusted integration が opaque な外部 agent session を専用 Environment に結びつける場合:

```bash
haco-agent-host prepare --session <opaque-id> [workspace]
haco-agent-host release --session <opaque-id>
```

```text
VS Code Agents window
        |
    Remote SSH
        |
Hacocoon-managed alias
        |
 haco-agent-host
        |
 dedicated Environment
        |
    /workspace
```

SSH private key は Client 側に保持します。Hacocoon は Environment allocation と安全な接続準備を担当し、外部 Client が Agent Host / Agent Host Protocol の挙動を所有します。

詳しくは [Per-Agent Sandbox](docs/09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md) と [VS Code Remote Agent Host Adapter](docs/10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.ja.md) を参照してください。

## Base image と再現性

Environment 作成時に logical Base を選択できます。

```bash
haco image list
haco image inspect haco/ubuntu-26.04
haco create --base haco/ubuntu-26.04 --workspace "$PWD" dev
```

Incus provider では mutable source を validated immutable fingerprint に解決してから作成し、その revision を Environment に保存します。

```text
my-dev -> revision A -> Environment 1
my-dev -> revision B -> Environment 2

Environment 1 は revision A のまま。
```

詳しくは [Base Images](docs/BASE_IMAGES.ja.md) を参照してください。

## Resource Budget

Resource ceiling は Environment 作成時に明示できます。

```bash
haco create \
  --cpu 4 \
  --memory 8GiB \
  --pids 1024 \
  --root-size 40GiB \
  --workspace . dev

haco run --cpu 2 --memory 4GiB --workspace . -- go test ./...
```

Finite limit は **enforce されるか reject されるか** のどちらかです。Provider が requested finite budget を黙って無視することは許しません。

詳しくは [Sandbox Resource Limits](docs/12_v0.12_SANDBOX_RESOURCE_LIMITS.ja.md) を参照してください。

## 特権 Git push は plugin

Git push は意図的に Core CLI surface の外に置いています。

```bash
haco plugin git push ...
```

Plugin であっても Host-side Policy / Capability boundary を通ります。`haco plugin` 配下に移したから trusted-by-default になるわけでも、Environment に Host credential を渡すわけでもありません。

## Hacocoon がやらないこと

Hacocoon は意図的に次のものを自前で持ちません。

- IDE / AI chat UI
- Git worktree manager
- Agent scheduler / DAG engine
- Model router / retry engine
- Development review queue
- Model budget manager

これらは Hacocoon の上に置き、Hacocoon を execution / security boundary として利用できます。

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

Trusted Host が Hacocoon state、Policy、Credential、Resource ceiling、privileged Capability execution を所有します。

主なルール:

- 長期 Host credential を便利さのために Environment へ mount しない;
- privileged external action は narrow Capability を通す;
- Policy evaluation は fail closed;
- Capability request / decision を audit できる;
- Workspace write access は persisted lease で守る;
- local exposure は loopback-oriented を default にする;
- Provider-specific / Client-specific concept を Core に混ぜない;
- Custom Base content が Host-side authority を得ることはない;
- requested finite resource limit を silent ignore しない;
- cleanup / recovery failure を成功扱いに変換しない。

Security-sensitive な変更の前に [Security Architecture](docs/00B_SECURITY_ARCHITECTURE.ja.md) を読んでください。

## 現在の成熟度

Roadmap は **v0.12 まで連続して実装済み**です。

`v0.1 Runtime` → `v0.2 Workspace & Lease` → `v0.3 Access` → `v0.4 Policy & Capability` → `v0.5 Git/GitHub` → `v0.6 Agent Integration` → `v0.7 EC2` → `v0.8 VS Code` → `v0.9 Per-Agent Sandbox` → `v0.10 Agent Host Adapter` → `v0.11 Base Images` → `v0.12 Resource Limits`

ただし「実装済み」と「全 real provider / client で production acceptance 済み」は同義ではありません。正確な状態は以下を参照してください。

- [実装状況](docs/IMPLEMENTATION_STATUS.ja.md)
- [Versioning / Release status](docs/00D_VERSIONING_AND_RELEASE_STATUS.ja.md)
- [ドキュメント一覧](docs/README.ja.md)

## CLI 一覧

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
haco plugin git push
haco capability request
haco run
haco events
haco doctor

haco-vscode open
haco-vscode delete

haco-agent-host prepare
haco-agent-host release
```

すべて pre-1.0 の surface であり、今後変更される可能性があります。

## Experimental EC2 provider

EC2 provider は **experimental / disabled by default** です。

```bash
export HACO_RUNTIME_PROVIDER=runtime.ec2
export HACO_EXPERIMENTAL_EC2=1
```

両方の explicit opt-in が必要です。Opt-in されていない状態では real provider construction や AWS call より前に fail する必要があります。

Real AWS / EC2 / SSM / EBS acceptance は別途 tracking 中で、v0.12 finite ResourceBudget も equivalent enforcement が証明できていないため AWS-side creation より前に reject します。

## 開発

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/haco
go build ./cmd/haco-vscode
go build ./cmd/haco-agent-host
python tools/check_docs.py
```

External infrastructure が必要な acceptance は、実際にその Provider / Client 上で走っていない限り pass と扱いません。

## ドキュメント

まずはここから:

- [ドキュメント一覧](docs/README.ja.md)
- [Architecture Guide](docs/ARCHITECTURE_GUIDE.ja.md)
- [Security Architecture](docs/00B_SECURITY_ARCHITECTURE.ja.md)
- [Windows / WSL bootstrap](docs/WINDOWS_WSL_BOOTSTRAP.ja.md)
- [Base Images](docs/BASE_IMAGES.ja.md)
- [実装状況](docs/IMPLEMENTATION_STATUS.ja.md)
- [Release Security](docs/RELEASE_SECURITY.ja.md)

## License

Hacocoon は [Apache License 2.0](LICENSE) で公開します。

## Breaking Change 方針

Stable compatibility milestone に到達するまでは、**どの revision 間でも Breaking Change が起こり得ます**。

Roadmap gate や command が実装済みであることだけを根拠に、compatibility guarantee、production support、real-host acceptance があるとはみなさないでください。
