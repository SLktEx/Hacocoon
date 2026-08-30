# Hacocoon

**読み方: はこーん**

[English](README.md) | **日本語**

Hacocoon は、人間・開発ツール・コーディングエージェント向けの OSS **Secure Workspace Runtime（安全なワークスペース実行基盤）**です。

既存の Workspace を隔離された Environment に置き、Environment lifecycle、実行、接続、Policy、承認、Capability、監査、Client Adapter、再現可能な Base 選択、Resource Budget を Host 側で管理します。

> [!WARNING]
> **Hacocoon はまだ pre-1.0 で、Breaking Change は今後も発生します。**
>
> CLI、補助バイナリ、state format、API、Capability、Provider、Client Adapter、Base/image 設定、Resource Budget、roadmap 番号は互換性なく変わる可能性があります。

## いちばんやりたい使い方

```text
VS Code / Coding Agent
        |
 optional adapter
        v
    Hacocoon
        |
  Environment
  resource budget
        |
  +-- /workspace
  +-- Terminal
  +-- Build / Test / Debug
```

**AI 専用 UI を Hacocoon に作りません。** VS Code など既存 client の UI を使い、Hacocoon は Environment と外部 authority、host resource ceiling の境界を担当します。

```text
Coding Agent
    |
    v
Environment             <- broad local freedom
within ResourceBudget
    |
---- trust boundary ----
    |
 Hacocoon
 Policy / Capability
    |
GitHub / AWS / Host
```

## 現在のバージョン

実装済み milestone は **v0.1〜v0.12 まで連番**です。

| Version | Gate | 状態 |
|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | 実装済み |
| v0.2 | Workspace Abstraction & Lease | 実装済み |
| v0.3 | Client & Interactive Access | 実装済み |
| v0.4 | Policy & Capability Foundation | 実装済み |
| v0.5 | Git / GitHub Capability | 実装済み |
| v0.6 | Agent & Orchestrator Integration | 実装済み |
| v0.7 | Remote / Cloud Runtime | experimental 実装済み。real AWS acceptance pending |
| v0.8 | Client Adapters & VS Code | 実装済み。real client acceptance pending |
| v0.9 | Per-Agent Sandbox & Agent Host Integration | broker foundation 実装済み |
| v0.10 | VS Code Remote Agent Host Adapter | 実装済み。real Agent Host acceptance pending |
| v0.11 | Base Images & Custom Environments | first slice 実装済み。build/import/GC 等は今後 |
| v0.12 | Sandbox Resource Limits | first slice 実装済み。real Incus enforcement acceptance pending |

詳しくは [`docs/00D_VERSIONING_AND_RELEASE_STATUS.ja.md`](docs/00D_VERSIONING_AND_RELEASE_STATUS.ja.md) と [`docs/IMPLEMENTATION_STATUS.ja.md`](docs/IMPLEMENTATION_STATUS.ja.md) を参照してください。

## VS Code で開く

ソースから使う場合:

```bash
git clone https://github.com/SLktEx/Hacocoon.git
cd Hacocoon

go build -o ./bin/haco ./cmd/haco
go build -o ./bin/haco-vscode ./cmd/haco-vscode
go build -o ./bin/haco-agent-host ./cmd/haco-agent-host
./bin/haco doctor
```

通常の Remote-SSH adapter:

```bash
./bin/haco-vscode open .
```

```text
Workspace を認識
  -> Environment を作成または再利用
  -> loopback-only SSH を準備
  -> adapter-owned SSH config を生成
  -> VS Code Remote-SSH で /workspace を開く
```

Private SSH key は Client 側に残り、Environment に渡しません。

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
```

Coding agent 自身に `haco` / Incus 管理 authority を渡しません。Deterministic な Environment 名だけでも ownership proof にはせず、persisted binding が一致しなければ Acquire / Release は fail closed します。

Parallel RW agent は原則として別 Git worktree / 別 canonical Workspace を使います。

正本は [`docs/09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md`](docs/09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md) です。

## v0.10: VS Code Remote Agent Host Adapter

v0.10 は `haco-agent-host` として実装済みです。

```bash
haco-agent-host prepare --session <opaque-id> [workspace]
haco-agent-host release --session <opaque-id>
```

```text
VS Code Agents window
        |
   Remote SSH
        |
Hacocoon-managed loopback SSH alias
        |
 haco-agent-host
        |
v0.9 bound Environment
        |
 /workspace
```

Hacocoon は Environment と安全な接続準備を所有し、VS Code が Agent Host / Agent Host Protocol を所有します。SSH private key は client 側に保持します。

正本は [`docs/10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.ja.md`](docs/10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.ja.md) です。

## v0.11: Base Images & Custom Environments

v0.11 の first slice は実装済みです。

```bash
haco image list
haco image inspect <base>
haco create --base <base> --workspace <path> <environment>
```

```text
logical Base
   -> provider-owned source
   -> create 時に一度だけ resolve
   -> immutable Base revision
   -> Environment に persist
```

Incus provider では mutable alias/source を immutable fingerprint に解決してから `incus init` します。そのため logical Base が後で動いても、既存 Environment の記録は変わりません。

```text
my-dev -> revision A -> Environment 1
my-dev -> revision B -> Environment 2
Environment 1 は revision A のまま
```

Host/operator の custom logical mapping は現在 `HACO_INCUS_BASES_JSON` で追加できます。`haco/` namespace は予約です。

Custom Base build/import、revision history、rollback、physical deletion、GC は first slice では未実装です。

正本は [`docs/11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md`](docs/11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md)、詳しい日本語設計は [`docs/BASE_IMAGES.ja.md`](docs/BASE_IMAGES.ja.md) です。

## v0.12: Sandbox Resource Limits

v0.12 の first slice は実装済みです。

```bash
haco create \
  --cpu 4 \
  --memory 8GiB \
  --pids 1024 \
  --root-size 40GiB \
  --workspace . dev

haco run --cpu 2 --memory 4GiB --workspace . -- go test ./...
```

provider-neutral ResourceBudget として CPU、memory bytes、process/PID count、Environment root storage bytes を保持します。各dimensionは finite または `unlimited` で、未指定の場合も provider default に放置せず Hacocoon が explicit `unlimited` effective value に解決して Environment metadata に保存します。

Incus provider では有限値を **Environment start 前** に設定し、read-backして一致を検証します。適用・検証に失敗した場合は constrained Environment の作成成功として扱わず、既存の cleanup/recovery semantics に従います。

```text
ResourceBudget  -> Environment 内部の消費量を制限
Capability      -> Environment 境界を越える authority を制御
```

requested finite limit を provider が enforce できない場合は silent ignore せず fail closed します。experimental EC2 は現時点では finite budget を AWS side effect 前に `unsupported` として拒否します。

byte size は `512MiB` / `8GiB` / `40GiB` のように明示 unit を使います。live resize、aggregate host scheduling、Workspace quota は first slice の対象外です。

詳しくは [`docs/12_v0.12_SANDBOX_RESOURCE_LIMITS.ja.md`](docs/12_v0.12_SANDBOX_RESOURCE_LIMITS.ja.md) と英語正本 [`docs/12_v0.12_SANDBOX_RESOURCE_LIMITS.md`](docs/12_v0.12_SANDBOX_RESOURCE_LIMITS.md) を参照してください。

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

Task 分解、worktree、Agent 選択、retry、model budget、development review は Orchestrator 側の責任です。

## 低レベル CLI

```bash
haco create --workspace "$PWD" dev
haco exec dev -- go test ./...
haco shell dev
haco status dev
haco delete dev

haco run --workspace "$PWD" -- go test ./...

haco image list
haco image inspect haco/ubuntu-26.04
haco create --base haco/ubuntu-26.04 --workspace "$PWD" dev

haco create --cpu 4 --memory 8GiB --pids 1024 --root-size 40GiB --workspace "$PWD" limited-dev
```

現在の主な command:

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

## EC2

v0.7 EC2 provider は **experimental / disabled by default** です。

```bash
export HACO_RUNTIME_PROVIDER=runtime.ec2
export HACO_EXPERIMENTAL_EC2=1
```

両方の explicit opt-in が必要です。Real AWS / EC2 / SSM / EBS acceptance は pending です。v0.12 first slice では finite ResourceBudget もprovider side effect前に拒否します。

## 開発とテスト

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/haco
go build ./cmd/haco-vscode
go build ./cmd/haco-agent-host
python tools/check_docs.py
```

Real Incus、Windows/WSL + VS Code、Agent Host/AHP、v0.11 real Base/image sources、AWS/EC2/SSM/EBS、v0.12 real resource enforcement の acceptance は、実際の対応環境で実行していない限り pass と扱いません。

資料の優先順位は [`docs/README.ja.md`](docs/README.ja.md)、詳しい architecture は [`docs/ARCHITECTURE_GUIDE.ja.md`](docs/ARCHITECTURE_GUIDE.ja.md) を参照してください。

## Breaking Change 方針

Hacocoon が stable compatibility milestone に到達するまでは、**どの revision 間でも Breaking Change が起こり得ます**。

ただし security boundary を弱めたり、silent data loss を許したりするための自由ではありません。より小さく安全な境界へ直すことを優先します。
