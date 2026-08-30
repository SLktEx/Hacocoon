# Trusted `haco-host`

Status: partial.

## 概要

`haco-host` はHacocoonが管理する永続的なtrusted logical Hostです。Local Incus backendでは`haco-host`という名前のIncus system instanceとして実装し、通常のuntrusted Environmentとは明確に分離します。

Physical Host controller、Incus daemon、loop device、storage mountを実際に動かすLinux / WSL distributionを **Physical Host** と呼びます。Physical Hostはplatform primitiveのauthorityを持ち続けます。`haco-host`はuserが普段入るhost-likeな場所であり、今後developer toolingやexternal-service operationの標準実行場所にもしていきます。

```text
Physical Host / WSL
  |- haco-controller
  |- Incus daemon
  |- loop / Btrfs platform primitives
  `- haco-host                  TRUSTED
       |- client-only haco-host CLI
       `- /run/hacocoon/control.sock
            |
            `- narrow Incus Unix proxy -> Physical Host controller

Managed Environments            UNTRUSTED
```

`haco-host`はtrusted computing baseの一部です。Environmentではなく、agent sandboxとして扱ってはいけません。

## 現在実装済みの slice

現在のLocal Incus implementationには次が入っています。

- `haco host ensure`: 永続的な`haco-host`とそのclient control pathをreconcileする。
- `haco host shell`: control pathがreadyであることを確認したうえでinteractive login shellへ入る。
- `user.hacocoon.role=trusted-host`というIncus ownership marker。
- Hacocoonが選択したmanaged Incus storage pool上へのrootfs配置。
- infrastructure instanceと衝突しないようEnvironment名`host`を予約。
- trusted instanceへのclient-only `/usr/local/bin/haco-host` provisioning。
- instance内の`/run/hacocoon/control.sock`をPhysical Host Hacocoon controller socketへ接続するtrusted-host専用Incus `proxy` device。
- 既存proxy configurationのexact validation。期待値と違う場合はfail closed。
- `haco host ensure`成功前のguest-side `haco-host doctor` verification。
- Physical Hostで`haco-controller`をsystemd serviceとして起動・readiness確認し、その後`haco-host`をreconcileしてからnormal interactive WSL entryを変更するbootstrap。
- real Ubuntu 26.04 + Incus + managed Btrfs CIでのtrusted-host control path acceptance。

#275全体はまだpartialです。Git/GitHub、OCI/containerd、cloud credentials、一般external tooling、Windows mount、WSL interopはまだ`haco-host`へ移していません。またPhysical Host authorityを必要とする既存`haco` commandは、current `haco` binaryを安全にguestへprovisionできるようcontroller-client interfaceへ移行する必要があります。

## Trust と authority

Incusとplatform control authorityはPhysical Hostに残します。

`haco-host`へIncus daemon socket、`/var/lib/incus`、Physical Host上のHacocoon state directory、広いPhysical Host filesystem mountを渡しません。渡すのはcontroller APIが公開するoperationを呼ぶためのHacocoon-ownedの狭いclient endpointだけです。

```text
haco-host client
   |
   | /run/hacocoon/control.sock
   v
Incus unix proxy (bind=instance)
   |
   v
Physical Host haco-controller
   |
   | Incus API/socket remains here
   v
incusd
```

通常Environmentには`haco-control` proxy deviceもPhysical Host controller socket pathも渡しません。

trusted-host reconcilerはprovisioning前にexact ownership markerを確認します。既存`haco-control` proxyを再利用するのは`listen`、`connect`、`bind`、`uid`、`gid`、`mode`がHacocoon-managed configurationと一致する場合だけです。未知の設定を暗黙にtakeoverせずincompatible stateとして失敗します。

## Ownership と name collision

Incus instance名`haco-host`はinfrastructure-ownedです。

作成時に`incus init`と同時にHacocoon ownership markerを付与します。既存instanceを再利用する場合はそのmarkerが完全一致することを要求します。無関係なinstanceや古いinstanceが`haco-host`を占有している場合、Hacocoonはtakeover、start、delete、client-channel provisioningをせずfail closedします。

通常Environment名`host`もprovider-localでは同じ名前へ衝突するため、Incus adapterはIncusに触る前に拒否します。

複数`ensure`が同時に走ってcreate raceになった場合、負けた側はexact ownership markerを確認できたときだけwinnerのinstanceを再利用できます。未知のIncus stateは推測せずincompatible stateとして失敗します。

## Storage

`haco-host`は通常のHacocoon Incus storage integrationが選択したroot storage poolを使います。Default local backendではrootfsがHacocoonのsparse-raw Btrfs-backed Incus poolへ入り、将来のHost stateをunmanagedなPhysical Host filesystem locationへ依存させずに済みます。

ただし同じBtrfs上にあるだけで、将来の`haco-host` dataがSeed / Environmentと自動的に物理COW shareされるとはみなしません。そのようなclaimは別途measurementが必要です。

## Client provisioning

`haco host ensure`はcompatibleなclient-only `haco-host` binaryを解決します。test/dev用overrideでは別のabsolute fileを指定できますが、missing、non-regular、group/world-writableなcandidateは拒否します。

binaryはtrusted instanceへ次のpathでinstallします。

```text
/usr/local/bin/haco-host
```

現在のsliceでは通常の`haco` binaryをguestへinstallしません。`haco`にはまだdirect local-composition pathが残るため、Physical Host authority commandをcontroller-client interfaceへ移行する前にguestへ置くとguest-local stateやguest-local Incusへ誤って向く可能性があります。

trusted `haco-host` CLIは現在、Environment create/list/status/exec/shell/deleteと`doctor`をPhysical Host controller経由で提供します。

## WSL の default entry と controller service

supported WSL bootstrapはrelease install後、Physical Host上で`haco-controller`をsystemd serviceとして設定します。serviceはmanaged Hacocoon rootを使い、`/run/hacocoon`配下のlocal controller socketを管理します。

bootstrapはsocketの存在だけでなく実際の`haco-host doctor` requestが通ることを確認してから進みます。その後`haco host ensure`を実行し、trusted instanceへclient binaryとproxyをprovisionし、instance内からのdoctorも成功することを確認します。

これらが成功した後だけ、専用WSL distributionの通常non-root userのlogin shellを`hacocoon-login` entryへ変更します。

通常UXは次になります。

```text
wsl -d Hacocoon
  -> hacocoon-login
  -> sudo -n <system-owned-haco> host shell
  -> verified haco-host
```

Installerがpasswordless sudoを許可するのはexactな`haco host ensure`と`haco host shell`だけです。`incus-admin`はdefaultでは付与しません。

明示的なWSL commandはPhysical Host commandのままで、root accountのlogin shellも変更しません。緊急時は例えば次でPhysical Hostへ直接入れます。

```powershell
wsl -d Hacocoon -u root
```

`-SkipIncus`では必要なbackend/controller pathを保証できないため、自動`haco-host` entryは設定しません。

## Interactive warning

`haco host shell`は`haco-host`へ入る直前に短いprivileged-environment warningを表示します。Japanese localeでは日本語、その他では英語です。non-interactive WSL commandのoutputには混ぜません。

## 今後の follow-up

次はこのsliceの範囲外です。

- Git/GitHubやselected external-service toolingの標準実行場所を`haco-host`にする。
- Host OCI store / containerdを`haco-host`内で動かす。
- reusable credentialを通常Environmentへ渡さないexplicit credential injection / broker。
- trusted Hostだけにoptional WSL / Windows interopを与える。
- Physical Host authorityを必要とする`haco` operationをcontroller-client interfaceへ移行し、適切な`haco` client UXを`haco-host`内へprovisionする。
- `haco`と`haco-host`のCLI responsibility splitを完了する。
- repositoryを永久に`haco-host`固定とCoreに仮定させず、長期的なWorkspace / repository location seamを実装する。
- PTY resize framingとgeneric Environment port forwardingをcontroller streamへ追加する。

## Acceptance boundary

Repository testではownership reconciliation、name collision refusal、stopped/running state、create race、client binary validation、proxy create/reuse/mismatch refusal、CLI routing、locale warning、login-mode identificationを確認します。

GitHub-hosted Ubuntu 26.04上のreal Incus + managed Btrfs acceptanceでは、trusted instanceへのclient-only binary provisioning、専用Unix proxy経由の`doctor`、Physical Host controller経由のEnvironment lifecycle / exec operation、通常Environmentへcontrol channelが付与されないことまで確認します。

ただしcompleteなWindows user journeyの証明ではありません。実Windows terminal -> WSL 2 -> systemd -> default `haco-host` loginの挙動は引き続きhost-dependent acceptanceです。
