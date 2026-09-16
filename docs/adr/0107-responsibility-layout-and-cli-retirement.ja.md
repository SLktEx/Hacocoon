# 責務別配置と旧CLIの廃止

状態: 採用。Refs #654。基準は main `ee8bf7fbd2e858f9b6fd1d22ecce77fdde6bf082`。

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

Agent Host補助CLIも一つの明示的なコマンド分岐に統合する。`init`による横取り、
prepareの重複した引数解析、標準出力の取り直し、旧出力形式の変換処理は削除する。
prepareとlookupは同じ型付きセッション記述を直接出力し、補助バイナリと信頼済み
セッションbrokerはCoreから分離したまま保持する。SSH grantの交換では、Envの作成世代、
Workspace、アクセスモード、serviceの一致を要求し、grant IDだけは更新できる。
target全体の一致を要求すると交換を常に拒否し、人向けの名前だけを比較すると別の所有対象を
許してしまう。旧grantの失効は、新しい接続設定の準備が完了した後に行う。

Env作成は一つのRouterが担当し、providerが返すBase識別情報、実効資源上限、
所有権の作成記録を保持する。BaseRouterの重複した作成処理を撤去し、Base一覧と
snapshot・archive操作も同じRouterを使う。参照元のない無効化用・資源上限未対応用
providerラッパーと、provider側の旧`PrepareSSH`別名も削除する。
現行clientは`PrepareSSHAccess`と対応する失効契約を使う。公開client-adapter APIと、
保持中の所有対象を安全に削除するための読み取り処理は維持する。

旧Sessionのmanager、専用JSONストア、未使用のruntime/storage契約、Incusの
Session作成・実行入口は、それらだけを対象とする旧テストとともに削除する。
現行Envの状態と所有権は別のcatalogと正規のlifecycle遷移で管理し、このSession
ストアを参照しない。登録されていない`switch-base`の実装も削除する。
CLIの既存の拒否は保持し、代わりの操作や保存データの自動移行は追加しない。

未使用のRuntime可用性probeとSession状態への観測変換も削除する。
Env観測の正確な識別情報の検査は保持する。対話実行は元のprocessエラーを直接返す。
追加の結果変換は全呼び出し元が捨てていたもので、終了状態は元のエラーから取得できる。

IncusのEnv作成もSandboxProviderに一本化する。Baseを固定し、所有権を記録してから
専用ネットワークを設定してguestを起動する。Runtime/BaseProviderの作成処理、Runtimeの
Prepare、旧共有bridge・ACL・profileの準備処理は削除する。E2Eのstorage選択は既存の
遅延storage providerを使う。現行の専用bridge、source guard、保存データの読み取りと
正確な所有者に基づく清掃は保持する。予約名の拒否、Base選択、読み取り専用mount、
期限付きの失敗清掃のテストは現行実装を対象にする。Base catalogに別のEnv作成経路を設けない。

保存する経路の形式は`haco-runtime-v1:<provider>:<base64url-native-ref>`とする。
解析時に`:`を所有providerの区切りとして使うため、この文字を含むprovider IDは
登録時に拒否する。受け入れると、作成後に解決できない所有記録が生じる。
native参照の内容は解釈しない。Base公開では、経路を外す前にEnvとleaseの保存参照が
一致することを要求する。Envだけを基に両方を書き換えると、providerのlease検証前に
不一致が消えてしまう。正常な経路、未対応・曖昧な登録、lease側のproviderまたは
native所有先が異なる公開を回帰テストで確認する。

外向き通信の接続元も、この正確な経路比較を使う。Incusだけに存在した旧形式の
論理名aliasは削除し、`haco-<名前>` の組立てによる別の所有先推測を認めない。
保存資源の読み取り処理や、現行Env作成が返す実体参照は変更しない。

インストーラーのソースは `scripts/` から `install/` へ移る。
ソースを直接取得するURLは変更し、転送用スクリプトを残さない。
配布アーカイブ名とバンドル内のファイル名は同じ。
旧バイナリを含むアーカイブはインストーラーが拒否する。
この変更は既存導入のファイルや保存データを自動削除しない。

導入済みLinuxとWindows/WSLの利用手順を検証するドライバーは、
`test/e2e/installed`と`test/e2e/windows`へ置く。ビルド・配布・診断・検証用観測の
ツールは`tools/`に保持する。Windows再起動の転送専用スクリプト二つを削除し、
初回導入・再起動・再導入を一つのドライバーが担当する。キャッシュ利用は明示的な
引数で製品の`-UseCachedWslImage`を選び、端末の書込み処理の差替えやテスト専用の
製品セットアップを追加しない。

## 維持する境界

controllerが管理するストリームは、接続時に合意した完了確認用の識別子を必須とする。
session IDを返さない旧peerを受け入れる互換処理は削除する。EOFだけではプロセスの
終了状態を証明できず、識別子がなければ完了確認・キャンセルの制御もできない。
pre-1.0のclientとcontrollerは一緒に更新する。現行APIが独自に結果やイベントの
形式を定める生のストリームは保持し、管理セッションの代替としては使わない。

Envのexportは必ず保護されたWorkspaceの対応表を取得し、現行のversion 2で出力する。
未使用のversion 1生成経路と公開writerラッパーを削除する。対応表の欠落は保存情報を
失わせるため、読取り処理が未設定なら取得前に拒否する。保存済みアーカイブに必要な
version 1のreaderとオフラインimportは、[ADR 0052](0052-transfer-routing-metadata.md)の
契約に従って保持する。

Baseのビルド、保存asset、確認付きimage削除は別パッケージとする。
Envの経路選択・コピー・移送は、snapshot復元および正規のWorkspace lifecycleと区別する。
保存snapshotは稼働中のEnvに従属しない。SSH設定と公開鍵検証は利用者が異なり、
移動によってworkloadへ管理権限を与えない。

Gitプロセスと通信形式はadapter、登録所有権と承認の手順はGit機能が担当する。
[Git packのストリーム転送](0106-streaming-git-packs.ja.md)のフレーム交換と最終receiptは
adapter内で完了する。brokerは交換の呼び出し中も、接続元の実行枠と認可を保持する。
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
