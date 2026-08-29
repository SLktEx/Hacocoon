# Hacocoon ドキュメント案内

[English](README.md) | **日本語**

このファイルは、2026-08-29 の architecture rebaseline と、その後の v0.7 までの実装進行を踏まえて、Hacocoon の資料をどの順番で読めばよいかを日本語で説明します。

> [!NOTE]
> 日本語資料は読みやすさのための補助資料です。設計上の最終的な正本は英語版の各 authoritative document です。

Hacocoon はまだ **pre-1.0** です。資料に現在の architecture や実装済み roadmap が書かれていても、API / CLI / state format / provider / configuration の互換性を保証するものではありません。

## まず日本語で読むなら

次の3つから読めば、ほぼ全体像が分かります。

1. [`../README.ja.md`](../README.ja.md) — Hacocoon が何者か、Quick Start、現在の機能。
2. [`ARCHITECTURE_GUIDE.ja.md`](ARCHITECTURE_GUIDE.ja.md) — architecture、security boundary、v0.1〜v0.7 の流れ。
3. [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md) — `main` に今なにが実装されていて、何が未検証か。

詳細な設計判断が必要になったら、対応する英語版 authoritative document を参照してください。

## 正本の優先順位

資料同士が矛盾して見える場合、英語版の正本は次の順番で優先します。

1. `00_REBASELINE_AND_ROADMAP.md` — product boundary と roadmap progression。
2. `00C_TERMINOLOGY_AND_BOUNDARIES.md` — architecture 用語と responsibility boundary。
3. `00B_SECURITY_ARCHITECTURE.md` — trust / security の横断ルール。
4. `IMPLEMENTATION_STATUS.md` — `main` が実際に何を実装しており、何が未 acceptance か。
5. `01_...`〜`07_...` — 各 roadmap stage の versioned design contract。
6. `CLIENT_ACCESS.md` / `REMOTE_CLOUD_PROVISIONING.md` 等 — 個別領域の詳細 contract。
7. `00A_PLUGIN_ARCHITECTURE.md` — extension / adapter guidance。
8. `90_CODEX_IMPLEMENTATION_HANDOFF.md` — 現在の実装・maintenance workflow。
9. `91_IMPLEMENTATION_REFERENCE_NOTES.md` — non-normative な外部 reference と historical notes。
10. `adr/` 配下 — 個別 decision。

`README.md`、`CODEX_START_HERE.md`、`Hacocoon_v0.1-v0.7_MASTER.md` は入口です。Architecture を再定義する資料ではありません。

## 現在のドキュメントの読み方

最初の rebaseline 資料は v0.1 が active implementation gate だった時期に書かれました。

現在の `main` は v0.7 implementation pass まで進んでいます。

そのため現在は次のように読みます。

- v0.1〜v0.7 spec は **versioned design contract**。
- `IMPLEMENTATION_STATUS.md` が現在の repository reality。
- roadmap gate が実装済みでも public surface が固定されたわけではない。
- real-provider acceptance と unit / integration / fake-provider E2E / race / vet / build / CI は別物。
- EC2 は実装されていても experimental / disabled by default のまま。
- v0.7 まで実装したからといって、勝手に v0.8 以降の scope を invent してはいけない。

## Specification と Implementation の違い

Release specification は、その roadmap stage で「どういう設計・acceptance を目指したか」を示します。

`IMPLEMENTATION_STATUS.md` は **現在コードに何が存在するか**を示します。

この2つは意図的に別の claim です。

たとえば v0.7 の EC2 adapter はコードとして存在しますが、real AWS acceptance はまだ別途必要です。

逆に repository に historical package が残っていても、それだけで current supported architecture になるわけではありません。

## Breaking Change

Hacocoon は pre-1.0 であり、まだ simplification / hardening を続けています。

Breaking Change の対象になり得るもの:

- CLI command / flag / output
- persisted state / migration behavior
- Core / adapter API
- Capability / Policy contract
- Provider configuration
- experimental backend
- documentation structure

Architecture を弱くする accidental compatibility を守るより、責任分界を明確にして安全で小さい設計に置き換えることを優先します。

ただし Breaking Change の自由は silent data loss や security regression の免罪符ではありません。

Material な operator impact は明示し、安全な migration path を提供する場合は migration 方法も書きます。

## Historical material

- `Session`、Runtime/Storage-centric、plugin-heavy な旧コードは migration inventory として残る場合があります。
- 削除済み・superseded な資料が Git history や古い GitHub search index に出る場合があります。
- Btrfs / raw / QCOW2 等の historical experiment は、現在の spec / ADR が明示的に再導入しない限り roadmap commitment ではありません。
- 「v0.1 を終えるまで v0.2 以降を実装するな」という古い指示は historical であり、現在の `main` の状態を表していません。

## 日本語資料の保守方針

日本語版は日常的に読めることを優先します。

- README と implementation status は英語版の内容に追従する。
- Architecture 全体は `ARCHITECTURE_GUIDE.ja.md` にまとめる。
- Security-sensitive な細部や exact contract は英語版 authoritative document を優先する。
- 日本語版と英語版が矛盾して見える場合は英語版を正本とする。

## ドキュメント変更時のルール

1. まず authoritative document を更新する。
2. コード reality が変わったら `IMPLEMENTATION_STATUS.md` を更新する。
3. 入口資料が古くなったら README / CODEX / handoff を更新する。
4. 日本語版の summary も必要に応じて追従させる。
5. 実装済みという claim と real-provider acceptance 済みという claim を混ぜない。
6. experimental provider の default-off status を維持する。
7. `python tools/check_docs.py` を実行する。
8. implementation detail を accidental compatibility promise にしない。
