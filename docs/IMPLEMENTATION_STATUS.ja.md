# 実装状況

[English](IMPLEMENTATION_STATUS.md) | **日本語**

Status date: 2026-08-29 — v0.11 VS Code Remote Agent Host Adapter implementation pass 後。

このファイルは **現在のコードの事実**を説明する日本語版です。理想architectureや互換性保証ではありません。

Hacocoonはまだ **pre-1.0** です。実装済みであることはinterface固定、本番support、real-provider/client acceptance済みを意味しません。

重要な区別:

- v0.9 Base Images & Custom Environmentsは **design only / implementation pending**。
- v0.10はSession→Environment Broker foundationを実装済み。
- v0.11はそのv0.10 bindingをVS Code Agents windowから標準Remote-SSHで使う`haco-agent-host`を実装済み。
- Real VS Code Agent Host/AHP + Incus acceptanceは別途必要。

| 領域 | 現在のrepository reality | Release | 検証状況 |
|---|---|---:|---|
| Secure Workspace Runtime | `haco create --workspace` / `exec` / `shell` / `delete` | v0.1 | unit/process integration pass。real Incus pending |
| Workspace / Lease | canonical Workspace identity、RO/RW lease、RW conflict prevention、stale recovery | v0.1-v0.2 | unit/persistence/concurrency pass |
| Client access | status、loopback forward、SSH prepare/revoke | v0.3 | unit/process integration pass |
| Policy / Capability | fail-closed allow/deny/require-approval、human security approval、audit | v0.4 | unit/integration/CLI E2E pass |
| Git / GitHub | Host-side brokered push。broad Host credentialをEnvironmentへexportしない | v0.5 | unit/adversarial/real-git/CLI E2E pass |
| Agent / Orchestrator | trusted `haco run`、machine JSON、security event export | v0.6 | unit/race/integration/CLI E2E pass |
| EC2 / AWS | experimental / disabled by default | v0.7 | fake-AWS path pass。real AWS pending |
| VS Code Client Adapter | `haco-vscode`で通常Remote-SSH `/workspace` | v0.8 | helper unit。real VS Code + Incus pending |
| Windows / WSL | dedicated `Hacocoon` WSL 2 + systemd、Windows側SSH profile | v0.8 | static CI。real Windows acceptance pending |
| Base Images & Custom Environments | logical Base / immutable revision / Incus fingerprint boundary | v0.9 | **design only / implementation pending** |
| Per-Agent Sandbox Broker | `internal/agenthost`がopaque Session IDを専用Environmentへbind | v0.10 | allocation/idempotence/rebind/restart/proof-required release等のunit coverage |
| Agent Binding State | `agent-bindings.json`にtrusted ownership proofを永続化。raw Session IDはhash化 | v0.10 | Linux process lock + atomic/fsync write |
| VS Code Remote Agent Host Adapter | `haco-agent-host prepare/release`でv0.10 bindingをloopback SSH aliasへ変換 | v0.11 | helper unit +通常CI gate。real Agents-window acceptance pending |
| Agent Host / AHP ownership | AHPはHacocoonで実装せずVS Code側に任せる | v0.11 | contract定義。real current-VS-Code acceptance pending |
| Release packaging | `haco` / `haco-vscode` / `haco-agent-host`をLinux amd64/arm64 archiveへ同梱 | v0.11 | GoReleaser dry-run / archive validation対象 |

## v0.10: SessionごとのEnvironment

```text
trusted client
    |
opaque Session ID
    |
internal/agenthost Broker
    |
persisted ownership proof
    |
Environment / Incus
```

大事なルール:

- exact reacquireはidempotent。
- 別Workspace/access modeへのrebindはfail closed。
- raw Session IDをEnvironment名/stateへ直接出さない。
- deterministicなEnvironment名だけではownership proofにしない。
- persisted bindingがないEnvironmentをAgent Sessionからadopt/deleteしない。
- Coding Agent自身に`haco`やIncus管理権限を渡さない。

並列RW Agentには通常別Git worktreeを使います。

## v0.11: VS Code Agents windowから使う

準備:

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
       New -> Remote -> SSH -> alias
  -> VS Code側がremote CLI / Agent Hostを管理
```

`--no-launch`を付けなければ`code --agents`でAgents windowを開きます。

Private SSH keyはClient側に残り、Environmentへ渡すのはpublic keyだけです。

終了:

```bash
haco-agent-host release --session task-a
```

Releaseはv0.10のbinding proofを使うため、任意Environment名を削除できるinterfaceにはしていません。

### 隔離単位

v0.11で保証するのは:

> **1 Hacocoon `--session` slot = 1 v0.10 Environment**

です。

HacocoonがVS Code内部のAgent Session UUIDを自動受信するわけではありません。同じprepared SSH aliasを使ってVS Code内で複数Sessionを作れば、それらは同じEnvironmentを共有します。

独立させたい場合:

```bash
haco-agent-host prepare --session task-a /worktrees/a
haco-agent-host prepare --session task-b /worktrees/b
```

のようにslotとworktreeを分けます。

### なぜAHPをHacocoonで書かないか

VS CodeのAgent Host/AHPはVS Code側の責務にします。Hacocoonは標準SSH targetを提供するだけです。

そのためCoreへ:

- AHP JSON-RPC/WebSocket
- Agent Host token
- VS Code内部process flag
- Agent UI/model routing

を持ち込みません。

## prepare failureの扱い

`prepare`途中でSSH config作成等に失敗してもv0.10 Session binding自体は暗黙Releaseしません。

これは並列prepareで、一方がもう一方の取得したEnvironmentを誤削除するraceを避けるためです。

新しく作ったSSH forwardingだけはsetup failure時にcleanupし、Environmentの破壊は明示`release`に限定します。

## Windows / WSL

`haco-vscode`と`haco-agent-host`は、HacocoonがWSL、VS CodeがWindows Desktopの場合にWindows user profile側の`.ssh`設定を扱います。

Host setupはdedicated `Hacocoon` WSL 2 + systemdを使い、無関係なWSL distribution/global defaultを触りません。`incus-admin`もexplicit opt-inです。

## v0.9 Base Images

v0.9は引き続き未実装です。

```text
logical Base
  -> immutable Base revision
  -> Incus fingerprint (adapter内部)
  -> new Environment
```

予定CLI例:

```text
haco image list
haco image inspect <base>
haco create --base <base> --workspace <path> <environment>
```

これはまだ実装済みでも固定済みでもありません。

## Acceptanceの区別

以下がpassしてもreal-client/provider acceptanceの代替にはなりません。

- unit test
- race
- vet
- build
- workflow policy
- release dry-run
- docs consistency
- fake-provider E2E
- repository CI

Real Incus、Windows/WSL、VS Code Remote-SSH、VS Code Agents window/Agent Host、AWS/EC2等は対応環境で別途確認します。

## Compatibility status

v0.1〜v0.11のどのdesign / implementationも、現在のconcrete interfaceが変更されないという約束ではありません。

Breaking ChangeによりCLI、helper binary、state、provider、Base/image lifecycle、Capability/Policy、Client/Agent integration、host bootstrap behavior等は変更可能です。
