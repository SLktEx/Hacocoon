# Hacocoon ドキュメント案内

[English](README.md) | **日本語**

このファイルは、2026-08-29 のarchitecture rebaselineと、v0.8〜v0.11のClient/Agent連携を含む現在の資料案内です。

> [!NOTE]
> 日本語資料は読みやすさのための補助資料です。設計上の最終的な正本は英語版authoritative documentです。

Hacocoonはまだ **pre-1.0** です。API / CLI / state / provider / client adapter / Base image / agent integrationは互換性なく変わる可能性があります。

## まず日本語で読むなら

1. [`../README.ja.md`](../README.ja.md) — 全体概要。
2. [`ARCHITECTURE_GUIDE.ja.md`](ARCHITECTURE_GUIDE.ja.md) — architecture / security boundary。
3. [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md) — 現在何が実装されているか。
4. [`10_v0.10_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md`](10_v0.10_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md) — SessionごとのEnvironment割当。
5. [`11_v0.11_VSCODE_REMOTE_AGENT_HOST_ADAPTER.ja.md`](11_v0.11_VSCODE_REMOTE_AGENT_HOST_ADAPTER.ja.md) — VS Code Agents windowから専用Environmentを使う方法。
6. [`BASE_IMAGES.ja.md`](BASE_IMAGES.ja.md) — v0.9 Base Images詳細設計。

## 正本の優先順位

1. `00_REBASELINE_AND_ROADMAP.md`
2. `00C_TERMINOLOGY_AND_BOUNDARIES.md`
3. `00B_SECURITY_ARCHITECTURE.md`
4. `IMPLEMENTATION_STATUS.md`
5. `01_...`〜`11_...` のversioned release specification
6. 個別詳細資料 (`CLIENT_ACCESS.md`, `BASE_IMAGES.md`等)
7. `00A_PLUGIN_ARCHITECTURE.md`
8. `90_CODEX_IMPLEMENTATION_HANDOFF.md`
9. `91_IMPLEMENTATION_REFERENCE_NOTES.md`
10. `adr/`

README類は入口であり、上記正本を上書きしません。

## 現在の状態

- v0.1〜v0.11 specは **versioned design contract**。
- v0.1〜v0.8は実装passあり。
- v0.9 Base Images & Custom Environmentsは **design only / implementation pending**。
- v0.10はopaque Session ID→専用Environmentのpersisted Brokerを実装。
- v0.11は`haco-agent-host`で、そのEnvironmentをVS Code Agents window用Remote-SSH targetとして準備する。
- AHPそのものはHacocoonで再実装せずVS Code側へ任せる。
- Real provider/client acceptanceとrepository CIは別物。

## v0.8: 普通のVS Code

```text
haco-vscode open .
  -> Environment
  -> loopback-only SSH
  -> VS Code Remote-SSH /workspace
```

VS Codeのeditor / terminal / Git UI / AI UIはVS Code側が所有します。

## v0.9: Base Images

```text
logical Base
  -> immutable revision
  -> Incus fingerprint (adapter内部)
  -> Environment
```

これはまだ実装pendingです。`haco image` / `haco create --base`は実装済みとは扱いません。

## v0.10: SessionごとのEnvironment

```text
opaque Session ID
      |
trusted persisted binding
      |
Environment / Incus
```

Coding Agent自身に`haco`、Incus socket、Hacocoon管理権限を渡しません。

同じrepositoryを複数Agentがwriteする場合は通常別worktreeを使います。

## v0.11: VS Code Agents window

準備:

```bash
haco-agent-host prepare --session task-a /path/to/worktree-a
```

```text
v0.10 Environment
   -> localhost SSH alias
   -> VS Code Agents window
      New -> Remote -> SSH -> alias
   -> VS Code側Agent Host / AHP
```

`--no-launch`なしなら`code --agents`を起動します。

終了:

```bash
haco-agent-host release --session task-a
```

Private SSH keyはClient側に残り、Environmentへ渡すのはpublic keyだけです。

### 隔離単位

保証単位は **1 Hacocoon `--session` slot = 1 Environment** です。

HacocoonがVS Code内部のSession UUIDを自動で取得するわけではないため、同じSSH aliasで複数VS Code Sessionを作れば同じEnvironmentを共有します。

完全に分けたい場合:

```bash
haco-agent-host prepare --session task-a /worktrees/a
haco-agent-host prepare --session task-b /worktrees/b
```

のようにslotとworktreeを分けます。

## Specification と Implementation

Specificationが存在することとcode/real-client acceptance済みであることは別です。

特に:

- v0.9はdesign-only。
- v0.10 Brokerは実装済みだがreal Agent Host routingは別Acceptance。
- v0.11 adapterは実装済みだがreal Windows/WSL/Incus + current VS Code Agents window acceptanceは別。

## Breaking Change

Hacocoonはpre-1.0です。CLI、helper binary、state、Provider、Capability、Client/Agent Integration等はBreaking Change可能です。

Security boundaryを弱めるためにaccidental compatibilityを残しません。

## ドキュメント変更時のルール

1. authoritative documentを先に更新する。
2. code realityが変わったら`IMPLEMENTATION_STATUS.md`を更新する。
3. entry point / 日本語summaryも追従する。
4. implementation claimとreal acceptanceを混ぜない。
5. experimental providerのdefault-offを維持する。
6. `python tools/check_docs.py`を実行する。
