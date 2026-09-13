# 実装状況

[English](IMPLEMENTATION_STATUS.md) | 日本語

現在のmilestone位置は **v0.64**。番号の正本と履歴は[バージョンとリリース状況](status/versioning-and-release-status.ja.md)を参照してください。

このページは現在の開発候補のコードで使える範囲を示します。初めて使う場合は[利用開始ガイド](guides/getting-started.ja.md)へ進んでください。実機で確認できた範囲・失敗・スキップは[検証証拠](status/acceptance-evidence.ja.md)、残りの開発方針は[ロードマップ](status/architecture-and-roadmap.md)が管理します。

**状態:** 実装済み、部分実装、未実装の計画、延期を区別します。実装済みでも全Host・プロバイダーでの動作確認を意味しません。

| 機能 | 状態 | 使える範囲・制約・残課題 |
|---|---|---|
| [日常操作・setup診断](reference/daily-workflow.ja.md) | 実装済み | 制限付きの進捗・相関IDをstderrへ出力し、最終応答を検証。切断後も処理終了まで排他を保持し、非対話の確認は入力待ちしない。専用Linuxでの検証とWindows既定エントリー・IDEの確認は別。 |
| [Workspaceのパス参照・fork](design/workspace-workflow.md) | 実装済み | 明示したリポジトリの準備、所有者を固定したパスによる再開、正規ライフサイクルを使う停止中のGit/OCI独立コピー。復旧が必要なコピーは所有記録を保持。Windows自動接続と大規模リポジトリ性能は未確認。 |
| [TCP/UDP開発接続](design/network-connections.md) | 実装済み | ゲストの明示的なループバック待受、生成ID付きポリシー・承認、任意のルール期限、接続中の失効を実装。HTTP/SNI・送信元保護を維持。専用基盤の検証は限定範囲で、外部インターネット・VPN・Windows UI全体は未完了。 |
| [導入・Host](guides/installation.ja.md) | 実装済み | Ubuntu 26.04以降・専用WSL 2、コントローラー経由のsetup/doctor、永続的な信頼済み`haco-host`。Ubuntuのログインシェルは変更しない。Windowsネイティブの`haco.exe`は未提供。既存の非rootアクセスグループを検証して管理ユーザーに再利用。英語Windows配布物の入場／interopは確認済み。日本語Windows新規導入は未確認。 |
| [リポジトリ・Workspace](guides/git-workflow.ja.md) | 実装済み | 既存ブランチのclone、独立した管理コピーとcollectionを作成。停止後も排他的リースを保持。構成メンバーの編集と準備中断からの一般的な復旧は未完了。 |
| [Envの作成・停止・再開・削除](guides/data-lifetime.ja.md) | 実装済み | 管理対象・外部Workspaceから作成、一覧・状態・停止・開始・削除。rootfsは使い捨てだがWorkspaceとStoreは削除後も保持。所有状態が不明なら解放を拒否。`switch-base`は無効。別Baseは通常の環境再作成で選択。 |
| [SSH・エディター](design/client-and-interactive-access.md) | 実装済み | 鍵・設定を再利用するセットアップ、`haco open`の選択、鍵を固定したループバック SSH。既定はVS Code、`--client ssh`でシェル。プロキシ変数は自動設定。広範なIDE・Windows・AHPの確認はクライアント依存。 |
| [対話端末の画面サイズ](design/controller-client-transport.ja.md#対話端末の画面サイズ) | 実装済み | Host・Envのシェルで初期サイズを渡し、別途合意した制限付きのサイズ変更要求を送信。Linuxでは専用のraw PTYを使用。構成要素・実PTY試験で編集、サイズ変更、バイト保持、終了、端末復元を確認。導入済みIncus・Windows・WSLの実機確認は未完了。 |
| [通常のGit操作](guides/git-workflow.ja.md) | 部分実装 | 全heads fetch/pull（1024 heads・pack合計32 MiB）とcontroller所有の資格情報による内容固定push。単一refの新規ブランチ作成とfast-forward更新を正確なrefで個別承認し、作成競合は拒否。pushの保存記録と正確なrefの読み取り照合を実装し、不明な結果は再送せず保持。大きなpack、ブランチ削除、force／複数ref push、LFS/submodule、一般的な復旧は非対応。全heads／新規pushの実Env受入は未確認。 |
| [ポリシー・設定](reference/configuration.ja.md) | 実装済み | revision付きの参照・編集、要求単位の承認と範囲の保存。deny、require-approval、allowの順で優先。プロバイダー・デスクトップの広い検証は別途必要。通知失敗で権限は付与されない。 |
| [ネットワーク・DNS](design/egress-authorization.ja.md) | 実装済み | コントローラー所有のStandardプロキシ、Incus下位層の直接通信防止、信頼済み送信元に結び付けたDNS。名前解決と接続の許可は別。カーネルの送信元保護を観測する処理は実装済みだが、Windowsパッケージ全工程と偽装パケットの検証は別途必要。VPN/NRPT・再起動の組合せ・広いIncus構成の確認は未完了。 |
| [セットアップ手順・プレビュー](design/project-setup.ja.md) | 部分実装 | Host実体ごとの自動設定、明示的なscriptのみの再適用と非公開出力・終了値の記録、EnvのWorkspaceセットアップ、承認付きの限定HTTPプレビュー、対象を絞ったdoctorを実装。再作成・キャンセル、既定ブラウザー、広いアプリの検証は残る。 |
| [一時実行](design/temporary-execution.ja.md) | 実装済み | `haco run`は作成世代を照合して片付け、指定したWorkspace／OCIを保持。既定は出力収集、`-i`はパイプ、`-it`は実端末。逐次出力の実機確認は未完了。片付け失敗時は所有記録を保持。 |
| [永続OCI](design/persistent-oci-store.md) | 部分実装 | Workspace単位のStore自動初期化・再利用、排他的接続、停止中の独立コピー。`--no-oci`で省略可能。Host領域のコピー境界と完了証明による復旧を実装。導入構成・実行基盤バージョン全体の確認とDocker Store互換は残る。 |
| [Baseの作成](design/base-images-and-custom-environments.md) | 実装済み | 定義からのビルド、論理ID・revisionの参照、確認付きイメージ削除。Baseは初期rootfsの選択と由来を表し、スナップショットが保持する実体の依存先ではない。 |
| [スナップショット・復元・コピー](design/environment-snapshots.md) | 実装済み | 停止した管理Workspace/OCIと独立保存rootfsを対象に、新しいEnvと権限を作成。外部Workspace取得、その場での置換、任意の稼働アプリの整合性は非対応。 |
| [保持対象の削除](guides/data-lifetime.ja.md) | 実装済み | Workspace、作成Base、Store全体、元リポジトリを確認して削除。参照とnative childが保持対象を保護。所有記録は不存在確認後のみ解放。 |
| [OCIイメージ単位の操作](design/oci-image-deletion.ja.md) | 部分実装 | 接続中・Host・非接続nerdctlの一覧・削除、未使用候補の確認付き削除。非接続ツール配備はLinux amd64のみ。導入済みコントローラー全体の確認と非接続Dockerは未完了。 |
| [ストレージ・容量回収](design/storage-reclamation.ja.md) | 実装済み | Incus所有のBtrfs プール（`compress=zstd:3`）、rootfs・データ配置、登録済みWindows/WSLの容量回収を実装。CIで実際の回収量を確認。現在の記録がなければ読み取りだけで結果なしと応答し、不正な記録はエラー。既存環境と中断workerの実機レビューは未確認。 |
| [Envのexport/import](design/environment-transfer.ja.md) | 部分実装 | 停止した管理bundle、検証済みLinux配備、導入済みコントローラー・Windows投影ファイル経路を実装。管理データのWSL間移送を一構成で確認し、停止したcontainerdのイメージ・書込みデータの移送も確認済み。稼働中の移行や環境全体のバックアップではなく、import後の認証Gitと広い実行基盤整合性は未完了。 |
| [データ退避・環境置換](guides/data-evacuation.ja.md) | 部分実装 | 読み取り専用の棚卸しと明示した通常ファイルのアーカイブを実装。スナップショット失敗を再現した隔離試験も実施。Incus標準のexport/importで分割イメージ2件を移送。単一形式と新Env起動は未確認。環境全体の分類・取得・復元比較と最終置換は未完了。 |
| [AWS S3](design/aws-operations.ja.md) | 部分実装 | 承認付きの制限ある一覧・検証済みobject取得、送信元を固定したゲスト要求を実装。リポジトリ・模擬native試験あり。認証を伴う実AWS検証はスキップ。EC2のEnv プロバイダーではない。 |
| [通知・クライアントAPI](reference/interaction-events.ja.md) | 実装済み | `pkg/clientadapter`、情報を絞った対話 event、`haco-notify`のブラウザー・OS・VS Code アダプター。Windowsレビューは限定範囲で確認済み。新規トーストからの人間の判断とLinux通知起動は未確認。 |
| [旧OCI Seed・Docker](reference/cli-migration.md) | 部分実装 | 任意の`HACO_PLUGIN_OCI=nerdctl`または`docker`連携は移行用`hacoq`に残る。Seedのbuild/publish・保護を実装。非公開 registry・COW・失敗条件の広い確認は残る。現行の永続Store手順とは別。 |
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

## 開発候補への統合

Workspace入口・fork、TCP/UDP接続、日常setup診断は#581でmain反映済みです。
この候補はmainの`74bc2205`と、後続の日英CLI、世代を照合する対話run、Git改善、
GUI／通知内回答を統合します。mainの人向け出力、Host Git/gh、世代ごとのscript処理を
再利用しています。配布と統合候補の実機確認は別です。
[統合の検証記録](status/acceptance-evidence.ja.md#main-sync-candidate)を参照してください。


M1は**partial**です。階層別の日英ヘルプに位置引数・オプション・既定値を追加し、日常の失敗・
保持／再開案内、BATの共通終了表示・待機省略、nativeの同種失敗通知抑制を実装しました。
`0c79f820`では英語WindowsからHostへの表示言語一致と、有効なIncus 7.0.1 Core／Btrfs、
Ubuntu／Windows配布物のworkflowがすべてPASSです。通常SSH／VS Code、移送、public reclaim、
通知経路を確認しました。残る結果・エラーの翻訳、日本語Windows経路、元のSSH失敗再現、
導入済み長文入力／resizeは未完了です。人間のtoast／新GUI回答は未確認です。
[検証証拠](status/acceptance-evidence.ja.md)を参照してください。開発ブランチ上の実装・確認であり、
main反映済み・配布済みを意味しません。

VS CodeはローカルGUI内で回答まで完結し、共通保存範囲と表示snapshotに束縛したprivate sessionを使用します。`e7ba7987`で導入済み画面の描画・古い要求の拒否がPASS、新規要求への人の回答とWindows通知内回答は残件です。[承認の契約](design/pending-approval-review.ja.md)と[検証証拠](status/acceptance-evidence.ja.md)を参照してください。

一時runのcleanupは共通lifecycle APIで正確な作成identityを必須にし、未完了runが
ある間の名前再利用を拒否します。旧記録のidentity不足は復旧待ちとして保持します。
所有権修正は`9f4cf510`で全native workflowがPASSです。後続stdin／TTYも同じlifecycleを使い、
実Incusのpipe／PTYは`b3169814`でPASSです。Windowsの入力確認は一度FAILし、driver修正後の`9767fd93`でPASSしました。[一時実行](design/temporary-execution.ja.md)を参照してください。

保持データの確認表、削除範囲・確認・結果、snapshot結果の見出しを共通の日英辞書で表示します。確認付き削除の5経路は一つの確認処理を使い、警告・確認文の出力失敗時は削除しません。元のエラー、JSON、所有権確認、未入力時の拒否は維持しています。全結果の翻訳と日本語Windowsは引き続きpartialです。

Windowsの確認は非表示COM helperから通知内のページ・選択欄で回答し、固定したディストリビューションと共通の非公開承認sessionを使います。実COMと英日ToastGeneric履歴には構成要素の検証がありますが、人のクリック・見切れ・導入済み新規回答は未確認です。[承認の契約](design/pending-approval-review.ja.md)と[検証証拠](status/acceptance-evidence.ja.md)を参照してください。

後続の開発修正で、native COM二重起動の古い要求拒否を保持し、固定分類の失敗診断を追加しました。
実Windows COM回帰は旧実装でFAIL、修正後PASSです。#611の導入済みWindows失敗は経路の再確認まで
未解決とします。この構成要素の分類回帰だけで、その失敗原因を特定したとは扱いません。
