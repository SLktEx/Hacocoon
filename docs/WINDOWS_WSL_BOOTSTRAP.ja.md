# Windows / WSL セットアップ

Windows で Hacocoon のローカル runtime を使う場合、基本構成は次です。

```text
Windows desktop
  -> WSL 2
     -> Incus
        -> Hacocoon Environment
  -> VS Code desktop
     -> haco-vscode
     -> Remote-SSH
```

Windows や WSL のライフサイクルそのものを Hacocoon Core に持ち込まず、bootstrap script で初期セットアップだけを補助します。

## まず実行するもの

Hacocoon を checkout した PowerShell から実行します。

```powershell
.\scripts\bootstrap-windows.ps1
```

既に WSL と Linux distribution がある場合は、default distribution を使います。Default が判定できない場合は、最初に見つかった installed distribution を使います。

Distribution を明示したい場合:

```powershell
.\scripts\bootstrap-windows.ps1 -Distro Ubuntu
```

この script は Windows 標準の `wsl.exe` を使います。既存 distribution を unregister / reset / delete したり、勝手に置き換えたりしません。

利用可能な distribution 名は、そのPC上で次を実行して確認してください。

```powershell
wsl --list --online
```

特定バージョンの Ubuntu などを使いたい場合は、ここに表示された名前を `-Distro` に渡してください。

## WSL がまだない場合

利用できる distribution がない場合、bootstrap は WSL 2 distribution の install を開始します。

新規 WSL install では Windows reboot や、distribution 初回起動時の Linux user 作成が必要になる場合があります。この部分は Windows / WSL が所有するセットアップです。

そのため fresh PC では、最初の実行で WSL を入れたあと一旦終了する場合があります。

その場合:

```powershell
wsl -d <Distro>
```

で一度起動し、Linux user の初期設定を済ませてから同じ bootstrap をもう一度実行してください。

## WSL 内で入るもの

apt 系 distribution では `scripts/bootstrap-wsl.sh` が次を準備します。

- CA certificates
- `curl`
- `tar`
- `git`
- Incus（`-SkipIncus` を付けない場合）

その後、Hacocoon 本体の install は既存の `scripts/install.sh` に委譲します。

つまり Windows bootstrap が別の download / release install 実装を持つわけではありません。

Install される binary:

```text
haco
haco-vscode
```

特定 version を入れる場合:

```powershell
.\scripts\bootstrap-windows.ps1 -HacocoonVersion v0.8.0
```

Private repository の release を取得する場合は、`scripts/install.sh` が利用できる GitHub authentication が別途必要です。

## incus-admin は勝手に付けない

Incus の package install と、ユーザーへ Incus daemon の管理権限を与えることは別です。

`incus-admin` は local Incus 上で host path や device を instance に付けたり security setting を変更できるため、実質的に root 相当の強い権限です。

そのため bootstrap は user を `incus-admin` に自動追加しません。

このPCの所有者として明示的に許可する場合だけ:

```powershell
.\scripts\bootstrap-windows.ps1 -GrantIncusAdmin
```

を使います。

Group membership を追加した後は、WSL shell を開き直すか `newgrp incus-admin` が必要です。

Incus を別手段で管理している場合:

```powershell
.\scripts\bootstrap-windows.ps1 -SkipIncus
```

も使えます。

## systemd と Incus init

systemd が動いている場合、bootstrap は package で入った Incus service の起動を試みます。

Incus daemon に接続でき、storage pool がまだ存在しない場合だけ:

```text
incus admin init --minimal
```

を使って最小構成を作ります。

`/etc/wsl.conf` は bootstrap が勝手に書き換えません。Systemd が無効な distribution なら、その設定は明示的に直してから再実行してください。

## 既存 WSL を壊さない

既存 WSL distribution は user-owned state として扱います。

Bootstrap は次を自動では行いません。

- unregister
- reset
- delete
- WSL 1 -> WSL 2 conversion
- default distribution の変更
- Linux user の置換
- 任意の WSL config 書き換え

選択した distribution が WSL 1 の場合は停止し、必要なら利用者自身で次を実行するよう案内します。

```powershell
wsl --set-version <Distro> 2
```

Conversion は時間・容量・既存VM状態に影響し得るため、自動化しません。

## Workspace は WSL 側に置く

通常の Incus backend を使う Workspace は、Windows の `/mnt/c` 配下より **WSL の Linux filesystem 側**へ置くのを基本にします。

例:

```bash
mkdir -p ~/src
cd ~/src
git clone <repository>
cd <repository>
haco-vscode open .
```

Linux ownership / filesystem semantics / Incus bind mount / 開発ツールを同じ側に揃えられます。

## VS Code との関係

この bootstrap は VS Code の AI UI を作ったり置き換えたりしません。

VS Code は Windows desktop client のままで、`haco-vscode` は薄い Client Adapter です。

最終的な流れ:

```text
PowerShell bootstrap
  -> WSL 2 + Incus + Hacocoon
  -> WSL filesystem 上の Workspace
  -> haco-vscode open .
  -> Windows VS Code Remote-SSH
  -> Hacocoon Environment の /workspace
  -> VS Code 既存 AI / terminal / Git UI
```

## Acceptance の扱い

CI で確認できるのは script syntax や repository integration までです。

次は本物の Windows host で実行しない限り pass と扱いません。

- Windows feature enablement
- reboot 後の WSL 起動
- Store / web download の distribution install
- real WSL 2 kernel
- real Incus daemon
- Windows desktop VS Code Remote-SSH

これらは Windows + WSL 2 + Incus host での real acceptance test として別扱いにします。
