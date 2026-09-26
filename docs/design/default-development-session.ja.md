# 通常の開発環境

[English](default-development-session.md) | 日本語

通常は`haco repo add <URL>`に続けて`haco open`を実行します。
最後に開いたEnvironmentを再開し、停止中なら起動します。Environmentがなければ
既定Imageから作成します。Repositoryが0件でも利用できます。
新しい作業は`haco open --new [IMAGE]`で始めます。

作成元の優先順位、変更できない構成、既定Imageの保存、Volumeの寿命、失敗時のcleanupは
[Environment作成](environment-creation.ja.md)が定義します。最後に開いたEnvironmentは
controller catalogに保存します。旧`~/.haco-default`の準備記録は読み書きしません。
既存Environmentのデータを削除したり、自動移行したりすることはありません。

既定はRemote-SSHを導入したVS Code、`--client ssh`ならシェルです。
`--client none --json`はデスクトップ接続なしで開き、Environment情報を返します。
進捗はstderr、結果はstdoutに出力します。SSH接続は選択済みEnvironmentの所有者を
確認し、同名資源が差し替わっていたら接続権限を与える前に拒否します。
承認待ちは`haco approve`、Policyの拒否は`haco config --edit`で明示的に確認します。
open自体は権限を与えません。

クライアントを閉じてもEnvironmentは停止しません。エディタ起動失敗でも作成済みの
作業環境を保持し、同じopenを再試行できます。壊れた既存環境は自動修復・再作成しません。
ディレクトリ参照は独立管理Workspaceの明示操作として残り、通常openの選択には使いません。
[日常操作](../reference/daily-workflow.ja.md)、[旧設計判断](../adr/0111-default-development-session.ja.md)、
[現在の設計判断](../adr/0112-unified-environment-creation.ja.md)を参照してください。

`haco open --select`は既存Environmentを明示的に選び、同じopenサービスで開いて
last-openedを更新します。通常のopenでは選択画面を要求しません。
