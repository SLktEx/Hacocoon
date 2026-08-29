# リリースのセキュリティと provenance

日本語 | [**English**](RELEASE_SECURITY.md)

Hacocoon のリリース検証は、整合性・公開許可・provenance を別々の層として扱います。それぞれ守る対象が異なります。

## 1. SHA-256 による整合性確認

GoReleaser は `checksums.txt` を公開し、`scripts/install.sh` は展開・インストール前に対象アーカイブの SHA-256 が一致することを確認します。

これは通信途中の破損や取り違えを検出できます。一方、アーカイブと `checksums.txt` は同じ GitHub Release の権限で公開されるため、publisher の真正性を独立して証明するものではありません。

## 2. リリースの許可判断は trusted `main` が行う

release workflow は、`v*` tag の push では起動しません。tag が指す commit 自身に「この tag を公式リリースしてよいか」を判断する workflow や trust checker を差し替えさせないためです。

`.github/workflows/release.yml` は `release` という `repository_dispatch` event で起動します。GitHub の `repository_dispatch` は repository の default branch 上の workflow を実行します。trusted control-plane は dispatcher の `GITHUB_SHA` を checkout し、その trusted な `tools/check_release_tag_trust.sh` で要求された tag を検証して、リリース対象として許可した commit SHA を固定します。

その後にだけ、2 回目の checkout でリリース対象をその SHA から取得します。

```text
repository_dispatch(release, tag=vX.Y.Z)
       |
       v
trusted main の workflow + trust checker
       |
       +--> tag -> commit が trusted main 履歴内か確認
       |
       v
許可された exact commit を checkout
       |
       v
read-only build job
  test / vet / package
       |
       v
同一 workflow run の artifact
       |
       v
最小権限の publish job
  tag を再解決して同じ commit か確認
  公開物を attest
  GitHub Release を publish
```

署名・公開の直前にも GitHub API から remote tag を再解決します。annotated tag も peel し、build job が許可した SHA と完全一致しなければ公開しません。build 後に tag が差し替えられた場合は fail closed になります。

許可された maintainer / automation から release を要求する例です。

```bash
gh api --method POST repos/SLktEx/Hacocoon/dispatches \
  -f event_type=release \
  -F 'client_payload[tag]=v0.8.0'
```

この workflow 側の対策だけで GitHub repository 設定の代わりにはなりません。Public 化前には `main` の保護、release tag の変更制限、write/bypass actor の最小化、public fork 用 runner policy が別途必要です。

## 3. GitHub / Sigstore attestation

Public Hacocoon では、publish job が公開対象そのものに 2 種類の attestation を生成します。

1. standard GitHub/Sigstore build provenance: trusted `main` から実行された期待する Hacocoon release workflow が artifact を署名したことを検証するもの
2. Hacocoon release-binding attestation (`https://hacocoon.dev/attestations/release/v1`): trusted release control-plane が許可した release tag、release commit SHA、control ref、control SHA を署名するもの

build job の権限は `contents: read` のみです。publish job だけが `contents: write`、`id-token: write`、`attestations: write`、`artifact-metadata: write` を持ちます。publish job は repository source を checkout せず、repository の test/build script も実行しません。

`actions/attest`、`actions/upload-artifact`、`actions/download-artifact` は full commit SHA に pin します。

### アーカイブを検証する

まず trusted `main` の release workflow による provenance を確認します。

```bash
gh attestation verify ./haco_linux_amd64.tar.gz \
  --repo SLktEx/Hacocoon \
  --signer-workflow SLktEx/Hacocoon/.github/workflows/release.yml \
  --source-ref refs/heads/main \
  --deny-self-hosted-runners
```

明示的な version を確認するときは、signed release binding も確認できます。

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

predicate には trusted release control-plane が選んだ `tag` と `commit` が記録されます。

### installer の挙動

installer は SHA-256 の一致を常に必須とします。artifact attestation に対応した GitHub CLI が利用できる場合は、trusted-main workflow provenance も検証します。明示的な version を指定した場合はさらに release-binding attestation の `tag` が要求した version と完全一致することを確認します。

provenance を必須にする場合は次のようにします。

```bash
HACO_REQUIRE_PROVENANCE=1 ./install.sh v0.8.0
```

strict provenance mode では明示的な version が必須です。`latest` では呼び出し側が独立した期待 tag を指定していないため、`HACO_REQUIRE_PROVENANCE=1` と `latest` の組み合わせは拒否します。

## private repository の制約

GitHub artifact attestation は public repository では現在の GitHub plan で利用できます。private / internal repository で利用するには GitHub Enterprise Cloud が必要です。

Hacocoon が private の間は、SHA-256 を publisher authenticity と誤認させず、public 用 attestation を skip します。repository を public にすると 2 種類の attestation は必須になり、生成に失敗した場合は publication まで進みません。

## Immutable Releases

Artifact provenance は asset の差し替えを検出可能にします。GitHub Immutable Releases はさらに、公開済み release asset と関連 tag の変更・削除を server-side で防ぎます。

最初の public release 前に次を有効にしてください。

```text
Repository Settings
  -> Releases
  -> Enable release immutability
```

この設定には repository administration 権限が必要で、長期 admin token を release workflow に持たせて自動化はしません。workflow は全 asset と attestation の準備後に populated draft を publish する構成です。

Immutable Releases が有効なら、新しい GitHub CLI で release-level attestation も検証できます。

```bash
gh release verify v0.8.0 --repo SLktEx/Hacocoon
gh release verify-asset v0.8.0 ./haco_linux_amd64.tar.gz --repo SLktEx/Hacocoon
```

## 各チェックが保証するもの

| チェック | 主に保証するもの |
|---|---|
| `checksums.txt` | 同じ release authority を基準にした転送・ファイル整合性 |
| trusted `repository_dispatch` control-plane | tag 側 source が自分自身の release authorizer を差し替えられないこと |
| publish 前の tag 再検証 | build と publish の間の tag 差し替え検出 |
| standard artifact attestation | artifact digest と期待 repository/workflow・trusted-main 実行の紐付け |
| release-binding attestation | 同じ artifact に対する許可済み release tag / commit identity の署名 |
| Immutable Release | 対応範囲で公開後の release/tag 変更を server-side で防止 |

これらは相互補完です。protected `main`、release tag 制限、安全な fork-PR 設定、runner isolation の代わりにはなりません。
