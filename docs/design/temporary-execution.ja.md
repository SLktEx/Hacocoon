# 一時実行

日本語 | [English](temporary-execution.md)

状態: **product CLI は implemented、実 Incus の検証は 4adfe19 で成功**。

Environment の命名や事前作成なしに、コマンドを1回実行します。

```sh
haco run --rm -- uname -a
haco run -- sh -c 'echo hello > message.txt; cat message.txt'
```

削除は既定のため --rm は省略できます。コマンドは /workspace から始まります。
--workspace を省略すると、このディレクトリは一時 Environment に属し、
Environment とともに内容が消えます。既定の Hacocoon Base を使い、--base で変更できます。
Docker の image 指定ではありません。

ファイルを残す場合は、既存 Workspace を指定します。

```sh
haco run --workspace managed:dev -- sh -c 'echo result > result.txt'
haco run --workspace managed:dev --read-only -- ls
```

指定した Workspace とその OCI Store は保持します。別の retained Environment が
lease を持つ場合は、その Environment を削除してから貸し出します。stop は lease を保持します。
独立した Workspace copy を選ぶ方法もあります。

公開済み OCI 内容は一時 Workspace にも自動コピーし、--no-oci で無効化できます。
公開内容がなければコピー対象はありません。runtime 削除後に片付けるのは、
その一時 Workspace に結び付いた既定のコピーだけです。Host Docker/nerdctl の公開や
実 image 利用の残件は [実装状態](../IMPLEMENTATION_STATUS.ja.md) と区別します。

コマンドの stdout/stderr と終了コードを返します。出力は既存の上限付きで収集し、
対話的なストリーミングではありません。切り詰めは明示します。
--json は execution と cleaned_up を返します。コマンドの非ゼロ終了は、
片付け成功時にも失敗です。片付け失敗を別に報告し、成功扱いにしません。
--rm=false と対話 stdin/TTY は未対応です。

Ctrl+C は controller に中断を要求し、130 を返します。client は切断済みなので、
削除を確認できたとは報告しません。haco env list と haco env status <name> で残件を確認します。
controller は片付け失敗時に recovery marker を残し、起動時または次の run で再試行します。
エラーを隠すために state file を削除しないでください。不確かな OCI copy の予約解除には、
既存の provider 別調査が必要になる場合があります。

canonical lifecycle が runtime の不在まで所有権と lease を保ち、
一時リソースの片付けは Workspace 所有権を原子的に照合します。
Environment 名が再利用されても、別の作業を削除しません。
[ADR 0020](../adr/0020-runtime-owned-temporary-workspaces.md) と
[接続切断時の中断](../adr/0018-ephemeral-run-cancellation.md) を参照してください。

repository test は argv の保持、既定の一時領域、既存 Workspace の保護、
片付け失敗と再試行、OCI 公開元の保持を検証します。既存 Incus GHA の 4adfe19（run 34115004878、job 101719650209）で、通常の製品コマンドによる
成功、exit 17、既存ファイルへの書き込み、中断後の実体不在が確認できました。
対話利用、ローカル installed acceptance、内容入り OCI image の実行を確認したとは扱いません。

## cleanup 結果の責任

実装済み: 通常終了、activation 失敗、中断された一時実行の復旧は、時間制限付きの
canonical runtime／scratch cleanup と、共通のマーカー更新処理を使います。
作成失敗も同じマーカー更新を使いますが、canonical 作成が runtime 所有状態を確認不能と
報告した間は scratch データを削除しません。cleanup またはマーカー永続化の失敗は元の原因を
保持して一貫して recovery-required を返し、既存の起動時／次回実行時の復旧で再試行します。
JSON 形式と guest 終了コードは変わりません。cleaned_up は runtime／scratch cleanup の
完了を表し、マーカー削除の失敗は引き続きエラーです。リポジトリ回帰テストでマーカー削除の
再試行、activation 失敗、キャンセル、保持 Workspace／Store の境界、scratch の途中失敗を
検証します。
