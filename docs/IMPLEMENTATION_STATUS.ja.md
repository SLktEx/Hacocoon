# 実装状況

[English](IMPLEMENTATION_STATUS.md) | 日本語

現在のmilestone位置は **v0.67**。番号の正本と履歴は[バージョンとリリース状況](status/versioning-and-release-status.ja.md)を参照してください。

このページはmainのコードで使える範囲を示します。初めて使う場合は[利用開始ガイド](guides/getting-started.ja.md)へ進んでください。実機で確認できた範囲・失敗・スキップは[検証証拠](status/acceptance-evidence.ja.md)、残りの開発方針は[ロードマップ](status/architecture-and-roadmap.md)が管理します。

**状態:** 実装済み、部分実装、未実装の計画、延期を区別します。実装済みでも全Host・プロバイダーでの動作確認を意味しません。

実装済み: [Incus 7.0 LTS導入](design/installer.md#incus-package-baseline)をUbuntu、
Windows/WSLと両方の実機CI準備経路で共有し、パッチ更新と実server版の検証を行います。
doctorは非対応版を報告し、6.0互換はベストエフォートで保持します。vendor daemonの
認識と匿名volume exportでも所有確認を維持します。[検証証拠](status/acceptance-evidence.ja.md#incus-lts)で
統合候補の成功と今回のmain向け切り出しを区別します。

| 機能 | 状態 | 使える範囲・制約・残課題 |
|---|---|---|
| [Experimental VS Code](reference/experimental-vscode.ja.md) | 実装済み | 共通YAMLサブツリーをエディタ・ファイル・JSONから編集。EnvのRemote settingsと、依存先を含む日数・pre-release・固定版によるExtension選択。安定版・既定serverパス・Linux x64/arm64が対象。実Marketplace・エディタ・Windows/WSLでの受け入れ確認は未実施。 |
| [Host の標準ツール](design/trusted-host.ja.md#host-の標準ツール) | 実装済み | 通常のローカル setup がユーザースクリプトの前に Git/gh と固定版 containerd/nerdctl/BuildKit を導入。管理対象 OCI データと Host 内のソケットを利用し、再 setup はデータを保持。公開版 Windows インストーラー、arm64 実機、独自の既存導入環境の確認は別途必要。 |
| [日常操作・setup診断](reference/daily-workflow.ja.md) | 実装済み | 制限付きの進捗・相関IDをstderrへ出力し、最終応答を検証。切断後も処理終了まで排他を保持し、非対話の確認は入力待ちしない。専用Linuxでの検証とWindows既定エントリー・IDEの確認は別。 |
| [Workspaceのパス参照・fork](design/workspace-workflow.md) | 実装済み | 明示したリポジトリの準備、所有者を固定したパスによる再開、正規ライフサイクルを使う停止中のGit/OCI独立コピー。復旧が必要なコピーは所有記録を保持。Windows自動接続と大規模リポジトリ性能は未確認。 |
| [TCP/UDP開発接続](design/network-connections.md) | 実装済み | ゲストの明示的なループバック待受、生成ID付きポリシー・承認、任意のルール期限、接続中の失効を実装。HTTP/SNI・送信元保護を維持。専用基盤の検証は限定範囲で、外部インターネット・VPN・Windows UI全体は未完了。 |
| [導入・Host](guides/installation.ja.md) | 実装済み | Ubuntu 26.04以降・専用WSL 2、コントローラー経由のsetup/doctor、永続的な信頼済み`haco-host`。Ubuntuのログインシェルは変更しない。Windowsネイティブの`haco.exe`は未提供。既存の非rootアクセスグループを検証して管理ユーザーに再利用。現行P/PF修正と日本語Windows新規導入のパッケージ確認は残る。 |
| [リポジトリ・Workspace](guides/git-workflow.ja.md) | 実装済み | 既存ブランチのclone、独立した管理コピーとcollectionを作成。停止後も排他的リースを保持。構成メンバーの編集と準備中断からの一般的な復旧は未完了。 |
| [Envの作成・停止・再開・削除](guides/data-lifetime.ja.md) | 実装済み | 管理対象・外部Workspaceから作成、一覧・状態・停止・開始・削除。rootfsは使い捨てだがWorkspaceとStoreは削除後も保持。所有状態が不明なら解放を拒否。`switch-base`は無効・保留。 |
| [SSH・エディター](design/client-and-interactive-access.md) | 実装済み | 鍵・設定を再利用するセットアップ、`haco open`の選択、鍵を固定したProxyCommand／controller UDS経由のポート不要SSH。既定はVS Code、`--client ssh`でシェル。プロキシ変数は自動設定。広範なIDE・Windows・AHPの確認はクライアント依存。 |
| [対話端末の画面サイズ](design/controller-client-transport.ja.md#対話端末の画面サイズ) | 実装済み | Host・Envのシェルで初期サイズを渡し、別途合意した制限付きのサイズ変更要求を送信。Linuxでは専用のraw PTYを使用。構成要素・実PTY試験で編集、サイズ変更、バイト保持、終了、端末復元を確認。導入済みIncus・Windows・WSLの実機確認は未完了。 |
| [通常のGit操作](guides/git-workflow.ja.md) | 部分実装 | 全head取得と各refの独立した読み取り確認、新規ブランチ一つまたは既存fast-forwardの内容固定push承認。clone/fetchはpush許可を与えず、mainへの承認を維持。packは32 MiBまでで、LFS/submodule、force・削除・複数ref、不明結果からの一般的な復旧は制限または非対応。認証付き実機利用と巨大レポは別途受入が必要。 |
| [ポリシー・設定](reference/configuration.ja.md) | 実装済み | revision付きの参照・編集、要求単位の承認と範囲の保存。deny、require-approval、allowの順で優先。プロバイダー・デスクトップの広い検証は別途必要。通知失敗で権限は付与されない。 |
| [ネットワーク・DNS](design/egress-authorization.ja.md) | 実装済み | コントローラー所有のStandardプロキシ、Incus下位層の直接通信防止、信頼済み送信元に結び付けたDNS。名前解決と接続の許可は別。カーネルの送信元保護を観測する処理は実装済みだが、Windowsパッケージ全工程と偽装パケットの検証は別途必要。VPN/NRPT・再起動の組合せ・広いIncus構成の確認は未完了。 |
| [セットアップ手順・プレビュー](design/project-setup.ja.md) | 部分実装 | Host実体ごとの自動設定、明示的なscriptのみの再適用と非公開出力・終了値の記録、EnvのWorkspaceセットアップ、承認付きの限定HTTPプレビュー、対象を絞ったdoctorを実装。再作成・キャンセル、既定ブラウザー、広いアプリの検証は残る。 |
| [一時実行](design/temporary-execution.ja.md) | 実装済み | `haco run`は一時Envを作り、終了時に後始末を行う。明示したWorkspaceは保持。既定は出力取得、`-i/-it`で入力・対話端末に対応し実Incusで確認済み。Windowsの逐次実行は未確認。後始末失敗時は所有記録を保持。 |
| [永続OCI](design/persistent-oci-store.md) | 部分実装 | Workspace単位のStore自動初期化・再利用、排他的接続、停止中の独立コピー。`--no-oci`で省略可能。Host領域のコピー境界と完了証明による復旧を実装。導入構成・実行基盤バージョン全体の確認とDocker Store互換は残る。 |
| [Baseの作成](design/base-images-and-custom-environments.md) | 実装済み | 定義からのビルド、論理ID・revisionの参照、確認付きイメージ削除。Baseは初期rootfsの選択と由来を表し、スナップショットが保持する実体の依存先ではない。 |
| [スナップショット・復元・コピー](design/environment-snapshots.md) | 実装済み | 停止した管理Workspace/OCI、名前付き使い捨てデータと独立保存rootfsを対象に、新しいEnvと権限を作成。外部Workspace取得、その場での置換、任意の稼働アプリの整合性は非対応。 |
| [保持対象の削除](guides/data-lifetime.ja.md) | 実装済み | Workspace、作成Base、Store全体、元リポジトリを確認して削除。参照とnative childが保持対象を保護。所有記録は不存在確認後のみ解放。 |
| [OCIイメージ単位の操作](design/oci-image-deletion.ja.md) | 部分実装 | 接続中・Host・非接続nerdctlの一覧・削除、未使用候補の確認付き削除。非接続ツール配備はLinux amd64のみ。導入済みコントローラー全体の確認と非接続Dockerは未完了。 |
| [ストレージ・容量回収](design/storage-reclamation.ja.md) | 実装済み | Incus所有のBtrfs プール（`compress=zstd:3`）、rootfs・データ配置、登録済みWindows/WSLの容量回収を実装。CIで実際の回収量を確認。現在の記録がなければ読み取りだけで結果なしと応答し、不正な記録はエラー。既存環境と中断workerの実機レビューは未確認。 |
| [Envのexport/import](design/environment-transfer.ja.md) | 部分実装 | 停止した管理bundle、検証済みLinux配備、導入済みコントローラー・Windows投影ファイル経路を実装。管理データのWSL間移送を一構成で確認し、停止したcontainerdのイメージ・書込みデータの移送も確認済み。稼働中の移行や環境全体のバックアップではなく、import後の認証Gitと広い実行基盤整合性は未完了。 |
| [データ退避・環境置換](guides/data-evacuation.ja.md) | 部分実装 | 現行schema16の追加データ・世代参照・ライフサイクル未完了記録を含む読み取り専用棚卸しと、明示した通常ファイルのアーカイブを実装。スナップショット失敗を再現した隔離試験も実施。Incus標準のexport/importで分割イメージ2件を移送。単一形式と新Env起動は未確認。環境全体の分類・取得・復元比較と最終置換は未完了。 |
| [AWS S3](design/aws-operations.ja.md) | 部分実装 | 承認付きの制限ある一覧・検証済みobject取得、送信元を固定したゲスト要求を実装。リポジトリ・模擬native試験あり。認証を伴う実AWS検証はスキップ。EC2のEnv プロバイダーではない。 |
| [通知・クライアントAPI](reference/interaction-events.ja.md) | 実装済み | `pkg/clientadapter`、情報を絞った対話 event、`haco-notify`のブラウザー・OS・VS Code アダプター。Windowsレビューは限定範囲で確認済み。新規トーストからの人間の判断とLinux通知起動は未確認。 |
| [Seed撤去](design/oci-seed-and-cow.ja.md) | 実装済みの候補 | Seedの実行・構築・harvest・カタログ・収集・推奨と、旧イメージ削除・再有効化の状態を撤去。現行のBase・管理対象イメージ・OCI Storeと、独立した任意のDocker連携を維持。旧版の互換性・移行は対象外。 |
| [クラウド・registry・管理UI](status/architecture-and-roadmap.md) | 延期 | 具体的なクラウドEnv プロバイダー、必須のlocal registry、管理UI、Storeの同時書込み共有、live 移行は現行機能ではない。プロバイダー境界と将来方針は保持。 |

正規ライフサイクルは送信元保護を含む基盤の削除完了後だけ所有記録を解放します。
実体の不存在や不完全な所有状態は復旧が必要な状態として返します。独立したカタログ変更APIを
削除し、一時実行の後始末とマーカー処理を共通化しました。rootfs importは対応CPUを2系統に
制限したままIncus SDKの別名を受け付けます。[所有権](adr/0002-environment-lifecycle-ownership.md)と
[移送](design/environment-transfer.ja.md)を参照してください。

## 確認の境界

コマンドと既定値は[CLI参照](reference/cli.ja.md)、設定は[設定参照](reference/configuration.ja.md)を参照してください。

CIはリポジトリの試験、実Incusの基盤試験、パッケージ導入試験を区別します。実AWS・非公開 registry・実デスクトップなど、前提がなくスキップした検証は合格扱いにしません。障害時の権限・リース・後始末は[失敗時の表](reliability/failure-injection-matrix.md)と各設計が定義します。

古い開発日誌の全文はGit履歴に残ります。現在の判断に必要な固有の証拠・未解決事項は[検証証拠](status/acceptance-evidence.ja.md)に集約しています。

## main向け日英CLI統合候補

**partial**：#580/#583の日常操作の日英表示と共通の縦型ヘルプを現在のmainへ
再利用しています。明示的なJSON指定、ポート指定の不要なSSH、Git接続、
Experimental VS Codeの既存動作を保持します。通常のWindows起動とHostセッションへの
言語引き継ぎを実装し、インストーラはOS言語設定を保持します。結果表示の全文翻訳と
新しい導入済み環境での確認は未完了です。
[言語対応範囲](reference/cli-language.ja.md)と
[検証記録](status/acceptance-evidence.ja.md#main-cli-language)を参照してください。

## 通知・インストーラの統合候補

**実装済みの候補**：#583の同種失敗通知の1分間の集約と、BATの結果表示・キー待ちを
再利用します。承認・回復要求の通知、監査・再開位置、元の終了コードを保持します。
ローカルの通知回帰とWindows BAT・ConPTY確認は成功しました。新しい配布パッケージの
Windows/SSH確認とExplorer操作は別の残件です。
[通知仕様](reference/interaction-events.ja.md#同じ失敗によるnative通知の連発)と
[インストーラの結果](design/installer.md#windows-final-result)を参照してください。

## 詳細案内とHost準備の共通化

**実装済み候補:** #592/#593の日英の引数・オプション説明と保持データの結果を現在mainへ合わせました。
削除確認はclientの共通処理を使い、controllerの所有権検査を維持します。#659の旧Hostツール二重準備も撤去します。
ローカル試験と新規導入での受入は区別します。旧版互換・旧版移行は今回のM0〜M5の対象外です。


## 対話一時実行の候補

**実装済み候補:** `haco run -i/-it`を上限付きの双方向転送と共通runライフサイクルへ接続します。
生成時の識別子を使い、同じ名前で作り直した別Envを片付けません。分割済みの責務と共通cleanup結果処理を維持します。
旧版の移行・代替cleanupは対象外です。新しいローカル試験と実機受入は区別します。


## キャッシュ世代管理の共通処理

**部分実装:** `haco cache settings/configure/status/collect`でHost設定、新規Envの対象登録、停止中の領域全体の収集、独立した世代コピーの再利用を扱います。名前付き履歴・クリア・完了記録付き復旧は実装済みです。snapshot/copyは未収集データを保持します。追加領域のポータブル転送は新しい所有権で取り込む実装済み候補で、対応Incusでの実機受入は未確認です。既存Envへの後付け登録は未完成です。Workspace・OCI保持は別に維持します。[キャッシュ世代管理](design/cache-generations.ja.md)を参照してください。

## PackerによるBase作成の候補

**部分実装:** 実際のPacker HCL2と外部スクリプトを通常のbuild用Envで実行します。追加adapterは既存のBase公開・cleanupを共有します。Packerの完走・download・導入済みWindowsの受入は未確認です。[Packerの操作](design/packer-base-builds.ja.md)を参照してください。

キャッシュ履歴・クリアの後続: 実装済み候補。名前付き履歴で現在の再利用元と保持中の候補を分ける。
確認時のrevisionで固定して再利用元をリセットし、共通の所有権付き削除を使う。
既存Env・Workspace・OCIデータと結果不明のコピーは保持する。名前付き完了記録の復旧と追加データ転送は実装済み、孤立した再利用元の一覧は未完成。[キャッシュ操作](design/cache-generations.ja.md#収集データの確認とクリア)を参照。

キャッシュ完了復旧: 名前付き領域の完了記録があるコピーと世代選択を復旧する実装済み候補。共通復旧はOCIも含め対象の所有権を固定する。native完了が不明な場合、孤立source、既存Envへの追加は未完了。新しい実機復旧の受入は別に確認する。

## 手元のアプリからTCP接続

**実装済み候補:** `haco env tunnel --target-port 8080 demo`でアプリ用のループバック待受を開きます。Linuxでは手元、通常のWSL/Host入口では導入済みWindowsクライアントを使い、Env作成実体とWSL登録を固定します。手元の操作を終了すると待受と接続も閉じます。引数、プロセス通信、中断、導入先は既存の開発成果を共通処理として再利用しています。新しい導入済み確認は別扱いで、DNSモードとVPN/NRPT受入は未完了です。[通信の契約](design/controller-client-transport.ja.md)を参照してください。

名前解決の選択: 実装済み候補。Env作成時の`--dns host|backend|disabled`を受け付け、通常はPhysical Hostを使い、snapshot/copy/転送で設定を保持します。無効時はguestの処理を再起動してもcontrollerが問い合わせを拒否します。導入済み3モードの受入は未確認。[名前解決](design/name-resolution.ja.md)を参照。

## 構成整理と旧CLIの廃止

製品の入口は `cmd/haco` です。実装の場所は[構成案内](../CONTRIBUTING.md#repository-map)を参照してください。`hacoq`、旧GitHub capability、Docker status/prepareコマンドは撤去しました。現行Git・OCIとclient helperは保持しています。native Ubuntuではcontroller経由の管理コマンドを使えますが、製品の対話的なtrusted Hostシェル接続コマンドはありません。Windowsのログイン経路は保持しています。[決定記録](adr/0096-responsibility-layout-and-cli-retirement.ja.md)も参照してください。
