# Hacocoon

**読み方: はこーん**

[English](README.md) | **日本語**

Hacocoon は、人間・開発ツール・コーディングエージェント向けの OSS **Secure Workspace Runtime（安全なワークスペース実行基盤）**です。

既存の Workspace を受け取り、隔離された実行境界の内側に配置し、Environment のライフサイクル、コマンド実行、アクセス、Policy、承認、Capability、監査をホスト側の小さなコントロールプレーンで扱います。

> [!WARNING]
> **Hacocoon はまだ pre-1.0 であり、活発に開発中です。Breaking Change は今後も発生します。**
>
> CLI、永続化状態、API、Capability 契約、Provider Interface、設定形式などは互換性なく変更される可能性があります。依存する場合は利用するバージョンまたはコミットを固定し、更新前に変更内容を確認してください。

## Hacocoon は何をするものか

Hacocoon は IDE、Git worktree manager、AI agent scheduler そのものにはなりません。

それらは Hacocoon の外側に置き、Hacocoon を **実行境界とセキュリティ境界**として利用します。

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

信頼されるホスト側が Hacocoon の状態・Policy・Credential・特権 Capability 実行を所有します。

Environment には Workspace と、その処理に本当に必要な権限だけを渡します。

## 現在の状態

`main` には現在 **v0.7 までのロードマップ実装**が入っています。

ただしこれは「コードが存在する」という意味であり、安定版・互換性保証・本番サポートを意味しません。

| 領域 | 現在の状態 |
|---|---|
| Secure Workspace Runtime | Environment の create / exec / shell / delete を実装済み |
| Workspace model | 正規化した Workspace identity と永続化 RO/RW lease を実装済み |
| Lease safety | RW競合防止、stale lease recovery、process serialization を実装済み |
| Local runtime | Incus が標準 Environment provider |
| Client access | status、loopback port forwarding、connection管理、SSH準備/失効を実装済み |
| Policy / Capability | fail-closed の allow / deny / require-approval と audit を実装済み |
| Git / GitHub | host credential を Environment に渡さない brokered push を実装済み |
| Agent / orchestrator | `haco run`、machine-readable JSON、security event export を実装済み |
| Runtime routing | provider-neutral な Environment routing を実装済み |
| EC2 runtime | **experimental / default disabled** として実装済み |
| AWS capability | host-side の狭い read capability を実装済み |
| EBS replacement | replacement / migration flow を実装済み。in-place shrink や source volume 自動削除はしない |

実プロバイダーでの acceptance は別管理です。

Real Incus host、real AWS / EC2 / SSM / EBS の acceptance は、それぞれ適切な外部環境で実行する必要があります。Unit test、integration test、fake-provider E2E、race、vet、build、CI が通っていても、それらの代わりにはなりません。

詳細は [`docs/IMPLEMENTATION_STATUS.ja.md`](docs/IMPLEMENTATION_STATUS.ja.md) を参照してください。

ドキュメントの優先順位は [`docs/README.ja.md`](docs/README.ja.md) にまとめています。

## Quick Start

### 必要なもの

標準のローカル runtime を使う場合:

- Go **1.26**
- 現在のユーザーから利用できる Incus

ソースからビルドします。

```bash
git clone https://github.com/SLktEx/Hacocoon.git
cd Hacocoon

go build -o ./bin/haco ./cmd/haco
./bin/haco doctor
```

Workspace から Environment を作り、その中でコマンドを実行します。

```bash
./bin/haco create --workspace "$PWD" dev
./bin/haco exec dev -- uname -a
./bin/haco status dev
./bin/haco shell dev
./bin/haco delete dev
```

書き込みが不要なら read-only lease を使えます。

```bash
./bin/haco create --read-only --workspace "$PWD" review
```

一回だけツールや Agent を動かす場合は `haco run` が使えます。

```bash
./bin/haco run --workspace "$PWD" -- go test ./...
```

機械から扱う場合は JSON output を要求できます。

```bash
./bin/haco run --workspace "$PWD" --json -- go test ./...
```

## 現在の CLI

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

CLI もまだ pre-1.0 です。Command 名、flag、output、semantics は今後の整理で変更される可能性があります。

## セキュリティモデル

Hacocoon は **host と execution Environment を異なる trust domain として扱います**。

主なルール:

- 長寿命の host credential を便利だからという理由で Environment に mount しない。
- 特権 external action は broad credential を Environment に渡す代わりに narrow Capability を経由させる。
- Policy evaluation は fail closed とする。
- Human approval は orchestration engine ではなく security boundary として扱う。
- Capability request と decision は audit 可能にする。
- Workspace の write access は永続化 lease で保護する。
- Local port exposure は標準で loopback に限定する。
- Provider 固有概念を Core に持ち込まない。
- Cleanup / recovery failure を成功として隠さない。

ただし Hacocoon 単体で、誤設定された Incus や Cloud 環境を安全に変換できるわけではありません。Host・Provider・Deployment 側の設定もセキュリティ境界の一部です。

Security-sensitive な変更をする場合は [`docs/00B_SECURITY_ARCHITECTURE.md`](docs/00B_SECURITY_ARCHITECTURE.md) と [`.github/security/ADVERSARIAL_AUDIT.md`](.github/security/ADVERSARIAL_AUDIT.md) を確認してください。

## Experimental EC2 Provider

EC2 provider は v0.7 の実験機能として存在しますが、AWS credential があるだけ、または AWS CLI が入っているだけでは有効になりません。

現在は両方の明示設定が必要です。

```bash
export HACO_RUNTIME_PROVIDER=runtime.ec2
export HACO_EXPERIMENTAL_EC2=1
```

明示的な experimental opt-in がない場合、Hacocoon は real EC2 provider の構築や AWS call より前に fail closed する必要があります。

現在の remote path は S3 staging と SSM を利用します。

Real AWS acceptance はまだ別途必要です。詳細は [`docs/REMOTE_CLOUD_PROVISIONING.md`](docs/REMOTE_CLOUD_PROVISIONING.md) を参照してください。

## 設計境界

Hacocoon は Core を意図的に小さく保ちます。

Core が所有しないもの:

- IDE / editor UX
- Git branch / worktree orchestration
- Model selection
- Agent DAG / retry
- Model / token budget
- Provider 固有 storage mechanics
- Provider 固有 cloud API

Incus、Git/GitHub、AWS/EC2/EBS、VS Code、外部 orchestrator などの具体的な統合は、共通の **Workspace / Environment / Execution** モデルの外側にある明示的な境界として扱います。

より詳しい日本語の設計説明は [`docs/ARCHITECTURE_GUIDE.ja.md`](docs/ARCHITECTURE_GUIDE.ja.md) を参照してください。

## 開発とテスト

通常のチェック:

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/haco
python tools/check_docs.py
```

外部 Infrastructure が必要な integration / acceptance path は、実際にその Provider 上で実行していない限り「pass」とは扱いません。

## v0.1〜v0.7

v0.1〜v0.7 のドキュメントは **その段階の versioned design contract** です。

現在の具体的な interface を永久に固定する約束ではありません。

1. [`v0.1 Secure Workspace Runtime`](docs/01_v0.1_SECURE_WORKSPACE_RUNTIME.md)
2. [`v0.2 Workspace Abstraction & Lease`](docs/02_v0.2_WORKSPACE_ABSTRACTION_AND_LEASE.md)
3. [`v0.3 Client & Interactive Access`](docs/03_v0.3_CLIENT_AND_INTERACTIVE_ACCESS.md)
4. [`v0.4 Policy & Capability Foundation`](docs/04_v0.4_POLICY_AND_CAPABILITY_FOUNDATION.md)
5. [`v0.5 Git / GitHub Capability`](docs/05_v0.5_GIT_AND_GITHUB_CAPABILITY.md)
6. [`v0.6 Agent & Orchestrator Integration`](docs/06_v0.6_AGENT_AND_ORCHESTRATOR_INTEGRATION.md)
7. [`v0.7 Remote / Cloud Runtime & External Capabilities`](docs/07_v0.7_REMOTE_AND_CLOUD_RUNTIME.md)

日本語で流れを先に理解したい場合は [`docs/ARCHITECTURE_GUIDE.ja.md`](docs/ARCHITECTURE_GUIDE.ja.md) から読むのがおすすめです。

## Breaking Change 方針

Hacocoon が明示的な安定互換性 milestone に到達するまでは、**どの revision 間でも Breaking Change が起こり得る**と考えてください。

これは意図的です。

まだ設計境界を hardening している段階なので、より小さく・安全で・責任分界が明確になるなら、既存機能を削除・rename・replace・redesign することがあります。

ただし Breaking Change が許されることと、silent data loss や security regression を許すことは別です。

安全な migration path が提供できる場合は operator impact と migration 方法を明示し、destructive operation や authority boundary は compatibility より安全性を優先します。
