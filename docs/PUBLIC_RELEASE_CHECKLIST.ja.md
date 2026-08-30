# Public repository セキュリティチェックリスト

[English](PUBLIC_RELEASE_CHECKLIST.md) | **日本語**

Hacocoon は public repository ですが、現在の運用方針は意図的に **solo maintainer / external contribution closed** です。

目標は「public OSS なら誰でも PR を送れるようにする」ことではありません。現在の trust boundary は次です。

```text
誰でも read / fork / Issue 作成はできる
        |
        X  upstream への external PR は受け付けない
        |
trusted write authority は repository owner だけ
        |
protected main + required CI
        |
trusted release workflow
```

`tools/check_public_release_readiness.py` は、この **現在の運用モデルそのもの** を検証します。maintainer / contribution model を変更する場合は、checker を弱める前に threat model を更新してください。

## 1. Contribution boundary

現在の必須条件:

- [ ] repository は public
- [ ] `pull_request_creation_policy = collaborators_only`
- [ ] owner 以外の direct collaborator が存在しない
- [ ] external user は upstream repository に Pull Request を作成できない
- [ ] bug report / discussion 用の Issue は public のままでよい

これは、external fork PR を受け入れる前提だった以前の public-launch 設計より意図的に閉じた構成です。

external PR を有効化する前には、fork workflow approval、token permission、secret、runner 選択、workflow file 変更、review ownership を含む security review をやり直します。

## 2. `Protect main` ruleset

active branch ruleset の exact name:

```text
Protect main
```

default branch を対象にし、最低限次を enforce します。

- [ ] enforcement が active
- [ ] bypass actor なし
- [ ] branch deletion を禁止
- [ ] non-fast-forward / force push を禁止
- [ ] `main` への変更は Pull Request 経由
- [ ] new push 後に stale review state を無効化
- [ ] solo-maintainer の間は `require_last_push_approval = false`
- [ ] review thread の解決を必須化
- [ ] latest base branch に対する required status checks を必須化

Required status contexts:

```text
docs
workflow-policy
release-config
test (1.26.x)
test (1.27.x)
race
e2e
```

`gitleaks` も required status にすることを強く推奨します。required status に含めない場合でも dedicated secret-scan workflow 自体は維持します。

### approving review が 0 でもよい理由

現在 Hacocoon の trusted maintainer は 1 人です。independent approving review を必須化すると、2 人目の trusted maintainer または bypass を追加しない限り通常の maintenance ができなくなります。

そのため `required_approving_review_count = 0` は **solo-maintainer 専用の明示的な例外**です。一般的な推奨値ではありません。同じ理由で `require_last_push_approval` も `false` のままにします。GitHub では latest-push approval は「最後に push した本人以外」の承認が必要なため、1 人運用で有効にすると自分で作った PR を merge できなくなります。

この例外を許容する条件は次の両方です。

- external PR creation が disabled のまま
- owner 以外の direct collaborator が存在しない

どちらかが変わったら、人手 review と latest-push approval semantics を新しい trusted actor を追加する前に再設計します。

## 3. `Protect release tags` ruleset

active tag ruleset の exact name:

```text
Protect release tags
```

対象:

```text
refs/tags/v*
```

現在の必須 protection:

- [ ] enforcement が active
- [ ] bypass actor なし
- [ ] tag deletion を禁止
- [ ] tag update / movement を禁止
- [ ] non-fast-forward movement を禁止

現在は owner 以外に repository write authority を持つ collaborator がいないため、tag creation 自体は別ルールで restrict していません。

ただし、**tag を作れることは release authority ではありません**。release workflow は要求された tag が current trusted `main` HEAD と一致することを別途必須化し、publication 直前にも tag / main identity を再検証します。

write-capable collaborator や release bot を追加する場合は、その actor を trusted にする前に tag creation authority を再設計します。

## 4. `release` GitHub Environment

privileged publish job は引き続き次の Environment を使用します。

```text
release
```

現在は single-maintainer repository のため required reviewer は必須にしません。独立した trust decision を行える 2 人目の人間がいないためです。

Environment は privilege boundary の名前付き領域、および将来 protection を追加する場所として維持します。

2 人目の trusted maintainer を追加した時点で、independent required reviewer と、利用可能なら prevent self-review を設定します。

## 5. Self-hosted runner

現在の policy:

- [ ] repository self-hosted runner count は exact `0`
- [ ] normal CI は approved GitHub-hosted runner のみ
- [ ] SSH / AWS / GitHub credential、Incus / Docker / containerd authority、internal-network access 等を持つ persistent host を repository workflow から選択できない
- [ ] organization-owned repository に移行する場合は visible runner group を再監査

privileged Incus / cloud E2E を動かすためだけに self-hosted runner を通常 CI へ追加しません。将来必要になった場合は別の trusted execution boundary を設計します。

## 6. Release workflow trust

official release では次を維持します。

- [ ] `repository_dispatch` が default branch 上の trusted workflow を実行
- [ ] requested release tag は current `main` HEAD と一致必須
- [ ] build/test job は repository read-only
- [ ] publish job は別 job かつ最小権限
- [ ] publish 直前に current main / tag identity を再検証
- [ ] release payload に GitHub/Sigstore attestation を付与
- [ ] trusted workflow の Action は immutable commit SHA pin を維持

詳細は [Release security](RELEASE_SECURITY.ja.md) を参照してください。

## 7. Live readiness check

ruleset、collaborator、Environment、runner inventory を読める認証済み GitHub CLI で実行します。

```bash
python3 tools/check_public_release_readiness.py --repo SLktEx/Hacocoon
```

期待結果:

```text
PUBLIC RELEASE READINESS OK
```

warning は solo-maintainer として意図的に受け入れている tradeoff、または defense in depth の推奨項目を示します。API permission 不足は success ではなく `UNVERIFIED` です。

## 8. 再監査が必要になる変更

次のいずれかを行う前に現在の policy を再評価します。

- external Pull Request を有効化
- owner 以外の direct collaborator を追加
- 2 人目の trusted maintainer に write authority を付与
- bot / app に広い write / release authority を付与
- self-hosted runner / runner group を追加
- repository を organization へ移行
- release trigger / publisher authority model を変更

public であること自体が trust boundary ではありません。**trusted history を誰が変更できるか、untrusted code がどこで実行できるか**が trust boundary です。
