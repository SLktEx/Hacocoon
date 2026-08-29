# Hacocoon

**読み方: はこーん**

[English](README.md) | **日本語**

Hacocoon は、人間・開発ツール・コーディングエージェント向けの OSS **Secure Workspace Runtime（安全なワークスペース実行基盤）**です。

既存の Workspace を隔離された Environment に置き、Environment lifecycle、実行、接続、Policy、承認、Capability、監査を Host 側で管理します。

> [!WARNING]
> **Hacocoon はまだ pre-1.0 で、Breaking Change は今後も発生します。**
>
> CLI、補助バイナリ、state format、API、Capability、Provider、Client Adapter、Base/image 設定は互換性なく変わる可能性があります。

## いちばんやりたい使い方

v0.8 では、普段の対話的な開発を次の形にします。

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

AI は隔離 Environment の中ではかなり自由に動かせます。Package install、コード変更、build、test、壊してやり直す、といった処理は Environment 内に閉じ込めます。

一方で、GitHub、AWS、Host credential など Environment の外の authority は Hacocoon の Policy / Capability / Audit を通します。

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

## VS Code で開く

標準ローカル runtime は Incus です。

ソースから使う場合:

```bash
git clone https://github.com/SLktEx/Hacocoon.git
cd Hacocoon

go build -o ./bin/haco ./cmd/haco
go build -o ./bin/haco-vscode ./cmd/haco-vscode
./bin/haco doctor
```

VS Code Remote-SSH と SSH key pair を用意したうえで、開きたい Workspace から:

```bash
./bin/haco-vscode open .
```

これで概念的には次を自動化します。

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

主な option:

```bash
./bin/haco-vscode open --name dev .
./bin/haco-vscode open --identity /path/to/id_ed25519 .
./bin/haco-vscode open --read-only .
./bin/haco-vscode open --no-launch .
```

## Windows + WSL

Hacocoon/Incus が WSL 側、デスクトップ VS Code が Windows 側にある場合、SSH client の設定場所も別です。

v0.8 の `haco-vscode` は WSL から実行された場合に Windows user profile を解決し、**Windows 側の `.ssh` 設定**を対象にします。

```text
Windows VS Code
   -> Windows OpenSSH
   -> 127.0.0.1:<port>
   -> WSL / Hacocoon
   -> Incus:22
```

Real Windows/WSL + Incus + VS Code Remote-SSH acceptance は、対応環境上で別途確認する必要があります。

## v0.9: Base image を選べるようにする

次の roadmap gate は **v0.9 Base Images & Custom Environments** です。

Hacocoon から Incus の alias や fingerprint を直接選ぶのではなく、Hacocoon の **Base** を指定します。

```text
my-dev                 <- logical Base name
  |
  v
immutable Base revision
  |
  v
Incus fingerprint      <- Incus adapter 内部
  |
  v
Environment
```

大事なルールは、logical Base を更新しても既存 Environment を勝手に変えないことです。

```text
my-dev -> revision A -> Environment 1

my-dev を revision B に更新

Environment 1 -> revision A のまま
Environment 2 -> revision B
```

つまり、Base を変えたい場合は新しい Environment を作ります。起動済み Environment の root の出発点を途中で差し替える機能にはしません。

Custom Base も信用しません。Image の中身や metadata だけで次の権限を増やすことは禁止します。

- Host filesystem mount
- Incus device
- privileged container
- Linux capability
- Host network authority
- GitHub / AWS / cloud credential
- SSH private key
- Hacocoon control authority

想定している CLI は例えば次です。

```text
haco image list
haco image inspect <base>
haco create --base <base> --workspace <path> <environment>
```

ただし **これは v0.9 の設計 contract で、まだ実装済み CLI ではありません**。CLI/config は pre-1.0 の間は変更可能です。

正本は [`docs/09_v0.9_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md`](docs/09_v0.9_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md)、詳しい日本語設計は [`docs/BASE_IMAGES.ja.md`](docs/BASE_IMAGES.ja.md) です。

## VS Code は最初の Client であって Core ではない

Hacocoon Core に VS Code 固有概念は入れません。

```text
VS Code adapter -----+
JetBrains adapter ---+----> Hacocoon ----> Environment
Web client ----------+
Daintree adapter ----+
```

将来別の IDE や Client を使うときも、同じ Environment / client-access boundary を利用します。

VS Code Extension を将来作る場合も、ボタン・状態表示・通知・Approval UX などを足す **薄い Adapter** とし、Remote-SSH や AI UI を再実装しません。

## Orchestrator との関係

Daintree などの Orchestrator は Hacocoon の上に置けます。

```text
Daintree / Orchestrator
  -> Task / worktree / Agent 管理
  -> Workspace
  -> Hacocoon
  -> isolated Environment
```

Task 分解、worktree、Agent 選択、retry、budget、development review は Orchestrator 側の責任です。

Hacocoon は「どこで安全に動かすか」と「外へ出る authority」を担当します。

## 低レベル CLI

`haco` 自体の低レベル CLI も、script、debug、Client Adapter、Orchestrator から利用できます。

```bash
haco create --workspace "$PWD" dev
haco exec dev -- go test ./...
haco shell dev
haco status dev
haco delete dev
```

一回だけ Agent / tool を実行する場合:

```bash
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

`haco image` と `haco create --base` は v0.9 の予定であり、現時点の実装済み command としては扱いません。

## 現在の状態

`main` の **実装進行は v0.8 まで**です。Roadmap/design は **v0.9** まで確定しています。

- v0.1: Secure Workspace Runtime
- v0.2: Workspace Abstraction & Lease
- v0.3: Client & Interactive Access
- v0.4: Policy & Capability Foundation
- v0.5: Git / GitHub Capability
- v0.6: Agent & Orchestrator Integration
- v0.7: Remote / Cloud Runtime & External Capabilities
- v0.8: Client Adapters & VS Code Integration — 実装済み
- v0.9: Base Images & Custom Environments — **設計 contract、実装待ち**

EC2 は引き続き **experimental / disabled by default** です。`HACO_RUNTIME_PROVIDER=runtime.ec2` と `HACO_EXPERIMENTAL_EC2=1` の両方を明示しない限り有効にしません。

実装が存在することと real-provider/client acceptance が済んでいることは別です。詳細は [`docs/IMPLEMENTATION_STATUS.ja.md`](docs/IMPLEMENTATION_STATUS.ja.md) を参照してください。

## 設計境界

Hacocoon が所有するもの:

- Workspace と Environment の結合
- Environment lifecycle
- Workspace lease / ownership safety
- Base name → immutable Base revision の解決 contract
- generic execution / client access
- Policy / Approval / Capability / Audit
- Host / external authority の security boundary

Core が所有しないもの:

- IDE / editor / AI chat UX
- Git branch / worktree orchestration
- model selection
- Agent DAG / retry / budget
- VS Code 固有設定
- Daintree 固有 workflow
- Incus alias / remote / fingerprint の native mechanics
- Provider 固有 Cloud / Storage mechanics

詳しい日本語設計は [`docs/ARCHITECTURE_GUIDE.ja.md`](docs/ARCHITECTURE_GUIDE.ja.md)、資料の優先順位は [`docs/README.ja.md`](docs/README.ja.md) を参照してください。

## 開発とテスト

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/haco
go build ./cmd/haco-vscode
python tools/check_docs.py
```

Real Incus、Base/image lifecycle、Windows/WSL + VS Code、AWS/EC2/SSM/EBS の acceptance は、実際の対応環境で実行していない限り pass と扱いません。

## Breaking Change 方針

Hacocoon が stable compatibility milestone に到達するまでは、**どの revision 間でも Breaking Change が起こり得ます**。

ただし security boundary を弱めたり、silent data loss を許したりするための自由ではありません。より小さく安全な境界へ直すことを優先し、material な影響や safe migration path がある場合は明示します。
