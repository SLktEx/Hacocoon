# AgentごとのSandboxとAgent Host連携

**状態:** Broker foundation実装済み
**互換性:** pre-1.0
**実機Routing Acceptance:** real VS Code Agent Host/AHP + Incusは未確認です。

## 目的

v0.9では、信頼されたClient Integrationが、独立して経路選択できるCoding Agent Sessionごとに専用Hacocoon Environmentを割り当てます。

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

- Incus ソケット / Incus管理権限
- Hacocoon 状態 / management control access
- Host側Hacocoon 認証情報
- 広いGitHub / AWS / Cloud / Host 認証情報
- 任意Environmentを作成・削除する権限

`haco`は人間・運用者・信頼された automation向けです。Coding Agent自身が自分のSandboxを管理するために`haco`を実行する設計にはしません。

## Session Binding

`internal/agenthost`はCoreの外側に置き、既存Environment / WorkspaceLease ライフサイクルを再利用します。

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

1. SessionごとにEnvironment 識別を分ける。
2. 生の Session IDを実行基盤名や状態へそのまま保存しない。
3. 同じSession/Workspace/access モードの再Acquireは繰り返しても同じ結果になる。
4. 別Workspace/access モードへのrebindは安全側で拒否。
5. Session→Environment 関連付けを信頼された管理機構状態へ永続化する。
6. Releaseはpersisted 関連付け proofがあるEnvironmentだけを削除する。
7. 決定的なな名前が一致するだけでは所有権 proofとみなさない。

これにより、人間が偶然同じEnvironment名を作っていた場合でも、関連付け記録がなければAgent Sessionから削除できません。

## 並列AgentとWorktree

複数RW Agentへ同じ正規の Host ディレクトリを渡しません。通常は別Git worktreeを用意します。

```text
repo
  +-- worktree/a -> Incus A -> Agent A
  +-- worktree/b -> Incus B -> Agent B
```

Git worktree は作業ファイルを分けますが、Git の管理情報は共有します。通常の製品手順で使う独立した管理 Workspace コピーとは異なります。Incus は OS・実行環境の隔離を担当します。

## v0.11 Base Imagesとの関係

エージェントごとの Environment も、実装済みの通常の Base 解決を使います。セッション割り当ては Base の契約を置き換えません。

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

Hooksはライフサイクル観測や後始末補助には使えても、HooksだけをSandbox境界とはみなしません。実際のExecution HostがEnvironment内にある必要があります。

初期v0.9の単位は**独立して経路選択できるtop-level Agent Session**です。Clientから独立経路選択できないhidden subagentまで1体1Incusとは主張しません。

具体的な VS Code Remote Agent Host Adapter は次の v0.10 連携 gate として、この broker foundation とは分離します。

## 既存機能

次はそのまま残します。

```text
haco env create / status / delete; haco open --client ssh
haco run
haco-vscode open / delete
```

## Acceptance

Repositoryでは、allocation、idempotence、rebind拒否、再起動復元、生の Session ID非露出、persisted proofなしRelease拒否、WorkspaceLease維持をテストします。

Real VS Code Agent Host/AHP + Incusについては、2 Sessionがexecution / 再接続 / 後始末まで混ざらないことを実機で別途確認します。
