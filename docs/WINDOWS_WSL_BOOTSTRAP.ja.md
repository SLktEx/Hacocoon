# Windows / WSL セットアップ

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
4. fresh install の default path では固定の managed non-root user `hacocoon` を作成し、その account の password login を lock して WSL default user にする
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

## WSL image cache 検証経路

`-UseCachedWslImage` は、Windows / WSL の install を何度も検証するときのための validation-oriented option です。明示的に指定しない限り、通常 installer の挙動は変わりません。

有効時は、installer package と同じ場所にある `ubuntu.wsl` を Ubuntu 26.04 の local image cache として使います。`ubuntu.wsl` が無ければ、Microsoft の WSL `DistributionInfo.json` から現在の Windows architecture に対応する `Ubuntu-26.04` の URL と SHA256 を取得し、一時ファイルへ download、公開 SHA256 を検証してから `ubuntu.wsl` に昇格します。

専用 distribution の作成は named install 経路のままです。

```powershell
wsl --install --from-file .\ubuntu.wsl --name Hacocoon --no-launch
```

`-UseCachedWslImage` は現在 `Ubuntu-26.04` だけを対象とし、`-WebDownload` とは同時指定できません。`ubuntu.wsl` は Release installer package には同梱しません。Local / CI の install 検証を高速化するための artifact です。

GitHub Actions では、cache の trust boundary を untrusted PR と分離します。Trusted な `main` 上の `windows-wsl-image-cache` workflow だけが `actions/cache` で cache を生成し、cache miss 時は同じ `-UseCachedWslImage` 経路を実行するため、Microsoft metadata と SHA256 検証を通った `ubuntu.wsl` だけが保存されます。Pull request の Windows installer E2E は `actions/cache/restore` のみを使い、trusted cache があれば candidate package に `ubuntu.wsl` を copy して利用します。PR から cache state を書き込みません。Trusted cache が無い場合は、その run だけ installer 自身が通常どおり検証付き download を行います。

Restart / reinstall Windows E2E は最初と 2 回目の packaged BAT の両方に `-UseCachedWslImage` を渡すため、cache 経路そのものを acceptance しつつ、`wsl --terminate Hacocoon` 後の persistence と reinstall idempotency も同時に検証します。

Common installer 実行中だけ、選択された通常 WSL user に temporary passwordless sudo rule を付けます。これは ordinary workspace owner のまま `install.sh` を走らせるための bootstrap 用で、trusted installer invocation の間だけ存在します。`finally` で必ず削除し、通常の install 完了後には後述の narrow な `haco host ensure` / `haco host shell` rule だけを残します。

## Ubuntu 共通 main

Windows / WSL と native Ubuntu の両方が、package 内の同じ `install.sh` を呼びます。

Common main は次を担当します。

- Ubuntu 26.04+ と既に active な systemd を要求
- common Host dependency の install
- skip 指定がなければ Incus の install / init
- 同梱 architecture-specific archive の checksum 検証
- provenance 有効時の trusted GitHub/Sigstore provenance と signed release binding 検証
- archive が期待する regular Hacocoon binary だけを含むことの検証
- binary と root-owned storage helper の install
- `haco-controller.service` の install / restart
- `/run/hacocoon/control.sock` が root-owned mode `0600` Unix socket であることの確認
- `haco host ensure`
- 実際の `haco-host` 内から `/usr/local/bin/haco-host doctor` を実行する round-trip acceptance

`install.sh` は `/etc/wsl.conf` を編集せず、WSL を terminate せず、user login shell も変更しません。

## Windows post

Common main 成功後、PowerShell が WSL 固有 integration を行います。

System-owned `haco` を検証して `/usr/local/libexec/hacocoon-login` を作り、passwordless sudo は exact な `haco host ensure` / `haco host shell` だけに許可し、通常 non-root WSL user の login shell だけを変更します。

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

Windows gate は candidate `hacocoon-windows-amd64.zip` を作って展開し、trusted `ubuntu.wsl` cache があれば restore / stage して、shipped `-UseCachedWslImage` option 経由で packaged installer を実行します。最初の install が managed `hacocoon` user で完了した後、WSL distribution を明示的に terminate し、repair install を一度も挟まず既存 Environment が restart 後も使えることを確認し、最後に reinstall / idempotency まで検証します。Acceptance boundary は WSL 2、systemd、Incus、controller service/socket、`haco-host doctor`、WSL login integration までです。Opt-in の interactive user-setup path も別途維持しますが、default install contract にはしません。

Native Ubuntu gate は candidate `hacocoon-ubuntu-amd64.tar.gz` を作って展開し、package 内の `install-ubuntu.sh` を実行します。Controller と trusted `haco-host` round trip が成功し、native login shell が変更されていないことまで確認します。

PR candidate package はまだ public Release ではないため release attestation を持ちません。Candidate E2E で provenance を無効化できるのはこの synthetic package の検証だけです。Release workflow は別途、実際に publish する architecture-specific payload そのものへ署名 / attestation を行います。