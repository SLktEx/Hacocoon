# リリースの安全性と出所の証明

日本語 | [English](release-security.md)

配布物の整合性、公開元の権限、実行権限の分離、出所の証明は別の検査です。現在は所有者一人が保守し、外部 PR を受け付けません。設定条件は[公開リポジトリの点検](../guides/releasing.ja.md)に集約します。

## SHA-256 による整合性

GoReleaser が `checksums.txt` を公開し、`scripts/install.sh` が展開・導入前に検証します。破損や取り違えは検出できますが、配布物とチェックサムを同じ権限で公開するため、公開者の真正性を独立して証明する検査ではありません。

## 公開を許可するコミット

`.github/workflows/release.yml` はタグの push から直接起動せず、既定ブランチの `repository_dispatch` で実行します。信頼する制御側のチェックアウトが `tools/check_release_tag_trust.sh` を実行し、要求したタグを確認します。

タグは現在のリモート既定ブランチ HEAD を指し、解決した SHA は起動元の `GITHUB_SHA` と一致する必要があります。main に入っていないコミットと、古い main に新しいタグを付けた巻き戻し公開を拒否します。

```text
現在の main から公開要求
  -> タグ・main・起動元 SHA の一致
  -> 正確なコミットを読み取り専用権限でビルド・テスト
  -> 同じ実行内の成果物を公開ジョブへ渡す
  -> main とタグを再確認
  -> 成果物に証明を付け、GitHub Release を公開
```

署名・公開の直前にも、現在の main HEAD と承認した SHA、注釈付きタグを解決した SHA を照合します。ビルド中に変わった場合は拒否します。

## 公開者の権限

`publish` ジョブは `release` GitHub Environment を使います。一人運用では独立した必須承認者を設けません。二人目がいない状態で要求しても、公開不能になるか例外が必要になるためです。

この例外は、外部 PR 禁止、所有者以外の共同作業者なし、main の PR・CI 保護、タグの変更・削除禁止、現在の main だけの公開、ビルドと公開の分離、公開直前の再確認、成果物の証明を組み合わせて成立します。Environment は名前付きの権限境界として維持します。別の書き込み権限を追加する前に独立した承認者と、対応していれば自己承認禁止を設定します。

公開要求の例です。これは操作説明であり、文書検証が公開を実行することはありません。

```bash
gh api --method POST repos/SLktEx/Hacocoon/dispatches \
  -f event_type=release \
  -F 'client_payload[tag]=v0.8.0'
```

一人運用では起動と最終判断を同じ所有者が行います。それでも公開用の認証情報を持つコード量を最小化します。

## GitHub／Sigstore による証明

公開ジョブは正確な成果物へ二種類の証明を付けます。

1. 通常のビルド出所証明。成果物のダイジェストと、期待するリポジトリ・ワークフローの実行を結び付けます。
2. Hacocoon の公開識別証明（`https://hacocoon.dev/attestations/release/v1`）。タグ、コミット、信頼する制御側の参照・SHA、`release` Environment を記録します。

ビルドジョブは `contents: read` だけを持ちます。公開に必要な書き込み・OIDC・証明権限は公開ジョブだけに与えます。公開ジョブはソースをチェックアウトせず、リポジトリのビルド・テストを実行しません。外部 Action は完全なコミット SHA に固定します。

現在の GitHub CLI でアーカイブを検証できます。

```bash
gh attestation verify ./haco_linux_amd64.tar.gz \
  --repo SLktEx/Hacocoon \
  --signer-workflow SLktEx/Hacocoon/.github/workflows/release.yml \
  --source-ref refs/heads/main \
  --deny-self-hosted-runners
```

公開識別の証明を明示的に確認する場合:

```bash
gh attestation verify ./haco_linux_amd64.tar.gz \
  --repo SLktEx/Hacocoon \
  --signer-workflow SLktEx/Hacocoon/.github/workflows/release.yml \
  --source-ref refs/heads/main \
  --predicate-type https://hacocoon.dev/attestations/release/v1 \
  --deny-self-hosted-runners \
  --format json \
  --jq '.[].verificationResult.statement.predicate'
```

## インストーラーの検査

Linux インストーラーは SHA-256 と信頼する GitHub／Sigstore の出所証明を既定で要求します。`latest` は具体的なタグに解決し、公開識別を確認してから導入します。`HACO_REQUIRE_PROVENANCE=0` は非公開・開発用途で明示的に使う例外です。

Windows も `latest` と取得した入力の証明を確認し、Linux 側の証明検査を有効にしたまま呼び出します。

## 公開後の変更防止

成果物の証明は差し替えを検出可能にします。GitHub Immutable Releases は、公開したファイルと関連タグを通常のサーバー操作で変更できなくします。公式の安定した配布経路として使う前に有効にしてください。

有効化後はリリース単位でも確認できます。

```bash
gh release verify v0.8.0 --repo SLktEx/Hacocoon
gh release verify-asset v0.8.0 ./haco_linux_amd64.tar.gz --repo SLktEx/Hacocoon
```

チェックサム、現在の main への限定、権限分離、公開前の再確認、成果物・公開識別の証明、公開後の変更防止は相互に補完します。外部 PR、書き込み権限、self-hosted runner を追加する前に、全体の信頼境界を再監査します。
