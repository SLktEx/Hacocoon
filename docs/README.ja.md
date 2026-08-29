# Hacocoon ドキュメント案内

[English](README.md) | **日本語**

このファイルは、2026-08-29 の architecture rebaseline、**v0.8 までの実装進行**、そして v0.9 Base Images & Custom Environments の roadmap decision を踏まえて、Hacocoon の資料をどの順番で読めばよいかを説明します。

> [!NOTE]
> 日本語資料は読みやすさのための補助資料です。設計上の最終的な正本は英語版の authoritative document です。

Hacocoon はまだ **pre-1.0** です。現在の architecture や roadmap が書かれていても、API / CLI / state format / provider / client adapter / Base image / configuration の互換性保証ではありません。

## まず日本語で読むなら

1. [`../README.ja.md`](../README.ja.md) — Hacocoon の目的と、`haco-vscode open .` を中心にした使い方。
2. [`ARCHITECTURE_GUIDE.ja.md`](ARCHITECTURE_GUIDE.ja.md) — architecture、security boundary、v0.1〜v0.8 の実装の流れ。
3. [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md) — `main` に何が実装され、何が real acceptance 待ちか。
4. [`BASE_IMAGES.ja.md`](BASE_IMAGES.ja.md) — v0.9 の Incus standard/custom image を Hacocoon の `Base` として扱う詳細設計。
5. [`09_v0.9_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md`](09_v0.9_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md) — v0.9 の英語正本となる最小 roadmap/acceptance contract。

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

`README.md`、`CODEX_START_HERE.md`、`Hacocoon_v0.1-v0.7_MASTER.md` は入口です。最後の master filename は historical name であり、v0.9 の正本を上書きしません。

## 現在の読み方

現在の **実装進行は v0.8**、次の明示的な roadmap gate は **v0.9** です。

- v0.1〜v0.9 spec は **versioned design contract**。
- v0.1〜v0.8 は `main` に implementation pass が存在する。
- v0.9 は Base Images & Custom Environments の設計/roadmap gate として確定したが、まだ実装済みとは扱わない。
- `IMPLEMENTATION_STATUS.md` が現在の repository reality。
- 実装済みでも public surface が固定されたわけではない。
- real-provider / real-client acceptance と CI は別物。
- EC2 は引き続き experimental / disabled by default。
- v0.8 は Client Adapter を追加し、VS Code を最初の adapter とする。
- VS Code の editor / terminal / Git UI / AI UI は VS Code が所有し、Hacocoon Core には持ち込まない。
- v0.9 は logical Base を immutable revision に解決して Environment を作る。
- Incus alias / remote / fingerprint は Core API に出さず、Incus adapter の内部詳細にする。
- `BASE_IMAGES.md` は詳細 companion、`09_v0.9_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md` が v0.9 の最小 authoritative gate。
- v0.9 より後の scope は、明示的な decision なしに勝手に invent しない。

## v0.8 を読む順番

1. [`08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md`](08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md)
2. [`CLIENT_ACCESS.md`](CLIENT_ACCESS.md)
3. [`00_REBASELINE_AND_ROADMAP.md`](00_REBASELINE_AND_ROADMAP.md)
4. [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md)

意図している構造は次です。

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

## v0.9 を読む順番

1. [`09_v0.9_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md`](09_v0.9_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md)
2. [`BASE_IMAGES.ja.md`](BASE_IMAGES.ja.md) / [`BASE_IMAGES.md`](BASE_IMAGES.md)
3. [`00_REBASELINE_AND_ROADMAP.md`](00_REBASELINE_AND_ROADMAP.md)
4. [`00B_SECURITY_ARCHITECTURE.md`](00B_SECURITY_ARCHITECTURE.md)
5. [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md)

概念は次です。

```text
my-dev という logical Base
        |
        v
immutable Base revision
        |
        v
Incus fingerprint (adapter内部)
        |
        v
Environment
```

`my-dev` が新しい revision に更新されても、既存 Environment は古い revision のままです。新しい Environment だけが新 revision を使います。

また custom Base は guest の中身を決めるだけで、Host mount / privileged / capability / credential / external authority を増やす権限は持ちません。

## Specification と Implementation

Release specification は、その roadmap stage の設計・acceptance contract を示します。

`IMPLEMENTATION_STATUS.md` は **現在コードに何が存在するか**を示します。

たとえば v0.7 EC2 adapter が実装済みでも real AWS acceptance は別途必要です。同様に v0.8 の `haco-vscode` が実装済みでも、real Windows/WSL + Incus + VS Code Remote-SSH acceptance は対応環境で確認する必要があります。

v0.9 の spec が存在していても、`haco image` や `haco create --base` が実装されるまでは implementation 済みとは扱いません。

## Breaking Change

Hacocoon は pre-1.0 であり、simplification / hardening を継続します。

Breaking Change の対象には CLI、helper binary、state、Core / adapter API、Capability / Policy、Provider、Client Adapter configuration、Base/image lifecycle、experimental backend、document structure が含まれます。

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
