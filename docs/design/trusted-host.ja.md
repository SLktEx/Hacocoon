# Trusted `haco-host`

## Incus起動時のPID記録

共通Ubuntu installerはIncus起動前にprovider専用guardを実行する。
PID namespaceの起動が変わった場合だけ以前のnetwork/proxy process記録を退避し、
再利用PIDを新しいIncus workerへ送信する経路を防ぐ。同一namespace内のservice再起動では
記録を保持する。不明・危険なmetadataは起動を拒否し、processへのsignal送信や
resource・Workspaceの削除は行わない。初期導入、永続化、trust boundaryと上流に残る範囲は
[ADR 0013](../adr/0013-incus-pid-record-boot-identity.md)を参照。

## Windows連携

通常のWindows installerは、WSLが既に変換したWindows由来PATHだけを収集し、
Physical Hostのroot所有設定へ保存する。controller経由のsetupは、実在するDrvFs
ルート、読み取り専用の`/init`とWSL interop socketディレクトリを、所有確認済みの
`haco-host`へ投影する。drive letterの固定一覧は実装しない。Physical HostのLinux
PATHはコピーせず、trusted Host自身のLinux PATHを維持する。

fresh WSLが登録したnative `WSLInterop` binfmtと`/init`を再利用し、安定したinit
socket pathとWindows PATHをtrusted shellへ設定する。socketディレクトリは再作成される
`/run`の外へ読み取り専用でmountし、標準systemd tmpfilesが起動時に`/run/WSL`への
symlinkを復元する。これによりWSLの絶対path symlinkを保持する。別のbinfmt handlerや
独自Windows executable launcherは作らない。正常な登録は変更せず、消失した場合だけ
WSL自身が生成したsystemd integrationで復元する。無効化・非互換な登録は拒否する。
新しいtrusted shellで次を実行できる。

```bash
cmd.exe /c ver
powershell.exe -NoProfile -NonInteractive -Command "[Console]::Out.WriteLine('hello'); exit 23"
echo $?  # 23
```

UNCの作業directoryを受け付けないWindows toolでは、先に利用可能な投影済みWindows
ディレクトリへcdする。read/writeは同じWindows filesystemを変更し、Environmentや
Hostを作り直してもファイルは残る。Windows ACLは引き続き適用される。projection deviceは
Incus設定に保持され、setupがidentityを確認する。installerと通常setupは再実行可能。
drive着脱・再接続と汎用復旧は後続対象とする。

mount・PATH・Windows実行権限はtrusted `haco-host`専用。Environmentは共有profileを
継承せず、/init、WSL socket、Windows drive、trusted controller socketを受け取らない。
[ADR 0009](../adr/0009-trusted-host-windows-interop.md)と、commitを固定したfresh install・
restartの[実機結果](../IMPLEMENTATION_STATUS.ja.md)を参照。

Status: partial.

管理対象repoのWSL経路は **implemented**。登録upstreamのcloneとGitHub認証は
trusted Hostに置く。独立Workspace volume copyはEnvironment利用前にHostから外す。
Git専用broker要求は固定したtrusted Git操作だけを呼び、controller・Policy・Incus権限は
Physical Hostが保持する。[利用手順](../reference/managed-repository-workflow.md)と
[ADR 0008](../adr/0008-managed-repository-workspaces.md)を参照。
Windows drive・exe連携は通常installer/setupで構成する（上記参照）。

現在の製品hacoはcontroller経由のsetup/doctor、WSL通常入口、repo・Workspace・Environment・Git・OCI Storeの操作を提供する。保持したhacoq aliasはlegacy資産であり、新hacoのsubprocess依存ではない。

現在のpackageのWindows受入と未確認項目は[実装status](../IMPLEMENTATION_STATUS.ja.md)に記録する。製品診断は[読み取り専用controller契約](controller-client-transport.ja.md#host診断)を使う。

## 概要

`haco-host` は Hacocoon が管理する永続的な trusted logical Host です。Local Incus backend では `haco-host` という名前の Incus system instance として実装し、通常の untrusted Environment とは明確に分離します。

Hacocoon controller、Incus daemon、loop device、storage mount を実際に動かす Linux / WSL distribution は **Physical Host** です。Physical Host は platform primitive の authority を持ち続けます。`haco-host` は user が普段入る host-like な場所であり、今後の developer / external-service tooling の標準実行場所です。

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

`haco-host` は trusted computing base の一部です。Environment ではなく、agent sandbox として扱ってはいけません。

## 現在実装済みの slice

現在は次を実装しています。

- `haco setup`: 永続的な `haco-host` を1個reconcile
- 通常の`wsl -d Hacocoon`入口と、保持したlegacy `hacoq host shell` alias
- `user.hacocoon.role=trusted-host` ownership marker
- Hacocoon-managed Incus storage上へのrootfs配置
- provider-local collisionを避けるためEnvironment名`host`を予約
- Physical Host上の`haco-controller` Unix-domain endpoint
- trusted instanceだけに付与する専用`haco-control` proxy
- digest / ownershipを検証した`/usr/local/bin/haco-host` provisioning
- 同じsource / digest / metadata基準を使うsame-release `/usr/local/bin/haco` provisioning
- 未移行`haco` commandがguest-local compositionへsilentに落ちることを防ぐ`environment.HACO_CLIENT_MODE=controller`
- `haco-host doctor`を確認してからdefault interactive entryを有効化するsupported WSL bootstrap

Trusted Host全体のnamespace整理、cloud credential、汎用external toolingはまだpartial。上記のGit/GitHubとWindows連携はimplemented。現行OCI StoreはEnvironmentだけへattachし、Host runtimeを必須にしない。

## Trust と authority

Incus control authorityとauthoritative Hacocoon stateはPhysical Hostに残します。

`haco-host`には次を渡しません。

- `/var/lib/incus/unix.socket`
- `/var/lib/incus/unix.socket.user`
- `/var/lib/incus`
- Physical HostのHacocoon state directory
- raw provider-control socketのmount

代わりに1本だけ狭いcontroller pathを渡します。

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

通常のEnvironmentにはこのproxy、control-socket environment variable、trusted controller-client mode markerのいずれも渡しません。

Environmentからprivileged operationを要求する場合も、ambientなtrusted Host accessにせずHacocoonのpolicy / capability / approval boundaryを通します。

## Ownership と name collision

Incus instance名`haco-host`はinfrastructure-ownedです。

作成時に`incus init`と同時にownership markerを設定します。既存instanceを再利用する場合はexact markerを要求します。無関係なinstanceが`haco-host`を占有している場合、takeover、start、delete、device変更をせずfail closedします。

通常のEnvironment名`host`もprovider-localでは`haco-host`になるため、Incus mutation前に拒否します。

Concurrent create / device reconciliation raceは、最終的なowned stateが期待値へ完全一致した場合だけ受け入れます。

## Controller endpoint

Physical Host controllerは次を使います。

```text
/run/hacocoon/control.sock
```

Supported WSL bootstrapでは `haco-controller` をsystemdで常駐させ、socketが `root:hacocoon` mode `0660` であることを検証します。`hacocoon` groupは特権controller権限を与えます。trusted-instance側proxyは下記のroot-only設定を維持します。

Trusted instanceにはexactに次のproxyを設定します。

```text
device: haco-control
type=proxy
bind=instance
listen=unix:/var/lib/hacocoon-control.sock
connect=unix:/run/hacocoon/control.sock
mode=0600
uid=0
gid=0
```

さらに次を設定します。

```text
environment.HACO_CONTROL_SOCKET=/var/lib/hacocoon-control.sock
environment.HACO_CLIENT_MODE=controller
```

既存endpointのtarget、mode、owner、bind方向、socket pathが異なる場合はincompatible stateとして拒否し、silent repurposeしません。

Client modeも想定外のnon-empty値ならincompatible stateとして拒否します。Trusted instanceに既に別のexecution-context policyがある場合、それをsilent overwriteしません。

Instance側socketを`/run`配下に置かないのは、guest runtime tmpfs initializationによってIncus proxy listenerが隠れるboot-order依存を避けるためです。

## Client provisioning

`haco setup`はreleaseのclient binaryを2本ともprovisionします。

```text
/usr/local/bin/haco-host
/usr/local/bin/haco
```

Physical Host側sourceはregular executable、invoking effective UID所有、group/other writableではないことを要求します。SHA-256とfinal `0755 root:root` metadataを比較して、必要な場合だけpushします。

これによりrepeated ensureをidempotentにし、trusted instance内の任意の既存binaryをそのまま信頼しません。

製品 `haco` はguest-local compositionへfallbackせず、`hacoq` も呼び出しません。一時的な `hacoq` は未移行操作のためPhysical Host配布物に残るが、fresh trusted-host setupでは配備しない。既存guest内のcopyは製品の依存ではない。controller-mode guardは引き続きguest-local操作を拒否する。

このmode markerはauthorization credentialではありません。`haco-host`自体がtrustedであり、policy、state、provider operationのauthorityは引き続きPhysical Host controllerです。

## 専用trusted-host network

Incus adapterはdefault resource projectの `haco-host0` を所有し、`user.hacocoon.owner=trusted-host-network-v1` で識別する。利用前にowner、managed bridge型、private IPv4 subnet、DHCP/DNS/NAT/routing/firewall設定、利用対象を検証する。不明なrouting/DNS override、external interface、別の利用対象はfail closed。最初のtrusted-network契約ではIPv6を無効にする。

Fresh trusted hostはlocal NIC/root diskを明示し、profileを継承しない。common installerはIncusの準備を確認し、minimal初期化やdefault directory pool作成を行わない。既知のdefault profile・`incusbr0` NICを持つ正確に所有した既存hostだけを一度graceful stopし、明示的NICへ移行して再開する。root disk・UUID・fileを保持し、不明なprofile/deviceは移行せず失敗する。中断した移行は再実行で回復でき、旧shared bridge/profile/poolは削除しない。

Bootstrap/入口の前にIPv4転送を検査し、Dockerの `DOCKER-USER` 拡張点がある場合に照合する。2つの規則はこのbridge/subnetからの送信とestablished/relatedの戻り通信だけに一致する。global FORWARD policyとEnvironment bridgeは変更せず、対応する拡張点なしのDROPは明示的に失敗する。対話session中のfirewall reloadや後発Docker起動を常時監視する実装ではなく、次の入口で再検査する。

Installerは成功を表示する前に、実際のtrusted host内でDNS・default IPv4 route・HTTPSを確認する。これはEnvironmentのproxy/default-deny受入とは別の基盤検証。[ADR 0005](../adr/0005-trusted-host-network-ownership.md)を参照。repository回帰と隔離Linuxのpacket検証は、最終packaged Windows受入と区別する。

## Storage

`haco-host`は通常のHacocoon Incus storage integrationが選んだroot storage poolを使います。Default local backendではHacocoonのsparse-raw Btrfs-backed Incus poolにrootfsを置きます。

ただし、同じBtrfs上にあるだけで将来の`haco-host` dataがBase image / Environmentと物理的にCOW shareされるとはみなしません。そのclaimはmeasurement依存です。

## WSL default entry

Supported installer成功後、通常non-root WSL userのlogin shellを専用`hacocoon-login` entryに変更します。

Interactive no-command launchでは次へdelegateします。

```text
controlapi.Client.OpenTrustedHostShell
```

製品aliasはcontrollerへ直接接続し、sudo ruleや `hacoq` subprocessを使いません。root側installerは通常userのexact UID/GIDを保持し、`hacocoon` groupでcontroller accessを与えます。`incus-admin` はdefaultで付与しません。[ADR 0004](../adr/0004-wsl-installer-authority.md)を参照してください。

Login shellを変更する前にbootstrapは次をすべて確認します。

1. Incusがactive
2. `haco-controller`がroot-owned system binary
3. current releaseで`haco-controller.service`をrestart
4. `/run/hacocoon/control.sock`が `root:hacocoon` mode `0660` Unix socket
5. `haco setup`でtrusted Host、proxy、client mode、2本のclient binaryがreconcile
6. 実trusted instance内の`haco-host doctor`が成功

すべて成功した後だけ通常entryは次になります。

```powershell
wsl -d Hacocoon
```

```text
Physical Host login entry
    -> product haco login alias -> Physical Host controller
    -> haco-host
```

Explicit WSL commandはPhysical Host commandのままです。root accountのshellは変更せず、次のrecovery pathを維持します。

```powershell
wsl -d Hacocoon -u root
```

`-SkipIncus`ではcontroller / trusted Host automatic entryを設定しません。

## Interactive warning

`hacoq host shell`は`haco-host`へ入る前に短いprivileged-management warningを表示します。Japanese localeでは日本語、その他では英語です。

Warningはinteractive Host-shell pathだけに出し、non-interactive WSL commandのoutputには混ぜません。

## 今後の follow-up

別workとして残るもの:

- Git/GitHubやselected external-service toolingの標準実行場所を`haco-host`にする
- 任意OCI runtimeの対応範囲を拡張（現行StoreはEnvironmentだけにattach）
- reusable credentialを通常Environmentへ置かないcredential broker
- 実機確認したCLI以外のWindows application互換性を評価
- 残る適切な`haco` commandをclassifyしてcontroller client pathへ移行
- trusted Host-local operationをlong-termの`haco-host` namespaceへ移しtemporary ambiguityをなくす
- `haco` / `haco-host` CLI responsibility splitを完了
- Coreがrepositoryを永久に`haco-host`へ固定すると仮定しないWorkspace / repository location seam

## Acceptance boundary

Repository testではownership reconciliation、collision refusal、state recovery、exact controller proxy validation、2本のclient binary provisioning / idempotency、client-mode drift refusal、CLI routing、local fallbackのfail-closed、warning、login-mode identificationを確認します。

維持するreal Incus E2E gateはcontroller経由の `haco setup`、endpoint投影、必要な2本のclientのdigest一致、`haco-host doctor` / `haco-host env ...` のcontroller経由操作、restart復旧、fresh setupでguestに旧`hacoq`がないこと、raw Incus socket非露出、通常Environmentのtrusted endpoint / client-mode marker非露出を検査する。保持した旧alias・Base routing・local composition拒否はcomponent testで検証する。更新gateは `b71f88e` で成功した。commitを固定したWindows結果と残る制約は[実装status](../IMPLEMENTATION_STATUS.ja.md)に記録する。

Windows/WSLの確認済み範囲は、実装statusに記録したcommit固定の実機受入に限る。別hardware・別構成への互換性は未確認として扱う。

## 保存したカスタマイズ手順

状態: **controller setup からの明示実行・再実行は implemented、Windows GHA は bcc1baf で成功**。

`haco setup --script <path>` は利用者が選んだ Bash 手順を、通常の Host 準備後に保存・実行します。
`haco setup` は保存した内容を再実行します。元ファイルを編集しただけでは変わらず、再び `--script` を指定して更新します。
`haco setup --clear-script` は保存した手順を実行せずに解除します。新しいトップレベル command は増やしません。

client は最大1 MiB の通常の UTF-8 file を読み、BOM と Windows CRLF を正規化します。
controller は設定した Hacocoon root の `host-customization/recipe.sh` に private な snapshot を保存します。
既定の絶対パスは `/var/lib/hacocoon/host-customization/recipe.sh` です。
所有権を確認した trusted Host の `/root` で `/bin/bash -se` を起動し、内容を標準入力から渡します。
固定名の systemd transient unit が同時実行を拒否し、controller 終了後も最長14分で process group を停止します。
request の期限が短い場合は unit の期限も短くします。
Physical Host で実行せず、Environment へコピーせず、project の hook を自動探索しません。

trusted `haco-host` 内の例:

```sh
cat > ~/host-setup.sh <<'SH'
install -d -m 0755 "$HOME/.local/bin"
cat > "$HOME/.local/bin/hello-haco" <<'HELLO'
#!/bin/sh
echo "hello from haco-host"
HELLO
chmod 0755 "$HOME/.local/bin/hello-haco"
SH
haco setup --script ~/host-setup.sh
haco setup
haco setup --clear-script
```

手順は再実行できる形で書きます。途中で失敗すると保存内容を保持して失敗を返し、それ以前の利用者 command を rollback しません。
認証情報が含まれる可能性があるため、script の stdout/stderr は controller の診断へ転送しません。
script 自身の出力を調べるときは、元の script を trusted Host 内で直接実行してください。
保存 file の不正な権限や link は拒否するため、controller 所有の設定を確認する必要があります。
Host 再作成後の明示的な controller setup から保存内容を再利用できますが、実際の再作成と setup 外での暗黙の再作成は未検証です。
[ADR 0019](../adr/0019-trusted-host-customization.md) を参照してください。
