# リリースのセキュリティと provenance

日本語 | [**English**](RELEASE_SECURITY.md)

Hacocoon のリリース検証には、役割の異なる 2 つの層があります。両者を同じものとして扱いません。

## 1. SHA-256 による整合性確認

GoReleaser は `checksums.txt` を公開し、`scripts/install.sh` は展開・インストール前に対象アーカイブの SHA-256 が一致することを確認します。

これは通信途中の破損や取り違えを検出できます。一方、アーカイブと `checksums.txt` は同じ GitHub Release の権限で公開されるため、publisher の真正性を独立して証明するものではありません。両方を書き換えられる権限が侵害された場合、悪意あるアーカイブと悪意ある checksum を一致させられます。

## 2. GitHub / Sigstore の build provenance

Hacocoon が public repository の場合、`.github/workflows/release.yml` は GitHub Release を公開する前に、公開対象そのものへ GitHub artifact attestation を生成します。attestation は GitHub Actions OIDC と Sigstore を利用し、SLSA build provenance を持ちます。

release workflow は build 権限と署名・公開権限を分離します。

```text
main 上の信頼済み tag
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
  公開対象を attest
  draft release 作成
  全 asset を添付
  draft を publish
```

build job は `contents: read` のみです。publish job だけが `id-token: write`、`attestations: write`、`contents: write` を持ちます。publish job は repository を checkout せず、repository の build code も実行しません。

`actions/attest`、`actions/upload-artifact`、`actions/download-artifact` は full commit SHA に pin します。

### アーカイブを検証する

新しい GitHub CLI を利用します。

```bash
gh attestation verify ./haco_linux_amd64.tar.gz \
  --repo SLktEx/Hacocoon \
  --signer-workflow SLktEx/Hacocoon/.github/workflows/release.yml \
  --source-ref refs/tags/v0.8.0 \
  --deny-self-hosted-runners
```

ダウンロードした release tag に置き換えてください。artifact digest に加え、期待する repository、署名 workflow、tag ref、GitHub Actions OIDC identity、provenance signature chain を検証します。

`latest` を tag 固定なしで検証する場合は `--source-ref` を省略できますが、明示した release tag を指定するほうが強い検証になります。

### installer の挙動

installer は SHA-256 の一致を常に必須とします。artifact attestation に対応した GitHub CLI が利用できる場合は、展開前に provenance 検証も試みます。

provenance を必須にして、確認できない場合も fail closed にするには次のようにします。

```bash
HACO_REQUIRE_PROVENANCE=1 ./install.sh v0.8.0
```

strict provenance を使う場合は、`refs/tags/<version>` まで検証できるよう明示的な version を指定することを推奨します。

## private repository の制約

GitHub artifact attestation は public repository では現在の GitHub plan で利用できます。private / internal repository で利用するには GitHub Enterprise Cloud が必要です。

Hacocoon が private の間は、SHA-256 を publisher authenticity と誤認させるのではなく、public 用 artifact-attestation step を明示的に skip します。repository を public にすると attestation は必須になり、生成に失敗した場合は publish step まで進みません。

private の Enterprise Cloud 環境で attestation を使う場合は、private attestation を有効にしたうえで visibility gate を明示的に見直します。

## Immutable Releases

Artifact provenance があれば、release asset が差し替えられても署名済み digest と一致しないため検証に失敗します。GitHub Immutable Releases はさらに、公開済み release asset と関連 tag の変更・削除そのものをサーバー側で禁止します。

最初の public release 前に次を有効にしてください。

```text
Repository Settings
  -> Releases
  -> Enable release immutability
```

この設定には repository administration 権限が必要です。長期 admin token を release workflow に持たせることはせず、意図的に repository setting として管理します。

Hacocoon の workflow は Immutable Releases と両立するよう、最初に draft を作成し、全 asset を添付してから publish します。immutability は publish 後に適用されます。

Immutable Releases が有効なら、新しい GitHub CLI で GitHub の release-level attestation も検証できます。

```bash
gh release verify v0.8.0 --repo SLktEx/Hacocoon
gh release verify-asset v0.8.0 ./haco_linux_amd64.tar.gz --repo SLktEx/Hacocoon
```

## 各チェックが保証するもの

| チェック | 破損検知 | repository / workflow の確認 | artifact digest | 公開後の差し替え防止 |
|---|---:|---:|---:|---:|
| `checksums.txt` | yes | no | yes（同じ権限元） | no |
| Artifact attestation | yes | yes | yes（署名済 provenance） | 差し替えると検証失敗 |
| Immutable Release | yes | GitHub release identity | yes（release attestation） | yes |

public Hacocoon release では artifact attestation を publisher / build provenance の層として使い、Immutable Releases も defense in depth として有効化します。
