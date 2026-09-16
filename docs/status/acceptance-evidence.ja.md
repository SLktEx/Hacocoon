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
Git・capability・製品はその時点でもPASSし、指定を既存の`internal/policy/review`へ修正しました。
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


<a id="main-gui-approval"></a>
## main向け画面内承認

VS Code部分は #588（`e7ba7987`）をmain `7e876bc1`へ再利用しました。
desktop・共通review・control・製品の集中試験（2.67秒）と変更差分の
golangci-lint 2.13.2（4.49秒）はPASS。最初の全体確認はGoとrenderer/clientの32試験が
PASSした後、検証archiveの未コミット新規ファイルに日時0を付けたためVSIX作成2試験がFAIL。
Windowsのcheckout上でのパッケージ試験はPASSし、製品の作成条件は緩めていません。
検証コピーが元の日時を保持するように直し、標準ローカル試験（13.99秒）、
関連race（6.18秒）、CLI E2E（3.49秒）、文書と回帰（4.51秒）がPASSしました。
新しく導入した実Webviewや人の回答の受入ではありません。
#588の過去の受入を今回のmain統合の成功に読み替えません。

PR #664のhead `8c1cc435`は通常・品質・Ubuntu・IncusがPASSしましたが、
Windows実行34894187686の導入済み通知確認（job104143946090）はFAIL。
stage=clear、reason=timeout、子の終了1、8023ms、HRESULT未取得でした。
それ以前のインストール、厳格なSSH、2段階の容量回収はPASSです。原因は未確定で、
コンポーネント成功で消しません。後続修正は固定した進行段階だけを上限付きで観測し、
期限延長や履歴処理の省略はしていません。追加の検証結果は別に記録します。

main `119e3007`取り込み後の`ffb31f2b`は、集中2.48秒、lint3.49秒、全ローカル
10.53秒、race6.15秒、CLI E2E2.91秒、文書4.83秒、workflow policy1.04秒がPASS。
Windows試験・GUI形式の構築とvetに加え、実Windows review11.41秒、desktop0.42秒、
一時登録2.38秒もPASSです。段階の分割受信・無関係な出力・上限超過・子の失敗を確認し、
通常の初回clear・表示・履歴・削除も再確認しました。導入CIのclear期限切れは
更新候補での確認待ちとして保持します。

先行する検証用アーカイブはmerge commit完了中に取得したため、削除済みSeed fixtureが
混入し、そのsource guard不一致で全体試験がFAILしました。これは誤った検証ソースの
失敗として残し、統合候補の証拠には使いません。作成時のcommitを固定し途中の変更を
拒否するようにした新しい正確なアーカイブで、上記が成功しました。製品の保護や期限は
緩和していません。


#664 head `38dc1ffe`のWindows run34914309433 / job104208480202は導入・厳密SSH・公開reclaimまでPASSし、通知review step20でFAILしました。固定診断は`stage=activation, reason=unavailable`、HRESULT・child所要時間・renderer進捗は未観測。約10秒で、存在しない要求への期待された「承認待ちではない」拒否に到達しませんでした。これは先行clear timeoutとは別の未解決activation失敗で、以前の問題の修復とは扱いません。新しい人のGUI回答は未確認です。


起動診断の追補はCOMのinitialize/register/create/dispatchと数値HRESULTを記録し、読み取り専用の期限切れ・中止をCOM応答と非公開peer終了後も保持します。集中1.22秒、文書と回帰10.37秒、実Windows通知4.75秒、desktop0.44秒、専用登録2.81秒、Windows build/vet、PowerShell probe構文がPASS。最後の整形は空白のみです。導入済み起動失敗の修復とは主張せず、上記の失敗runを保持します。


main `ef443132`を`effc7801`へ統合後、GUI候補の全ローカル72.69秒、CLI E2E6.79秒、文書と回帰8.14秒がPASSしました。導入済み起動と先行clearの失敗は未解決で、更新headのWindows実行を待ちます。


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

#664のhead aef58798、Windows34918511743/job104221234323は導入・厳密SSH・Linux回収に成功。公開回収のHost再入場で02:08:10 UTCにstage=notification_setup reason=failedとなり、観測側が02:37:51まで待ってタイムアウトした。公開reclaim本体には到達せず、通知step20はSKIP。他4CIは成功。以前のclear/COM起動失敗とは別の失敗として保持する。後続は5a6fb54cの分類と入場失敗検出を再利用し、原因修復の成功とは主張しない。

通知準備のmain統合はGUI aef58798とmain5e89597aへ5a6fb54cを再利用。集中4.37秒、通知Python回帰0.69秒、変更範囲lint30.66秒、全ローカル117.79秒、race20.22秒、CLI11.78秒、文書17.46秒、workflow2.88秒がPASS。初回はmainにない将来のstream受入scriptのimportでFAIL。無関係なimportを除き、既存の通常入場回帰を保持した。Windows上のnative観測回帰6件も0.555秒でPASS。新しい導入済み通知の成功は主張しない。



main `5e89597a`を`3d8c2877`へ統合後、キャッシュ基盤の全ローカル13.62秒、CLI E2E3.27秒、文書と回帰4.86秒がPASS。先行head `22b119d8`はWindows34917359766を含む全5workflowがPASSしました。公開収集は別の追補で、この基盤だけでは登録を有効化しません。

現main5121b205をac145abcへ統合し、GUI/通知候補の全ローカル50.00秒、CLI12.86秒、文書・回帰31.34秒がPASS。aef58798の導入済み通知準備失敗は未解決で、新しい固定操作分類の証拠を待つ。

## Git push照合の統合確認

GUI・通知の8d509399へ既存42aa706fを再利用しました。Go 1.27.1で集中テスト9.31秒、変更コードのlint12.57秒、標準ローカル25.06秒、race28.69秒、CLI E2E4.44秒、文書・回帰8.66秒、workflow1.50秒が成功しました。最初のlintでは監査テストのClose結果未処理が見つかり、修正後に全項目が通りました。実Gitリポジトリと読み取り専用のリモート照合で、結果不明の送信記録やEnv作成実体の置換も確認しています。認証付きリモートGitと人による承認操作は未実施です。照合はpushを再送せず、現在のブランチ一致から元の送信成功を推定しません。

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

<a id="main-packer-builds"></a>
## main向けPacker HCL2によるBase作成

候補はmain `9da3ec8f`へ`1103505b`の実Packer・HCL2入力と外部スクリプト、任意adapter、共通Baseライフサイクル、失敗段階の非公開出力を再利用しました。Go1.27.1で集中31.91秒、変更部分lint38.45秒、標準ローカル全体45.78秒、関連race12.70秒、CLI E2E11.78秒、文書と回帰8.50秒、workflow policy1.88秒がPASS。初回lintのread handle終了未確認・エラー表現・switchを修正後の結果です。初回patchはmainにないヘルプcatalogで拒否されファイル未変更、現行の共通表示へ合わせました。

これらはPacker完走や導入済み受入ではありません。元候補の全体試験PTY timeoutと導入済みproxyによるUbuntu依存package取得のHTTP403は未解決の履歴として保持します。通常Envでのdownload/fmt/init/validate/build完走、Base公開・再利用、arm64、独自plugin失敗、Windows入口は未確認です。試験専用の権限付与やHost上のHCL実行は追加していません。既存の単純なJSON shell定義は現行機能で、旧版移行の要求ではありません。


#666の`629f33ed`を`068c8106`へ統合後、標準ツールで本候補をv0.62（Packer HCL2 Base builds）へ進めました。Packerと対話実行を含む全ローカル34.93秒、CLI E2E5.34秒、文書と回帰6.95秒がPASS。配布やM4全体の受入ではありません。
main `ef443132`を`a0352044`へ統合した初回の全ローカルは52.96秒でFAIL。自動統合でrunのヘルプ項目3件が重複し、コンパイルとmilestone blackboxの構築が失敗しました。その試行の後続確認は未実施。同一内容の重複を削除した統合ソースは全ローカル57.89秒、CLI E2E8.06秒、文書と回帰9.86秒がPASSしました。先行Windowsのcompact_attached失敗は原因未解明として保持します。


#667 head `61aeff8f`のWindows34916801756 / job104216088784は導入・HTTPS・Windows相互運用・Base作成・初回の厳密SSHと通常aliasまでPASSし、fixture WSL終了後の並列cold reconnectでexit255、ssh_progress=stream_deniedによりFAIL。後続reclaimと通知はSKIPです。原因は未解明で、Packerの実ビルド完走の証拠にはしません。他の4workflowはPASSしました。


main `5e89597a`を`075fc746`へ統合後、Packerと現行の詳細ヘルプを含む全ローカル13.24秒、CLI E2E3.18秒、文書と回帰4.79秒がPASS。最後の整形はヘルプ2ファイルの空白だけです。`61aeff8f`の並列cold SSH拒否は原因未解明として保持し、新CIの成功で消しません。

現main `5e89597a`、基盤 `929346bf`、Packer `6ad776ad` を `25923518` へ統合した v0.63 候補で、ローカル全体テスト（13.84秒）、CLI E2E（3.22秒）、文書・回帰確認（4.93秒）が成功した。公開キャッシュの詳細ヘルプとチェックポイントを含む統合確認であり、上記の実Incus確認範囲と残件は変わらない。

## 名前付きキャッシュ履歴とクリア

#669の後続は集中15.58秒、変更範囲lint4.08秒、全ローカル14.83秒、race6.28秒、CLI E2E3.66秒、文書・回帰5.57秒、workflow4.23秒がPASS。最初の集中試験は文字列readerを実際の非端末入力と誤認したテストでFAIL。実pipeを使うfixtureへ修正し、製品の確認条件は変更していない。

新しい実Incus保守確認はFAIL。TestRealIncusEnvironmentDataPlacementE2E、command251.40秒/test243.38秒、Env data-e2e-c78cf57de85ce050、catalog /var/lib/haco-data-placement-529644104/state.json。既存の再開・access確認で4分の期限に達しsignal: killedとなり、収集・新しい履歴/クリア確認には未到達。成功でもSKIPでもない。前の通常収集の成功はそのソースの範囲で保持し、実機クリアは未確認。正確な所有catalogで片付け結果を確認中。

読み取り確認で、失敗fixtureの正確なcatalogにEnv・lease・永続領域が残っておらず、nativeの名前照会も該当なしと確認した。空の世代項目2件だけが残る。元のタイムアウトは未解決。

main4cd0c7dcを611bedafへ統合し、Git/GUI/Packer/キャッシュを合わせた全ローカル14.30秒、CLI3.36秒、文書・回帰5.62秒が通りました。前のGUI8d509399のWindows34923857407はSSHと公開reclaimが成功し、通知clearは8024ms、progress=decodeで失敗しました。このPCの通常Windows権限で同じ固定処理を専用テストIDに実行すると0.53秒/0.23秒で成功しました。制限付き実行枠は通知処理前に拒否しました。CI失敗や人による回答の解決を示す結果ではありません。

dca688f6のWindows34926634572/job104245971260も最初のclearでtimeout、child_exit=1、8019ms、progress=decodeとなりました。直前のSSH/公開reclaimは成功しています。製品のPowerShell/WinRT初回起動上限を30秒にし、呼び出し側の短い期限と正確な子の中断は維持します。これは検証対象の起動上限修正であり、CI遅延の根因やGUI受入の成功を確認したという意味ではありません。

修正ソースのWindows通知テスト全体は、このPCで8.22秒でPASS。実際の日英ToastGeneric表示・履歴・削除（6.90秒）、activation callback、秘匿化、正確な子の中断を含みます。実機componentの証拠であり、人のクリック・見た目の受入・CIのcold起動結果は未確認です。

## 名前付きキャッシュ完了復旧

#670の後続はライフサイクル/キャッシュ/OCI/CLI/controller集中19.10秒、変更範囲lint15.71秒、全ローカル39.78秒、race14.61秒、CLI7.25秒、文書・回帰10.41秒、workflow2.00秒がPASS。実catalogと段階的provider失敗の試験で、完了記録からコピーし直さず復旧し、検査後に元データの保持を解除すること、リセット後の候補を保持し、不明・provider拒否・所有権違いは拒否することを確認。OCIの共通復旧も正確な所有参照を渡す。部品検証であり、新しいnative復旧やWindows受入ではない。先行の実機保守timeoutは未解決。

最後の復旧の使い方案内と日英ヘルプはCLI/UI/controller9.40秒、文書・回帰12.72秒がPASS。所有権・復旧コードは上記全体確認から変更していない。

復旧3ac51f91のquality34924782288はcache_maintenance.go:41のQF1003で失敗しました。test34924782199、Ubuntu34924782166、Incus34924782231は成功しました。分岐を同じ挙動のswitchへ整理しています。この失敗は先の変更差分lint成功と区別します。

分岐修正後、control/cache/workspaceの集中テスト11.23秒、現在mainに対する件数上限なしlint18.59秒、文書・回帰8.48秒が通りました。確認済み親1ee2962bと全ファイル一致するmain4cd0c7dcへ載せ替え、内容は変更していません。

main df22a1a5を2f07fa3dへ統合後、全ローカル95.04秒、CLI8.56秒、文書・回帰10.27秒がPASS。通知の実装は上記の実Windows確認から変更していません。

## クライアント転送のmain向け統合

キャッシュ復旧3ac51f91へ640c66ff/4b0b5baa/70037223/44a30b1a/bc8b915b/297b3095を再利用しています。最初の集中ビルドは既存SSHの準備・完了型と二重定義になり失敗し、44a30b1aの共通byte-stream処理へ統一しました。次は集中6.54秒が通り、lintは取り込んだコードの結果未処理73件と静的指摘8件で失敗しました。検証を弱めず修正し、Go1.27.1で集中3.76秒、lint4.64秒、全ローカル11.26秒、race7.74秒、CLI2.84秒、文書5.24秒、workflow0.98秒が成功しました。Windows amd64部品もstreamio0.27秒、clientforward1.09秒、wsllaunch0.34秒、native CLI0.69秒で成功しました。実pipe/TCP/半切断、親消失、対象変更拒否、ローカルhelpの確認であり、新しい導入済みWindows/Incusの成功ではありません。

PowerShell5.1のインストーラ部品コマンドはPCのスクリプト実行制限によりテスト前に拒否され、未実施です。実行ポリシーや隔離設定は緩和していません。既存297b3095は手元の中断回帰が成功し、導入済みCtrl+Cの再確認は保留だった点を維持します。最初のcheckpoint更新はLinuxからWindows worktree管理パスを解釈できず変更前に失敗しました。既存のWindows側ロックを取得して同じWSL更新ツールを動かすと成功し、ロックは迂回していません。

既存PowerShell7のインストーラ部品確認では、両実行ファイルの導入・再導入、所有記録欠落/不一致、checksum、使用中実行ファイルとjunctionの拒否が成功しました。PowerShell5.1は実行制限により未実施で、どちらの実行ポリシーも変更していません。#634の既存証拠では親#632のUbuntu34763684021/job103740815424が準備表示修正後の導入済みLinux転送8並列×2MiBで成功しています。再利用する#642は後のWindows Ctrl+C未確認を維持します。どちらも今回のmain候補の新受入へ読み替えません。

v0.64と導入案内の更新後も表示/ビルド情報2.81秒、architecture/checksum/破損archive梱包0.54秒、文書5.72秒、workflow1.11秒が通りました。Windows CIは実際の10ビルドを確認し、転送用成果物も必須にします。公開対象のamd64/arm64は維持します。

修正済み復旧c35f6610へ合わせたc682faaaは、集中4.22秒、main差分lint16.99秒、全ローカル104.53秒、race36.42秒、CLI18.56秒、文書19.60秒、workflow2.66秒が成功しました。その後#671は5つのCIに通りmaindf22a1a5へ反映されました。c35f6610と全ファイル一致するため、転送を11823e02として載せ替えても内容は変わっていません。

mainの転送2f995027を14b8fb9aへ統合後、Git/GUI/通知候補は全ローカル53.41秒、CLI8.88秒、文書・回帰19.00秒、workflow2.79秒がPASS。両方の非公開入口とWindows client試験を保持し、通知プロセス動作は実Windows8.22秒成功時から変更していません。

DNS選択候補: 最初の集中72.65秒がPASS。snapshot作成・import設定保持の回帰追加後は集中30.07秒、変更範囲lint23.44秒、全ローカル45.48秒、race14.76秒、CLI35.53秒、文書・回帰13.46秒、workflow5.91秒がPASS。最終の状態表示とnative adapter回帰追加は別途確認します。この部品検証は3モードの実Incus受入ではありません。

最終候補の集中確認は27.69秒でFAIL。英語statusの完全一致fixtureが、新しく表示する名前解決の行を含んでいませんでした。期待表示を更新し、既存の日英データ・escaping確認を保持します。その試行の後続確認は未実施です。

hacocoon-secondの実Incusでab98e0ccのTestRealIncusDNSModesE2EがPASS（コマンド84.63秒/テスト75.10秒）。host24.12秒、backend13.09秒、disabled37.89秒。新しく所有するEnvと現行companionで共通の作成・停止・再開・削除、loopback resolverとservice状態を確認し、backendは正確な所有関係のtooling adapter経由でexample.comの解決も確認しました。catalogは/var/lib/haco-dns-modes-905851288/state.json、-4084255079/state.json、-70566286/state.json。全Envの共通削除が成功。Policy許可・guest管理権限・別DNS fallbackは追加していません。native構成と再開・backend解決の証拠であり、guestからPolicyを通す全経路・VPN/NRPT変化・Windows再起動・GUI回答の受入ではありません。

期待するstatus表示を直した最新main上の最終ソースは、集中34.86秒、PR全差分lint52.03秒、全ローカル33.57秒、race20.05秒、CLI8.60秒、文書13.53秒、workflow2.47秒がPASS。以前の期待表示に合わせるための製品動作変更はしていません。


## 名前付きデータの保存とコピー

実装26939f1bは集中13.10秒、main差分lint11.93秒、全ローカル24.72秒、race10.61秒、CLI4.84秒、文書7.51秒、workflow1.36秒PASS。catalog回帰は再利用元の世代変更、全子領域の予約、検証前のコピー完了記録、保存元削除の排除、結果不明と削除失敗時の保持を確認。初期の集中確認は旧unsupported期待と保存元lease拒否の残りで失敗し、修正後focused-3が17.32秒PASS。SKIPにはしていない。

hacocoon-secondの実Incus、Btrfs pool haco-local-default、cached Ubuntu image b36d486c9412aee50d36c8875437070014bebd94d2207e0f703cd1b235c63033でdata-native-rootfs-1は52.97秒PASS（test46.52秒、build7.33秒）。fixture saved-data-9fa8a00cd2833843、catalog /var/lib/haco-saved-data-2533260138/state.json。rootfsの2領域と2レポWorkspaceで、稼働中保存と全領域付き再開、停止中独立copy、未収集の内容、新しい所有権、独立変更、元Env削除、複製の再開、共通の所有対象cleanupを確認。世代公開やPolicy許可は追加していない。性能・レポ配下配置の成功ではない。

先行data-native-1はbuild8.99秒成功後、試験compileで失敗。data-native-2は14.54秒（test7.17秒）で、レポ配下配置に必要なfile_storage_volumeが実機Incusにないため保存前に失敗。fixture saved-data-9cb00c708dc5ace0とcatalog /var/lib/haco-saved-data-1321951232/state.jsonを記録し、作成したWorkspace試験volumeは正確なcleanupのため保持。rootfs試験は対応済みの別配置であり、迂回やレポ配置の成功とは扱わない。

DNS #674 head a44c0cc5はLinux4CI成功、Windows34930836374/job104258506823は導入・HTTPS・SSH・Linux回収成功後、公開回収がcompact_attachedで失敗（open1回、圧縮未開始、未再開）。通知SKIP。Git/GUI #672 head7454efabもLinux4CI成功、Windows34931359363/job104260056333は並行cold SSH再接続でstream_denied/exit255。回収・通知SKIP。いずれもmain未マージであり、以前の成功や通知失敗と区別して保持する。DNSは98c0ddd2としてこの候補へ統合し、追加データ検査とDNS検査の重なりだけ競合解消した。

DNS統合とv0.66生成後の候補は集中22.16秒、main全差分lint17.84秒、全ローカル29.81秒、race11.90秒、CLI4.77秒、文書8.15秒、workflow1.50秒PASS。上記の実機追加データの証拠は変更していないライフサイクル/provider処理を対象とする。新しいWindows成功とは扱わない。


main9f5bc9e3（DNSと追加データ保存・コピー）をab77483fとして統合し、競合した検証記録は両側を保持した。Git/GUI統合後の全ローカル105.78秒、CLI7.58秒、文書10.98秒、workflow2.62秒PASS。以前のWindows並列cold SSH stream_deniedは原因未解決であり、ローカル成功を実機受入の代わりにしない。


## 呼び出しスレッドのネットワーク識別

実機名前空間回帰は従来の先頭スレッド参照で11.20秒FAILし、別スレッドの識別を検出した。呼び出しスレッド参照へ変更後、同じ権限で23.70秒PASS。集中13.84秒、main差分lint12.61秒、全ローカル33.40秒、race18.79秒、CLI4.29秒、文書8.59秒、workflow1.50秒PASS。Host名前空間拒否と専用スレッド破棄を維持する。欠陥と修正の証拠であり、以前のWindows stream_deniedとの因果は導入後再接続で別途確認する。

f68a8c6bから作成した通常WindowsインストーラがHacocoon-Roadmap-f68a8c6b（Ubuntu26.04.1/Incus7.0.1）で完了。storage・trusted Host・doctor DNS/HTTPS・Windows登録・通知登録を含み、試験用権限変更はない。対応版のrootfs追加データsnapshot/copy/export/importは76.26秒/test76.23秒PASS。fixture saved-data-c1685c13e42f7479、台帳 /var/lib/haco-saved-data-90071584/state.json。新しい所有権、独立変更、import後再開、共通cleanupを確認。人の通知回答・認証付きGit・巨大レポの成功とはしない。

別のリポジトリ配下配置は11.61秒/test11.56秒でcapture時にFAIL。capability staleとなりEnvは停止状態を保持した。fixture saved-data-ec2813c23b2ada60、台帳 /var/lib/haco-saved-data-3393557504/state.json。対応7.0.1で起きる製品側の不具合として調査し、以前の6.0.5のAPI不足と区別する。元Envのcleanupは実行済みで、保持Workspace fixtureの記録を残す。

## 名前付きEnvironmentデータの持ち出し

#675のfa6c1312を基にしたv0.67候補で、既存export/import形式と共通Environmentライフサイクルへ追加データを統合した。最終の固定ソースでfocused16.47秒、main差分全体lint15.45秒、ローカル全体27.39秒、race12.82秒、CLI4.59秒、文書・回帰8.38秒、workflow1.48秒がPASS。全データ、新しいローカル所有ID、import専用予約、配置の完全性、Workspace作成前の未対応provider拒否、cleanup不明時の所有保持を確認した。full-1はfixtureの一時stagingディレクトリが非公開でなくFAILし、0700へ厳格化して修正。full-2はfocused12.92秒PASS後、テストreaderのClose結果2件の未確認でlint FAILし、結果確認を追加した。製品の権限を緩めていない。

実機portability-native-1は46.36秒（build6.16秒/test41.23秒）でFAIL。import前に所有snapshot volumeのexportが利用できなかった。fixture saved-data-f7c512b0ca4392e9、台帳 /var/lib/haco-saved-data-2912432684/state.json、snapshot volume haco-snapshot-342b7e71e895c7a826c2d337efe90156。Envと保存処理は共通cleanupを使い、保持されたWorkspace fixtureは所有対象を確認して回収する。一括削除していない。

追跡でhacocoon-secondのIncus client/serverは6.0.5と判明した。既存exportが使う--forceと、リポジトリ配下の配置に必要なfile_storage_volumeがない。現行製品の対象はIncus7.0 LTS（>=7.0.1、<7.1）。以前のDNSおよびrootfsのsnapshot/copy成功は6.0.5での限定的な観測で、対応基盤の受入成功ではない。旧provider向けの回避処理は追加していない。対応版での新規インストール、実機持ち出し、リポジトリ内配置は確認待ち。巨大レポ・性能測定はユーザー指示で後続。既存Windowsのreclaim・並列SSH失敗も未解決のまま保持する。


## 現行データの退避前確認

現行schema16の追加データ、選択世代、import・コピー・cleanup未完了記録を台帳を書き換えず表示する。台帳参照の照合と実体の観測を分け、過去の生成元の不在を削除候補にしない。固定候補でLinux退避回帰78件0.96秒、文書・回帰34.39秒、workflow4.39秒PASS。Windowsでも探索実行PASSだがLinux専用29件は明示的SKIPで、Linux実行で別途確認した。現行schema対応を共通Go台帳と照合する回帰を追加。これは読み取り専用一覧の検証であり、全環境のbackup・復元・対応provider実機受入ではない。既存の転送実機・Windows失敗は未解決として保持する。


## 保存Envのリポジトリ内追加データ

対応7.0.1のsaved-data-ec2813c23b2ada60はsnapshot計画でcapability staleとなり11.61秒/test11.56秒FAIL。台帳 /var/lib/haco-saved-data-3393557504/state.json。通常のリポジトリ内配置はWorkspaceのstorageも固定するが、計画側は追加データだけのdigestを比べていた。作成・再開・importで使うruntimeの配置解決を共用し、完全な照合の受入とデータだけへの置換拒否の部品回帰を追加した。

f71ab282に基づく修正後固定候補で集中16.88秒、main全差分lint15.95秒、全ローカル27.33秒、race11.29秒、CLI4.21秒、文書7.90秒、workflow1.45秒、実機試験compile1.61秒PASS。新規Hacocoon-Roadmap-f68a8c6b（Ubuntu26.04.1、Incus7.0.1）でplacement-supported-native-1は55.61秒/test55.58秒PASS。fixture saved-data-054e30231401ddac、台帳 /var/lib/haco-saved-data-2319641996/state.json。2つのWorkspace、rootfsのcompilerデータ、/workspace/two/node_modulesを含む稼働中保存・再開、停止copy、export/import、未収集内容、新しい所有権、独立編集、元削除、import後再開、共通cleanupを確認。先行失敗fixtureと保持Workspace記録は別に残す。

対応版rootfs限定の持ち出しはf68a8c6bで76.26秒/test76.23秒PASS。通常Windowsインストールでdoctor・DNS/HTTPS・Windows登録・通知登録が完了し、hacocoon-secondは保持。現行schema一覧も保持中の6.0.5 fixture台帳と実体を2.89秒で読み取り照合でき、authority=falseを維持した。

通常導入後のPackerサンプルは依存導入で10.39秒FAILし、非公開診断付き再試行も6.56秒FAIL。UbuntuのHTTP取得に通常proxyが403を返した。公開configはdefault=deny・ruleなしであり、試験専用の許可を追加していない。実Packer実行は通常の通信設定待ち。#676 head f68a8c6bのWindows34935395589/job104272085316は導入・並列cold SSH・実VS Code編集・setup・preview・通常export/importに成功後、通常Windows tunnelでtimeout/connection resetとなりFAIL。回収・通知は未実施。過去の失敗原因も解決扱いにしない。

統合候補b42be03eは現main a3d0f4fd、Git/GUI #672、リポジトリ内配置 #679を含みます。focused21.73秒、main全差分lint23.92秒、ローカル全体41.13秒、race21.73秒、CLI5.39秒、docs/regressions11.17秒、workflow1.75秒が成功しました。実namespace・配置の検証済み実装は変更していません。#677はf71ab282で5ワークフローすべて成功し、a3d0f4fdとしてmainへ反映しました。#676は取り込み済みとして閉じましたが、同PRのWindowsトンネル失敗は上記の別結果として残します。

## Workspaceのレポ選択

6b376a62に基づく候補でfocused10.58秒、main全差分lint16.44秒、全ローカル25.61秒、race27.75秒、CLI2.86秒、docs/regressions8.59秒、workflow1.36秒、実機試験compile1.55秒が成功しました。
先行full-1も成功し、full-2では追加レポのコピー失敗時の参照保持と実機試験を追加しています。

Hacocoon-Roadmap-f68a8c6b（Ubuntu26.04.1/Incus7.0.1）のmembership-native-1は16.98秒/test16.95秒で成功しました。
fixture selection-c393aee4e83d、台帳 /var/lib/haco-selection-154075238/state.json。
試験用にローカルで作ったGitデータと実リポジトリbackend、共通snapshot/Env lifecycleを使い、未コミット変更・HEAD・indexの保持、通常のGit準備による登録済みレポ追加、選ばなかったレポの元側保持、独立編集、元Env削除、コピー先再開、所有対象のcleanupを確認しました。
Policyの変更やEnvへの管理権限追加はありません。OCIの選択は既存の関連データ処理の部品検証範囲です。この実機試験ではOCI・認証付きGit・公開CLI・巨大レポ性能は検証していません。

## Windows候補の未解決失敗

#678の6b376a62はquality・test・Ubuntu・IncusのCIが成功しました。
Windows34938847867/job104282639159は通常SSH/editor/tunnel、容量回収、保持データ復元に成功後、通知の失効要求起動がactivation/timeoutで失敗しました。
dispatchのHRESULT -2147220990はhelperの読み取り期限です。回収では割当7,864,320,000→5,041,553,408バイトを観測しています。
以前のtunnel・回収失敗の解決や人の承認回答の成功とはしません。

#680の2e8d905cもLinux側4ワークフローが成功しました。
Windows34940269831/job104287130520は導入・通常SSH/editor・Linux回収に成功後、公開reclaimがcompact_attachedで失敗しました。
実体openは1回、compact未実行、WSLは再開済みです。通知はSKIPです。この失敗結果で両PRをmainへマージしていません。

ローカルのHacocoon-Roadmap-f68a8c6bでも通知確認はreview timeoutで失敗しました。
ただし更新したのはWindows helperだけで、導入済みLinuxはf68a8c6bのまま、_desktop-reviewを実装していませんでした。
この混在候補は#678の受入ではなく、CIの別のactivation失敗の原因も確定しません。
次のローカル一貫確認は通常インストーラで両側の候補を揃えてから行います。

## 既存worktreeの独立取り込み

2e8d905cを基にした固定入力候補のfull-3でfocused36.42秒、main全差分lint22.84秒、全ローカル39.45秒、race31.22秒、CLI4.92秒、docs/regressions9.04秒、workflow1.71秒、実機試験build1.96秒が成功しました。
最後に追加したarchiveのtraversal・alias・xattr・特別権限・余分な末尾の回帰もrace3.01秒、main全差分lint19.81秒で成功しました。
CLIでは結果不明の記録保持、再送・置換拒否、未確認importのopen拒否を確認しました。
実ローカルGitの回帰はcheckout・linked worktree・split index・packed refs・stage/dirty保持・Host設定や管理情報の除外を含みます。
full-1はfocused26.38秒成功後にlint15件で停止し、修正しました。full-2も追加CLI・拒否回帰前の全項目が成功しています。

Hacocoon-Roadmap-f68a8c6b（Ubuntu26.04.1/Incus7.0.1）のinput-native-1は26.23秒/test26.19秒で成功しました。
fixture selection-384863c4ef38、台帳 /var/lib/haco-selection-1381113910/state.json。
実linked worktreeをprovider共通のcaptureと実Incus importで新しい管理volumeへ取り込み、HEAD・ファイル、guest独立編集、通常Env停止・再開、所有対象cleanupを確認しました。既存レポ選択試験も併せて成功しました。
製品実装は2e8d905cへ今回差分を重ねた候補であり、未変更の2e8d905cではありません。
専用試験binaryを使い、導入済みCLIの受入とは分けます。Policy緩和・Envへの管理権限追加はありません。
認証付きGit・人のGUI回答・導入済み入力・巨大レポ実測はこの実機試験では未実施です。


## 保持キャッシュ台帳の整理

809bfb33を基にした候補でfocused12.91秒、main全差分lint15.08秒、全ローカル26.56秒、race11.53秒、CLI4.17秒、docs/regressions7.95秒、workflow1.32秒、実機試験compile1.45秒が成功しました。
生成元不在、確認対象の正確な所有権、選択/表示対象変更、未完了/使用中cleanup、削除しない復旧、callerのpath/owner拒否、CLI共通確認と表示を回帰で検証しています。

専用WSLのUbuntu26.04.1/Incus7.0.1でcache-native-1は実行全体17.70秒、試験17.67秒でPASS。
fixture data-e2e-25cadfe2239e648b、台帳 /var/lib/haco-data-placement-24466710/state.json。
通常Env作成、2つのキャッシュ領域、停止収集、実データの独立再利用、使用先Envを残した元整理、両Env削除後の保持データに対する全体一覧/cleanupを確認しました。
外部Workspaceのmarkerは保持しました。試験専用台帳と共通所有権処理だけを使い、既存データやPolicy緩和は対象にしません。
実backendとStandard workflowの受入で、導入済み公開CLI・OCI・巨大レポ性能の受入ではありません。

#681の809bfb33はLinux側4 CIが成功しました。
Windows34943936799/job104298802478は導入・SSH/editor・tunnel・公開容量回収に成功後、失効通知起動のdispatchが10秒期限とHRESULT -2147220990でFAILしました。
通知後続や人の回答の成功は確立していません。起動準備は#682で対応し、この過去の失敗は保持します。


## 通知の起動準備

Hacocoon-Roadmap-f68a8c6bを通常インストーラで809bfb33へ揃えました。
最初の確認はPowerShell5.1のため、7が必要なfixture本体は未実施です。
PowerShell7のnative-review-matching-2はreview timeoutでFAILしました。
同じWSL起動・環境・非公開パイプによる読取probeは13.56秒で空のpending一覧を返し、以前の初回review上限10秒を超えました。
ローカルの起動待ち失敗の根拠であり、過去の全CI失敗の原因確定とはしません。

809bfb33を基にした準備確認候補はfocused5.32秒、main全差分lint20.03秒PASS。
Windows部品は0.83秒PASSで通知表示は当初SKIP。別途有効にした実通知表示・履歴・削除は2.64秒/test2.63秒PASSし、日英XMLの受入を確認しました。
人のclick・見た目の確認ではありません。先行のPowerShellによる-test.v解釈は試験前に失敗し、構造化引数へ直したfixture実行と区別します。

通常のWindows helper導入処理で候補を専用WSLへ適用しました。Linux側809bfb33のprotocol実装は変更していません。
installed-review-1は実COM登録・所有再開/反復・別所有者/activator不一致拒否・失効/不正入力拒否・Host通知controller購読・listener cleanupがPASS。
人のtoast click・新しいGUI回答は明示的SKIPです。Policy/認証の変更・回答の再送はありません。
#678 CIのactivation timeout、#680のcompact_attachedは別の未解決観測として保持します。


最終の準備確認候補も全ローカル26.41秒、focused5.11秒、main全差分lint19.06秒、race2.30秒、CLI4.14秒、docs/regressions8.03秒、workflow1.54秒が成功しました。
未実施の人の回答を受入済みにするものではありません。


統合候補2f49460fは通知準備修正#682も含みます。
focused15.09秒、main全差分lint14.42秒、全ローカル28.63秒、race11.70秒、CLI4.18秒、docs/regressions8.27秒、workflow1.48秒が成功しました。
対応Incus・実Windowsで別途確認した両実装は変更していません。

## 保存データの削除診断候補

実装 `9c2736db` の `snapshot-full-3` で、対象回帰17.96秒、main差分全体のlint
10.05秒、保守対象の全体テスト25.99秒、ライフサイクル/APIのrace検査16.47秒、
CLI E2E 4.12秒、文書8.06秒、workflow policy 1.51秒、実Incus用ビルド1.51秒が成功。
初回は表示書式と試験側の要求オブジェクトに残った項目で失敗し、修正した。
2回目は対象回帰成功後、新しい診断出力のerrcheckで失敗。修正後の3回目が全成功。

専用WSL `Hacocoon-Roadmap-f68a8c6b` / Incus 7.0.1 の `snapshot-native-2` は
25.75秒（試験25.71秒）で成功。試験名 `saved-data-61784250e8f288f3`、台帳
`/var/lib/haco-saved-data-3210844539/state.json`。rootfs、Workspace 2件、管理データ
2領域の存在・所有を読み取り専用で確認し、通常削除、独立コピー、元Env削除、
コピー再開、正確な所有対象のcleanupまで成功した。サービスとproviderの確認であり、
インストール済みCLI、Btrfs内部の整合性、OCI実行、人による承認、巨大レポ性能の確認
ではない。`snapshot-native-1` は実機試験の指定漏れにより **SKIP**。終了コード0を成功とは数えない。

## 通知起動のCI残件

[PR #682](https://github.com/SLktEx/Hacocoon/pull/682)、`bd83251b` はLinux系4項目成功、
[Windows run 34946460934](https://github.com/SLktEx/Hacocoon/actions/runs/34946460934) は
job `104306922882` の通知起動で失敗。COM生成のHRESULTは `-2146959355`、native進捗は未観測。
その前のインストール、厳格なSSH/editor、転送、公開reclaim、切り離したWorkspace/OCI/snapshot
の復元は成功している。ローカルのインストール済み通知確認成功で、この失敗や以前のdispatch
タイムアウトは消さない。#683はこの変更を含み、候補自身の確認が必要。

## Baseアーカイブ取り込み候補

実装 `35a0c496` の `base-full-3` で、対象回帰23.08秒、main差分全体のlint
18.20秒、保守対象の全体テスト33.10秒、一時保存/build/転送/ライフサイクル/APIの
race検査18.88秒、CLI E2E 4.51秒、文書8.78秒、workflow policy 1.57秒、CLIビルド
0.71秒、実機試験ビルド1.93秒が成功。追加した一時Workspace境界の回帰もrace付き
24.29秒、差分lint 39.84秒、文書9.49秒で成功した。初回は既存の一時保存上限値の
回帰で失敗し、共通化によって有効な境界値を拒否しないよう修正した。2回目は対象
回帰成功後、新規3件のerrcheckで失敗して修正した。ビルド用WSLの整形と最終検査には
systemdのrootユーザーセッション起動警告が出たが、コマンドとテストは成功した。
この警告をWindows利用の新しい確認成功とは数えない。

専用WSL `Hacocoon-Roadmap-f68a8c6b` / Incus 7.0.1 の `base-native-1` は実際の
`haco base import` CLI/controller転送を165.94秒（試験165.89秒）で完了した。
専用台帳は `/var/lib/haco-base-import-1372522241/state.json`。所有する元rootfsを
書き出して元Envを削除し、アーカイブを隔離した一時作成環境で取り込み、不変Base
`sha256:6fbaf82f1e891f16d32da5186119908cd1499614018eeb2646f7828fd74986b8` を公開。
新しいEnvで取り込んだツールを使えた。通常の所有確認によるEnv・イメージ整理も成功し、
入力アーカイブは保持した。native CLI/providerの確認であり、配布済みWindows入口、
認証付きGit、Packer依存物、巨大レポの性能確認ではない。

[PR #683](https://github.com/SLktEx/Hacocoon/pull/683)、`bda75b67` はLinux系4項目成功。
[Windows run 34947135337](https://github.com/SLktEx/Hacocoon/actions/runs/34947135337) の
job `104309115278` は公開reclaim成功後、#682と同じCOM生成HRESULT `-2146959355` で失敗。
このheadはmainマージ条件を満たさない。ローカルの通知成功で失敗を消さない。

## 通知の所有・準備完了とcontrollerの初回起動

`e043b740`で、実Windowsのkernel objectを使った未準備の所有者・終了待ちの引き継ぎ・
起動失敗後の再取得が通りました。`lifecycle-local-2`のWindows構成要素試験は1.84秒でPASS。
通知表示の最初の実行はworkspace権限で一時registry作成を拒否され、通常ユーザーのregistry操作を
許した同じ試験は11.28秒（test 11.18秒）でPASS。日英XML・履歴・除去を確認し、人の回答・見切れは未確認です。

後続`92ce27a5`を含む`lifecycle-full-2`はfocused 11.02秒、mainとの差分全体lint 21.31秒、
維持中の全体試験35.57秒、private client/reviewのrace 15.74秒、CLI E2E 4.75秒、
docs 10.72秒、workflow policy 1.81秒、native compile 4.24秒ですべてPASS。
遅れて準備されるcontrollerを要求消費前に待つこと、拒否を再送しないこと、一覧取得失敗を成功扱いにしないことを確認します。

Windows helperだけ更新した`installed-review-1`と`-2`は、Linux側が`809bfb33`のままで
private準備確認に失敗しました。読み取り調査ではpending成功とpending_unavailableの両方を観測し、
WSL起動時にcontrollerの接続口がまだ無いことを確認しました。権限・サービス・Policyの抜け道は加えていません。

通常のWindowsパッケージインストーラで専用WSL`Hacocoon-Roadmap-f68a8c6b`と全companionを
`92ce27a5`へ更新しました（ローカルbuild 37.42秒、v0.0.0-e2e、未配布）。このWSLだけ停止した後の
`installed-review-3`は、実登録・所有対象の再開・冪等性・他所有者とactivator不一致の拒否・
古い要求と不正な要求の拒否がPASS。直後のHost購読確認は通知サービスの起動前に失敗しました。
後の読み取りではenabled/active/running、再起動0回、successでした。最初の起動時の結果は未解決の
タイミングとして保持します。`installed-review-4`は同じ経路とHost controller購読、auditの投影なし、
所有listenerのcleanupまでPASS。人による新規回答は明示SKIPで、CI全体やM0〜M5全体の完了ではありません。

[PR #684](https://github.com/SLktEx/Hacocoon/pull/684)の`a45e936f`はLinux側4workflowがPASSですが、
[Windows run 34949250114](https://github.com/SLktEx/Hacocoon/actions/runs/34949250114)のjob
`104316004120`はreclaim成功後にclear/timeout（20,013 ms、native progress decode）で失敗しました。
[PR #685](https://github.com/SLktEx/Hacocoon/pull/685)の`d6c9fa13`もLinux側4workflowはPASSですが、
[Windows run 34951609643](https://github.com/SLktEx/Hacocoon/actions/runs/34951609643)のjob
`104323633089`はcompact_attachedで失敗しました。Linux回収は完了、Windows圧縮は未実行、再開成功、
通知受け入れはSKIPです。失敗したheadはマージせず、以前のCOM作成・dispatch失敗も保持します。

## Env内キャッシュの掃除

候補 `2ca6af59c16b49d499c8c557f589934c1fcb33ad` の`empty-full-1`は対象回帰19.93秒、
main全差分lint40.76秒、通常の全体テスト102.05秒、race50.65秒、CLI E2E6.73秒、
docs14.90秒、workflow policy2.46秒、native build4.97秒で成功しました。
台帳への直接snapshot/収集も掃除中に止める追加後、`empty-final-guards`は対象22.16秒、
lint13.25秒、race27.00秒、docs9.49秒、native build2.15秒で成功しています。
保存済みsnapshotの保持、台帳再読込後の停止継続、新規snapshot拒否、同じ所有者への明示的再試行を確認しました。

先行`empty-local-2`は追加テストの依存不足と非端末を模したメモリreaderの使い方で失敗しました。
`empty-local-3`では既存の非端末拒否が期待した1ではなく終了コード2を返しました。
fixtureを直した`empty-local-4`は29.89秒、native build2.31秒で成功し、製品の権限や確認は緩和していません。
整形処理ではWSLのroot user-session起動警告も出ましたが、整形自体は成功しています。

専用Incus7.0.1の`empty-native-1`は34.00秒（試験33.96秒）で成功しました。
fixtureは`data-e2e-ee7f06d38f566742`、台帳は
`/var/lib/haco-data-placement-3790057143/state.json`です。
通常Env作成・収集・独立再利用、単一領域と全Envの掃除、入れ子ファイル、Workspaceへのリンクを
たどらない削除、別領域・共通元・Workspaceの保持、通常再開と共通cleanupを確認しました。
導入済みCLI、実OCI/snapshot保持、人によるUI回答、巨大レポ性能の受入はこの結果に含みません。

最終の台帳側ガードを含む`empty-native-2`も26.36秒（試験26.31秒）で成功しました。
fixtureは`data-e2e-37c63c45b570dc31`、台帳は
`/var/lib/haco-data-placement-605572226/state.json`で、確認範囲は同じです。

親[PR #686](https://github.com/SLktEx/Hacocoon/pull/686)の`935c0752`はLinux4 workflow成功です。
[Windows run 34955347257](https://github.com/SLktEx/Hacocoon/actions/runs/34955347257)の
job `104335936830`はpublic reclaimで`compact_attached`となりました。
Linux完了・停止要求済み・open1回・Windows圧縮未実施・resume成功で、通知試験はSKIPです。
ローカル通知成功とこの失敗を分け、このheadはmainへマージしていません。

## main統合と容量回収の診断

[PR #687](https://github.com/SLktEx/Hacocoon/pull/687) のhead
`21b2452b63cb58d86e9adc3cda546fc1c3214149` で5 workflowがすべて成功し、
mainへ `2f421006d1ce86edbb5a1deb3da9c17c46a5ef5c` としてsquash mergeした。
[Windows job 104346984027](https://github.com/SLktEx/Hacocoon/actions/runs/34958740591/job/104346984027)
では通常入口、SSH/editor/tunnel、容量回収、保持Workspace/OCI/snapshotの復元、
導入済み通知の拒否・購読・cleanupが成功。割当量は7,864,320,000から5,020,581,888 bytesへ
減少（2,843,738,112 bytes回収）、仮想容量1 TiBとpool容量128 GiBを維持した。
人による通知クリック・新規GUI回答は明示的なSKIP。この後続成功だけでは以前の
`compact_attached` やCOM失敗の原因は確定せず、元の結果を保持する。

既存の専用導入環境（`92ce27a5`）では `ordinary-reclaim-1` が端末開始から36.78秒で
保存操作の作成前に失敗し、Windowsの保存状態も `none` のままだった。再送や記録削除は
していない。別の正確なGUIDによるsystemd停止観察では、停止後に共有エラー32が続き、
81.30秒でディスク解放を観測、108.41秒で同じGUIDの再開が成功した。圧縮試験でも
CIのディスク使用中の観測を再現した証拠でもない。

診断候補の `detach-local-2` は対象回帰3.31秒、Windowsのwslreclaim 0.77秒、
reclaimclient 3.83秒、haco-wsl 0.45秒で成功。導入識別の読み取り5.54秒、
既存登録・所有者・ファイルの照合5.78秒も成功し、操作の作成・trim・停止は行っていない。
初期の補助プログラム試験は失敗時の標準出力が空という旧assertion 2件で失敗し、
制限された診断応答に合わせて修正した。`detach-full-1` は追加した表示処理の
errcheckで失敗し、戻り値の扱いを修正した。導入済みの通常操作の再試行成功は未確認。

`detach-full-2` は対象回帰3.25秒、main差分lint 5.27秒、ローカル全体12.31秒、race 10.68秒、CLI E2E 2.88秒、docs 6.16秒、workflow policy 0.97秒、native build 1.11秒で成功した。

候補 `97ffa2d66c223ebced04b195bc1d409d59b43829` を通常パッケージでbuild（21.54秒）し、専用の既存WSLへLinux/Windowsを揃えて導入（44.86秒）、インストーラのdoctorは成功した。`ordinary-reclaim-2` は端末開始から21.88秒で再び失敗したが、今回は準備/登録情報、Windowsエラー2と表示できた。同じ対象のWindows直接照合は成功しており、経路による差は未解決。見つからない登録の作り直しやワーカー再送はしていない。

## 復元ツリーの照合

実装 `9f946abc561393141df5d0ef9a081ab1949f6b6f` で、既存の読み取り一覧を使う
持ち運び可能なツリー比較を追加。`compare-full-1` は対象ファイル/実tar回帰0.48秒、
ローカル全体82.69秒、CLI E2E12.45秒、docs18.00秒、workflow policy2.71秒で成功。
対象6試験は実際の復元、ツリー内リンク、symlinkをたどらない確認、xattr、
内容/mode/所有者の差、未完了、差し替え、不正な一覧を確認した。

`compare-native-1` は27.59秒で成功。保持中のhacocoon-secondの
`/home/codex-second/fixtures/workflow`（26項目、ファイルlogical合計2,343 bytes）を
既存のGNU tar手順で `/var/tmp/haco-reviewed-capture-md4e11_9` へ取得。
51,200 bytesのarchiveをWSL外へ保持してSHA-256を照合し、
Hacocoon-Roadmap-f68a8c6bの新しい非公開ツリー
`/var/tmp/haco-reviewed-restore-8f9kdnr5/tree` へ復元した。
Linux側の走査とWindows側の一覧比較が一致し、元データ・取得物・保持archive・
復元先を残した。数値所有者はLinux Host側の名前空間で比較しており、guestのidmap対応、
現行データ全体の移行、認証付き開発、巨大レポ性能の成功とはしない。先に確認した
network-finalは空だったため、内容復元の証拠には使っていない。旧版の再構築・置換は対象外。

保持側の読み取り一覧はnative照会がすべて成功し、hacocoon内の12 instance・
48 custom volume項目・2 imageと別所有のcache volume 1件を観測した。
所有関係の確認、手置きデータの分類、全体backup完了は一覧取得とは別の状態として保持する。

別件の容量回収調査では、Windows直接と信頼済みHost経由が同じ所有者ハッシュ・64bitでも
登録の見え方が異なった。最初の端末probeは位置照会でtimeoutし、修正した読み取りprobeで
差を観測。保守目的のWSL再起動は、対象外のUbuntu-24.04稼働を検知して実施しなかった。
全体停止・登録上書き・データ削除はしていない。実行経路差の原因は未確定。


<a id="reclamation-language"></a>
## 容量回収の日英表示

実装 `99522ebd893e7fbdc0752e3db3d6f84595f10fb6` は、通常の容量回収結果・容量・
失敗後の案内・中断記録の確認を共通翻訳へ接続した。対象テスト9.45秒、差分lint19.86秒、
ローカル全体43.65秒、race14.50秒、CLI E2E4.05秒、docs13.30秒、workflow1.78秒、
実Incus用ビルド1.85秒が成功。日本語でも操作・対象・状態への確認と生の通信値を
保持する。通常経路の表示確認は日英を扱い、完了判断には元の機械応答を使う。
導入後の表示確認は別途記録する。初回整形は同時編集をハッシュ照合で拒否し、
新しい入力を固めて整形してから上記の検証を行った。

先行 #688 は `4e0a483e` の5 CIすべてが成功し、main `ee8bf7fb` へ反映した。
専用導入で登録情報が見えない手元の失敗や、人による新しいGUI回答の確認は別に残る。

同じ実装から通常パッケージを51.68秒で生成し、Linux/Windowsを揃えて60.72秒で
導入した。doctorは全項目成功。Windows→WSL→Hostの通常入口で日本語の縦ヘルプと
保存結果の読み取りが45.28秒で成功し、結果なし・終了0を確認した。初回の専用観測は
内部の簡易ヘルプを期待し、実際の共通縦ヘルプに対して誤って失敗したため修正して
再実行した。WSLは管理ユーザーのsystemd session警告を出したがHostへ入場できた。
警告の原因は未調査。この読み取り成功は、新たな容量回収開始・圧縮や人のGUI回答の
成功を意味しない。


復元照合head `379f0b156e853d81b00a2933dbd6983f00718cc9` のWindows
[run34964494309/job104365643144](https://github.com/SLktEx/Hacocoon/actions/runs/34964494309/job/104365643144)
は導入・HTTPS・interop・SSH/エディタ/転送・Linux容量回収が成功した後、通常reclaimが
失敗。Linux両段階complete、Windows停止要求済み、open attempts1、compact_attached、
圧縮未試行・再開未成功。通知確認はSKIP。Linux側4 CIは成功した。このheadはmainへ
マージせず、同じ実装を含む #690 でmainとの同内容の履歴重複を整理した。候補の
ファイル内容は変えていない。Windows失敗の原因は未確定として保持する。

導入済み `99522ebd` で3回目の通常・日本語reclaimを実行したが、91.11秒で
準備/導入時の登録情報・Windowsエラー2となった。読み取りは成功し、その後も保存結果なし。
新しい操作記録・停止・圧縮・自動再送は観測されていない。日本語の診断は機能したが、
導入記録の見え方の差は未解決。


<a id="supported-dns-modes"></a>
## 対応IncusでのDNSモード確認

専用Hacocoon-Roadmap-f68a8c6b / Incus7.0.1と導入済み製品
`99522ebd893e7fbdc0752e3db3d6f84595f10fb6` で、既存の3モード実機回帰が24.68秒
（テスト24.61秒）で成功。host10.31秒、backend8.09秒、disabled6.20秒。新しい所有Envを
作成し、停止/再開前後のresolver service/設定とbackend名前解決を確認後、共通の所有確認付き
削除が成功。カタログは `/var/lib/haco-dns-modes-2079785240/state.json`、
`-3117962770/state.json`、`-811325797/state.json` に保持。Policy・設定・依存取得の
テスト用変更はない。従来6.0.5の証拠を補うもので、ゲストのPolicy問い合わせ全体、
Windows DNS tunneling、VPN/NRPT、巨大レポの証拠ではない。初回の専用観測はIncus infoの
不正なオプションでテスト前に停止し、既存の query /1.0 に直して対応server版を確認した。

M0〜M5・実装状況の整理はローカルの維持されているdocs確認39.09秒と、その後のリンク確認が
成功。古い候補の日誌を整理し、固有の失敗証拠を保持して日英の所有文書を更新した。
製品コード・チェックポイントの変更はない。


#690 head `b6dec8807e026bf9c765db0af16eea238186da06` はLinux4 CIが成功したが、
Windows [job104373589771](https://github.com/SLktEx/Hacocoon/actions/runs/34966961367/job/104373589771)
でtest_windows_environment_ssh.ps1:554のhost key変更確認が失敗。そこまでのSSH・エディタ、
承認Webviewの古い要求拒否、保存選択の確認、setup、preview、import/再作成後の作業、
Windows転送は成功。Linux/通常容量回収・native通知はSKIP。該当assertは終了値・
禁止したstdout・host-key診断の欠落をまとめており、どの条件だったかはログだけでは不明。
その後のPolicy後始末と切断ではWSLのCatastrophic failureが記録された。鍵照合の迂回が
証明されたわけでも、拒否確認の成功でもない。原因を確認するまでこのheadは未マージとし、
先の #687/#688 成功で失敗を消さない。


## Windowsのhost key拒否診断

#690の後続では既存の3条件を維持し、失敗した条件と許可リスト内のSSH進捗を分ける。
job104373589771で観測したNULを含むWSL E_UNEXPECTEDも、生の子プロセス出力を
表示せず分類する。ローカルPowerShellの回帰と、実プロセスのtimeout/非ゼロ確認が成功。
次の通常経路の証拠を改善する変更であり、以前の失敗原因の確定・解決ではない。
新しい許可・再起動・再試行は追加していない。

## 環境名からsnapshotを復元する

実装 `e1ec0894` は `snapshot restore --latest <source-env> [new-env]` を追加した。
ローカルの対象11.21秒、差分lint13.35秒、通常全体30.53秒、race14.92秒、
CLI3.78秒、docs8.23秒、workflow1.41秒が成功。実カタログの再読み込みで保存日時を確認し、
順不同の保存・未完了/他Envの除外・日時不明/同時刻・一覧失敗・選択後の失敗でも
別の保存を再選択しないことを確認した。初回lintの診断出力の戻り値指摘は修正後に成功した。
v0.68更新後の生成識別子・通常docs確認も成功。

同版10バイナリの通常パッケージ生成は34.67秒で成功。導入後の実機確認は別に記録する。
このローカル確認だけではWindows SSHやOCI復元の成功とはしない。
以前のWindows失敗とPacker設定の確認待ちは未解決のまま保持する。

専用Ubuntu26.04.1/Incus7.0.1へ通常導入39.73秒、doctor成功。同版の通常CLIで
このリポジトリを取得し、管理対象WorkspaceとOCIなしのEnvを作成した。
異なる内容を2世代保存（4.19/4.27秒）、元Envを削除した後、その環境名から
最新を復元した（5.53秒）。復元先はrunningとなり、2回目の内容を確認できた。
元の2つの保存記録は不変。確認用ファイルの読み書きだけはPhysical HostのIncus execを使い、
作成・保存・復元・削除はすべて通常の導入済みCLIを使った。
デスクトップSSH・OCIの一連操作や巨大レポ性能の証拠ではない。
新しく作ったEnv・2つのsnapshot・2つのWorkspace・取得元登録のみ、通常CLIの6操作で後片付けも成功。
実機記録: `latest-b61bbc62`、復元Workspace `restore-5dbfe05afd5db0c4`、
実装 `e1ec08947e8ae2c7db5d0251f246cdc9403446a5`。

## Windows統合の再実行とinterop観測

PR #692 head `4e7a45a75047c8d372da1ab889eb7d2796f4370b` はLinux4 CI成功。
Windows34970515521/job104385385746はVS Code拡張導入のHTTP503で失敗。
通常SSH・cold並行接続・鍵変更拒否・DNS/Policy・setup・preview・転送・復元後の作業は成功。
Linux/通常容量回収とnative通知はSKIP。外部依存の503に対してWindows失敗ジョブのみ
1回再実行した。成功・マージ済みとはせず、以前の#690の原因不明失敗も保持する。

手元は通常Envと容量回収操作がないことを確認し、専用WSLだけ再起動。doctor成功後も
登録の見え方は変わらなかった。Windows直接と物理WSLの通常呼び出しは登録ありだが、
`/run/WSL/1_interop` を使うと物理WSLでも信頼Hostでも登録なしになる。
同じWindows所有者・64bit・package identityなしを観測した。
登録そのものの欠落やIncus境界だけの問題ではなく、init interop経路で差が生じることまで
切り分けた。Windows側の原因は未確定。登録/操作履歴の変更・製品の接続口置換・
WSL全体停止・新しい容量回収開始は行っていない。

## WSLのWindows実行登録の復旧

導入候補 `e1ec0894` で後続の登録情報観測は、WSLInterop binfmt登録が存在せず、Windows実行前に失敗しました。
導入済みの通常 `haco setup` でWSL自身の登録を復旧し、Windowsプログラムから
期待する文字列を返すことを確認しました。容量回収の開始成功を意味しません。

## 通常Gitの既存履歴再利用

開発実装 `6088e6c542ffb9b13e0a2b3650c4d992c8a57435`
（[#695](https://github.com/SLktEx/Hacocoon/issues/695)）で、対象11.52秒、lint24.76秒、
通常全体29.90秒、Git race36.27秒、CLI9.34秒、docs9.28秒、workflow policy1.38秒、
nativeテストのコンパイル1.75秒が成功しました。実Gitのcomponent回帰では既存の
ランダムデータ34,603,008 bytesに対し、小さな変更のfetch293 bytes、push準備319 bytesを
確認しました。準備でリモートは変化せず、既存brokerの承認テストも成功しています。

初回は埋め込んだ `bytes.Buffer.ReadFrom` により、プロセス出力が上限付きWriteを
経由しない不具合を検出しました。埋め込みを除去後、全履歴上限と実pipe転送の回帰が
成功しました。33 MiBのローカル機能確認であり、導入済みIncus/Windows・認証Gitの受入や
代表的な巨大レポ性能ではありません。新規target・新しいpack自体の上限は残ります。
この修正でv0.68のcheckpointは進めず、リリースもしていません。

#692 head `4e7a45a7` のWindows再実行job `104394453906` はSSH/editorとLinux回収に成功後、
公開回収で `compact_attached` となりました。停止要求あり、open1回、compaction未実行、
同じtargetの再開は成功です。通知試験はSKIP。初回HTTP503と以前の未解明失敗は別に保持し、
このheadをmainへ反映する条件は未達です。

## 仮想ディスク観測ハンドルの寿命

開発実装 `f50c0d93445f3f6f427b0e301294e5101f94a65e` は、接続中の観測ハンドルを閉じて
期限内で待ち、実ファイル・親の固定を維持します。
[ADR 0103](../adr/0103-virtual-disk-observation-lifetime.ja.md)にnative APIの根拠と
観測ハンドル・所有固定の違いを記録しています。

ローカル対象9.91秒、lint17.21秒、通常全体39.95秒、回収関連race1.61秒、CLI4.96秒、
docs13.49秒、workflow1.99秒、nativeテストコンパイル2.18秒が成功しました。
Windowsの回収0.80秒・client6.09秒・helper0.47秒も成功。Windows専用lintの初回は
テストのclose結果確認漏れを指摘し、修正後lint1.82秒、対象native回帰1.22秒が成功しました。
空ディスクのnative接続fixtureは `ERROR_PRIVILEGE_NOT_HELD` により**SKIP**です。
権限の昇格・迂回はしていません。他の専用ディスクnative試験も有効化していません。
これらを新候補での導入済み公開回収成功とは扱いません。

別の読み取り観測では、導入済み `e1ec0894` の `Hacocoon-Roadmap-f68a8c6b` に対し、
Envが空・操作記録なしを確認してpoweroffを要求しました。native openは90秒間共有違反となり、
90.94秒で同じtargetの再開に成功しました。比較に必要な保持ハンドルは取得できていません。
systemd記録には途中の起動があり、別のWindowsプロセスによる対象bashも観測しました。
誰が利用しているかは確認待ちで、プロセスを終了していません。この試行からハンドル寿命の
結論は出せず、CI #692の `compact_attached` とは分けて保持します。過去の失敗を消しません。

#697 head `b0b2fcbc` の通常10バイナリのインストーラ生成は34.90秒で成功しました。
専用WSLの並行利用が未確認のため、上書き導入はしていません。導入済み `e1ec0894` と
既存データを保持しています。これは配布物の生成確認であり、公開回収の受入ではありません。

## 新規Gitブランチの履歴再利用

開発実装 `e17e5132e0ce9769a7c6446ac496797baa7df209` は、既存履歴の再送削減を
通常の新規ブランチ準備へ広げます。実Gitの部品回帰では、既存ランダムデータ34,603,008 bytesに
対し、小さな新規ブランチ変更のpackは322 bytesでした。対象は未存在を要求する新規ブランチの
ままで、準備でリモートを変更していません。基準refの読み取り拒否・移動・不一致ではpush前に
停止し、既存の通常broker承認回帰も成功しました。

最初の対象検査13.70秒に続き、最終対象13.85秒、lint24.54秒、通常全体40.22秒、
Git race33.37秒、CLI4.34秒、docs11.65秒、workflow1.72秒、nativeテストコンパイル1.69秒が
成功しました。部品と実Gitの証拠であり、導入済み認証Git、main反映、リリース、巨大レポ性能の
受入ではありません。新しいpack自体が32 MiBを超える場合は非対応です。v0.68は変更しません。
[ADR 0104](../adr/0104-new-branch-git-history.ja.md)を参照してください。

## 統合候補のWindowsトンネル失敗

#697 head `b0b2fcbc` はquality `34979869527`、test `34979869379`、Ubuntu `34979869467`、
Incus `34979869532` に成功しました。Windows `34979869494` のjob `104417065184` は、
導入済みSSH・停止後の並行再接続、実VS Code編集、承認webviewの拒否、保存した判断、preview、
Windows経由の転送・復元後作業・再作成に成功した後、通常トンネルで失敗しました。
native listenerは確認できましたが、8接続の転送でWindows reset `10054` が発生し、
アプリfixtureも `accept` でtimeoutになりました。アプリ側40秒の待機、stream準備、
ほかの条件のどれが原因かはログから確定できません。製品・fixtureの原因を断定しません。

この実行のLinux回収・公開回収・native通知は**SKIP**です。切断待ち修正の導入済み回収受入に
到達していません。mainは `ee8bf7fb` のままで、この失敗後の再試行・マージは行っていません。
以前の失敗も保持します。

#698の `023ca03e` では通常10バイナリとインストーラを40.22秒で生成できました。
導入は並行利用の確認待ちです。起動中の専用WSLでHostの `gh auth status` を読み取り、
GitHub未ログインと確認しました。本人に通常Host内のログイン手順を提示済みで、認証情報の
出力・転送はしていません。認証付きGitの受入は未実施です。

## カタログ単位のロックと通常Env作成（PR #699）

専用Incus 7.0.1／導入済み `e1ec0894` で通常cloneとWorkspace作成は成功したが、
Env `tunnel-b60c7032` の作成はprovider呼び出し前に失敗した。最初に調べたtrusted
Host内には対象がなく、管理サービスが動くPhysical Hostで共通一時ロック領域の
所有者UID/GID 1000・0700、管理サービスの実効UID 0を確認した。状態管理領域
`/var/lib/hacocoon/state` はUID/GID 0・0700。未知の一時領域は変更・削除していない。
新規repo `tunnel-b60c7032-repo` とWorkspace `tunnel-b60c7032-work` は保持している。
Env状態のnot-foundだけをprovider不在の根拠にして後片付けしていない。

Windowsから通常の登録済み制御経路への読み取りpingは、`e34c2bf8` で1回46ms、
8並列67〜101ms。TCPデータ転送の成功や#697の失敗原因を示すものではない。

#699の `1ae5b410` はロックをカタログの保護領域へ移し、必須の共通操作として
作成・削除・cleanup等の排他を統一した。最初の検査は親ディレクトリが0755の場合に
拒否して失敗した。所有者と他者書き込み不可を確認した親を固定し、0700の子領域を
使う修正後は、対象9.55秒・lint18.78秒・全体30.88秒・race12.11秒・CLI4.18秒・
docs8.50秒・workflow1.39秒・Incus試験コンパイル1.94秒が成功した。後続文書と
Windows試験の経過記録も整合・構文検査済み。WSLのroot systemd-user-session警告は
残る。本人ログイン・通知／VS Code操作はリリース後の確認とし、main反映を止める
条件にしない。未実施を成功へ置き換えない。

同headの品質 `34986231603`、通常テスト `34986231525`、Ubuntu `34986231600` は成功。
Incus `34986231453` はstandalone/Core成功だが、Btrfs job `104438909790` は全製品操作
（保存・export/import・復元・copy・保持データ・所有資源削除）後、試験用の後片付けが
追加された `lifecycle-locks` ディレクトリを想定外として失敗した。失敗履歴を保持し、
ロック自体は残したまま、全項目を確認後に完了済みの回復用ファイルだけ整理する。
未知のディレクトリやシンボリックリンクの拒否、provider不在条件は維持する。

`1ae5b410` の通常Linux/Windowsパッケージ10バイナリは69.93秒で生成成功。
導入・公開はしておらず、実行中のWSLは停止していない。修正後の導入済み受入は未完了。

## 複数ブランチのGit取得を順番に取り込む

`45555463` で1024head・各pack 32 MiB・正確なrefごとの認可を維持し、batch合計の
サイズ拒否を撤去した。一つの応答を取り込んでから次を要求する。独立した17 MiBの
ランダムデータを持つ2ブランチで、修正前は合計サイズ拒否で失敗（コマンド18.41秒）、
修正後は両方のcommitの内容を確認できた（9.69秒）。合計35,662,881 bytes、各packは
33,554,432 bytes未満。最初の試験呼び出しはWindows側シェルの構文エラーで未実行であり、
上記の失敗／成功は呼び出しを修正して実際の回帰を実行した結果。

最終のGit回帰18.23秒、lint28.19秒、全体47.06秒、race42.43秒、CLI5.57秒、docs9.77秒、
workflow2.06秒、native試験コンパイル1.72秒が成功。機能の構成要素確認であり、巨大レポの
性能や認証付き導入済みGitの受入ではない。個々のpackが32 MiBを超える場合は残件。
Incus試験の後片付け修正 `d4c264a3` も対象回帰23.41秒（試験部分0.041秒）とlint17.36秒が
成功。両変更を `5980d18f` で競合なくローカル統合した。

Windows上の短い実プロセス回帰で、native受入の時間切れ時に完了済みstdout/stderrの記録が失われることも再現した。修正後は制限付き出力を時間切れ例外に保持し、失敗前に表示する。30分の期限や必須成功表示は変更しない。修正前は出力が空で回帰失敗、修正後はnative-runner全7試験がWindows上1.671秒で成功した。実行中#699のWindowsの原因や成功を示す証拠ではない。統合した `77a4c8cc` の通常10バイナリパッケージは31.96秒で生成成功し、導入・公開はしていない。

## カタログロック統合候補のWindows受入

#699の `1ae5b410` はWindows workflow `34986231470`、job `104438908869`、
evidence job `104450106625` が成功。通常インストール・再起動・再導入、egress/DNS拒否、
冷間並列SSH、VS Code 1.136.1の実編集・端末、保存したsetup、承認、preview、
export/import・保持データの再作成が成功した。通常のnative TCP転送も8並列1MiBの
送受信・片方向終了・Ctrl+C後の待受終了が成功。アプリ準備234ms、Host準備26,234ms、
待受27,405ms、native所有者確認29,875ms、送受信完了31,969msだった。この成功から
#697のreset/accept timeoutの原因を断定しない。

導入済みLinux段階と公開Windows回収も完了。Windows割当は7,730,102,272から
4,965,007,360 bytesへ減少（回収2,765,094,912 bytes）、仮想容量1,099,511,627,776 bytesは
維持し、同じWSLの再開に成功した。320回の期限内のopen観測後に圧縮を一度実行。
回収後のHostデータ・切り離したWorkspace/OCI・保存データ復元も成功。通知の登録・
所有者・古い/不正な回答の拒否・購読確認は成功した。人による通知クリック/新しいGUI回答、
VPN/NRPTは明示的SKIPとしてリリース後に残し、認証Gitの成功は主張しない。

同headの品質・test・Ubuntu・Windowsは成功だが、Incusは前述の試験用ロック領域の
後片付けで失敗したまま。追補 `d4c264a3` はローカルで修正・確認済み。mainはまだ
`ee8bf7fb` であり、この成功を異なる未確認headのマージ根拠にはしない。

## セットアップ結果の日英表示

`e5a4e1e8`はHost・プロジェクトの結果と次の操作を共通の日英カタログへ集約し、
縦ヘルプを再利用する。ローカルの対象テスト8.62秒、lint18.14秒、全体test34.72秒、
race13.73秒、CLI4.84秒、docs8.58秒、workflow policy1.91秒、nativeテストの
コンパイル1.55秒が成功した。追加ファイルも含む最終lintは22.42秒、Linuxの
承認確認runnerの全9回帰は0.57秒で成功。Windowsでは対象6回帰が成功し、
Linux端末用3件は明示的にSKIP（テスト時間0.021秒）。runnerは正確な成功markerと、
日英いずれかの正しい完了文1行だけを要求する。

初回の実行準備はLinux GitからWindowsのworktree情報を解決できず、テスト前に失敗した。
patchをWindowsで生成するよう修正した。初回の対象テストではHost結果の二重整形を検出して修正し、
変更行lintの未処理の出力戻り値の指摘も修正してから全体確認が成功した。
終了状態、スクリプト出力、非公開結果の閲覧、明示的な再実行、構造化診断の境界を保つ。
これはリポジトリ・componentの証拠であり、新たな実機・人のGUI操作の成功ではない。

## 保持データ・ライフサイクル修正のmain統合

[#699](https://github.com/SLktEx/Hacocoon/pull/699)をmain
`e4d99700b976e2166a4a0b27dc9f37cf3aaaacc1`へ統合した。同一head `51ba4f24`の
quality `34989824960`、test `34989824791`、Ubuntu `34989824936`、
Incus `34989824882`、Windows `34989824868`がすべて成功し、統合後のtreeも一致する。
IncusのCore・standalone・Btrfs・evidence全jobが成功し、先行headで失敗した
aggregate fixtureの後片付けも成功した。

Windows job `104451279682`では通常導入、EnvのHTTPSと直接通信の拒否、
native入口・SSH・実エディタ・転送、Linux/公開容量回収、Workspace/OCI/snapshotの
保持と復元、native通知の所有確認・拒否・購読が成功した。公開操作
`{68E10593-EC50-41D2-875E-F71D4F303386}`は**2,840,592,384 bytes**を回収した。
Windows割当容量は7,797,211,136 → 4,956,618,752、仮想容量1,099,511,627,776は不変。
252回のopen待ち後に圧縮が完了し、同じWSLが再開した。人による通知クリック・新たなGUI回答と
VPN/NRPTはSKIPのまま。過去の失敗と専用ローカル環境の登録情報問題は区別して保持し、
後続の成功から原因を推定しない。

日英セットアップの追補はこのmainへ載せ直し、元の`724adc2d`とtreeが一致することを確認した
（実装`e5a4e1e8` → `8b95f79a`）。元の`724adc2d`から通常10バイナリの
Linux/Windowsパッケージ生成が45.49秒で成功した。導入・WSL停止・公開は行っておらず、
新たなリリースの証拠とは扱わない。

## セットアップ候補のWindows容量回収失敗

#700のhead `5e2ee17bcdc6e2ee36766f00ed2877485777e2c8`ではquality
`34993511402`、test `34993511349`、Ubuntu `34993511337`、Incus
`34993511396`が成功した。Windows `34993511328`の初回job
`104463877953`は公開容量回収で失敗した。Linux両段階とWindowsの停止要求は成功したが、
既存の90秒間で359回確認してもディスクの接続が残り、`compact_attached`で圧縮を実行せず、
同じWSLを再開した。後続のnative通知確認はSKIP。通常導入・SSH・実エディタ・転送と、
先行する導入済みLinux容量回収は成功した。

原因は未確定。セットアップ変更は停止・圧縮処理を変更していない。
再現性確認のため同じheadの失敗jobのみを一度再実行したが、初回の失敗は保持する。
待機時間・接続中ディスクの拒否・WSL全体設定は緩和していない。
人の操作が必要な受入確認はリリース後の項目として維持する。

## 通信コマンドの日英表示

`0387258d`は通信結果と次の操作を共通の日英案内へ揃える。
CLI・カタログ・relayの対象テスト11.76秒、変更lint26.63秒、ローカル全体test44.33秒、
race15.72秒、CLI E2E4.40秒、docs10.50秒、workflow policy1.58秒、
nativeテストのコンパイル2.00秒が成功した。日英のJSON一致、取り消し対象とaskルールの
範囲・期限、Host登録変更時の無関係なGit承認ルールと既定拒否、元のエラー詳細を確認した。
結果を書けない場合も操作を再実行しない。Linuxで実際にloopbackのTCP/UDP接続口を開き、
中止後に同じ表示アドレスへ再bindできることを確認した。上流への接続は行っていない。
これはローカルCLI・relayの証拠であり、導入済みの通信・外部サービス・VPN・人のUI操作の
確認ではない。構造化ログの項目やPolicy本体の実装は変更していない。
#701のhead `f354464337f84e876a97628ded69249fa84f13f6`から通常10バイナリの
Linux/Windowsパッケージ生成が45.28秒で成功した。導入・リリースは行っていない。
#701は#700に続く開発ブランチの追補である。

## 設定の日英案内

`c3fb376f`は設定のヘルプ・確認と保存後の操作・保存未確認と編集ファイルの案内を日英へ揃える。
対象テスト10.64秒、変更lint15.93秒、ローカル全体test36.01秒、race13.90秒、
CLI E2E4.23秒、docs9.80秒、workflow policy1.37秒、nativeコンパイル1.71秒が成功した。
日英の確認・適用JSONのバイト一致、Policyとrevisionの保持、競合時の編集保持、
元のエラー、表示失敗時にも適用が一度だけであることを確認した。初回lintが指摘した
診断出力5箇所の扱いを明示してから全体が成功した。実際のPolicy変更や導入済み・本人操作の
受入確認は、この検証には含まれない。

## 転送試験の準備待ち修正

同じheadで一度だけ再実行した#700も失敗した。Windows run `34993511328`、
job `104476272439`は通常のnative TCP転送で停止した。アプリ準備187ms、
Host準備26,577ms、待受28,015ms、native所有者確認38,859msで、41,375ms時点の
失敗にはアプリの`accept`の`TimeoutError`とバイナリ応答の不一致が含まれる。
試験用アプリの40秒の接続待ちをHostの準備前から数えており、実際の送受信前に
ほぼ消費していた。後続のLinux/公開容量回収・通知確認はSKIP、evidence job
`104483624401`も失敗した。初回の別の`compact_attached`失敗は解消扱いにしない。

#701の追補は待受準備とWindowsのnative所有者確認後に、共通の試験用アプリへ
開始を伝える。準備未完了は別の180秒の上限で失敗させ、従来の接続待ち40秒、
8並列のバイナリ送受信、片方向終了、製品の有効期間とキャンセルの確認を維持する。
実プロセス/TCPの回帰では接続待ち上限より長く準備を遅らせてから8応答の完全一致を
確認し、接続不足・開始なし・入力終了は失敗する。ローカルとリポジトリCIの共通試験へ追加した。

ローカル検証はアプリ回帰1.80秒、Windows観測回帰1.44秒、対象テスト11.36秒、
変更lint34.51秒、全体test96.28秒、race15.82秒、CLI E2E4.50秒、docs9.54秒、
workflow policy1.47秒、native試験コンパイル1.73秒が成功した。構成要素の証拠であり、
修正後の導入済みWindows手順は確認待ち。#701は#700の同一headを含むmain向けの
統合候補とし、その反映が証明されるまで#700はopenを維持する。

## Git packの順次転送候補

開発候補はhelperとtrusted agentの両境界でpack全体のJSON/base64保持を撤去する。
旧製品`e4cbd257`へ40 MiBのランダムデータを使う通常Git試験だけを追加すると、
`git pull --ff-only`で失敗した（コマンド10.02秒、試験9.19秒）。旧製品の通信方式と
Policyは変更していない。新実装では同じ通常pullと、別の拒否・内容固定の承認pushが
双方向それぞれ40 MiBの追加データで成功した。実際のUnix HTTP brokerに加え、
Host agentの形式もpipe越しに確認する。native Incusや認証付きリモートの試験ではない。
別途測定した単一fetch packの転送量は**41,956,043 bytes**だった。

最終のローカル検証はGit/Incus構成要素38.62秒、変更lint15.73秒、全体test56.13秒、
Git race71.11秒、CLI E2E6.89秒、docs12.96秒、workflow policy2.12秒、
nativeコンパイル1.84秒が成功した。最初のコンパイルで削除したバイト配列への参照が
照合処理に一箇所残っていることを検出し、初回の全体lintではcloseの戻り値7箇所と
表記13件を指摘した。すべて修正してから全体が成功した。追補後の文書確認も成功。

refごとの読み取り判断、push拒否、承認中の手元の変更から対象commitを固定すること、
新規ブランチの未作成lease、差分履歴の再利用を維持する。不正フレーム、上限超過、
入力終了・完了証明の不足、余分なデータ、バイト数不一致、出力失敗は失敗として確認した。
40 MiBの順次転送は代表的な巨大レポの速度・容量の証拠ではない。大規模計測と
導入済みproviderの受入は別途残す。[ADR 0106](../adr/0106-streaming-git-packs.ja.md)参照。

## セットアップ・通信・設定案内のmain統合

[#701](https://github.com/SLktEx/Hacocoon/pull/701)をmain
`f225e5c1a005358929a5c3bfe2b5154cc55a4ec6`へ統合した。同一head `161f3854`の
quality `35000716746`、test `35000716642`、Ubuntu `35000716870`、Incus
`35000716637`、Windows `35000716814`がすべて成功し、統合後のtreeも一致する。
#700の同一headが祖先であることを確認し、そのPRは統合済みとして閉じた。

Windows job `104488129559`とevidence job `104497872650`が成功した。通常導入・
再開、HTTPS/直接通信拒否、native SSH・実エディタ・TCP、Linux/公開容量回収、
native通知の確認経路が成功。転送アプリはnative所有者確認後の26,202msで開始した。
公開操作`{40F8CDCB-AE50-4141-BC3B-F5A1A64B2E14}`は2,683,305,984 bytesを回収。
Windows割当は7,629,438,976 → 4,946,132,992、仮想容量1,099,511,627,776は不変、
255回のopen確認後に圧縮が完了し、同じWSLが再開した。人による通知クリック・
新たなGUI回答とVPN/NRPTはSKIPのままリリース後に残す。過去の接続中ディスクの
失敗は区別して保持し、今回の成功から原因を推定しない。

後続のGit実装`68135b19`は、このmainへ載せ替える前の`ace86ee4`とtreeが一致する。
後者から通常10バイナリのLinux/Windowsパッケージ生成が42.32秒で成功した。
手元への導入・WSL停止・公開は行っていない。製品コードは全体検証した載せ替え前の
`9c9d2ed0`とも一致し、文書4箇所の競合は双方の独立した実績を保持して解消した。
