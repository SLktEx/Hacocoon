# Experimental VS Code連携

[English](experimental-vscode.md) | 日本語

VS Code設定とExtension導入は任意のクライアントアダプターが担当します。
Core・プロバイダー・EnvironmentライフサイクルAPIにエディタ固有の権限を追加しません。
YAMLの`experimental.vscode`を正本とし、Experimental期間の互換性は強く保証しません。
schema・既定値・コマンド・前提・制約の正本は[参照文書](../reference/experimental-vscode.ja.md)です。

信頼されたHostユーザーのYAMLを、共通の厳密なparse・schema検証・revision照合・
ロック・atomic保存で更新します。読取ではファイルを作りません。対象サブツリーだけを
置換して他のYAML値を保持します。書き込みを調整した直接編集も可能です。
コントローラー内の別DBや、ゲストがHost設定を変更する経路は作りません。

`haco open`と単独のVS Codeアダプターが同じ適用処理を使います。既存の固定された
SSH aliasとライフサイクル・Workspaceの紐付けを再利用します。Hostから固定HTTPS宛先の
Marketplaceメタデータを取得し、サイズとExtensionの同一性を検証します。
共通のUTC時刻を基準に経過時間を評価し、セマンティックバージョンを比較します。
明示した版は日数・pre-release条件の例外です。依存ExtensionとPackも同じ条件で、
上限付きの依存グラフとして起動前に解決します。日数の評価前にplatform成果物を選び、
古いuniversal成果物を根拠に新しいplatform版を導入することを防ぎます。

serverの配置はRemote-SSHが担当します。デスクトップと同じ安定版ビルドを待ち、
そのNodeでEnv内のExtension CLIを実行します。追加のPythonパッケージや
バックエンド専用の準備は不要です。settingsはSSHのstdinからJSONで渡し、shellの
ソースやargvへ埋め込みません。ゲストのadvisory lockでHacocoonの適用を直列化します。
ディレクトリ記述子、リンクを辿らないopen、単一リンクの通常ファイル確認、atomic置換に
よりRemote settingsをrepoパスから分離します。所有コメントには派生情報である
管理キーだけを記録し、別の設定正本にはしません。

ゲストの結果は信頼しません。適用成功の応答はHost権限を付与せず、エディタUIの
接続完了も証明しません。子プロセスの出力を制限し、生の出力を診断に転記しません。
途中で導入に失敗してもEnv・設定・導入済みExtensionを保持します。次回openで版を
再確認し、非互換を理由に未選択の最新版へ変更しません。UIでの変更や自動更新の
継続的な強制管理は対象外です。

設定はEnv/Workspace共通のRemote settingsです。repoのFolder Settings、
`.vscode/settings.json`、`.code-workspace`は生成しません。credentialはSSH Agent・
Credential Storeや既存の信頼された所有者に分離します。将来のUIが扱うのは状態と
参照であり、credential本体ではありません。
不採用の選択肢は[ADR 0066](../adr/0066-experimental-vscode-ownership.md)を参照してください。
