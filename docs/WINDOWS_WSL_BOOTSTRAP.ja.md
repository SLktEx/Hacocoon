# Windows / WSL セットアップ

Hacocoon の local Host baseline は Ubuntu 26.04+ です。Windows では専用の Ubuntu WSL 2 distribution を作り、native Ubuntu ではその Host を直接使います。

Installer は WSL と native Ubuntu を無理に同一化せず、**pre / main / post** に分けます。

## Install phase

現在の状態は **partial**。reset後の製品 `haco` はhelp/versionとWSL loginのみ。保持しているlifecycle commandは一時的な `hacoq` の実装である。[CLI移行](CLI_MIGRATION.md)を参照。一回のBATでの継続とroot側での準備はimplemented、実機完走・network・再起動・現在版installer再実行の受入はpending。common phaseは特権storage実行ファイルを配布せず、Btrfs lifecycleはIncusだけが所有する。installer権限の正本は[ADR 0004](adr/0004-wsl-installer-authority.md)。

```text
Windows / WSL
install-windows.bat
  -> install-windows.ps1 pre
  -> install.sh             Ubuntu 共通 main
  -> install-windows.ps1 post

native Ubuntu
install-ubuntu.sh pre
  -> install.sh             Ubuntu 共通 main
  -> install-ubuntu.sh post
```

`install.sh` に置くのは両方の Ubuntu で共通な処理だけです。Common package、Incus、Hacocoon binary、Physical Host controller、trusted `haco-host` reconcile、controller round-trip acceptance を担当します。

WSL lifecycle は `install.sh` に入れません。Native Ubuntu 固有の policy も入れません。

## Architecture 別 package

Release package は CPU architecture ごとに完全に分けます。片方の package にもう片方の binary は入りません。

```text
hacocoon-windows-amd64.zip
├─ install-windows.bat
├─ install-windows.ps1
├─ install.sh
├─ haco_linux_amd64.tar.gz
├─ checksums.txt
└─ VERSION

hacocoon-windows-arm64.zip
├─ install-windows.bat
├─ install-windows.ps1
├─ install.sh
├─ haco_linux_arm64.tar.gz
├─ checksums.txt
└─ VERSION

hacocoon-ubuntu-amd64.tar.gz
├─ install-ubuntu.sh
├─ install.sh
├─ haco_linux_amd64.tar.gz
├─ checksums.txt
└─ VERSION

hacocoon-ubuntu-arm64.tar.gz
├─ install-ubuntu.sh
├─ install.sh
├─ haco_linux_arm64.tar.gz
├─ checksums.txt
└─ VERSION
```

そのため通常 install では **`install.sh` も Hacocoon binary archive も Release から取り直しません**。最初に取得した installer package に入っている archive をそのまま install します。

Ubuntu package の導入や provenance 検証のために network access が必要になる場合はあります。これは Hacocoon release payload を二重 download することとは別です。

Raw の `haco_linux_amd64.tar.gz` / `haco_linux_arm64.tar.gz` は advanced / standalone 用として Release に残しますが、通常の Host install 入口にはしません。

## Windows pre

PowerShell が Windows / WSL 固有の事前準備を担当します。

1. current WSL を要求する
2. `Ubuntu-26.04` から Hacocoon 専用 distribution だけを作成 / 再利用する
3. global WSL default は変えず、その distribution だけ WSL 2 を保証する
4. 通常の non-root Ubuntu user を要求する
5. WSL guest が Ubuntu 26.04+ であることを確認する
6. `/etc/wsl.conf` の他設定を残しつつ以下を保証する

   ```ini
   [boot]
   systemd=true
   ```

7. 必要なら `wsl --terminate Hacocoon` でその distribution だけ restart する
8. systemd が PID 1 であることを確認する
9. WSL architecture と同じ `haco_linux_<arch>.tar.gz` が installer package 内にあることを確認する

freshまたは中断したsetupでは、同じBAT内で通常のUbuntu初回起動を開きます。アカウント作成と表示されたmetrics同意などのOS対話を済ませ、最初のLinux shellで `exit` を入力するとBATが続行します。現在版の導入が完了しHacocoon login shellを持つ場合だけ、この初回sessionを省略します。既存ユーザーとdataは保持します。失敗・中断時もdistributionを残し、現在版BATの再実行で続行します。

## Ubuntu 共通 main

Windows / WSL と native Ubuntu の両方が、package 内の同じ `install.sh` を呼びます。

WindowsはWSL rootで実行し、通常login名を `HACO_INSTALL_USER` で渡します。common phaseはHost準備前に実在accountとnon-root UID/GIDを検証し、そのexact IDをIncus subordinate-IDへ使用します。特権実行のrootをWorkspace ownerへ代入してはいけません。nativeの通常user/sudo実行は通常caller identityを保持します。

Common main は次を担当します。

- Ubuntu 26.04+ と既に active な systemd を要求
- common Host dependency の install
- skip 指定がなければ Incus の install / init
- 同梱 architecture-specific archive の checksum 検証
- provenance 有効時の trusted GitHub/Sigstore provenance と signed release binding 検証
- archive が期待する regular Hacocoon binary だけを含むことの検証
- Hacocoon binary の install
- `haco-controller.service` の install / restart
- 通常userを特権local `hacocoon` controller access groupへ追加
- `/run/hacocoon/control.sock` が `root:hacocoon` mode `0660` Unix socketであることの確認
- CLI移行中の内部bootstrapとして保持している `hacoq host ensure` の実行
- 実際の `haco-host` 内から `/usr/local/bin/haco-host doctor` を実行する round-trip acceptance

`install.sh` は `/etc/wsl.conf` を編集せず、WSL を terminate せず、user login shell も変更しません。

## Windows post

Common main 成功後、PowerShell が WSL 固有 integration を行います。

System-owned `haco` を検証して `/usr/local/libexec/hacocoon-login` aliasを作り、通常non-root WSL userのlogin shellだけを変更します。aliasはcontrollerへ直接接続します。installerはbootstrap/loginのsudo policyを書き換えず、明示指定がなければ `incus-admin` も付与しません。

その後:

```powershell
wsl -d Hacocoon
```

で trusted `haco-host` management environment に入ります。Explicit / non-interactive WSL command は Physical Host 側に残ります。

Root user の shell は変更しません。Recovery は:

```powershell
wsl -d Hacocoon -u root
```

です。Raw Incus socket を `haco-host` へ expose することもありません。

## Native Ubuntu pre / post

Native package の入口は:

```bash
./install-ubuntu.sh
```

です。

Pre では WSL を拒否し、Ubuntu 26.04+、systemd PID 1、必要な sudo を確認してから同梱 `install.sh` を実行します。

Post では native Ubuntu user の login shell を変更しません。Trusted Host へは明示的に:

```bash
hacoq host shell
```

で入ります。

## E2E acceptance boundary

Installer E2E は `install.sh` 単体成功ではなく、実際の user-visible entry point から判定します。

Windows gateはcandidate ZIPを展開し、通常terminalで未変更のBATを入力し、Ubuntuの実際の初回対話へ応答します。2回目のBATより前に完了が必要です。続いて `wsl -d Hacocoon` で入り、実装済み製品help/versionとtrusted-host fileの作成を確認し、WSLを停止して再入場します。その後で同じBATを再実行し、fileとsudo policyの保持を確認します。root検査はread-onlyです。製品override、account/sudoers fixture、テスト専用bridge、mount修復で不足を補ってはいけません。

このfile保持はtrusted-hostの受入であり、Environment/Workspaceの作業保持ではありません。新CLIのlifecycle/SSH、installer生成networkのDNS・経路・HTTPS、Environmentの許可proxy通信と直接通信拒否は別の必須受入です。LinuxのIncus/network基盤CIは継続し、削除したWindows旧fixture導線を新製品導線の証拠にはしません。

Native Ubuntu gate は candidate `hacocoon-ubuntu-amd64.tar.gz` を作って展開し、package 内の `install-ubuntu.sh` を実行します。Controller と trusted `haco-host` round trip が成功し、native login shell が変更されていないことまで確認します。

PR candidate packageはpublic releaseではありません。同梱payload経路は製品環境変数のoverrideなしでchecksumを検証し、release配布workflowは公開するexact packageを別途署名・attestします。単独download payloadは引き続き既定でprovenance検証を要求します。
