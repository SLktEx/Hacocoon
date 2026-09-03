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
4. fresh / default install では専用の non-root Ubuntu user `hacocoon` を自動作成する
5. `-InteractiveUserSetup` 指定時だけ Ubuntu 標準の first-launch user setup を使えるようにする
6. WSL guest が Ubuntu 26.04+ であることを確認する
7. `/etc/wsl.conf` の他設定を残しつつ、選択した login user と systemd を保証する

   ```ini
   [user]
   default=hacocoon

   [boot]
   systemd=true
   ```

8. 必要なら `wsl --terminate Hacocoon` でその distribution だけ restart する
9. systemd が PID 1 であることを確認する
10. WSL architecture と同じ `haco_linux_<arch>.tar.gz` が installer package 内にあることを確認する

Fresh install の default path は **1回で完走**します。`install-windows.bat` を1回実行すると、WSL distribution 作成、`hacocoon` user 作成、systemd 有効化、package 内の `install.sh`、WSL post integration まで続けて実行し、install 完了後にだけ戻ります。途中で自分で `wsl -d Hacocoon` を起動して BAT をもう一度叩く必要はありません。

Ubuntu account setup を対話で行いたい場合だけ、次を使います。

```powershell
.\install-windows.bat -InteractiveUserSetup
```

この場合 installer が Ubuntu 標準の first-launch setup を起動します。Ubuntu user を作成して shell まで到達したら `exit` を1回実行すると、待機していた Windows installer が自動で再開し、そのまま `install.sh` と post integration を完了します。すでに dedicated distribution に通常の non-root default user がある場合、通常の再実行ではその user を再利用します。

自動作成 user には common install 実行中だけ一時的な passwordless bootstrap sudo rule を付与します。この広い一時 rule は `finally` 経路でも削除します。Install 完了後に残す passwordless sudo は、後述する exact な `haco host ensure` / `haco host shell` だけです。

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

Windows default path の gate は candidate `hacocoon-windows-amd64.zip` を作って展開し、package 内の `install-windows.bat` を **1回だけ**実行します。その1回で dedicated WSL user 作成から WSL 2、systemd、共通 `install.sh`、Incus、controller service/socket、`haco-host doctor`、WSL login integration まで成功することを要求します。Interactive user setup は opt-in の別 path であり、default installer の前提にはしません。

Native Ubuntu gate は candidate `hacocoon-ubuntu-amd64.tar.gz` を作って展開し、package 内の `install-ubuntu.sh` を実行します。Controller と trusted `haco-host` round trip が成功し、native login shell が変更されていないことまで確認します。

PR candidate package はまだ public Release ではないため release attestation を持ちません。Candidate E2E で provenance を無効化できるのはこの synthetic package の検証だけです。Release workflow は別途、実際に publish する architecture-specific payload そのものへ署名 / attestation を行います。
