# 手元のGit作業ディレクトリを取り込む

日本語 | [English](workspace-input.md)

状態: Linux/WSL client向けの実装済み候補です。実機確認は実装状況と分けて記録します。

## 操作

接続したい取得元を`haco repo clone`で登録し、元ディレクトリを読めるclientから実行します。
Git接続先には現在のHost登録情報を使い、入力からHost上のパスや認証情報を取り込みません。

```bash
mkdir task
haco workspace import --repo sample --name task --path ./task ../linked-worktree
haco open ./task
```

コピー先には既存のWorkspace参照を保存します。元checkout・linked worktreeの内容は独立した管理volumeへコピーし、元の場所にも残します。元ディレクトリをEnvへ直接mountしません。
コピー中はエディタ・ビルド・Gitの書き込みを止めてください。取り込み後の変更は同期しません。
Base・OCIは通常の選択（`--base`、`--oci`、OCI既定auto）を使い、ローカル入力からOCI Storeは取り込みません。後の停止forkで保持作業を複製できます。

未追跡・ignoredファイルを含む作業ファイル、選んだHEAD・index、独立したobjects・refsを保持します。
HostのGit設定・認証情報・hooks・reflog・他のworktree管理情報は含めません。
新しい設定は`haco://<登録名>`だけを接続先とし、取得・承認付きpush・main確認は通常の規則に従います。
登録名の選択は接続したい取得元の明示であり、入力内容がその取得元から来たことを保証しません。

## 境界と失敗

clientは元のGit・設定・hooksを実行せず、ファイルとして読みます。linked worktreeのディレクトリと戻り参照を照合し、Git情報のsymlinkや別objectsパスの追跡を拒否します。
provider共通のtreeをバイト列で送り、Env転送と同じ分割送信・キャンセル・checksum照合を使います。
管理endpointだけが操作を受け付け、controller側の読取パスやnative設定は指定できません。

Standardは現在の登録先と既存importの作成・所有記録・公開・cleanupを使います。
Incus内で全treeを非公開のまま変換・検査してから、新しい所有情報でimportします。
UID/GIDはEnvのrootへ揃え、通常の権限ビットと範囲内の相対symlinkを保持します。
元の所有権・xattr・device・特別権限は再現しません。
失敗・結果不明では参照と正確な回復記録を残し、再度開いただけでimportの再送や既存作業の上書きをしません。

初期対応はSHA-1の通常checkout・linked worktree、packed refs、split indexです。
sparse/partial clone、別objectsへの参照、入れ子のrepo/submodule、特殊ファイルは拒否します。
上限は64GiB・100万entryで、provider側にはより小さい上限がある場合があります。
読み取り中のファイル・ディレクトリ変更を検査しますが、動いているファイルシステム全体の原子的snapshotではありません。
巨大レポの速度・容量実測は後回しです。[設計判断](../adr/0098-independent-worktree-input.ja.md)を参照してください。
