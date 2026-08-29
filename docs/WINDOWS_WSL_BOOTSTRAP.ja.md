# Windows / WSL セットアップ

Windows で Hacocoon のローカル runtime を使う場合、**普段使いの WSL を再利用せず、Hacocoon 専用の WSL 2 instance を1個作る**のを標準にします。

```text
Windows desktop
  |
  +-- 普段使いの Ubuntu / Debian 等      <- 触らない
  |
  +-- WSL 2: Hacocoon                    <- はこーん専用
        -> Incus
           -> Hacocoon Environment

Windows VS Code
  -> haco-vscode
  -> Remote-SSH
  -> Hacocoon Environment
```

Windows / WSL の lifecycle 自体は Hacocoon Core に入れず、bootstrap script だけが host setup を補助します。

## まず実行するもの

Hacocoon を checkout した PowerShell から:

```powershell
.\scripts\bootstrap-windows.ps1
```

標準の WSL instance 名は:

```text
Hacocoon
```

標準 base distribution は `Ubuntu-26.04` です。

別の base を使う場合:

```powershell
.\scripts\bootstrap-windows.ps1 -BaseDistro Ubuntu
```

専用 instance の名前だけ変える場合:

```powershell
.\scripts\bootstrap-windows.ps1 -InstanceName Hacocoon-Dev
```

WSL は同じ Ubuntu release でも `--name` で別 instance として install できます。Hacocoon はこの仕組みを使い、既存の普段使い WSL と完全に分けます。

## 専用 WSL のルール

Bootstrap は **default WSL を選びません**。最初に見つかった distribution を流用することもしません。

`Hacocoon` が既に存在する場合だけ、その named instance を再利用します。存在しない場合は概念的に次を実行します。

```powershell
wsl --set-default-version 2
wsl --install Ubuntu-26.04 --name Hacocoon --no-launch
```

必要なら `-WebDownload` も利用できます。

既存の Ubuntu / Debian / Arch 等には次を行いません。

- unregister
- reset
- delete
- WSL 1 -> WSL 2 自動変換
- default distribution 変更
- Linux user の置換
- 任意の WSL config 書き換え

利用中の WSL catalog に `Ubuntu-26.04` がない場合は:

```powershell
wsl --list --online
```

で確認し、使える名前を `-BaseDistro` に渡してください。

## fresh PC の場合

WSL component の有効化や distribution install では、Windows reboot が必要になる場合があります。また、新規 distribution は初回 Linux user 作成が必要です。

そのため fresh PC では、1回目の bootstrap が専用 WSL を作ったところで止まる場合があります。

その場合:

```powershell
wsl -d Hacocoon
```

で一度起動し、Linux user の初期設定を完了して終了し、同じ bootstrap をもう一度実行してください。

Windows reboot や Linux credential を勝手に自動化しないため、意図的に resume 可能な2段階にしています。

## 専用 WSL 内で入るもの

apt 系 distribution では `scripts/bootstrap-wsl.sh` が次を準備します。

- CA certificates
- `curl`
- `tar`
- `git`
- Incus（`-SkipIncus` なしの場合）

その後、Hacocoon 本体は既存の `scripts/install.sh` へ委譲します。

Install される binary:

```text
haco
haco-vscode
```

特定 version を入れる場合:

```powershell
.\scripts\bootstrap-windows.ps1 -HacocoonVersion v0.8.0
```

Private repository の release を取得する場合は、`scripts/install.sh` が使える GitHub authentication が必要です。

## incus-admin は勝手に付けない

Incus package の install と、Incus daemon の管理権限付与は別です。

`incus-admin` は host path / device の attach や security setting 変更が可能なので、実質 root 相当の強い権限です。

そのため自動では付与しません。

この専用 WSL user に Incus 管理権限を明示的に与える場合だけ:

```powershell
.\scripts\bootstrap-windows.ps1 -GrantIncusAdmin
```

を使います。

Group membership 変更後は WSL shell を開き直すか `newgrp incus-admin` を使ってください。

Incus を別手段で管理する場合:

```powershell
.\scripts\bootstrap-windows.ps1 -SkipIncus
```

も利用できます。

## systemd と Incus init

systemd が動いている場合、bootstrap は package で入った Incus service の起動を試みます。

Incus daemon に接続でき、storage pool が存在しない場合だけ `incus admin init --minimal` を実行します。

`/etc/wsl.conf` や Windows 側 `~/.wslconfig` を bootstrap が勝手に書き換えることはしません。必要な host configuration は明示的に直してから再実行します。

## Workspace は専用 WSL に置く

通常の Incus backend を使う Workspace は **Hacocoon 専用 WSL の Linux filesystem** に置くのを標準にします。

PowerShell:

```powershell
wsl -d Hacocoon
```

その中で:

```bash
mkdir -p ~/src
cd ~/src
git clone <repository>
cd <repository>
haco-vscode open .
```

これで Linux ownership / filesystem semantics / Incus bind mount / Hacocoon tooling を同じ host boundary に揃えられます。

## VS Code との関係

VS Code は Windows desktop client のままです。専用 AI UI は作りません。

```text
PowerShell bootstrap
  -> Hacocoon 専用 WSL 2
  -> Incus + Hacocoon
  -> 専用 WSL filesystem 上の Workspace
  -> haco-vscode open .
  -> Windows VS Code Remote-SSH
  -> Hacocoon Environment の /workspace
  -> VS Code 既存 AI / terminal / Git UI
```

## Acceptance の扱い

CI で確認できるのは script syntax や repository integration までです。

次は real Windows host 上での acceptance が必要です。

- Windows feature enablement
- reboot 後の WSL 起動
- dedicated distribution download / install
- first-run Linux user setup
- real WSL 2 kernel
- real Incus daemon
- Windows desktop VS Code Remote-SSH

これらは実際に Windows + Hacocoon専用WSL 2 + Incus host で実行した場合だけ pass と扱います。
