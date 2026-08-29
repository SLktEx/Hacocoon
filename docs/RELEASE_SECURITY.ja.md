# リリースのセキュリティと provenance

日本語 | [**English**](RELEASE_SECURITY.md)

Hacocoon の release では、integrity、provenance、publication control を別の層として扱います。それぞれ防ぐものが違います。

## trusted release authority

release tag を push した commit 自身に、publication workflow を選ばせません。

`.github/workflows/release.yml` は `release` という `repository_dispatch` で起動します。GitHub の `repository_dispatch` は repository の default branch を基準に workflow と `GITHUB_SHA` を選ぶため、release tag が指す commit の workflow 定義ではなく、trusted default-branch history の workflow が release を判断します。

request に渡せる source 情報は version 文字列だけです。branch や source SHA は指定できません。build source は dispatch 時点の trusted default-branch `GITHUB_SHA` に固定されます。

release request は次で送れます。

```bash
bash tools/request_release.sh v0.13.0
```

この helper は GitHub repository-dispatch API を呼ぶ operator 向けの convenience wrapper であり、trust root ではありません。trusted workflow 側で version、event ref、source SHA、default-branch ancestry、release tag の状態を独立して検証します。

remote の release tag は事前に存在していてはいけません。read-only build job は GoReleaser が version を認識できるよう local tag だけを作ります。privileged publish job は `--target "$GITHUB_SHA"` 付きで GitHub Release を作るため、remote tag が無ければ GitHub が trusted SHA に release tag を作成します。そのあと tag を commit に解決し直し、`GITHUB_SHA` と一致しなければ draft を publish しません。

```text
repository_dispatch(release, version only)
        |
        v
trusted default-branch workflow + SHA
        |
        v
read-only build job
  dispatch context 検証
  existing remote tag を拒否
  test / vet / package
        |
        v
同一 workflow run の artifact
        |
        v
release Environment
最小権限 publish job
  payload を attest
  trusted SHA に tag + draft 作成
  tag -> trusted SHA を再確認
  draft publish
```

build job は `contents: read` のみです。publish job だけが `id-token: write`、`attestations: write`、`contents: write` を持ち、repository を checkout したり build source を実行したりしません。release job は全体で直列化して、複数 version request の race を避けます。

publish job は GitHub Environment `release` を参照します。public release 前に、この Environment に reviewer と deployment restriction を設定してください。YAML に Environment 名を書くだけでは protection rule の代わりにはなりません。

`main` と release 関連の repository setting も保護が必要です。write 権限を持つ actor が侵害された場合、trusted `main` の release request 自体は送れる可能性があります。branch/ruleset、tag/release policy、release Environment review は引き続き defense in depth として重要です。

## 1. SHA-256 による整合性確認

GoReleaser は `checksums.txt` を公開し、`scripts/install.sh` は展開・インストール前に対象 archive の SHA-256 が一致することを確認します。

これは通信途中の破損や取り違えを検出できます。一方、archive と `checksums.txt` は同じ GitHub Release authority で公開されるため、publisher の真正性を独立して証明するものではありません。両方を書き換えられる authority が侵害された場合、悪意ある archive と checksum を一致させられます。

## 2. GitHub / Sigstore の build provenance

Hacocoon が public repository の場合、`.github/workflows/release.yml` は GitHub Release 作成前に、公開対象そのものへ GitHub artifact attestation を生成します。attestation は GitHub Actions OIDC と Sigstore を利用し、SLSA build provenance を持ちます。

`actions/attest`、`actions/upload-artifact`、`actions/download-artifact` は full commit SHA に pin します。

trusted release workflow は `main` から動くため、attestation の source ref は `refs/heads/main` です。workflow が作る release tag は同じ source commit を指します。したがって verification では trusted source ref と、release tag から解決した exact commit の両方を照合します。

### archive を検証する

新しい GitHub CLI を利用します。

```bash
source_sha="$(gh api repos/SLktEx/Hacocoon/commits/v0.13.0 --jq .sha)"

gh attestation verify ./haco_linux_amd64.tar.gz \
  --repo SLktEx/Hacocoon \
  --signer-workflow SLktEx/Hacocoon/.github/workflows/release.yml \
  --source-ref refs/heads/main \
  --source-digest "$source_sha" \
  --deny-self-hosted-runners
```

artifact digest、期待する repository と signer workflow、GitHub Actions OIDC identity、trusted `main` source ref、release tag が指す exact source commit を検証します。

### installer の挙動

installer は SHA-256 の一致を常に必須とします。artifact attestation 対応の GitHub CLI が使える場合は、`latest` 指定時も実際の release tag を解決し、その tag の source commit を解決してから provenance verification を試みます。

provenance を必須にして確認できない場合も fail closed にするには次のようにします。

```bash
HACO_REQUIRE_PROVENANCE=1 ./install.sh v0.13.0
```

strict provenance mode では release identity の解決と検証に必要な GitHub CLI/API access が必要です。

## private repository の制約

GitHub artifact attestation は public repository では現在の GitHub plan で利用できます。private / internal repository で利用するには GitHub Enterprise Cloud が必要です。

Hacocoon が private の間は、SHA-256 を publisher authenticity と誤認させるのではなく、public 用 artifact-attestation step を明示的に skip します。repository を public にすると attestation は必須になり、生成に失敗した場合は release 作成まで進みません。

private の Enterprise Cloud 環境で attestation を使う場合は、private attestation を有効にしたうえで visibility gate を明示的に見直します。

## Immutable Releases

Artifact provenance があれば、release asset が差し替えられても署名済み digest と一致しないため検証に失敗します。GitHub Immutable Releases はさらに、公開済み release asset と関連 tag の変更・削除そのものを server side で禁止します。

最初の public release 前に次を有効にしてください。

```text
Repository Settings
  -> Releases
  -> Enable release immutability
```

この設定には repository administration authority が必要です。長期 admin token を release workflow に持たせることはせず、意図的に repository setting として管理します。

Hacocoon の workflow は Immutable Releases と両立するよう、asset が揃った draft を作成し、生成された tag が trusted source SHA を指すことを確認してから publish します。immutability は publish 後に適用されます。

Immutable Releases が有効なら、新しい GitHub CLI で GitHub の release-level attestation も検証できます。

```bash
gh release verify v0.13.0 --repo SLktEx/Hacocoon
gh release verify-asset v0.13.0 ./haco_linux_amd64.tar.gz --repo SLktEx/Hacocoon
```

## 各チェックが保証するもの

| チェック | 破損検知 | repository / workflow の確認 | source の確認 | 公開後の差し替え防止 |
|---|---:|---:|---:|---:|
| `checksums.txt` | yes | no | no | no |
| Artifact attestation | yes | yes | `main` + exact commit digest | 差し替えると検証失敗 |
| Trusted release dispatch | n/a | workflow は default branch 由来 | dispatch `GITHUB_SHA` に固定 | n/a |
| Immutable Release | yes | GitHub release identity | associated release tag | yes |

public Hacocoon release では `main` を保護し、`release` Environment と tag/release rule を設定し、artifact attestation を publisher/build provenance として使い、Immutable Releases も defense in depth として有効化します。
