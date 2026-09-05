# Windows / WSL セットアップ

Status: **partial**。固定管理accountのbootstrapはimplemented、実Windowsでのinstall/network/restart受入は[実装status](IMPLEMENTATION_STATUS.ja.md)で別管理する。製品 `haco` は現在help/versionとcontroller経由のWSL login aliasを持つ。保持しているlifecycle commandは[CLI移行](CLI_MIGRATION.md)中の一時的な `hacoq` の機能である。

Hacocoon の local Host baseline は Ubuntu 26.04+ です。Windows では専用の Ubuntu WSL 2 distribution を作り、native Ubuntu ではその Host を直接使います。

Installer は WSL と native Ubuntu を無理に同一化せず、**pre / main / post** に分けます。

## Install phase

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
4. fresh installのdefault pathではpassword loginをlockした固定non-root user `hacocoon` を作成し、WSL default userと管理済み初回設定を構成する
5. `-InteractiveUserSetup` 指定時だけ Ubuntu の対話式 user setup を使う
6. 旧 install の upgrade では既に設定済みの non-root default user を維持する
7. WSL guest が Ubuntu 26.04+ であることを確認する
8. `/etc/wsl.conf` の他設定を残しつつ managed/default user と以下を保証する

   ```ini
   [boot]
   systemd=true
   ```

9. login user / systemd 設定変更に必要な場合だけ `wsl --terminate Hacocoon` でその distribution を restart する
10. systemd が PID 1 であることを確認する
11. WSL architecture と同じ `haco_linux_<arch>.tar.gz` が installer package 内にあることを確認する

Fresh install の default path は **1 回で完了**します。

```powershell
install-windows.bat
```

を 1 回実行すると、`Hacocoon` WSL distribution 作成、固定 `hacocoon` user 作成、systemd 設定、package 内 `install.sh` 実行、WSL post integration まで同じ installer process で続けて実行します。途中で自分で WSL を起動したり、BAT をもう一度叩いたりする必要はありません。

Ubuntu の通常の対話式 account creation を明示的に使いたい場合だけ次を実行します。

```powershell
install-windows.bat -InteractiveUserSetup
```

この場合も installer 自身が WSL の user-setup session を起動します。User setup を完了してその shell から exit すると、同じ installer process がそのまま `install.sh` と post phase を再開します。BAT の 2 回目実行は不要です。

既定の管理accountはpassword入力不要で、retry時に既存accountのpasswordをresetしません。この経路だけ、既知のUbuntu account/metrics OOBE commandを空にし、検証済みdefault UIDを設定します。他のdistribution設定を保持してatomicに置換し、未知のOOBE設定ではfail closedします。対話optionでは通常のUbuntu setupを維持し、利用者をmetrics送信へopt-inしません。[ADR 0004](adr/0004-wsl-installer-authority.md)を参照してください。

## WSL image cache 検証経路

`-UseCachedWslImage` は、Windows / WSL の install を何度も検証するときのための validation-oriented option です。明示的に指定しない限り、通常 installer の挙動は変わりません。

有効時は、installer package と同じ場所にある `ubuntu.wsl` を Ubuntu 26.04 の local image cache として使います。`ubuntu.wsl` が無ければ、Microsoft の WSL `DistributionInfo.json` から現在の Windows architecture に対応する `Ubuntu-26.04` の URL と SHA256 を取得し、一時ファイルへ download、公開 SHA256 を検証してから `ubuntu.wsl` に昇格します。

SHA-256検証は.NETを直接使い、BATが起動するWindows PowerShell 5.1での `Get-FileHash` moduleの有無に依存しません。hash検証失敗時はdistro作成前に停止し、途中のdownloadをcacheへ昇格しません。

WSL作成失敗時は子プロセスの終了codeを表示し、common Ubuntu準備前に停止します。WSLが古いことが原因とは決めつけません。

Download先は表示し、関数内だけPowerShellのchunkごとの進捗描画を抑えます。PS5.1での大容量downloadの遅延を避け、呼び出し元の進捗設定は変更しません。

専用 distribution の作成は named install 経路のままです。

```powershell
wsl --install --from-file .\ubuntu.wsl --name Hacocoon --no-launch
```

`-UseCachedWslImage` は現在 `Ubuntu-26.04` だけを対象とし、`-WebDownload` とは同時指定できません。`ubuntu.wsl` は Release installer package には同梱しません。Local / CI の install 検証を高速化するための artifact です。

GitHub Actions では、cache の trust boundary を untrusted PR と分離します。Trusted な `main` 上の `windows-wsl-image-cache` workflow だけが `actions/cache` で cache を生成し、cache miss 時は同じ `-UseCachedWslImage` 経路を実行するため、Microsoft metadata と SHA256 検証を通った `ubuntu.wsl` だけが保存されます。Pull request の Windows installer E2E は `actions/cache/restore` のみを使い、trusted cache があれば candidate package に `ubuntu.wsl` を copy して利用します。PR から cache state を書き込みません。Trusted cache が無い場合は、その run だけ installer 自身が通常どおり検証付き download を行います。

Restart / reinstall Windows E2E は最初と 2 回目の packaged BAT の両方に `-UseCachedWslImage` を渡すため、cache 経路そのものを acceptance しつつ、`wsl --terminate Hacocoon` 後の persistence と reinstall idempotency も同時に検証します。

PowerShellはWSL rootでcommon準備を実行し、通常login名を `HACO_INSTALL_USER` で渡します。common phaseは特権準備前に実際のnon-root UID/GIDを解決・検証し、exact IDをIncus subordinate-IDに使い、同じuserをcontroller access groupへ追加します。bootstrap/loginのsudo policyは書かず、`incus-admin` は明示optionのままです。

## Ubuntu 共通 main

Windows / WSL と native Ubuntu の両方が、package 内の同じ `install.sh` を呼びます。

Common main は次を担当します。

- Ubuntu 26.04+ と既に active な systemd を要求
- common Host dependency の install
- skip 指定がなければ Incus の install / init
- 同梱 architecture-specific archive の checksum 検証
- provenance 有効時の trusted GitHub/Sigstore provenance と signed release binding 検証
- archive が期待する regular Hacocoon binary だけを含むことの検証
- Hacocoon binaryのinstall（storage helperは削除済み）
- `haco-controller.service` の install / restart
- `/run/hacocoon/control.sock` が `root:hacocoon` mode `0660` Unix socketであることの確認
- CLI移行中の内部bootstrapとして保持している `hacoq host ensure`
- 実際の `haco-host` 内から `/usr/local/bin/haco-host doctor` を実行する round-trip acceptance

`install.sh` は `/etc/wsl.conf` を編集せず、WSL を terminate せず、user login shell も変更しません。

## Windows post

Common main 成功後、PowerShell が WSL 固有 integration を行います。

System-owned `haco` を検証して `/usr/local/libexec/hacocoon-login` aliasを作り、通常non-root WSL userのlogin shellだけを変更します。aliasはPhysical Host controllerへ直接接続し、sudoや `hacoq` subprocessを使いません。

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
haco host shell
```

で入ります。

## E2E acceptance boundary

Installer E2E は `install.sh` 単体成功ではなく、実際の user-visible entry point から判定します。

Windows gateはcandidate ZIPとtrustedなrestore-only cache経路を使い、packaged BATを `-UseCachedWslImage` 付きで実行します。通常の `wsl -d Hacocoon` から実装済み製品help/versionとtrusted-host file作成を確認し、installer再実行より前にWSLを停止して再入場します。その後の2回目BATでもfileとsudo policyを保持します。root検査はread-onlyで、製品override、account/sudoers fixture、テストbridge、mount修復で不足を補いません。対話account optionは管理accountの既定経路と別です。

Native Ubuntu gate は candidate `hacocoon-ubuntu-amd64.tar.gz` を作って展開し、package 内の `install-ubuntu.sh` を実行します。Controller と trusted `haco-host` round trip が成功し、native login shell が変更されていないことまで確認します。

PR candidateはpublic releaseではありません。同梱payloadのchecksumは製品環境変数のoverrideなしで検証し、release workflowは公開するexact packageを署名・attestします。単独downloadは引き続き既定でprovenance検証を要求します。
