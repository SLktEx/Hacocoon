# ADR 0103: 接続中の仮想ディスク観測ハンドルを閉じてから待つ

[English](0103-virtual-disk-observation-lifetime.md) | 日本語

状態: 実装方針として採用。導入済み環境の容量回収確認は別途扱います。

## 背景

Windows workerはWSLが接続を解除する前でもディスクを開ける場合があります。
その仮想ハンドルを保持して切断を待つと、待っている状態変化を自分で妨げます。
Microsoftの[DetachVirtualDisk仕様](https://learn.microsoft.com/en-us/windows/win32/api/virtdisk/nf-virtdisk-detachvirtualdisk)は、
切断には他の仮想ディスクハンドルを閉じる必要があり、恒久接続でない場合も最後の
ハンドルを閉じるまで接続が残ると規定しています。

## 決定

登録・保存した操作・排他・実ファイルと親ディレクトリの固定は全処理を通じて保持します。
これらが対象ファイルとvolume相対pathを固定します。一方、接続中を確認した仮想ディスクの
観測ハンドルは待機前に閉じ、既存90秒の範囲で同じ固定pathを開き直します。再試行は
open時の共有違反、または接続中と確認して正常に閉じた場合だけです。不明な観測やcloseの
失敗は直ちに停止します。中断時も失敗した観測ハンドルを閉じてから返ります。

切断を確認したハンドルは呼び出し元へ渡し、一度だけ圧縮します。変更開始後にopenや
観測を再試行しません。実ファイル・親の固定解除、強制detach、別WSLの停止、idle設定変更、
タイムアウトを完了と扱う方式は採用しません。同じtargetの再開と失敗記録の所有を維持します。

拒否の根拠は引き続き、使用中のWSL filesystemをofflineディスクとして圧縮しないことです。
観測には文書化された
[GET_VIRTUAL_DISK_INFO.IsLoaded](https://learn.microsoft.com/en-us/windows/win32/api/virtdisk/ns-virtdisk-get_virtual_disk_info)を使います。
component回帰はclose後のreopen・中断・close失敗を確認します。独立して作成した空VHDでも、
Windowsアカウントに接続権限がある場合にnativeハンドルの寿命を確認します。このfixtureは
導入済みWSLの受入とは別であり、以前の `compact_attached` の失敗を消しません。

## 更新した仮定

同じ仮想ハンドルを保持して状態を読み続ける仮定を更新します。
[ADR 0048](0048-storage-reclamation-identity.md)の所有固定と変更再送禁止は維持します。
観測自身が切断を妨げる場合、期限延長だけでは解決しません。別clientによるWSLの再起動は
独立した同時利用の条件として扱います。
