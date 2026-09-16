# ADR 0109: 通常の開発環境をクライアントで組み立てる

日本語 | [English](0109-default-development-session.md)

状態: accepted。#715に対応し、ブランチ非依存の登録（#709）を前提とします。

## 決定

通常の入口をリポジトリ登録と引数なしのopenにします。クライアントが取得元を選び、
既存Workspace workflow・正規Environment lifecycle・デスクトップSSHを使います。
準備前にロック付きの参照を永続保存し、応答後に所有者を固定します。
[ディレクトリによる入口](0062-workspace-entry-and-data-fork.md)を拡張し、
controllerに別のlifecycleやカタログを追加せず、Coreにprovider固有の既定処理や
新しい権限を持たせません。

環境名・ディレクトリ指定と上級操作は維持します。既存環境の選択は`open --select`で
利用できます。既定Baseの取得・SSH準備はPolicy・Approvalを含む通常のprovider操作を
使います。CLIは処理段階を案内しますが、権限拡大や失敗状態の自動削除・置換はしません。

## 代替案と影響

適当な既存環境の選択は無関係な作業を開く危険があります。所有者を固定しない名前だけの
選択は同名再作成を取り違えます。新しい管理カタログは既存の準備参照・lifecycleと重複し、
leaseとmetadataの個別操作は[所有権の契約](0002-environment-lifecycle-ownership.md)に反します。

登録変更時に既存構成を再作成すると、編集内容・rootfs変更・OCI状態を失う可能性があります。
データ保持を保証する拡張契約を別途設計するまでは構成の不変性を維持し、明示的なforkを
案内します。通常の参照は管理ユーザーのhome単位で、異なるhome間では同期しません。
一連の操作の実機確認とリポジトリE2Eの証拠は分けます。

詳細は[通常の開発環境の設計](../design/default-development-session.ja.md)に記載します。
上級操作の機能は削除しません。
