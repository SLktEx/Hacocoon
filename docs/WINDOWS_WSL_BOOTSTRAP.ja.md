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

Fresh Ubuntu WSL では通常の first-launch user setup が必要な場合があります。その場合 installer は distribution 作成後に止まり、次を案内します。

```powershell
wsl -d Hacocoon
```

Ubuntu user setup 完了後、`install-windows.bat` をもう一度実行します。

## WSL image cache 検証経路

`-UseCachedWslImage` は、Windows / WSL の install を何度も検証するときのための validation-oriented option です。明示的に指定しない限り、通常 installer の挙動は変わりません。

有効時は、installer package と同じ場所にある `ubuntu.wsl` を Ubuntu 26.04 の local image cache として使います。`ubuntu.wsl` が無ければ、Microsoft の WSL `DistributionInfo.json` から現在の Windows architecture に対応する `Ubuntu-26.04` の URL と SHA256 を取得し、一時ファイルへ download、公開 SHA256 を検証してから `ubuntu.wsl` に昇格します。

専用 distribution の作成は named install 経路のままです。

```powershell
wsl --install --from-file .\ubuntu.wsl --name Hacocoon --no-launch
```

`-UseCachedWslImage` は現在 `Ubuntu-26.04` だけを対象とし、`-WebDownload` とは同時指定できません。`ubuntu.wsl` は Release installer package には同梱しません。Local / CI の install 検証を高速化するための artifact です。

Windows installer の GitHub Actions E2E でもこの経路を使います。Candidate Windows package を展開したあと、OS / architecture / Ubuntu version を含む key で `actions/cache` から `ubuntu.wsl` を復元し、phase 1 / phase 2 の両方で packaged BAT に `-UseCachedWslImage` を渡します。Cache miss の場合は installer が上記の検証付き download を行い、生成された `ubuntu.wsl` は job 終了時に Actions cache へ保存され、次回以降の E2E で再利用されます。

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

Windows gate は candidate `hacocoon-windows-amd64.zip` を作って展開し、利用可能なら cache 済み Ubuntu 26.04 `.wsl` image を復元して、package 内の `install-windows.bat -UseCachedWslImage` を実行します。必要な Ubuntu first-launch user creation を CI で再現したあと、**同じ cache option を付けて同じ packaged BAT をもう一度実行**し、WSL 2、systemd、Incus、controller service/socket、`haco-host doctor`、WSL login integration まで成功を要求します。

Native Ubuntu gate は candidate `hacocoon-ubuntu-amd64.tar.gz` を作って展開し、package 内の `install-ubuntu.sh` を実行します。Controller と trusted `haco-host` round trip が成功し、native login shell が変更されていないことまで確認します。

PR candidate package はまだ public Release ではないため release attestation を持ちません。Candidate E2E で provenance を無効化できるのはこの synthetic package の検証だけです。Release workflow は別途、実際に publish する architecture-specific payload そのものへ署名 / attestation を行います。
