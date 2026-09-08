# ADR 0029: デスクトップのローカル承認

日本語 | [English](0029-local-desktop-approval-review.md)

状態: accepted。repository 実装済み、installed 受け入れは未確認。

Remote-SSH window の開発 terminal は通常、信頼しない Environment で動きます。
承認にはローカル利用者の既存 controller 権限を使う必要があります。
任意の UI 拡張は VS Code Pseudoterminal と直接起動するローカル子プロセスを使用し、
shell terminal・remote command・workspace task を使いません。
起動前に UI host、desktop、workspace trust を確認します。

installed CLI/WSL の絶対パスと最小環境は adapter が固定します。
Windows の既定 distribution は Hacocoon で、別の installed distribution は local user 設定だけで選べます。
workspace 設定、event endpoint、通知 payload は実行ファイル・認証・controller socket を指定できません。
ID は既存の信頼された要求を選ぶだけです。表示・保存範囲の検証・Policy 永続化・実行は通常の CLI/controller に残します。

クリックは回答せず review を開きます。制御文字・escape・過大な入力を拒否し、出力を制限して制御文字を escape します。
閉じる操作や子プロセスとの接続喪失は rollback の証明ではありません。
controller が回答を受理済みの可能性があるため UI は再試行しません。既存 queue と実行の期限も維持します。

却下した方式: remote window の通常 createTerminal/shellPath、workspace 内の管理コマンド実行、
workspace で指定できる実行ファイル、controller override の継承、read-only event bridge への承認 POST、
通知クリックによる自動回答。任意連携は Core や通常の haco に必須依存を追加しません。

[機能契約](../design/pending-approval-review.ja.md) と
[VS Code Pseudoterminal API](https://code.visualstudio.com/api/references/vscode-api#Pseudoterminal) を参照してください。
