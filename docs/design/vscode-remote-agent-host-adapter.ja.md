# VS Code Remote Agent Host Adapter

[English](vscode-remote-agent-host-adapter.md) | **日本語**

**Status:** v0.10 の基盤は PR #137 で `main` に実装済み。#345 で direct orchestration descriptor / remote-workspace launch を追加します。  
**Compatibility:** pre-1.0 のため helper CLI / integration detail は Breaking Change の対象です。  
**Host acceptance:** real Windows/WSL + Incus + 現行 VS Code Agents window の確認は host-dependent で、product E2E で別途確認します。

## 何をする機能か

Hacocoon の「trusted session -> 専用 Environment」broker と、VS Code の Remote Agent Host をつなぐ薄い trusted adapter です。

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

独立して routing できる top-level agent session ごとに、1つの Hacocoon Environment と1つの worktree を割り当てられます。session orchestration、model/harness 選択、UI、AHP は VS Code 側の責務のままです。

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
- opaque session ID を hash 化して SSH alias を作る
- SSH private key は client 側に保持し、public key だけを既存 SSH access path に渡す
- `~/.ssh/hacocoon/` 以下の adapter-owned SSH config fragment だけを管理する
- Hacocoon の loopback-only SSH connection を利用する
- 互換な connection は再利用し、変更時は replacement を準備してから古い connection を外す
- Environment、SSH alias、`/workspace`、VS Code remote-folder URI を含む session descriptor を出力する
- `--no-launch` がなければ Hacocoon remote workspace を指定した状態で VS Code Agents window を起動する

default launch は概念的に次です。

```text
code --agents --folder-uri vscode-remote://ssh-remote+<managed-alias>/workspace
```

これにより、以前必要だった `New -> Remote -> SSH -> <alias>` の手選択を不要にします。

trusted automation / orchestrator からは次の形を使えます。

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

raw session ID は trusted な machine-readable response にだけ返します。Environment name や SSH alias には引き続き raw session ID を使いません。

### `lookup`

`lookup` は read-only の orchestration introspection です。既存の persisted session binding を解決して descriptor を返しますが、Environment の create / adopt / rebind / delete は行いません。

```text
haco-agent-host lookup --session <id> --json
```

persisted binding が ownership proof であり、unknown / stale session は fail closed します。

### `release`

`release` は persisted per-session binding を解放し、managed SSH fragment を削除します。cleanup の結果が曖昧な場合は成功扱いにせず recovery-required として扱います。

## Worktree ownership

Git worktree 作成は Hacocoon Core の責務にはしません。trusted client / orchestrator が、独立して書き込みを行う agent session ごとに通常の linked worktree を用意し、その path を `prepare` に渡します。

```text
repository
  +-- worktree/session-a -> Environment A -> Agent session A
  +-- worktree/session-b -> Environment B -> Agent session B
```

Git worktree は code change を分離し、Hacocoon Environment は OS/runtime を分離します。

## Security boundary

coding agent 自身を Hacocoon client にはしません。Environment allocation、Workspace ownership、SSH preparation/revocation、release は trusted side に残ります。

raw session ID を persisted/public SSH alias に使いません。SSH private key を Environment にコピーしません。SSH access は既存の loopback-only client boundary を再利用します。

AHP は VS Code 側の external integration protocol であり、Core vocabulary にはしません。task decomposition、model routing、retry、token budget、Agents UI も Hacocoon の責務にはしません。

## 他の orchestrator

session descriptor は、trusted な非 VS Code orchestrator でも Environment の allocate/reconnect と generic client-access boundary の利用に使える形にします。一方、VS Code 固有の folder URI launch はこの adapter に閉じ込め、Core へ持ち込みません。

これにより、将来の task scheduler / multi-agent orchestrator も同じ per-session runtime boundary を再利用できます。

## 配布

`haco-agent-host` は `haco` / `haco-vscode` と同じ Linux release archive と installer に含まれます。

## Validation

repository CI では helper behavior、descriptor serialization、remote-folder URI construction、SSH config injection 防止、connection reuse/rotation、Go test/vet/race、release packaging、installer、host-independent E2E を確認します。

real VS Code Agent Host、Windows/WSL path translation、real Incus SSH、multi-session routing は real-host acceptance が必要です。#344 が fresh Windows -> `haco-host` -> worktree -> Environment -> VS Code の composed user journey を追跡します。

> **VS Code が agent orchestration を担当し、Hacocoon は per-session の isolated workspace runtime と authority boundary を担当します。**
