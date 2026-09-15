# ADR 0106: Git packを既存の権限境界で順次転送する

日本語 | [English](0106-streaming-git-packs.md)

状態: 実装方針を採用。導入済み環境の受入確認は別途必要。

## 背景

従来のJSON/base64転送は各境界でpack全体をメモリへ保持し、一つが32 MiBを超えると
拒否した。既存履歴の再利用とheadごとの順次取得だけでは、大きな追加変更を転送できない。
リポジトリの大きさまでメモリ上限を引き上げる方式は採用しない。

## 決定

Standard Gitは既存のゲスト用Unix HTTP接続口と、所有確認したHost agentの
標準入出力で、上限のあるバイナリフレームを順次転送する。要求のメタデータはpackの前、
応答の完了証明はpackの後に必須とする。1フレーム64 KiB、メタデータ2 MiB、
1 packの転送量16 GiBを上限とし、転送用メモリをpack全体の大きさに比例させない。
転送用の一時ファイル・検索トークン・ゲスト指定のHostパス・管理接続口を追加しない。
Gitのオブジェクトは従来のリポジトリ内へ取り込む。

brokerはEnvごとの同時実行枠、source/Workspace/Env実体の一致、refごとのfetch判断、
agent完了までの登録情報ロックを維持する。HTTPヘッダーは5秒のまま。
承認と転送が旧アップロード上限30秒を超え得るため、本文は既存の通信上限10分へ揃える。
brokerの操作9分、agentの実行5分は維持し、裏での再実行や転送再開を追加しない。

prepareはstrict index-packと同じ有限サイズ上限でオブジェクトを取り込み、
フレーム終端と入力全体の終了まで確認して、commitと祖先関係を検証する。
後のpushには別のref単位の承認・監査が必要で、packを再送しない。
旧OID/未作成のleaseとporcelainの完了確認を維持する。clone・一覧・fetch・履歴hint・
転送したバイトからpush権限は生じない。

フレーム途中終了・上限超過・終端不足・余分なデータ・コマンド/出力失敗・完了証明の
バイト数不一致は失敗する。fetchの途中失敗で通常の未参照Gitオブジェクトが残る場合も、
完了証明と手元のindex-packの両方が成功するまでhelperは成功を返さない。
helperが独自にrefを書き換えることはない。遅延読み込み応答へ登録ロックを逃がさず、
所有するpipeを閉じ子プロセスを待つ。他の利用者のプロセスは停止しない。

## 不採用案と確認範囲

pack全体のRAM上限引き上げ、Physical Hostへの全量重複保存、Hostファイル/ソケットの
公開、分割取得トークンの状態管理、fetch/pushの権限緩和は採用しない。
pre-1.0のclient/controller/agentを一緒に置換し、旧base64形式への互換fallbackは残さない。

32 MiBを超える実Git・loopback/pipe回帰は機能上の制限修正の証拠であり、
巨大レポの速度・容量や導入済みIncusの受入確認ではない。大規模計測は現在の利用優先方針に
従って後続とする。LFS/submodule/force/delete/複数ref pushの対象外条件も維持する。
[ADR 0102](0102-incremental-git-history.ja.md)の全量転送上限のみを置き換え、
履歴・権限の規則と[ADR 0081](0081-git-read-and-push-authority.md)を維持する。
