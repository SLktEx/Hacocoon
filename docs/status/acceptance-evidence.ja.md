# 検証証拠と未確認の範囲

[English](acceptance-evidence.md) | 日本語

状態: 検証記録。ここに記載した試験は過去のコミットで実施されたものです。文書整理時に実機試験を再実行したという意味ではありません。現在の機能は[実装状況](../IMPLEMENTATION_STATUS.ja.md)を参照してください。

成功・失敗・スキップは試験構成に結び付けて読みます。同じ実行内の一部成功や後続の成功だけで、別の失敗原因が解決したとは判断しません。日々の実行ログを追記するのではなく、判断を変える証拠と未解決事項だけを更新します。

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
| `72058fc` / `dev/2.x` | 専用Incus/Btrfsで合成外部IPv4/IPv6・Physical Host・Env間のTCP/UDP、Hostからの転送が成功（各経路0.099〜0.169秒）。期限、失効、ポリシー期限、生成ID置換の拒否、DNS固定を確認。初回DNS fixtureはcontroller準備前に失敗し、読取りの準備待ちで順序を修正。公開Internet・VPN・本番サービスの確認ではない。 |
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

## main統合後の実Base build fixture失敗

#616 head `6d5a713f`のUbuntu `34742660449`はPASS。Incus `34742660441`は
standaloneとCore egress／lifecycle、owned-Btrfsの通常create／run、trim、tree capture、
aggregate snapshot／CoW、native volume importがPASSでした。後続Base build
（job `103684889525`）はJSON読取りで`invalid character 'b' looking for beginning of value`
となりFAIL。main #602で人向け出力が既定になった後も、試験のCLI呼び出しが`--json`を
付けていませんでした。workflowのcleanupは両方PASS、後続persistent-copy／Store整理と
private registryはSKIPです。Base buildの受入完了とは扱いません。

`codex/base-build-json-fixture`はE2E呼び出しでJSONを明示し、同じbuild定義を使う
人向け／JSON出力のcontroller往復回帰を追加します。製品の出力・lifecycle権限は変えません。
Go 1.26.8の関連回帰と標準ローカルtestはPASS。実Incus Base buildと後続SKIPは再実行待ちです。
観測時の#616 testはqueued、Windowsは実行中で、#619の新runも未完了でした。

## client TCP転送候補

実装 `640c66ff4ce98d50946a31c6b5c284c59a1c0890` はprivate controller byte
sessionによるclient側TCP待受です。開発候補でありmain反映・配布済みではありません。

- Go 1.26.8のclient/control/API/provider/product集中試験と、独立Linuxコピーの
  標準`bash tools/ci-local.sh test`がPASS。明示的sessionキャンセル追加後も、関連
  client/control/controlapi/streamioのrace 5回反復がPASS。文書検査もPASS。
- 実TCP/UDS componentで8本同時のbinary往復、request EOF後のresponse排出、相手の
  EOF後も送信、接続拒否、世代交代、停止中／復旧待ちEnvの拒否、接続待ちの中断、
  EOFを無視するアプリでもキャンセル応答前にprovider socketを閉じることを確認。
- 独立試験コピーでbyte sessionを既存process用の暗黙EOF待ちへ戻すと、相手からの
  半切断回帰がFAIL。明示的byte sessionではPASSし、キャンセル追加前の関連race
  10回反復もPASS。意図した失敗はprotocolを分ける根拠であり、未解決の製品失敗ではない。
- Windows amd64実機の`internal/streamio`試験でTCP binary往復・半切断、接続待ち中の
  キャンセル、非loopback拒否がPASS。transport基本処理の受入であり、導入済みの
  Windows→WSL転送の受入ではない。
- 導入済みnetwork-security journeyへ、`haco env tunnel`で8本同時・各2MiBの往復と
  待受回収を追加。親試験が所有する使い捨てEnvと通常のアプリprocessを使う。
  この新しい実Incus経路は今回**未実施**。network例外やguest権限の抜け道は追加していない。
- checkpointツールは最初にWSLからWindows worktreeのGit管理pathを解決できず、変更前に
  FAIL。Windows側の公式lock helper内で同じ更新ツールを実行して成功。lockを迂回していない。

Windows側待受から`wsl.exe`を通す経路、汎用process caller統合、通常のネットワーク
以外の環境でのDNS mode／VPNはM3の残件。過去の実機FAIL／SKIPは維持する。

## WSLプロセス転送候補

実装`4b0b5baaf5a7acfdfabd94cea3c22f26745293c2`、
[PR #632](https://github.com/SLktEx/Hacocoon/pull/632)（親#626）の部分実装です。
main反映・配布済みではありません。

- Go 1.26.8集中試験と標準ローカルCIがPASS。stream・WSL起動・通知起動の
  関連race試験は10回PASSしました。
- 実子プロセスで2 MiB binary、子の終了後の応答保持、書込中断、並行closeと
  回収、異常終了とEOFの区別を確認しました。別のframe試験で両半切断順序、
  不正frame、期限切れを確認しました。
- Windows amd64実機試験がPASS。Windows→wsl.exe→隔離Linux試験プロセス→
  loopback TCPで2 MiB binary往復と半切断を確認しました。導入済みbinary、
  distribution設定、既存Env、ネットワーク設定は変更していません。製品の
  導入済みcontroller/Incus経路の受入ではなく、公開Windows待受と導入は未実装です。
- #626 Ubuntu 34744901263は新しい転送開始案内の照合でFAILし、binary往復前に
  停止しました。日英文の二重書式展開による接続先破損をCLI/controller回帰で
  再現し、一度だけ展開する修正後にPASSしました。実Incus再実行は確認待ちで、
  先行失敗を消しません。
- 途中の標準ローカルCIは、進捗ページから追記前の受入節へのリンクでFAILしました。
  全文書を揃えた最終コピーでは標準ローカルCIと文書検査がPASSし、失敗ログも
  保持しています。
- #623はtest 34743500916・Ubuntu 34743500914・Incus 34743500907がPASS。
  #626はtest 34744901265・Incus 34744901270がPASS。#619の先行Incus
  34743064668は#623で修正したBase build JSON fixtureと同じ理由のFAILでした。
  その時点の後続storage SKIPは過去のSKIPとして保持します。
- #623 Windows 34743500887と#626 Windows 34744901283はともにstep15の
  install/restart/reinstallでFAIL。WSLのsystemd user-session警告後に、正確な
  通常端末sessionの待機がtimeoutしました。後続native確認はSKIPで、原因は
  未確定です。#611の別の通知回答経路の失敗も未解決のままです。

最新の到達対象はM0〜M5全体です。Windows公開統合・DNS mode・通常のネットワーク
以外の環境はM3残件です。M1/M2の実機残件とM4/M5をcomponent成功で完了扱いに
しません。

## Windows公開転送クライアント候補

実装`700372233a9b470e06daa97433ce7c9948751497`、#632を親にした
`codex/windows-tunnel-client`で、Windowsの明示的な転送入口と配布・配置を実装しています。component確認をmain反映・配布済み・Linux入口の
自動委譲・導入済みWindows/WSL/Incus経路の受入とは扱いません。

- Windows amd64の実client/controller/TCPで、8並行×1 MiB binary、半切断、
  不正宛先の拒否、終了後の待受回収がPASSしました。
- PowerShell 7.6.6実機で両componentの新規導入・再導入、所有権不一致／欠落、
  checksum不一致、使用中worker、junction拒否がPASS。一時データのみ使用しました。
  PowerShell 5.1の`-File`は実行前にローカルのscript policyで拒否され、試験は
  **未実施**です。設定を緩和していません。
- 新規nativeヘルプ試験は、共通ヘルプへ切り替える際に古いmessage keyが残って
  FAILしました。修正後は日英ともnative再実行がPASSし、明示ヘルプがstdoutへ出て
  診断出力が空であることも確認しました。
- Go 1.26.8集中試験と最終標準ローカルCIはヘルプ修正・v0.66更新後にPASS。client転送のraceは
  10回PASSしました。広いcontrolapi全体のrace10回バッチは120秒の全体上限でFAILし、
  その時点の未変更import subtestは実行開始から1秒でした。ログを保持し、バッチを
  成功扱いにしません。分離した転送race10回と当該import race1回はPASSしました。
- Windows amd64/arm64ビルド、architecture別bundle・内部checksum・改変archive拒否、
  workflow policyがPASS。arm64での実行は未確認です。
- 親#632 `e7ca6735`はtest 34763683967・Incus 34763683968・Ubuntu 34763684021が
  PASS。Ubuntu job 103740815424で導入済みcontrollerの8並行×2 MiB転送・半切断・
  中断と待受回収のPASSを明示確認しました。#626の案内表示失敗後の修正を実経路でも
  確認したものです。前の失敗は保持します。Windows 34763683983は確認時点で実行中で、
  過去の起動・通知経路の失敗を解消扱いにしません。

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

<a id="windows-main-integration"></a>

## Windows接続候補へのmain統合

統合実装commit: `44a30b1a0f03f639f3df76eaf5730c37dc50e3ea`.

`codex/windows-main-sync`はmain `f47a9a41e5c175b8f7a4dca41680595be1687c99`を
PR #634候補へ統合しています。#631のSSH接続、#625のWSL起動修正、#612の共通build/cacheを
再利用します。導入済み統合候補の受入は未実施です。
初回統合試験はRelayの重複定義、次はcoreの重複importでbuild失敗しました。両方を修正して再検証しました。
既存のSSHとTCP両方のbyte処理を共用し、target権限を分離したまま、準備期限と稼働中sessionの
寿命を区別する回帰を追加しました。v0.66を維持し、main反映・配布・M0〜M5完了とは扱いません。

親 #632 `e7ca6735`はtest 34763683967、Ubuntu 34763684021、Incus 34763683968がPASS。
Windows 34763683983/job 103740781508はstep15（install・terminate/restart・reinstall）でFAILし、
後続egress・SSH・TTY・reclaim・通知はSKIPでした。mainの起動修正と同じ原因とは断定しません。
#634 `422a8f80`はtest 34765863859、Ubuntu 34765863849、Incus 34765863870がPASS。
これらは個別候補の証拠であり、今回のmain統合候補やWindows実経路の成功を代替しません。

統合後のローカル試験は、Go 1.26.8のcontrol/controlapi/client/clientforward/sshclient/streamio/Incus回帰、
標準 `bash tools/ci-local.sh test`（Go 1.27.1、shuffle 615）、関連stream race 5回、
文書と18件のchecker回帰、CI契約・policy、installer packageがPASSです。
Windows PowerShell 7.6.6ではWSL停止観測、両companionの導入・再導入・所有不一致・固定worker・junction拒否がPASS。
このPCのPowerShell 5.1追加実行は今回未実施で、以前のpolicy拒否を成功へ変えません。

新しく取り込んだlogin PTY試験は全体実行で5秒の終了待ちに一度FAILし、変更前の単独実行ではPASSしました。
fixtureのprivate login profileで実際のBash入力待ち表示を観測してから入力するようにし、
同じ5秒の合計期限・15秒の外側期限で10回PASS、標準ローカル試験もPASSです。
製品の親process選択・Host setup・隔離は変更していません。初回の失敗でPTY transcriptを採取しておらず、
入力消失などの原因を断定しません。過去の実Windows起動FAILの原因証明や受入の代替でもありません。

実PTYのresize・SIGWINCH・切断はraceで10回PASS。統合候補をWindows amd64へbuildし、実Windowsでclient/controller/TCPの8並行1MiB往復・半切断・cancel・待受回収、日英help・不正引数拒否がPASSです。新規WSL/Incusへの導入、通常入口からWindows待受への自動委譲、arm64実行、fresh GUI回答は未確認です。

`44a30b1a`のGit archiveを新しい一時領域へそのまま展開し、記録された実行属性のまま標準ローカルCI全体を再実行してPASSしました。

<a id="windows-tunnel-delegation"></a>
## Windows転送の自動起動候補

実装commit: `bc8b915b93331eeb6a746e9826cf4b3c2c5b7d8b`。

`codex/windows-tunnel-entry`は通常WSL／trusted Hostからの自動委譲、登録先・導入世代・Env世代の照合、
元の期限と親pipeによる寿命管理を実装しています。開発ブランチの候補でありmain反映・配布ではありません。
最初の集中試験は補助関数の引数削除後に一つの呼び出しが残ってbuild失敗しました。修正後に再検証し、初回失敗を保持しています。

Go 1.26.8の集中回帰がPASSです。実Windows amd64の構成要素試験で不正要求、導入先／Env置き換わりの待受前拒否、
親EOFと余分なbyteによる回収、実子プロセスの1 MiB通信中キャンセルと転送先・待受・子の終了がPASSです。
既存の8並行1 MiB／半切断回帰もPASSです。制御されたcontroller fixtureによる確認です。

導入済み経路はローカルで**未実施**です。現在の`hacocoon-second`はWSLInterop登録がなく、読み取り専用の存在確認は終了1でした。
Windows利用者領域の`Hacocoon/client`もありません。試験を通すための導入・修復はしていません。
保守対象のnative SSH試験には、同じ試験所有Envで`windows-tunnel-entry-e2e.py`を実行する経路を追加しました。
通常端末のコマンド、Windowsプロセスが所有する待受、8並行1 MiBの半切断往復、Ctrl+C後の回収を確認する設計ですが、
新しい導入済み試験の実行は残件です。arm64実行と人の新規GUI回答も未確認です。

親 #635 `bc28c32b`はtest34768317313、Ubuntu34768317315、Incus34768317317がPASSです。
Windows34768317303は実行中であることを確認した段階で、成功とは扱いません。過去のWindows失敗も未解決として保持します。

最終候補のコピーはGitの実行属性を保持し、Go 1.26.8のclient／製品集中試験、関連race10回、
標準`tools/ci-local.sh test`（Go 1.27.1）、文書とchecker回帰、workflow policyがPASSです。
実Windows amd64の回帰と日英companionヘルプも再度PASSし、amd64／arm64のbuildがPASSです。
Linuxの所有記録回帰は欠落・不一致・symlink・FIFOを停止せず拒否します。
新しい導入済みdriverはローカルでは構文確認までで、実経路のPASSではありません。

親Windows34768317303はjob103753188863のstep21（native通知回答）で**FAIL**しました。
step13〜20のPowerShell 5.1導入部品、通常install／restart／reinstall、egress、native SSH／VS Code、
一時TTY、Linux回収と公開reclaim／保持データ復元はPASSです。VHDX割当は7,931,428,864から5,040,504,832 bytesへ減少しました。
失効要求の試験は期待どおり終了1でしたが、期待した拒否文がなく、`stage=clear`、`reason=unavailable`、`native=unrecorded`でした。
原因と新規通知回答は未解決です。後続の起動成功で過去のFAILを消さず、今回の新転送経路の受入へも読み替えません。

その後、正確な`bc8b915b`のGit archiveによる追加確認で、変更した転送packageはPASSしましたが、
既存の`TestLoginBootstrapPTYDoesNotStartHostSetup`はprivate Bash入力待ち表示の5秒期限にFAILしました（試験全体6.90秒）。
この試験はPTY transcriptを残しておらず、原因は未解決です。変更せずGo 1.26.8で単独10回はPASSしましたが、
先の組み合わせ実行のFAILを消さず、修正済みとも扱いません。先に成功した標準ローカルCIとも分けて記録します。
commit済み文書、既存native-access driverの5回帰、新driverのPython構文確認は独立してPASSしました。
製品／試験の期限やplatform設定を緩めていません。

<a id="native-toast-process-diagnostics"></a>
## native通知プロセスの診断

実装`8eeac2b8786a476277080568a32a79dcf39fced4`の
[PR #640](https://github.com/SLktEx/Hacocoon/pull/640)（base #638）は、#635のWindows34768317303、job103753188863のstep21
（`stage=clear`、`reason=unavailable`、native番号なし）を調査しています。
ローカルの隔離probeでは定数を返すだけのPowerShell methodも`MethodInvocationNotSupportedInConstrainedLanguage`で失敗しました。
隔離の外で同じ固定probeを実行すると、定数応答2329 ms、最初の空履歴clear4281 ms、再clear3469 msで全て終了0でした。
言語モード・実行policy・通知設定・期限は変更していません。今回はローカルの隔離制限を区別できましたが、
CI失敗の原因証明ではなく、過去のローカルPowerShell policy拒否と同じ原因とも断定しません。

別の決定的な不具合として、旧結果分類が`context.Canceled`／`DeadlineExceeded`を一般的な利用不能へ失うことを再現しました。
追加回帰は修正前FAIL、修正後PASSです。描画処理はこれらの理由と安全な数値の子終了値・経過時間を保持し、
起動した子の終了を待ちます。停止前の成功markerでキャンセルを成功へ変えません。
導入済み失敗観測も数値を表示しますが、子の生出力は出しません。8秒の描画上限・回収・認可は変更していません。
Windows amd64の回帰は実子の停止、native失敗番号、非公開出力を表示しないことを含めPASSです。
実表示履歴、導入済み失効要求／新規回答はそれぞれ別の受入であり、この回帰で代替しません。v0.67を維持します。

明示実行した実Windows履歴確認はtoolの隔離外で29.41秒でPASSしました。
最初の所有する空履歴clear、日英の選択XML、履歴取得と削除を確認しました。人によるクリックと見た目は未確認です。
Go 1.26.8の対象テストとrace 10回、Windows amd64テスト、Windows arm64ビルドはPASSです（arm64実行は未確認）。
標準ローカル全体テストでは既存のlogin-bootstrap PTY入力待ち期限が再度FAILしました（試験全体6.73秒）。
全体PASSとは扱わず、以前の失敗も未解決です。期限やplatform設定は緩めていません。
文書整合性とcheckerの18回帰は独立してPASSしました。

親#638の`3ed748bf44e11839f6c6c0e07fb6de29746154d4`はtest34770808202、Ubuntu34770808200、
Incus34770808191がPASSです。Windows34770808189、job103759936727はstep17の新しい通常入口tunnel driverでFAILしました。
待受アドレス表示の後、driverが端末sessionの一致を待つ間に時間切れになりました。
先行するnative SSH、VS Code、転送確認は成功を記録していますが、tunnelの所有確認・転送・中断の受入は未完了です。
通知回答を含むstep18〜21はSKIPでした。このrunで以前の#635通知clear失敗を解決済みにしません。

<a id="windows-tunnel-interruption"></a>
## Windows転送の端末中断

`codex/windows-tunnel-cancel`の実装`297b30954153c9699a7aac356ddfca3030c57f6a`は、#638の待受表示後のCtrl+C失敗を扱います。
Windows34770808189のdriverはnative所有者確認、8並行1 MiBの半切断往復、application終了確認を通ってから
中断を送っています。その後の端末結果が必要な0ではなく`TUNNEL-EXIT:1`だったため、回収までの受入はFAILです。

Linux回帰は専用process sessionを所有し、実子とcontroller経由で1 MiBを転送してから前面groupへSIGINTを送ります。
旧起動方法は`signal: interrupt`でFAILし、pipe経由の正常キャンセルより先に子が終了しました。
interop子のgroupを分ける修正後は、子・転送先・待受の回収を含む転送packageのrace 10回がPASSしました。
終了値の成功への置換、期限・認可・interop設定・既存Envの変更はしていません。
ローカルの信号所有の不具合を確認した証拠であり、導入済みWindowsのCtrl+C再受入はまだ未確認です。
標準ローカル全体では既存login-bootstrap PTYの入力待ち期限が再FAILし、以前の失敗も未解決として残します。
今回の試験全体は6.68秒でした。実Windows amd64では、対象世代の置換拒否、親EOFと余分なbyte、
1 MiB転送、子・転送先・待受の回収が独立してPASSしました。Windowsビルドと文書checkerの18回帰もPASSです。
v0.67を維持します。中断処理の修正であり、M3完了や配布ではありません。

main `266cc47f0d1e898b92a4f700f02a7e02e9c7e412`を`e82db9607b0a3973b986fe002fc6af0ea4b2be59`で統合し、
#637の信頼済みLinux Go cacheと#639のWindows受入時amd64のみのbuildを再利用しました。
生成前の確認を、この候補の`haco-tunnel`を含む10本へ合わせています。実PowerShellの生成結果はarchitecture以外の配布設定を保持し、
既存workflow-policy確認もPASSしました。配布そのものは両architectureを維持します。新しい導入CI所要時間の実測ではありません。

親#640 `b0ab1858b67278e3b070e9c3d339abbddd28416b`はtest34772703642、Ubuntu34772703628、Incus34772703619がPASSです。
Windows34772703625/job103765084905はstep17で再び`TUNNEL-EXIT:1`となり、step18〜21はSKIPでした。
通知診断の修正について、導入済み受入の成功が増えたわけではありません。

別のGo 1.27診断用コピーはfixtureの5秒上限を維持し、固定の観測値だけを追加しました。
最初は7.15秒でFAILし、Bashは生存、PTY出力118 byte、入力待ちmarkerなしでした。続く2回はPASSです。
失敗時も分岐先は既にBashへ到達していました。profile／端末の生出力は出さず、製品や期限も変更していません。
表示が来ない原因は未解決であり、toolchainの不具合と断定しません。


PR #642の`750b7e069337561bfbdc76e1824eb7482a127cba`は、test34774457312・
Ubuntu34774457603・Incus34774457355がPASSしました。Windows34774457337／
job103769879451はstep13〜20がPASSし、通常のnative SSH／VS Code、自動転送の
`TUNNEL-EXIT:0`と回収、一時TTY、公開reclaimを確認しました。#638／#640で失敗した
中断処理は導入済み経路でも修正を確認できました。step21の通知回答は依然FAILで、今回は
`stage=activation`・`reason=unavailable`、native／子の終了値／経過時間は未記録です。
以前のclear段階の失敗と同じ原因とせず、人の新規通知回答も確認済みにしません。
過去の失敗記録はそれぞれの確認範囲とともに保持します。


<a id="guest-packer"></a>
## Env内のPacker構築

`codex/packer-base-build`候補は、実Packerの呼び出しとHCL2・別ファイルのshell転送を
共通Baseサービスへ追加しています。ローカルGo 1.26.8の部品・CLIテストで、段階の順序、
失敗時の停止、入力上限、リンク・特殊ファイル拒否、正確な一時Env削除、明示指定時だけの
非公開出力を確認しました。これらはPacker本体を実行しておらず、導入済み受入の証明ではありません。

既存`hacocoon-second`のWSL／Incus 6.0.5では、controllerはv0.57の
`ac67fadbe1a3b2359e6b41438e3f34d392f49a78`です。旧Baseコマンドは新しいHCL要求と
`--json`引数に未対応で、引数拒否時には何も作成していません。その後、通常の旧Base定義から
候補のゲスト準備・Packer各段階を実行しました。これはHCL→JSON変換による製品実装ではなく、
旧導入版での実行確認です。専用名`packer-check-f33f425edda2`は準備時に終了90でFAILし、
固定の数値分類による再試行でOpenSSH不足（171）を確認しました。いずれも残ったbuilderなしの
failed結果でした。候補には、通常のplugin処理として一時Env内で依存ツールを準備する処理を追加しています。

修正後の通常ビルド`packer-check-69f98b715ee0`も依存ツール準備でFAIL（終了89）しました。
同じ依存スクリプトを通常の`haco run --no-oci --json`で分離確認すると、ゲスト終了100で、
Ubuntuのarchive／securityへのHTTP取得が既存proxyから403で拒否されていました。
結果は`cleaned_up: true`で、確認時に承認待ちはありませんでした。特定のpolicy・送信元照合が原因と
断定しません。許可ルール追加・送信元登録の修復・通信制限回避・製品経路外でのパッケージ注入・
導入済みcontrollerの置き換えはしていません。実Packerの完走、Base公開・revision再利用、
外部plugin失敗、arm64実行、新しいWindows→WSLコマンドの導入済み受入は未確認です。
過去の旧Base作成の受入は、それぞれの記録された範囲で保持します。


実装commitは`1103505b14dedb76151056cc8cf8951435bc3867`で、開発チェックポイントを
v0.68へ進めました。M4完了や配布を意味しません。Go 1.26.8でBase／Packer／制御API／
構成／責務境界とCLIのファイル転送・出力の集中回帰、Base／Packerのrace 5回、CLIの入力・出力の
raceがPASSしました。文書整合性と18件のchecker回帰もPASSです。標準ローカルテスト
（Go 1.27.1、shuffle 615）は既存の`TestLoginBootstrapPTYDoesNotStartHostSetup`でFAIL
（6.64秒）しました。今回はBashの入力待ち表示を観測しましたが、共通の5秒期限の残りが約42 msで、
終了37を待つ処理が時間切れになりました。以前の入力待ち表示が来ない失敗も未解決のままです。
全体を成功にせず、以前の失敗の修正確認にも読み替えません。期限は延長していません。


<a id="environment-owned-data"></a>
## Env用使い捨て領域のライフサイクル

`codex/cache-env-attachments` の実装 `c4b7af503f77a6f02e08bb23529bd1f79c8e64a8` で、複数領域の一括予約、元世代の記録、
作成完了、Env実体不在の記録と子領域の共通回収を追加しました。v0.70は **partial** です。
製品の対象選択とIncus配置は未有効化で、通常Envのキャッシュ利用・指定パス収集・
追加領域のsnapshot/transfer・新しい実機受入の成功とは扱いません。

Go 1.26.8のCore/state/保存領域/Workspace/責務検査/Incus集中回帰がPASSしました。
実際のカタログとサービスを、模擬した基盤資源につないで、一括予約、重複・同時取得、
作成・検証の失敗、元世代の更新・リセット後の寿命、結果不明と完了済みのコピー、
Env・子領域の削除失敗と再試行、Workspace・OCIの保持、不正カタログの拒否を確認しています。
Environment関連のrace 3回もPASSです。

標準ローカル `tools/ci-local.sh test`（Go 1.27.1、shuffle 615）は、既存の
`TestLoginBootstrapPTYDoesNotStartHostSetup` でBash入力待ち表示を観測できず、6.70秒でFAIL。
その他のGo packageはPASSでした。その実行の後続vet・通知・packagingはSKIPですが、
独立したvet、JavaScript構文3件、通知32試験、packaging 2試験はPASSです。
以前のPTY失敗は未解決で、全体CI成功には読み替えません。

全体をWindows向けにビルドした試行は、変更していないLinux専用部分の
Incus Hostの `syscall.Stat_t` と製品CLIの `clientforward.DesktopCommand` でFAILしました。
配布設定のWindowsクライアント `haco-review`・`haco-wsl`・`haco-tunnel` と、変更した
共通Core/state/保存領域/Workspace packageのWindows amd64向けビルドは別途PASSしています。
Windows上での実行や新規の通知回答は未実施です。

文書整合性と18 checker回帰がPASSしました。初回のソース照合はexport・作業コピーの改行差でFAILし、
製品コードの内容差は残りませんでした。文書10件の改行だけをそろえ、文書と責務検査を再確認した後、
commit内の1,484ファイル全てがbyte単位で一致しました。最初のrace呼び出しはWSLへの引数受け渡しで失敗し、
製品試験は未実行でした。修正後が上記3回のPASSです。試験期限・権限・隔離は緩めていません。

親PR #644の `d29ce0a4061a4e4c1d107d4f3a65714b9775b665` は、
[test](https://github.com/SLktEx/Hacocoon/actions/runs/34782161147)、
[Ubuntu](https://github.com/SLktEx/Hacocoon/actions/runs/34782161107)、
[Incus](https://github.com/SLktEx/Hacocoon/actions/runs/34782161134) がPASSしました。
[Windows](https://github.com/SLktEx/Hacocoon/actions/runs/34782161108) のjob 103791018485は、
step13〜20の通常SSH・tunnel終了0・一時TTY・reclaimがPASS。
通知step21は再びactivationの `reason=unavailable` でFAILし、native/child_exit/durationは未記録です。
以前のclear失敗、日本語Windows、人のGUI回答、元のSSH障害の再現を解決済みにはしません。
以前失敗したnative cache fixtureと、残した作成途中の試験領域の復旧・回収も未解決です。
