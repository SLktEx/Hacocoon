# Windows / WSL セットアップ

Windows では、**普段使いのWSLを再利用せず、Hacocoon専用のWSL 2 distributionを1個作り、systemdをPID 1として動かす**のをsupported pathにします。

この専用distribution自体が **Physical Host** です。Incus、managed Btrfs primitive、Hacocoon controllerはここが所有します。Bootstrap成功後、通常のinteractive entryは永続的なtrusted `haco-host`へ直接入ります。

```text
Windows
  |
  +-- 普段使いのWSL distribution             <- 触らない
  |
  `-- WSL 2: Hacocoon                        <- Physical Host
       |- systemd (PID 1)
       |- Incus
       |- loop / Btrfs primitives
       |- haco-controller
       |    `- /run/hacocoon/control.sock
       |
       `- Incus: haco-host                   <- TRUSTED / 普段入るHost
            |- /usr/local/bin/haco-host
            `- /var/lib/hacocoon-control.sock
                 `- 専用Incus proxy -> controller

       Incus: managed Environments            <- UNTRUSTED
```

詳細は [`design/trusted-host.ja.md`](design/trusted-host.ja.md) と [`design/controller-client-transport.ja.md`](design/controller-client-transport.ja.md) を参照してください。

## 普通に使うinstaller

GitHub Releaseでは **`hacocoon-windows-installer.zip` を通常のWindows installer** として配布します。Repository cloneは不要です。

ZIPを展開すると:

```text
hacocoon-windows-installer/
├─ install-windows.bat
└─ install-windows.ps1
```

が入っています。通常は `install-windows.bat` を右クリックして **「管理者として実行」** するか、管理者Command Promptから:

```bat
install-windows.bat
```

を実行します。

BAT launcherは同じdirectoryの`install-windows.ps1`だけを:

```text
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File ...
```

で起動します。`-ExecutionPolicy Bypass`はそのPowerShell processだけに指定し、`Set-ExecutionPolicy`でmachine/userの恒久設定を変更しません。Installer引数とexit codeもそのまま転送します。

これはinternet由来の`.ps1`を直接起動するとExecution Policy / Mark-of-the-Webで引っかかりやすい問題を通常導線から外すためのlauncherです。組織管理の`MachinePolicy` / `UserPolicy`、Windowsのreputation protection、その他endpoint policyを無効化する仕組みではありません。それらで禁止されている環境ではfail closedします。

`install-windows.ps1` standalone assetも高度な利用・互換用としてReleaseに残します。PowerShellから直接実行する場合は、その端末のExecution Policyに従ってください。

標準distribution名は`Hacocoon`、標準baseは`Ubuntu-26.04`です。

Fresh distributionでは最初に通常のLinux user作成が必要な場合があります。Hacocoon bootstrap完了前の`wsl -d Hacocoon`は、そのfirst-run setupのため通常のbase distributionへ入ります。User setup後に`install-windows.bat`をもう一度管理者として実行します。

Bootstrap成功後は:

```powershell
wsl -d Hacocoon
```

でtrusted `haco-host`へ入ります。

## WSL 2 と systemd

InstallerはHacocoon専用distributionだけをWSL 2であると検証し、必要なら`wsl --set-version Hacocoon 2`を使います。Global WSL defaultや無関係なdistributionは変更しません。

Linux bootstrapは既存`/etc/wsl.conf`の他設定を残しつつ:

```ini
[boot]
systemd=true
```

を保証します。

まだsystemdがPID 1でなければ、Windows installerは`wsl --terminate Hacocoon`でHacocoon専用distributionだけをrestartします。`wsl --shutdown`は使いません。

## Incus と managed storage

`-SkipIncus`でない限り、bootstrapはIncusをinstall/startし、必要ならminimal initします。

Default local storageはHacocoon-managed sparse-raw Btrfsで、`compress=zstd:3`を使います。`compress-force`やautomatic defrag/recompressionはreflink/COW sharingを壊し得るので行いません。

## Physical Host controller service

Release install後、bootstrapは`haco-controller`が`/usr/local/bin`または`/usr/bin`のroot-ownedかつgroup/world writableでないsystem binaryであることを検証します。

そのうえでPhysical Hostに:

```text
haco-controller.service
```

をinstallし、current releaseでrestartします。

Controllerはlocalhost TCPを開かず、次のUnix socketだけでlistenします。

```text
/run/hacocoon/control.sock
```

次へ進む前に、このpathが`root:root` mode `0600`のUnix socketであることをbootstrapが確認します。

Upgrade時にserviceをrestartするのは、install直後のreleaseとdaemon processのversionを一致させるためです。

## Trusted `haco-host` reconcile

Controller active後、Physical Host authorityで:

```text
haco host ensure
```

を実行します。

このoperationは次をreconcileします。

- `haco-host`というIncus instanceをちょうど1個
- `user.hacocoon.role=trusted-host` ownership marker
- Hacocoon-managed root storage
- owned stopped instanceのautomatic restart
- `environment.HACO_CONTROL_SOCKET=/var/lib/hacocoon-control.sock`
- 専用`haco-control` proxy
- trusted instance内のclient-only `/usr/local/bin/haco-host`

Proxyは意図的に狭くします。

```text
type=proxy
bind=instance
listen=unix:/var/lib/hacocoon-control.sock
connect=unix:/run/hacocoon/control.sock
mode=0600
uid=0
gid=0
```

既存instance、endpoint variable、proxy shapeが期待と違う場合、silent repurposeせずfail closedします。

Client binaryはSHA-256とfinal root ownership/modeを確認してprovisionします。

## Bootstrapでround tripまで証明する

`haco host ensure`後、bootstrapは実際のtrusted instance内で:

```text
/usr/local/bin/haco-host doctor
```

を実行します。

これにより実path:

```text
haco-host CLI
  -> /var/lib/hacocoon-control.sock
  -> Incus haco-control proxy
  -> /run/hacocoon/control.sock
  -> Physical Host haco-controller
```

が通ることを確認します。

失敗した場合はnormal userのlogin shellを変更する前にbootstrapを止め、Physical Host recovery pathを表示します。

Raw Incus daemon socketを`haco-host`へmount/proxyすることはありません。

## `wsl -d Hacocoon` が `haco-host` に入る仕組み

Controller / trusted Host acceptance成功後、bootstrapはroot-owned `/usr/local/libexec/hacocoon-login`を作り、通常non-root WSL userのlogin shellにします。

Interactive no-command entryでは:

```text
sudo -n <system-owned-haco> host shell
```

へdelegateします。

`haco host shell`はtrusted Hostとclient binaryを再reconcileし、privileged-management warningを表示してから`haco-host`内の`/bin/bash -l`へ入ります。

Explicit / non-interactive WSL commandはPhysical Host側のままで、interactive warningを混ぜません。

## Automatic entry の privilege boundary

Automatic entryのために通常WSL userへ`incus-admin`を付与しません。

Passwordless sudoを許可するのはsystem-owned binaryに対するexactな:

```text
haco host ensure
haco host shell
```

だけです。

Raw Incus socketと`/var/lib/incus`はPhysical Host authorityに残します。

Physical Host userへroot-equivalentなIncus authorityを意図的に与える場合だけ:

```bat
install-windows.bat -GrantIncusAdmin
```

を使います。PS1 standaloneを直接使う場合は`./install-windows.ps1 -GrantIncusAdmin`でも同じです。

## Physical Host recovery

通常利用:

```powershell
wsl -d Hacocoon
```

は`haco-host`へ入ります。

Root accountのshellは変更しないため、Physical Hostへ直接戻るrecovery pathは:

```powershell
wsl -d Hacocoon -u root
```

です。

Explicit commandもPhysical Hostへ向けられます。例えば:

```powershell
wsl -d Hacocoon -- haco status
```

Physical Host root authorityが必要なoperationはauthorized sudo pathまたはroot recovery shellを使います。

## `-SkipIncus`

Incusを別手段で管理する場合は:

```bat
install-windows.bat -SkipIncus
```

を使えます。PS1 standaloneの`./install-windows.ps1 -SkipIncus`も引き続き利用できます。

このmodeではtrusted backend readyをHacocoonが保証できないため、Physical Host loginを変更せず、controller-connected automatic `haco-host` entryも設定しません。

## Workspace location

Default interactive entryとWorkspace ownershipは別architecture seamです。

Repository/workspace ownershipをlogical Hostへ完全移行することは、このbootstrapだけで実現済みとは扱いません。それまではPhysical Host pathをexplicit commandで扱えます。VS Code / external orchestratorも`haco-host`をmandatory SSH jump hostとせず、Hacocoonのclient/control surfaceからtarget Environmentへ接続する方針です。

## Installerの処理順

Supported pathは次の順で進みます。

1. named WSL distributionとreleaseを検証
2. Hacocoon専用distributionだけを作成/再利用
3. そのdistributionだけWSL 2を保証
4. systemdを有効化し、必要ならそのdistributionだけrestart
5. skip指定がなければIncusをinstall/start
6. Btrfs toolとHacocoon release binaryをinstall
7. Physical Hostの`haco-controller.service`をinstall/restart
8. root-only controller Unix socketを検証
9. trusted `haco-host`、narrow proxy、client binaryをreconcile
10. `haco-host doctor`からPhysical Host controllerへの疎通を証明
11. narrow automatic-entry sudo ruleをinstall
12. 通常non-root userのlogin shellを`hacocoon-login`へ変更

Global WSL default、`.wslconfig`、無関係なdistribution、root userのlogin shellは変更しません。

## Checkout版

Repository checkoutでは引き続き:

```powershell
.\scripts\bootstrap-windows.ps1
```

を使えます。Checkout scriptを使いますが、同じWSL 2 / systemd / Incus / controller / trusted-host entry contractに従います。

## Release integrity

`hacocoon-windows-installer.zip`はRelease payloadの一部としてSHA-256 checksumとGitHub artifact attestationの対象にします。ZIPの中身はCIで`install-windows.bat`と`install-windows.ps1`の2ファイルだけであることと、sourceとbyte-for-byte一致することを検証します。

BAT launcher自体はbootstrapのpolicy boundaryを弱めません。実際のHacocoon release download / checksum / signed provenance verificationは引き続き`install-windows.ps1`がfail closedで行います。

## Acceptance boundary

Repository CIとreal Incus E2Eではcontroller protocol、実proxy device、client provisioning、`haco-host doctor` round trip、restart recovery、raw Incus socket非露出、通常Environmentにtrusted endpointがないことに加えて、BAT launcher / ZIP packaging contractを確認できます。

一方、browser downloadしたZIPをWindowsで展開した後のBAT起動、first-run Linux user setup、WSL restart behavior、`wsl -d Hacocoon`からのlogin-shell transition、WindowsからのPhysical Host recovery、Windows editor/orchestrator integrationはreal Windows + WSL acceptanceが必要です。
