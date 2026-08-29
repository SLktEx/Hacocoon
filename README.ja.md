# Hacocoon

**読み方: はこーん**

[English](README.md) | **日本語**

Hacocoon は、人間・開発ツール・Coding Agent向けの OSS **Secure Workspace Runtime（安全なWorkspace実行基盤）**です。

既存Workspaceを隔離されたEnvironmentに置き、Environment lifecycle、実行、Client access、Policy、承認、Capability、監査をtrusted Host側で管理します。

> [!WARNING]
> **Hacocoon はまだ pre-1.0 で、Breaking Change は今後も発生します。**
>
> CLI、helper binary、persisted state、API、Provider、Base/image設定、Client Adapter、Agent integrationは互換性なく変わる可能性があります。

## Hacocoonが担当するもの

```text
VS Code / Shell / Coding Agent / Orchestrator / other clients
                           |
                   thin/trusted integration
                           |
                        Workspace
                           v
                 +-------------------+
                 |     Hacocoon      |
                 | Environment       |
                 | execution/access  |
                 | policy/approval   |
                 | capabilities      |
                 | audit             |
                 +---------+---------+
                           |
                Environment provider
                    /             \
             runtime.incus    runtime.ec2
              local default   experimental
```

HacocoonはIDE、Git worktree manager、model router、Agent scheduler、AI Chat UIにはなりません。

Agentは隔離Environmentの中ではかなり自由に動かせますが、GitHub、AWS、Host credential等の外部authorityはEnvironmentの外側でHacocoonが仲介します。

## 現在の状態

現在のrepositoryは次の状態です。

- **v0.1〜v0.8**: Secure Workspace Runtime、Lease、Client access、Policy/Capability、Git/GitHub、Orchestrator、experimental cloud、VS Code Client Adapterまで実装済み。
- **v0.9 Base Images & Custom Environments**: 設計contractのみ。実装はまだpending。
- **v0.10 Per-Agent Sandbox**: trusted Session→専用Environmentのpersisted bindingを実装済み。
- **v0.11 VS Code Remote Agent Host Adapter**: `haco-agent-host`でv0.10 EnvironmentをVS Code Agents window用Remote-SSH targetとして準備する機能を実装済み。

実装済みであることとreal-provider/client acceptance済みであることは別です。Real Incus、Windows/WSL + VS Code、current VS Code Agents window / Agent Host、AWS等は対応環境で別途確認します。

詳細は [`docs/IMPLEMENTATION_STATUS.ja.md`](docs/IMPLEMENTATION_STATUS.ja.md)、architectureは [`docs/ARCHITECTURE_GUIDE.ja.md`](docs/ARCHITECTURE_GUIDE.ja.md)、資料の優先順位は [`docs/README.ja.md`](docs/README.ja.md) を参照してください。

## 普通のVS Code — v0.8

ソースからbuild:

```bash
git clone https://github.com/SLktEx/Hacocoon.git
cd Hacocoon

go build -o ./bin/haco ./cmd/haco
go build -o ./bin/haco-vscode ./cmd/haco-vscode
go build -o ./bin/haco-agent-host ./cmd/haco-agent-host
./bin/haco doctor
```

通常のVS Code Remote-SSHでWorkspaceを開く場合:

```bash
./bin/haco-vscode open .
```

```text
Workspace
  -> Hacocoon Environment
  -> loopback-only SSH
  -> VS Code Remote-SSH
  -> /workspace
```

終了:

```bash
./bin/haco-vscode delete .
```

HacocoonはVS Codeのeditor、terminal、debugger、Git UI、AI UIを再実装しません。

## Base Images — v0.9設計contract

v0.9では新しく作るEnvironmentの出発点をprovider-neutralな **Base** として扱う設計です。

```text
logical Base
   -> immutable Base revision
   -> provider-native starting point
   -> Environment
```

Incus alias / remote / fingerprintはAdapter内部の詳細にします。logical Baseを更新しても既存Environmentは古いrevisionのまま、新しく作るEnvironmentだけが新revisionを使います。

予定CLI例:

```text
haco image list
haco image inspect <base>
haco create --base <base> --workspace <path> <environment>
```

**これはまだ実装済みでも固定済みでもありません。**

正本は [`docs/09_v0.9_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md`](docs/09_v0.9_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md)、日本語詳細は [`docs/BASE_IMAGES.ja.md`](docs/BASE_IMAGES.ja.md) です。

## AgentごとのSandbox — v0.10

v0.10ではSession ownershipをCoreの外側に追加しています。

```text
opaque trusted Session ID
          |
          v
 internal/agenthost Broker
          |
 persisted ownership proof
          |
 dedicated Environment
```

重要なルール:

- 同じSession/Workspace/access modeの再Acquireはidempotent。
- 同じSessionを別Workspace/access modeへrebindしない。
- raw Session IDをそのままstateやruntime名へ出さない。
- deterministicなEnvironment名だけではownership proofにしない。
- persisted bindingがなければAgent SessionからEnvironmentをadopt/deleteしない。
- Coding Agent自身にIncus socketやHacocoon management authorityを渡さない。

同じrepositoryを複数Agentがwriteする場合は、通常は別Git worktree / Workspaceを使います。

詳細は [`docs/10_v0.10_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md`](docs/10_v0.10_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md) を参照してください。

## VS Code Agents window — v0.11

1つの独立したHacocoon remote slotを準備:

```bash
haco-agent-host prepare --session task-a /path/to/worktree-a
```

概念的には:

```text
Session task-a
  -> v0.10 Environment
  -> loopback-only SSH
  -> hashed SSH alias
  -> VS Code Agents window
       New -> Remote -> SSH -> haco-agent-...
  -> VS Code側のremote CLI / Agent Host / AHP
```

`--no-launch`がなければ`code --agents`でAgents windowを開きます。

別の独立RW Agentを作る場合:

```bash
haco-agent-host prepare --session task-b /path/to/worktree-b
```

明示cleanup:

```bash
haco-agent-host release --session task-a
```

### 隔離単位

v0.11で保証するのは:

> **1 Hacocoon `--session` slot = 1 v0.10 Environment**

です。

HacocoonがVS Code内部のtop-level Agent Session UUIDを自動取得するわけではありません。同じprepared SSH aliasから複数VS Code Sessionを作れば、それらは同じEnvironmentを共有する可能性があります。

完全に分けたいAgentは別`--session`と別worktreeを用意します。

### AHPはVS Code側

HacocoonはAHPを再実装しません。VS Codeがremote CLI / Agent Host / AHPの互換性とlifecycleを所有します。

Private SSH keyはClient側に残し、Environmentへ渡すのはpublic keyだけです。

詳細は [`docs/11_v0.11_VSCODE_REMOTE_AGENT_HOST_ADAPTER.ja.md`](docs/11_v0.11_VSCODE_REMOTE_AGENT_HOST_ADAPTER.ja.md) を参照してください。

## Windows + WSL

想定構成:

```text
Windows VS Code / OpenSSH
        |
   127.0.0.1:<port>
        |
 dedicated Hacocoon WSL 2
        |
      systemd
        |
      Incus
```

Host setupはdedicated `Hacocoon` WSL 2を使い、systemdをPID 1として動かします。無関係なWSL distributionやglobal defaultは触らず、`incus-admin`もexplicit opt-inです。

`haco-vscode`と`haco-agent-host`はWSL内からWindows Desktop VS Codeを使う場合、Windows user profile側の`.ssh`設定を対象にします。

## 低レベルCLI / trusted automation

```bash
haco create --workspace "$PWD" dev
haco exec dev -- go test ./...
haco shell dev
haco status dev
haco delete dev
```

trusted one-shot execution:

```bash
haco run --workspace "$PWD" -- go test ./...
haco run --workspace "$PWD" --json -- go test ./...
```

現在のhelper:

```text
haco-vscode open
haco-vscode delete

haco-agent-host prepare
haco-agent-host release
```

## Orchestratorとの関係

Daintree等はHacocoonの上位に置きます。

```text
Orchestrator
  -> task / model / worktree / retry / budget
  -> Workspace
  -> Hacocoon Environment
```

Hacocoonは「どこで安全に動かすか」と外部authority boundaryを担当し、task decompositionやmodel routingは所有しません。

## EC2

EC2 providerは引き続き **experimental / disabled by default** です。

```bash
export HACO_RUNTIME_PROVIDER=runtime.ec2
export HACO_EXPERIMENTAL_EC2=1
```

Real AWS / EC2 / SSM / EBS acceptanceはrepository上のfake-provider testとは別です。

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

Release CIではworkflow trust boundaryと、`haco` / `haco-vscode` / `haco-agent-host`のGoReleaser packagingも検査します。

## Roadmap

- v0.1: Secure Workspace Runtime
- v0.2: Workspace Abstraction & Lease
- v0.3: Client & Interactive Access
- v0.4: Policy & Capability Foundation
- v0.5: Git / GitHub Capability
- v0.6: Agent & Orchestrator Integration
- v0.7: Remote / Cloud Runtime & External Capabilities
- v0.8: Client Adapters & VS Code Integration
- v0.9: Base Images & Custom Environments — **design contract / implementation pending**
- v0.10: Per-Agent Sandbox & Agent Host Integration — implemented foundation
- v0.11: VS Code Remote Agent Host Adapter — implemented foundation

## Breaking Change 方針

Hacocoonがstable compatibility milestoneに到達するまでは、**どのrevision間でもBreaking Changeが起こり得ます**。

Security boundaryを弱めたり、曖昧なownershipやunsafe cleanupを残したりするためにaccidental compatibilityを守ることはしません。
