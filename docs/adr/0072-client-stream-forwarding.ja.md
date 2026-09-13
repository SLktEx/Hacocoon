# Controller sessionによるclient TCP転送

状態: 採用・実装候補。[English](0072-client-stream-forwarding.md)

## 背景

Physical Host上のIncus proxyは、clientのネットワーク名前空間に待受を
作りません。clientがアプリへ接続するために、Incus管理権限やEnv bridgeへの
直接到達性を必要としない経路が必要です。

## 決定

`haco env tunnel`が数値ループバックTCP待受を所有し、接続ごとにprivate
controller byte sessionを開きます。準備時はreadyなcatalogと正確な作成世代を
選択するだけでprovider resourceを作りません。各接続でも世代・ready状態・
実行状態を照合し、Incus adapterが既存の世代検証と固定したnetwork namespace
内でsocketを開きます。宛先は選択Env内の明示的な数値ループバックのみです。
同名Envの再作成へ自動的に接続先を変えません。

Host→Envの権限は既存の管理socketのアクセス制御に従います。新methodはこの
endpointだけに登録し、guest Git・DNS・network endpointへ追加しません。
guest発の通信は既存のCapability／Policy／承認／監査を維持します。Env名や
返却された作成世代は認証情報ではありません。

既存sessionにbyte streamの明示的な完了待ちを追加します。相手の半切断を
最終完了待ちより先に伝えるため、片側の送信終了後も残りを受信できます。
接続成功・失敗の応答をアプリのbyteより先に返し、EOFだけで成功としません。
両側の半切断より前のcloseは、既存private管理endpointの`_control.session.cancel`
で明示的に終了を求め、stream worker終了まで応答を待ちます。EOFを無視する
接続先も終了させ、アプリbyteへcancel制御を混ぜません。controllerへ到達できない
場合は遠隔の回収を証明できず、controller側の絶対期限が残る上限です。
socket生成はtransport設定後の所有されたstream callback内で行い、最初の
応答送信失敗でsocketが漏れる順序にしません。

待受は同時16接続、1秒〜1時間。controllerの準備・接続は10秒、streamは
最大1時間です。キャンセルは待受・socketを閉じ、copy workerの終了を待ちます。
proxy device、永続的な転送記録、自動再起動・再接続は作りません。
保持データとlifecycle所有権は変えません。

## 不採用案と残件

- Incus proxy device追加ではclient側の待受を実現できない。
- 新しいTCP管理serverやguest管理socketは権限境界を広げる。
- processと同じEOF時の暗黙の完了待ちは双方向の半切断を止めるため、
  byte sessionでは明示的に完了を待つ。
- namespace経由の任意の外部宛先は別の外向きproxyになる。外向き通信は
  既存の独立した承認を伴うnetwork接続が担当する。

Windows側待受から`wsl.exe`を通すtransport、汎用process streamの残る統合、
VPN／DNS modeはM3の別残件です。WSL／trusted Host内で実行するLinux clientは
その場所で待ち受けるため、Windows native待受の証拠とはしません。
[契約](../design/controller-client-transport.ja.md#client側tcp待受)を参照してください。
