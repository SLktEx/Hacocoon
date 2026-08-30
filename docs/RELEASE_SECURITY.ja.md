# リリースのセキュリティと provenance

日本語 | [**English**](RELEASE_SECURITY.md)

Hacocoon のリリース検証は、整合性・公開許可・provenance を別々の層として扱います。それぞれ守る対象が異なります。

## 1. SHA-256 による整合性確認

GoReleaser は `checksums.txt` を公開し、`scripts/install.sh` は展開・インストール前に対象アーカイブの SHA-256 が一致することを確認します。

これは通信途中の破損や取り違えを検出できます。一方、アーカイブと `checksums.txt` は同じ GitHub Release の権限で公開されるため、publisher の真正性を独立して証明するものではありません。

## 2. 公式リリースの許可境界

release workflow は、`v*` tag の push では起動しません。tag が指す commit 自身に「この tag を公式リリースしてよいか」を判断する workflow や trust checker を差し替えさせないためです。

`.github/workflows/release.yml` は `release` という `repository_dispatch` event で起動します。GitHub の `repository_dispatch` は repository の default branch 上の workflow を実行します。trusted control-plane は dispatcher の `GITHUB_SHA` を checkout し、その trusted な `tools/check_release_tag_trust.sh` で要求された tag を検証します。

公式リリースの tag は、単に trusted `main` 履歴内にあるだけでは不十分で、**remote default branch の現在 HEAD と完全一致**する必要があります。さらに解決した release SHA は dispatcher の `GITHUB_SHA` と一致しなければなりません。これにより、write 権限を持つ actor が古い脆弱な `main` commit に新しい version tag を付けて、正しい provenance 付きの rollback release を作る経路を拒否します。

その後にだけ、2 回目の checkout でリリース対象をその exact SHA から取得します。

```text
repository_dispatch(release, tag=vX.Y.Z)
       |
       v
trusted current main の workflow + trust checker
       |
       +--> tag -> remote main の現在 HEAD を必須化
       +--> release SHA == dispatcher GITHUB_SHA を必須化
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
GitHub Environment: release
  必須の人手承認（repository setting）
       |
       v
最小権限の publish job
  main HEAD が今も同じ commit か確認
  tag を再解決して同じ commit か確認
  公開物を attest
  GitHub Release を publish
```

`publish` job は `release` という専用 GitHub Environment を参照します。ただし、**YAML に `environment: release` と書くだけでは human approval の security boundary にはなりません。** Public 化前後の GitHub repository settings で、この Environment に required reviewer を設定し、利用できる場合は prevent self-review も有効にしてください。reviewer 集合は通常の repository write actor より狭くする必要があります。

GitHub の現在の仕様では、Free / Pro / Team の required Environment reviewers は public repository でのみ利用できます。したがって、private repository の現状では `release` Environment 名が workflow に存在するだけで human authorization 要件を満たしたとは扱いません。Public 化後、最初の公式 public release より前に Environment protection を設定・再確認することが publication checklist の必須条件です。

署名・公開の直前にも publisher が GitHub API を使い、次の 2 点を再確認します。

1. default branch の現在 HEAD が、build 時に許可した release SHA とまだ一致していること
2. release tag を annotated tag まで peel した結果が、同じ release SHA とまだ一致していること

build 後に `main` が進んだ場合、tag が動いた場合、または identity が変わった場合は fail closed とし、新しい current HEAD から release request をやり直します。

release を要求する例です。

```bash
gh api --method POST repos/SLktEx/Hacocoon/dispatches \
  -f event_type=release \
  -F 'client_payload[tag]=v0.8.0'
```

repository dispatch を送れることと、公式 release を publish できることは別の権限です。privileged publisher の別系統の human authorization boundary が server-side の `release` Environment approval です。

この workflow 側の対策だけで GitHub repository 設定の代わりにはなりません。Public 化前には `main` の保護、release tag の作成・更新・削除制限、write/bypass actor の最小化、`release` Environment reviewer の設定、public fork 用 runner policy が別途必要です。

## 3. GitHub / Sigstore attestation

Public Hacocoon では、publish job が公開対象そのものに 2 種類の attestation を生成します。

1. standard GitHub/Sigstore build provenance: trusted `main` から実行された期待する Hacocoon release workflow が artifact を署名したことを検証するもの
2. Hacocoon release-binding attestation (`https://hacocoon.dev/attestations/release/v1`): trusted release control-plane が許可した release tag、release commit SHA、control ref、control SHA、期待する `release` authorization Environment 名を署名するもの

build job の権限は `contents: read` のみです。publish job だけが `contents: write`、`id-token: write`、`attestations: write`、`artifact-metadata: write` を持ちます。publish job は repository source を checkout せず、repository の test/build script も実行しません。Environment protection は privileged publish job が runner に送られる前に通過する必要があります。

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

predicate には trusted release control-plane が選んだ `tag`、`commit`、control identity、期待する authorization Environment が記録されます。

### installer の挙動

Linux installer は SHA-256 の一致に加えて、**trusted GitHub/Sigstore provenance を標準で必須**とします。`latest` は最初に明示的な `vX.Y.Z` tag へ解決し、その tag に対して trusted release workflow、`refs/heads/main`、signed release-binding predicate を検証してからインストールします。

`HACO_REQUIRE_PROVENANCE=0` は private/development 用の明示的な escape hatch としてのみ残しています。public install の標準 trust model ではありません。

Windows installer も `latest` を明示 tag に解決し、`checksums.txt`、`bootstrap-wsl.sh`、`install.sh` をそれぞれ trusted provenance と signed release binding で検証してから downloaded script を実行します。Linux installer を呼ぶ際も `HACO_REQUIRE_PROVENANCE=1` を強制します。

## private repository の制約

GitHub artifact attestation は public repository では現在の GitHub plan で利用できます。private / internal repository で利用するには GitHub Enterprise Cloud が必要です。

Hacocoon が private の間は、SHA-256 を publisher authenticity と誤認させず、public 用 attestation を skip します。repository を public にすると 2 種類の attestation は必須になり、生成に失敗した場合は publication まで進みません。

human approval にも同じ publication boundary があります。GitHub Free / Pro / Team では required Environment reviewers は public repository でのみ利用できます。Public 化後、最初の公式 public release の前に `release` Environment の required reviewer を設定・検証してください。

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
| current-main tag authorization | detached commit と historical-main rollback release の拒否 |
| `release` Environment required reviewer | 通常の repository write / dispatch authority と human publication approval の分離 |
| publish 前の main/tag 再検証 | build と publish の間の default branch / tag 差し替え検出 |
| standard artifact attestation | artifact digest と期待 repository/workflow・trusted-main 実行の紐付け |
| release-binding attestation | 許可済み release tag / commit / control identity / expected authorization Environment の署名 |
| Immutable Release | 対応範囲で公開後の release/tag 変更を server-side で防止 |

これらは相互補完です。protected `main`、release tag 制限、安全な fork-PR 設定、runner isolation の代わりにはなりません。
