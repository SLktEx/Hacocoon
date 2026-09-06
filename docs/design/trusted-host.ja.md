# Trusted `haco-host`

## Windows連携

状態: 手動セットアップをimplemented。WSL基盤側のcheckoutで、`haco setup`後に
`sudo python3 scripts/setup-wsl-host-interop.py`を実行する。所有マークと衝突を
確認し、実在するDrvFsの`/mnt/<letter>`をtrusted `haco-host`の同じパスへ
マウントする。Environmentや共有profileには追加しない。
新しいtrusted shellで`cd /mnt/c`した後、
`/init /mnt/c/Windows/System32/cmd.exe /d /c ver`を実行できる。
`/init`とinteropディレクトリは読み取り専用で、Windows利用者権限が
ドライブアクセスを制約する。書き込みには利用者所有のディレクトリを使う。
WSL再起動でsocket識別が変わる場合は設定を再実行する。
`/init`なしの直接起動、着脱・再接続、全アプリ互換性はdeferred。
[ADR 0009](../adr/0009-trusted-host-windows-interop.md)を参照。

Status: partial.

管理対象repoのWSL経路は **implemented**。登録upstreamのcloneとGitHub認証は
trusted Hostに置く。独立Workspace volume copyはEnvironment利用前にHostから外す。
Git専用broker要求は固定したtrusted Git操作だけを呼び、controller・Policy・Incus権限は
Physical Hostが保持する。[利用手順](../reference/managed-repository-workflow.md)と
[ADR 0008](../adr/0008-managed-repository-workspaces.md)を参照。
Windowsドライブ・exe連携はdeferred。

現在のCLI境界: 製品 `haco` はhelp/version・controller経由の `setup` / `doctor` とWSL login aliasを実装する。以下の保持しているlifecycle commandは[CLI移行](../CLI_MIGRATION.md)中の一時的な `hacoq` の機能であり、新製品commandの実装完了を意味しない。

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
- `hacoq host shell`: instanceをrunningにしてinteractive login shellへ入る
- `user.hacocoon.role=trusted-host` ownership marker
- Hacocoon-managed Incus storage上へのrootfs配置
- provider-local collisionを避けるためEnvironment名`host`を予約
- Physical Host上の`haco-controller` Unix-domain endpoint
- trusted instanceだけに付与する専用`haco-control` proxy
- digest / ownershipを検証した`/usr/local/bin/haco-host` provisioning
- 同じsource / digest / metadata基準を使うsame-release `/usr/local/bin/haco` provisioning
- 未移行`haco` commandがguest-local compositionへsilentに落ちることを防ぐ`environment.HACO_CLIENT_MODE=controller`
- `haco-host doctor`を確認してからdefault interactive entryを有効化するsupported WSL bootstrap

Trusted Host全体はまだpartialです。Git/GitHub、OCI/containerd、cloud credentials、一般的なexternal tooling、Windows mount、WSL interopはまだすべて移行済みではなく、`haco`と`haco-host`の責務分割も完了していません。

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

ただし、同じBtrfs上にあるだけで将来の`haco-host` dataがSeed / Environmentと物理的にCOW shareされるとはみなしません。そのclaimはmeasurement依存です。

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
- Host OCI store / containerdを`haco-host`内で動かす
- reusable credentialを通常Environmentへ置かないcredential broker
- trusted Hostだけにoptional WSL / Windows interopを付与
- 残る適切な`haco` commandをclassifyしてcontroller client pathへ移行
- trusted Host-local operationをlong-termの`haco-host` namespaceへ移しtemporary ambiguityをなくす
- `haco` / `haco-host` CLI responsibility splitを完了
- Coreがrepositoryを永久に`haco-host`へ固定すると仮定しないWorkspace / repository location seam

## Acceptance boundary

Repository testではownership reconciliation、collision refusal、state recovery、exact controller proxy validation、2本のclient binary provisioning / idempotency、client-mode drift refusal、CLI routing、local fallbackのfail-closed、warning、login-mode identificationを確認します。

維持するreal Incus E2E gateはcontroller経由の `haco setup`、endpoint投影、必要な2本のclientのdigest一致、`haco-host doctor` / `haco-host env ...` のcontroller経由操作、restart復旧、fresh setupでguestに旧`hacoq`がないこと、raw Incus socket非露出、通常Environmentのtrusted endpoint / client-mode marker非露出を検査する。保持した旧alias・Base routing・local composition拒否はcomponent testで検証する。更新gateは `b71f88e` で成功した。commitを固定したWindows結果と残る制約は[実装status](../IMPLEMENTATION_STATUS.ja.md)に記録する。

実Windows terminal起動、WSL distribution restart、login-shell transition、Windows integrationはReal Windows + WSL acceptanceが完了するまでhost-verifiedとは扱いません。
