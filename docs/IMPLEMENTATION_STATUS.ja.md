# 実装状況

[English](IMPLEMENTATION_STATUS.md) | 日本語

現在のmilestone位置は **v0.68**。番号の正本と履歴は[バージョンとリリース状況](status/versioning-and-release-status.ja.md)を参照してください。

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
| [導入・Host](guides/installation.ja.md) | 実装済み | Ubuntu 26.04以降・専用WSL 2、コントローラー経由のsetup/doctor、永続的な信頼済み`haco-host`。Ubuntuのログインシェルは変更しない。Windowsネイティブの`haco.exe`は未提供。既存の非rootアクセスグループを検証して管理ユーザーに再利用。WindowsパッケージCIで通常入口・native interopの候補版別の証拠あり。広い導入構成は別途確認。 |
| [リポジトリ・Workspace](guides/git-workflow.ja.md) | 実装済み | 既存ブランチのclone、独立した管理コピーとcollectionを作成。停止後も排他的リースを保持。独立forkのメンバー選択とcheckout/linked worktree入力を実装。その場でのメンバー編集と準備中断からの一般的な復旧は非対応・未完了。 |
| [Envの作成・停止・再開・削除](guides/data-lifetime.ja.md) | 実装済み | 管理対象・外部Workspaceから作成、一覧・状態・停止・開始・削除。rootfsは使い捨てだがWorkspaceとStoreは削除後も保持。所有状態が不明なら解放を拒否。`switch-base`は無効・保留。 |
| [SSH・エディター](design/client-and-interactive-access.md) | 実装済み | 鍵・設定を再利用するセットアップ、`haco open`の選択、鍵を固定したProxyCommand／controller UDS経由のポート不要SSH。既定はVS Code、`--client ssh`でシェル。プロキシ変数は自動設定。広範なIDE・Windows・AHPの確認はクライアント依存。 |
| [対話端末の画面サイズ](design/controller-client-transport.ja.md#対話端末の画面サイズ) | 実装済み | Host・Envのシェルで初期サイズを渡し、別途合意した制限付きのサイズ変更要求を送信。Linuxでは専用のraw PTYを使用。構成要素・実PTY試験で編集、サイズ変更、バイト保持、終了、端末復元を確認。導入済みIncus・Windows・WSLの実機確認は未完了。 |
| [通常のGit操作](guides/git-workflow.ja.md) | 部分実装 | 全head取得と各refの独立した読み取り確認、新規ブランチ一つまたは既存fast-forwardの内容固定push承認。clone/fetchはpush許可を与えず、mainへの承認を維持。送信済みpushは正確なold/newとリモート観測を照合し、自動再送しない。fetchは検証した祖先履歴を再利用し、既存targetへのpushは手元にある旧履歴を省く。新規targetも正確なrefの新しい読み取り判断を通して利用可能な公開済み祖先を再利用する。複数headのfetchは個々のpackを順番に取り込み、batch合計32 MiB超に対応する。個々の全体packや差分自体は32 MiBまでで、LFS/submodule、force・削除・複数ref、不明結果からの一般的な復旧は制限または非対応。認証付き実機利用と巨大レポは別途受入が必要。 |
| [ポリシー・設定](reference/configuration.ja.md) | 実装済み | revision付きの参照・編集、要求単位の承認と範囲の保存。deny、require-approval、allowの順で優先。プロバイダー・デスクトップの広い検証は別途必要。通知失敗で権限は付与されない。 |
| [ネットワーク・DNS](design/egress-authorization.ja.md) | 実装済み | コントローラー所有のStandardプロキシ、Incus下位層の直接通信防止、信頼済み送信元に結び付けたDNS。名前解決と接続の許可は別。Envのhost/backend/disabled選択をsnapshot/copy/importで保持し、手元のTCP待受からcontroller経由で転送。カーネルの送信元保護を観測する処理は実装済みだが、Windowsパッケージ全工程と偽装パケットの検証は別途必要。VPN/NRPT・再起動の組合せ・広いIncus構成の確認は未完了。 |
| [セットアップ手順・プレビュー](design/project-setup.ja.md) | 部分実装 | Host実体ごとの自動設定、明示的なscriptのみの再適用と非公開出力・終了値の記録、EnvのWorkspaceセットアップ、承認付きの限定HTTPプレビュー、対象を絞ったdoctorを実装。再作成・キャンセル、既定ブラウザー、広いアプリの検証は残る。 |
| [一時実行](design/temporary-execution.ja.md) | 実装済み | `haco run`は一時Envを作り、終了時に後始末を行う。明示したWorkspaceは保持。既定は出力取得、`-i/-it`で入力・対話端末に対応し実Incusで確認済み。Windowsの逐次実行は未確認。後始末失敗時は所有記録を保持。 |
| [永続OCI](design/persistent-oci-store.md) | 部分実装 | Workspace単位のStore自動初期化・再利用、排他的接続、停止中の独立コピー。`--no-oci`で省略可能。Host領域のコピー境界と完了証明による復旧を実装。導入構成・実行基盤バージョン全体の確認とDocker Store互換は残る。 |
| [Baseの作成](design/base-images-and-custom-environments.md) | 実装済み | 定義からのビルド、論理ID・revisionの参照、確認付きイメージ削除。非圧縮のコンテナarchive取り込みも共通の公開・後始末を利用。Baseは初期rootfsの選択と由来を表し、スナップショットが保持する実体の依存先ではない。 |
| [PackerによるBase作成](design/packer-base-builds.ja.md) | 部分実装 | 通常builder内で実Packer 1.16.0がHCL2と外部shellを評価し、公開・後始末を共通化。導入後の実ビルドは通常の依存取得権限で停止。独自plugin・arm64・再利用の実機確認は残る。 |
| [キャッシュ世代管理](design/cache-generations.ja.md) | 部分実装 | Host設定による作成時の登録、停止中の領域単位収集、独立CoW再利用、履歴・完了証明による復旧・共通元削除・確認付きEnv内掃除を実装。snapshot/copy/transferで未収集データも保持。既存Envの追加登録、結果不明コピーの中止は未完了。巨大性能は後続。 |
| [スナップショット・復元・コピー](design/environment-snapshots.md) | 実装済み | 停止した管理Workspace/OCI、名前付き使い捨てデータと独立保存rootfsを対象に、新しいEnvと権限を作成。削除失敗時の構成要素・存在・参照・次の操作を表示。外部Workspace取得、その場での置換、任意の稼働アプリの整合性は非対応。 |
| [保持対象の削除](guides/data-lifetime.ja.md) | 実装済み | Workspace、作成Base、Store全体、元リポジトリを確認して削除。参照とnative childが保持対象を保護。所有記録は不存在確認後のみ解放。 |
| [OCIイメージ単位の操作](design/oci-image-deletion.ja.md) | 部分実装 | 接続中・Host・非接続nerdctlの一覧・削除、未使用候補の確認付き削除。非接続ツール配備はLinux amd64のみ。導入済みコントローラー全体の確認と非接続Dockerは未完了。 |
| [ストレージ・容量回収](design/storage-reclamation.ja.md) | 実装済み | Incus所有のBtrfs プール（`compress=zstd:3`）、rootfs・データ配置、登録済みWindows/WSLの容量回収を実装。CIで実際の回収量を確認。現在の記録がなければ読み取りだけで結果なしと応答し、不正な記録はエラー。準備・起動段階とWindowsエラーを日英で案内。手元の開始失敗と実際の中断worker確認は別の残件。 |
| [Envのexport/import](design/environment-transfer.ja.md) | 部分実装 | 停止した管理bundle、検証済みLinux配備、導入済みコントローラー・Windows投影ファイル経路を実装。管理データのWSL間移送を一構成で確認し、停止したcontainerdのイメージ・書込みデータの移送も確認済み。稼働中の移行や環境全体のバックアップではなく、import後の認証Gitと広い実行基盤整合性は未完了。 |
| [データ退避・環境置換](guides/data-evacuation.ja.md) | 部分実装 | 現行schema16の追加データ・世代参照・ライフサイクル未完了記録を含む読み取り専用棚卸しと、明示した通常ファイルのアーカイブを実装。スナップショット失敗を再現した隔離試験も実施。Incus標準のexport/importで分割イメージ2件を移送。単一形式と新Env起動は未確認。現在必要なデータの選定と復元後開発は未完了。旧版の再構築・置換は現在のM0〜M5対象外。 |
| [AWS S3](design/aws-operations.ja.md) | 部分実装 | 承認付きの制限ある一覧・検証済みobject取得、送信元を固定したゲスト要求を実装。リポジトリ・模擬native試験あり。認証を伴う実AWS検証はスキップ。EC2のEnv プロバイダーではない。 |
| [通知・クライアントAPI](reference/interaction-events.ja.md) | 実装済み | 情報を絞ったeventと任意のadapter。VS Code GUIとWindows通知内のページで共通review/Policyを通して明示回答が完結。開くだけでは回答しない。新規の導入GUI・人の回答・Linux起動は未確認で、native/部品の証拠は別管理。 |
| [Seed撤去](design/oci-seed-and-cow.ja.md) | 実装済み | Seedの実行・構築・harvest・カタログ・収集・推奨と、旧イメージ削除・再有効化の状態を撤去。現行のBase・管理対象イメージ・OCI Storeと、独立した任意のDocker連携を維持。旧版の互換性・移行は対象外。 |
| [クラウド・registry・管理UI](status/architecture-and-roadmap.md) | 延期 | 具体的なクラウドEnv プロバイダー、必須のlocal registry、管理UI、Storeの同時書込み共有、live 移行は現行機能ではない。プロバイダー境界と将来方針は保持。 |

正規ライフサイクルは送信元保護を含む基盤の削除完了後だけ所有記録を解放します。
実体の不存在や不完全な所有状態は復旧が必要な状態として返します。独立したカタログ変更APIを
削除し、一時実行の後始末とマーカー処理を共通化しました。rootfs importは対応CPUを2系統に
制限したままIncus SDKの別名を受け付けます。[所有権](adr/0002-environment-lifecycle-ownership.md)と
[移送](design/environment-transfer.ja.md)を参照してください。

## 確認の境界

コマンドと既定値は[CLI参照](reference/cli.ja.md)、設定は[設定参照](reference/configuration.ja.md)を参照してください。古いrootコマンドとSeed/Docker操作は[CLI移行情報](reference/cli-migration.md)へ分離しています。

CIはリポジトリの試験、実Incusの基盤試験、パッケージ導入試験を区別します。実AWS・非公開 registry・実デスクトップなど、前提がなくスキップした検証は合格扱いにしません。障害時の権限・リース・後始末は[失敗時の表](reliability/failure-injection-matrix.md)と各設計が定義します。

古い開発日誌の全文はGit履歴に残ります。現在の判断に必要な固有の証拠・未解決事項は[検証証拠](status/acceptance-evidence.ja.md)に集約しています。

## mainの統合と開発候補

main `e4d99700` / [#699](https://github.com/SLktEx/Hacocoon/pull/699)に、
#687/#688に続いて#689〜#693と#696〜#698を統合した。復元ツリーの照合、容量回収結果の
日本語表示、SSH失敗の分類、環境名からの最新保存の復元、Git既存履歴の再利用と
複数headの順次fetchはmain反映済み。カタログ単位の共通ライフサイクルロックと
仮想ディスク観測ハンドルの寿命修正も統合した。
[ADR 0105](adr/0105-catalog-lifecycle-locks.ja.md)と
[ADR 0103](adr/0103-virtual-disk-observation-lifetime.ja.md)を参照。

#699の同一head `51ba4f24`で5系統のCIすべてが成功した。Windowsパッケージの通常導入、
SSH・実エディタ・転送、Workspace/OCI/snapshotの保持と復元、公開容量回収、
native通知の所有確認・拒否経路を確認した。実際の割当容量を2,840,592,384 bytes回収し、
仮想容量は維持した。過去のWindows失敗や専用ローカル環境の登録情報の観測問題は
解決済みと推定せず、[検証証拠](status/acceptance-evidence.ja.md)に範囲を保持する。
小さな復元ツリーの照合を、現在データ全体や巨大リポジトリの確認とは扱わない。

[#700](https://github.com/SLktEx/Hacocoon/pull/700)はHost・プロジェクトのセットアップ結果と
次の操作を共通の日英表示へ揃える。スクリプトの元の出力、明示的な再実行、診断値、
縦ヘルプを維持する。開発ブランチに実装済みで、ローカル全体確認と通常パッケージ生成が成功した。
[CLI全体の翻訳範囲](reference/cli-language.ja.md)はまだ部分的である。

本人ログイン、通知クリック、新たなVS Code内の回答はリリース後に確認し、
CI成功済みの実装のmain反映を止める条件にはしない。候補・main・配布物は区別し、
新たなリリースは作成していない。性能・追加の厳密検証と対象外の旧版再構築は、
現在データの保持と分けて[M0〜M5ロードマップ](status/architecture-and-roadmap.md)に記録する。

開発中の通信案内の追補では、登録・取り消し・ルール保存・接続口の結果を日英で表示する。
機械向けの結果と許可の意味は維持する。M1/M3の表示改善の一部であり、
導入済み受入とCLI全体の翻訳範囲は別に追跡する。
