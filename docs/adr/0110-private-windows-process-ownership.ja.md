# ADR 0110: Windows専用接続の子孫プロセス所有

日本語 | [English](0110-private-windows-process-ownership.md)

状態: 実装候補として採用。

## 証拠と決定

main `63bc41d1` に対する実Windowsの回帰テストで、通知接続をキャンセルして
`Close` した後も、起動を遅延していた孫プロセスが動く不具合を再現した。
`exec.CommandContext` が終了するのは直下の起動プロセスだけであり、その子は
起動排他の解放後も実行できた。これは[ADR 0108](0108-background-wsl-start-coordination.ja.md)
の終了確認の契約を満たさない。ただし、過去の接続中ディスクの失敗全件と同じ原因だと
確定したわけではない。

Windows実装で、名前のない専用Job Objectを `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`
付きで作成する。`CreateProcessW` の `PROC_THREAD_ATTRIBUTE_JOB_LIST` により、
実行開始前に通知プロセスを所属させる。所属前に子を起動して逃れる競合を作らず、
離脱を許可する設定も使わない。Jobのハンドルは継承させない。別の継承リストにより、
匿名の標準入力・出力と破棄先の標準エラーだけを渡す。確認済み実行ファイル、引数、
作業ディレクトリ、最小限の環境変数を明示する契約は維持する。

キャンセルでは所有するJobだけを終了し、稼働中の所属プロセスがゼロであることを
確認してからハンドルを閉じる。PIDや名前の列挙ではなく、WindowsのJob集計情報で
直下のプロセスが先に終了した場合も確認する。確認の上限は2秒とする。
終了または確認に失敗した場合は失敗を返してJobハンドルを保持する。
接続準備の失敗時には、所有するアプリが終了するまで起動排他も保持する。
アプリ終了で最後のJobハンドルが閉じられると、残る所属プロセスの終了が要求される。
プロセス名での一括終了、WSL全体の停止、追加権限は導入しない。

この共通処理はCore外に置き、現在は通知の専用接続だけに使う。
意図的に独立稼働させる容量回収ワーカーは所属させない。
人による回答、Policy保存、ディスクの同一性・切断確認は変更しない。

## Windowsの契約と拒否理由

[UpdateProcThreadAttribute](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-updateprocthreadattribute)
がWindows 10以降の起動時Job一覧と継承ハンドルの指定を定義する。
[Job Objects](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects)
が子孫の所属と最後のハンドルを閉じた際の動作を定義する。
[TerminateJobObject](https://learn.microsoft.com/en-us/windows/win32/api/jobapi2/nf-jobapi2-terminatejobobject)
と[基本集計情報](https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-jobobject_basic_accounting_information)
を終了要求と稼働数の観測契約とする。所有を確立できない場合は起動を拒否する。
囲い込みを省いた起動は再現済みの競合を復活させるためであり、推測によるOS版の
事前拒否は追加しない。

## 不採用案と確認

起動後の所属設定には子の生成との競合が残る。直下だけの終了では子孫が残る。
名前での列挙やPIDの再取得には、別プロセスやPID再利用の危険がある。
成功するまで再実行する、圧縮の期限を伸ばす、他のdistributionを停止する方法は、
所有の不具合を修正する代わりにならない。

実Windowsの回帰テストで、準備失敗時の遅延した孫、先に終了した起動プロセス、
無関係な接続への非干渉、通常交換、キャンセル、排他解放を確認する。
手元のHacocoonでも、導入済み通知接続の読み取り確認が成功した。
[実機確認記録](../status/acceptance-evidence.ja.md#private-windows-launch-descendants)
で、人の通知操作、配布物での確認、転送・ディスクの未解決な間欠的失敗と区別する。
