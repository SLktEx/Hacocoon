# Trusted `haco-host`

実装済み: Host の `haco setup` は、時間制限付きの読み取り専用 Ping でコントローラーの準備を待ち、setup を一度だけ送ります。setup の失敗応答は自動再試行しません。systemd のサービス起動からソケットの準備完了までの差を吸収し、CLI の手順は増やしません。


WSL の自動入室は、実ユーザーの対話 shell と、systemd ユーザーセッション準備用に
別 PTY で起動される背景の `login` shell を区別する。login が管理する shell は
Physical Host の通常 Bash に留まり、実ユーザーの入室だけがコントローラー経由の
Host 準備を要求する。親コマンドの識別は UI の選択であり権限を与えない。
setup の排他と peer 認可は変更しない。[ADR 0064](../adr/0064-wsl-login-bootstrap-routing.md) を参照。


## 通知バイナリ

実装済み: setup は同じリリースの `/usr/local/bin/haco-notify` も配布し、プロバイダーの
変更前に必要な全バイナリを検証します。Host 所有権・ダイジェスト・root 所有の実行権限は
既存の検証を再利用します。通知はコントローラーモードで既存の管理接続を読み、Physical Host
の監査ファイルを必要としません。[イベント契約](../reference/interaction-events.ja.md)を参照してください。
この追加変更の新規パッケージでの受入確認は未完了です。


## Incus起動時のPID記録

共通Ubuntu インストーラーはIncus起動前にプロバイダー専用保護処理を実行する。
PID 名前空間の起動が変わった場合だけ以前のnetwork/proxy プロセス記録を退避し、
再利用PIDを新しいIncus workerへ送信する経路を防ぐ。同一名前空間内のサービス再起動では
記録を保持する。不明・危険なメタデータは起動を拒否し、プロセスへのsignal送信や
resource・Workspaceの削除は行わない。初期導入、永続化、trust 境界と上流に残る範囲は
[ADR 0013](../adr/0013-incus-pid-record-boot-identity.md)を参照。

## Windows連携

通常のWindows インストーラーは、WSLが既に変換したWindows由来PATHだけを収集し、
Physical Hostのroot所有設定へ保存する。コントローラー経由のsetupは、実在するDrvFs
ルート、読み取り専用の`/init`とWSL interop ソケットディレクトリを、所有確認済みの
`haco-host`へ投影する。drive letterの固定一覧は実装しない。Physical HostのLinux
PATHはコピーせず、信頼された Host自身のLinux PATHを維持する。

新規 WSLが登録したnative `WSLInterop` binfmtと`/init`を再利用し、安定したinit
ソケットパスとWindows PATHを信頼されたシェルへ設定する。ソケットディレクトリは再作成される
`/run`の外へ読み取り専用でマウントし、標準systemd tmpfilesが起動時に`/run/WSL`への
symlinkを復元する。これによりWSLの絶対パス symlinkを保持する。別のbinfmt handlerや
独自Windows executable launcherは作らない。正常な登録は変更せず、消失した場合だけ
WSL自身が生成したsystemd 連携で復元する。検証は`flags: P`または`flags: PF`だけを
許可し、enabled・`/init`・offset 0・magic `4d5a`の登録項目は完全一致を要求する。
復元後も含めて全`WSLInterop*` entryを確認し、無効化・未許可・非互換な登録は拒否する。
新しい信頼されたシェルで次を実行できる。

```bash
cmd.exe /c ver
powershell.exe -NoProfile -NonInteractive -Command "[Console]::Out.WriteLine('hello'); exit 23"
echo $?  # 23
```

UNCの作業ディレクトリを受け付けないWindows ツールでは、先に利用可能な投影済みWindows
ディレクトリへcdする。read/writeは同じWindows ファイルシステムを変更し、Environmentや
Hostを作り直してもファイルは残る。Windows ACLは引き続き適用される。projection デバイスは
Incus設定に保持され、setupが識別を確認する。インストーラーと通常setupは再実行可能。
drive着脱・再接続と汎用復旧は後続対象とする。

マウント・PATH・Windows実行権限は信頼された `haco-host`専用。Environmentは共有プロファイルを
継承せず、/init、WSL ソケット、Windows drive、信頼されたコントローラーソケットを受け取らない。
[ADR 0009](../adr/0009-trusted-host-windows-interop.md)と、commitを固定した新規 install・
再起動の[実機結果](../IMPLEMENTATION_STATUS.ja.md)を参照。

Status: partial.

管理対象repoのWSL経路は **実装済み**。登録上流のcloneとGitHub認証は
信頼された Hostに置く。独立Workspace ボリュームコピーはEnvironment利用前にHostから外す。
Git専用broker要求は固定した信頼された Git操作だけを呼び、コントローラー・Policy・Incus権限は
Physical Hostが保持する。[利用手順](../guides/git-workflow.md)と
[ADR 0008](../adr/0008-managed-repository-workspaces.md)を参照。
Windows drive・exe連携は通常installer/setupで構成する（上記参照）。

現在の製品hacoはコントローラー経由のsetup/doctor、WSL通常入口、repo・Workspace・Environment・Git・OCI Storeの操作を提供する。保持したhacoq aliasは旧実装資産であり、新hacoのsubprocess依存ではない。

現在のパッケージのWindows受入と未確認項目は[実装status](../IMPLEMENTATION_STATUS.ja.md)に記録する。製品診断は[読み取り専用コントローラー契約](controller-client-transport.ja.md#host診断)を使う。

## 概要

`haco-host` は Hacocoon が管理する永続的な信頼された logical Host です。Local Incus バックエンドでは `haco-host` という名前の Incus system instance として実装し、通常の信頼しない Environment とは明確に分離します。

Hacocoon コントローラー、Incus daemon、loop デバイス、ストレージマウントを実際に動かす Linux / WSL distribution は **Physical Host** です。Physical Host は platform primitive の権限を持ち続けます。`haco-host` は利用者が普段入る host-like な場所であり、管理リポジトリ・Git・任意の外部サービスツールの実行場所です。

```text
Physical Host / WSL
  |- haco-controller
  |- Incus daemon
  |- loop / Btrfs platform primitives
  `- haco-host                         TRUSTED
       |- haco-host CLI
       |- guard付きgeneral haco client
       `- Hacocoon controller UDS only

Managed Environments                   UNTRUSTED
```

`haco-host` は信頼する基盤の一部です。Environment ではなく、エージェント sandbox として扱ってはいけません。

## 現在実装済みの slice

現在は次を実装しています。

- `haco setup`: 永続的な `haco-host` を1個照合・調整
- 通常の`wsl -d Hacocoon`入口と、保持した旧実装 `hacoq host shell` alias
- `user.hacocoon.role=trusted-host` 所有権識別情報
- Hacocoon-managed Incus ストレージ上へのrootfs配置
- provider-local 衝突を避けるためEnvironment名`host`を予約
- Physical Host上の`haco-controller` Unix-domain 接続先
- 信頼された instanceだけに付与する専用`haco-control` proxy
- ダイジェスト / 所有権を検証した`/usr/local/bin/haco-host` 配備
- 同じ元データ / ダイジェスト / メタデータ基準を使う同じリリースの `/usr/local/bin/haco` 配備
- 未移行`haco` コマンドがguest-local 構成へ暗黙のに落ちることを防ぐ`environment.HACO_CLIENT_MODE=controller`
- `haco-host doctor`を確認してから既定 interactive entryを有効化する対応している WSL 初期設定

Trusted Host全体の名前空間整理、cloud 認証情報、汎用external ツールはまだ部分実装。上記のGit/GitHubとWindows連携は実装済み。標準のローカル setup は下記の Host ツールを導入します。Core と Environment の実行基盤の選択は独立したままです。

## Trust と authority

Incus control 権限とauthoritative Hacocoon 状態はPhysical Hostに残します。

`haco-host`には次を渡しません。

- `/var/lib/incus/unix.socket`
- `/var/lib/incus/unix.socket.user`
- `/var/lib/incus`
- Physical HostのHacocoon 状態ディレクトリ
- 生の provider-control ソケットのマウント

代わりに1本だけ狭いコントローラーパスを渡します。

```text
haco-host process
  |
  | unix:/var/lib/hacocoon-control.sock
  v
Incus proxy device: haco-control
  |
  | unix:/run/hacocoon/control.sock
  v
Physical Host haco-controller
  |
  v
policy / state / provider authority
```

通常のEnvironmentにはこのproxy、control-socket environment variable、信頼された controller-client モード識別情報のいずれも渡しません。

Environmentからprivileged operationを要求する場合も、暗黙に継承するな信頼された Host accessにせずHacocoonの方針 / capability / 承認境界を通します。

## Ownership と name collision

Incus instance名`haco-host`はinfrastructure-ownedです。

作成時に`incus init`と同時に所有権識別情報を設定します。既存instanceを再利用する場合は正確な識別情報を要求します。無関係なinstanceが`haco-host`を占有している場合、takeover、起動、削除、デバイス変更をせず安全側で拒否します。

通常のEnvironment名`host`もprovider-localでは`haco-host`になるため、Incus mutation前に拒否します。

Concurrent 作成 / デバイス照合・調整 raceは、最終的なowned 状態が期待値へ完全一致した場合だけ受け入れます。

## Controller endpoint

プロキシ、ソケットの権限・所有者、実行モードの正確な設定は[通信契約](controller-client-transport.ja.md#trusted-haco-host-endpoint)に集約します。Physical Host のコントローラー接続は特権です。正確に所有する信頼された Host だけへ公開し、対象・所有者・権限・接続方向・既存の実行モードが異なる場合は拒否します。ゲスト起動時の tmpfs 初期化で隠れないよう、インスタンス側のソケットは /run の外に置きます。

## Client provisioning

`haco setup`はreleaseのクライアントバイナリを2本とも配備します。

```text
/usr/local/bin/haco-host
/usr/local/bin/haco
```

Physical Host側元データは通常の executable、invoking effective UID所有、group/other writableではないことを要求します。SHA-256とfinal `0755 root:root` メタデータを比較して、必要な場合だけpushします。

これによりrepeated ensureを繰り返しても同じ結果になるにし、信頼された instance内の任意の既存バイナリをそのまま信頼しません。

製品 `haco` はguest-local 構成へ代替経路せず、`hacoq` も呼び出しません。一時的な `hacoq` は未移行操作のためPhysical Host配布物に残るが、新規 trusted-host setupでは配備しない。既存guest内のコピーは製品の依存ではない。controller-mode 保護処理は引き続きguest-local操作を拒否する。

このモード識別情報はauthorization 認証情報ではありません。`haco-host`自体が信頼されたであり、方針、状態、プロバイダー operationの権限は引き続きPhysical Host コントローラーです。

## 専用trusted-host network

Incus アダプターは既定 resource projectの `haco-host0` を所有し、`user.hacocoon.owner=trusted-host-network-v1` で識別する。利用前に所有者、管理対象の bridge型、非公開 IPv4 subnet、DHCP/DNS/NAT/routing/firewall設定、利用対象を検証する。不明なrouting/DNS override、external interface、別の利用対象は安全側で拒否。最初のtrusted-network契約ではIPv6を無効にする。

Ubuntu インストーラーはIncus bridgeのDNS/DHCP用に `dnsmasq-base` を明示的に導入する。Incusがrecommended パッケージなしで導入済みの場合も対象とする。パッケージ導入に失敗した場合はdaemon準備確認やtrusted-host setupへ進まず停止する。追加の `haco` オプションや手動DNS設定は不要。

Fresh 信頼された Hostはlocal NIC/root diskを明示し、プロファイルを継承しない。common インストーラーはIncusの準備を確認し、minimal初期化や既定ディレクトリプール作成を行わない。既知の既定プロファイル・`incusbr0` NICを持つ正確に所有した既存Hostだけを一度graceful 停止し、明示的NICへ移行して再開する。root disk・UUID・ファイルを保持し、不明なprofile/deviceは移行せず失敗する。中断した移行は再実行で回復でき、旧shared bridge/profile/poolは削除しない。

Bootstrap/入口の前にIPv4転送を検査し、Dockerの `DOCKER-USER` 拡張点がある場合に照合する。2つの規則はこのbridge/subnetからの送信とestablished/relatedの戻り通信だけに一致する。global FORWARD 方針とEnvironment bridgeは変更せず、対応する拡張点なしのDROPは明示的に失敗する。対話セッション中のfirewall reloadや後発Docker起動を常時監視する実装ではなく、次の入口で再検査する。

Installerは成功を表示する前に、実際の信頼された Host内でDNS・既定 IPv4 route・HTTPSを確認する。これはEnvironmentのproxy/default-deny受入とは別の基盤検証。[ADR 0005](../adr/0005-trusted-host-network-ownership.md)を参照。リポジトリ回帰と隔離Linuxのパケット検証は、最終packaged Windows受入と区別する。

## Storage

`haco-host`は通常のHacocoon Incus ストレージ連携が選んだroot ストレージプールを使います。Default local バックエンドではHacocoonのsparse-raw Btrfs-backed Incus プールにrootfsを置きます。

ただし、同じBtrfs上にあるだけで将来の`haco-host` データがBase イメージ / Environmentと物理的にCOW shareされるとはみなしません。そのclaimは測定依存です。

## WSL default entry

Supported インストーラー成功後、通常non-root WSL 利用者のlogin シェルを専用`hacocoon-login` entryに変更します。

Interactive no-command 起動では次へdelegateします。

```text
controlapi.Client.OpenTrustedHostShell
```

製品aliasはコントローラーへ直接接続し、sudo ルールや `hacoq` subprocessを使いません。root側インストーラーは通常利用者の正確な UID/GIDを保持し、`hacocoon` groupでコントローラー accessを与えます。`incus-admin` は既定で付与しません。[ADR 0004](../adr/0004-wsl-installer-authority.md)を参照してください。

Login シェルを変更する前に初期設定は次をすべて確認します。

1. Incusが稼働中
2. `haco-controller`がroot-owned system バイナリ
3. 現在の releaseで`haco-controller.service`を再起動
4. `/run/hacocoon/control.sock`が `root:hacocoon` モード `0660` Unix ソケット
5. `haco setup`で信頼された Host、proxy、クライアントモード、2本のクライアントバイナリが照合・調整
6. 実信頼された instance内の`haco-host doctor`が成功

すべて成功した後だけ通常entryは次になります。

```powershell
wsl -d Hacocoon
```

```text
Physical Host login entry
    -> product haco login alias -> Physical Host controller
    -> haco-host
```

Explicit WSL コマンドはPhysical Host コマンドのままです。root accountのシェルは変更せず、次の復旧パスを維持します。

```powershell
wsl -d Hacocoon -u root
```

`-SkipIncus`ではコントローラー / 信頼された Host 自動 entryを設定しません。

## Interactive warning

通常の製品CLIによるHost接続は[権限の案内](#host-入口の言語)を表示します。移行用の`hacoq host shell`にも言語設定に応じた短い管理権限の警告があります。通常の開発作業はEnvironmentで行ってください。

## 今後の follow-up

別workとして残るもの:

- 実装済みの Git/GitHub 以外へ、Host で使う外部サービスツールを拡張する
- 任意OCI 実行基盤の対応範囲を拡張（Host の元データ領域と Environment の独立 Store は実装済み）
- 再利用可能な認証情報を通常Environmentへ置かない認証情報 broker
- 実機確認したCLI以外のWindows application互換性を評価
- 残る適切な`haco` コマンドをclassifyしてコントローラークライアントパスへ移行
- 信頼された Host-local operationをlong-termの`haco-host` 名前空間へ移しtemporary ambiguityをなくす
- `haco` / `haco-host` CLI responsibility splitを完了
- Coreがリポジトリを永久に`haco-host`へ固定すると仮定しないWorkspace / リポジトリ location seam

## Acceptance boundary

Repository テストでは所有権照合・調整、衝突拒否、状態復旧、正確なコントローラー proxy 検証、2本のクライアントバイナリ配備 / 再実行時の一貫性、client-mode 不一致拒否、CLI 経路選択、local 代替経路の安全側で拒否する、warning、login-mode identificationを確認します。

維持する実際の Incus E2E gateはコントローラー経由の `haco setup`、接続先投影、必要な2本のクライアントのダイジェスト一致、`haco-host doctor` / `haco-host env ...` のコントローラー経由操作、再起動復旧、新規 setupでguestに旧`hacoq`がないこと、生の Incus ソケット非露出、通常Environmentの信頼された接続先 / client-mode 識別情報非露出を検査する。保持した旧alias・Base 経路選択・local 構成拒否は構成要素テストで検証する。更新gateは `b71f88e` で成功した。commitを固定したWindows結果と残る制約は[実装status](../IMPLEMENTATION_STATUS.ja.md)に記録する。

Windows/WSLの確認済み範囲は、実装statusに記録したcommit固定の実機受入に限る。別hardware・別構成への互換性は未確認として扱う。

## 保存したカスタマイズ手順

Status: **implemented（実装済み）**。所有するIncus Hostの実体ごとに自動適用を記録します。
旧save/replay経路は`bcc1baf`でWindows GHAに合格していますが、新しい自動適用・再作成・
結果確認経路のinstalled受入は最新headで別途必要です。

```bash
haco setup --script ~/host-setup.sh   # 保存し、必須provisioningの後に適用
haco setup                          # 必須処理を照合し、未適用のHostだけ自動実行
haco setup --reapply-script          # 保存したユーザースクリプトだけ再実行
haco setup --script-result           # 最後のstdout・stderr・終了コードを確認
haco setup --clear-script            # 今後の適用を解除
```

通常のcontroller経由Host shell入口も同じ準備経路を使い、Host再作成時も必須provisioningを
完了してからユーザー処理を実行します。成功済みの同じHostでは入口、通常コマンド、upgradeや
reprovisionで再実行しません。providerの`volatile.uuid`が変わると保存内容を再適用します。
元ファイルの変更だけでは保存内容は変わらず、`--script`で明示的に更新します。
`--reapply-script`は必須provisioningを省略し、既に所有・起動済みのHostだけで実行します。
追加した2フラグはHost専用で、Environment指定のWorkspace recipe契約は維持します。

クライアントは最大1 MiBの通常UTF-8ファイルを読みます。実行bitは不要で、UTF-8 BOMと
CRLFを正規化します。`~/`はクライアント側アカウントで解決します。PowerShellではWSLの
Physical Hostに入っているLinuxクライアントを使います。

```powershell
wsl.exe -d Hacocoon --exec /usr/local/bin/haco setup --script 'C:\Users\Example\host-setup.sh'
wsl.exe -d Hacocoon --exec /usr/local/bin/haco setup --script-result
```

Windows driveパスは空白や設定済みmountを含めWSL自身の`wslpath`で解決します。
trusted `haco-host`内では`/mnt/c/Users/Example/host-setup.sh`のような既存の投影先Linuxパスを
使います。native Windows `haco.exe`、手作業の改行変換、chmodは不要です。

controllerの非公開Hacocoon root（通常`/var/lib/hacocoon`）配下の
`host-customization/recipe.sh`と`result.json`に保存します。結果はHost実体、正規化したscriptの
SHA-256、running/succeeded/failed、終了コード、上限付き出力を持ちます。実行前にrunningを
永続化し、完了時に原子的に更新します。失敗・中断・完了不明は同じHostでの自動再試行を拒否し、
controller再起動後も維持します。結果を調べ、`--script`で修正するか`--reapply-script`で明示的に
再試行します。解除後も最後の結果は保持し、実行済み変更は取り消しません。旧版で保存され実行記録が
まだないrecipeは次のsetup/入口で一度適用し、その後はHost実体ごとの規則に従います。

scriptは明示的な再実行と部分的な副作用に耐えるように書いてください。任意コマンドはrollback
できません。所有を検証したHost内のrootとして、作業場所と`HOME`を`/root`に固定し、
`/bin/bash -se`へ上限付きstdinで渡します。固定名`hacocoon-user-setup`のsystemd unitが重複を
拒否し、最長14分（要求期限が短ければそれ以下）で子孫も終了します。controller終了後もこの上限を
維持します。非公開storeのlockで変更・実行を直列化し、不正・リンク・公開権限・別所有者のstateを拒否します。

scriptは既存のHost権限を使えるtrusted codeです。Physical Hostのsocket、credential、環境変数、
mount、controller経路を追加せず、Physical Hostで実行したり通常Environmentへ配布したりしません。
repository hookの自動検出も行いません。

stdout/stderrは別々に64 KiBまで保持し、打ち切りはmarkerと総byte数で明示します。
`--script-result`でのみ表示し、秘密を含み得る明示的なcommand outputとして扱ってください。
application log、progress stage、auditには出しません。非zero終了はsetup失敗、`-1`は確実な
process終了値がない状態です。controller crashでは出力なしのrunning記録が残り得ますが、成功や
自動再実行の許可にはしません。[ADR 0019](../adr/0019-trusted-host-customization.md)を参照してください。

## ネストした OCI runtime

通常の OCI setup は、非特権の所有確認済み Host と正規の ready 元データ
領域を検証してから `security.nesting=true` を設定します。所有権の欠落、継承
プロファイル、pause 中・コピー未完了の状態、曖昧なプロバイダー応答は setup を拒否します。
設定は永続化され、再 setup で確認して再利用します。
[ADR 0032](../adr/0032-owned-host-nested-runtime.md) を参照してください。
標準のローカル連携が Host ツールを導入します。Docker と Environment の実行基盤の
選択は任意のままで、イメージの復旧は実行基盤ごとの受け入れ確認が必要です。

## Host の標準ツール

通常の `haco setup` は、保存済みのユーザースクリプトを実行する前に Git、GitHub CLI、
containerd、nerdctl、BuildKit を導入します。Windows/WSL インストーラーから呼ぶ共通の
Ubuntu setup も同じ経路です。利用者ごとの導入スクリプトは不要です。ツールは所有権を
確認した非特権の `haco-host` 内で root として動作し、Physical Host では実行しません。

| 構成要素 | 対応する導入元・バージョン |
|---|---|
| Git、GitHub CLI (`gh`) | Ubuntu 26.04 以降の設定済み署名付きパッケージリポジトリ（universe を含む）。初回は候補版を導入し、再 setup は導入済みパッケージを再利用 |
| nerdctl | 公式 `nerdctl-full` 2.3.5。Linux amd64/arm64 ごとに SHA-256 を固定 |
| containerd / runc / BuildKit / CNI | 同じ配布物から必要な実行ファイルだけを導入。2.3.3 / 1.5.1 / 0.31.2 / 1.9.1 |

構成要素の出所は[公式配布物](https://github.com/containerd/nerdctl/releases/tag/v2.3.5)を
参照してください。HTTPS で取得して固定ダイジェストを検証し、許可した通常ファイルだけを
導入します。検証済みアーカイブは `/var/cache/hacocoon/host-tooling` に保持し、再 setup
で再取得しません。既存の実行ファイル・設定との衝突、不正なリンク・権限は上書きせず拒否します。
既存データの移行と任意の独自実行基盤の引継ぎは未対応です。イメージを削除して回避せず、衝突を確認してください。

`containerd.service` と `buildkit.service` を有効化し、利用可能になるまで確認します。
nerdctl は既定で `default` 名前空間と `native` snapshotter を使い、containerd の
ダウンロード後の展開設定も一致させます。setup 成功後、信頼済み `haco-host` 内で実行します。

```bash
git --version
gh --version
nerdctl pull docker.io/library/busybox:latest
nerdctl run --rm docker.io/library/busybox:latest echo ready
# Dockerfile があるディレクトリで実行:
nerdctl build -t example:local .
```

イメージと BuildKit キャッシュは `/var/lib/hacocoon-oci/containerd` と
`/var/lib/hacocoon-oci/buildkit` に残り、ソケットは Host 内の `/run` に置きます。
停止・再開・再 setup は管理領域を保持します。既存の
[独立 Store コピー](persistent-oci-store.md#default-environment-creation-flow)の所有権と
pause/copy/resume の契約は維持します。受け取り側 Environment/Base は対応する実行基盤を
別途用意する必要があります。Host のソケット、レジストリ認証情報、管理権限はコピーしません。
Docker は導入せず、既存の管理対象設定とデータを保持します。

失敗は `host_packages`、`host_tooling`、`host_services` の段階で表示し、導入処理の生の
出力は診断に含めません。時間制限付きの一時サービスはコントローラー終了後も重複導入を
拒否します。再試行は完全なファイルを再利用し、不足分を導入します。パッケージ変更の巻戻し、
OCI データの初期化、正常なサービスの再起動は行いません。
[ADR 0063](../adr/0063-standard-trusted-host-tooling.md)を参照してください。
リポジトリ試験と専用 Incus 試験は、公開済み Windows インストーラーや認証付きレジストリの
受け入れ確認とは区別します。

専用の Linux/WSL Incus/Btrfs 試験 Host で root として実行します。

```bash
HACO_E2E_HOST_TOOLING=1 go test -count=1 -run '^TestRealIncusHostToolingE2E$' \
  -v -timeout 18m ./modules/runtime/incus
```

試験は専用のプロジェクトとプールを作り、成功時に削除します。ネットワークは通常の
Host 処理で検証・構成する `haco-host0` を使い、管理基盤として保持します。
既存の別 Host がそのネットワークを利用している場合は拒否します。失敗時は表示した
所有対象を調査用に保持します。結果は[検証証拠](../status/acceptance-evidence.ja.md#installation)を参照してください。


## Host 入口の言語

実装済み: 信頼済み Host へ入るときの案内は、Physical Host のログインプロセス の `LC_ALL`、`LC_MESSAGES`、`LANG` の順で最初の空でない値を使います。日本語の言語設定 なら日本語、それ以外は英語です。Host 権限を使う場所であることと、通常の開発には Environment を使う案内を維持します。対話端末の stderr は `NO_COLOR` が空なら黄色にし、出力のリダイレクト時は色コードを付けません。

Windows の新規インストールでは、日本語の Windows UI 言語を Ubuntu の言語設定ツール で `ja_JP.UTF-8` に設定してから ログインユーザー を準備します。既存ディストリビューションの言語設定 は変更せず、他の Windows 言語は Ubuntu の既定値を維持します。言語設定に失敗した場合はインストールを中断します。表示だけの変更で、Host／Env 権限、コントローラーの準備待ち、認証情報の転送は変更しません。日本語 Windows 上の新規インストールの実機確認は未検証です。

## setupの進捗と失敗診断

状態: **implemented**。`haco setup` は既存の所有権確認付き処理を管理controller経由で観測します。stderrにrunning/succeeded/failedの工程を表示し、stdoutは最終結果用に保ちます。進捗率は推測せず、プロセスを起動しただけで完了にしません。対象はclient検証、project/storage、Hostの所有権確認・作成、network、controller endpoint、起動、WSL interop、client mode/provisioning、Host storage、通知、customizationです。同じ工程の再表示は実際の再確認を表し、未設定の任意工程を完了とは表示しません。

controllerは共有構造化loggerに固定stage/state/reason、所要時間、生成した`request_id`を記録します。CLIも上限付きの固定語彙を再検証します。providerの任意エラー、helperの生出力、秘密、recipe本文は診断欄に含めません。WSL helperの終了値42だけを`native_binfmt_incompatible`と分類し、原因未確定は`failed`のままにします。timeout、canceled、incompatible_state、recovery_required、unavailable、denied、busy、not_found、unsupported等も区別します。

現在の状態は`haco doctor`で確認します。WSL/Linuxの**Physical Host**で管理者が`journalctl -u haco-controller.service --since '30 minutes ago' --no-pager`を実行し、表示されたrequest IDを探せます。保存・ローテーションはsystemd-journaldが管理します。既存の`HACO_LOG_LEVEL=debug`と`HACO_LOG_FORMAT=json`を利用できますが、client側設定でcontrollerのDEBUGを遠隔有効化はしません。DEBUGでもredactionを維持します。

Host setupには承認操作はありません。busyは別setupの実行中を表し、承認待ちとは異なります。Capability承認は`haco approve`で別に扱います。Ctrl+Cは観測を終了し、既存lifecycle RPC同様controllerの時間制限付き処理は接続断後も続く可能性があります。排他は実際の処理終了まで保持します。通信断、最終応答欠落、古いcontrollerとの不一致から変更処理を再送したり成功表示したりしません。

成功した工程表示はその試行の記録であり、現在のresource一覧ではありません。失敗時は作成済みresourceが残り得るためrollbackを約束しません。requestとdoctorの状態を確認してから明示的な再実行を判断します。保存したcustomizationには副作用があり、安易な再実行を案内しません。観測の追加によってcleanup権限、所有権、lease、network、認可の不変条件を変更しません。
