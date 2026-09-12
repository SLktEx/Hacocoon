# データの寿命と整理

[English](data-lifetime.md) | 日本語

作成・停止・削除は信頼された管理ターミナルから実行し、通常の開発はEnvironment内で行います。
Physical HostはIncusと保護されたHacocoonの状態を管理し、
信頼された `haco-host` は管理ツールや認証付きGit操作を扱います。
どちらのHostも、信頼しないプログラムの開発場所ではありません。

## 対象と寿命

```text
Physical Host: コントローラー、Policy、Incus、ストレージ管理権限
  ├─ 信頼された haco-host: 認証情報と取得元リポジトリ
  ├─ Baseイメージ ──作成──> Environmentのルートファイルシステム
  ├─ 管理対象Workspace ──排他的な利用権──> /workspace
  └─ 任意のOCI Store ──排他的な利用権──> /var/lib/hacocoon-oci
```

| 操作 | Environmentのルート領域 | WorkspaceとGit | OCI Store | 利用権・接続 |
|---|---|---|---|---|
| SSH・エディターを閉じる | 残る。実行も続く場合がある | 残る | 残る | 保持 |
| `haco env stop dev` | 残る。プロセスは停止 | 残る | 残る | 利用権の予約は保持 |
| `haco env start dev` / `haco open dev` | 同じ実体を再開 | 同じデータ | 同じデータ | 所有権・ネットワークを再確認 |
| `haco env delete dev` | 削除 | 残る | 残る | 実体の不在を確認してから解放 |
| `haco workspace delete work` | Envが参照中なら拒否 | 管理対象ファイルとGit情報をすべて削除 | 残る | 有効・処理途中の利用権があれば拒否 |
| `haco plugin oci store delete store` | Storeが予約中なら拒否 | 残る | 選択したStore全体を削除 | 停止中のEnvも削除を妨げる |

Baseは作成時の固定された開始イメージで、Environmentの変更を保存するバックアップではありません。
Baseの別名が変わっても影響するのは次の作成だけです。
OCI Storeにはイメージ、ビルドキャッシュ、永続的な管理情報が残り、
バイナリやプロセス・ソケットはEnvironmentに属します。
再接続には互換性のあるツールが必要で、タスクは自動再開しません。

単一リポジトリでは `/workspace`、集合では `/workspace/<member>` が永続領域です。
集合の各マウントの外に書いたファイルはEnvironmentだけに属します。
外部Workspaceは指定した**コントローラー側**のパスに残ります。
ゲストの `/tmp` は起動時に消去される場合があります。
これらの仕組みは、エージェントによるWorkspaceの書き換えを防ぐものではありません。

## プロジェクトを残して再作成する

必要なEnvironment内だけのファイルをWorkspaceに移すか、先にスナップショットを作ります。
パッケージやルート領域の変更を意図的に捨てる場合は、次のようにします。

```bash
haco env stop dev
haco env delete dev
haco env create --workspace managed:work --base haco/ubuntu-26.04 dev
haco git connect dev
haco open dev
```

元のWorkspace IDを指定します。関連付けられた既定のStoreは再利用されます。
以前に独立したStoreを指定した場合は、同じ `--resource oci:<store>` を指定します。
OCIを接続しない場合は `--no-oci` を再度指定します。
作り直したEnvには新しい作成・SSH識別子が割り当てられ、以前のEnvironment専用の承認は引き継ぎません。
`switch-base` は無効です。[Workspaceの所有権](../design/workspace-abstraction-and-lease.md)を参照してください。

## 開発状態全体を保存・コピーする

```bash
haco snapshot create dev
haco snapshot list
haco snapshot restore <snapshot-id> restored-dev
haco env stop dev
haco env copy dev independent-dev
```

スナップショットはルート領域、全Workspaceメンバー、任意のOCI、メタデータの独立したコピーです。
実行中の保存元は停止し、保存が完了してから再開します。Envのコピーは停止済みの保存元が必要です。
復元・コピーでは新しいデータと権限の識別子を作り、既存のEnvironmentを上書きしません。
復元先の既定名は `<source>-restored`、コピー先の既定名は[コピー仕様](../design/environment-copy.md)にあります。
整合性が必要なアプリは書き込みを止めてから保存してください。
ファイルの保持を確認しても、任意のデータベースの整合性を保証したことにはなりません。

保存元を削除してもスナップショットは残ります。
`haco snapshot delete <id>` は選択した保存だけを削除し、独立した現在の作業は保持します。
失敗時は確認用の識別子を残します。[スナップショット仕様](../design/environment-snapshots.md)を参照してください。

## 残ったデータを明示的に削除する

先に対象を確認します。

```bash
haco workspace list
haco plugin oci store list
haco repo list
haco base list --all
```

各削除は管理対象を表示し、確認を求めます。`--yes` は意図した自動操作向けです。
取得元は、Git接続先として参照するWorkspaceの記録がなくなってから削除します。
依存関係を消すためだけに必要な作業を削除しないでください。
`haco repo delete <id>` は上流リポジトリ、認証情報、独立した保存データを削除しません。

Incusの子スナップショット・バックアップ・保存スケジュールがあると親ボリュームの削除を拒否します。
所有権が不明、作成途中、コピー未完了の場合も危険な削除を拒否します。
`deleting` の記録が残った場合は同じ明示的なコマンドで再試行します。
作成途中の記録を一般的に修復するコマンドはありません。
[Baseの削除](../design/base-images-and-custom-environments.md#explicit-built-image-cleanup)と
[OCIイメージ単体の削除](../design/oci-image-deletion.ja.md)にも参照の確認があります。

データを消すこととWindowsのディスク割り当てを回収することは別です。
`haco reclaim`、状態確認・再確認、中断時の扱いと実機確認範囲は
[ストレージ容量回収](../design/storage-reclamation.ja.md)にあります。
[Environmentの持ち出し](../design/environment-transfer.ja.md)は完成したbundleを移す機能です。
[データ退避](data-evacuation.ja.md)は部分実装の保守作業で、インストール全体のバックアップではありません。
