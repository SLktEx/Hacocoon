# Windows / WSL セットアップ

Windows で Hacocoon の local runtime を使う場合、**普段使いの WSL を再利用せず、Hacocoon 専用の WSL 2 distribution を1個作り、systemd を PID 1 として動かす**のを標準にします。

この専用 distribution 自体が **Physical Host** です。通常の install が完了した後は、`wsl -d Hacocoon` でその Physical Host shell に留まらず、永続的な trusted Incus instance `haco-host` へそのまま入ります。

```text
Windows desktop
  |
  +-- 普段使いの Ubuntu / Debian 等      <- 触らない
  |
  +-- WSL 2: Hacocoon                    <- Physical Host / substrate
        |- systemd (PID 1)
        |- Hacocoon + Incus
        |- loop / Btrfs storage primitives
        |
        `- Incus: haco-host               <- TRUSTED / 普段入る Host
             |
             +-- 将来の Git / OCI / external tooling
             `-- operator shell

        Incus: managed Environments       <- UNTRUSTED agent workloads

Windows VS Code / external orchestrator
  -> stable Hacocoon client surface
  -> target Environment
```

現在の `haco-host` 実装範囲は [`design/trusted-host.ja.md`](design/trusted-host.ja.md) にまとめています。

## 普通に使うインストーラー

GitHub Release に `install-windows.ps1` を standalone installer として載せます。**Hacocoon repository を先に clone する必要はありません。**

管理者 PowerShell から:

```powershell
.\install-windows.ps1
```

を実行します。

標準の専用 WSL distribution 名は `Hacocoon`、標準 base distribution は `Ubuntu-26.04` です。

Fresh PC では Windows reboot や新規 distribution の初回 Linux user 作成が必要な場合があります。Hacocoon bootstrap が完了する前の:

```powershell
wsl -d Hacocoon
```

は通常の base distribution に入り、Linux user の初期設定を行えます。初期設定後に `install-windows.ps1` をもう一度実行してください。

Bootstrap 完了後は同じ:

```powershell
wsl -d Hacocoon
```

が通常の Hacocoon entry point になり、そのまま `haco-host` へ入ります。

## WSL 2 は必須で、installer が保証する

現在の `wsl --install` では新規 distribution は通常 WSL 2 で作られますが、Hacocoon はその default だけには依存しません。

Installer は専用 instance を `wsl --list --verbose` で確認し、もし **Hacocoon専用instanceだけ** が WSL 1 なら:

```powershell
wsl --set-version Hacocoon 2
```

で WSL 2 に変換します。

`wsl --set-default-version` は実行しません。普段使いの Ubuntu / Debian / Arch 等を WSL 2 に勝手に変換することも、global WSL default を変更することもありません。

## systemd も必須で、installer が保証する

Hacocoon の標準 local Incus path では、専用 Physical Host 内で **systemd が PID 1 として動いていること**を前提にします。

Linux bootstrap は必要な systemd package を入れ、既存 `/etc/wsl.conf` の他設定を残しながら:

```ini
[boot]
systemd=true
```

を保証します。

まだ PID 1 が systemd でなければ Windows 側は **Hacocoon専用distributionだけ** を:

```powershell
wsl --terminate Hacocoon
```

で停止して bootstrap を再実行します。

他の distribution まで止める `wsl --shutdown` はこの path では使いません。

必要な systemd support がない古い WSL は明示的に失敗し、必要なら利用者に `wsl --update` を案内します。Hacocoon が勝手に WSL 自体を update することはしません。

## Managed storage filesystem

Windows / WSL の supported path では Hacocoon-managed local storage に Btrfs を使います。Default local backend は sparse raw filesystem を作成し、`compress=zstd:3` で mount します。

`compress-force` は意図的に使いません。また自動 defrag / recompress は reflink / COW sharing を壊す可能性があるため行いません。

`haco host ensure` が最初の managed rootfs pool 作成になる場合があるので、WSL bootstrap は `btrfs-progs` も install します。

## Trusted `haco-host` bootstrap

Incus と Hacocoon release の準備後、bootstrap は Physical Host authority で:

```text
haco host ensure
```

を実行します。

この operation は:

- `haco-host` という Incus instance をちょうど1個 reconcile する
- rootfs を Hacocoon が選んだ managed Incus storage pool に置く
- `user.hacocoon.role=trusted-host` を ownership marker として付ける
- 同じ名前の既存 instance に marker がなければ takeover せず fail closed する
- owned instance が stopped なら start する
- 未知の state は推測せず失敗する

という動作をします。

通常 Environment 名 `host` は provider-local では同じ `haco-host` に衝突するため、Incus adapter 側で予約します。

通常 WSL user の login shell を変更するのは、この reconcile が成功した**後だけ**です。Trusted Host bootstrap が失敗した場合は Physical Host shell をそのまま recovery 用に残し、壊れた automatic entry loop にはしません。

## `wsl -d Hacocoon` が `haco-host` に入る仕組み

Installer は root-owned の `/usr/local/libexec/hacocoon-login` entry を作り、system-owned `haco` binary を専用 executable name で参照させ、その path を通常 non-root WSL user の login shell に設定します。

`haco` binary はこの invocation mode を判定し、command なしの interactive WSL launch なら:

```text
sudo -n <system-owned-haco> host shell
```

へ delegate します。

`haco host shell` は trusted Host を reconcile し、privileged-environment warning を表示してから `haco-host` 内の `/bin/bash -l` に入ります。

Explicit shell argument や non-interactive login-shell use は Physical Host 側の `/bin/bash` に残します。また WSL の explicit command execution は default interactive shell entry と分離できるので automation output に login warning を混ぜません。

## Automatic entry の privilege boundary

Automatic entry のためだけに `incus-admin` を付与しません。

代わりに、通常 WSL user が passwordless sudo できるのは system-owned binary に対する次の exact command だけです。

```text
haco host ensure
haco host shell
```

この sudoers rule を入れる前に、`haco` executable が `/usr/local/bin/haco` または `/usr/bin/haco` にあり、root-owned で、group/world writable でないことを確認します。生成した sudoers file は install 前に `visudo` で検証します。

Incus daemon socket や `/var/lib/incus` を `haco-host` に mount しません。

専用 Physical Host user に raw Incus admin authority を意図的に与えたい場合だけ、従来通り:

```powershell
.\install-windows.ps1 -GrantIncusAdmin
```

を使います。`incus-admin` は実質 root 相当なので opt-in のままです。

## Physical Host へ直接入る escape hatch

通常の interactive use:

```powershell
wsl -d Hacocoon
```

は `haco-host` に入ります。

Bootstrap / repair / host-only operation のために Physical Host は明示的に到達可能なまま残します。Root account の login shell は `hacocoon-login` に変更しないため、主な recovery path は:

```powershell
wsl -d Hacocoon -u root
```

です。

Explicit command は default interactive entry を通さず Physical Host 側で実行できます。例えば:

```powershell
wsl -d Hacocoon -- haco host ensure
```

です。Physical Host root authority が必要な command は、root escape hatch または明示的に許可された sudo rule を使って実行してください。

Automatic entry に失敗した場合は、別shellに黙ってfallbackして成功したふりをせず、Physical Host recovery command を明示します。

## `-SkipIncus`

Incus を別手段で管理する場合の:

```powershell
.\install-windows.ps1 -SkipIncus
```

は引き続き利用できます。

ただしこの mode では Hacocoon が trusted Host backend ready を保証できないため、default Physical Host login shell は変更せず、automatic `haco-host` entry も設定しません。

## Workspace / repository の置き場所

Default interactive entry と Workspace location は別の設計課題です。

WSL の通常体験では、今後 Hacocoon-managed storage と `haco-host` を中心に repository/workspace を扱う方向を優先します。ただし Core に「repository は永久に `haco-host` にある」と焼き込まず、将来 Physical Host や external Workspace provider に置ける seam は残します。

今回の login-shell 変更だけでは、`haco-host` rootfs 内の任意 path が sibling Environment へ自動 mount できるようにはなりません。Workspace-location migration が入るまでは、既存 Physical Host Workspace path を Windows から explicit command で操作できます。例えば:

```powershell
wsl -d Hacocoon -- sh -lc 'cd ~/src/my-repo && haco-vscode open .'
```

VS Code や external orchestrator は `haco-host` を SSH jump host として前提にせず、Hacocoon の stable client/control surface から対象 Environment へ接続する方針を維持します。

## standalone installer / bootstrap がすること

Relevant な処理順は次です。

1. instance 名 / base distribution / Hacocoon version を検証
2. Hacocoon専用 named WSL だけを作成または再利用
3. その distribution だけ WSL 2 を保証
4. systemd support を install し `systemd=true` を保証
5. 必要ならその distribution だけ restart
6. `-SkipIncus` でない限り Incus を install / start
7. Btrfs userspace tool と Hacocoon release を install
8. trusted `haco-host` を reconcile
9. narrow automatic-entry sudo rule を install
10. 通常 non-root user の login shell を `hacocoon-login` に変更

Global WSL default、`.wslconfig`、無関係な distribution、root user の login shell は変更しません。

## Hacocoon開発者向けのcheckout版

Repository checkout 内には引き続き:

```powershell
.\scripts\bootstrap-windows.ps1
```

があります。

Checkout 内の `bootstrap-wsl.sh` / `install.sh` を使いますが、standalone installer と同じ WSL 2 / systemd / Incus / trusted-host entry contract に合わせます。

## Acceptance の扱い

Repository CI では Go test、shell / PowerShell syntax、WSL 2 / systemd policy、installer / release integrity、Windows kernel がなくても確認できる trusted-host reconciliation を検証します。

次は real Windows host 上での acceptance が必要です。

- 初回 Linux user setup
- WSL restart behavior
- real managed Btrfs creation
- real `haco-host` image acquisition / start
- `wsl -d Hacocoon` から `haco-host` への login-shell transition
- Physical Host root recovery
- Default entry変更後の Windows VS Code / orchestrator integration

Repository CI だけでこれらの Windows-host-dependent behavior を実機確認済みとは扱いません。
