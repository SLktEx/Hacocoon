<div align="center">

<img src="docs/assets/readme/hacocoon-logo.webp" alt="Hacocoon — Secure Workspace Runtime" width="520">

# Hacocoon

**読み方: はこーん**

### AI は中で自由に。Host の権限は外で守る。

**人間・開発ツール・コーディングエージェント向けの OSS Secure Workspace Runtime。**

[English](README.md) · [ドキュメント](docs/README.ja.md) · [セキュリティ](docs/00B_SECURITY_ARCHITECTURE.ja.md) · [実装状況](docs/IMPLEMENTATION_STATUS.ja.md) · [ロードマップ](docs/00_REBASELINE_AND_ROADMAP.ja.md)

[![CI](https://github.com/SLktEx/Hacocoon/actions/workflows/test.yml/badge.svg)](https://github.com/SLktEx/Hacocoon/actions/workflows/test.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

</div>

<p align="center">
  <img src="docs/assets/readme/hacocoon-hero.webp" alt="Hacocoon の Secure Workspace Runtime 全体像。AI は中で自由に、Host authority は外で守る" width="100%">
</p>

Hacocoon は Workspace を隔離された実行境界の中に置き、特権 authority を trusted Host 側に残します。

Coding Agent は disposable な Environment の中で package install、source edit、build、test、debug、破壊的変更まで自由に行えます。一方で、それだけを理由に Host credential、Incus 管理 authority、無制限の外部アクセス、自分自身の resource limit を引き上げる権限までは与えません。

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
                     |
              Incus (current)
```

> [!WARNING]
> **Hacocoon はまだ pre-1.0 で、Breaking Change は今後も発生します。**
>
> 現在は Core と Provider contract がまだ大きく変わる段階なので、Runtime 実装は local Incus に集中します。以前の experimental EC2/AWS/EBS 実装は current tree から外し、local foundation が十分安定して cloud acceptance を意味のある形で試せるようになるまで deferred とします。正確な状態は [実装状況](docs/IMPLEMENTATION_STATUS.ja.md) を参照してください。

## なぜ Hacocoon?

Coding Agent は、source を編集し、dependency を入れ、test を走らせ、server を起動し、自分の失敗から復旧できるくらい自由な方が役に立ちます。

でも、**Environment 内での自由** と **Host を触れる authority** は別物です。

Hacocoon はそこを分離します。

- **Environment 内では広く自由** — Tool / Agent は普通の開発作業を行えます。
- **外部 authority は狭くする** — Host や外部サービスへの特権操作は明示的な Capability / Policy を通します。
- **Credential は Host 側に置く** — 長期 credential を便利さのために Environment へ mount する前提にしません。
- **Approval を監査できる** — Sensitive operation の decision と event を Host 側に残します。
- **Resource ceiling を Host が持つ** — CPU / memory / PID / root storage の上限を Provider 側で enforce できます。
- **既存 UI を使う** — 最初の convenience client は VS Code。Hacocoon 専用 AI UI は必須ではありません。

## Quick Start

<p align="center">
  <img src="docs/assets/readme/hacocoon-quickstart.webp" alt="Hacocoon の VS Code Quick Start。Workspace を開き、隔離 Environment を作成または再利用し、loopback-only Remote-SSH で接続して test を実行する" width="100%">
</p>

### Build

```bash
git clone https://github.com/SLktEx/Hacocoon.git
cd Hacocoon

go build -o ./bin/haco ./cmd/haco
go build -o ./bin/haco-vscode ./cmd/haco-vscode
go build -o ./bin/haco-agent-host ./cmd/haco-agent-host

./bin/haco doctor
```

Windows + WSL host を準備する場合は [Windows / WSL bootstrap](docs/WINDOWS_WSL_BOOTSTRAP.ja.md) から始めてください。

### Workspace を隔離環境で実行

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

### VS Code で開く

```bash
./bin/haco-vscode open .
```

Hacocoon が Environment を作成または再利用し、loopback-oriented SSH と adapter-owned SSH config を準備して、通常の VS Code Remote-SSH で `/workspace` を開きます。

```text
Workspace -> Hacocoon Environment -> loopback SSH alias -> VS Code Remote-SSH
```

削除:

```bash
./bin/haco-vscode delete .
```

## できること

| 領域 | Hacocoon が提供するもの |
|---|---|
| **Isolation** | 現在は Incus を使う Provider-backed Environment |
| **Workspace safety** | Canonical Workspace identity と persisted write lease |
| **Execution** | `create` / `exec` / `shell` / `run` / lifecycle / recovery |
| **Interactive access** | Loopback-oriented SSH、forwarding、VS Code Remote-SSH integration |
| **Agent isolation** | Persisted ownership proof を使う per-agent Environment broker |
| **Policy** | Fail-closed な Host-side Policy と explicit approval |
| **Capabilities** | Sandbox に広い Host credential を渡さず narrow privileged operation を実行 |
| **Git / GitHub** | Privileged Git push を plugin capability として提供 |
| **Base images** | Provider-neutral logical Base を immutable revision に解決 |
| **OCI tooling** | `haco plugin oci` 配下の optional containerd/nerdctl Seed telemetry / image lifecycle |
| **Resource limits** | CPU / memory / PID / Environment root storage budget |
| **Audit** | Lifecycle / Capability / Approval / Recovery-sensitive operation の event |
| **Providers** | 現在有効なのは Incus。将来の adapter 用 Provider seam は維持 |

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
             GitHub / external services / Host
```

Agent は sandbox の中で強い権限を持てますが、**sandbox 自体を管理する authority** にはなりません。Source edit / build / test / debug のためだけに、Coding Agent 自身へ `haco` や Incus の管理 credential を渡す必要はありません。

## VS Code と per-agent sandbox

通常の interactive development:

```bash
haco-vscode open .
```

Trusted integration が opaque な外部 agent session を専用 Environment に結びつける場合:

```bash
haco-agent-host prepare --session <opaque-id> [workspace]
haco-agent-host release --session <opaque-id>
```

SSH private key は Client 側に保持します。Hacocoon は Environment allocation と安全な接続準備を担当し、外部 Client が Agent Host の挙動を所有します。

- **v0.9**: [Per-Agent Sandbox & Agent Host](docs/09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md)
- **v0.10**: [VS Code Remote Agent Host Adapter](docs/10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.ja.md)

## Base image

Environment 作成時に logical Base を選択できます。

```bash
haco base list
haco base inspect haco/ubuntu-26.04
haco create --base haco/ubuntu-26.04 --workspace "$PWD" dev
```

`haco base` は Hacocoon Environment の starting point 専用です。OCI/container image はここに混ぜず、`haco plugin oci` 配下に置きます。

Incus Provider では mutable source を validated immutable fingerprint に解決してから作成し、その revision を Environment に保存します。

```text
my-dev -> revision A -> Environment 1
my-dev -> revision B -> Environment 2
Environment 1 は revision A のまま
```

- **v0.11**: [Base Images & Custom Environments](docs/11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md)
- 詳細: [Base Images](docs/BASE_IMAGES.ja.md)

## OCI plugin

OCI/containerd/nerdctl 固有の操作は optional OCI plugin surface にまとめます。

```bash
haco plugin oci seed sample
haco plugin oci seed recommend
haco plugin oci image delete docker.io/library/node:24
```

これで container image lifecycle と Hacocoon/Incus Base image lifecycle を混同しません。

## Resource Budget

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

- **v0.12**: [Sandbox Resource Limits](docs/12_v0.12_SANDBOX_RESOURCE_LIMITS.ja.md)

## 特権 Git push は plugin

Git push は意図的に Core CLI surface の外に置いています。

```bash
haco plugin git push ...
```

Plugin であっても Host-side Policy / Capability boundary を通ります。`haco plugin` 配下にあるから trusted-by-default になるわけでも、Environment に Host credential を渡すわけでもありません。

## Hacocoon がやらないこと

Hacocoon は IDE、AI chat UI、Git worktree manager、Agent scheduler、DAG engine、model router、retry engine、development review queue、model budget manager を自前で持つことを目的にしません。

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

`v0.1 Runtime` → `v0.2 Workspace & Lease` → `v0.3 Access` → `v0.4 Policy & Capability` → `v0.5 Git/GitHub` → `v0.6 Agent Integration` → `v0.7 Provider Routing` → `v0.8 VS Code` → `v0.9 Per-Agent Sandbox` → `v0.10 Agent Host Adapter` → `v0.11 Base Images` → `v0.12 Resource Limits`

以前の v0.7 EC2/AWS/EBS 実装は意図的に **deferred** とし、現在の implementation tree には含めません。将来 local runtime / Provider contract が落ち着いた時に戻せるよう、Git history と設計資料は参照用として残します。

「実装済み」と「全 real Provider / Client で production acceptance 済み」は同義ではありません。

- [実装状況](docs/IMPLEMENTATION_STATUS.ja.md)
- [Versioning / Release status](docs/00D_VERSIONING_AND_RELEASE_STATUS.ja.md)
- [ドキュメント一覧](docs/README.ja.md)

## `haco` CLI

`haco` は Host 側で動く管理 CLI です。普段使う lifecycle は小さく直接的に保ち、tool / provider 固有の操作は Core の top-level command を増やすのではなく plugin namespace の下へ分離します。

Environment 内で code edit / build / test / debug をするだけなら、Coding Agent 自身に `haco` や Incus の管理 authority を持たせる必要はありません。

### 普段の使い方

```bash
# Local runtime の確認
haco doctor

# One-shot: 一時 Environment を作って command 実行後に cleanup
haco run --workspace "$PWD" -- go test ./...

# Environment を残して作業する場合
haco create --workspace "$PWD" dev
haco exec dev -- go test ./...
haco shell dev
haco status dev
haco delete dev
```

### Command map

| 領域 | Command | 用途 |
|---|---|---|
| **Environment** | `create`, `exec`, `shell`, `status`, `delete` | Named Environment の作成と操作 |
| **One-shot execution** | `run` | Workspace を隔離した一回限りの command lifecycle |
| **Base selection** | `base list`, `base inspect` | Hacocoon Environment の starting point を確認 |
| **Local access** | `connections`, `forward`, `unforward`, `ssh` | Host 管理の loopback access / port forwarding |
| **Policy / audit** | `capability request`, `events` | Host authority boundary を明示的に越える操作と audit event の確認 |
| **Git plugin** | `plugin git fetch`, `plugin git push` | Host-side credential を使う mediated Git operation |
| **OCI plugin** | `plugin oci seed ...`, `plugin oci image ...` | Optional containerd/nerdctl image cache / Seed operation |
| **Diagnostics** | `doctor` | Local runtime が利用可能か確認 |

### 主な command form

```text
haco create [--read-only] [--base <base>] [resource flags] --workspace <path> <environment>
haco exec <environment> -- <command...>
haco shell <environment>
haco status <environment> [--json]
haco delete <environment>

haco run [--read-only] [resource flags] --workspace <path> [--json] -- <command...>

haco base list [--json]
haco base inspect <base> [--json]

haco connections <environment> [--json]
haco forward <environment> --host-port <port> --target-port <port>
haco unforward <environment> <connection-id>
haco ssh <environment> --public-key <path> --host-port <port>

haco plugin git fetch <environment> [--remote <remote>]
haco plugin git push <environment> --branch <branch> [--source <ref>] [--remote <remote>] [--force]

haco plugin oci seed sample [--json]
haco plugin oci seed recommend [--json]
haco plugin oci image delete <reference[@sha256:...]> [--all-environments] [--json]

haco capability request <capability> <action> [--resource <resource>] [--environment <environment>] [--param <key=value>]...
haco events [--json] [--since-offset <byte-offset>]
haco doctor
```

`create` と `run` で共通する resource flag は `--cpu`、`--memory`、`--pids`、`--root-size` です。それぞれ finite value または `unlimited` を指定できます。

Convenience client は Core CLI と分離します。

```text
haco-vscode open
haco-vscode delete

haco-agent-host prepare
haco-agent-host release
```

すべて pre-1.0 の surface であり、今後変更される可能性があります。

## Remote / Cloud Runtime

Remote / Cloud の Environment Provider は **いったん deferred** とします。現在の build が登録する Environment Provider は Incus のみで、以前のEC2 runtime、AWS capability、EBS helperはactive implementation treeに含まれません。

Hacocoon の Core、state、resource、networking、image、client contract がまだ頻繁に変わる段階で cloud-specific implementation を追従させても、real cloudで十分に試せない間はmaintenance costだけが増えるためです。Provider routing の汎用 seam は残しているので、local runtime が落ち着いた後に cloud adapter を Core へ混ぜず復活できます。

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

## ドキュメント

- [ドキュメント一覧](docs/README.ja.md)
- [Architecture Guide](docs/ARCHITECTURE_GUIDE.ja.md)
- [Security Architecture](docs/00B_SECURITY_ARCHITECTURE.ja.md)
- [Windows / WSL bootstrap](docs/WINDOWS_WSL_BOOTSTRAP.ja.md)
- [実装状況](docs/IMPLEMENTATION_STATUS.ja.md)
- [Release Security](docs/RELEASE_SECURITY.ja.md)

## License

Hacocoon は [Apache License 2.0](LICENSE) で公開します。

## Breaking Change 方針

Stable compatibility milestone に到達するまでは、**どの revision 間でも Breaking Change が起こり得ます**。
