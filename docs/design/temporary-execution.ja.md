# 一時実行

日本語 | [English](temporary-execution.md)

状態: **product CLI は実装済み、実 Incus の検証は 4adfe19 で成功**。

Environment の命名や事前作成なしに、コマンドを1回実行します。

```sh
haco run --rm -- uname -a
haco run -- sh -c 'echo hello > message.txt; cat message.txt'
```

削除は既定のため --rm は省略できます。コマンドは /workspace から始まります。
--workspace を省略すると、このディレクトリは一時 Environment に属し、
Environment とともに内容が消えます。既定の Hacocoon Base を使い、--base で変更できます。
Docker のイメージ指定ではありません。

ファイルを残す場合は、既存 Workspace を指定します。

```sh
haco run --workspace managed:dev -- sh -c 'echo result > result.txt'
haco run --workspace managed:dev --read-only -- ls
```

指定した Workspace とその OCI Store は保持します。別の保持済み Environment が
lease を持つ場合は、その Environment を削除してから貸し出します。停止は lease を保持します。
独立した Workspace コピーを選ぶ方法もあります。

公開済み OCI 内容は一時 Workspace にも自動コピーし、--no-oci で無効化できます。
公開内容がなければコピー対象はありません。実行基盤削除後に片付けるのは、
その一時 Workspace に結び付いた既定のコピーだけです。Host Docker/nerdctl の公開や
実イメージ利用の残件は [実装状態](../IMPLEMENTATION_STATUS.ja.md) と区別します。

コマンドの stdout/stderr と終了コードを返します。出力は既存の上限付きで収集し、
対話的なストリーミングではありません。切り詰めは明示します。
--json は execution と cleaned_up を返します。コマンドの非ゼロ終了は、
片付け成功時にも失敗です。片付け失敗を別に報告し、成功扱いにしません。
--rm=false と対話 stdin/TTY は未対応です。

Ctrl+C はコントローラーに中断を要求し、130 を返します。クライアントは切断済みなので、
削除を確認できたとは報告しません。haco env list と haco env status <name> で残件を確認します。
コントローラーは片付け失敗時に復旧識別情報を残し、起動時または次の run で再試行します。
エラーを隠すために状態ファイルを削除しないでください。不確かな OCI コピーの予約解除には、
既存のプロバイダー別調査が必要になる場合があります。

正規のライフサイクルが実行基盤の不在まで所有権と lease を保ち、
一時リソースの片付けは Workspace 所有権を原子的に照合します。
Environment 名が再利用されても、別の作業を削除しません。
[ADR 0020](../adr/0020-runtime-owned-temporary-workspaces.md) と
[接続切断時の中断](../adr/0018-ephemeral-run-cancellation.md) を参照してください。

リポジトリ内のテストは argv の保持、既定の一時領域、既存 Workspace の保護、
片付け失敗と再試行、OCI 公開元の保持を検証します。既存 Incus GHA の 4adfe19（run 34115004878、job 101719650209）で、通常の製品コマンドによる
成功、exit 17、既存ファイルへの書き込み、中断後の実体不在が確認できました。
対話利用、ローカル導入済み検証、内容入り OCI イメージの実行を確認したとは扱いません。

## 出力と中断後の所有権

共有のHost プロセス runnerは既定でstdout・stderrをそれぞれ4 MiBまで保持します。
超過分は読み捨て、子プロセスを停止したり終了値を変えたりしません。
切り詰めた出力には表示識別情報を付け、JSONの`stdout_truncated` /
`stderr_truncated`をtrueにします。`stdout_bytes` / `stderr_bytes`は
切り詰め前に観測したバイト数です。不完全な出力を完全な結果として解釈しません。
制御用subprocessにも同じ制限があり、過大な構造化出力は解析失敗として扱います。

`--json`の結果は`environment`、`execution`（終了値、両出力、上記の切詰め情報）、
`cleaned_up`を返します。実行成功と後始末成功は別です。

実行前に保護した状態へ`ephemeral_runs`記録を保存し、実行中はrun単位の
Linux `flock`を保持します。プロセス終了後にロックが取得できる場合だけ、
後続の照合・調整が期限付きで正規Env削除を試みます。`run-`の名前、
識別情報だけ、PID推測は削除権限ではありません。稼働所有者のロックは飛ばし、
失敗時は`cleanup-required`を保持します。非対応platformでは弱い証明へ切り替えません。
SIGINT/SIGTERM時の後始末は実行キャンセルとは独立した期限で動きます。
