# 検証証拠と未確認の範囲

[English](acceptance-evidence.md) | 日本語

状態: 検証記録。ここに記載した試験は過去のコミットで実施されたものです。文書整理時に実機試験を再実行したという意味ではありません。現在の機能は[実装状況](../IMPLEMENTATION_STATUS.ja.md)を参照してください。

成功・失敗・スキップは試験構成に結び付けて読みます。同じ実行内の一部成功や後続の成功だけで、別の失敗原因が解決したとは判断しません。日々の実行ログを追記するのではなく、判断を変える証拠と未解決事項だけを更新します。

## 一時実行の所有権とストリーム

`9f4cf5105f01c5da7dfe40e080651979789c799b`（PR #590）はtest
[34729490922](https://github.com/SLktEx/Hacocoon/actions/runs/34729490922)、Ubuntu
[34729490923](https://github.com/SLktEx/Hacocoon/actions/runs/34729490923)、Incus
[34729490810](https://github.com/SLktEx/Hacocoon/actions/runs/34729490810)、Windows
[34729490822](https://github.com/SLktEx/Hacocoon/actions/runs/34729490822)がPASSです。
Incus Btrfs job 103649531126は7.0.1で、出力捕捉型の一時実行・exit 17・保持Workspace・
キャンセルcleanup・切り離したStoreのcleanupに成功しました。private registryはSKIPです。
後続stdin／TTY実装前の、使い捨て実機構成における所有権修正の証拠です。

後続ストリーム候補では、早期終了のintegration raceが一度FAILしました。未読入力が残る
Unix socketを閉じるとresetで最終receiptを失う問題です。入力停止通知とEOF排出の確認を
追加後、control／control API／CLIのrace回帰を3回実行してPASSし、未読入力を残す早期終了も
計30回成功しました。バイナリpipe・Linux PTYと通常Windows ConPTYの編集／resize／終了／
復元を既存CIで確認します。以下に正確な実機結果を記載し、component成功から推定しません。

全体local test CIの初回は既存`TestUDPIdleCountsBothDirections`（idle期限200ms）がFAILでした。
単独20回と、その後の`ci-local.sh test`全体再実行はPASSです。初回失敗の原因は未確定で、
relayの動作や期限は緩和していません。最終候補の6packageのprocess／temporary race、
文書検査、workflow-policy、実機fixture構文検査も別途PASSです。

`b31698148db03915504c476e52e617fe70ecb527`（PR #591）のtest 34732860619、
Ubuntu 34732860608、Incus 34732860628はPASSです。Btrfs job 103658786664は
Incus **7.0.1** で、2MiB binary pipe・実PTYの編集／resize・exit 17・端末復元・cleanupを
**PASS** と記録しています。既存の出力捕捉型キャンセルと保持WorkspaceもPASSです。
private registryはSKIPです。

Windows run 34732860626は新TTYの入力確認がFAILでした。意図した入力より先に空行を読み、
resize・exit 17・端末／catalog復元は観測されています。ドライバーが起動行にCRLFを送り、
二つ目の改行がguestに残っていました。Enterに対応するCR一つへ変更し、空入力やnative所有権
変化を成功としないdriver回帰を追加しました。初回の失敗記録は保持します。

修正後の`9767fd93ad16f9ee20ea9b2eb1394c47ca68fbbb`はtest 34734033900、
Ubuntu 34734033821、Incus 34734033816、
[Windows 34734033825](https://github.com/SLktEx/Hacocoon/actions/runs/34734033825)でPASSです。
Windows job 103662066493では通常Windows／WSL／HostのConPTY入力編集
（`RUN-INPUT:abD`）、43x132へのリサイズ、exit 17、端末とcatalogの復元、
providerリソース一覧が不変であることを確認しました。同じ試験経路の再実行で初回の
入力失敗を解消しています。日本語Windows、新GUI／toast回答、VPN／NRPTの確認は含みません。

## PTY試験の同期

個別ヘルプ候補`c7169760838e4cce24443ae9c27d4b1c21afadb1`（PR #592）は
Windows 34734829163、Ubuntu 34734829173、Incus 34734829154でPASSでした。
test run 34734829186のGo 1.26 job 103664252026では
`TestSizedInteractivePTYReadlineResizeAndExit`がFAILしました。同runのGo 1.27／raceの
成功で相殺しません。同じ失敗はローカルGo 1.27の100回反復でも再現しました。

使い捨てのサイズ観測とシステム呼び出し記録から、代役Bashが`TIOCGWINSZ`で24x80を読み、
transportが17x37へ変更した後、次のプロンプトへ戻るBashが先ほど読んだ24x80を
`TIOCSWINSZ`で書き戻す競合を確認しました。この試験ではBashとtransportが同じPTYを
使用し、実際のIncusとguestは別のPTYを使います。トレース実行は原因取得後に明示的に
終了しており、受け入れ成功ではありません。

最初のプロンプト待機だけの修正もFAILしました。Bashのプロンプトが別のstderrパイプから
届き、先行するPTY stdoutを追い越すためです。長い反復は診断のため明示的に停止しました。
試験はBashのstderrも実guestと同じPTYへ流し、直前のコマンド結果の後に
新しい入力待ちプロンプト全体を待ちます。
記録位置を指定するため、以前のプロンプトで待機を終えません。実際の複数行編集、
正確なサイズ、SIGWINCH、exit 17、最終出力、サイズ不正拒否、切断確認は維持します。
製品の端末コード・期限・隔離は変更しません。最終版はGo 1.26.8と1.27.1の両方で
PTYの3試験を各100回、race付きで各10回実行してPASSしました。Incus package全体と
文書検査もPASSです。一般の導入済み長文入力まで確認済みとは扱いません。

## ローカルGUI候補

`e7ba798728dcbe48a5179845673a333f8ff8968f`（PR #588）はtest 34727370959、
Ubuntu installer 34727370966、Incus 7 34727370817、Windows installer
34727370876がすべてPASSです。Windows job 103643786611には
`VS CODE LOCAL APPROVAL WEBVIEW / REAL RENDERER HANDSHAKE / INSTALLED CONTROLLER STALE REFUSAL: PASS`
があります。配布相当のローカル画面、実rendererのready、導入済みcontrollerを通した
古い要求の拒否の証拠です。通常SSH・interop・installer・再起動・reclaimもPASSです。
人間のtoast click／新GUI回答とVPN／NRPTは明示 **SKIP** です。日本語Windowsは未確認、
既存ローカルWSLInteropの失敗は未解決です。開発ブランチの証拠であり、配布済みや
後続run所有権修正の実機確認とは扱いません。

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

<a id="development-branch-integration"></a>

## 統合した開発ブランチの検証証拠

以下は各開発コミットで記録された結果であり、統合後のmainを検証した結果ではありません。
統合によって確認範囲を広げません。

| 対象 | 成功・失敗と残る制約 |
|---|---|
| `58c4a56` / `dev/1.x` | test/vet・race・模擬E2E・systemd・隔離した転送試験・固定AWS SDKの15試験は成功。全工程CIはUbuntu 24.04上でinstallerの26.04以降という条件により停止し、条件は緩和していない。Incus 6.0.0の観測・削除と正確な後始末を確認。rootfs importはamd64メタデータで一度失敗し、回帰試験で再現後、修正したBtrfs集約試験が64.20秒で成功。公開export/import、snapshot/copy、新規生成ID、Git/Workspace/OCIのデータ保持を確認し、失敗・成功fixtureを正確な所有記録で削除。導入済みcontroller import、SSH接続、稼働OCI、Windows導入はこの試験では未確認。 |
| `6cf9295` / `dev/v2` | 専用Ubuntu 26.04/Incus 6.0.5へのローカルビルド導入でsetup・Host doctor全6項目、外部Workspace作成・Linux SSH編集/build・停止再開・重複拒否・空選択キャンセル・ファイルを残すEnv削除が成功。模擬customizationのexit 29は秘密出力を漏らさず工程・理由・request IDを表示。観測中断後も処理完了まで排他を保持。初回SSHはdefault denyとsshd不足で失敗し、限定した4つのパッケージ規則で準備後、その規則を削除。専用network namespaceとAppArmor無効のkernelでの確認であり、既定ネットワークやAppArmor隔離の検証ではない。Windows IDE/既定接続、非公開Git/registry、OCI保持、cold restartは未確認。 |
| `ae19db6` / `dev/v2` | test/vet/JS、race、模擬E2E、interop 22試験とWindows installer構成要素試験が成功。installer変更処理は模擬化し、読取りtransportだけ対象WSLに固定。Linux PowerShellはSystemDirectoryが空でWindows専用fixtureを実行できなかった。実際の導入を証明する結果ではない。 |
| `72058fc` / `dev/2.x` | 専用Incus/Btrfsで合成外部IPv4/IPv6・Physical Host・Env間のTCP/UDP、Hostからの転送が成功（各経路0.099〜0.169秒）。期限、失効、ポリシー期限、生成ID置換の拒否、DNS固定を確認。初回DNS fixtureはcontroller準備前に失敗し、読取りの準備待ちで順序を修正。公開Internet・企業VPN・本番サービスの確認ではない。 |
| `ac67fad` / `dev/2.x` | 導入済みCLI/Incusで2リポジトリの準備・再開、SSH編集、再作成後のファイル/Store保持、独立fork、OCIなし、Store明示再利用、Base交換が成功。宛先OCI衝突は不完全な所有記録と元snapshot予約を保持して再開を拒否し、既存Storeは不変。専用ファイル・明示したWSL/namespace経路でWindows SSHと転送したブラウザー表示は成功。自動open、VS Code UI、既定installerネットワークは未確認。Windows TCP/UDPサービスはWindows内から成功したがWSL/controllerからはtimeout。ゲストTCPはconnect/failed/timeoutを記録し、UDP応答なし。Windowsサービスへの外向き通信と失敗原因は未確認。 |

小さいWorkspace fixtureの時間/Btrfsプール増分は、準備0.556秒/126,976バイト、
open 7.064秒/25,333,760バイト、再open 0.793秒/147,456バイト、fork 1.128秒/458,752バイト、
fork open 6.384秒/23,162,880バイト、再作成3.396秒/23,650,304バイトでした。
Base交換は17.986秒で容量未計測。各sourceのextentは12,075,008バイト、準備したコピーの
exclusive extentは0バイトでした。プール増分はmetadata・runtimeの活動を含み、
Linux kernel規模の性能や負荷を統制したbenchmarkを示しません。

統合候補`215019a`ではdocs/workflow-policy、全Go test/vet、JavaScript 27試験、
全race、模擬E2E、systemd検証が成功しました。変更操作を模擬化したWindows installer
構成要素試験も成功。全工程のローカルCIは検証HostがUbuntu 24.04のためinstallerの
26.04以降という条件で停止しました。転送試験は非対話sudoが利用できず一度停止し、
同じkernel回帰試験をrootの専用network namespaceで実行して3.25秒で成功しました。
これらは統合候補の導入済みIncus・Windows/WSL製品経路・非公開registry・稼働OCIの
実機確認を意味しません。

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


## push中断後の照合候補

`codex/git-push-reconciliation`の実装
`42aa706fd2fec31f1c3f565337e246aafc12f752`は、Issue #470の保存記録確認と
読み取りだけの照合を追加します。最終ソースは独立したLinuxコピーで
`bash tools/ci-local.sh test`と文書検査がPASSです。関連packageはGo 1.26.8でもPASS。
そちらは最後のcontroller往復fixture追加前で、製品コードは同じです。

実際のローカルGitで、リモート変更後の応答喪失、broker再起動、同一commitの競合作成、
現在の新旧commit・不存在・別commitの照合、新しい読み取り拒否、実行中の照合拒否、
承認待ち中のEnv世代変更、取得元所有者変更、送信前・送信確認・観測記録の保存失敗を確認しました。
破損・重複・未完了の記録は拒否します。pushは再送せず、OIDが一致しても元の未確認状態を保持します。
controllerの経路と日英表示・JSONも回帰対象です。

初回の集中試験は既存の集合fixtureにEnv世代と必須監査sinkがなくFAILとなり、
共通serviceと正確な世代を持つfixtureへ更新しました。controller往復fixture追加後の
初回全体試験は、通信上のエラーをCoreのsentinelと比較してFAIL。既存の
`recovery_required`通信コードを検査するよう修正し、最終全体再試験がPASSです。
製品の権限・エラー契約を緩和して解決していません。
外部認証Git、新しい導入済みHost agent操作、通常Windows/WSLからの新コマンド利用は
**未実施**であり、成功ではありません。過去のGit受入や先行native CIで代替しません。

## Windows通知内承認の開発候補

実装 `667ae5bf236aeff91a4bb711e07258652e3030bb`、ブランチ`codex/windows-toast-approval`は、
コンソール表示を通知内ページ・選択欄・非表示COM helperへ置き換えます。共通の非公開確認・
Policy・監査を再利用します。開発実装であり、main反映・配布済み・Issue #568受入完了ではありません。

この実装commitの正確なarchiveを独立したLinuxコピーへ展開し、標準の
`bash tools/ci-local.sh test`がPASSです。最終文書検査もPASSです。以下の実機残件とは分けて扱います。

Go 1.26.8/1.27.1の集中回帰と関連raceで、保存範囲の全ページ確認、要求ごとの独立した選択、
古い・変更済み・期限切れ要求の拒否、Show失敗、上限付き不正出力の拒否、一度だけの回答、
不明結果の再送禁止がPASSです。Windows Go 1.26.8実行試験では、実COMの所属先照合、
読み取り専用表示の応答、入力検証、子プロセス停止・回収、native診断の秘密情報保護がPASSです。
Windows通知APIでも、英日ToastGeneric選択XMLの履歴と所有通知の削除がPASSです。
providerを実行しないfixtureであり、人の承認を模擬して導入済み受け入れとは扱いません。

Windows amd64/arm64ビルドはGUI subsystem 2です。arm64の実行は未実施です。
PowerShell 7の登録試験は、固定COM識別子・起動先、正確な所有状態からの再開、再実行、
別所有者と異なるactivatorの拒否、テスト資源の回収がPASSです。追加のPowerShell 5.1
`-File`登録試験は、このPCのscript policyで実行前に拒否され、試験自体は**未実施**です。
実行ポリシーは緩和していません。native描画自体は製品と同じWindows PowerShell 5.1の
固定encoded commandとstdin上のJSONで実際に実行しました。

最初の通知表示は、Show段階の通知設定比較で**FAIL**となりました（HRESULT `-2146233087`）。
WinRTの設定値を数値で比較するよう修正し、通知設定を変更せず最終の英日履歴・削除がPASSです。
初回Windowsビルドの待機状態型不一致と、Windows vetの整数からのポインタ変換指摘も修正し、
型付きCOM引数による実ABI試験・静的検査がPASSです。

computer-useはkernel assetsのパス不在で2回初期化に失敗しました。**見切れ、導入済み新規要求への
通知内回答、複数クライアントでの同時回答、新規要求への人のVS Code回答は未確認**です。
履歴・COMコールバック・古い要求拒否では、これらの残件を完了扱いにしません。

過去の`4bb8dad`／Windows run `34176272125`は、使用できない`Get-FileHash`への依存で
デスクトップ受け入れ前にFAILでした。後続の.NET hash実装と構成要素回帰で依存を解消しましたが、
失敗runの後続SKIPを成功へ変えません。親`ac2b81dec811bf956d309b32a40d7dd1efe308e3`（PR #598）は
test `34738580505`、Ubuntu `34738580518`、Incus `34738580490`、Windows `34738580548`がPASSです。
親の証拠であり、今回の新しい通知UIの受け入れとは区別します。

PR #611のhead `f31ce3f7`ではtest `34741502449`、Ubuntu `34741502448`、
Incus `34741502443`がPASSでした。Windows `34741502440`（job `103681856689`）は
native client構成要素、配布物導入・再起動・再導入、HTTPS／直接egress拒否、通常Windows
SSH／interop、一時TTY、Linux trim、公開reclaimと保持Workspace／OCI／snapshot復元がPASS。
VHDX割当は7,897,874,432 → 3,969,908,736 bytesでした。最後の通知reviewは**FAIL**です。
COM登録・所有状態からの再開は成功しましたが、最初の導入済み古い要求probeが期待した拒否と
一致しませんでした。logには固定分類された実応答がなく、原因は未確定です。後続の不正入力・
別所有者・購読の確認は完了していません。人による新規回答も未確認のままです。

<a id="main-sync-candidate"></a>

## ロードマップ候補へのmain統合

`codex/roadmap-main-sync`は#611の`f31ce3f7`とmainの`74bc2205`を合わせ、
#581／#597／#602／#604を含みます。mainの責務分割、制限付きIncus状態取得、削除全体の
完了判定を維持しました。schema 14と一時実行の世代照合を分割先へ移し、共通cleanupの
再試行でも同じidentityを使います。追加回帰は送信元保護の削除失敗で一時leaseとmarkerが
残り、別世代の再試行を拒否し、全cleanup完了後のみ解放できることを確認します。

独立Linuxコピーで主要package、標準`bash tools/ci-local.sh test`、Go 1.26.8の全Go試験、
関連race、文書整合がPASSでした。CLIは日英helpと明示的なJSONを維持し、人向け表示だけ
外部由来の制御文字をエスケープします。初回統合試験は重複したtest断片、identityを欠いた
旧marker fixture、JSONを既定とする旧assertionで失敗し、製品の検査を緩めず修正しました。
初回の全体ローカルCIは一時コピーがGitの実行bitを失ったためFAIL。記録された属性の復元後に
同じCI入口がPASSしました。これらは統合／コピーの失敗で、上の導入済みWindows失敗とは別です。

統合候補での新しい実Incus／Windows／WSL受入は未実施です。親の成功や取り込んだmainの
証拠では代替しません。元のdirtyなmain作業ツリーと既存の基盤resourceは変更していません。
M1の実機言語・SSH不足、新規GUI／外部認証Git受入、M3のDNS mode／VPN／client転送は残件です。

## native二重review拒否の後続修正

`codex/native-review-refusal`で、既存helperの「要求は終了済み」が失われることを
実Windows COM往復で再現しました。古い要求／wrapされた古い要求の新回帰は#616の旧callbackで
FAIL、専用の読み取り応答HRESULTでPASSです。controller障害は利用不能のままで、表示成功応答や
回答にはしません。native入力検証、非公開子プロセス停止・回収、厳密な設定、診断の秘密情報保護も
PASSです。この実行では通知表示・履歴は明示SKIPで、人による新規回答は検証していません。

共通loggerの固定項目で登録・所有権・COM受信・通知削除・peer起動・要求確認・event処理の失敗を
区別します。検証済み表示失敗は生出力を出さず型付きHRESULTを保持します。導入済みWindows probeの
不一致時も固定分類・終了値・期待文一致の真偽値だけを出します。#611の旧logではこのCOM分類が
失敗原因か判断できないため、導入済みの失敗は未解決です。native COM構成要素試験でWindows設定、
既存登録、基盤データを変更していません。
