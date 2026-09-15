# 固定の同一PC WSLプロセス転送

状態: accepted、部分的な実装候補。[English](0092-wsl-process-transport.md)

## 決定

Windows clientは検証した同一PCのdistributionを選び、非表示のSystem32 wsl.exeから
導入済み製品の固定操作を呼びます。controller転送の引数・環境変数を共通の起動処理で構成します。通知との共通化は別途進めます。任意コマンド・WSL利用者変更・継承認証環境・任意socketは受け取りません。
Linux側は通常のUDSアクセス制御を維持します。同一PCであること自体は認可ではなく、
通常Envへ新しい接続先も渡しません。

匿名pipeでTCP半切断を保つため、アプリケーションのEOFを明示します。上限付き
frameでbyteとEOFを子の異常終了から区別し、pipeの切断をtransport失敗と扱います。
別接続の制御は既存sessionの完了・cancelとcontroller側の対象世代確認を維持します。
キャンセル・closeで起動した子を回収し、pipeを明示的に所有して正常終了後の
未読stdoutを保持します。期限切れは安全側に接続を終了します。

## 不採用と残件

同一PCのWindows/WSLにTCP管理daemonは追加しません。stdinのEOFだけでは半切断と
プロセス消失を区別できません。shell文字列・PATH依存のWSL選択・rootへのfallbackは
起動境界を広げます。インストーラhelperと通知helperの責務は維持し、公開Windows
待受はclient側の統合で提供します。

公開companionの配布と通常WSL入口からの自動委譲は実装候補です。後者は[ADR 0093](0093-windows-tunnel-delegation.ja.md)を参照してください。
導入済みWindows経路の受入は残件です。実fixtureの転送成功を
導入済みcontroller・Incusへのアクセス確認とは扱いません。
[契約](../design/controller-client-transport.ja.md#windowsのプロセス転送)を参照してください。
