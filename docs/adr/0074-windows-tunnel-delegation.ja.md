# Windows転送の自動委譲

状態: 採用、実装候補。[English](0074-windows-tunnel-delegation.md)

## 決定

WSL／trusted Hostの通常転送コマンドはWindows側で待ち受け、通常のLinuxはローカル待受を維持します。
既存controllerの導入先取得、通常のWindows連携、導入済みクライアントを使います。
その経路に失敗したら、別の名前空間で代わりに待ち受けません。

WSL登録GUID、導入世代、準備したEnvの正確な作成世代、元の期限を引き継ぎます。
Windows側はprivate controllerで両方の世代を照合してから待ち受け、以後の接続も共通の
世代・lease・名前空間照合を通します。名前やWSL環境情報は権限ではありません。
通常のWSL利用者として起動し、rootへ切り替えません。

所有する入力pipeで上限付き要求を一度渡し、開いたpipeを寿命の目印にします。
親の消失・キャンセルでpipeが閉じ、子は待受と転送先を回収します。
親はその子だけの終了を待ち、応答がない場合は期限付きで終了します。
切り離した起動、常駐サービス、追加の管理待受やguest endpointは作りません。

Linux側のinterop子は独立したprocess groupに置きます。端末の中断は前面のCLIだけが受け、
pipeを閉じて子の終了を待ちます。同じgroupではpipe経由のキャンセル完了前に中継が終了し得ます。
groupを分けても親の所有・終了待ちは維持し、別sessionにはしません。
キャンセル時の非zero終了を成功へ置換せず、実際の失敗と既存の期限付き強制終了を保持します。

導入済み実行ファイルは、その利用者領域を所有する既存Windows利用者として動きます。
Linux側のpath／記録確認は静的な所有不一致を拒否しますが、Windowsのfile pinやACL保証ではありません。
通常Envへdrive・interop・管理接続を渡さず、Windows容量回収workerの権限も取り込みません。

## 不採用案

表示名だけの再選択は別登録先や別Envへ接続し得ます。両側で時間を数え直すと利用者の期限を延ばします。
PowerShellのバックグラウンド起動は子の寿命を隠し、待受を残し得ます。Host環境変数や再利用可能な認証情報の複製は不要です。
手動の`/init`、独自binfmt、試験専用の連携修復を通常導入の代替にはしません。
[転送の契約](../design/controller-client-transport.ja.md#windows転送の自動起動)を参照してください。
