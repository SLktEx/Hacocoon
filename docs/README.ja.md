# Hacocoon ドキュメント案内

[English](README.md) | **日本語**

このファイルは、2026-08-29 の architecture rebaseline と、その後の **v0.9 までの実装進行**を踏まえて、Hacocoon の資料をどの順番で読めばよいかを説明します。

> [!NOTE]
> 日本語資料は読みやすさのための補助資料です。設計上の最終的な正本は英語版の authoritative document です。

Hacocoon はまだ **pre-1.0** です。現在の architecture や実装済み roadmap が書かれていても、API / CLI / state format / provider / client adapter / agent integration / configuration の互換性保証ではありません。

## まず日本語で読むなら

1. [`../README.ja.md`](../README.ja.md) — Hacocoon の目的と、`haco-vscode open .` を中心にした使い方。
2. [`ARCHITECTURE_GUIDE.ja.md`](ARCHITECTURE_GUIDE.ja.md) — architecture、security boundary、v0.1〜v0.8 の流れ。
3. [`09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md`](09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md) — AgentごとにIncus Environmentを分け、Agent自身には`haco`やIncus管理権限を渡さないv0.9設計。
4. [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md) — `main` に何が実装され、何が real acceptance 待ちか。
5. [`BASE_IMAGES.ja.md`](BASE_IMAGES.ja.md) — Incus の standard/custom image を Hacocoon の `Base` として扱う設計案。

## 正本の優先順位

資料同士が矛盾して見える場合、英語版は次の順番で優先します。

1. `00_REBASELINE_AND_ROADMAP.md` — product boundary と roadmap progression。
2. `00C_TERMINOLOGY_AND_BOUNDARIES.md` — architecture 用語と responsibility boundary。
3. `00B_SECURITY_ARCHITECTURE.md` — trust / security の横断ルール。
4. `IMPLEMENTATION_STATUS.md` — 現在の code reality と未 acceptance 項目。
5. `01_...`〜`09_...` — 各 roadmap stage の versioned design contract。
6. `CLIENT_ACCESS.md` / `REMOTE_CLOUD_PROVISIONING.md` / `BASE_IMAGES.md` 等 — 個別領域の詳細 contract または明示的な design proposal。
7. `00A_PLUGIN_ARCHITECTURE.md` — extension / adapter guidance。
8. `90_CODEX_IMPLEMENTATION_HANDOFF.md` — 実装・maintenance workflow。
9. `91_IMPLEMENTATION_REFERENCE_NOTES.md` — non-normative reference / historical notes。
10. `adr/` 配下 — 個別 decision。

`README.md`、`CODEX_START_HERE.md`、`Hacocoon_v0.1-v0.7_MASTER.md` は入口です。最後の master filename は historical name であり、v0.8-v0.9 の正本を上書きしません。

## 現在の読み方

現在の実装進行は v0.9 のBroker foundationまで進んでいます。

- v0.1〜v0.9 spec は **versioned design contract**。
- `IMPLEMENTATION_STATUS.md` が現在の repository reality。
- 実装済みでも public surface が固定されたわけではない。
- real-provider / real-client acceptance と CI は別物。
- EC2 は引き続き experimental / disabled by default。
- v0.8 は Client Adapter を追加し、VS Code を最初の adapter とする。
- v0.9 は trusted integration layer に Session -> Environment Broker を追加する。
- Agent自身には`haco`実行、Incus socket、Hacocoon control authorityを渡さない。
- real VS Code Agent Host/AHP + Incus per-session routingは、実機Acceptanceが終わるまで実証済みとは扱わない。
- `BASE_IMAGES.md` は design proposal であり、current versioned contract が明示しない範囲を勝手に実装順序へ追加しない。

## v0.9 を読む順番

1. [`09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md`](09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md)
2. [`09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md`](09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md)
3. [`02_v0.2_WORKSPACE_ABSTRACTION_AND_LEASE.md`](02_v0.2_WORKSPACE_ABSTRACTION_AND_LEASE.md)
4. [`08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md`](08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md)
5. [`00_REBASELINE_AND_ROADMAP.md`](00_REBASELINE_AND_ROADMAP.md)
6. [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md)

意図している構造は次です。

```text
VS Code Agents window / trusted client
                 |
       trusted integration
                 |
      per-session Broker
          /           \
 Environment A     Environment B
      |                 |
    Incus A           Incus B
```

Coding Agent自身はHacocoon control pathには入りません。Environment内で作業するだけで、自分のSandboxを作成・削除するために`haco`を実行しません。

同じrepositoryを複数Agentがwriteする場合は、通常はAgentごとに別Git worktreeを用意します。Git worktreeはコード変更の分離、Incus EnvironmentはOS/runtime security isolationを担当します。

## v0.8 を読む順番

1. [`08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md`](08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md)
2. [`CLIENT_ACCESS.md`](CLIENT_ACCESS.md)
3. [`00_REBASELINE_AND_ROADMAP.md`](00_REBASELINE_AND_ROADMAP.md)
4. [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md)

既存の構造は引き続き利用できます。

```text
VS Code / another client
        |
 thin Client Adapter
        |
    Hacocoon
        |
 isolated Environment
```

Environment 内では coding agent を permissive に動かせますが、GitHub / AWS / Host 等の外部 authority は Policy / Capability / Audit boundary を維持します。

## Specification と Implementation

Release specification は、その roadmap stage の設計・acceptance contract を示します。

`IMPLEMENTATION_STATUS.md` は **現在コードに何が存在するか**を示します。

たとえば v0.7 EC2 adapter が実装済みでも real AWS acceptance は別途必要です。同様に v0.8 の `haco-vscode` が実装済みでも real Windows/WSL + Incus + VS Code Remote-SSH acceptance は対応環境で確認する必要があります。v0.9もBroker foundationとreal Agent Host/AHP routing acceptanceを分けて扱います。

## Breaking Change

Hacocoon は pre-1.0 であり、simplification / hardening を継続します。

Breaking Change の対象には CLI、helper binary、state、Core / adapter API、Capability / Policy、Provider、Client Adapter / Agent Integration configuration、experimental backend、document structure が含まれます。

Accidental compatibility を守るために architecture や security boundary を弱くすることはしません。ただし silent data loss や security regression も許容しません。

## Historical material

- `Session`、Runtime/Storage-centric、plugin-heavy な旧コードは migration inventory として残る場合があります。
- 削除済み・superseded 資料が Git history や stale search index に見える場合があります。
- Btrfs / raw / QCOW2 等の historical experiment は current spec / ADR が再導入しない限り roadmap commitment ではありません。
- 「v0.1 を終えるまで後続を実装するな」という古い指示は historical です。

## ドキュメント変更時のルール

1. authoritative document を先に更新する。
2. code reality が変わったら `IMPLEMENTATION_STATUS.md` を更新する。
3. README / CODEX / handoff 等の入口も追従させる。
4. 日本語 summary も user-facing description が古くなったら更新する。
5. implementation claim と real-provider/client acceptance claim を混ぜない。
6. experimental provider の default-off status を維持する。
7. `python tools/check_docs.py` を実行する。
8. implementation detail を accidental compatibility promise にしない。
