# Windows / WSL セットアップ

Windows で Hacocoon のローカル runtime を使う場合、**普段使いの WSL を再利用せず、Hacocoon 専用の WSL 2 instance を1個作り、systemd を PID 1 として動かす**のを標準にします。

```text
Windows desktop
  |
  +-- 普段使いの Ubuntu / Debian 等      <- 触らない
  |
  +-- WSL 2: Hacocoon                    <- はこーん専用
        -> systemd (PID 1)
        -> Incus
           -> Hacocoon Environment

Windows VS Code
  -> haco-vscode
  -> Remote-SSH
  -> Hacocoon Environment
```

Windows / WSL の lifecycle 自体は Hacocoon Core に入れず、host-side installer が初期 setup だけを担当します。

## 普通に使うインストーラー

GitHub Release に `install-windows.ps1` を standalone installer として載せます。**Hacocoon repository を先に clone する必要はありません。**

Release asset を Windows に保存して、管理者 PowerShell から:

```powershell
.\install-windows.ps1
```

を実行します。

標準の専用 WSL instance 名は `Hacocoon`、標準 base distribution は `Ubuntu-26.04` です。

Fresh PC では Windows reboot や新規 distribution の初回 Linux user 作成が必要な場合があります。その場合 installer は専用 WSL を作ったところで一度止まります。

```powershell
wsl -d Hacocoon
```

で一度起動して Linux user の初期設定を済ませ、終了後に `install-windows.ps1` をもう一度実行してください。

## WSL 2 は必須で、installer が保証する

現在の `wsl --install` では新規 distribution は通常 WSL 2 で作られますが、Hacocoon はその default だけには依存しません。

Installer は専用 instance を `wsl --list --verbose` で確認し、もし **Hacocoon専用instanceだけ** が WSL 1 なら:

```powershell
wsl --set-version Hacocoon 2
```

で WSL 2 に変換します。

`wsl --set-default-version` は実行しません。普段使いの Ubuntu / Debian / Arch 等を WSL 2 に勝手に変換することもありません。

Hacocoon が所有する専用 instance だけを自動修正し、その他の WSL は user-owned state として扱います。

## systemd も必須で、installer が保証する

Hacocoon の標準 local Incus path では、専用 WSL 内で **systemd が PID 1 として動いていること**を前提にします。

apt 系 distribution では Linux bootstrap が次を install します。

```text
systemd
systemd-sysv
```

そのうえで専用 instance の `/etc/wsl.conf` を更新し、`[boot]` section に:

```ini
[boot]
systemd=true
```

を保証します。

既存の `/etc/wsl.conf` にある他の section / key は残します。Windows 全体に効く `.wslconfig` は変更しません。

まだ PID 1 が systemd でなければ、Linux bootstrap は Windows installer に restart-required を返します。Windows側は **Hacocoon専用instanceだけ** を:

```powershell
wsl --terminate Hacocoon
```

で停止し、自動で bootstrap を再実行します。

再起動後も systemd が PID 1 でなければ install は失敗として止まります。

また `wsl --version` が使えない古い WSL は systemd 対応外として止め、必要なら利用者に:

```powershell
wsl --update
```

を明示的に実行してもらいます。Hacocoon が勝手に WSL 自体を update することはしません。

## Managed storage filesystem

Windows / WSL の supported path では、Hacocoon-managed local storage に Btrfs を使います。Managed Btrfs filesystem は標準で `compress=zstd:3` を付けて mount します。`compress-force` は意図的に使わず、圧縮しにくいdataはBtrfsの通常heuristicsに任せてuncompressedのまま扱えるようにします。

ZFS は supported WSL baseline の必須要件にしません。ZFSのためのcustom WSL kernel / module stackをHacocoonが要求・管理することもありません。Native Linux向けの別storage backendは将来独立に検討できますが、WSL pathはBtrfs-firstのままです。

Mount optionの変更は新しく書かれるextentに効きます。既存dataを自動defrag/recompressするとCOW/reflinkの共有extentを減らす可能性があるため、Hacocoonは自動再圧縮を行いません。

## standalone installer がすること

Installer は次を行います。

1. instance 名 / base distro / Hacocoon version を検証
2. `Hacocoon` という named WSL が既にあればそれだけを再利用
3. なければ `wsl --install <distro> --name <name> --no-launch` で専用 instance を作成
4. 専用 instance が WSL 2 であることを確認し、必要ならそのinstanceだけ変換
5. 選択した Hacocoon Release から `checksums.txt` / `bootstrap-wsl.sh` / `install.sh` を取得
6. Linux bootstrap script を SHA-256 checksum で検証
7. systemd package を入れ、`/etc/wsl.conf` に `systemd=true` を設定
8. 必要なら Hacocoon専用WSLだけ terminate / restart
9. systemd が PID 1 であることを検証
10. Incus を install して systemd service として起動
11. `haco` / `haco-vscode` を install

Repository が private の間は、release asset の取得に authenticated `gh` CLI または `GH_TOKEN` / `GITHUB_TOKEN` が必要です。Public release になれば通常の HTTPS download が使えます。

## 専用 WSL のルール

Installer は **default WSL を選びません**。最初に見つかった distribution を流用することもしません。

`Hacocoon` がなければ、重要な platform operation は概念的に:

```powershell
wsl --install Ubuntu-26.04 --name Hacocoon --no-launch
```

です。

既存の無関係な WSL に対して、unregister / reset / delete / default distribution変更 / global WSL default変更 / Linux user置換 / Windows `.wslconfig` 書き換えは行いません。

Base distribution や専用 instance 名は明示的に変更できます。

```powershell
.\install-windows.ps1 -BaseDistro Ubuntu
.\install-windows.ps1 -InstanceName Hacocoon-Dev
```

これは general-purpose WSL を再利用する機能ではなく、別名の専用 instance を作るための option です。

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

も利用できます。ただし `-SkipIncus` の場合でも、**WSL 2 + systemd** はHacocoon専用hostの標準contractとして保証します。

## Hacocoon開発者向けのcheckout版

Repository checkout 内には引き続き:

```powershell
.\scripts\bootstrap-windows.ps1
```

も残します。

これは checkout 内の `bootstrap-wsl.sh` / `install.sh` を使いますが、standalone installer と同じく WSL 2 と systemd を保証します。

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

## VS Code との関係

VS Code は Windows desktop client のままです。Hacocoon 専用 AI UI は作りません。

```text
install-windows.ps1
  -> Hacocoon 専用 WSL 2
  -> systemd
  -> Incus + Hacocoon
  -> 専用 WSL filesystem 上の Workspace
  -> haco-vscode open .
  -> Windows VS Code Remote-SSH
  -> Hacocoon Environment の /workspace
  -> VS Code 既存 AI / terminal / Git UI
```

## Acceptance の扱い

CI で確認するのは PowerShell / shell syntax、WSL 2/systemd contract、checksum inclusion、release packaging、repository integration までです。

次は real Windows host 上での acceptance が必要です。

- Windows feature enablement
- reboot 後の WSL 起動
- named distribution download / install
- first-run Linux user setup
- real WSL 2 conversion
- systemd PID 1 activation
- real Incus daemon
- Windows desktop VS Code Remote-SSH

これらは実際に Windows + Hacocoon専用WSL 2 + systemd + Incus host で実行した場合だけ pass と扱います。
