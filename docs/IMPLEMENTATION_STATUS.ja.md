# 実装状況

[English](IMPLEMENTATION_STATUS.md) | 日本語

現在のmilestone位置は **v0.58**。番号の正本と履歴は[バージョンとリリース状況](status/versioning-and-release-status.ja.md)を参照してください。

このページはmainのコードで使える範囲を示します。初めて使う場合は[利用開始ガイド](guides/getting-started.ja.md)へ進んでください。実機で確認できた範囲・失敗・スキップは[検証証拠](status/acceptance-evidence.ja.md)、残りの開発方針は[ロードマップ](status/architecture-and-roadmap.md)が管理します。

**状態:** 実装済み、部分実装、未実装の計画、延期を区別します。実装済みでも全Host・プロバイダーでの動作確認を意味しません。

| 機能 | 状態 | 使える範囲・制約・残課題 |
|---|---|---|
| [導入・Host](guides/installation.ja.md) | 実装済み | Ubuntu 26.04以降・専用WSL 2、コントローラー経由のsetup/doctor、永続的な信頼済み`haco-host`。Ubuntuのログインシェルは変更しない。Windowsネイティブの`haco.exe`は未提供。既存の非rootアクセスグループを検証して管理ユーザーに再利用。現行P/PF修正と日本語Windows新規導入のパッケージ確認は残る。 |
| [リポジトリ・Workspace](guides/git-workflow.ja.md) | 実装済み | 既存ブランチのclone、独立した管理コピーとcollectionを作成。停止後も排他的リースを保持。構成メンバーの編集と準備中断からの一般的な復旧は未完了。 |
| [Envの作成・停止・再開・削除](guides/data-lifetime.ja.md) | 実装済み | 管理対象・外部Workspaceから作成、一覧・状態・停止・開始・削除。rootfsは使い捨てだがWorkspaceとStoreは削除後も保持。所有状態が不明なら解放を拒否。`switch-base`は無効・保留。 |
| [SSH・エディター](design/client-and-interactive-access.md) | 実装済み | 鍵・設定を再利用するセットアップ、`haco open`の選択、鍵を固定したループバック SSH。既定はVS Code、`--client ssh`でシェル。プロキシ変数は自動設定。広範なIDE・Windows・AHPの確認はクライアント依存。 |
| [対話端末の画面サイズ](design/controller-client-transport.ja.md#対話端末の画面サイズ) | 実装済み | Host・Envのシェルで初期サイズを渡し、別途合意した制限付きのサイズ変更要求を送信。Linuxでは専用のraw PTYを使用。構成要素・実PTY試験で編集、サイズ変更、バイト保持、終了、端末復元を確認。導入済みIncus・Windows・WSLの実機確認は未完了。 |
| [通常のGit操作](guides/git-workflow.ja.md) | 部分実装 | コントローラー所有の資格情報によるfetch/pullと内容を固定したpush。制限付きの単一ref fast-forward push承認。大きなpack、ブランチ作成・削除、force push、LFS/submodule、不明な結果からの一般的な復旧は通常手順では非対応。 |
| [ポリシー・設定](reference/configuration.ja.md) | 実装済み | revision付きの参照・編集、要求単位の承認と範囲の保存。deny、require-approval、allowの順で優先。プロバイダー・デスクトップの広い検証は別途必要。通知失敗で権限は付与されない。 |
| [ネットワーク・DNS](design/egress-authorization.ja.md) | 実装済み | コントローラー所有のStandardプロキシ、Incus下位層の直接通信防止、信頼済み送信元に結び付けたDNS。名前解決と接続の許可は別。カーネルの送信元保護を観測する処理は実装済みだが、Windowsパッケージ全工程と偽装パケットの検証は別途必要。VPN/NRPT・再起動の組合せ・広いIncus構成の確認は未完了。 |
| [セットアップ手順・プレビュー](design/project-setup.ja.md) | 部分実装 | Host設定とEnvのWorkspaceセットアップ、承認付きの限定HTTPプレビュー、対象を絞ったdoctorを実装。再作成・キャンセル、既定ブラウザー、広いアプリの検証は残る。 |
| [一時実行](design/temporary-execution.ja.md) | 実装済み | `haco run`は一時Envを作り、終了時に後始末を行う。明示したWorkspaceは保持。出力取得のみで対話stdin/TTYは非対応。後始末失敗時は所有記録を保持。 |
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

## 確認の境界

コマンドと既定値は[CLI参照](reference/cli.ja.md)、設定は[設定参照](reference/configuration.ja.md)を参照してください。古いrootコマンドとSeed/Docker操作は[CLI移行情報](reference/cli-migration.md)へ分離しています。

CIはリポジトリの試験、実Incusの基盤試験、パッケージ導入試験を区別します。実AWS・非公開 registry・実デスクトップなど、前提がなくスキップした検証は合格扱いにしません。障害時の権限・リース・後始末は[失敗時の表](reliability/failure-injection-matrix.md)と各設計が定義します。

古い開発日誌の全文はGit履歴に残ります。現在の判断に必要な固有の証拠・未解決事項は[検証証拠](status/acceptance-evidence.ja.md)に集約しています。

## 開発候補への統合

実装済み（開発候補のみ）: [Workspace入口・独立fork](design/workspace-workflow.md)、[TCP/UDP接続](design/network-connections.md)、[日常操作・setup診断](reference/daily-workflow.ja.md)。main反映・配布・統合候補の実機確認とは区別します。


M1は**partial**です。階層別の日英ヘルプ、日常失敗・保持／再開の日本語案内、BATの共通
終了表示・待機省略、nativeの同種失敗通知抑制を実装しました。正規化したWSL→Host sessionの
言語引き継ぎは実装済みで、Windowsの自動言語選択と残る翻訳は未完了です。共通Incus 7.0 LTSは
Ubuntu配布物と範囲を限定したWindows／Incus受入が成功しました。承認／previewは試験修正後に
成功し、volume exportと依存するWindows reclaimは失敗が残ります。
[commitごとの証拠](status/acceptance-evidence.ja.md)を参照してください。配布済み・実機での言語転送完了ではありません。
