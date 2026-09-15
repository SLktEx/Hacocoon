# 責務別配置と旧CLIの廃止

状態: 採用。Refs #654。基準は main `a3d0f4fd7bbf7134c113029f80124e8f2c332bd6`。

## 決定

ディレクトリ名は、コードを所有する機能または外部システムを表す。
Core・Standard・Plugin は設計上の役割であり、並立するディレクトリの分類ではない。
通常の Go パッケージと明示的な組み立てを使い、移動前の内部パッケージを残す
互換 alias は設けない。[構成案内](../../CONTRIBUTING.md#repository-map)を経路の正本、
[拡張アーキテクチャ](../design/plugin-architecture.md)を責務の正本とする。

配布する各バイナリは `cmd/` に独立した薄い入口を持つ。
現行製品は `cmd/haco` からビルドし、引数解析と表示は `internal/cli` が担当する。
旧 `hacoq` はソース・配布アーカイブ・インストーラーから削除する。
旧GitHub capability、Docker status/prepare サービス、組み立て用の
`HACO_PLUGIN_OCI` 設定も廃止する。現行の管理Git、OCI Store・image操作、
controller API、通知、client adapter はそれぞれ保持する。
汎用 capability・event API には現在のclient・通知からの利用があるため保持する。

インストーラーのソースは `scripts/` から `install/` へ移る。
ソースを直接取得するURLは変更し、転送用スクリプトを残さない。
配布アーカイブ名とバンドル内のファイル名は同じ。
旧バイナリを含むアーカイブはインストーラーが拒否する。
この変更は既存導入のファイルや保存データを自動削除しない。

## 維持する境界

Baseのビルド、保存asset、確認付きimage削除は別パッケージとする。
Envの経路選択・コピー・移送は、snapshot復元および正規のWorkspace lifecycleと区別する。
保存snapshotは稼働中のEnvに従属しない。SSH設定と公開鍵検証は利用者が異なり、
移動によってworkloadへ管理権限を与えない。

Gitプロセスと通信形式はadapter、登録所有権と承認の手順はGit機能が担当する。
OCIの取得・展開はadapter、保持Storeの所有権はstorage serviceが担当する。
Windows/WSLのidentity・登録・workerは、回収の値と呼出側clientから分離する。
作成receipt、正確な所有者を確認する後始末、承認、隔離は新しい配置でも必要である。

保存済みBase・snapshotの所有記録には、確認付きの後始末が必要である。
その読取りや検証を消すと、資源の所有権が失われるか、名前だけによる採用を招く。
これらは互換aliasとして削除せず、データ寿命の契約を明示的に置き換えるまで維持する。

## 却下した代案

- CLIを二つ残す、または製品から旧CLIを呼ぶ: 組み立てと旧権限経路が二重になる。
- バイナリ数を減らすためにhelperを統合する: 実行場所・OS・権限が異なる。
- Standardの全実装をadapterへ移す: 承認・キャッシュ選択・Workspaceの手順は製品動作である。
- Env・snapshot・Workspaceの変更処理を統合する: データ寿命が異なり、所有権の書込みは正規のlifecycleへ限定する必要がある。
- リポジトリテストから実機受入を主張する: 配布Ubuntu/Windows、Incus、認証付きサービスの受入は別に確認する。
