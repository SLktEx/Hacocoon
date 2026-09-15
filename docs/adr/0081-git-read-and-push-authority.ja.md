# ADR 0081: Gitのブランチ取得とpush権限を分ける

[English](0081-git-read-and-push-authority.md) | 日本語

状態: 実装方針として採用。実機の受入は別に確認する。
日付: 2026-09-15

## 背景

通常のGitで別ブランチを取得し、新しい作業ブランチをpushできるようにします。
登録時のcheckoutブランチは由来であり、外部への書き込み許可ではありません。
mainへの統合ではStandard Gitの既存成果 #585 / #587 を再利用します。

## 決定

全headの一覧取得には`refs/heads/*`へのfetch判断を必要とします。
信頼されたagentが件数を制限したhead名とcommit OIDを返し、仲介側は一覧を公開する前と
オブジェクト取得前に各refを個別確認します。個別refの拒否を全体への許可で回避できません。
取得commitは一覧で示したheadと一致する必要があります。
Workspaceは独立したGitデータと全ブランチのfetch設定を持ちます。

pushの準備には正確な対象へのfetch権限を必要とし、Gitオブジェクトだけを取り込んで
新commitを固定します。実行は既存のPolicy/Approvalを通し、リポジトリ・上流・
Environment作成時の識別子・対象ref・変更前後のOIDに結び付いたpush権限を別に確認します。
cloneやfetchからpush許可を保存しません。

新規対象の旧OIDは全ゼロとし、実行時まで未作成であることをGitの明示的な空leaseで確認します。
既存対象は観測した正確な旧OIDとfast-forwardを必要とします。承認中に動きうるbranch名ではなく、
確認したcommit OIDを送信します。保存する判断は正確なrefと`create`/`fast-forward`を区別し、
他方の操作・別ブランチ・mainへ広げません。mainへの承認要求は引き続き優先します。

Gitのporcelain結果では正確なref・commit・期待する変更を一件だけ確認します。
他者が同じ内容のブランチを先に作成した場合、終了0でもup-to-dateとなり得るため、
今回の新規作成の成功とは扱いません。結果不明時は観測を必要とし、pushを自動再実行しません。
根拠はGit公式の[leaseと出力契約](https://git-scm.com/docs/git-push)、
[remote-helper protocol](https://git-scm.com/docs/gitremote-helpers)です。

CoreへGit専用の認可規則を追加しません。既存の共通処理でHost側ソースの所有権、
Workspaceの利用権、Environmentの作成世代を再確認します。Envから渡されたパス、
設定・remote・hook・認証情報で信頼されたソースを置き換えられません。
再利用可能なHost認証情報は通常Envへ渡しません。

## 採用しない方式と限界

cloneからのpush許可、保存済み対象の暗黙拡大、暗黙または全体へのlease、一覧外OIDの取得、
終了コードだけでの成功判断、EnvのGit設定の転送は採用しません。

初期transportのpack合計32 MiB制限を、現在は各headのpackを32 MiBまでとして順番に取得・
取り込む方式に修正しました。一回1024headと個別の認可、メッセージ・pack単位の上限は維持し、
batchの合計は32 MiBを超えられます。headごとの転送で履歴が重複する場合があり、
巨大レポ向けtransportと実測は別作業として残します。force push・削除・複数ref・LFS・submoduleは
延期します。実Gitを使った部品試験を、認証GitHub・導入済みEnv・GUI・巨大レポの受入とは扱いません。
旧版移行は最新のユーザー指定により今回の対象外です。
