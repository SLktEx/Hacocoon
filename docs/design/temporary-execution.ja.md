# 一時実行

日本語 | [English](temporary-execution.md)

状態: **候補では通常出力・対話実行を実装済み。新しいstreamの実機確認は未完了**。以前の通常実行の実Incus成功は4adfe19の範囲に限定します。

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

失敗時は、生の実行基盤エラーを表示せず、固定の理由分類をstderrへ出します。
`busy`の場合は停止後もWorkspaceの使用権が残ることを説明し、既存Envまたは独立コピーを
案内します。この案内で削除・使用権解除・実行の再試行は行わず、片付け結果も変更しません。
JSONはstdoutに保ち、拒否理由が分かっても未確認の片付けを成功扱いにしません。

公開済み OCI 内容は一時 Workspace にも自動コピーし、--no-oci で無効化できます。
公開内容がなければコピー対象はありません。実行基盤削除後に片付けるのは、
その一時 Workspace に結び付いた既定のコピーだけです。Host Docker/nerdctl の公開や
実イメージ利用の残件は [実装状態](../IMPLEMENTATION_STATUS.ja.md) と区別します。

コマンドの stdout/stderr と終了コードを返します。出力は既存の上限付きで収集し、
切り詰めは明示します。`-i`でパイプ入力とstdout/stderrを逐次転送し、`-it`で端末の入力・編集・サイズ変更を使えます。
--json は execution と cleaned_up を返します。コマンドの非ゼロ終了は、
片付け成功時にも失敗です。片付け失敗を別に報告し、成功扱いにしません。
`--rm=false`は未対応です。対話転送と`--json`は併用できません。JSONの実行結果が必要な場合は通常出力を使います。

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

## 対話操作と片付け対象の固定

```sh
printf 'input\n' | haco run -i -- cat
haco run -it --workspace managed:dev -- bash
```

`-it`には実際の端末が必要です。PTYでは出力を統合し、`-i`ではstdout/stderrを分けます。
入力終了と接続切断を区別し、終了値と片付けの完了が確認できたときだけ成功を返します。
応答が不明な操作を自動で再送しません。明示したWorkspaceとOCIは保持します。

作成前の永続記録にEnvの生成時の識別子を固定し、共通作成処理でその予約を照合します。
削除時もライフサイクルのロック内で照合するため、同じ名前で作り直した別Envを片付けません。
mainで分割済みの責務と共通cleanup結果処理を維持します。所有権が欠けた記録は拒否し、
旧版の移行や代替cleanupは追加しません。[所有権の判断](../adr/0084-ephemeral-run-creation-ownership.md)と
[転送の契約](../adr/0085-bounded-process-streams.md)を参照してください。
