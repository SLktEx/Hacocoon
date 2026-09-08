# Windows 通知からの承認確認

## trusted Host の通知購読

通常の setup は既存のソース・ダイジェスト・Host 所有権の検証で通知バイナリを配布します。
イベントには既存の trusted 管理接続を再利用し、client 内で公開スキーマへ絞ります。
controller の購読失敗をローカル監査の読み取りへ暗黙に切り替えてはいけません。
Windows インストーラーは検証済みの永続的なディストリビューション名を記録し、
Host に渡します。既存の異なる名前は拒否します。利用者の新しいコマンドや監査の
マウントを増やさず、workload の入力で通知先を選ばせないための設計です。


状態: accepted。Windows adapter の実装・installed 検証を進めています。

## 背景

ロードマップ D2 は OS 通知を主入口とし、VS Code を任意にします。
通知は要求を選択できますが、コマンド・回答・認証情報・管理接続先を渡しません。
複数の WSL installation が互いの接続先を上書きしないことも必要です。

## 決定

Windows adapter は distribution ごとにユーザー単位の protocol と通知 identity を登録します。
登録名は正規化した distribution 名から作り、更新前に既存の登録・ファイルの所有対象が一致することを確認します。
installer は checksum を検証した native helper とローカルの distribution 設定を配置します。

helper は小文字 32 桁の要求 ID を含む正規 URI 一つだけを受け付けます。
別 authority、escape、query、fragment、余分な引数、未対応 platform は拒否します。
固定の Windows System32 WSL と Linux haco approve を、独立した引数と最小限の環境で起動します。
shell は起動せず回答も送りません。利用者が既存の信頼された表示を確認して console で明示的に回答します。

approval-required だけが、対応するローカル登録がある場合に起動導線を持ちます。
他の event と不正 ID は表示だけです。リンクは相関情報であり権限ではありません。
別アプリや Web サイトから呼ばれても承認や要求変更はできず、古い要求の拒否・Policy 保存・実行は既存 controller が所有します。

## 採用しない方式

- toast 起動に任意コマンドを含めること、shell interpreter で解釈すること。
- 二つ目の検証 distribution 導入で単一の登録を黙って上書きすること。
- 読み取り専用通知 bridge に公開 HTTP の承認 endpoint を追加すること。
- 起動・通知到着・クライアント不在を承認とみなすこと。
- 無効な native WSL interop を /init で迂回したり、通知 adapter から binfmt 登録を変更すること。欠落した標準登録の復旧は既存 WSL setup が所有します。

## 制限と確認

Windows helper の導入と protocol 登録は Core・guest 権限から分離します。
Linux desktop の起動導線は後続です。OS 表示・protocol 起動・新規要求への回答は、それぞれ実機証拠が必要です。
コマンドの成功だけで通知が画面に表示されたとは扱わず、先行する失敗と未確認を明記します。

## Native client の状態所有権

native client は非公開の状態ディレクトリを固定し、購読・表示の間 OS のプロセスロックを
保持します。排他的なランダムファイルと同期した atomic 置換で保存します。固定の一時名は
リンク経由の書き込みを許し、ロックの削除は複数 inode への所有権分裂を許すため、どちらも
採用しません。これは client が所有する表示状態であり、capability の権限や exactly-once
表示の保証ではありません。Windows setup が登録後の任意のバックグラウンド起動を管理します。
