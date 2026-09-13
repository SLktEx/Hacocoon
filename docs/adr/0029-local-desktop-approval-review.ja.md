# ADR 0029: デスクトップのローカル承認

日本語 | [English](0029-local-desktop-approval-review.md)

状態: 表示方式はhistorical。[ADR 0067](0067-local-gui-approval-session.md)が置き換えます。
ローカル権限の境界は維持し、VS Codeはprivate GUI sessionを使用します。

Remote-SSH window の開発 terminal は通常、信頼しない Environment で動きます。
承認にはローカル利用者の既存コントローラー権限を使う必要があります。
任意の UI 拡張は VS Code Pseudoterminal と直接起動するローカル子プロセスを使用し、
シェル terminal・remote コマンド・workspace task を使いません。
起動前に UI Host、デスクトップ、workspace trust を確認します。

導入済み CLI/WSL の絶対パスと最小環境はアダプターが固定します。
Windows の既定 distribution は Hacocoon で、別の導入済み distribution は local 利用者設定だけで選べます。
workspace 設定、event 接続先、通知 payload は実行ファイル・認証・コントローラーソケットを指定できません。
ID は既存の信頼された要求を選ぶだけです。表示・保存範囲の検証・Policy 永続化・実行は通常の CLI/controller に残します。

クリックは回答せず確認を開きます。制御文字・escape・過大な入力を拒否し、出力を制限して制御文字を escape します。
閉じる操作や子プロセスとの接続喪失は rollback の証明ではありません。
コントローラーが回答を受理済みの可能性があるため UI は再試行しません。既存 queue と実行の期限も維持します。

却下した方式: remote window の通常 createTerminal/shellPath、workspace 内の管理コマンド実行、
workspace で指定できる実行ファイル、コントローラー override の継承、読み取り専用 event bridge への承認 POST、
通知クリックによる自動回答。任意連携は Core や通常の haco に必須依存を追加しません。

[機能契約](../design/pending-approval-review.ja.md) と
[VS Code Pseudoterminal API](https://code.visualstudio.com/api/references/vscode-api#Pseudoterminal) を参照してください。
