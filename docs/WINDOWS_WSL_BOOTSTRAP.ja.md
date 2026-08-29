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

Windows / WSL の lifecycle 自体は Hacocoon Core に入れず、host-side installer だけが初期 setup を補助します。

## 普通に使うインストーラー

GitHub Release に `install-windows.ps1` を standalone installer として載せます。**Hacocoon repository を先に clone する必要はありません。**

Release asset を Windows に保存して、管理者 PowerShell から:

```powershell
.\install-windows.ps1
```

を実行します。

標準の専用 WSL instance 名は:

```text
Hacocoon
```

標準 base distribution は:

```text
Ubuntu-26.04
```

です。

Fresh PC では Windows reboot や新規 distribution の初回 Linux user 作成が必要な場合があります。その場合 installer は専用 WSL を作ったところで一度止まります。

```powershell
wsl -d Hacocoon
```

で一度起動して Linux user の初期設定を済ませ、終了後に `install-windows.ps1` をもう一度実行してください。

## standalone installer がすること

Installer は次を行います。

1. instance 名 / base distro / Hacocoon version を検証
2. `Hacocoon` という named WSL が既にあればそれだけを再利用
3. なければ `wsl --install <distro> --name <name> --no-launch` で専用 instance を作成
4. 選択した Hacocoon Release から `checksums.txt` / `bootstrap-wsl.sh` / `install.sh` を取得
5. Linux bootstrap script を SHA-256 checksum で検証
6. 専用 WSL 内で Linux bootstrap を実行
7. Incus と `haco` / `haco-vscode` を install

Release には Windows installer だけでなく、Linux bootstrap script も個別 asset として載せます。そのため standalone installer は repository checkout に依存しません。

Repository が private の間は、release asset の取得に authenticated `gh` CLI または `GH_TOKEN` / `GITHUB_TOKEN` が必要です。Public release になれば通常の HTTPS download が使えます。

## 専用 WSL のルール

Installer は **default WSL を選びません**。最初に見つかった distribution を流用することもしません。

`Hacocoon` がなければ、重要な platform operation は概念的にこれだけです。

```powershell
wsl --install Ubuntu-26.04 --name Hacocoon --no-launch
```

`wsl --set-default-version` は実行しません。既存の default WSL distribution も変更しません。

既存 Ubuntu / Debian / Arch などに対して、次を自動では行いません。

- unregister / reset / delete
- WSL 1 -> WSL 2 conversion
- default distribution 変更
- 今後作る distribution に効く global default 変更
- Linux user の置換
- `/etc/wsl.conf` や Windows `.wslconfig` の任意書き換え

利用中の WSL catalog に `Ubuntu-26.04` がない場合は:

```powershell
wsl --list --online
```

で確認し、使える名前を指定できます。

```powershell
.\install-windows.ps1 -BaseDistro Ubuntu
```

専用 instance 名を変えたい場合:

```powershell
.\install-windows.ps1 -InstanceName Hacocoon-Dev
```

も使えます。これは別の general-purpose WSL を再利用する機能ではなく、別名の専用 instance を作るための option です。

## Hacocoon version

既定では最新 Release を入れます。Version を固定する場合:

```powershell
.\install-windows.ps1 -HacocoonVersion v0.8.0
```

Linux 側の release installer も binary archive を `checksums.txt` で検証してから `haco` / `haco-vscode` を install します。

## incus-admin は勝手に付けない

Incus package の install と、Incus daemon の管理権限付与は別です。

`incus-admin` は host path / device の attach や security setting 変更が可能なので、実質 root 相当の強い権限です。

そのため自動では付与しません。

この専用 WSL user に Incus 管理権限を明示的に与える場合だけ:

```powershell
.\install-windows.ps1 -GrantIncusAdmin
```

を使います。

Group membership 変更後は WSL shell を開き直すか `newgrp incus-admin` を使ってください。

Incus を別手段で管理する場合:

```powershell
.\install-windows.ps1 -SkipIncus
```

も利用できます。

## Hacocoon開発者向けのcheckout版

Repository checkout 内には引き続き:

```powershell
.\scripts\bootstrap-windows.ps1
```

も残します。

これは Hacocoon 自体を開発するとき用で、checkout 内の `bootstrap-wsl.sh` / `install.sh` を使います。

一般利用者の標準導線は Release asset の **`install-windows.ps1`** です。

## systemd と Incus init

systemd が動いている場合、Linux bootstrap は package で入った Incus service の起動を試みます。

Incus daemon に接続でき、storage pool が存在しない場合だけ `incus admin init --minimal` を実行します。

`/etc/wsl.conf` や Windows `.wslconfig` を installer が勝手に書き換えることはしません。必要な host configuration は明示的に直してから再実行します。

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

VS Code は Windows desktop client のままです。Hacocoon 専用 AI UI は作りません。

```text
install-windows.ps1
  -> Hacocoon 専用 WSL 2
  -> Incus + Hacocoon
  -> 専用 WSL filesystem 上の Workspace
  -> haco-vscode open .
  -> Windows VS Code Remote-SSH
  -> Hacocoon Environment の /workspace
  -> VS Code 既存 AI / terminal / Git UI
```

## Acceptance の扱い

CI で確認するのは PowerShell / shell syntax、checksum inclusion、release packaging、repository integration までです。

次は real Windows host 上での acceptance が必要です。

- Windows feature enablement
- reboot 後の WSL 起動
- named distribution download / install
- first-run Linux user setup
- real WSL 2 kernel
- real Incus daemon
- Windows desktop VS Code Remote-SSH

これらは実際に Windows + Hacocoon専用WSL 2 + Incus host で実行した場合だけ pass と扱います。
