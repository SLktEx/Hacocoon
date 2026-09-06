# 実装状況

## Incus起動時のPID再利用防止

Status: **implemented。repository回帰は成功、配布providerの受入はpending**。
共通Ubuntu/WSL installerはroot専用のIncus ExecStartPre guardを導入する。
前namespaceのdnsmasq/proxy PID記録をdaemon起動前に退避し、同一namespaceの記録と
resourceデータを保持する。[Host契約](design/trusted-host.ja.md#incus起動時のpid記録)と
[ADR 0013](adr/0013-incus-pid-record-boot-identity.md)を参照。

19件のcomponent回帰で再利用PID、WSL/native起動、service再起動、初期導入、同時実行、
中断復帰、不正metadataを確認した。installer/packageとWindows driver回帰も成功した。
Windows配布gateには再起動後のmarker更新とdnsmasq記録の退避確認を追加した。
同一namespace内のhelper PID再利用と任意device familyは上流側の残課題とする。

ユーザーの現在のinstallationには**未適用**。自動承認reviewが未レビューのdrop-in配備を
拒否したため、その試行ではinstall済み設定を変更していない。hosted受入とmerge状況は
確認後に記録する。本修正はcheckpoint v0.28内の保守修正とする。

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

## 第二段階

状態は**implemented・以下のWindows/WSL構成でB1〜B6を受入済み**。
Dockerとnerdctlの両方で一方向配布・独立起動が成功した。
選択したB5/B6改善と、影響を受けるAの基本導線も確認済み。
[利用手順](reference/managed-repository-workflow.md)、
[OCI契約](design/oci-image-distribution.md)、
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
[再現手順](design/oci-image-distribution.md)を参照。

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

Hacocoon は pre-1.0 です。現在のmilestone位置は **v0.28** です。milestoneは軽量なdevelopment checkpointとして扱い、v0.17のacceptance残件のようなpartial状態があっても、後続の実装済みcheckpointへ進めます。repository実装は、明示的に名前を付けたacceptance checkを除き、すべてのreal-host supportを意味しません。

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
