# Public repository 公開チェックリスト

[English](PUBLIC_RELEASE_CHECKLIST.md) | **日本語**

これは Hacocoon を private repository から public repository へ切り替えるための **fail-closed な公開手順**です。単なる作業メモではなく、release trust boundary の一部として扱います。

以下の必須項目がすべて完了し、live public repository に対して `tools/check_public_release_readiness.py` が `PUBLIC RELEASE READINESS OK` を返すまでは、**公式 public release を公開してはいけません**。

## なぜ Public 化の途中で Actions を無効化するのか

現在の GitHub Free では、この private repository のままでは必要な repository ruleset と Environment required reviewer を最終設定できません。そのため「public にした後でしか設定できない security control」があります。

public 化直後、server-side protection の設定が終わる前に external fork workflow が動ける時間を作らないため、次の順序を必須にします。

1. private のまま code-side hardening をすべて merge する。
2. 通常 CI と secret scan が green であることを確認する。
3. **repository の GitHub Actions を無効化する**。
4. repository を public に変更する。
5. Actions を無効のまま、以下の server-side control をすべて設定・検証する。
6. readiness checker を実行する。
7. checker が PASS した後でだけ Actions を再有効化する。

変換途中では external contribution の承認・merge、公式 release 作成、Actions 再有効化を行いません。

## 1. private のまま満たす事前条件

visibility を変える前に確認します。

- [ ] `main` CI が green。
- [ ] complete reachable Git history に対する `secret-scan / gitleaks` が green。
- [ ] 既知の未解決 Critical / High public-release code blocker がない。
- [ ] `.github/CODEOWNERS` が workflow、release config、installer、policy checker、この checklist を cover している。
- [ ] `.github/workflows/release.yml` の privileged `publish` job が専用 `release` Environment を使っている。
- [ ] 公式 release tag は current trusted `main` HEAD と一致するときだけ許可される。
- [ ] Linux / Windows public installer が provenance failure で fail closed する。
- [ ] Public 化の直前に GitHub Actions を無効化した。

## 2. Actions を無効のまま Public 化する

repository visibility を public に変更します。この時点では Actions を再有効化しません。

Public 化を契機に意図しない workflow run が開始されていないことも確認します。

## 3. `protect-main` repository ruleset

次の exact name で active ruleset を 1 個だけ作ります。

```text
protect-main
```

default branch (`main` / `~DEFAULT_BRANCH`) を対象にし、最低限以下を必須にします。

- [ ] enforcement: `active`
- [ ] target: branch
- [ ] bypass actor なし
- [ ] branch deletion を禁止
- [ ] non-fast-forward / force push を禁止
- [ ] merge 前に Pull Request を必須化
- [ ] approving review を最低 1 件必須化
- [ ] commit が追加されたら stale approval を無効化
- [ ] latest reviewable push の承認を必須化
- [ ] review thread の全解決を必須化
- [ ] latest base branch に対する required status checks を必須化
- [ ] 下記の現在の Hacocoon CI context をすべて required にする

Required status contexts:

```text
docs
workflow-policy
release-config
test (1.26.x)
test (1.27.x)
race
e2e
gitleaks
```

`require_code_owner_review` は、owner が作った変更を承認できる **2 人目の trusted reviewer** が存在するようになった時点で有効化するのを推奨します。1 人運用の状態で mandatory CODEOWNER review を有効化すると self-approval できず repository が詰む可能性があります。

一方、Public 化して external PR を受け入れる運用で「approval 1 件必須・bypass なし」を成立させるには、独立した trusted reviewer が最低 1 人必要です。

## 4. `protect-release-tags` repository ruleset

次の exact name で active ruleset を作ります。

```text
protect-release-tags
```

対象:

```text
refs/tags/v*
```

必須条件:

- [ ] enforcement: `active`
- [ ] target: tag
- [ ] tag creation を restrict
- [ ] tag deletion を禁止
- [ ] non-fast-forward tag update / tag movement を禁止
- [ ] 公式 release tag の作成を許可する bypass actor は、明示レビュー済みの **exactly one actor** に限定
- [ ] 通常の write collaborator に release-tag bypass を付けない

readiness checker は bypass actor type として `RepositoryRole` / `Integration` / `Team` を 1 個だけ許可し、実際の actor type を warning で表示します。personal repository の間は、最小の現実的な authority として repository administrator / owner role を使い、organization 移行や dedicated release integration 導入時には再設計してください。

## 5. `release` GitHub Environment

exact name:

```text
release
```

server-side protection:

- [ ] required reviewer を最低 1 人設定
- [ ] prevent self-review を有効化
- [ ] reviewer set を通常の repository write actor より狭くする
- [ ] 通常 release workflow に secret が不要なら Environment secret を置かない

workflow YAML に `environment: release` と書くだけでは不十分です。server-side required reviewer が `repository_dispatch` authority と official publication authority を分離する human authorization boundary です。

## 6. external contributor は毎回 workflow approval 必須

fork pull-request workflow setting を次にします。

```text
approval_policy = all_external_contributors
```

first-time contributor だけを対象にする設定は使いません。一度 harmless contribution が受け入れられたことを理由に、将来の attacker-controlled workflow を恒久的に自動許可しないためです。

## 7. Public fork PR が self-hosted runner に到達できないことを証明

public repository では次を必須にします。

- [ ] repository self-hosted runner count が exact `0`
- [ ] SSH / AWS / GitHub credential、Incus / Docker / containerd socket、internal network 等の authority を持つ persistent runner を fork PR から選択不能
- [ ] 将来 organization-owned repository になった場合、別途 adversarial audit した設計へ切り替えない限り、Hacocoon から visible な organization runner group が `0`

Actions 再有効化後、harmless external fork PR から次を試します。

```yaml
runs-on: self-hosted
```

さらに既知の custom self-hosted label も指定し、どちらも persistent/private runner で job が start しないことを確認します。

`tools/check_workflow_policy.py` は defense in depth です。attacker-controlled PR job は policy job と並列に schedule され得るため、server-side runner isolation の代わりにはなりません。

## 8. Immutable Releases

最初の official public release 前に GitHub Immutable Releases を有効化し、公開済み asset / release tag の supported mutation path からの差し替え・削除を防ぎます。

provenance / release binding は `RELEASE_SECURITY.ja.md` を参照してください。

## 9. live fail-closed checker

ruleset、Environment protection、fork workflow policy、self-hosted runner inventory を読み取れる認証済み GitHub CLI で実行します。

```bash
python3 tools/check_public_release_readiness.py --repo SLktEx/Hacocoon
```

期待結果:

```text
PUBLIC RELEASE READINESS OK
```

API permission が足りず確認できない場合は success ではなく `UNVERIFIED` です。設定が欠ける・弱い場合は failure です。

checker は `protect-main` / `protect-release-tags` / `release` という exact name を要求します。似た名前の無関係な設定が偶然 security gate を満たすことを防ぐためです。

## 10. Actions を再有効化して adversarial validation

live checker が PASS した後だけ次へ進みます。

1. GitHub Actions を再有効化する。
2. required CI job が GitHub-hosted `ubuntu-24.04` runner で動くことを確認する。
3. harmless external fork PR を作り、workflow approval が要求されることを確認する。
4. `self-hosted` / custom runner label を harmless に指定し、persistent/private runner が開始しないことを確認する。
5. `tools/check_public_release_readiness.py` を再実行する。
6. exact public configuration に対して adversarial security audit を再実行する。
7. live evidence を記録してから public-launch blocker issue を close する。

## 公開判断

server-side control の設定中に **Actions disabled の public repository** として存在することは許容します。ただし、この checklist と live readiness checker を通過するまでは **official public release 禁止**です。
