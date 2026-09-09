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

この段階は runtime が利用できる Env に接続済みの Store が対象です。Host の配布元は次節で扱います。
未接続 Store、候補を選ぶ GC、管理用 Env の自動起動は planned です。Seed・tombstone・
隠れた backup・新 catalog 状態・schema 移行は追加しません。保存 snapshot は独立コピー
なので削除の影響を受けません。unit／CLI 回帰と既存の実 runtime COW fixture は別の範囲を
検証します。実際の実行結果は実装状況と PR に記録します。

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

公開 CLI にはまだ接続していません。一時 run の予約は、正確な Store owner と
永続化した run の識別情報に一致させます。通常の Workspace 対応、Store の排他 lease、
source-only の拒否は維持します。catalog 再読込時も正確な scratch identity を要求し、実行中・cleanup 中の lease を維持します。
native 不在確認後に lease を解放するまで、根拠となる run 記録の削除・差し替えを拒否します。
独立した Incus 準備処理は専用の systemd／Btrfs fixture で成功しましたが、
未接続 Store の実 Docker／nerdctl image 操作は未検証です。
[ADR 0047](../adr/0047-detached-store-maintenance.md) を参照してください。

SandboxProvider の receipt 付き作成は、現在の network 検証後に準備→Store 接続→metadata
起動を行います。receipt なし作成と snapshot restore は maintenance を拒否します。
既存 run service が確認済み owner を固定し、操作全体と canonical cleanup の間、所有記録と
lock を保持します。公開の image 経路は未接続です。関連 run／adapter race test は成功し、
統合した作成経路全体の実機受入は未完了です。

## 未接続 containerd metadata service

内部の起動処理は、native Store owner、単独の接続先、現在の Env 世代、非特権の mount を
照合して containerd 2.3.3 の metadata service を起動します。guest 内の専用 socket／設定／
実行状態は、保存済み設定や通常 daemon の起動から分離します。task・restart・CRI・NRI・
sandbox controller を無効にし、保存済み container 記録と restart label は維持します。
専用 Incus 6.0.5/Btrfs テストは179.66秒で成功し、task API の拒否、restart 設定を持つ
container 情報の不変性、使用中 image 保持、未使用 alias 削除、Store 保持と所有対象の
cleanup を確認しました。既存 Incus/Btrfs GHA にも接続しています。処理単体の受入であり、
公開 maintenance 作成や Docker 対応は有効にならず、
利用者指定の socket も受け付けません。[ADR 0047](../adr/0047-detached-store-maintenance.md#containerd-metadata-only-startup)を参照してください。
