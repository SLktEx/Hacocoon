# ADR 0102: 読み取り権限を広げず既存Git履歴を再利用する

[English](0102-incremental-git-history.md) | 日本語

状態: 実装方針として採用。導入済み環境の受入は別途扱います。

## 背景

fetchや小さなpushのたびに全履歴を送ると、取得済みレポジトリでも新しいデータとは
無関係に転送上限へ達します。処理はStandard統合が所有し、Coreの権限契約を維持します。

## 決定

helperは最大32個の重複しないローカルブランチ先端SHA-1をfetchの補助情報にします。
brokerとtrusted agentはfetch以外でこれを受け付けません。既存のref単位のPolicy判断と
最新リモート先端の照合後、その先端の祖先と確認したものだけをpackから除外します。
不存在・無関係な候補は無視し、Host内の非公開objectや別refを取得する根拠にはしません。
revision入力には検証済みOIDだけを使います。補助情報はpush権限を与えません。

既存targetへのpushは、一覧にある更新前commitが手元にある場合、その履歴を省きます。
既存の準備処理は厳密なobject取り込み前に当該targetをfetchして再照合します。
承認は同じrepository・ref・old/new OIDに固定し、実行時のleaseと別判断を維持します。
この当初の決定では新規targetへのpushに完全packを使います。後続の
[ADR 0104](0104-new-branch-git-history.ja.md)が、独立したref単位の読み取りによる
公開済み祖先の再利用を追加します。

通常の[Git revision packing](https://git-scm.com/docs/git-pack-objects)と
[厳密なobject取り込み](https://git-scm.com/docs/git-index-pack)を使います。
thin packや書き込み可能なGitディレクトリの共有は導入しません。出力bufferは上限付き
Writeのみを公開し、プロセスのpipe転送が継承された無制限ReadFromを使うことを防ぎます。
32 MiB pack・message・ref数・操作時間の既存上限を維持します。

## 不採用案と限界

任意のレポジトリ全体に合わせたメモリ上限引き上げ、guestのpath/config利用、任意の
have OID取得、clone/fetchを承認と扱う方式は採用しません。補助情報は限定した最適化で、
完全なGit交渉ではありません。無関係なbranch、共通先端がない場合、新しいpack自体が
大きい場合は上限へ達します。partial/shallow clone、LFS、submodule、大容量転送全体を
この変更で実装したとは扱いません。33 MiBの回帰は代表的な性能受入ではありません。
[読み取りとpushの権限分離](0081-git-read-and-push-authority.ja.md)も参照してください。
