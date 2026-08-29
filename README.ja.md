# Hacocoon

**読み方: はこーん**

[English](README.md) | **日本語**

Hacocoon は、人間・開発ツール・コーディングエージェント向けの OSS **Secure Workspace Runtime（安全なワークスペース実行基盤）**です。

既存の Workspace を隔離された Environment に置き、Environment lifecycle、実行、接続、Policy、承認、Capability、監査を Host 側で管理します。

> [!WARNING]
> **Hacocoon はまだ pre-1.0 で、Breaking Change は今後も発生します。**
>
> CLI、補助バイナリ、state format、API、Capability、Provider、Client Adapter、Agent Integration、設定は互換性なく変わる可能性があります。

## いちばんやりたい使い方

既存のv0.8では、普段の対話的な開発を次の形にします。

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

v0.9ではさらに、**独立してroutingできるAgent Sessionごとに別Environmentを割り当てる**ためのtrusted Broker基盤を追加します。

```text
VS Code Agents window / trusted client
                 |
       trusted integration
                 |
      Session -> Environment
          /              \
 Environment A        Environment B
      |                    |
    Incus A              Incus B
      |                    |
 Agent Host A          Agent Host B
      |                    |
   Agent A              Agent B
```

ここで重要なのは、**AIくん自身に`haco`を叩かせない**ことです。Hacocoon/Incusの管理権限はAgentの外側、trusted control planeに置きます。

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

## v0.9: AgentごとにEnvironmentを分ける

v0.9のBrokerは、trusted Client Integrationから渡されたopaqueなSession IDを、既存のEnvironment lifecycleへbindします。

- 別Sessionは別Environment identityへ割り当てる。
- 同じSession/Workspace/access modeの再取得はidempotent。
- 同じSessionを別Workspaceへ勝手にrebindしない。
- raw Session IDをIncus instance名へ直接使わない。
- Release対象はSession bindingから決め、AgentやClientに任意Environment名を削除させない。
- AgentにはIncus socket、Hacocoon state/control API、Host credentialを渡さない。

同じrepositoryを複数Agentが同時にwriteする場合、通常は別Git worktreeを用意します。

```text
repo
  +-- worktree/a -> Incus A -> Agent A
  +-- worktree/b -> Incus B -> Agent B
```

既存のWorkspaceLeaseを緩めて同じHost directoryへ複数RW Agentを突っ込む方式にはしません。Git worktreeはコード変更の分離、IncusはOS/runtimeのSecurity Sandboxを担当します。

VS Code連携はAgent Host / AHPを本命にします。ただし現在repositoryに入ったのはSession -> Environment Brokerの基盤であり、real VS Code Agent Host/AHP + Incus per-session routingは対応環境でE2E acceptanceするまで実証済みとは扱いません。

詳細は [`docs/09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md`](docs/09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md) を参照してください。

## VS Code で開く

従来のv0.8経路もそのまま残ります。標準ローカル runtime は Incus です。

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

## VS Code は最初の Client であって Core ではない

Hacocoon Core に VS Code / AHP 固有概念は入れません。

```text
VS Code adapter -----+
JetBrains adapter ---+----> Hacocoon ----> Environment
Web client ----------+
Daintree adapter ----+
```

将来別の IDE や Client を使うときも、同じ Environment / client-access boundary を利用します。

VS Code Extension を将来作る場合も、ボタン・状態表示・通知・Approval UX などを足す **薄い Adapter** とし、Remote-SSH や AI UI を再実装しません。

Per-Agent Session bindingやAHP固有処理もCoreではなくtrusted integration layerに置きます。

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

`haco` 自体の低レベル CLI は、人間、trusted script、debug、Client Adapter、Orchestrator から利用できます。

```bash
haco create --workspace "$PWD" dev
haco exec dev -- go test ./...
haco shell dev
haco status dev
haco delete dev
```

一回だけ Agent / tool をtrusted callerから実行する場合:

```bash
haco run --workspace "$PWD" -- go test ./...
haco run --workspace "$PWD" --json -- go test ./...
```

v0.9のPer-Agent経路はこれとは別です。Coding Agent自身には`haco run`や`haco agent ...`を実行させず、trusted integrationが外側からEnvironmentを割り当てます。

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

v0.9ではAI向けの`haco agent ...` commandは追加していません。

## 現在の状態

`main` の実装進行は **v0.9 Broker foundation** までです。

- v0.1: Secure Workspace Runtime
- v0.2: Workspace Abstraction & Lease
- v0.3: Client & Interactive Access
- v0.4: Policy & Capability Foundation
- v0.5: Git / GitHub Capability
- v0.6: Agent & Orchestrator Integration
- v0.7: Remote / Cloud Runtime & External Capabilities
- v0.8: Client Adapters & VS Code Integration
- v0.9: Per-Agent Sandbox & Agent Host Integration

EC2 は引き続き **experimental / disabled by default** です。`HACO_RUNTIME_PROVIDER=runtime.ec2` と `HACO_EXPERIMENTAL_EC2=1` の両方を明示しない限り有効にしません。

実装が存在することと real-provider/client acceptance が済んでいることは別です。詳細は [`docs/IMPLEMENTATION_STATUS.ja.md`](docs/IMPLEMENTATION_STATUS.ja.md) を参照してください。

## 設計境界

Hacocoon が所有するもの:

- Workspace と Environment の結合
- Environment lifecycle
- Workspace lease / ownership safety
- generic execution / client access
- Policy / Approval / Capability / Audit
- Host / external authority の security boundary

Core が所有しないもの:

- IDE / editor / AI chat UX
- Git branch / worktree orchestration
- model selection
- Agent DAG / retry / budget
- VS Code / AHP 固有設定
- Daintree 固有 workflow
- Provider 固有 Cloud / Storage mechanics

Per-Agent Session -> Environment bindingはCoreの外側のtrusted integration concernです。

詳しい日本語設計は [`docs/ARCHITECTURE_GUIDE.ja.md`](docs/ARCHITECTURE_GUIDE.ja.md)、v0.9は [`docs/09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md`](docs/09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md)、資料の優先順位は [`docs/README.ja.md`](docs/README.ja.md) を参照してください。

## 開発とテスト

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/haco
go build ./cmd/haco-vscode
python tools/check_docs.py
```

Real Incus、Windows/WSL + VS Code、VS Code Agent Host/AHP per-session routing、AWS/EC2/SSM/EBS の acceptance は、実際の対応環境で実行していない限り pass と扱いません。

## Breaking Change 方針

Hacocoon が stable compatibility milestone に到達するまでは、**どの revision 間でも Breaking Change が起こり得ます**。

ただし security boundary を弱めたり、silent data loss を許したりするための自由ではありません。より小さく安全な境界へ直すことを優先し、material な影響や safe migration path がある場合は明示します。
