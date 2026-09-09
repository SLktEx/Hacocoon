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

この段階は runtime が利用できる Env に接続済みの Store が対象です。Host の配布元、
未接続 Store、候補を選ぶ GC、管理用 Env の自動起動は planned です。Seed・tombstone・
隠れた backup・新 catalog 状態・schema 移行は追加しません。保存 snapshot は独立コピー
なので削除の影響を受けません。unit／CLI 回帰と既存の実 runtime COW fixture は別の範囲を
検証します。実際の実行結果は実装状況と PR に記録します。

## 過去の Seed 削除

v0.16 の Host Seed cache・tombstone・全 Env 操作は隔離された旧実装のものです。
現在の製品コマンドや Store 寿命モデルではありません。過去の仕様は Git 履歴で確認でき、
今回の変更で既存記録を黙って削除しません。release 表の v0.16 リンクは過去の checkpoint
を示し、今回の partial 実装の番号ではありません。[ADR 0046](../adr/0046-reviewed-runtime-image-deletion.md)を参照してください。
