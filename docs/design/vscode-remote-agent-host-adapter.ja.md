# v0.10 VS Code Remote Agent Host Adapter

[English](vscode-remote-agent-host-adapter.md) | **日本語**

**Status:** PR #137 で `main` に実装済み。  
**Compatibility:** pre-1.0 のため helper CLI / integration detail は Breaking Change の対象です。  
**Host acceptance:** real Windows/WSL + Incus + 現行 VS Code Agents window の確認は host-dependent です。

## 何をする機能か

v0.9 の「trusted session -> 専用 Environment」broker と、VS Code の Remote Agent Host をつなぐ薄い trusted adapter です。

```text
VS Code Agents window
        |
    Remote SSH
        |
Hacocoon 管理の loopback alias
        |
  haco-agent-host
        |
 v0.9 の専用 Environment
        |
    /workspace
```

実装された入口は次です。

```text
haco-agent-host prepare --session <opaque-id> [options] [workspace]
haco-agent-host release --session <opaque-id>
```

`prepare` は `internal/agenthost` で Environment を取得し、client 側の SSH private key は外へ出さず public key だけを既存 SSH access path に渡します。session ID はそのまま SSH alias にせず hash から alias を作り、`~/.ssh/hacocoon/` 以下の adapter-owned config だけを管理します。

互換な SSH connection は再利用し、変更が必要な場合は新 connection / config を用意してから古い connection を外します。`--no-launch` がなければ最後に `code --agents` を起動します。

`release` は v0.9 binding を解放し、managed SSH fragment を削除します。cleanup の結果が曖昧な場合は成功扱いにせず recovery-required として扱います。

## Security boundary

coding agent 自身が `haco` や Incus control plane を操作する設計にはしていません。Environment allocation、Workspace ownership、SSH preparation/revocation、release は trusted side に残ります。

SSH private key を Environment にコピーしません。SSH access は既存の loopback-only client boundary を再利用します。

## 配布

`haco-agent-host` は `haco` / `haco-vscode` と同じ Linux release archive と installer に含まれます。

## Validation

repository CI では helper unit test、SSH config injection 防止、connection reuse/rotation、Go test/vet/race、release packaging、installer、host-independent E2E を確認します。

real VS Code Agent Host、Windows/WSL path translation、real Incus SSH は別途 real-host acceptance が必要です。

> **v0.10 は VS Code Agent Host と v0.9 Environment ownership をつなぐ trusted bridge であり、agent に Hacocoon 管理権限を渡す機能ではありません。**
