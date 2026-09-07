# 実装状況

## VS Code の信頼された承認画面

状態: **repository 実装済み、ロードマップ D2 は partial**。
任意の UI 拡張は Review または command palette から通常の承認 CLI をローカル専用 terminal で開きます。
実行先と環境を固定し、remote/web・信頼しない window を拒否、回答は既存 CLI で入力します。
VSIX 作成に npm download は不要です。関連 JavaScript 26 件は成功しました。
実 VS Code GHA に local terminal → installed controller の古い要求の拒否を追加しましたが結果は未確認です。
OS 通知からの起動と、新規要求への人間の実回答はこの段階では証明していません。
[契約](design/pending-approval-review.ja.md) と [ADR 0029](adr/0029-local-desktop-approval-review.ja.md) を参照してください。

`0754280` の test 34161070477、Ubuntu 34161070466、Incus 34161070522 は成功しました。
Windows 34161070471 は実 VS Code、preview/Edge、doctor 全 4 項目が成功しました。
承認受け入れだけが review 前の Python 準備で失敗し、cleanup は成功しました。
setup unit・package・DNS 等を生出力なしの固定分類で識別する診断を追加しました。
原因は未確定で、これは SKIP ではなく FAIL です。診断を追加した再実行は未確認です。


生成した任意拡張の VSIX は archive/manifest 検証と、実ローカル VS Code の独立 profile への
インストールに成功しました。これは package の確認であり、新規承認や local terminal/controller の往復の証明ではありません。

## 承認待ちの確認

状態: **repository の一段階を実装済み。ロードマップ D2 は partial です。**
haco approve は候補が一つなら直接表示し、複数なら番号で選択できます。
Standard queue が background の待機を制限し、共通の private review API は
元の Git 要求も扱います。単発回答、6 種類の保存、キャンセル、期限切れ、
二重回答、session の完了所有権、Policy 変更、失敗時の安全な receipt、
実際の local Git helper 経路は関連 race／vet で成功しました。
通知から開く操作は planned です。[契約](design/pending-approval-review.ja.md)を参照してください。

installed GHA に、通常の設定操作と実際の HTTPS を使う ask 保存・今回拒否・
単発許可・再確認／拒否・対象を限定した cleanup を追加しました。実行結果は下記のとおりです。preview／doctor の失敗は、生出力を使わず固定 phase と数値を記録します。

f6d193b の test 34154746874、Ubuntu 34154746842、Incus 34154746852 は成功しました。
Windows 34154746844 は実際の VS Code、SSH、設定、project setup、doctor が成功し、
HTTP preview が失敗しました。正確な原因は未確定です。

`5ad8c3e` の Windows run 34159087435 は ask 保存・今回拒否、preview の
setup/open、doctor 呼び出しで失敗しました。実 VS Code、SSH、設定、project setup は成功。
test 34159087438、Ubuntu 34159087434、Incus 34159087447 とローカル test/E2E・docs は成功。
承認 fixture は通常の setup で Python を準備し、正常な完了行を受け入れるよう修正しました。
この修正の installed 再検証は未完了です。

ローカルの installed `71dbb4f` で Windows native SSH、接続先キー不一致の拒否、
cleanup が成功しました。port 33105、Environment `win-ssh-67210d9ab7994c7d` を使用し、
一時 Policy・接続・Environment・Workspace を削除、listener 不在と再接続拒否を確認しました。
先行する 2 回は Host 停止により失敗し、成功した実行は通常 Host ターミナルを開いたまま行いました。
自動 desktop setup は disposable GHA profile 用のためローカルでは SKIP です。
これは installed snapshot の SSH 検証であり、新しい承認 review や新たなローカル VS Code の成功ではありません。

## 承認要求の照合

状態: **照合の基礎は実装済み。ロードマップ D2 は partial です。**
承認画面・信頼された controller の応答・Git の承認待ち情報に、
capability の監査・実行結果と同じ request ID を渡します。
コマンド・承認権限・通知からの操作 endpoint は追加していません。
[Interaction Event](INTERACTION_EVENTS.ja.md) を参照してください。

`2584ec6` の GHA は test 34152700790、Ubuntu 34152700745、
Incus 34152700884 が成功しました。Windows 34152700897 は native SSH、
実際の VS Code 接続、設定 round-trip、project setup が成功し、HTTP preview と
Environment doctor が失敗しました。失敗の正確な原因は未確定です。
以前の installed 成功は別の検証結果として扱い、失敗した項目の成功とはしません。

## 承認方針の設定編集

状態: **repository の一段階を実装済み。ロードマップ D は partial のままです。**
`haco config` と任意の `--edit`／`--file` で、通常の承認保存と同じ Policy を扱います。
revision の確認と共通の private writer により、同時に保存された変更を上書きしません。
監査には操作・revision の情報だけを記録します。関連 test／race／vet と製品 CLI／
controller E2E は成功しました。設定 round-trip の installed 受け入れは 2584ec6 で成功しました。
[設定](reference/configuration.ja.md)を参照してください。

ローカル `71dbb4f` の通常インストールは Host 診断 6 項目が成功し、config の取得・反映・
receipt・実ファイル revision・監査を照合しました。default deny と元の 8 ルールは維持しました。
空の saved_decisions 配列が保存時に省略され、JSON 表示の一致は FAIL でした。
snapshot 表示の正規化と component／CLI 回帰テストを追加して成功していますが、修正の
installed 受け入れは未確認です。[正確な観測](reference/configuration.ja.md#ローカル-installed-での観測)
を参照してください。

`71dbb4f` は GHA 全 4 系統が成功しました。test 34151576434、Ubuntu 34151576429、
Incus 34151576447、Windows 34151576493 です。Windows は通常 config の往復、
実際の VS Code・project setup・Edge preview・Environment doctor を含みます。

別のローカル `71dbb4f` 検証では、`haco config --file` で `preview-71dbb4f` だけに
Ubuntu archive の一時ルール 4 件を追加・削除しました。通常の project setup で
loopback HTTP server を起動し、Windows が port 36059 で正確な Workspace marker を取得しました。
preview の再利用・閉鎖後の拒否と、runtime／Workspace／DNS の doctor が成功しました。
このローカル probe には SSH 接続を用意していません。marker・recipe・Environment・
listener を削除し、default deny・元の 8 ルール・保存方針 0 件を確認しました。
既存 Workspace `git-save-eb16300` は保持しています。過去の preview／doctor 失敗は
この実行では再現せず、元の原因は未解明のままです。

`729f008` の GHA test 34149690153、Ubuntu 34149690280、Incus 34149690192 は PASS。
Windows 34149690178 は DNS・desktop SSH／再開・実際の VS Code・project setup が成功し、
preview と Environment doctor が失敗しました。変更した fixture は両方を実行し、
最終 job を失敗に保ちました。正確な原因は未解明です。

## 通常 Git 承認の方針保存

状態: **repository の一段階を implemented。D1／D2 は partial**。既存 Git
approve／deny に任意の --save で env／全 env の allow・deny・ask を接続しました。
pending は今回の exact commit と provider が定義する再利用範囲を分けます。
OID と operation ID だけを wildcard にし、repository・remote・ref・fast-forward
update kind と属性名完全一致を維持します。永続化・監査済み応答を確認し、非対応 peer
は拒否します。実ローカル Git helper で次 commit、ask、deny、history rewrite 拒否を
確認しました。`eb16300b6700` の installed Windows／WSL で通常 SSH、ask 方針の
保存、GitHub push、次 commit の再確認、拒否時の remote 不変を確認しました。
正確な commit と保持資源は[管理 Git の検証](reference/managed-repository-workflow.md#installed-saved-approval-acceptance)
に記載しています。
[ADR 0026](adr/0026-reusable-git-approval-scope.ja.md) を参照してください。

`eb16300` の GHA test 34146281274、Ubuntu 34146281278、Incus 34146281289 は
PASS。Windows 34146281264 は FAIL です。DNS・通常 SSH・再開は成功しましたが、
VS Code が 10 分以内に完了しませんでした。後続の setup／preview／doctor は未実行です。
失敗を SKIP や現行 editor の受け入れ成功として扱いません。
検証 fixture を変更し、editor 失敗後も独立した setup・preview・doctor を実行します。
各失敗は最終 job 結果に保持します。ローカルの PowerShell 構文確認は成功しましたが、
変更した fixture の GHA 実行結果は未確認です。

953d1e5 は全 4 GHA workflow が PASS しました。修正した orchestrator／crash fixture
も含みます。それ以降の変更の受け入れを証明するものではありません。

## Policy に従う名前解決

状態: **ロードマップ C3 は partial**。installed Standard mode では canonical な
Environment 作成・再開時に guest loopback DNS service を自動導入します。
UDP/TCP の bind 後に readiness を通知し、導入・起動失敗時は成功を返しません。
bare controller mode では component は optional のままです。自動導入のために
新しい command、nameserver 引数、allow rule は不要です。名前解決そのものには
専用の Policy 許可が必要です。

既存の隔離された listener が永続化された送信元を識別し、Capability Policy と監査の
後で Physical Host resolver を使います。接続権限は別です。Windows、WSL、
trusted Host、Environment の通常 getaddrinfo と default DNS 拒否を比較する GHA
fixture は `c05528a` の Windows run 34132173483 で成功しました。
VPN/NRPT、DNS 変更・再起動後の反映は未検証です。[名前解決](design/name-resolution.ja.md)を参照してください。


`72096d8` のローカル test/vet/docs/e2e と関連 race は成功しました。
GHA の test と Incus は成功し、Ubuntu run 34121278716 と Windows run 34121278578 は
Environment DNS service の設定中に失敗しました。新しい DNS fixture には未到達です。
古い installed substrate 上の独立したローカル probe では同じ DNS unit が起動しましたが、
GHA の失敗の再現・原因の説明にはなりません。probe は canonical に削除し、
空の Workspace も削除しました。adapter は任意の guest 出力を公開せず、
許可した処理段階と数値の service 終了コードだけを返す診断を追加しています。
`a1d084b` は Ubuntu installer・Incus・test が成功し、Windows は job 制限時間で CANCELLED になりました。
`c05528a` も test・Ubuntu installer・Incus は成功しました。Windows run 34132173483 は
DNS の一致・default 拒否と VS Code 実接続に成功し、その後の project setup 検証で失敗しました。
PowerShell で生成した Bash script の CRLF により、setup 実行前の `set` が終了コード 2 を返しました。
setup と preview の script を LF に正規化しています。setup・preview・Environment doctor の実機検証は再実行待ちです。

C4 の[プロジェクト setup](design/project-setup.ja.md)は Workspace ごとの recipe を
`haco setup --script <path> <environment>` で保存・実行し、再実行・削除する部分を実装済みです。
Host recipe は既存の挙動を保ちます。起動前と実行 lifecycle lock 内で対象 identity を確認し、
script は上限付き stdin で渡します。関連 race test は成功しました。
installed GHA は `c05528a` で setup 実行前の検証スクリプトが失敗しました。
package 導入と実際の cancel cleanup は未検証です。

## 現在のdesktop開発checkpoint

状態: **ロードマップ C は partial**。desktop SSH の準備、`haco open [--client vscode|ssh] [environment]`、
保持した環境の再開、読みやすい対象一覧を提供します。Host の保存手順は `haco setup --script <path>`、
再実行、`--clear-script` で implemented で、bcc1baf のインストール済み GHA も成功しました。
一時実行 CLI は実装済みで、4adfe19 の実 Incus 検証も成功しました。より広い C1 の対象選択、C3–C5 と後続段階は未完了です。

`4f1f512` では4つの GHA workflow が成功しました。Windows job は通常の `haco open` から実際の VS Code
1.136.1 Remote-SSH に接続し、document の読み書き、terminal 実行、検証用ファイルの削除を確認しました。
使い捨て portable profile で Linux platform を保存し、Workspace trust prompt を無効化した構成です。
通常の desktop prompt や全 Windows/VPN 構成の確認ではありません。
以前の `703ec76` は executable の探索で失敗し、`506c38f` は起動後の editor 検証待ちで timeout しました。
これらの失敗と、その後の成功は区別します。

ローカルの通常 installer による最終更新は `8752431`（v0.33、build `2026-09-07T06:44:17Z`）です。
doctor の6項目が成功し、`stage-b-git-dev` と Workspace の登録を保持しました。Installer ZIP SHA-256 は
`c2c5b720643d98e586996e2d2413d1af196d764331e2b160a5bda647c76946a9` です。
この installation は現在の DNS 変更を含みません。2026-09-07 にユーザーが
`desktop-8752431` の一時 package-egress rule を許可した後、ローカル検証が成功しました。
Windows 標準 OpenSSH と VS Code 1.136.1 Remote-SSH で Workspace marker、
editor の読み書き、remote terminal、probe 削除を確認しました。専用の別 profile と
明示した Remote-SSH URI を使った検証であり、ローカルの通常 `haco open` の検証では
ありません。通常の `haco open` は別途 GHA で確認済みです。
追加した 4 rule は削除済み（残り 0）、SSH 接続 `ssh-39493` は解除済み、
Windows の専用 key は削除済み、検証 Environment は停止済みです。
`stage-b-git-dev` は停止状態を維持しました。最初の再開は WSL 再起動後に volatile
source guard が消えていたため失敗し、canonical な削除・作成で検証環境を作り直しました。
古い /tmp Workspace も存在せず、新規の検証 directory を作りました。
この失敗と、その後の接続成功は区別します。欠落 guard の再開修正には回帰テストを
追加しましたが、その修正の実際の再起動検証は未完了です。

## 永続Storeの独立コピー

状態: **storageの実装単位はimplemented、改訂B4全体はpartial**。
最新main `3b2d0b6`（PR #481 merge）を基準に、既存の
`haco plugin oci store create dev --from shared` で未接続Storeを独立複製する。
コマンド群は増やさない。正規catalogでコピー元を予約し、provider完了と検証後の
公開まで接続・削除を拒否する。失敗時は所有権を保持し、手動recoveryが必要。
中断コピーの自動回復は未実装。[Store契約](design/persistent-oci-store.md#independent-offline-copies)参照。

ローカルの実Incus 6.0.5-8 / WSL / Btrfsで合成データの受入が成功。
Parent UUID一致、両方向の書込み独立、元Store削除後のコピー保持、検証用volume・
project・poolの削除を確認した。OCI image/runtime、配布済みCLI、trusted Hostからの
publicationの成功は意味しない。初回E2Eはpool確認前にテストprojectを作っていない
fixtureの問題で失敗し、修正後に成功した。RPC回帰では不正引数がinternal errorに
分類される問題を検出し、handlerを修正した。

A/B成果は保持。PR #481は`3b2d0b6`としてmerge済み。最終`0b79cac`の
[test](https://github.com/SLktEx/Hacocoon/actions/runs/34081379821)、
[Ubuntu installer](https://github.com/SLktEx/Hacocoon/actions/runs/34081379810)、
[Incus](https://github.com/SLktEx/Hacocoon/actions/runs/34081379802)、
[Windows installer](https://github.com/SLktEx/Hacocoon/actions/runs/34081379870)
は成功済み。今回のコピー変更のCI結果とは区別する。新しいstorage testは既存Incus CIへ追加。

今回のSKIP: trusted Hostからのimage取得/publicationとコピーしたimageのcontainerd/Docker
実利用（その製品経路は未完成）、VS Codeでの実開発（今回IDE操作なし）、今回のWindows
配布物受入（installed productは前のB候補のまま）、新しいGit実push（Git・認証・refの
変更はなく、storage受入にrepoを使わない）。下記Bのpush OIDは過去に照合済みの結果であり、
今回pushしたものではない。C-Gは更新した[ロードマップ](status/architecture-and-roadmap.md#user-facing-development-order)に沿うplanned項目。


## Storeコピー実装単位の検証

ローカル成功: 維持CIの`test`（Go test/vet・installer component・JavaScript）、
`e2e`のcommand/capability/Git/orchestrator assertion、docs/workflow policy、
実Incusの合成データCOW検証。`forwarding`は最初sudoの認証で失敗したが、開発用WSLで
同じ隔離namespaceテストをrootとして実行して成功した。初回E2Eは本体assertion成功後、
一時Go module cacheの権限でcleanupエラーが出た。製品assertionの失敗とは分ける。
既存`GOMODCACHE`を明示してcommand E2Eだけ再実行した結果、cleanupエラーなく成功。
以前の一時パスが存在しないことも別途確認した。

初回全体`race`は既存CONNECTのupstream-prefix停止テストで失敗した。
serve側がCONNECT上流を同期closeし、停止後に完了したdialをwrite前に拒否するよう修正。
proxy packageのrace反復100回と、その後の全体`race`が成功した。
[egress契約](EGRESS_AUTHORIZATION.ja.md)参照。認可Policyや利用コマンドは変更していない。

`bash tools/ci-local.sh`全体実行は、開発用Ubuntu WSLに`pwsh`がないため
release-configで**失敗**した。Windows PowerShellでinstaller component単独検証は成功。
配布由来の検査も最初Ubuntu 22.04の最低OS条件で失敗したが、Ubuntu 26.04でfixture限定の
検査を行い、由来・installer package契約が成功した。rootとWindows所有者の違いは、その
検証プロセスに限り既知worktreeだけをsafe.directoryとして扱って解消。GoReleaser設定検査も成功。
全release archive buildとfresh package installは今回は**SKIP**。

新しいGHA実行は**SKIP／公開阻害**。自動承認レビューが、push検証の明示許可先は
`SLktEx/Hacocoon-test`だけとして、実装branchの本体`SLktEx/Hacocoon`へのpushを拒否した。
sourceのpushもPR作成も行っていない。新しいCOWテストは既存Incus workflowに追加済みで、
公開が承認されれば実行可能。過去のBのCI成功を今回のrevisionの成功として扱わない。

## Incus起動時のPID再利用防止

Status: **implemented。repository回帰とhosted Ubuntu/WSL配布packageの受入は成功**。
共通Ubuntu/WSL installerはroot専用のIncus ExecStartPre guardを導入する。
前namespaceのdnsmasq/proxy PID記録をdaemon起動前に退避し、同一namespaceの記録と
resourceデータを保持する。[Host契約](design/trusted-host.ja.md#incus起動時のpid記録)と
[ADR 0013](adr/0013-incus-pid-record-boot-identity.md)を参照。

19件のcomponent回帰で再利用PID、WSL/native起動、service再起動、初期導入、同時実行、
中断復帰、不正metadataを確認した。installer/packageとWindows driver回帰も成功した。
Windows配布gateには再起動後のmarker更新とdnsmasq記録の退避確認を追加した。
同一namespace内のhelper PID再利用と任意device familyは上流側の残課題とする。

`1b2d6ae`の[Windows配布gate](https://github.com/SLktEx/Hacocoon/actions/runs/34051931616)で、
install、通常のWSL終了・再接続、marker更新、dnsmasq記録の保持、installer再実行、
install済みegress制御が成功した。
[Ubuntu配布gate](https://github.com/SLktEx/Hacocoon/actions/runs/34051931562)、
[実Incus gate](https://github.com/SLktEx/Hacocoon/actions/runs/34051931583)、
[repository CI](https://github.com/SLktEx/Hacocoon/actions/runs/34051931607)も成功した。
任意のauthenticated-private-registry jobはskipであり、private-registryの受入実績は追加しない。
最終revisionとmerge状況は[PR #480](https://github.com/SLktEx/Hacocoon/pull/480)に記録する。

当時のhosted受入はローカル環境を更新していなかった。下記の改訂Stage B fresh受入には、
main由来のこのPID guardも含まれる。

## WSL起動失敗の調査 — 2026-09-07

Status: **historical調査。起動失敗を再現し、古いPIDの再利用が原因であることを強く裏付けた**。
namespaceをまたぐ再発防止は上記のとおりimplemented。
対象は手元のWSL 2.7.12、製品 `029ff08e34c98e075b7b0b3d3a7fc7f639e89323`、
Ubuntu package `incus 6.0.5-8`。別Windowsアカウント・別端末の原因を確定するものではない。

02:32:21 JSTの成功起動では `haco-host0` のdnsmasqにPID 424が割り当てられていた。
通常のWSL終了・再起動後、02:33:13にkernel traceで
`kill(424, SIGKILL)` の対象が `libuv-worker` になっていることを捕捉し、
直後にIncus本体PID 248がsignal 9で終了した。traceの対象kernel PIDは9468であり、
424は呼出側namespaceのIDなので番号空間を混同しない。
独立したprocess一覧でlibuv workerがIncus本体のthreadであることを確認した。
OOMの記録は確認できていない。

上流の[v6.0.5 dnsmasq終了処理](https://github.com/lxc/incus/blob/a87f49a2491fa3a0e74896c1f2322bd356c59ddc/internal/server/dnsmasq/dnsmasq.go)は
保存済み `dnsmasq.pid` を読み、
[`Process.Stop`](https://github.com/lxc/incus/blob/a87f49a2491fa3a0e74896c1f2322bd356c59ddc/shared/subprocess/proc.go)を呼ぶ。
数値PIDの存在確認後に終了させ、boot・process開始時刻・実行ファイルの同一性は検証しない。
以前のdnsmasq PIDがIncusのthread IDとして再利用される経路と一致する。
失敗したsignalのuser-space呼出stackそのものは未採取であり、
PID・対象thread・直前のdnsmasq identity・失敗時刻を照合した結果である。
正常なforkproxy/helper終了でもSIGKILLが出るため、それだけで本体の障害と判定しない。

テスト専用の子processだけを使った独立検証では、
`pidfd_open(worker_tid)` はENOENT、数値IDへのsignal-0は成功し、
同じTIDへのSIGKILLでprocess全体が終了した。
これはOS上の誤終了の仕組みの検証であり、Incus修正の適用や上流packageのE2E回帰検証ではない。

起動中にIncus本体が終了すると、600秒の `waitready` start-post processが残り、
controllerの `After=incus.service` が待機を続け得る。
その結果、製品loginの2分の期限が切れる。login clientは最後のtransport errorも捨てるため、
timeout表示だけでは本原因やsocket権限の失敗を区別できない。

一時kernel trace・uprobeは全て解除した。時間限定の観測でIncus serviceを明示的に1回再起動し、
後続のinstall済み `haco doctor` は全6項目成功した。
回復は間欠障害の解消を意味しない。provider binary、PID file、storage、network、
install済みservice設定へのpatchは行っていない。
正しいprocess所有identityの検証はproviderのlifecycleの責務であり、
CoreによるPID fileの自動削除やlogin timeout延長は根本修正ではない。

## 現在のStage B改訂

状態: **implemented。改訂後のローカル配布物・実機受入は完了**。mainの基準は
`0665ba9`。これは作業branch候補の確認であり、公開releaseや他の未merge PRの完了を
意味しない。対象はnative WSL Interop、実在する複数drive、Persistent OCI Store、
Windows標準OpenSSH。`switch-base`は公開CLIで無効、Stage D以降で再検討し、A-Cを
blockしない。過去のコード・ADR・証拠は残す。[roadmap](status/architecture-and-roadmap.md#current-stage-b-scope)、
[OCI Store](design/persistent-oci-store.md)、[Windows SSH手順](reference/windows-environment-ssh.md)を参照。

**実機の配布物:** `c86c43e4f2702c5fccadd91f542d82bd8b733706`、checkpoint `v0.29`、
`0.27.0-SNAPSHOT-c86c43e`、build 2026-09-07 10:38:30 JST。Windows ZIP SHA-256は
`39205fea38aa7b38f8474edb6f957363b3565a22a00e6957d64ba542217566db`。
後続commitはテスト・文書の改善であり、実際に導入した製品はこの候補である。
構成はWindows 10.0.26200.9278、WSL 2.7.12、kernel
`6.18.33.2-microsoft-standard-WSL2`、Ubuntu 26.04、Incus `6.0.5-8`、
Incus所有Btrfs pool `haco-local-default`。既定Baseのrevisionは
`sha256:297ce79fb308c09126222dd6e64c260003c5d1e1ea1ce46ea43e80a419941636`。

**fresh手順:** UbuntuとUbuntu-24.04だけがありHacocoonがない状態を確認し、branchから
作ったZIPを展開して通常の`install-windows.bat`を実行。`wsl -d Hacocoon`からHostへ入り、
doctor全6項目が成功。既存Windows installer gateでWSL終了・再入場、Host内データ保持、
同じBATの再実行、cold doctorも成功した。追加でHostだけの
`incus restart haco-host --project hacocoon`直後を再設定なしで確認し、実際の
`wsl --shutdown`後の通常入口と、その後の通常`haco setup`再実行・doctorも確認した。
最終配布物の受入では、source由来binary、mount、PATH、socket、serviceの手動修復を
注入していない。Windows/WSL機能自体は導入済みであり、Hacocoon distributionのfresh
installであって、OS再インストールやWindows OS再起動の試験ではない。

| 項目 | 最終候補で確認した結果 |
|---|---|
| B1 drive | 実在するDrvFsからCとQを検出し、`/mnt/c`と`/mnt/q`へ投影。両NTFS driveでWindows作成ファイルをHostからread、HostのwriteをWindowsからreadし、空白入りpath/argumentも成功。WSL削除前のファイルもfresh install・Host restart・WSL shutdown後に保持された。同じWindows filesystemを参照しており、Environment内のコピーではない。 |
| B1 exe/PATH | 明示的な`/init`なしの絶対path cmd.exe、`cmd.exe /c ver`、`powershell.exe -NoProfile -NonInteractive`、Windows PATH上のwhere.exe/findstr.exeが成功。stdoutのhello、stderr marker、終了23を確認。検出drive下にあるWSL変換済みWindows PATHだけを継承。OCI/SSHのEnvironment作成・削除後も、開いたままのHostでInteropが動いた。 |
| 旧B3 | 配布物の`haco env switch-base`はcontroller操作前に終了2、currently disabledとStage D案内を返す。通常のEnvironment作成時のBase選択は維持。 |
| B4 | 別々のprovider identityを持つ2 Storeを作成。Aをattachし、使用中deleteを拒否。BusyBox pull、local image build/run、Environment削除、Store保持、別Environmentへの再attach、registry接続なしのimage再利用、buildのCACHEDを確認。別Storeは空でWorkspaceは保持。Environment deleteはStoreを残し、明示Store delete後のinspectはnot found。 |
| B5 | Windows標準System32/OpenSSH/ssh.exeから127.0.0.1:22229、WSL Physical Host、Incus loopback proxy、Environment sshdへ実接続。生成configはstrict checking、dedicated known_hostsをtrusted provider由来の公開host keyでpin。/workspaceを利用でき、鍵不一致は実行前に失敗。disconnect/delete後はWSL listenerが消え、Windows再接続も失敗。終了後のWindowsエラーはrefusedではなくtimeoutになる場合がある。 |
| B2/A/B6 | 一つのstage-b-git-devで独立した2つの.gitを確認し、commondir/alternates共有なし。Windows SSHでfetch/pull、編集、commit、製品helperの承認付きpushが両repoで成功。disconnect/stop後のstatusはEnvironment・Workspace・Base・保持状態を表示。通常WSL再入場後にもEnvironmentが停止状態で、Workspaceの未追跡noteが残ることを確認した。 |

OCIの実機版はcontainerd `2.2.2-0ubuntu1.1`、nerdctl `2.3.5`、BuildKit `0.33.0`。
各Environmentへ必要なruntimeとdaemon proxy設定を通常導入し、download Policyは
対象Environment・宛先に限定した。`pull --unpack=false`でcontentを保存し、
`run --snapshotter native`でsnapshotを用意する。imageとBuildKit cacheはStore、
process/socketは`/run`に分離。再attach前後でBusyBoxのIDは
`sha256:c6348fa86ba0fb2108c9334f5fe913ddc6d853313e655891f133a0127c30099f`、
local imageのIDは`sha256:475bcd7010f2b330b1b82f7a43a911baeb6be801dfd1d2fb2d6b7b498a99c7bb`
で一致した。Docker Storeの互換性は**未確認**。下記の旧Docker配布受入は過去の証拠である。

Git成果の送信先は`https://github.com/SLktEx/Hacocoon-test.git`のみ。
指定に従い同じURLの異なるbranchを2 repoとして登録した。異なるremote URLの動作は
local real-Git回帰で扱う。両pushともrepo・Environment・URL・ref・操作・旧OID
`f4ff6e33588a7183b0c7d3db2f4c2214a527678f`と固定新OIDを照合して`haco git approve`を実行し、
Windows側の独立した`git ls-remote`で一致を確認した。

| repo / branch | 確認したremote commit |
|---|---|
| stage-b-first / codex/stage-b-20260907-first | `7f9f9ecaaae1cc332c3a42d9724eeddbb9701f4d` |
| stage-b-second / codex/stage-b-20260907-second | `98168553a91e20f2f97b0658bfd305ab4ed488e6` |

Btrfsの観測ではsource UUID `8dbe4029-79b1-5f46-96d1-522b9cf9fd6a`と
`d84cae46-8f22-2144-99c8-d7d6ac5a6c9f`が、それぞれWorkspace
`a558d778-1ac5-1047-9c04-d72568d530ff`と`8b7b06f4-8b40-7d4d-92e8-4643074ca769`の
parent UUIDに一致した。独立COW関係の証拠であり、性能評価ではない。
`managed:stage-b-both`と停止済みEnvironmentは利用者向けに保持している。

**境界・検証:** Environmentに/init・WSL socket・Windows drive・Windows exe権限がない。
client秘密鍵はWindowsでのみ作成・削除し、公開鍵だけをHacocoonへ渡した。Git資格情報は
trusted Hostのroot専用標準gh storeだけに置いた。既存Windows Git tokenの本人性と
テストrepoのpush権限を公式gh apiで確認し、stdin経由で設定。tokenのログ出力や
Environmentへの保存はない。gh auth login --with-tokenは追加OAuth scope不足で失敗したが、
権限を拡張していない（[GitHub CLI仕様](https://cli.github.com/manual/gh_auth_login)）。
これは手動の資格情報設定であり、新しい製品credential brokerではない。

installed egress検証は許可HTTPS、拒否proxyの403、直接TCP拒否、管理socket非共有が成功。
local CIのdocs、workflow policy、Go test/vet、JavaScript、race、E2E、隔離namespaceの
kernel forwardingが成功した。release/package、native interop、lifecycle ownership、
guest systemd readiness回帰も成功。対応systemdが必要なrelease checkはUbuntu 26.04、
Windows installer/BAT componentは実PowerShell 7/5.1で確認。開発用Ubuntu 22.04では
release phase全体をそのまま実行できず、対応platformで各componentを分けて実行した。
新しいhosted CIやprivate registryの受入を意味しない。

再実行用driverは[Windows installer](../tools/windows-installer-user-path-e2e.py)、
[native access](../tools/windows-native-access-e2e.py)、
[Windows SSH](../tools/test_windows_environment_ssh.ps1)、
[OCI lifecycle](../tools/test_persistent_oci_store.py)、
[installed egress](../tools/installed-egress-check/main.go)。
ローカル証拠はbin/stage-b-c86-*、特にfresh-package-gate、persistent-oci、
native-ssh-cleanup、approved-git-standard-credentials、two-repository-cow、
retained-workspaceのログに保存した。ログや資格情報はsource archiveに含めない。

Git認証・Policy、SSH key/config/pin、任意OCI runtime/proxy設定は手動。
Windows/image/runtimeの広い互換性、drive着脱、開いたsession中の外部操作によるWSL
binfmt登録削除、異常切断、upgrade、汎用復旧は未確認。以前観測したhandler消失のtriggerは
未特定であり、通常setup/entryはhandlerがない場合だけWSL自身の生成serviceで補完する。
D+の自動化、switch-base再検討、registry/broker、Store同時共有、live migrationは
[後続課題](status/development-follow-ups.md)に残す。

## 過去の第二段階（対象範囲を変更済み）

以下は以前の依頼に対するcommit固定の実行証拠。旧B3と配布専用B4は現行要件ではない。


状態は**implemented・以下のWindows/WSL構成でB1〜B6を受入済み**。
Dockerとnerdctlの両方で一方向配布・独立起動が成功した。
選択したB5/B6改善と、影響を受けるAの基本導線も確認済み。
[利用手順](reference/managed-repository-workflow.md)、
[OCI契約](design/persistent-oci-store.md)、
[残課題](status/development-follow-ups.md)を参照。

| 段階 | 実装と確認結果 |
|---|---|
| B1 | trusted Host限定の明示setupで既存DrvFsドライブを検出。PowerShellへ独立した空白付き引数を渡し、stdout/stderr・終了23を確認。利用者所有`/mnt/c`の読み書き成功。最終候補029ff08でも再確認。実機はCのみ。追加ドライブは解析回帰のみで実機未検証。EnvironmentへWindows device・`/init`・interop環境変数は渡っていない。 |
| B2 | 配布物087e7e2でb-dev内の`/workspace/b-first`・`/workspace/b-second`を確認。独立Btrfs copyと専用.git、alternates/commondirなし。SSH fetch/pull・編集・commit・固定内容承認付きpushが両repoで通った。GitHub側OIDは`be34f60c2c3d1ab5761e821fbdaada5e4d5802dc`と`b834ee67dbc8f5e37e73656f13872d42ceda40f3`。異なるremote URLもローカル実Git回帰で確認。 |
| B3 | 配布物3747baeでBase一覧と26.04→24.04切替。全54ファイル（.gitを含む）のハッシュ一致。未push commit bce47b9 / 6d5fc53、未コミット変更、未追跡notesを保持。Git/SSH再接続、新host key固定、Ubuntu24.04.4上のSSH編集も成功。 |
| B4 | 配布物029ff08の`haco plugin oci distribute --runtime <runtime> --image hacocoon-b4:smoke b-dev`でDockerとnerdctlを個別に確認。通常SSHからguestコンテナを起動・ファイル変更・停止し、対応するHostコンテナが元の内容で稼働し続けることを確認した。両driver、export失敗、入力・サイズ上限・instance内固定socketの回帰も成功。 |
| B5 | 配布物029ff08の`haco env ssh-config b-dev`で生成した設定から通常SSH接続が成功。host/port/userの手動転記を削減。Incus6.0.5にないconfig show --formatをJSON query APIへ修正し、回帰を追加。 |
| B6 | 配布物029ff08のenv statusで対象Environment・状態・Workspace・access・Baseを読みやすく表示。停止時はWorkspace保持を明示。機械可読出力は--jsonで取得できる。 |

実push先は指定の`https://github.com/SLktEx/Hacocoon-test.git`だけ。
branchは`codex/stage-b-b-first-20260906`と`codex/stage-b-b-second-20260906`。
実機の2登録は同じ許可済みURLの別branchを使用し、別URLの振分けはrepository回帰で確認した。

Btrfs source UUID `411102dc-d913-264a-96a0-b09d079eb898` /
`58ccd7df-d8df-3444-98b4-67b35d85018e`が、それぞれWorkspace volume
`49eff338-40d8-244b-9276-e35952b475b2` / `a23fadbc-77af-be4a-b7a9-f9829e96e613`
のparent UUIDに一致した。実際のCOW関係の確認であり、性能計測ではない。

最終導入候補は`029ff08e34c98e075b7b0b3d3a7fc7f639e89323`、checkpoint v0.28、
snapshot `0.27.0-SNAPSHOT-029ff08`、build時刻`2026-09-06T10:25:40Z`。
Windows ZIP SHA-256は`20f308cb5bcccfdaef1f0c76914bdae65834c957afd6c446fd0effdda26717fe`。
各候補のbranch commitから配布物を作り、通常BATで既存Hacocoon WSLへ適用した。
製品の検証用overrideや内部state修復は使っていない。fresh導入は再実行していない。
保持したA構成はWindows26200.9278 / WSL2.7.12 / Incus6.0.5、Incus所有Btrfs
pool haco-local-default。

B3の26.04 revisionは`sha256:d071290fb40659981198baf0161a8bcc9910ebae79a15f5ef5d9c06dbdb2ea4c`、
切替先24.04は`sha256:f38ca805517f5b6e301f33b0f44523386c5a050847564c1233e586106b31dbc9`。
後の26.04明示作成では`sha256:297ce79fb308c09126222dd6e64c260003c5d1e1ea1ce46ea43e80a419941636`
へ解決された。先に作成したEnvironmentの固定revisionは変わっていない。

最終候補のA回帰では単一repo b-a-work / b-a-devを通常作成。生成SSH設定、
fetch・f4ff6e3からbe34f60へのfast-forward pull、Python compile/assert、commit、
push拒否（remote不変）、続く承認pushが成功。GitHub側で第一branchの
`145fd7fce49a5a8771e39e7b142d47aa49c910c3`一致を確認。disconnectと正常stop後も
全28ファイル（未コミット・未追跡・Git状態）とcanonical lease・volumeを保持。
内外clientとcontrollerのbuild一致、doctor6項目も正常。元のA資源は保全した。
OCI導入後も両repoのSSH/helper fetchが成功し、許可proxy通信・外部直接TCP拒否・
guestへのWindows interop/controllerパス非公開を再確認した。通常の`env stop b-dev`後も
全57Workspaceファイルのハッシュとcollection所有権を保持し、Hostの両コンテナは稼働継続。
b-a-devとb-devは停止している。

B4構成・結果（2026-09-06）：所有確認した非privilegedのhaco-hostとhaco-b-devに
明示的にnestingを設定し、既存deviceとnetwork guardを保持した。両側へUbuntuの
docker.ioを独立導入し、Docker29.1.3、containerdはHost2.2.2・guest2.2.1。
公式最小nerdctl2.3.5配布物のSHA-256は
`de3206aeb7cbd5f20f5fb1f55c1e3bf2db1be567812a8a3f5e65eba2488347ee`。
full bundle・privileged化・AppArmor無効化・runtime device共有は不要だった。
イメージはUbuntuのbusybox-static、shell symlink、固定/data/messageだけを含み、IDは
`sha256:9bafa1f9ed06b9fcc33ef5b6674ef3c4d79ae819b7724d5b228923712112b46f`。
両方の製品配布は1,183,232 bytes、archive SHA-256は
`a2ea9ac81b39572d424bd2b63461ac659c2b0a4c327ccb963e110f08ed553c57`。
両方とも--network noneで起動。Docker guestはguest-only、nerdctl guestは
nerd-guest-onlyへ変更したが、Host側は両方host-originalのままだった。
[再現手順](design/persistent-oci-store.md)を参照。

検証はci-local.shのdocs・workflow-policy・test（Go/vet/JS）・race・e2eが
B5/B6変更後に通過。関連するlifecycle/Git/collection mount/OCI/SSH設定回帰、
GoReleaser check/buildと配布checksum、独立Linux network namespaceのforwarding jobも成功。
ローカルGoは1.27.1。release-configとinstaller/provider jobを含むhosted CI結果は
[PR #473](https://github.com/SLktEx/Hacocoon/pull/473)に記録する。
広い実機runtime/network matrixの受入は主張しない。

手動操作はB1のPhysical Host設定、trusted側認証と限定Policy、client所有SSH鍵と
host key固定。別WSL distroのloopbackから届かない構成があり、controllerの
Physical HostまたはWindows loopbackを使う。SSH内proxy exportは既存#469として残る。
Base切替ではroot filesystem/packagesを破棄しGit/SSH再接続が必要。
B4は各instanceへの明示nesting/runtime設定が必要で、Base交換後は再設定する。
以前の自動実行レビュー拒否はB完了の再依頼後に解消し、対象instanceを固定した設定と
既知の検証image配布が実行・成功した。追加Windowsドライブは実機になく未確認。
広いWindows/image互換性、中断処理、汎用復旧は未検証として残る。

## 管理対象repoのWSL利用経路 — 2026-09-06

**implemented・以下のローカルWindows/WSL構成でA1〜A6を受入済み**。v0.27候補は、新hacoのrepo登録、
独立したIncus Btrfs Workspace copy、controller経由のEnvironment作成・SSH、
Git専用remote helper、Workspace所有権を保持する正常停止を実装する。
認証付きGitはtrusted `haco-host` 内で実行し、Policy・承認・state・Incus権限は
Physical Hostのcontrollerに置く。実Gitを使うローカル回帰では通常fetch・競合のないpull・
push拒否・旧/新OIDを固定した承認付きpushが成功し、承認待ち中のlocal branch変更でも
送信対象が変わらないことを確認した。repository検証と以下の実機観測は区別する。
[利用手順](reference/managed-repository-workflow.md)と
[所有権の決定](adr/0008-managed-repository-workspaces.md)を参照。

**配布物の受入:** commit `7a4d1227c95642f27cb118c3d20d2cd554e8be32`、
version `0.27.0-SNAPSHOT-7a4d122`、build `2026-09-06T07:57:54Z`。
Windows ZIPのSHA-256は
`0468c8f95c5b431c5d4160aead860deb152ed8d8e381b321c6b85b2f650d1a80`。
Windows build `26200.9278`、WSL `2.7.12.0`、kernel `6.18.33.2-2`、
Ubuntu 26.04、Incus `6.0.5`、Incus所有Btrfs pool `haco-local-default`。
既存Hacocoon distributionへ同梱BATを通常実行し、doctor全6項目が成功して終了0。
CI専用の製品設定や内部資源の修復は使っていない。この候補の未登録distroからのfresh導入は
**未再実行**であり、下記の過去installer受入とは区別する。

| 段階 | 観測結果 |
|---|---|
| 入口・controller | 通常の `wsl -d Hacocoon` でtrusted Hostへ入り、内外のhacoが同じbuildと `poc-dev` を返した |
| repo・COW | `https://github.com/SLktEx/Hacocoon-test.git` の `codex/wsl-poc-20260906` を `poc` として登録。独立copy `poc-work2` を `poc-dev` の `/workspace` に配置 |
| 既定Base | `haco/ubuntu-26.04`、revision `sha256:d071290fb40659981198baf0161a8bcc9910ebae79a15f5ef5d9c06dbdb2ea4c` |
| 開発 | client所有鍵と固定host keyによる標準OpenSSHで編集、Python byte compile・unittest 2件・commitが成功。専用.gitを持ち、commondir/alternates・Host gh認証ファイル・管理socketはなく、trusted元worktreeも未変更 |
| fetch/pull | 同じWorkspaceで通常helper fetchと `pull --ff-only` が `f4ff6e3` からremoteで作った `19caa79e123b981227d1c0b58783c7a6af80e930` へ進んだ |
| 拒否 | `haco git deny` 後のremoteは `19caa79` のまま |
| 承認 | proposalの登録URL/ref・操作・`19caa79` → `c18cbb8e202cecc0d6c80b29a8cd700dc1c0558f` を確認してapprove。通常git pushが終了0となり、GitHubからも同じOIDを取得。auditの拒否・承認・成功を確認 |
| 終了 | SSH commandが終了し、`env disconnect poc-dev ssh-2222` と `env stop poc-dev` が成功。内外statusはstopped、再接続は拒否 |
| 保持 | canonical leaseとcustom volumeを保持。未push HEAD `5650953d591fc6294a0db8db5f71a408e7917555`、変更済greeting.py、未追跡notes、branch refのSHA-256は停止前後で一致。remoteはc18cbb8のまま |

元repoのBtrfs UUID `760d7b7e-0e0e-7f4c-9f88-7303ad96f55c` と、Workspace
`f3326bfe-8cb0-684e-a3a3-d437dd3b817e` の親UUIDが一致した。COW関係の観測であり、
性能計測ではない。最初の候補 `c116307` はrepo登録後、copyのIncus ID-map履歴を
落としてEnvironment書込み検証に失敗した。provider回帰と `7a4d122` の修正で履歴を保持する。
失敗copy `poc-work` は保全し、同じ登録repoから通常workspace createで作った新copyで
受入した。chownや特権Environmentによる回避はしていない。

**残る手動setup:** trusted Hostにgit/ghを導入して認証、Physical Hostで対象Git/Ubuntu
取得だけをPolicy許可、SSH公開鍵の準備・host key固定、SSH shellでcredentialを含まない
Standard proxy URLをexport。既存の許可済gh credentialは標準入力でtrusted Hostだけへ
渡した。適用前のHost HTTP/HTTPSはtimeoutしたが、通常BATのsetup後はreadinessが成功した。
別途の修復は行わず、最初の失敗原因は未確定。

**repository検証:** `ci-local.sh docs`・`workflow-policy`・`test`（全Go、vet、JS）・
`race`・`e2e` が成功。env未実装を前提にした既存E2Eを更新し、一時HOME外の既存Go cacheで
実行した。ID-map修正は関連race回帰、配布物はGoReleaser check/buildとchecksumが成功。
完全なrelease-config/forwarding jobや新しいhosted CI実行の成功は主張しない。

**deferred・未検証:** SSH proxy自動設定は [#469](https://github.com/SLktEx/Hacocoon/issues/469)、
承認中断・push結果不明・retryは [#470](https://github.com/SLktEx/Hacocoon/issues/470)。
次の依頼のB1は既存 [#275](https://github.com/SLktEx/Hacocoon/issues/275) の境界に沿った
trusted haco-hostのWindows exe実行・利用可能なWSLドライブmountであり、Environmentへ自動公開しない。
B2/B3の複数repo・Base変更、大きなpack・他認証方式・force/複数ref・LFS/submodule、
汎用復旧/削除・resume UX・広いhost matrixは未受入。test Environmentは停止し、両copyと
test branchを意図して保持した。B/Cの実装は今回に含めない。

以下のM1記録は各記載buildに対する過去の観測として保持する。

## WSL向け更新 — 2026-09-06

この候補branchのWSL向け実装を以下に示す。今回指定されたWSL M0–M1の範囲は **implemented、受入済み**。更新main `e8974ef` は#441/#442/#453/#456と#458/#459を含む。後続のrelease準備・取消2commitの最終ファイル差分はなく、取込merge `b58f82c` の製品treeは受入対象 `c749ff9` と同じ。merge済みPRや過去のgreenを後続製品変更の受入としない。

- **storageはimplemented:** 配布/runtimeはIncus所有Btrfsだけを使う。外部 `driver`/`source` attachmentや不確実な検査はfail closed。desired policyは `compress=zstd:3,noatime,nodiscard`。[読み取り専用mount診断](design/btrfs-storage-layout.ja.md#読み取り専用のmount診断)は設定・検証済みlive反映・反映待ち `pending` を区別する。backing device/inode、単一の全image loop関連付け、Btrfs root mountの一致を要求し、不明・不正・観測中の変化を成功としない。独自のimage/loop/mount lifecycleや診断修復は追加しない。
- **installerとtrusted Hostはimplemented:** 既定はnon-root `hacocoon`、passwordはlocked。`-InteractiveUserSetup` は任意。現在版再実行はaccount識別/password状態を保持し、sudo policyを書かない。controller所有の `haco setup` が所有trusted hostと限定endpointを準備し、common installerは製品doctorの全項目成功を完了条件とする。fresh hostはprofileを継承せずdeviceを明示する。所有 `haco-host0` はtrusted基盤向けDNS/DHCP/NATを提供し、Docker転送許可はそのbridgeと戻り通信だけに限定する。[bootstrap](WINDOWS_WSL_BOOTSTRAP.ja.md)、[trusted Host](design/trusted-host.ja.md)、ADR [0004](adr/0004-wsl-installer-authority.md)・[0005](adr/0005-trusted-host-network-ownership.md)・[0006](adr/0006-controller-owned-host-setup.md)を参照。
- **製品CLIはpartial:** 新 `haco` はhelp/version・`setup`・`doctor`・controller経由WSL login aliasを提供し、`hacoq` を呼ばない。controller state・Policy・provider・Incus権限はPhysical Hostが所有し、guestにcontrollerやIncus daemonを置かない。[診断](design/controller-client-transport.ja.md#host診断)は順序付き6項目と長さを制限した失敗/pending actionを返す。controller待機とguest DNS/routeの読み取り専用起動待機は、失敗した外部検査の再試行やresource修復をしない。広いlifecycle/Base/SSH CLI移行は別件で、#456のcontroller adapterは再利用できる。
- **Standard proxy lifecycleはimplemented:** install済みcontrollerは固定proxy listenerを所有し、bind前に共有guardを検証する。control/proxyの停止を連動させ、hijack済みCONNECTも閉じる。daemonはambient approval providerを持たず、exact allowのauditを維持し、require-approvalはfail closed。同PID listenerと未管理元拒否はEnvironmentの許可通信と区別する。[ADR 0007](adr/0007-controller-owned-standard-egress.ja.md)を参照。
- **repository検証 — `c749ff9`:** 対象race/vet、pendingのCLI/API回帰、Windows assertion 9件、installer実shell 5件、shell構文、文書検査が成功した。維持する `ci-local.sh test` から全Go shuffle test・vet・JavaScript構文2件・notification test 5件が成功した。先行local vetは `bin/` に取得した調査用sourceを含めて停止したが、その観測資料を `.txt` に直してentry point全体を再実行し成功した。製品環境変数のoverrideやinstall済みresourceの修復は与えていない。
- **Seed撤去はplanned:** codeは残り、[Base/任意OCIとの依存](design/oci-seed-and-cow.ja.md)を保持する。Base選択と任意Pluginの境界は維持する。
- **登録時の続行はimplemented、Windows package受入済み:** WSL一覧取得失敗を不存在とせず、native作成成功後も対象名の登録を読戻し確認する。作成/読戻し失敗時は手動で現在版BATを再実行するための段階/option記録を保存する。記録は権限を与えず、実行もしない。明示的な終了3010は再起動待ちとして伝え、終了0でも未登録なら未完了とし、再起動案内は条件付きにする。PowerShell 5.1 component testと実BATの終了code伝達testは成功した。これらはWindows機能installやOS再起動の受入ではない。[bootstrap続行](WINDOWS_WSL_BOOTSTRAP.ja.md#登録の中断とwindows再起動)を参照。

package受入の対象は **`c749ff9033b33c3526e108f60ce2009638075152`**:

| 環境 | 実測した受入 |
|---|---|
| [Windows gate](https://github.com/SLktEx/Hacocoon/actions/runs/34008408570) | 正規cached BATのfresh作成、通常入口、停止/再入場、同版再実行、cold doctor、build識別、保持、proxy所有、未管理元403が成功 |
| [Ubuntu installer](https://github.com/SLktEx/Hacocoon/actions/runs/34008411207) | 配布物からのordinary-user installとtrusted-host検査が成功 |
| [Incus gates](https://github.com/SLktEx/Hacocoon/actions/runs/34008410296) | standalone・owned Btrfs・authenticated private registry・Coreの全jobが成功 |
| 現在のWindows実機 | 未変更ZIPの適用と同版BAT再実行はreadiness全6項目成功後に終了0。通常入口、両clientのbuild全体一致、UUID/file/account/sudo policy保持、Btrfs状態、proxy検査が成功。distro停止確認後のdoctorは51.906秒で終了0 |

実機ZIPは `0.26.1-SNAPSHOT-c749ff9`、build日時 `2026-09-06T03:12:38Z`、SHA-256 `f638379fb293cf249f32ef46b5576b95906ff775bc2f00f96ae3ed602724d3f9`。fresh Windowsはrunnerのcurrent WSL基盤でHacocoon distributionがない状態を意味し、Windows機能無効状態やWindows OS再起動の受入ではない。実機の保持証拠はtrusted-host sentinelと基準値であり、Workspaceの未commit・未追跡・未push作業保持の証明ではない。

**未解決の起動失敗:** `42e2fb3` の通常入口で11:33:30 JSTにIncus本体PID 282がSIGKILLを受け、標準600秒start-post待機とcontroller依存が残った。signal送信元は未確定で、得られたkernel記録はOOMを示していない。手動service/mount修復なしで11:43:16にIncus標準の自動再起動が始まり、後の入口/保持検査は成功した。guest DNS/DHCP起動の競合は別途修正・受入済みであり、その修正や後の `c749ff9` 成功からSIGKILL送信元や以前の独立したWSL終了9の原因を確定しない。

**登録package受入 — `4df465a71aedcdc70c28b543220b79b2465808ab`:** [Windows run 34010791925](https://github.com/SLktEx/Hacocoon/actions/runs/34010791925)、job `101426135649` で正規fresh cached BAT、通常入口、停止/再開、同版再実行、データ保持、doctor 6項目、PowerShell/BAT回帰が成功した。手元のPS5.1実一覧/引数伝達、配布/provenance、`ci-local.sh docs` / `workflow-policy`、native文書検査も成功。provenanceの最初のUbuntu 22.04実行は26.04以上の条件で正しく停止し、製品条件を変えず対応基盤で成功した。実機向けZIPのSHA-256は `439dfc8a0a4dab5ef4adf05f1b1ed9b3e02883a5009b66dca7513c528d0d3105`、version `0.26.1-SNAPSHOT-4df465a`、build `2026-09-06T04:10:02Z`。build/checksum確認まで行い、手元で再installは繰り返していない。現在の実機installは受入済み `c749ff9` のままで、変更したfresh登録/再実行はCIで確認した。
**現在のM1範囲:** 最新のユーザー方針により、実Windows OS再起動の実装/受入と続行案内の追加作り込みは対象外。具体的な変更や失敗に見合う検証に絞り、追加で維持する回帰はCIへ置く。新しい根拠なしに成功済み検証を繰り返さない。必須だった既存controller/provider境界を使うinstall済みEnvironmentの許可proxy通信/直接通信拒否の受入は成功した。原因未確定の起動事象は記録に残し、後の限定signal観測でもその原因は特定できていない。診断機能の拡大、firewall起動順の網羅、CLI/SSH開発導線、Workspace保持は後続とし、追加の完了条件にしない。

**Environment接続元の修正:** `f373cfc` のWindows gateは正規BAT経路に成功したが、許可HTTPS probeがproxy 403になった。永続接続元resolverがprovider-local参照とEnvironment作成のroute付き参照を比較していたため、正規router decoderでproviderとnative参照の両方を照合するよう修正した。実際のBase routerの作成結果を使う最小回帰で失敗を再現し、別providerの同一native参照は拒否する。

**M1受入 — `81c0d160722b96864daa8d6f5f3b9ea86423ff48`:** [Windows run 34013409969](https://github.com/SLktEx/Hacocoon/actions/runs/34013409969)、job `101432997324` でfresh cached BAT install、通常入口、停止/再開、同版再実行、trusted-hostデータ保持、doctor 6項目が成功した。install済みcontrollerのEnvironment検証でも、証明書を検証する許可HTTPS、未承認hostnameの403、直接TCP拒否、管理socket非公開、controller cleanupが成功した。CIの対象はPR merge commit `9049df39f8000e32103b6a2f3939ea3d14fc5ffe` で、candidate `81c0d16` と全treeが一致することを確認した。route付き参照の回帰は修正前に失敗し、修正後の手元egress・Environment router・composition・Standard proxy testはすべて成功した。文書整合性検査も成功。

新しい手元ZIPは `0.26.1-SNAPSHOT-81c0d16`、build日時 `2026-09-06T05:12:08Z`、SHA-256 `4938622b994a66b71d5647086819db63e7ee7a7a8ea1189e3b2ad964ccb69c6b`。GoReleaser配布物作成と全checksum検証が成功した。このZIPは現在のWindows実機に再installしておらず、実機のinstall版は `c749ff9` のまま。上記candidateのWindows受入はCIでの結果である。実Windows OS再起動は対象外のままとする。

**次の具体的な一件:** M2として、既存controller-backed adapter経由のEnvironment作成を新 `haco` から利用できるようにする。

以下の表は元のcheckpoint時点の履歴文脈を保持する。

[English](IMPLEMENTATION_STATUS.md) | **日本語**

> 現在の `main` の code reality を示す companion です。番号の正本は [`status/versioning-and-release-status.ja.md`](status/versioning-and-release-status.ja.md) です。

Hacocoon は pre-1.0 です。現在のmilestone位置は **v0.38** です。milestoneは軽量なdevelopment checkpointとして扱い、v0.17のacceptance残件のようなpartial状態があっても、後続の実装済みcheckpointへ進めます。repository実装は、明示的に名前を付けたacceptance checkを除き、すべてのreal-host supportを意味しません。

| 領域 | 現在の状態 | Milestone |
|---|---|---:|
| Runtime / Workspace | Incus Environment lifecycle、Workspace identity、RO/RW lease | v0.1-v0.2 |
| Client access | status、loopback forwarding、SSH prepare/revoke | v0.3 |
| Policy / Capability | fail-closed policy、approval、audit | v0.4 |
| Git push | trusted Host がbrokerし、reusable Host credentialをEnvironmentへ渡さない | v0.5 |
| Agent integration | `haco run`、machine output、events。orchestrationはCore外 | v0.6 |
| Client-neutral interaction events | public `pkg/interaction` がcapability auditを最小化済みeventへprojectionし、stable ID、resume cursor、bounded batch、recovery/attention flag、public corruption errorを提供。観測はcapabilityを承認・実行しない | v0.6 / cross-cutting |
| Environment routing | provider-neutral seamは維持。**具体的なcloud implementationは現在deferred**で、EC2/AWS/EBS実装はactive treeにない | v0.7 |
| Reusable client adapter contract | public `pkg/clientadapter` がexact Environment ensure/reuse、status、loopback SSH/TCP、revoke/delete、`/workspace` discovery、`pkg/interaction` batchをpackage-owned DTOで公開。通常の `haco ssh` がnon-VS-Code proof path | v0.8 / cross-cutting |
| VS Code / Agent Host | `haco-vscode`、per-agent binding、`haco-agent-host` | v0.8-v0.10 |
| Base | `haco base list` / `inspect`、immutable Base revision | v0.11 |
| Resource budget | CPU / memory / PID / root storage | v0.12 |
| Managed Sandbox Network | `haco-sandbox0`、proxy-only ACL transport guard、`haco-sandbox` profile。DHCPを残してbridge DNSを停止し、driftはfail closed | v0.13 / cross-cutting |
| Git Fetch Plugin | `haco plugin git fetch`、Host `gh auth git-credential` | v0.14 |
| OCI Seed Recommendation | `haco plugin oci seed sample` / `recommend`、top 10%を `auto_promote=true` | v0.15 implemented |
| OCI Image Deletion | `haco plugin oci image delete`、deletion tombstone、exact immutable identityの明示reenable | v0.16 implemented |
| OCI Seed Builder / Btrfs COW | `seed build/current`、Base単位pin、保守的GC/recover、trusted Host acquisition、managed Environmentからのcredential-free exact-image harvest、offline no-NIC build、immutable publish/current pointer、exact-parent resolutionを実装。real-host/authenticated-registry/COW acceptanceはpending | v0.17 partial |
| Docker Compatibility | `haco plugin oci docker status/prepare`。Base提供profileとpinned systemd unitを検証し、active vendor daemonを勝手に停止せずEnvironment-local socket activationだけを有効化 | v0.18 implemented |
| Domain-aware egress authorization | Core `network.egress/connect`、Standard HTTP/HTTPS proxy、Host DNS pinning、private-address reject、CONNECT/SNI検証、trusted Incus source-IP mapping、`haco egress serve` を実装 | v0.19 implemented |
| Managed Btrfs rootfs storage | local compositionが `haco-local-default` Incus-owned loop-backed Btrfs poolをlazyにensureし、Hacocoon所有のBase/Tooling/Seed/Environment/trusted-host rootfsをそのpoolへ配置 | v0.20 implemented |
| Managed Btrfs transparent compression | default Incus pool作成時に `compress=zstd:3` を要求する。`compress-force` と `autodefrag` はdesired defaultにせず、mount lifecycleはIncusが所有 | v0.21 implemented |
| Interaction notification clients | `haco-notify` がloopback interaction deliveryをbrowser/native OS向けに提供し、optional VS Code notification extensionも同じinteraction streamを利用。replay/dedup behaviorをtest済み | v0.22 implemented |
| Real Incus E2E acceptance | GitHub-hosted Ubuntu 26.04でstandalone real Incus system-containerを先に検証し、その後fresh runnerでHacocoon Core lifecycle E2Eを実行。systemd/exec、network、hotplug、storage/snapshot、diagnostics、guarded cleanupをphased gateで検証 | v0.23 implemented |
| Structured logging | shared `log/slog` foundation、INFO-default text/JSON output、Environment lifecycle operation field、sanitize済みDEBUG Host-command trace、egress authorization trace、secret redactionをmaintained executableへ実装 | v0.24 implemented |
| Incus-owned Btrfs storage acceptance | actual ordinary-user `haco` をreal Incusへ接続し、lazy pool creation、Incus-owned sparse backing image、loop attach、Btrfs mount、zstd policy、writable Workspace、pool reuse、guarded cleanupまで自動検証 | v0.25 implemented |
| Trusted `haco-host` / default WSL entry | local Incus runtimeがpersistent trusted logical `haco-host` をensure/shellでき、exact ownership markerとreserved-name collision拒否で境界を守る。managed storageを使い、WSL interactive entryはdefaultで`haco-host`へ入り、Physical Host rootは明示recovery pathとして残す。raw Incus controlは`haco-host`へ公開しない | v0.26 implemented |
| OCI plugin boundary | `HACO_PLUGIN_OCI=nerdctl|docker` の明示opt-in。未設定でもCoreは動作する | cross-cutting |
| Optional Local OCI Registry | optional。通常pullやSeed constructionの必須経路ではない | unversioned optional / deferred |

## Domain-aware egress境界

ordinary HTTP/HTTPS egressはDNS-to-IP ACL近似ではなくStandard proxyでenforceします。Incus NICはdefault denyを維持し、managed bridge gatewayのStandard proxy portへのTCPだけをallowします。bridgeはDHCPを残しつつ `raw.dnsmasq=port=0` でDNS listenerを停止し、unmanaged DNS/ACL configはfail closedです。

managed profileがEnvironmentへHTTP(S) proxy discoveryを提供します。proxyはtrusted Incus source-IP stateからEnvironment identityを導出し、hostname / port / protocolごとに既存Policy / Approval / Capability / audit経路を通し、authorization後だけHost DNSを解決してpublic answer setをconnection単位でpinします。HTTPS CONNECTはTLS bytesをupstreamへ流す前にClientHello SNIとauthorized hostnameの一致を検証します。`haco egress serve` はtrusted Host foregroundの起動経路です。詳細は [`EGRESS_AUTHORIZATION.ja.md`](EGRESS_AUTHORIZATION.ja.md) を参照してください。

## Notification clients

v0.22ではclient-neutral interaction streamをuser-visible notification adapterへ接続しますが、approval authorityはclientへ移しません。`haco-notify` がbrowser/native向けloopback bridgeを提供し、`clients/vscode-notify` がoptional VS Code consumerを提供します。cursor persistence、replay、dedup、corruption handling、browser behavior、VS Code behaviorをrepository testで検証します。notificationの表示は観測だけであり、Capabilityを承認・実行しません。詳細は [`INTERACTION_EVENTS.ja.md`](INTERACTION_EVENTS.ja.md) を参照してください。

## Real Incus E2E acceptance

v0.23は新しいCore APIではなくsupport-confidence checkpointです。GitHub ActionsのUbuntu 26.04上でまずIncus substrateだけを独立に検証し、その後Hacocoon Core lifecycleをfresh runner上のreal Incusで検証します。standalone stageはreal system container、systemd/exec、network、device hotplug、storage/snapshot、diagnostics、exact cleanupを確認し、Core stageはその成立済みsubstrateに対してHacocoon lifecycleを確認します。これによりIncus側のfailureとHacocoon regressionを切り分けやすくし、fake-only E2Eを十分なacceptanceとは扱いません。

## Structured logging

v0.24ではstructured loggingを独立したmilestoneとして扱います。maintained executableは `HACO_LOG_LEVEL` / `HACO_LOG_FORMAT` から1つのshared `log/slog` rootをconfigureします。defaultはINFO/textで、JSONへ切り替えてもstdoutのcommand resultは変えません。Environment create/exec/shell/deleteは `operation`、`environment_id`、duration、result/error fieldをcontext経由で持ち回ります。trusted Host runnerはsanitize済みcommand metadataをDEBUGで追加し、Incus/network/storage/Git/OCIをcomponent分類しますが、subprocess stdout/stderrを自動logしません。

shared handlerはpassword/token/API key、authorization/cookie、credential-bearing URL、secret assignmentのknown patternをDEBUGを含めてdefense-in-depthでredactします。ただしcall site側でもarbitrary header、environment、config object、private key、request body、untrusted outputを渡してはいけません。詳細は [`reference/logging.ja.md`](reference/logging.ja.md) を参照してください。

## Trusted `haco-host` / WSL entry

v0.26ではlocal Incus pathにpersistent trusted logical Hostを導入します。`haco host ensure` が `haco-host` を作成・reconcileし、`haco host shell` がensure後に入ります。Hacocoonはexact ownershipをmarkし、非owned instanceとのname collisionを拒否し、managed storageへ配置し、raw Incus control socketを `haco-host` の外に残します。WSL login shimにより通常のinteractive distro entryは `haco-host` を開き、明示的なPhysical Host root entryはrecovery escape hatchとして残ります。

real Incus acceptanceはtrusted-host creation、ownership、idempotent ensure、stopped-state recovery、managed-storage behavior、control-socket non-exposureをcoverします。real Windows/WSL interactive-login acceptanceはhost-dependentです。現在のsliceはlifecycle/default-entryまでで、Git/OCI/credential/control-channelの全面移行はfollow-upです。詳細は [`design/trusted-host.ja.md`](design/trusted-host.ja.md) と [`WINDOWS_WSL_BOOTSTRAP.ja.md`](WINDOWS_WSL_BOOTSTRAP.ja.md) を参照してください。

## Client adapter境界

`pkg/clientadapter` がVS Codeに依存しないreusable adapter-facing contractです。canonical Host Workspaceとrequested access modeが完全一致する場合だけEnvironmentをensure/reuseし、guest内Workspaceは `/workspace` として公開します。connection metadataのreconcileとpublic `pkg/interaction` event contractも同じ境界から利用できます。

SSH prepareが受け取るのはpublic-key materialだけで、private keyとIDE configはclientが保持します。返却/reconcileされたSSH/TCP connectionはloopback-onlyか再検証し、provider outputがcontract違反ならrejectします。既存の `haco create` + `haco ssh` + 通常の `ssh` がnon-VS-Code proofです。詳細は [`CLIENT_ADAPTER_CONTRACT.ja.md`](CLIENT_ADAPTER_CONTRACT.ja.md) を参照してください。

## Core と OCI plugin

containerd / nerdctl / Docker は Hacocoon Core の必須要件ではありません。project-maintained OCI plugin profile が必要に応じて containerd + nerdctl や Docker compatibility を提供します。Base lifecycle は `haco base ...`、OCI workload tooling は `haco plugin oci ...` に分離します。

## OCI Seed / storage

v0.17はbuild/publish、operations-hardening、credential-free managed-Environment harvestのrepository sliceを実装済みです。trusted Host acquisition/cache → offline no-NIC Seed Builder → immutable Seed revision/current pointer → exact-parent resolution → normal Incus/storage-driver clone の経路を維持し、複数Environmentで一つのwritable `/var/lib/containerd` を共有しません。

v0.20ではlocal rootfs storageをIncus-owned Btrfsへ統一します。Environment、Tooling Base builder、Seed builder、trusted hostがroot storageを必要とした時点でlocal compositionが `haco-local-default` をlazyにensureします。sparse backing file、loop device、Btrfs filesystem、mount lifecycleはIncusが所有し、Host Workspaceはpool外からbind mountします。

v0.21ではtransparent compressionのdefault policyとしてIncusへ `btrfs.mount_options=compress=zstd:3` を渡します。`compress-force` と `autodefrag` はdesired defaultにせず、既存extentを自動rewriteしてrecompressしません。

v0.25はこのstorage pathのreal ordinary-user acceptance checkpointです。GitHub-hosted Ubuntu 26.04でactual `haco` をreal Incusへ接続し、lazy `haco-local-default` creation、Incus-owned sparse backing image、loop attachment、live Btrfs mount、zstd policy、writable `haco create` / `exec` / `delete` / `run`、pool reuse、guarded cleanupまで確認します。詳細は [`design/btrfs-storage-layout.ja.md`](design/btrfs-storage-layout.ja.md) を参照してください。

Local Registryはprerequisiteではなくroadmap versionも予約しません。残件はauthenticated/private-registry combination、physical Btrfs compression ratio / CPU cost / COW measurement、compaction behavior、broader real-host failure injection、Windows/WSL behaviorなどです。

## Docker compatibility

v0.18のrepository gateは実装済みです。`HACO_PLUGIN_OCI=docker` で `haco plugin oci docker status <environment>` / `prepare <environment>` を使えます。`prepare` はpackage installやHost socket mountをせず、Base/Seed側にDocker CLI、dockerd、containerd、systemd、docker group、Hacocoon-pinned socket/service unitがあることを要求し、unit driftやactive vendor Docker daemonではfail closedします。

## Cloud status

v0.7のprovider-neutral Environment routing seamは維持します。以前のconcrete EC2/AWS/EBS implementationはactive treeから削除済みで、**cloud implementationは現在deferred**です。

## Acceptance gaps

v0.23でGitHub-hosted Ubuntu 26.04上のphased real-Incus substrate + Core lifecycleを、v0.25でordinary-user Incus-owned Btrfs CLI behaviorを、v0.26でtrusted-host lifecycle/control-socket isolationをreal Incusで自動証明するようになりました。ただしproxy-only bridge ACL/dnsmasqを含む全network/resource behavior、Windows/WSL + VS Codeとinteractive `haco-host` entry、private-registry credential、Docker compatibility、physical Btrfs compression/COW/compaction、broader storage failure injection、desktop notification delivery、future cloud adapterなどは引き続きenvironment-dependentです。前のmilestoneにacceptance残件があっても、後続minor checkpointへ進むことは妨げません。

## 保持したEnvironmentの再開

Status: **コマンドとcomponentの範囲はimplemented、ロードマップC/Eはpartial**。
`haco env start <name>` は必須flagを増やさず既存runtimeを再開します。
起動前にactive leaseの同一性とIncusのネットワーク隔離を検証し、Linux/WSLでは
create/start/stop/deleteをcontrollerプロセス間で直列化します。
[ADR 0016](adr/0016-resume-owned-environments.md)を参照してください。
ローカルCIのtest・race段階は全体で成功しました。独立した実Incus 6.0.5-8 / WSLで
停止・再開・再度start・root filesystemとWorkspace内容・保存済みleaseの保持・正規削除が成功しました。
初回fixtureはJSON保存でGoの単調時計情報が失われるためtimestamp比較だけ失敗し、
保存済みlease同士の比較へ修正後の再実行は成功しました。両試験runtimeは除去され、
既存のユーザーEnvironmentは停止状態を維持しています。既存Incus E2Eにもtrusted Hostからの
製品stop/startとWorkspace保持の検証を追加し、`f8517ba`のGHAで成功しました。
SSH setup自動化とHost再起動後に欠けたguardを復元する処理は未実装です。

## 通常APIからのSSH公開ホスト鍵取得

Status: **protocolの範囲はimplemented、SSH setup自動化はplanned**。
IncusのSSH準備は構造検証済みEd25519 `host_public_key` を通常のcontroller応答で返します。
native clientは別途管理者としてIncusを呼ばず鍵を固定できます。不正な鍵では管理対象の鍵と
proxyを撤回し、後始末の失敗はrecovery-requiredとします。公開adapterも任意fieldの鍵を再検証します。
関連race testは成功しました。Windows native受け入れscriptもこのfieldを使うよう更新しましたが、
インストール済みWindows経路も`f8517ba`のGHAで成功しました。

利用者の補足: 開発sourceはHacocoonの作業branchへpushしてPRを出し、Git push機能の
検証先は引き続きHacocoon-testに限定します。PR #482でv0.30/v0.31とSSH公開鍵の範囲を
`f8517ba`として公開しました。以前の公開拒否は本体pushの明示許可により解消しました。
B4の必須defaultを明確化しました。Environment作成時にOCIイメージの公開・COWコピーを
自動実行し、任意のOFF指定だけを設けます。公開済みsourceのコピーは実装済みですが、Hostイメージ公開は
完了していません。詳細はStoreの所有文書に記録しています。

## Workspace Storeの自動初期化

Status: **公開済みsourceのコピー・再利用・OFF指定はimplemented、B4全体はpartial**。
通常のEnvironment作成で任意OCI連携の既定resolverを呼びます。readyかつsource-onlyの
`oci-source:host`をWorkspaceに永続的に対応付けたStoreへコピーし、再作成では再利用します。
`--no-oci`で省略できます。公開元がない場合も非OCI作成は利用可能です。公開元の直接接続、
別Workspaceへの流用、不完全な公開/コピー、失敗を空データで成功扱いする経路は拒否します。
HostのDocker/nerdctlイメージproducerは未実装で、自動イメージ配布全体の完了ではありません。
Docker/runtime互換性も未検証です。関連回帰/race testが成功しました。実Incus/Btrfsの合成データ
fixtureも既定resolverを通し、COW親子関係・独立書込み・source削除・後始末を確認しました。
イメージ取得やruntime利用を証明する試験ではありません。PR #482の`f8517ba`ではWindows
native SSHを含む4つのGHA workflowが成功しました。以降の変更には別のCI結果が必要です。
private-registry E2Eはworkflow-dispatch限定のためSKIPです。
追加依頼の `docker run --rm` 相当は VS Code 接続確認後に製品 CLI へ接続しました。
実 Incus の受入結果は下の一時実行節で区別します。

## runtime側でのSSH自動ポート選択

Status: **implemented、Windows GHA bcc1baf の受入は成功**。
`haco env ssh --key <public-key-file> <name>`はポート引数が不要になりました。
SSHポート0をIncus runtimeへ渡し、Physical Hostで選択してからguestの鍵変更前に
proxyを確保します。Windows native E2Eもこの通常defaultを使い、実際のproxyを
確認し、bcc1baf で成功しました。鍵・config 自動設定と実際の VS Code 接続も確認済みです。

## Desktop SSH setupとVS Code起動

状態: **command は implemented、Windows GHA の editor 接続は成功**。
`haco ssh setup [name]` は desktop 所有の鍵と厳密な host-key pin を準備します。
`haco open [--client vscode|ssh] [name]` で client を選択し、Environment が1つなら名前を省略できます。
インストール済み GHA は native SSH、停止からの再開、接続の再利用に加え、`4f1f512` で実際の editor と terminal 接続を確認しました。
[client の契約](design/client-adapters-and-vscode-integration.md#desktop-ssh-setup-and-vs-code-opening) を参照してください。

native Windows fixture では鍵・config 作成と、インストール済み trusted Host 経由での editor 探索も確認しました。
別の開発用 Ubuntu からの試行は PowerShell の `exec format error` で失敗しました。
cold entry 後の raw Incus fixture は Host 停止中で失敗し、通常の対話 entry 後に成功しました。
これらの準備 fixture だけでは接続成功を証明しません。ローカルの version/許可の残件と GHA の正確な範囲は文書冒頭に記録しています。

日常 CLI の確認は **C6 の一部** です。`haco env list` は登録された名前・Workspace・Base を表示し、
スクリプトでは `--json` を使えます。list/status は端末制御文字を escape します。広い DNS・接続診断は未完了です。

## 保存した Host カスタマイズ

状態: **明示 setup/replay は implemented、Windows GHA は bcc1baf で成功**。
利用者が選んだ UTF-8 Bash 手順を controller が private に保存し、所有権を確認した trusted Host だけで実行します。
通常の setup で保存内容を再実行し、明示した script 更新で置き換え、clear で実行せず解除します。
Environment へ渡しません。回帰テストは file/link 保護、直列化、script 失敗前の保存、service 再作成後の replay、
対象の所有権、標準入力での受渡し、秘密を含まない失敗通知を確認します。
Windows GHA bcc1baf（run 34103036390、job 101681633357）で通常の保存・再実行・更新・解除が成功しました。
test・Ubuntu・Incus workflow も成功しました。
[Host カスタマイズ](design/trusted-host.ja.md#保存したカスタマイズ手順) を参照してください。

controller setup 外の暗黙の Host 再作成は未検証です。ユーザーの installation では、このカスタマイズや任意の package/dotfile 手順を実行していません。
当初のソース編集の自動レビュー拒否は、ロードマップ C2 の明示要件を確認し、同じソース編集をその根拠で再審査して解消しました。
保留中のローカル package policy の許可とは別の事項です。

## 一時実行

状態: **product CLI は implemented、実 Incus の検証は 4adfe19 で成功**。
`haco run [--rm] -- <command>` は既定で所有権付きの一時 Workspace を作り、
/workspace から実行し、runtime と自動 OCI copy を削除します。
`--workspace` は既存 Workspace と Store を保持し、`--no-oci` は自動コピーを無効化します。
一時 identity を作成前に記録し、canonical deletion が lifecycle lock 内で照合します。
resource cleanup も Workspace binding を原子的に確認します。失敗時は回復証拠を残します。
非ゼロ終了、片付け失敗、中断を区別し、stdin/TTY は未実装です。

race 回帰は既存作業の保護、片付け失敗と回復、既定の一時 source、OCI 公開元の保持、
provider の明示対応と argv の保持を検証します。実 Incus GHA の 4adfe19（run 34115004878、job 101719650209）で通常 CLI の成功、exit 17、
既存ファイルへの書き込み、中断後の実体不在を確認しました。
内容入り OCI image 実行とローカル installed acceptance は未検証です。
[一時実行](design/temporary-execution.ja.md) と
[ADR 0020](adr/0020-runtime-owned-temporary-workspaces.md) を参照してください。

`5f824b4` は Ubuntu・Incus が成功しました。Go 1.26/1.27 test/vet・race・docs も成功しましたが、
test workflow は失敗しました。Capability E2E が古い承認表示を期待し、別ジョブでは
GoReleaser 導入が HTTP 504 でした。表示期待値を更新し、ローカルの保存・再利用・
Environment 範囲の E2E と製品 CLI E2E は成功しました。
Windows run 34133686648 は DNS・通常 SSH の再利用/再開・VS Code が成功し、
project setup で失敗しました。実運用 runner decorator が Incus exec の optional な
stdin 契約を引き継ぐ修正を加えています。修正後の installed 検証は再実行待ちです。

保存 Policy に Environment 単位・全 Environment の毎回承認を追加しました。terminal で今回の許可・拒否を別に確認し、ask の保存から allow を作りません。通常の Git/通知への統合と、それらを不変の Environment identity に結び付ける作業は未完了です。

`347ca50` は test・Ubuntu・Incus workflow が成功しました。Windows run 34135390824 では DNS・VS Code と project setup の保存・再実行・非ゼロ終了・更新・削除が成功し、preview server recipe で失敗しました。fixture は Python がなければ準備し、loopback listener の起動を待つようにしました。前回の失敗原因はまだ確定していません。preview・Edge・Environment doctor の実機検証は未完了です。単発承認・最初から許可された要求も実行直前に Policy を再評価する修正は、関連 race test が成功しました。

Environment 作成時に canonical lease へランダムな instance ID を予約します。既存の整合した ready 状態には catalog lock 内で一度だけ付与します。保存 Policy・承認表示・監査で ID を扱い、実運用 Git は取得後、実行直前にも再確認します。同名再作成に識別済み保存方針を引き継ぎません。State/Workspace/Core と Capability/controller/Git の race test は成功しました。通常の Git 保存範囲/UI と network identity 統合は partial です。[ADR 0025](adr/0025-environment-approval-identity.ja.md)を参照してください。

`bffc3fd` は test・Ubuntu・Incus が成功しました。Windows run 34136858725 は VS Code と project setup が再度成功し、拡張子のない preview marker が PowerShell に byte 列で返ったため、内容確認で失敗しました。text/plain fixture への修正は installed 検証待ちです。Git pending には追加引数なしで trusted な作成識別子を表示します。

d4aef8d では 4 workflow が成功しました。Windows run [34139245378](https://github.com/SLktEx/Hacocoon/actions/runs/34139245378) で VS Code の実接続、project setup の保存・再実行・非ゼロ終了・更新・削除、Edge headless の preview 描画、HTTP preview の再利用・終了・接続拒否、Environment doctor の前提確認が PASS です。上記の preview 受け入れ待ちは解消しました。既定ブラウザの起動や物理端末の受け入れを証明するものではありません。VPN／NRPT は VPN と private name の fixture がないため SKIP です。

実運用の Capability service は全ての名前付き要求を trusted catalog の作成 ID に結び付け、実行直前にも照合します。env 限定の保存には ID が必須ですが、利用者の引数は増えません。通常の Git 保存範囲・UI と実 network/provider の受け入れ確認は partial です。

5272434 の GHA では Go 1.26／1.27 の tests・vet、race、release-config、docs、Ubuntu、Incus が PASS です。test workflow は orchestrator E2E で未作成の名前を承認元に使っていたため失敗しました。fixture を通常の create／delete に直し、ローカル E2E は PASS しました。Capability の保存範囲・再作成と Git transport 拒否の E2E も PASS です。

local CI 全体は docs／workflow 検査後、WSL の pwsh 不在で失敗し、それ以降の工程はその呼び出しでは未実行です。Go 工程の個別実行では、空の select が SIGKILL 用 helper を deadlock 終了させ、親が生存中の lock を確認する前に解放するテスト不具合が見つかりました。制限時間付き timer で親からの kill まで生存させ、実 subprocess／SIGKILL の回帰 20 回と run package の race 検証が PASS です。cleanup の権限を変える修正ではありません。
