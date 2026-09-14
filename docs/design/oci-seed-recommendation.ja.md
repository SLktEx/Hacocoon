# OCI Seed Recommendation

状態: **historical**。旧Seedのsample・recommendコマンドは撤去済みです。
以下は残存するtelemetry内部実装の過去の設計であり、現在のCLI手順ではありません。
共有helperと履歴データを確認した上で内部実装を撤去する作業は残っています。
[Seed撤去](oci-seed-and-cow.ja.md)を参照してください。

## CLI

```text
hacoq plugin oci seed sample
hacoq plugin oci seed recommend
```

pre-1.0で廃止した `haco image ...` はaliasとして残しません。Hacocoon Environmentのstarting pointは `haco base ...`、OCI/container イメージライフサイクルは `hacoq plugin oci ...` です。

## Telemetry model

HostはEnvironmentごとのlatest OCI イメージスナップショットを保持します。記録するのはSeed 選択に必要な識別のみです。

- Environment identifier
- sample 時刻
- repository/reference
- tag（存在する場合）
- 不変のダイジェスト（観測可能な場合）

コマンド history、container 出力、registry 認証情報、credential-helper 応答、Workspace 内容は記録しません。

現在の既定:

- 6時間以内のスナップショットはopportunistic samplingでは新規扱い
- 推奨 windowは直近30日
- Environmentごとにlatest スナップショットだけをcountするため、同じEnvironmentを何度sampleしてもweightは増えない

## Immutable identity

Recommendationには不変のダイジェストが必要です。変更可能な tagだけではSeed 識別として扱いません。

```text
reference@sha256:...
```

Tagが別ダイジェストへ移動した場合は別推奨識別です。

## Automatic promotion

eligible 推奨を決定的なにrankし、上位10%を `auto_promote=true` にします。

```text
1-10 candidates   -> 1 auto
11-20             -> 2 auto
21-30             -> 3 auto
```

countは `ceil(eligible_count * 0.10)`、candidateが1件以上なら最低1件です。

順位は、最近の Environment 数の多い順、最後に観測した時刻の新しい順、イメージ参照名、ダイジェストの昇順で決めます。

human-readable 出力では`auto`、JSONでは`auto_promote`を公開します。

## Safety boundary

Automatic 候補選択が選ぶのはOCI イメージ内容であり、認証情報や任意のEnvironment データではありません。

- registry/Host 認証情報をSeedへ候補選択しない
- Workspace ファイルを利用情報へ入れない
- sample失敗をempty Environmentとして黙殺しない
- 不変のダイジェストがないイメージはauto 選択しない
- 既存の Environment / 公開済み不変の Seedを直接mutationしない
- v0.16 deletion 削除指定は同一識別のrecommendation/auto 候補選択より優先

## v0.17との関係

v0.15は**将来のSeedに何を入れるか**を決めます。信頼された Host 側 acquisition/cache、offline Seed Builder、不変の公開、current-Seed pointer、Btrfs/COW 連携はv0.17の責務です。Local Registryはprerequisiteではありません。
