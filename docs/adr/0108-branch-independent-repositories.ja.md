# ADR 0108: チェックアウトするブランチに依存せずリポジトリを登録する

[English](0108-branch-independent-repositories.md) | 日本語

状態: 採用。関連: #709。

## 決定

`haco repo add <id> <remote>`と`repository.add`でGitの取得元を登録します。
ブランチはWorkspaceの初期チェックアウトや個々のGit操作で選び、ソースの識別や
brokerのソース照合には使いません。[ADR 0008](0008-managed-repository-workspaces.md)の
登録情報からブランチを決める部分を置き換えます。

既存の所有管理されたソースvolumeと、独立したWorkspace volume copyを維持します。
ソースは全branchを保持する通常のcloneですが、checkoutは行いません。新しいコピーで
取得元のheadsを更新し、指定ブランチまたは現在のremote symbolic HEADをcheckoutした後、
権限を持たないhelper接続設定に置き換えます。インポート・復元したゲストデータには
この認証付き準備を実行しません。ソース所有者、登録remote、Environmentの識別、
refごとのPolicy確認は維持します。既定HEADがなくても他のheadsは有効であり、
明示したブランチからWorkspaceを作成できます。

共通の永続化形式では、`branch`はWorkspaceの任意の初期選択情報だけを表します。
branchを含む旧ソース記録は非互換として拒否し、自動書き換えや所有volumeの削除は
行いません。保存済みWorkspaceのbranchは引き続き有効で、登録情報との一致は不要です。
snapshot・transferではbranchがない登録remoteも扱います。remoteがない作業は
オフラインのままで、同名登録によって権限を得ることはありません。

## 代替案と影響

必須branchや既定branchの保存を残すrenameでは、誤った識別モデルが残ります。
branchごとの重複登録は所有とGit権限を重複させます。worktreeやalternatesの共有は
信頼されたGit管理情報をゲストの書き込みにさらします。bare・mirror形式への移行は
volume copyの不要な変更を招きます。通常のcloneですでに全headsを保持できます。

pre-1.0の[互換性方針](../../CONTRIBUTING.md)に従い、旧CLIとAPIを削除します。
`--branch`を無視したり、登録情報として解釈したりするaliasは残しません。
単一取得元の初期選択は任意の`workspace create --branch`で行い、集合と
`workspace prepare`は各remoteの現在の既定を使います。再開・fork・importは
既存の作業を保持し、checkoutをやり直しません。

詳細な仕様と観測失敗時の扱いは[Git設計](../design/git-and-github-capability.md#branch-independent-registration)が管理します。
CLI・API・保存形式のbranch非依存性、複数branchの独立コピー、登録後の変更、
HEAD不在、brokerの所有確認をテストします。ローカルGitとリポジトリのテストは、
実Incusやインストール済みWindowsでの受け入れ確認を意味しません。
