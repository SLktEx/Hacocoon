# 検証証拠と未確認の範囲

[English](acceptance-evidence.md) | 日本語

状態: 検証記録。ここに記載した試験は過去のコミットで実施されたものです。文書整理時に実機試験を再実行したという意味ではありません。現在の機能は[実装状況](../IMPLEMENTATION_STATUS.ja.md)を参照してください。

成功・失敗・スキップは試験構成に結び付けて読みます。同じ実行内の一部成功や後続の成功だけで、別の失敗原因が解決したとは判断しません。日々の実行ログを追記するのではなく、判断を変える証拠と未解決事項だけを更新します。

<a id="portless-ssh"></a>

## ポート不要 SSH とエディターの cold reconnect

[PR #631](https://github.com/SLktEx/Hacocoon/pull/631) の
`e35a152e3d58fc917192d56e442517bd7805b8ff` で、
[Windows 実機試験](https://github.com/SLktEx/Hacocoon/actions/runs/34756266534)は、
実 Windows OpenSSH のコマンド実行、管理 alias/config、厳密な host key 確認、
4 本同時の cold reconnect、削除済み接続先の拒否に成功しました。
Environment と controller の停止、WSL 終了後の最初の接点は、ProxyCommand 経由の
`ssh.exe` でした。同じ provider generation と Workspace marker を保持し、
`ss -H -ltn` の Host TCP listener は増加せず、SSH 用 Incus proxy device も存在しません。
SSH metadata の Host は空、port は 0 でした。

別の cold cycle では、VS Code 標準の保存済み remote folder URI を最初に開きました。
VS Code 1.136.1 と Microsoft Remote-SSH 0.128.0 で、実 editor のファイル読み書き、
remote terminal の実行、ローカル承認経路の stale 拒否、probe cleanup に成功しました。
接続確立に Hacocoon 拡張は使わず、専用 UI observer は結果の検査だけを行います。
新しい host key を固定した Windows export/import SSH、保持データの再接続、preview、
public reclamation も成功しました。同候補の[通常 CI](https://github.com/SLktEx/Hacocoon/actions/runs/34756266527)、
[Ubuntu 導入](https://github.com/SLktEx/Hacocoon/actions/runs/34756266617)、
[実 Incus Core/Btrfs](https://github.com/SLktEx/Hacocoon/actions/runs/34756266512)も成功しています。

ただし Windows ジョブ全体は、意図的な WSL 終了で破棄された古い Host terminal に
driver が `exit` を書こうとして失敗しました。driver は cold 試験前に terminal を閉じ、
試験後に新しい terminal から通常入口を確認するよう修正します。元の失敗をジョブ全体の
成功として扱いません。初期 fixture の `/tmp` Workspace 消失は `/var/tmp` への変更で
解消しました。`d6059131` は cold SSH に成功しましたが、標準 Remote-SSH 拡張の導入漏れと
転送 fixture の旧 Host port 契約で失敗しています。
[run 34755298769](https://github.com/SLktEx/Hacocoon/actions/runs/34755298769)に記録を保持します。

リポジトリの回帰試験は、raw binary stdio/UDS、EOF/half-close、キャンセル、controller の
起動遅延・切断、古い identity/lease/grant の拒否、並行 resume、所有 entry の cleanup を
確認します。実 PC の電源再投入、Remote Explorer の手動クリック、VPN/NRPT、広範な IDE は
未検証です。private-registry ジョブは手動実行専用のため PR 実行では SKIP です。
公式 Base の初回 SSH setup の通信不要化は[Issue #603](https://github.com/SLktEx/Hacocoon/issues/603)の責務です。

<a id="incus-lts"></a>

## Incus 7.0 LTS対応基準

対応契約は`>= 7.0.1`, `< 7.1`です。以前の6.0.5での結果は過去の互換確認として保持します。
[PR #583](https://github.com/SLktEx/Hacocoon/pull/583)の開発候補
`0c79f8209eec42b597cc811a9114e0351d8226d7`で、
[Ubuntu導入](https://github.com/SLktEx/Hacocoon/actions/runs/34724986358)、
[Windows/WSL新規導入・再起動・再導入](https://github.com/SLktEx/Hacocoon/actions/runs/34724986361)、
[standalone/Core/BtrfsのIncus試験](https://github.com/SLktEx/Hacocoon/actions/runs/34724986357)が
server 7.0.1で成功しました。lifecycle、egress、Base build、snapshot/copy/import、
保持Store操作、所有資源のcleanupを含みます。
[リポジトリCI](https://github.com/SLktEx/Hacocoon/actions/runs/34724986411)も成功しました。
private registry、VPN/NRPT、人間の通知内回答は未確認です。

main向け#479では共通導入、doctorと必要なvendor daemon/export/fixture修正を切り出します。
上記の統合候補の成功は今回の切り出しの実機再実行や配布の証拠ではありません。
切り出し自体のパッケージ・実機CI結果は以下に記録します。

切り出し`9a4dc42`で[通常テスト](https://github.com/SLktEx/Hacocoon/actions/runs/34739589129)、
[Ubuntu](https://github.com/SLktEx/Hacocoon/actions/runs/34739589125)、
[実Incus](https://github.com/SLktEx/Hacocoon/actions/runs/34739589134)は成功しました。
[Windows](https://github.com/SLktEx/Hacocoon/actions/runs/34739589114)も新規導入・再起動・再導入、
egress、transfer、reclaimと保持データ復元は成功しましたが、desktop全体は承認操作と
後続preview setupで失敗しました。承認fixtureが端末必須のCLIへパイプで回答していたため、
専用PTYへ修正し、JSONの応答と時間制限付きの子プロセスcleanupを維持します。
mainの出力変更に合わせ、受入試験でJSONを読む呼び出しには`--json`を明示します。
修正後の切り出し`34ff371cedb7558959201b316a2aebe7f3542eee`
（[PR #600](https://github.com/SLktEx/Hacocoon/pull/600)）で、
[Windows/WSL](https://github.com/SLktEx/Hacocoon/actions/runs/34741178336)、
[Ubuntu](https://github.com/SLktEx/Hacocoon/actions/runs/34741178335)、
[Incus Core/Btrfs](https://github.com/SLktEx/Hacocoon/actions/runs/34741178370)が成功し、
Windowsのdesktop全体も成功しました。元の全体失敗は失敗として保持します。
[リポジトリCI](https://github.com/SLktEx/Hacocoon/actions/runs/34741178334)はGo 1.27ジョブだけの
再実行後に成功しました。初回は既存の対話PTYサイズ変更試験が時間切れになりましたが、
同じshuffle seedでのローカル30回はコード変更なしで成功し、間欠的な時間切れの原因は未確定です。
この結果は当該切り出しの試験範囲の証拠であり、リリース配布や後続main統合の合格を示しません。

main統合後の`d6f078e`の[Windows run 34742409841](https://github.com/SLktEx/Hacocoon/actions/runs/34742409841)は、
Incus 7.0.1の導入と初回Host診断に成功しましたが、WSLの終了・再起動直後の通常入口で
`Host setup is busy`と拒否されました。fixtureはその後時間切れとなり、後続のEnvironment・desktop試験は
スキップされました。shell準備は既存の期限内でcontroller setupの排他解放を待つよう修正し、
明示的setupの重複拒否と失敗recipeの復旧規則を維持します。構成要素・race試験で待機、キャンセル、
排他解放を確認しましたが、修正後の統合候補のWindows受入は別途必要です。

<a id="installation"></a>

## インストールとHost

| 候補・試験 | 結果と制約 |
|---|---|
| `fced264` / Host 標準ツール | WSL amd64 上の専用 Incus/Btrfs 構成で、Git/gh/containerd/nerdctl/BuildKit の新規導入、公開 BusyBox の pull/run、Dockerfile の build/run、サービス再起動、Host 停止・再開、再 setup 後のイメージ ID と BuildKit キャッシュ ID の保持、独立 Store でのオフライン実行と削除の独立性を確認。試験用 `haco-area-55b79c9d56f597f1` は所有確認付きの後片付けまで成功。先行試験で見つかったサービス準備待ち、システム D-Bus の起動待ち、systemd の引数展開の不具合は回帰修正済み。公開版 Windows インストーラー全工程、arm64 実機、認証付きレジストリ、独自の既存実行基盤の移行は未確認。 |
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

<a id="ci-reliability"></a>

## PR CI の信頼性に関する障害 (#615)

以下の過去の観測は [#615](https://github.com/SLktEx/Hacocoon/issues/615) に属する。
rerun の成功は障害の証拠であり、解決ではない。現在の routing と gate の意味は
[PR 検証契約](../reliability/ci-contracts.ja.md) が所有する。

| 候補 / 証拠 | 判明した事実と解決状況 |
|---|---|
| `f3ef57b3ea028e10e942418a8408edd89a94b605` / [attempt 1](https://github.com/SLktEx/Hacocoon/actions/runs/34740688741/attempts/1)、[attempt 2](https://github.com/SLktEx/Hacocoon/actions/runs/34740688741/attempts/2) | `test (1.26.x)` の `TestSizedInteractivePTYReadlineResizeAndExit` が端末サイズ更新のマーカー待ちで失敗し、同じ SHA の attempt 2 は成功した。readline による端末サイズ復元との競合を避けるため、foreground コマンド開始の観測後に resize する。Linux の sized-PTY 回帰3件はローカル Go 1.27.0 で100回反復成功した。hosted native acceptance の成功は意味しない。 |
| `69c85fb5214ba1a4a81c2c50cdec9d789c924315` / [storage attempt 2](https://github.com/SLktEx/Hacocoon/actions/runs/34738362521/job/103675967861) | `TestRealIncusResourceMaintenancePreparationE2E` が対話拒否を期待しながら pipe を渡し、shipped CLI は非端末の確認を exit 2 で正しく拒否した。fixture を実 Linux PTY に変更し、子プロセスの端末判定と読み取りの回帰を追加した。native maintenance の再検証は必要。 |
| `84062060e0ef465e73ec45b43b6ed785ce879d81` / [Windows job](https://github.com/SLktEx/Hacocoon/actions/runs/34740317809/job/103678816517) | terminate 後の通常 Host entry が `Host setup is busy` を返し、harness は期限まで待ち続けた。即時失敗への変更だけでは製品不具合は直らない。後続の login-bootstrap 修正と native 再起動の証拠は下記に記録する。 |

`7b4e2356d73a163b31e784a0a5b7400fed1a05cf` を基にした #615 候補 `4abadc16399dfdb1997351c7803fd76131cfdeed` では、
ローカル Linux 検証環境で全 Go test/vet、race、shipped command の fixture E2E、文書と workflow policy が成功した。
同環境では実 Incus と packaged Windows/WSL は未実施。commit に結び付く hosted 結果は別途記録する。

main を統合した `8c645317101e007d57c752f35ae0a95f637d81b5` では、Python 3.13.15 を使い composition/Incus/製品 CLI の関連テストと vet が成功した。初回はローカル Python 3.10 に `tomllib` がなく失敗したため、検証済みの別 runtime で新しい Host-tooling テストの前提を満たした。テストを弱める変更はない。固定 Go 1.26.7 でも sized-PTY と maintenance-terminal 回帰の100回反復が成功した。これらは repository/component の結果であり、installed native acceptance ではない。

候補 `8c645317101e007d57c752f35ae0a95f637d81b5` / [Windows job 103689222832](https://github.com/SLktEx/Hacocoon/actions/runs/34744299884/job/103689222832) で再起動後の busy を再現した。初回 install と通常入室は成功し、再起動後の入室は11.218秒で失敗した。reinstall と後続の SSH/IDE/network/reclamation/通知は未実施。WSL の実装から、PTY を持つ PAM login bootstrap による競合経路を特定し、[ADR 0066](../adr/0066-wsl-login-bootstrap-routing.md) に routing 修正と retry を採らない理由を記録した。修正後の再起動成功は、後続の受入失敗と分けて下記に記録する。

候補 `75007eccd3b6d4290e456b1e346031203dcef227` / [test run 34745868490](https://github.com/SLktEx/Hacocoon/actions/runs/34745868490) は古い run 34744299866 の終了を待っていた。本体 job が取消済みでも job-level の `always()` により古い証拠 job が runner 待ちに残り、concurrency 枠を保持していた。証拠 job を `!cancelled()` に変更し、依存 job の失敗・skip の検査を保ちつつ workflow 全体の取消を完了できるようにした。これは CI 実装の不具合であり、runner 障害の証明ではない。取消を妨げる条件への差し戻しは静的回帰検査で拒否する。

`7c73399bc36f2a6055c3f95d3c1f3671666481d5` の [repository checks](https://github.com/SLktEx/Hacocoon/actions/runs/34746556831) は Go 両系列、race、CLI E2E、両 architecture の build、release packaging、証拠 gate が成功した。[native Ubuntu 導入](https://github.com/SLktEx/Hacocoon/actions/runs/34746556876) も未改変 installer、追加した通常ユーザーの製品 CLI lifecycle／Workspace 保持、legacy journey、network isolation、証拠 gate が成功した。

Windows の [75007ec job](https://github.com/SLktEx/Hacocoon/actions/runs/34745868528/job/103693588946) と [7c73399 job](https://github.com/SLktEx/Hacocoon/actions/runs/34746556856/job/103695440904) は、ともに install、terminate/restart、reinstall、installed egress が成功した。再起動後の入室は33.547秒と35.844秒だった。native interop、Windows SSH、VS Code Remote も成功したが、設定と承認待ちの fixture で両 job とも**失敗**した。標準の人向け表示を `--json` なしで解析していたため、設定の取得・適用と承認一覧に JSON 指定を追加し、実行可能な fixture 回帰検査を設けた。後続の reclamation と通知は未実施。これは再起動復旧の証拠であり、Windows 全受入や同一 SHA の再実行成功を意味しない。

同じ `7c73399` 候補の [native Incus](https://github.com/SLktEx/Hacocoon/actions/runs/34746556850) は standalone と Core lifecycle／egress が成功したが、Btrfs の aggregate export と source 削除 fixture が失敗した。Incus 7 は adapter が保持する既存の匿名出力に `--force` を要求する。main に入った #600 の実装が対応する 7.0 LTS 向けにこのフラグを渡すため、別の互換 shim を作らず再利用する。source snapshot の確認にも volume と snapshot を別引数で渡す修正が必要だった。cleanup は失敗 fixture を拒否した後にも pool／project 削除へ進んでいたため、所有権や不存在が不明な時点で後続削除を止めるようにした。native 再検証は別途必要。

main `f590023` を統合した候補 `fb5da79768c3fac5bf69db3c0496f936e9e1646f` では、ローカルの workflow policy、Actionlint、docs、製品 CLI／composition／Incus の test と vet が成功した。JSON／対話 fixture 8 件と cleanup テスト 5 件も成功した。新しい LTS 導入 fixture は検証 Host が Ubuntu 22.04 のため 1 件失敗し、対応する >=26.04 のガードは回避していない。

hosted の初回実行 4 件（[test](https://github.com/SLktEx/Hacocoon/actions/runs/34748814241)、[Incus](https://github.com/SLktEx/Hacocoon/actions/runs/34748814235)、[Ubuntu](https://github.com/SLktEx/Hacocoon/actions/runs/34748814274)、[Windows](https://github.com/SLktEx/Hacocoon/actions/runs/34748814262)）は job が作成されず `startup_failure` で終了した。test run の annotation は GitHub の予期しないエラーを示し、request ID は `CFDF:38DCEF:B99783:11CB2DC:6AA66658`。確認時の公開 status ページに障害告知はなく、全体障害や復旧とは推定しない。native 製品受入の成功ではない。この事象で job のない開始失敗を履歴 reader が見落とす問題が分かり、workflow attempt の結果を job と独立に保持し、開始失敗後に成功する attempt の回帰検査を追加した。rerun は依頼していない。

既存の GitHub connector で取得した有効な [Protect main ruleset](https://github.com/SLktEx/Hacocoon/rules/21838612) は docs、workflow-policy、release-config、Go 両系列、race、e2e を必須としていた。追加した evidence 4 件は確認時に未指定だった。その追加は別途必要な設定作業であり、ruleset は変更していない。

`def11e9ff31131e02be0eb3270bb3ebd62e1c452` の [repository checks](https://github.com/SLktEx/Hacocoon/actions/runs/34749383437) は `test-evidence` まで成功した。[Ubuntu 製品受入](https://github.com/SLktEx/Hacocoon/actions/runs/34749383422/job/103703362684) は成功したが、evidence gate は失敗した。artifact 10315158077 は必須 step の成功と `needs_success=true` を記録しながら、API の完了済み job 結果だけが null だった。上限付きの読み取り確認でこの反映差を待ち、終端の失敗を待ち直すことはしない。

[Core](https://github.com/SLktEx/Hacocoon/actions/runs/34749383438/job/103703364764) と [Btrfs](https://github.com/SLktEx/Hacocoon/actions/runs/34749383438/job/103703364605) は、aggregate transfer、Base build、Store COW、maintenance を含む製品 step がすべて成功した。両 job とも、現在の project を示す CSV の補足表示を厳密な識別子検査が拒否し、cleanup で失敗した。検証済み JSON の名前を使うよう修正し、不存在の確認は維持する。これらは製品 step の成功範囲が分かった失敗 job であり、native 全受入の成功ではない。

同じ `def11e9` 候補の [Windows user-path job](https://github.com/SLktEx/Hacocoon/actions/runs/34749383429/job/103703363209) は、維持している native journey 全体が成功した。packaged install、通常入室、terminate/restart、reinstall、installed egress、厳密な Windows SSH／VS Code interop、設定と承認、transfer、public reclamation、通知、cleanup を含む。今回の調査で初めての Windows 製品 job 全体の成功であり、失敗 SHA の rerun ではない。workflow evidence gate も成功した。後続の CI helper 修正は、この製品受入記録と区別する。

`d1c7480bd69157fb65974e9e2f2673e2ffffe4b6` では、repository、Ubuntu、Incus の全必須 job と evidence gate が成功した。[Windows job](https://github.com/SLktEx/Hacocoon/actions/runs/34750642440/job/103706445008) は public reclamation と Host 復帰の成功後、保持済み Workspace／OCI を再接続する `haco env create` が非ゼロ終了して失敗した。snapshot の復元と保持内容の確認は成功済みだった。fixture が stderr を破棄していたため原因は未解決であり、前の候補の成功でこの失敗を解決済みにはしない。通知は未到達。retention の診断は、数値の終了コード、許可リスト内の CLI reason、上限付きの読み取り専用 controller 観測を残し、raw output や元の操作の再実行は行わない。これらの観測は調査の境界を示すもので、原因を確定するものではない。

<a id="main-cli-language"></a>
## main向け日常操作の言語対応候補

`codex/main-daily-ux`はmain `ed3ad1a5`に#580/#583の日英表示・ヘルプを再利用します。
最初のローカル限定検証では、承認一覧を`--json`なしでJSON解析する旧テストと、
Openヘルプを新しいstdout経路ではなくstderrから読む旧テストが失敗しました。
テストを現行の公開仕様へ合わせ、製品の出力・承認動作は後退させていません。
修正後の固定したローカルソースで、製品CLI・Host・辞書・承認の限定テスト（2.60秒）、
維持している`bash tools/ci-local.sh test`（全Go test/vetとPython/client検査、13.28秒）、
関連raceテスト（9.93秒）、文書チェックとその回帰（4.73秒）が成功しました。
現行の全Goソースは検証済みアーカイブと一致しています。この候補のCI・導入済み環境での
確認は実行待ちであり、実装とローカル検証の記録です。配布済みとは扱いません。

実際のCLIを起動するE2Eは、旧形式の1行のEnvironment usageを期待して初回失敗しました。
#583の既存修正`195172f4`を再利用し、縦型の見出し・コマンド・create項目、明示helpとの
一致、終了コードとstdout/stderrを検証します。修正後のCLI E2Eは5.16秒で成功しました。
導入済みIncus側の同じ確認も更新しましたが、新たな実Incus実行の成功は主張しません。

quality実行34883913571はcoverage成功、出力書き込み結果9件の未確認でlint失敗でした。
既に失敗が確定した診断とヘルプの表示について、戻り値を意図的に破棄する箇所を明示しました。
出力先が閉じてもヘルプから操作実行へ進めず、既存の終了・判定動作を維持します。
通常のテストCIの成功とは別の結果として記録します。

先行する#583の`0c79f820`では通常・Ubuntu・Incus・WindowsのCIが成功しています
（[Windows実行](https://github.com/SLktEx/Hacocoon/actions/runs/34724986361)）。
この開発ブランチの実績は今回のmain統合、日本語Windows、人によるGUI回答、
長文入力・サイズ変更、元のSSH失敗経路の成功証拠ではありません。
性能測定とM2〜M5の受け入れは別の残件です。

`2eb2e2f5` ではリポジトリ検査が成功しましたが、品質検査 34885076668 は
日本語化した診断出力の戻り値未処理を追加で検出しました。Ubuntu 検査
34885076467 はインストールを完了した後、古い横並びヘルプの期待値で失敗し、
後続の利用確認は未実施です。この失敗と後続の SKIP は区別して保持します。
変更した出力処理の既存結果を明示的に維持し、Ubuntu の期待値を配布 CLI E2E と
同じ縦ヘルプの見出し・コマンド・create 案内へ揃えました。ローカルの関連テスト
（2.58 秒）、CI と同じ golangci-lint 2.13.2 で変更範囲の指摘数を制限しない検査
（6.62 秒）、workflow policy（1.05 秒）、CLI E2E（3.06 秒）は成功しました。
この修正を含む実機の利用確認は保留です。

PR #660の最終head `24cd508369ca5937495b378f69cec4414b9e8cfd`では、通常・品質・
Ubuntu・Incus・WindowsのCIがすべて成功しました（実行34886106686、34886106919、
34886106764、34886106700、34886106868）。mainへ`7e876bc1e5e92432971427d3c778a4f5a07cb72e`
として反映済みです。以前のheadの失敗は記録を保持します。後続のWindows言語引き継ぎや
人によるGUI回答の証明には読み替えません。

<a id="main-notification-installer"></a>
## main向け通知・インストーラ統合候補

#583の通知集約とBAT最終結果表示を再利用しています。Go 1.27.1の通知・辞書・イベント
テスト（1.22秒）、関連race（3.93秒）、通常のローカルCI（13.11秒）、文書とその回帰
（4.79秒）が成功しました。100件の別リクエストの失敗、再起動後の再開位置、別対象の
独立通知、承認・回復要求を集約しない動作を確認します。

PowerShell 5.1のテストスクリプトは実行ポリシーRestrictedにより起動できませんでした。
テスト開始前の実行失敗であり、製品の失敗や成功ではありません。実行ポリシーは変更せず、
同じネイティブfixtureのソースと製品BATを直接起動しました。0/1/37/3010、PowerShell欠落、
隣接スクリプト欠落の各確認が成功しました。既存のpywinpty 3.0.2 ConPTY fixtureも、
最終待ち・明示的なキー入力・終了37保持が成功しました。隔離したローカルPython環境と
一時的なネイティブ代替プログラムを使い、WSLへのインストールは実施していません。
PowerShell wrapper独自の追加の共有ディレクトリcleanupテストは、この経路では再実行していません。
新たなExplorer操作、パッケージ全体のインストール、元のSSH失敗の受け入れは残ります。

<a id="main-host-language"></a>
## main向けWindows表示言語の引き継ぎ

`45b53f98bb3b24942011ddd7b0ff73474a8b65f5` は#583の`3b8eefce`、
`0c79f820`、`8c07e126`をM1候補へ再利用し、明示的な`LC_ALL`/`LC_MESSAGES`が
Windowsの自動判定より優先されるようにしました。製品・Host・辞書・通信・Incusの
関連テスト（15.05秒）、通常のローカル全体テスト（14.38秒）、辞書・通信のrace
（7.57秒）が成功しました。最初の品質検査はテスト内の接続終了の戻り値未確認で
失敗しました。修正後の通信テスト（2.79秒）と、新規ファイルを含めた固定版
 golangci-lint 2.13.2（15.27秒）、文書検査と回帰（6.21秒）は成功しました。

同じPCの`hacocoon-second`から実際のシステムPowerShellを呼ぶ
`TestNativeWindowsLanguageReadOnly`は`ja`を返しました（テスト0.26秒、コマンド0.95秒）。
Windowsインストーラの部品テストはPowerShell 7.6.6で成功しました。どちらもOS/WSLの
言語設定や導入済みバイナリを変更していません。読み取り専用の実機取得と部品の確認であり、
新しい配布パッケージからの起動、全文の日本語化、通知への回答や人によるGUI確認は
#577とともに残ります。

main `7e876bc1`への取り込み後、照合済み候補で標準ローカル試験（23.78秒）、製品CLI E2E（4.54秒）、文書チェックと回帰（5.17秒）がPASS。通常のWindows入口の観測は、Hostが出す正規化済みの言語マーカー1件をWindows UI言語の独立した取得結果と比較します。観測処理の13試験もローカルでPASS。初回WSL起動は試験開始前に`HCS_E_CONNECTION_TIMEOUT`で失敗しました。後続の通常起動はWSL再起動なしで成功しています。起動に失敗した試行では製品試験を実行していません。

<a id="main-seed-retirement"></a>
## 旧バージョン互換を残さないSeed撤去

`85c4c621cd478e300f481d16eb7142a358ea34e3` は#655〜#657のSeed実行・harvest・
builder撤去をmain `9e5f6f68`へ再利用し、収集・推奨と旧イメージ削除・再有効化の状態も
撤去しました。2026-09-15にユーザーが旧版の互換性・移行を対象外としたため、読み取りや
変換用の処理は残しません。現行のDocker連携と管理対象Base・OCIの操作は維持します。

固定したローカル候補でCLI・構成・OCI・Incusの関連テスト（4.31秒）、通常の全体テスト・
vet・Python・client検査（12.49秒）、新規ファイルを含む固定版golangci-lint 2.13.2
（2.53秒）、workflow policy（1.04秒）、配布CLI E2E（2.95秒）、文書検査と回帰（4.64秒）が
成功しました。その後の状況文書だけの変更も文書検査が成功しました。この候補では新しい
実機インストールや性能測定は行っていません。#655の実Incus配置の成功は当時のSHAの
範囲限定の証拠であり、このheadの確認へ読み替えません。撤去した非公開レジストリjobは
Seed取得専用で、その他の実機jobと過去の失敗記録は維持します。ユーザーデータや
インストール環境の削除はしていません。

Seed撤去のGo実装を変えずにmain `7e876bc1`へ取り込みました。最初の検証起動は試験開始前に`HCS_E_CONNECTION_TIMEOUT`で失敗し、後続の通常WSL起動は再起動なしで成功しました。これは製品試験とは別の起動失敗です。main取り込み後の標準ローカル試験（18.42秒）と製品CLI E2E（4.70秒）はPASS。文書チェックと回帰（5.64秒）もPASSです。

PR #661がmain `44211fd2`として反映された後、両方の証拠を残してSeed候補を更新しました。照合済みの統合ソースは、全ローカル試験13.96秒、CLI E2E3.41秒、文書と回帰4.96秒がPASS。#661の最終headはWindows34890523779を含む全5ワークフロー成功。Seedの旧head `8eda58ee`もWindows34890531022を含む全5件成功ですが、新しい統合headの証拠には読み替えません。

<a id="main-git-branches"></a>
## main向けGitブランチ操作

main `7e876bc1`へ #585（`d93f61fb`）と #587（`7bdd3db6`）を再利用しました。
照合済み1,359ファイルの候補をGo 1.27.1で確認し、Git・共通review・capability・製品試験
（2.80秒）、CI固定版golangci-lint 2.13.2（新規ファイルを含め表示件数を制限せず、5.00秒）、
標準ローカル試験（10.98秒）、関連race（23.59秒）、製品CLI E2E（3.03秒）、
文書チェックと回帰（4.49秒）がPASSです。

実Gitのローカルfixtureは、複数head・個別ref拒否・移動/削除、新規branchの拒否、
承認commitの固定、作成/更新の保存範囲分離、main承認の維持、同名branchの競合作成
（同一/別commit）、force・削除・複数ref拒否を確認します。実Incusへの導入、
認証GitHub、人のGUI回答、巨大レポの受入とは扱いません。

最初の集中コマンドは存在しない`internal/approvalreview`を指定したためFAIL。
Git・capability・製品はその時点でもPASSし、指定を既存の`internal/review`へ修正しました。
最初のlint差分はWindows Gitの改行変換を無効にして未変更ファイルも含んだため、
その広い指摘を新規変更の指摘とは扱いません。正しい差分では再利用コードのエラー文字列2件と
switch簡略化1件が検出され、修正後に上記の最終検証がPASSしました。
先行するコマンド・lintのFAILは別記録で保持しています。

Seed撤去がmain `119e3007bc55333841a076f53d774be22ea5b711`へ反映された後、
Git実装を変えずにPR #663を更新しました。統合後の全ローカル試験15.80秒、
CLI E2E3.59秒、文書と回帰5.16秒がPASS。旧head `6b436e4d`の全5CI
（Windows34892114103を含む）成功は、新headの結果へ読み替えません。
Seed #662の最終head `50e692d6`もWindows34894991920を含む全5CIと、
同一commitの証拠job104155046690が成功しています。過去の失敗や人の確認待ちは保持します。

## 詳細案内とHostツール準備の一本化

候補 `4d7435cc` はmain `44211fd2`へ#592/#593と#659を再利用しました。
JSONの明示指定、ポート指定不要のSSH、HTTPプレビュー、Hostごとの保存手順適用を維持します。
照合済みソースで集中試験4.39秒、固定版lint4.09秒、全ローカル44.27秒、race9.69秒、
CLI E2E3.64秒、文書5.03秒、workflow-policy1.08秒がPASSです。

初回は古いfixtureの言語指定と、HTTPプレビューの--portまで禁じるSSHの検査が失敗しました。
共通言語選択を使い、プレビューとSSHを区別する検査へ修正しています。続くlintで見つかった
試験用ファイルの終了結果2件も確認するよう修正しました。新しい導入環境でのHost準備と元のSSH障害は
このheadでは未実施です。#655の元のHost apt失敗は過去の未解決の証拠として保持します。

PR #665のhead `e6ef0431`は通常・品質・Ubuntu・Incus CIがPASSですが、
Windows実行34896159890はpackage・導入前の最初の容量回収通信試験でFAIL
（job104150566559、start、30.09秒の期限切れ、stderrはCLIXMLのみ）。
他の5モードはPASS、その後の製品経路はSKIPです。変更したファイルはこの通信実装に
触れていません。同じ照合済みソースをこのWindows PCで構築・実行すると全6モードが
5.84秒（コマンド7.67秒）でPASSし、startは3.68秒でした。WSL再起動、実際の容量回収、
登録変更、実行ポリシー緩和はありません。CIの原因不明の期限切れは記録を保持します。
候補にはmain `119e3007`も取り込みました。

main `119e3007`取り込み後の案内・準備の統合候補`3c2d4c5c`は、全ローカル試験19.51秒、CLI E2E10.60秒、文書と回帰5.55秒がPASS。更新headのWindows CIを再実行しますが、以前の通信試験の期限切れは未解決の記録として残します。


PR #665のhead `108dd40cca4ea7bad0d0c8d1ddcc977a282d98aa`はWindows34900315650を含む全5CIがPASS。main `9da3ec8f`を`35d5ea81`へ統合後、全ローカル13.58秒、CLI E2E3.22秒、文書と回帰4.65秒がPASSしました。先行するnative protocol起動timeoutの原因は未解明のまま保持し、後の成功で消しません。新しい人のデスクトップ操作の受入とは扱いません。


<a id="main-interactive-run"></a>
## main向け一時実行の対話操作

M3候補はmainの責務分割を保って #590/#591 を再利用しました。`acd61022`では、
集中試験5.37秒、変更部分lint4.28秒、標準ローカル全体11.55秒、関連race10.71秒、
CLI E2E6.47秒、文書5.58秒、workflow policy1.17秒がPASS。実際のローカルPTYや
バイナリ入力の試験は部品の証拠で、導入済みWindows・Incusの受入とは区別します。

最初のWSL起動は試験開始前に`0x800705b4`で失敗し、後続は再起動せず起動できました。
初期試験はfixtureの作成ID欠落・古いschema固定で失敗し、現行形式へ修正しました。
移行用処理は追加していません。存在しないEnvironmentのfield参照によるコンパイル失敗も
修正し、作成IDはWorkspace leaseを正本としました。初回lintの未確認write/closeも修正後に
上記の検証が成功しています。先行する失敗は保持します。

現在はmain `119e3007`上のGit PR #663（`ddb03e85`）へ積み、対話実行をv0.61へ記録します。
mainには既に前景処理の開始を待つPTY resize回帰があるため、適用検査で競合した
#594の古い方法は適用していません。統合後の検証と導入実機の受入は別に記録します。
旧版移行や所有権を確認しないcleanupは追加していません。

統合後の`8b4d00a2`と標準ツールで更新したv0.61の候補は、集中4.37秒、
件数制限なしの変更部分lint2.53秒、全ローカル試験10.68秒、race8.51秒、
CLI E2E2.95秒、文書と回帰4.95秒、workflow policy1.02秒がすべてPASS。
Gitと一時実行の両方を含む結果です。新しい実Incus・Windowsの逐次実行の受入は
通常の環境確認として別に残ります。


PR #666のhead `7ed40fe53850747ebd06231c09f9df9fc4a87959`はtest、quality、
Ubuntu installer、実IncusがPASS。Incus run34900137450 / job104163854329で、
製品の一時実行を通したバイナリ入力、実PTYの編集・サイズ変更、終了17、端末復元、
中止時cleanup、Workspace保持が成功しました。既存snapshot/copy/transferもfixtureの
範囲内でPASS。これは実Incusの逐次実行の受入です。

Windows run34900137466 / job104163854107のstep19は公開reclaimでFAIL。
Linux側回収完了後、Windows停止を要求しましたが、`compact_attached`で接続中VHDを拒否。
圧縮・再開は未試行、native errorは未記録です。通知stepはSKIP。Windows回収の未解決失敗であり、
逐次実行の受入やworker修復とは扱いません。接続中ディスクの保護は緩めません。
人のGUI回答、Windows逐次実行、認証Git、巨大レポ性能は未確認です。

#663がmain `9da3ec8f`へ反映された後、#666を`18073ee7`へrebaseしました。
`7ed40fe5`と全ファイルが同一で、その後は本記録と状態の要約だけを更新しています。
既存のソース別成功は保持し、新headのCI成功へ読み替えません。


<a id="cache-generation-foundation"></a>
## キャッシュ世代管理の基盤

実装 `2a0e94990710bd3db9143f97d8fa5c4934b6a664`（v0.69）で共通元の排他的採用と、
未接続の`build-cache`領域を追加しました。Go 1.26.8のCore・state・保存領域・
責務検査・Incus回帰と、対象を絞ったrace 3回がPASSです。
最終検証コピーとこのcommitの1,474ファイルはbyte単位で一致しました。
文書整合性と18件の文書検査回帰もPASSです。

製品変更に対する標準ローカル試験（Go 1.27.1、shuffle 615）は、他packageがPASSした一方、
既存`TestLoginBootstrapPTYDoesNotStartHostSetup`のBash入力待ち表示でFAILしました（6.64秒）。
以前の失敗も未解決です。この実行の後続段階はSKIPでしたが、別実行で`go vet`、
クライアント構文、通知クライアント32試験、packaging 2試験がPASSしました。
分離した成功を全体CIの成功へ読み替えません。新しい明示実行用キャッシュ実機fixtureは、
この全体試験の後に追加して別途実行しました。

実Incus 6.0.5/Btrfsでは専用1 GiBプールとランダムな8 MiBの試験データを使用しました。
独立コピー2つの作成は3.837846024秒でした。変更前の`btrfs filesystem du --raw -s`は、
元と2コピーの各領域でtotal/shared各8,388,608 byte、exclusive 0を示しました。
内容一致、独立変更、現在世代の削除拒否、リセット・元削除後のコピー保持、
native snapshot参照による削除拒否、試験所有領域の回収がPASSです（全体16.45秒）。
小さな合成データのextent計測であり、プール全体の使用量や巨大レポ性能の証拠ではありません。

初回の実機試験は、daemon固有の保存領域マウント空間を試験プロセスから参照できず、公開前にFAILしました。
プール／プロジェクト`haco-cache-15c4cf3cbcded3c0`とカタログ
`/var/lib/haco-cache-generation-2844418008/state.json`に、作成途中の所有領域と第0世代を保持しています。
カタログの強制編集やcleanupの抜け道は使用していません。
成功した試行は同じ試験バイナリを既存daemonのマウント空間で実行し、隔離・承認設定を変更せず、
自分のプール／プロジェクト`haco-cache-969c95ea6a2e3bc9`を回収しました。
初回に保持した領域の回収・解決を意味しません。この権限付きfixtureは自分の領域だけへ試験データを書いて観測するもので、
通常Envからの収集や導入済みクライアントの受入ではありません。

Host指定パス、互換性による登録、Envの複数領域接続、停止時の自動収集、履歴・クリア操作、
巨大レポ実測は[キャッシュ契約](../design/cache-generations.ja.md)の残件です。

親のPacker PR #643（`80a687d0`）は、後続確認でtest34778540239・Ubuntu34778540205・
Incus34778540180がPASSです。Windows34778540191/job103781180868はstep13〜20がPASSし、
tunnel終了0も確認しましたが、通知step21は`stage=activation, reason=unavailable`でFAILしました。
native／子終了／経過時間は未記録です。新規の人の通知回答と実Packerの完走は未確認のままです。



## main向けキャッシュ共通処理の統合

対話実行`7ed40fe5`へ`2a0e9499`・`c4b7af50`・`094cc930`・`3a6e2bbc`・`2b3a5b56`を再利用し、責務分割と一時実行の正確な所有識別子を保持しました。旧版移行は追加していません。初回集中試験で正規化時の世代検査の統合漏れと旧schema受入fixtureを検出。検査を復元し、fixtureを現行所有データの保持確認へ整理後、破損拒否もPASSしました。

最終結果はGo1.27.1の集中12.86秒、件数制限なしの変更lint10.84秒、全ローカル22.15秒、関連race9.78秒、CLI E2E4.01秒、文書と回帰6.96秒、workflow policy1.34秒がPASS。先行lintの応答close・fixture write・条件式も修正後の結果です。上記の過去実測を新しい実機受入には読み替えず、曖昧な旧fixture poolは操作していません。公開設定、停止Envからの採用、履歴・クリア、追加データのsnapshot/copy/transferは未完成のため通常の適用は無効です。
main `ef443132`を`a0352044`へ統合した初回の全ローカルは52.96秒でFAIL。自動統合でrunのヘルプ項目3件が重複し、コンパイルとmilestone blackboxの構築が失敗しました。その試行の後続確認は未実施。同一内容の重複を削除した統合ソースは全ローカル57.89秒、CLI E2E8.06秒、文書と回帰9.86秒がPASSしました。先行Windowsのcompact_attached失敗は原因未解明として保持します。


main `5e89597a`を`3d8c2877`へ統合後、キャッシュ基盤の全ローカル13.62秒、CLI E2E3.27秒、文書と回帰4.86秒がPASS。先行head `22b119d8`はWindows34917359766を含む全5workflowがPASSしました。公開収集は別の追補で、この基盤だけでは登録を有効化しません。


<a id="ordinary-cache-collection"></a>
## 通常Envからのキャッシュ収集候補

#668の追補は`haco cache settings/configure/status/collect`、Host設定保存、停止した生成元の所有関係付き世代公開を追加します。
照合済みソースで全ローカル10.17秒、集中4.53秒、変更部分lint3.11秒、関連race6.02秒、CLI E2E2.73秒、文書と回帰4.82秒、workflow policy0.93秒がPASS。
先行の全体試行は集中PASS後にCLI出力結果16件の未確認でlintがFAILし、後続は未実施。出力失敗を終了値へ反映するよう修正しました。その後の表示だけの変更は別途確認します。

WSL `hacocoon-second`の実Incusでも通常のデータ配置・収集fixtureが15.35秒（コマンド18.12秒）でPASSしました。
既存Btrfsプール`haco-local-default`とUbuntu 26.04のキャッシュ済みイメージ
`b36d486c9412aee50d36c8875437070014bebd94d2207e0f703cd1b235c63033`を使用しました。
通常Env内で2領域へ書き込み、停止して収集し、生成元を削除した後、同じイメージを別のBase名で選ぶ新規Envが正しい内容の独立コピーを取得しました。
子領域の片付け・世代選択のreset・正確な共通元の片付け・Workspace内容の保持がPASS。nativeの配置変更と参照だけの再開は拒否し、client経由再開も確認しました。
権限緩和、proxyの抜け道、guestへのHost管理socket、カタログ手修正は使っていません。初回起動文だけはPowerShell構文エラーでWSL・試験に到達せず、起動文を直して実行しました。

これは名前付きのEnv本体内領域の機能確認で、異なるイメージ内容、導入済みCLI、人のWindows操作、巨大レポ性能の受入ではありません。
既存Envの登録、履歴・クリア・復旧コマンド、追加領域を含む転送は未完成です。以前の不明なプール`haco-cache-15c4cf3cbcded3c0`は操作していません。



互換条件・共有範囲・cleanup-requiredを含む最後の領域表示は、CLI/UI/controllerの集中2.61秒と文書・回帰4.74秒がPASSしました。上記で実行した実機の収集処理は変更していません。
