# OCI image の一覧と削除

日本語 | [English](oci-image-deletion.md)

Status: partial。接続済み Store の実装と専用環境での実 runtime 検証は完了し、インストール済み controller 経由の受け入れ検証は未完了です。

## 現在のコマンド

```bash
haco plugin oci image list dev
haco plugin oci image list --runtime docker --json dev
haco plugin oci image delete dev example.local/app:dev
```

既定は nerdctl で、Docker は runtime を明示します。一覧には Env 世代、Store の
所有者、image ID・tag・digest、container の参照、独立 snapshot の由来を表示します。
削除は表示された正確な ID または tag を受け付け、runtime の不変 ID に解決して
対象を表示し、確認を求めます。`--yes` は確認入力だけを省略します。tag だけの
解除ではなく、選択 image 全体の削除です。複数 tag や container 等による runtime
の拒否はエラーとして返し、force で回避しません。

## 責務と保護

OCI plugin は runtime の一覧・削除を使い、別の image catalog を持ちません。
Docker の config ID と nerdctl の manifest/index digest は区別します。
nerdctl の Docker 互換 inspect は config ID と container の image 名を返すため、
削除対象の native ID へ明示的に対応付けます。CLI の環境をクリアし、固定の
ローカル socket・containerd namespace・snapshotter を指定します。呼び出し元は
socket や実行ファイルを選べず、layer file を直接削除しません。不正・不完全な
出力や失敗を成功扱いせず、削除後の一覧で不在を確認できなければ失敗を返します。

`internal/workspace.ExecForResource` は通常の Env lifecycle lock 内で、確認済み
世代と接続 Store の所有 ID を比較して通常の実行経路を呼びます。各 runtime 呼び出しで
再確認し、plugin も現在の ready Store の所有者を照合します。同名 Env の再作成で
待機中の削除が新 Env へ向かうことを防ぎます。guest の通常操作で一覧は変わり得るため、
最後の参照確認は runtime の非 force 削除が担当します。

## 範囲と検証

接続済み Store と Host source に加え、未接続 Store の経路は後述の partial 実装です。候補選択 GC は planned です。保存 snapshot は独立コピーであり、画像削除の影響を受けません。schema 移行や自動 backup は追加しません。

## 管理対象 Host source

partial の実装です。専用環境の実 runtime 検証は成功し、インストール済み controller 経由の検証は未完了です。

```bash
haco plugin oci image list --host
haco plugin oci image delete --host --runtime docker example.local/app:dev
```

`--host` と Env 名は同時指定できません。確認画面は正確な source owner と、今後の
Store コピーの元を変更することを表示します。既存の独立 Store／snapshot コピーには
影響しません。旧 Seed namespace・tombstone・全 Env 削除は復活させません。
対象は現在の管理 source `oci-source:host` であり、guest Store を Host に接続するための
指定には使えません。自動 setup・移行・復旧も実行しません。

plugin は現在の ready source owner を照合します。Incus adapter も独立して、固定の
image／container 一覧、限定した inspect template、不変 ID の非 force 削除だけを許可します。
実行ファイルの検索先・daemon socket は固定し、CLI の環境をクリアします。任意 shell・
program・daemon option・inspect template は Host 境界へ渡せません。

各命令は既存 Host-operation lock を保持し、未完了 copy journal を拒否します。
native volume owner、単独の接続先、正確な mount、local Host/source marker、非特権の
container 型、空の profiles、running 状態、管理対象 daemon layout を確認します。
停止中 Host を再開したり、復旧記録を消したりしません。所有・layout が不明、または
観測が失敗／切り詰められていれば拒否します。native 命令は2分、全体 request は5分で
制限します。container 参照と削除後の不在確認は接続済み Store と同じ runtime の検証を使い、
既存 copy／cleanup 状態は変更しません。

## 過去の Seed 削除

v0.16 の Host Seed cache・tombstone・全 Env 操作は隔離された旧実装のものです。
現在の製品コマンドや Store 寿命モデルではありません。過去の仕様は Git 履歴で確認でき、
今回の変更で既存記録を黙って削除しません。release 表の v0.16 リンクは過去の checkpoint
を示し、今回の partial 実装の番号ではありません。[ADR 0046](../adr/0046-reviewed-runtime-image-deletion.md)を参照してください。

## 未接続 Store の実装中の範囲

状態: **partial**。既存の image コマンドは Env 名の代わりに保持 Store ID を受け付けます。
新しいコマンドや必須 option は追加しません。

```bash
haco plugin oci image list oci:store-id
haco plugin oci image delete oci:store-id sha256:<displayed-digest>
```

確認対象は正確な Store ID・owner であり、古い一時 Env の識別情報は保持しません。
一覧・削除はそれぞれ一つの canonical maintenance run を取得し、全 runtime 呼び出しで
現在の Env 世代を確認します。run がキャンセル・Store 排他・cleanup を所有し、元の
Workspace 対応や借用 Store を変更・削除しません。cleanup が不明なら所有記録を保持します。
Host・Env・Store の混在した対象や古い確認結果は拒否します。未接続 Store の Docker は未対応です。

SandboxProvider の receipt 付き作成は、保持データなしで起動し、現在の network 保護を維持し、
通常 daemon を mask してから Store を接続し、専用の containerd 2.3.3 metadata service を起動します。
task・restart・CRI・NRI・sandbox service は無効にします。保持設定・restart label・権限は引き継ぎません。
receipt なしの作成と snapshot restore は maintenance を拒否します。
[ADR 0047](../adr/0047-detached-store-maintenance.md) を参照してください。

一時 Base への対応 OCI 実行ファイルの自動配置と、導入済み controller 経由の受け入れは未完了です。
素の Ubuntu Base に必要な containerd・ctr・nerdctl は揃わないため、公開経路の接続だけでは
通常導入で動作しません。ツール不足・未対応は操作失敗として返します。
schema 移行・自動 backup・任意の実行ファイルや socket を選ぶ option はありません。

## 未接続 containerd metadata service

独立した Incus 6.0.5/Btrfs primitive は179.66秒で成功し、task API 拒否、restart 設定付き
container metadata の不変性、使用中画像保持、未使用 alias 削除、mask 付き再起動、
所有対象だけの cleanup を確認しました。同じ socket 上の製品画像操作は別の受け入れ検証です。
fixture は表示タグ・不変 digest・実際の container 参照を区別します。同じタグであることは、
各 digest が参照されている証拠にはなりません。fixture の catalog・lifecycle adapter は、
controller 全体の作成とツール配置を表していません。

拡張した native fixture は224.64秒で成功しました。製品の一覧、実際の container 参照による削除拒否、
選択した未使用 digest の削除と不在、container metadata 保持、mask 付き再起動、Env 削除後の
Store 保持、所有対象だけの cleanup を確認しました。lifecycle・catalog adapter は fixture のままです。
