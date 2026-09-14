# 通常EnvからSeedの暗黙選択を撤去

状態: accepted、実装候補。[English](0078-seed-runtime-retirement.md)

## 判断

新しいEnvは明示的に選択したBaseと固定revisionを使います。OCIプラグインの設定だけで
現在Seedのカタログを読み、別イメージへ差し替えたり、そのイメージを理由に
入れ子コンテナの権限を有効にしたりしません。将来の呼出しで再び有効にできる分岐を
残さず、resolverのオプションとSeed専用の構成引数を削除します。

compositionからSeed store/serviceの生成と、通常コマンドをSeed harvest adapterで
包む処理を外します。旧Seedのbuild・current・pin・推奨・GC・recoveryを含むコマンド群を
削除します。通常のBase構築、管理対象の永続OCIデータ、既存の範囲限定された接続検査は維持します。

機能の撤去をデータの移行・削除とは扱いません。保存済みEnvのBase参照、イメージ、
カタログを変更・削除せず、導入済みEnvを作り直しません。以前のSeedデータを片付ける権限を
推測しません。既存の再開とデータ寿命管理を引き続き正規の経路とします。
旧builder・harvest・telemetryの内部実装は、共有helperと過去の復旧用データを
確認してから別途撤去する残件です。

## 採用しない方法

無効化したresolverだけを残すと、危険な別のBase選択経路も残ります。
OCIプラグインの切替をイメージ差替えの許可とみなすと、ツール連携とEnvの識別・権限が混ざります。
古いSeedイメージやカタログの自動削除は、機能撤去とデータ削除を混同し、
以前の作業を復旧する唯一のコピーを失うおそれがあります。`switch-base` は復活させず、
現在のデータ保持を伴う作り直し手順を使います。

[Seed撤去](../design/oci-seed-and-cow.ja.md)と
[データの寿命](../guides/data-lifetime.ja.md)を参照してください。
