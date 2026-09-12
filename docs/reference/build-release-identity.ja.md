# ビルド・チェックポイント・リリース・対応状況の識別

日本語 | [English](build-release-identity.md)

Hacocoon は開発の進捗、配布物の識別、実際に動作を確認した範囲を区別します。

## 開発チェックポイント

チェックポイントは pre-1.0 の `v0.N` という進捗の節目です。番号・現在値・Gate 名の機械可読な正本は [checkpoints.yaml](../status/checkpoints.yaml)、運用規則と履歴は[バージョン状況](../status/versioning-and-release-status.ja.md)、現在の機能と制約は[実装状況](../IMPLEMENTATION_STATUS.ja.md)です。

チェックポイントは main にまとまった機能・運用・観測・検証のどこまでが入ったかを示します。公開タグ、GitHub Release、互換性保証、過去の全実機試験の完了を意味しません。公開リリースより速く進んでも構いません。

## ソフトウェアのバージョン

ソフトウェアのバージョンはビルド済みの配布物を識別します。通常の `go build` は注入値がなければ `version: dev` を返します。GoReleaser はリンカーフラグでバージョン、コミット SHA、ビルド日時を注入します。公式公開の権限は[リリースの安全性](../security/release-security.ja.md)に従います。

タグ `v0.8.0` とチェックポイント `v0.8` は別です。`v0.26` の節目があっても `v0.26.0` の配布物があるとは限りません。

## 実機検証と対応状況

Host、Incus、ストレージ、WSL、クライアントの具体的な条件に対する証拠です。チェックポイントの実装が済んでも、実機条件に依存する未確認事項は残せます。[実装状況](../IMPLEMENTATION_STATUS.ja.md)から[検証証拠](../status/acceptance-evidence.ja.md)へ進んでください。

## 実行時に確認する

`haco version` は各情報を別項目で表示します。

```text
Hacocoon
  checkpoint: <development checkpoint>
  version: <software/release version or dev>
  commit: <source commit or unknown>
  built: <release build timestamp or unknown>
```

```bash
haco version --json
haco --version
```

JSON はツール向け、`--version` は短い表示です。どちらも Incus や Host 状態を初期化しません。

`internal/buildinfo/checkpoint_generated.go` は `tools/bump-milestone` が YAML から生成するビルド入力で、リリースの SemVer や独立した正本ではありません。

## チェックポイントを進める

```bash
tools/bump-milestone v0.N "Gate Name"
```

補助ツールは YAML の現在値から必ず次の番号だけを受け付けます。古い Markdown・ビルド入力を拒否し、Gate を追加し、英日版の現在値・一覧・生成コードを同期して文書検査を実行します。

YAML は番号と Gate の識別だけを持ちます。実装・部分実装・実機依存の状態は状況文書に残します。機械的な更新の後、実装状況と担当する設計・参照文書を実際のコードに合わせて仕上げます。

## PR の分類

PR は次の一つを選びます。

- 新しい開発チェックポイント。
- 現在のチェックポイント内の機能・堅牢化・検証。
- リリース・パッケージだけの変更。
- 文書・テスト・リファクタリング・保守だけの変更。

新しい節目では YAML と写しを同時に更新します。リリースだけの変更や文書整理を理由に、チェックポイントを暗黙に進めません。
