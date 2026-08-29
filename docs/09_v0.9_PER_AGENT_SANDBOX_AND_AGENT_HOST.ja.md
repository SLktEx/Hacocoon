# v0.9 AgentごとのSandboxとAgent Host連携

**状態:** Broker foundation実装済み  
**互換性:** pre-1.0  
**実機Routing Acceptance:** real VS Code Agent Host/AHP + Incusは未確認です。

## 目的

v0.9では、信頼されたClient Integrationが、独立してroutingできるCoding Agent Sessionごとに専用Hacocoon Environmentを割り当てます。

```text
VS Code Agents UI / trusted client
                 |
       trusted integration
                 |
      SessionごとのBroker
                 |
       +---------+---------+
       |                   |
 Environment A        Environment B
    Incus A              Incus B
       |                   |
 Agent Host A          Agent Host B
       |                   |
    Agent A              Agent B
```

Coding Agent自身にはHacocoonやIncusの管理権限を渡しません。

## セキュリティ境界

Agent Environmentには、便利だからという理由で次を渡しません。

- Incus socket / Incus管理権限
- Hacocoon state / management control access
- Host側Hacocoon credential
- 広いGitHub / AWS / Cloud / Host credential
- 任意Environmentを作成・削除する権限

`haco`は人間・運用者・trusted automation向けです。Coding Agent自身が自分のSandboxを管理するために`haco`を実行する設計にはしません。

## Session Binding

`internal/agenthost`はCoreの外側に置き、既存Environment / WorkspaceLease lifecycleを再利用します。

```text
opaque Session ID
      |
      v
trusted persisted binding
      |
      v
Environment
```

初期実装のルール:

1. SessionごとにEnvironment identityを分ける。
2. raw Session IDをruntime名やstateへそのまま保存しない。
3. 同じSession/Workspace/access modeの再Acquireはidempotent。
4. 別Workspace/access modeへのrebindはfail closed。
5. Session→Environment bindingをtrusted control-plane stateへ永続化する。
6. Releaseはpersisted binding proofがあるEnvironmentだけを削除する。
7. deterministicな名前が一致するだけではownership proofとみなさない。

これにより、人間が偶然同じEnvironment名を作っていた場合でも、binding記録がなければAgent Sessionから削除できません。

## 並列AgentとWorktree

複数RW Agentへ同じcanonical Host directoryを渡しません。通常は別Git worktreeを用意します。

```text
repo
  +-- worktree/a -> Incus A -> Agent A
  +-- worktree/b -> Incus B -> Agent B
```

Git worktreeはコード変更の分離、Incus EnvironmentはOS/runtimeのSecurity Sandboxを担当します。

## v0.11 Base Imagesとの関係

v0.9はv0.11を置き換えません。将来Base selectionが実装されたら、Per-Agent Environmentも通常のBase resolutionを通します。

```text
Session binding
      |
EnvironmentSpec
 /          \
Workspace  Base
 \          /
 Environment
```

## VS Code Agent Host / AHP

VS CodeではAgent Hostを割り当てられたWorkspaceの近く、つまり対象Environment内で動かし、AHP固有処理はClient Integration境界に置く方向です。

Hooksはlifecycle観測やcleanup補助には使えても、HooksだけをSandbox境界とはみなしません。実際のExecution HostがEnvironment内にある必要があります。

初期v0.9の単位は**独立してroutingできるtop-level Agent Session**です。Clientから独立routingできないhidden subagentまで1体1Incusとは主張しません。

具体的な VS Code Remote Agent Host Adapter は次の v0.10 integration gate として、この broker foundation とは分離します。

## 既存機能

次はそのまま残します。

```text
haco create / exec / shell / delete
haco run
haco-vscode open / delete
```

## Acceptance

Repositoryでは、allocation、idempotence、rebind拒否、restart復元、raw Session ID非露出、persisted proofなしRelease拒否、WorkspaceLease維持をtestします。

Real VS Code Agent Host/AHP + Incusについては、2 Sessionがexecution / reconnect / cleanupまで混ざらないことを実機で別途確認します。

## 一文で言うと

> **v0.9は、Coding Agent Sessionごとに専用Hacocoon Environmentを割り当てつつ、HacocoonとIncusの管理権限をAgentの外側に置き続けるバージョンです。**
