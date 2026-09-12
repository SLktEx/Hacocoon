# VS Code Remote Agent Host Adapter

[English](vscode-remote-agent-host-adapter.md) | **日本語**

**Status:** v0.10 の基盤は PR #137 で `main` に実装済み。#345 で構造化したセッション情報とリモート Workspace の直接起動を追加しました。
**Compatibility:** pre-1.0 のため helper CLI / 連携詳細は Breaking Change の対象です。
**Host 検証:** real Windows/WSL + Incus + 現行 VS Code Agents window の確認は実機の条件に依存で、product E2E で別途確認します。

## 何をする機能か

Hacocoon の「信頼されたセッション -> 専用 Environment」broker と、VS Code の Remote Agent Host をつなぐ薄い信頼されたアダプターです。

VS Code は orchestration / UI / AHP を担当し、Hacocoon は isolated execution/runtime を担当します。

```text
VS Code Agents window / trusted orchestrator
                 |
        opaque session identity
                 |
          haco-agent-host
                 |
       Hacocoon Environment
                 |
       専用 Git worktree
                 |
           /workspace
                 |
     VS Code remote Agent Host
                 |
       coding-agent harness
```

独立して経路選択できる top-level エージェントセッションごとに、1つの Hacocoon Environment と1つの worktree を割り当てられます。セッション orchestration、model/harness 選択、UI、AHP は VS Code 側の責務のままです。

## 実装された入口

```text
haco-agent-host prepare --session <opaque-id> [--json] [options] [workspace]
haco-agent-host lookup  --session <opaque-id> [--json]
haco-agent-host release --session <opaque-id>
```

### `prepare`

`prepare` は次を行います。

- `internal/agenthost` で Environment を acquire/reuse する
- Environment が running であることを確認する
- 内部を解釈しないセッション ID を hash 化して SSH alias を作る
- SSH 非公開 key はクライアント側に保持し、公開 key だけを既存 SSH access パスに渡す
- `~/.ssh/hacocoon/` 以下の adapter-owned SSH 設定 fragment だけを管理する
- Hacocoon のループバック限定 SSH 接続を利用する
- 互換な接続は再利用し、変更時は replacement を準備してから古い接続を外す
- Environment、SSH alias、`/workspace`、VS Code remote-folder URI を含むセッション descriptor を出力する
- `--no-launch` がなければ Hacocoon remote workspace を指定した状態で VS Code Agents window を起動する

既定起動は概念的に次です。

```text
code --agents --folder-uri vscode-remote://ssh-remote+<managed-alias>/workspace
```

これにより、以前必要だった `New -> Remote -> SSH -> <alias>` の手選択を不要にします。

信頼された automation / orchestrator からは次の形を使えます。

```text
haco-agent-host prepare --session <id> --json --no-launch <worktree>
```

JSON descriptor は次の情報を持ちます。

```json
{
  "session_id": "opaque-session-id",
  "environment": "agent-...",
  "workspace_path": "/trusted/host/worktree",
  "remote_workspace": "/workspace",
  "ssh_alias": "haco-agent-...",
  "host_port": 2222,
  "folder_uri": "vscode-remote://ssh-remote+haco-agent-.../workspace"
}
```

生のセッション ID は信頼されたな machine-readable 応答にだけ返します。Environment 名前や SSH alias には引き続き生のセッション ID を使いません。

### `lookup`

`lookup` は読み取り専用の orchestration introspection です。既存の persisted セッション関連付けを解決して descriptor を返しますが、Environment の作成 / adopt / rebind / 削除は行いません。

```text
haco-agent-host lookup --session <id> --json
```

persisted 関連付けが所有権 proof であり、不明な / 古いセッションは安全側で拒否します。

### `release`

`release` は persisted セッションごと関連付けを解放し、管理対象の SSH fragment を削除します。後始末の結果が曖昧な場合は成功扱いにせず recovery-required として扱います。

## Worktree ownership

Git worktree 作成は Hacocoon Core の責務にはしません。信頼されたクライアント / orchestrator が、独立して書き込みを行うエージェントセッションごとに通常の linked worktree を用意し、そのパスを `prepare` に渡します。

```text
repository
  +-- worktree/session-a -> Environment A -> Agent session A
  +-- worktree/session-b -> Environment B -> Agent session B
```

Git worktree は作業ファイルを分けますが、Git の管理情報は共有します。通常の製品手順で使う独立した管理 Workspace コピーとは異なります。Hacocoon Environment は OS・実行環境を隔離します。

## Security boundary

コーディングエージェント自身を Hacocoon クライアントにはしません。Environment allocation、Workspace 所有権、SSH preparation/revocation、release は信頼された side に残ります。

生のセッション ID を persisted/public SSH alias に使いません。SSH 非公開 key を Environment にコピーしません。SSH access は既存のループバック限定クライアント境界を再利用します。

AHP は VS Code 側の external 連携プロトコルであり、Core vocabulary にはしません。task decomposition、model 経路選択、再試行、token budget、Agents UI も Hacocoon の責務にはしません。

## 他の orchestrator

セッション descriptor は、信頼されたな非 VS Code orchestrator でも Environment の allocate/reconnect と generic client-access 境界の利用に使える形にします。一方、VS Code 固有の folder URI 起動はこのアダプターに閉じ込め、Core へ持ち込みません。

これにより、将来の task scheduler / multi-agent orchestrator も同じセッションごと実行基盤境界を再利用できます。

## 配布

`haco-agent-host` は `haco` / `haco-vscode` と同じ Linux release アーカイブとインストーラーに含まれます。

## Validation

リポジトリの CI では helper behavior、descriptor serialization、remote-folder URI construction、SSH 設定 injection 防止、接続 reuse/rotation、Go test/vet/race、release packaging、インストーラー、host-independent E2E を確認します。

real VS Code Agent Host、Windows/WSL パス translation、実際の Incus SSH、multi-session 経路選択は実機検証が必要です。#344 が新規 Windows -> `haco-host` -> worktree -> Environment -> VS Code の composed 利用者 journey を追跡します。

> **VS Code がエージェント orchestration を担当し、Hacocoon はセッションごとの isolated workspace 実行基盤と権限境界を担当します。**
