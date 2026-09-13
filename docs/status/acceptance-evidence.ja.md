# 検証証拠と未確認の範囲

[English](acceptance-evidence.md) | 日本語

状態: 検証記録。ここに記載した試験は過去のコミットで実施されたものです。文書整理時に実機試験を再実行したという意味ではありません。現在の機能は[実装状況](../IMPLEMENTATION_STATUS.ja.md)を参照してください。

成功・失敗・スキップは試験構成に結び付けて読みます。同じ実行内の一部成功や後続の成功だけで、別の失敗原因が解決したとは判断しません。日々の実行ログを追記するのではなく、判断を変える証拠と未解決事項だけを更新します。

<a id="installation"></a>

## インストールとHost

| 候補・試験 | 結果と制約 |
|---|---|
| `1817e7c` / 管理ユーザーの準備 | 専用の復旧用WSLで、旧関数は`hacocoon`グループが存在するため失敗した。修正後のインストーラー関数は管理アカウントを作成し、対象WSLだけを再起動して既定ユーザーを確認した。PowerShellの構成要素回帰試験も成功。アカウント準備の確認であり、完全なパッケージ導入や環境全体の復元ではない。日本語Windowsへの新規導入と接続案内の実機確認は残る。 |
| `c749ff9`, `81c0d16` / `9049df3`, `4df465a` | Windows/Ubuntuパッケージの導入、コントローラーとの往復、プロキシ許可・直接通信拒否、WSL登録の再実行をM0–M1の範囲で確認。Windows自体の再起動は対象外。 |
| `029ff08`, `42e2fb3`, `1b2d6ae` / run 34051931616 | 以前のIncusのSIGKILL失敗は保持。PIDとworkerの追跡は名前空間をまたぐ古いPID記録の再利用を強く示すが、すべてのkill元やOOMは確定していない。起動ガードの専用試験は成功。同一起動内のPID再利用は保護対象外。 |
| `63fdf24`; 現在の `73f63f2` | アカウントが存在するのにWSL照会が失敗した原因は未確定。現行コードの再試行は回数を制限した読み取り確認のみ。現行のbinfmt P/PF修正はリポジトリ試験で確認しており、以前の手動回避をパッケージ修正の合格とは扱わない。 |
| `c86c43e`; `61a26e3` | 当該候補でWindowsコマンド・ドライブ投影、データ保持、標準OpenSSHを確認。`029ff08`のBase切替・配布は過去の方式。`61a26e3`ではWSL再起動後も所有対象とデータを保持。ドライブの着脱、より広いWindowsツール、更新中断からの復旧は未確認。 |

元の詳細、検証用構成の識別子、ログ・成果物へのリンクは [整理前の実装状況](https://github.com/SLktEx/Hacocoon/blob/73f63f23b4a57d2fefa5764c523798b1fa8e1962/docs/IMPLEMENTATION_STATUS.md)に固定コミットで保持されています。現在の操作手順としては使用しないでください。

<a id="development"></a>

## 開発・設定・承認

| 候補・試験 | 結果と制約 |
|---|---|
| `1817e7c` / 導入済み送信元保護の観測 | 専用Incus/WSLで、復旧した停止中Envを正規経路で起動した後、世代と実際の保護ルールを固定して確認した。最初の起動はコントローラーのソケット準備前に失敗し、それ以前の単独観測も失敗した。これらを成功へ読み替えない。パッケージ導入からのWindows SSH検証、偽装パケットの送信、再起動・再作成の全工程を証明する結果ではない。 |
| `7a4d122`; `f8517ba`; `8752431` | 導入済み候補でclone、Workspace、SSH、承認付きGit push、停止を確認。`f8517ba`で再開とSSH鍵追加を確認。旧`8752431`環境で初回再開失敗後に手動のVS Code接続が成功。一時ルールと接続は削除済み。 |
| `347ca50`, `d4aef8d` / run 34139245378; `bcc1baf` | 導入済み環境でHost設定の保存・再適用・更新・解除、基本のWorkspaceセットアップ、HTTP/Edgeプレビュー、doctor前提を確認。再作成・キャンセル、既定ブラウザー起動、VPN/NRPTは別の未確認事項。 |
| `2584ec6` / run 34152700897; `71dbb4f` | 設定の往復は成功したがpreview doctorは失敗。保存済みルールが空配列の場合のローカル設定不具合は修正済み。後のプレビュー成功だけでは以前の失敗原因は確定しない。 |
| `eb16300b6700` | GitHubへの承認済みpush、保存した「毎回確認」の再利用、拒否を実機確認。他の保存方法はリポジトリ試験のみ。試験ブランチ`codex/stage-b-b-first-20260906`は`3ca59c…`へ進み、拒否した`26a7b…`と`git-save-eb16300`は保持。push結果が不明な場合はリモート確認が必要。 |
| `470a2b8` / run 34188963290 | Windows通知サービス、VS Code・端末の確認、古い要求の拒否を確認。新しいトーストからの人間の判断とLinux通知起動は未確認。以前の`711005a`はサービス起動制限に達しており、後のリセット・新unitによる確認とは区別する。 |
| `c05528a`; `226991b` / run 34479510230; `684e411` | DNSの同等ポリシーと既定拒否を確認。後続の試験でDNSサービスの再実行と導入済みGit-over-SSHが成功。SSHのプロキシ変数は現在自動設定される。再起動・VPN・NRPTの組合せは未完了。 |
| `4adfe19` / run 34115004878; `093ed159b80e` | 実Incusで一時実行とキャンセル後の後始末を確認。対話入力・TTYと内容を持つOCIの検証は未実施。AWSはゲストからの拒否と制限付きの模擬ダウンロードを確認。認証を伴う実AWSの一覧・取得は前提不足でスキップ。 |

元の詳細、検証用構成の識別子、ログ・成果物へのリンクは [整理前の実装状況](https://github.com/SLktEx/Hacocoon/blob/73f63f23b4a57d2fefa5764c523798b1fa8e1962/docs/IMPLEMENTATION_STATUS.md)に固定コミットで保持されています。現在の操作手順としては使用しないでください。


開発経路の過去の失敗も保持します。`72096d8` は名前解決の前に DNS 設定で失敗し、`7eecbdf` でサービス管理機構の準備待ちを特定しました。`c05528a` は後続の CRLF 手順入力、`5f824b4` は標準入力の引き継ぎ漏れ、`39b5ce4` は DNS の start-limit-hit で失敗しました。`bffc3fd` のプレビューは拡張子のない確認ファイルを文字列として比較して失敗しました。後の成功は修正範囲の証拠であり、すべての中断・再起動条件を保証しません。

ローカルの `71dbb4f` 設定・プレビュー検証では既定拒否と管理者ルール8件を保持し、対象を絞ったアーカイブ取得ルール4件を追加後に削除しました。ポート36059で Workspace の確認データを取得し、自分の検証資源を削除しました。SSH は準備しておらず、以前の失敗原因は未解明です。元の詳細記録: [設定](https://github.com/SLktEx/Hacocoon/blob/73f63f23b4a57d2fefa5764c523798b1fa8e1962/docs/reference/configuration.ja.md)、[名前解決](https://github.com/SLktEx/Hacocoon/blob/73f63f23b4a57d2fefa5764c523798b1fa8e1962/docs/design/name-resolution.ja.md)、[プロジェクト準備](https://github.com/SLktEx/Hacocoon/blob/73f63f23b4a57d2fefa5764c523798b1fa8e1962/docs/design/project-setup.ja.md)。

<a id="storage"></a>

## スナップショット・削除・容量回収

| 候補・試験 | 結果と制約 |
|---|---|
| PR #493, #501; `a2fcb72` / runs 34297739368, 34297739417 | スナップショット復元はBase実体の保持と元Envの削除に依存せず、権限を新規発行する。Base作成・Env作成・SSHはmachine-ID/stdio問題と600秒タイムアウトを経て成功。所有対象は不存在確認後のみ解放し、診断証拠は元の記録に残す。 |
| PR #504–507; `c4842c2` | Workspace、作成したBase、Store全体、元リポジトリの削除を個別の実機試験で確認。Base試験ではWindowsのSSH aliasが初回失敗。元リポジトリ試験は初期化で停止した回があり、その後段のスキップを成功とは扱わない。 |
| `4d9038b7`; `bd1c9a5` / run 34417051340; `9484d06` / run 34493016558 | 接続中・Hostのイメージ操作は実行基盤アダプターの試験構成で成功。非接続nerdctlの配備と単体controller/CLIは588.51秒で成功し、未使用候補の確認付き削除も成功。導入済みcontroller/Standard全体と非接続Dockerは未完了。 |
| `f3f5557`, `ae0c245` | Docker 28.5.2/vfs、nerdctl 2.3.5/containerd 2.3.3の試験構成でHost OCI領域の隔離、停止中のコピー、完了証明に基づく復旧を確認。初回のroot不一致は修正前の失敗。不明なプロバイダー完了状態は引き続き解放を拒否。全バージョン・導入構成の合格ではない。 |
| `5100d86` / run 34623036552, job 103341362151 | 公開reclaimの開始・結果確認が成功。Windows割当量は7,964,983,296→4,224,712,704バイト（3,740,270,592回収）。仮想1 TiB・Incus 128 GiBの容量は不変。Linux discard、指定WSLの停止・圧縮・再開、保持Workspace/OCI/スナップショットの復元を確認。 |
| `4369fdb`, `d675c5a`, `de72119`; earlier Windows trials | Job関連の起動エラーの原因は未確定。`d675c5a`はaccess-denied 5でLinux未開始の未完了を保持。`de72119`はworker失敗を保存したが起動元へ通知しなかった。以前のOpenVirtualDiskエラー32とディスク段階だけの成功は統合回収の証明ではない。junction拒否は成功、一部symlink試験は権限不足でスキップ。既存環境・電源断・セッション間・中断workerの実機レビューは未確認。 |

元の詳細、検証用構成の識別子、ログ・成果物へのリンクは [整理前の実装状況](https://github.com/SLktEx/Hacocoon/blob/73f63f23b4a57d2fefa5764c523798b1fa8e1962/docs/IMPLEMENTATION_STATUS.md) および [当該設計の検証記録](https://github.com/SLktEx/Hacocoon/blob/73f63f23b4a57d2fefa5764c523798b1fa8e1962/docs/design/storage-reclamation.md)に固定コミットで保持されています。現在の操作手順としては使用しないでください。

<a id="transfer"></a>

## Env移送・データ退避

| 候補・試験 | 結果と制約 |
|---|---|
| `653dc985` / [Incus・Btrfsの検証](https://github.com/SLktEx/Hacocoon/actions/runs/34636086219/job/103384200857) | 停止済みcontainerdの移送が115.96秒で再度成功。正規経路と配布コントローラーのimportで、元Env削除後もイメージIDと書込みデータを保持し、保存コンテナーを明示的に起動した。export前に元コンテナーとデーモンを停止している。稼働プロセス、Docker、任意アプリ、環境全体の復元は未確認。 |
| `1817e7c` / Incusイメージの退避 | 分割形式のコンテナーイメージ2件をWindows上の保持ファイル経由で専用WSL間移送した。全部品のSHA-256、専用イメージprojectでのfingerprintと種別の一致、保持ファイルが不変であることを確認。最初の`incus project show --format`は未対応オプションで失敗し、その記録を保持して`project list`で観測後にimportした。projectとイメージは保持。単一形式、新しいEnvの起動、環境全体の置換は未検証。 |
| `3d0dd9a` / run 34430493864; `b7297a3` / run 34455660292 | Linux export専用試験とimport統合試験は成功。ローカルexportの480/720秒とimport統合の720.07秒はタイムアウト失敗。カタログ`2545909325`、`462967548`、`1920048809`に証拠を保持。狭い範囲の成功でこれらの失敗は解消されない。 |
| `a58d553`, `6d5e027`, `e598270`, `0cc27a5`, `7517c27`; `684e411` / run 34471376143 | 単体コントローラーのimportは20.35秒で成功した一方、89.56秒の統合試験は失敗。その後もSSH、Git未導入（127）、apt（100）の失敗を経て、導入済みexport/import・元Env削除・新しい固定鍵によるSSHが成功。同じWindows実行内の別の承認失敗は失敗のまま。 |
| `c4449e1` / run 34482712957; `6974272` / run 34501951826 | Windows投影ファイルのサイズ・hashとimportを確認。停止したcontainerdデータの移送も成功（統合103.36秒、controller22秒）。`ba4dbcd`/`8103e3f`はexport前に失敗。WindowsネイティブCLI・直接DrvFSへの配備、import後の認証Git、Docker/BuildKit・任意の稼働DBの整合性は未完了。 |
| G2 inventory and direct-file fixtures | schema 10–13の読み取り専用棚卸し（9は非対応）、カタログ・native イメージ参照、未解決投影の明示を実装。あるファイル走査は47,848項目（ファイル39,061、ディレクトリ5,050、mount17、symlink3,718、特殊ファイル2）。`/var/lib/haco-file-inventory-4kvt5eyb/wsl-root.json`の未取得部分を含め、列挙を取得完了や削除権限とは扱わない。 |
| G2 synthetic 復旧 fixtures | 直接tar取得・復元20.59秒、保存rootfs取得24.52秒、スナップショット削除EPERM試験の手動取得・復元・後始末11.74秒（rootファイル9.37秒）が成功。隔離した試験構成であり、環境全体や実際の破損からの復旧ではない。 |
| Historical encrypted fixtures | 暗号化試験の10,440バイトのファイルは残ったが、`/tmp`の元の復号鍵がPrivateTmpをまたいで失われた。復旧済みとは扱わない。その後の保持修正（2.31秒）と新しいWindows所有の鍵も元の鍵を復元しない。現行の取得は通常のアーカイブであり、暗号化・鍵作成は必須ではない。現在の実機CIは内容・独立したハッシュ・明示的なxattr復元・不完全な取得の拒否を検査する。旧暗号化試験は任意の過去の証拠として保持し、維持対象CIでは実行しない。 |
| `61a26e3` 管理対象の cross-WSL 検証用構成 | 別の新規WSLで550,415,872バイトのbundleを171.27秒でimport。試験ボリューム全94項目、ゲストID、新しい固定鍵でのSSH・ローカルGit、再作成後のWorkspace/OCI保持を確認。初回のHost生UID比較はID変換により失敗。apt 100は限定ポリシーで解決。認証Git、Windows VS Code、環境全体、G4置換はスキップまたは未完了。元WSLは保持。 |

元の詳細、検証用構成の識別子、ログ・成果物へのリンクは [整理前の実装状況](https://github.com/SLktEx/Hacocoon/blob/73f63f23b4a57d2fefa5764c523798b1fa8e1962/docs/IMPLEMENTATION_STATUS.md) および [当該設計の検証記録](https://github.com/SLktEx/Hacocoon/blob/73f63f23b4a57d2fefa5764c523798b1fa8e1962/docs/design/environment-transfer.md)に固定コミットで保持されています。現在の操作手順としては使用しないでください。

[1817e7cの追加検証記録](https://github.com/SLktEx/Hacocoon/blob/1817e7cf9e8910bf31ae23b714053e2580d03fa0/docs/IMPLEMENTATION_STATUS.md)。

## 日常の入口とsetup診断

状態: **implemented、専用WSL/Linuxでの日常手順の実機確認は成功**。

Host setupは上限付き固定stage/state/reasonとrequest IDをstream表示し、構造化journalに
診断を記録します。最終応答欠落を成功にせず、切断後も実処理終了まで排他を維持します。
日常Env操作の進捗はstderr、JSON結果はstdoutです。helpと英日手順は作成・開く・作業・
停止・再開へ案内し、Env削除と保持データ削除を区別します。非対話確認は入力待ちになりません。
初回SSH失敗の案内は、原因をパッケージや承認と断定せず、読み取り専用の承認一覧とPolicy確認も示します。

2026-09-12、専用`hacocoon-v2`のUbuntu 26.04 / Incus 6.0.5へ`6cf9295`を
ローカルビルドし、common installerで導入しました。開発用bundleの実機確認であり、
署名付きreleaseのprovenance確認ではありません。

| 実際の確認 | 結果 |
|---|---|
| common installer・setup・Host doctor | 終了0。Incus所有Btrfsの実体・mount policy、trusted HostのDNS/HTTPSを含むdoctor全6項目が成功。 |
| 一般ユーザーの作成・開く・作業 | 既定Base、外部Workspace、`--no-oci`。SSHでsourceを編集し、大文字出力をbuildして期待内容と比較。 |
| 停止・起動・再度開く | stopped状態を観測。Workspace成果物とrootfs markerを保持し、pin付きLinux SSHで再接続。 |
| 重複作成 | `already_exists`で拒否し、既存Envは利用可能なまま。 |
| 実端末の空入力選択 | desktop・接続の変更前にキャンセル。 |
| Env削除 | 対象とデータへの影響を正規削除前に表示。Env不在と外部Workspaceファイル保持を確認。 |
| setup途中失敗 | 合成customizationのexit 29で失敗stage/reason/request IDを表示。偽の完了表示や合成秘密出力のCLI/journal露出なし。検証用recipeは正規APIで除去。 |
| setup中断 | 観測側は完了表示せず終了。元の処理のjournal完了まで別setupはbusyで拒否。 |

最初のSSH準備はdefault-deny Policy・sshd不在の状態で失敗しました。現在のEnv世代と
Ubuntu配布先だけに限定した明示的Policy更新後、通常のSSH準備が完了しました。
これは今回の経路の結果であり、他の導入で同じ汎用SSHエラーの原因を断定するものではありません。
パッケージ導入後は今回追加した4規則だけを正規設定APIで除去し、元のdefault denyへ戻しました。
その状態でも実端末のLinux SSHで保持ファイルの確認が成功しました。

専用の開発network namespace・veth・限定した外側NATで、Incus/controllerを他WSLの
bridgeから分離しています。両serviceはそのnamespaceのsysfs/Btrfs mount viewを共有します。
これは手元の検証設定であり製品既定値ではありません。WSLカーネルのAppArmorは無効で、
カーネルや隔離チェックは変更していません。AppArmorの隔離受入を意味しません。
Windows IDE・通常入口のinstall、Windows SSH、private Git/registry、OCI保持、
distribution全体のcold restartは今回未検証です。

repository検証では標準local test/vet/通知client・全体race・fixture CLI E2E・Python interop
22件が成功しました。Windows installer component fixtureもWindows上で、読み取り実通信先を
対象WSLへ固定して成功しています。installerの変更経路はmockです。Linux PowerShellは
SystemDirectoryが空のため、このWindows専用fixtureを実行できません。これらのテストを
上記provider/clientの実機結果に読み替えません。test workflowは`dev/v2`向けPRも検証します。

[日常手順](../reference/daily-workflow.ja.md)を参照してください。このdev/v2の受入は、別のdev/2.xのWorkspace・TCP/UDP実装より前の記録です。

## M0/M1統合候補、PR #583

`37c3e679`のtest（34713814211）とUbuntu installer（34713814185）は旧横並びenv
ヘルプの期待値で失敗しました。Ubuntu導入自体は完了しています。`195172f4`で縦ヘルプと
終了コード・出力先を検証する形へ直し、test（34714239387）とUbuntuパッケージ利用経路
（34714239415）が成功しました。後続のIncus LTS共通化を受入済みとする証拠ではありません。

Windows（34713814252、job 103607232075）は追加したパイプ接続のキー待ち試験で失敗し、
実導入へ進んでいません。ローカルのパイプ試験成功ではheadless consoleを証明できませんでした。
ConPTYで配布BATを起動し、待機表示・キー入力・終了37を確認する試験へ置換しました。
ローカルConPTY、0/1/37/3010、前提不足のnative componentは成功しました。置換後のCIと
Explorerダブルクリックは別ゲートです。隔離ライブラリをsandboxから読めず一度実行できなかった
後、導入時と同じ実行権限では成功しています。製品の権限を変更した結果ではありません。

LTS回帰は追加／欠落／重複鍵、異なる配布元・系列、依存操作の失敗、新しい既存系列を拒否します。
改行入り版がシェル検証を通る問題を試験で検出・修正しました。helper 6件、Host準備7件、
梱包、Go HostDiagnosticsが成功しました。既存WSLのパッケージ・保持データは変更していません。
7.0製品の新規導入受入は確認待ちです。

`cc18a60b`の全体test CI（34715459033）は成功しました。Incus（34715459013）は
standalone・Core／egress／lifecycleが成功しましたが、owned-BtrfsのStore保守試験で失敗しました。
製品CLIがパイプによる削除確認を正しく終了2で拒否する一方、旧試験は端末からの拒否を期待していました。
拒否・承認の両方を専用Linux PTYから回答する形へ直し、製品の確認条件やcleanup検証は維持しています。
実Incusでの再実行が必要です。

Ubuntu（34715458982）はIncus 7.0.1の導入・版確認後、Ubuntu版と異なるdaemonパスで
boot guardの採用に失敗しました。`2c9faa07`はroot・namespace・systemd MainPID照合を維持して
Zabblyの正規パスを認識し、回帰20件が成功しました。不明な稼働daemonは引き続き拒否します。
修正後の導入受入は未確認です。Windows（34715459045）はConPTY componentと7.0.1確認後、
同じboot guardのパスで失敗しました。driverがBATの明示的失敗を認識せず、さらに28分待って
timeoutになりました。最終失敗を認識して所有端末を閉じるよう修正し、2回目のBATで初回受入を
修復しない回帰試験を追加しました。後続のWindows SSH・reclaim・通知試験はSKIPです。

`96bbbdf8`の全体test（34717575075）は成功し、Ubuntu（34717575034）では修正済みboot guardを
含む配布物の導入が成功しました。次の試験が一般ユーザーで特権診断の旧`hacoq doctor`を実行して
失敗しました。Incus 7はdaemon管理権限のないユーザーで失敗を返し、rootでの診断は成功しています。
正規のcontrollerと利用グループを通る製品`haco doctor`で確認するよう直しました。
Incus-admin付与や権限緩和は追加していません。後続journey／security試験はSKIPで、再実行が必要です。

Windows `96bbbdf8`（34717575063）はキャッシュ付き配布物導入、WSL停止／再起動／再導入、
controller経由HTTPSと直接egress拒否、鍵pin付きWindows OpenSSHと停止からの再開、
VS Code 1.136.1 Remote-SSHでの実ファイル読み書き・端末実行が成功しました。
project setupの保存／再実行／失敗／更新も成功しました。一方、承認review・preview setup・
Env exportの独立probeは失敗しました。承認試験は端末必須のCLIへパイプ入力していたため、
専用PTYと分離したJSON／診断出力へ修正し、回帰6件が成功しました。previewは未解決のreview後に
失敗しましたが、因果関係は再実行まで未確定です。exportはexport段階で失敗し、分類証拠が不足して
いたため、生出力を出さない固定allowlist診断を追加しています。転送成功やcleanupを推定しません。

同じrunでLinux Btrfs／ext4 trimは成功しましたが、公開Windows reclaimは先行転送の保持manifestが
作成されず失敗しました。前提不足であり、VHDX圧縮の実行・成功ではありません。最後のnative通知経路は
SKIPです。範囲を限定した成功で、これらの残る失敗を消しません。

`96bbbdf8`のIncus run 34717575098は、その後、有効な全jobが成功しました。
standalone runtime、専用Btrfs poolでのBase／snapshot／CoW／importと実PTYによる
Store整理承認、Coreのegress／lifecycleが対象です。private registryは従来の前提不足で
SKIPでした。このLinuxの成功で、上記Windows転送の失敗を解決済みとは扱いません。

候補`5fe184a6`はtest CI 34719977795とUbuntu配布物34719977797が成功しました。
通常ユーザーのdoctor、導入済みjourney、network／spoofing guardを含みます。
Windows 34719977824も導入／SSH／VS Codeの既存範囲に加えて、pending-reviewの
saved-ask／今回拒否／一度だけ許可／再確認／cleanupと、Edge preview／再利用／拒否が
成功しました。先行するこの2probeの失敗は解消しましたが、GUIだけでの承認完了ではありません。
exportは引き続きFAIL（`phase=export`、固定診断`volume-export,unavailable`）です。
Linux trimは成功し、Windows reclaimは転送manifest不足で再び失敗、native通知はSKIPでした。
固定分類により、生出力を公開せず残る失敗箇所を絞れました。

Incus 7.0.1の`cmdStorageVolumeExport.run`は`--force`なしの既存出力先を拒否します。
controller所有の`/proc/<pid>/fd/<fd>`は意図的に存在するため、この匿名FDだけに同flagを
付けるよう修正しました。所有者・linkなし・非公開の通常ファイルであることを回帰で確認し、
利用者の既存出力先の上書き拒否は維持します。Windows転送の再試験が必要であり、
ソース上の原因特定やcomponent試験だけで実機の修正完了とは扱いません。

候補`3cac2e95`はtest CI 34721760620とUbuntu配布物34721760552が成功しました。
Incus 34721760571は専用Btrfsのaggregate exportとcleanup stepが失敗しました。
導入ログは7系ではなく **6.0.5-8** です。standalone用helperとは別の`ci-incus-core.sh`が
まだUbuntuパッケージを導入し、`>= 6.0.5`を受け入れていました。7系用export flagで
残る導入経路の差異が顕在化しました。Coreとstandaloneは成功、private registryと
後続Btrfs probeはSKIPです。先行する`96bbbdf8`と`5fe184a6`の全有効job成功も、
Core／Btrfsについては6.0.5での確認に限定します。Ubuntu／Windows配布物の7.0.1での証拠とは
区別します。両CI入口を署名検証付き共通LTS導入・版範囲検証へ統一し、経路の回帰を追加しました。
Core／Btrfsの7系受入は再実行待ちであり、失敗したcleanup記録も保持します。

その後、`3cac2e95`のWindows run 34721760573は全有効stepが成功しました
（試験merge `d3fb94a6e872bb44fd1d67d08ee842a1882f4843`、Incus 7.0.1）。
実配布物でbundle hash／不変性、export→元Env削除→import、Windows SSHと保持workからの
再作成が成功し、先行export失敗とmanifest不足を解消しました。公開reclaimはLinux trim、
WSL停止、VHDX割当量7,931,428,864→4,033,871,872 bytesへの圧縮、再開、Host sentinel保持、
非接続Workspace／OCI／snapshot restoreを確認しました。native通知の登録、stale／malformed／
他者所有の拒否、controller購読と所有listener cleanupも成功しました。人によるトーストクリックと
新規GUI回答、VPN／NRPTは引き続き明示的な **SKIP** です。既存のSSH／VS Code／review／previewも
成功しました。使い捨てWindows／WSL一構成での確認であり、巨大レポ実測・日本語UI全体・配布完了ではありません。

`655f03ce`はtest CI 34723210857が成功しました。Incus 34723210668ではCore／Btrfsも
**7.0.1**を確認し、standaloneとCore jobが成功しました。Btrfsのaggregate export/import、
OCI書込みデータ、snapshot／restore／copy、保持とnative child拒否も成功し、この基盤での
先行export失敗を解消しました。その後`TestRealIncusSourceDeletionE2E`のsnapshot `show`が
失敗しました。Incus 7はvolumeとsnapshotを別引数に取るため、fixtureを既存create/deleteと
同じ分離形式へ修正しました。製品の削除判定は変更していません。試験が所有cleanup前に止まり、
storage cleanup stepはFAIL、全体Incus cleanup stepはPASSでした。後続Btrfs probeと
private registryはSKIPで、Btrfs job全体の成功には再実行が必要です。

`28ca8ebf`はtest 34723923596とUbuntu 34723923612が成功しました。Incus 34723923619は
Core／standaloneとBtrfs aggregate／取得元削除、native volume import、定義からのBase buildが
成功しました。後続persistent-copy fixtureにも同じsnapshot結合引数が残っておりFAILとなったため、
volume／snapshotを別引数へ修正しました。workflowのcleanup 2stepはPASSですが、失敗した試験が
明示的に保持した復旧fixtureの記録は残します。Store maintenanceとprivate registryはSKIPです。
Btrfs job全体の成功とは扱いません。

Windows／WSL通常入場の表示言語自動選択は、製品CLI・control API・共通判定・architecture試験が
成功しました。user-path assertionも12件成功し、言語markerなし・echoのみ・重複・不一致を拒否します。
Windows上の直接PowerShell照会は`en`でした。一方、既存`hacocoon-second`からの明示native queryは
`exec format error`でFAIL、読み取り確認では`/proc/sys/fs/binfmt_misc/WSLInterop`登録がありませんでした。
既存環境の修復・設定変更は行わず、fallback回帰の成功とは分けて失敗を保持します。新規配布物の
Windows CIではHacocoonのoverrideを注入せず、通常入場・再起動・再導入後の実Host sessionの値を
WindowsユーザーのUI設定と照合する項目を追加しました。その実行結果は確認待ちです。

### Incus 7とWindows配布物の統合候補確認

`0c79f8209eec42b597cc811a9114e0351d8226d7`（PR #583）は、test
[34724986411](https://github.com/SLktEx/Hacocoon/actions/runs/34724986411)、Ubuntu
[34724986358](https://github.com/SLktEx/Hacocoon/actions/runs/34724986358)、Incus
[34724986357](https://github.com/SLktEx/Hacocoon/actions/runs/34724986357)、Windows
[34724986361](https://github.com/SLktEx/Hacocoon/actions/runs/34724986361)が成功しました。
Incusは**7.0.1**を確認し、有効なstandalone／Core／Btrfs jobがすべてPASSです。Base build、native import、
取得元削除、snapshot／copy、persistent CoW、Store maintenance／cleanupを含みます。private registryはSKIPです。
先行するsnapshot fixture 2件の失敗はこの候補で解消しましたが、保持した過去の失敗履歴は消しません。

Windows導入・再起動・再導入ではoverrideなしでWindows UI設定とHostの`HACO_UI_LANGUAGE=en`が一致しました。
native interop、通常の鍵固定SSH・接続再利用・再開、実VS Codeの編集／terminal、CLI承認、preview、
export／delete／importと保持データからの再作成がPASSです。public reclaimのVHDX割当量は
**7,864,320,000 → 3,974,103,040 bytes**で、再開後のHost sentinel・Workspace／OCI保持・snapshot復元も成功しました。
通知の所有権・古い／不正要求拒否・listener cleanupもPASSです。人間のtoast click／新GUI回答とVPN／NRPTは明示SKIPです。
日本語Windows実機確認と既存ローカルWSLInteropの失敗は未解決です。この結果は新GUI session実装前であり、
その受け入れや配布済みを意味しません。
