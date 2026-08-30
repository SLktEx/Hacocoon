# リリースのセキュリティと provenance

日本語 | [**English**](RELEASE_SECURITY.md)

Hacocoon の release verification は、整合性・source authorization・権限分離・provenance を別々の層として扱います。

現在の repository model は意図的に **solo maintainer / external contribution closed** です。external Pull Request は無効で、trusted repository write authority は repository owner だけが持ちます。詳細は [Public repository セキュリティチェックリスト](PUBLIC_RELEASE_CHECKLIST.ja.md) を参照してください。

## 1. SHA-256 による整合性

GoReleaser は `checksums.txt` を公開し、`scripts/install.sh` は archive を展開・install する前に SHA-256 を検証します。

これは通信途中の破損や取り違えを検出します。ただし artifact と checksum は同じ release authority が公開するため、publisher の真正性を独立して証明するものではありません。

## 2. 公式 release の source authorization

release workflow は `v*` tag push から直接起動しません。

`.github/workflows/release.yml` は trusted default branch 上の `repository_dispatch` event で起動し、trusted control-plane checkout が `tools/check_release_tag_trust.sh` で要求された tag を検証します。

official release tag は **remote default branch の現在 HEAD** と一致しなければなりません。さらに release SHA は dispatcher の `GITHUB_SHA` と一致する必要があります。

これにより次を拒否します。

- trusted `main` に入っていない detached commit
- 古い `main` commit に新しい version tag を付ける historical rollback release

authorization 後にだけ build job が exact approved release SHA を checkout します。

```text
repository_dispatch(release, tag=vX.Y.Z)
       |
       v
trusted current-main workflow + trust checker
       |
       +--> tag -> current remote main HEAD を必須化
       +--> release SHA == dispatcher GITHUB_SHA を必須化
       |
       v
exact authorized release commit を checkout
       |
       v
read-only build / test / package
       |
       v
same-run release payload
       |
       v
GitHub Environment: release
       |
       v
最小権限の publisher
  main + tag identity を再検証
  exact payload を attest
  GitHub Release を publish
```

署名・publication の直前にも publisher は次を確認します。

1. current default-branch HEAD が authorized release SHA とまだ一致する
2. annotated tag を peel した release tag が同じ SHA とまだ一致する

build 後にどちらかの identity が変わった場合は fail closed します。

## 3. Solo-maintainer authorization model

`publish` job は `release` GitHub Environment を参照しますが、現在の repository では **independent Environment reviewer を必須にしません**。

trusted maintainer が 1 人しかいないため、独立した承認を行える 2 人目の人間が存在しません。required reviewer を必須化すると release operation が deadlock するか bypass が必要になり、どちらも実質的な trust boundary にはなりません。

現在は次を組み合わせて authorization boundary を作ります。

- external Pull Request creation は disabled (`collaborators_only`)
- owner 以外の direct collaborator は存在しない
- `main` は protected で、required CI を通る PR 経由の変更だけを受ける
- release tag は作成後に move / delete できない
- release tag は current trusted `main` HEAD と一致するときだけ accepted
- build/test execution と write-capable publisher を分離
- publication 直前に tag / main identity を再検証
- published artifact に attestation を付与

`release` Environment 自体は名前付き privilege boundary と、将来 protection を追加する場所として維持します。

2 人目の trusted maintainer、または別の write-capable collaborator を追加する場合は、その multi-maintainer model を同等に trusted と扱う前に independent required reviewer を追加し、利用可能なら prevent self-review を有効化します。

release request の例:

```bash
gh api --method POST repos/SLktEx/Hacocoon/dispatches \
  -f event_type=release \
  -F 'client_payload[tag]=v0.8.0'
```

現在の solo-maintainer model では dispatch authority と最終的な human authority は同じ trusted owner に属します。それでも workflow は publication credential を持つコード量を最小化します。

## 4. GitHub / Sigstore attestation

Public Hacocoon では privileged publisher が exact release payload に 2 種類の attestation を生成します。

1. standard GitHub/Sigstore build provenance: artifact digest を期待する repository/workflow execution と結び付ける
2. Hacocoon release-binding attestation (`https://hacocoon.dev/attestations/release/v1`): release tag、release commit SHA、trusted control ref/SHA、`release` Environment identity を記録する

build job は `contents: read` のみです。publication に必要な write / OIDC / attestation 権限は publisher だけが持ちます。publisher は repository source を checkout せず、repository の test/build script も実行しません。

trusted Action は immutable full commit SHA pin を維持します。

### Archive verification

current GitHub CLI を使います。

```bash
gh attestation verify ./haco_linux_amd64.tar.gz \
  --repo SLktEx/Hacocoon \
  --signer-workflow SLktEx/Hacocoon/.github/workflows/release.yml \
  --source-ref refs/heads/main \
  --deny-self-hosted-runners
```

明示的な release binding も確認できます。

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

### Installer behavior

Linux installer は SHA-256 integrity と trusted GitHub/Sigstore provenance を標準で必須にします。`latest` は明示的な version tag に解決した後、release binding を検証してから install します。

`HACO_REQUIRE_PROVENANCE=0` は private/development 用の明示的な escape hatch です。

Windows installer も `latest` を解決し、downloaded release input を trusted provenance で検証し、Linux installer を provenance verification 有効のまま呼び出します。

## 5. Immutable Releases

Artifact provenance は差し替えを検出可能にします。GitHub Immutable Releases はさらに、published release asset と関連 tag の supported mutation を server-side で防ぎます。

official release を stable distribution channel として扱う前に release immutability を有効化してください。

有効化後は current GitHub CLI で release-level attestation も確認できます。

```bash
gh release verify v0.8.0 --repo SLktEx/Hacocoon
gh release verify-asset v0.8.0 ./haco_linux_amd64.tar.gz --repo SLktEx/Hacocoon
```

## 各チェックが保証するもの

| チェック | 主に保証するもの |
|---|---|
| `checksums.txt` | 同じ release authority を基準にした転送・ファイル整合性 |
| current-main tag authorization | detached commit と historical-main rollback release の拒否 |
| contribution-closed solo-maintainer policy | trusted repository write authority を owner に限定 |
| build/publish job split | release write authority を持つコード量を最小化 |
| publish 前の main/tag 再検証 | build と publication の間の default branch / tag movement を検出 |
| standard artifact attestation | artifact digest と期待 repository/workflow execution の紐付け |
| release-binding attestation | release tag / commit / control identity / release Environment identity の署名 |
| Immutable Release | supported path での公開後 release/tag mutation を防止 |

これらは相互補完です。external PR の有効化、別 write-capable actor の追加、self-hosted runner の追加前には trust model を再監査します。
