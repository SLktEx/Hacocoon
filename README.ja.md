# Hacocoon

**読み方: はこーん**

[English](README.md) | **日本語**

Hacocoon は、人間・開発ツール・コーディングエージェント向けの OSS **Secure Workspace Runtime（安全なワークスペース実行基盤）**です。

既存の Workspace を隔離された Environment に置き、Environment lifecycle、実行、接続、Policy、承認、Capability、監査を Host 側で管理します。

> [!WARNING]
> **Hacocoon はまだ pre-1.0 で、Breaking Change は今後も発生します。**
>
> CLI、補助バイナリ、state format、API、Capability、Provider、Client Adapter、Base/image 設定、roadmap 番号は互換性なく変わる可能性があります。

## いちばんやりたい使い方

```text
VS Code
  |
  | haco-vscode
  v
Hacocoon
  |
  | loopback-only SSH
  v
Incus Environment
  |
  +-- /workspace
  +-- Terminal
  +-- Build / Test / Debug
  +-- VS Code 上の AI / Coding Agent
```

**AI 専用 UI を Hacocoon に作りません。** VS Code にある Copilot / Codex / Claude 等の UI や拡張をそのまま使います。

AI は隔離 Environment の中ではかなり自由に動かせます。一方、GitHub、AWS、Host credential など Environment の外の authority は Hacocoon の Policy / Capability / Audit を通します。

```text
Coding Agent
    |
    v
Incus Environment       <- YOLO zone
    |
---- trust boundary ----
    |
 Hacocoon
 Policy / Capability
    |
GitHub / AWS / Host
```

## 現在のバージョン

実装済み milestone は **v0.1〜v0.9 まで連番**に整理しています。

| Version | Gate | 状態 |
|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | 実装済み |
| v0.2 | Workspace Abstraction & Lease | 実装済み |
| v0.3 | Client & Interactive Access | 実装済み |
| v0.4 | Policy & Capability Foundation | 実装済み |
| v0.5 | Git / GitHub Capability | 実装済み |
| v0.6 | Agent & Orchestrator Integration | 実装済み |
| v0.7 | Remote / Cloud Runtime | experimental 実装済み |
| v0.8 | Client Adapters & VS Code | 実装済み |
| v0.9 | Per-Agent Sandbox & Agent Host Integration | broker foundation 実装済み |
| v0.10 | VS Code Remote Agent Host Adapter | PR #111 で実装中。まだ `main` ではない |
| v0.11 | Base Images & Custom Environments | 設計のみ |
| v0.12 | Sandbox Resource Limits | 設計のみ |

以前は design-only v0.9 の後に implemented v0.10 が来るなど番号が飛び飛びでしたが、pre-1.0 のうちに **実装済み → 実装中 → 設計待ち** の順へ整理しました。

詳しくは [`docs/00D_VERSIONING_AND_RELEASE_STATUS.ja.md`](docs/00D_VERSIONING_AND_RELEASE_STATUS.ja.md) と [`docs/IMPLEMENTATION_STATUS.ja.md`](docs/IMPLEMENTATION_STATUS.ja.md) を参照してください。

## VS Code で開く

ソースから使う場合:

```bash
git clone https://github.com/SLktEx/Hacocoon.git
cd Hacocoon

go build -o ./bin/haco ./cmd/haco
go build -o ./bin/haco-vscode ./cmd/haco-vscode
./bin/haco doctor
```

開きたい Workspace から:

```bash
./bin/haco-vscode open .
```

概念的には次を自動化します。

```text
Workspace を認識
  -> Environment を作成または再利用
  -> SSH public key を設定
  -> localhost の SSH connection を用意
  -> Hacocoon 専用 SSH config を生成
  -> VS Code Remote-SSH で /workspace を開く
```

Private key は Client 側に残り、Environment に渡しません。

終了して Environment を捨てる場合:

```bash
./bin/haco-vscode delete .
```

## v0.9: Per-Agent Sandbox

v0.9 は opaque な Agent Session identity を dedicated Environment に結びつける trusted broker foundation です。

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
        |
 Incus
```

Coding agent 自身に `haco` / Incus 管理 authority を渡しません。Deterministic な Environment 名だけでも ownership proof にはせず、persisted binding が一致しなければ Acquire / Release は fail closed します。

Parallel RW agent は原則として別 Git worktree / 別 canonical Workspace を使います。

正本は [`docs/09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md`](docs/09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md) です。

## v0.10: VS Code Remote Agent Host Adapter

v0.10 は PR #111 の active integration candidate です。まだ `main` implementation claim ではありません。

```text
VS Code Agents window
        |
   Remote SSH
        |
Hacocoon-managed loopback SSH alias
        |
v0.9 bound Environment
        |
 /workspace + Agent Host
```

Hacocoon は Environment と安全な接続準備を所有し、VS Code が Agent Host / Agent Host Protocol を所有します。

## v0.11: Base Images & Custom Environments

v0.11 はまだ設計段階です。

```text
logical Base
   -> immutable Base revision
   -> provider-native starting point
   -> Environment
```

logical Base の更新で既存 Environment を silent retarget しません。

```text
my-dev -> revision A -> Environment 1
my-dev -> revision B -> Environment 2
Environment 1 は revision A のまま
```

Custom Base の中身や metadata だけで Host mount、privileged mode、device、credential、external authority を増やすことも禁止します。

想定 CLI は次のような形ですが、**まだ実装済みではありません**。

```text
haco image list
haco image inspect <base>
haco create --base <base> --workspace <path> <environment>
```

正本は [`docs/11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md`](docs/11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md)、詳しい日本語設計は [`docs/BASE_IMAGES.ja.md`](docs/BASE_IMAGES.ja.md) です。

## v0.12: Sandbox Resource Limits

v0.12 は design-only です。

予定する resource budget:

- CPU
- memory
- process / PID count
- Environment root storage size（安全に enforce できる場合）

```text
ResourceBudget  -> Environment 内部の消費量を制限
Capability      -> Environment 境界を越える authority を制御
```

requested limit を provider が enforce できない場合は silent ignore せず fail closed します。

詳しくは [`docs/12_v0.12_SANDBOX_RESOURCE_LIMITS.ja.md`](docs/12_v0.12_SANDBOX_RESOURCE_LIMITS.ja.md) を参照してください。

## Windows + WSL

Hacocoon/Incus が WSL 側、デスクトップ VS Code が Windows 側にある場合、`haco-vscode` は Windows user profile を解決して Windows 側の `.ssh` 設定を対象にします。

Hacocoon は専用 `Hacocoon` WSL 2 instance を使い、systemd を PID 1 として Incus を動かす方向です。普段使いの WSL distribution や global default を勝手に変更せず、`incus-admin` は explicit opt-in のままです。

## Orchestrator との関係

Daintree などの Orchestrator は Hacocoon の上に置きます。

```text
Daintree / Orchestrator
  -> Task / worktree / Agent 管理
  -> Workspace
  -> Hacocoon
  -> isolated Environment
```

Task 分解、worktree、Agent 選択、retry、budget、development review は Orchestrator 側の責任です。

## 低レベル CLI

```bash
haco create --workspace "$PWD" dev
haco exec dev -- go test ./...
haco shell dev
haco status dev
haco delete dev

haco run --workspace "$PWD" -- go test ./...
haco run --workspace "$PWD" --json -- go test ./...
```

現在の主な command:

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

## EC2

v0.7 EC2 provider は **experimental / disabled by default** です。

```bash
export HACO_RUNTIME_PROVIDER=runtime.ec2
export HACO_EXPERIMENTAL_EC2=1
```

両方の explicit opt-in が必要です。Real AWS / EC2 / SSM / EBS acceptance は pending です。

## 開発とテスト

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/haco
go build ./cmd/haco-vscode
python tools/check_docs.py
```

Real Incus、Windows/WSL + VS Code、Agent Host/AHP、Base/image lifecycle、AWS/EC2/SSM/EBS、resource enforcement の acceptance は、実際の対応環境で実行していない限り pass と扱いません。

資料の優先順位は [`docs/README.ja.md`](docs/README.ja.md)、詳しい architecture は [`docs/ARCHITECTURE_GUIDE.ja.md`](docs/ARCHITECTURE_GUIDE.ja.md) を参照してください。

## Breaking Change 方針

Hacocoon が stable compatibility milestone に到達するまでは、**どの revision 間でも Breaking Change が起こり得ます**。

ただし security boundary を弱めたり、silent data loss を許したりするための自由ではありません。より小さく安全な境界へ直すことを優先します。
