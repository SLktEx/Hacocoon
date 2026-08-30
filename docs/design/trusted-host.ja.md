# Trusted `haco-host`

Status: partial.

## 概要

`haco-host` は Hacocoon が管理する永続的な trusted logical Host です。Local Incus backend では `haco-host` という名前の Incus system instance として実装し、通常の untrusted Environment とは明確に分離します。

Hacocoon process、Incus daemon、loop device、storage mount を実際に動かす Linux / WSL distribution は **Physical Host** と呼びます。Physical Host は platform primitive の authority を持ち続けます。`haco-host` は user が普段入る host-like な場所であり、今後の slice では developer tooling や external-service operation の標準実行場所にもしていきます。

```text
Physical Host / WSL
  |- Hacocoon process
  |- Incus daemon
  |- loop / Btrfs platform primitives
  `- haco-host                  TRUSTED
       |
       `- normal interactive Host UX

Managed Environments            UNTRUSTED
```

`haco-host` は trusted computing base の一部です。Environment ではなく、agent sandbox として扱ってはいけません。

## 現在実装済みの slice

現在の Incus implementation には次が入っています。

- `haco host ensure`: 永続的な `haco-host` を1個 reconcile する
- `haco host shell`: instance を running にしたうえで interactive login shell に入る
- `user.hacocoon.role=trusted-host` という Incus ownership marker
- Hacocoon が選択した managed Incus storage pool 上への rootfs 配置
- infrastructure instance と衝突しないよう Incus backend で Environment 名 `host` を予約
- reconcile 成功後、通常の WSL interactive entry を `haco-host` に向ける WSL bootstrap

#275 全体はまだ未完です。この slice では Git/GitHub、OCI/containerd、cloud credentials、一般的な external tooling、Windows mount、WSL interop はまだ `haco-host` へ移していません。また #276/#277 で必要になる Physical Host controller API もまだ実装していません。

## Trust と authority

Incus control authority は Physical Host に残します。

`haco-host` に入るためだけに Incus daemon socket、`/var/lib/incus`、Physical Host 上の Hacocoon state directory、または同等の raw provider-control capability を渡してはいけません。

```text
operator
   |
   | haco host shell
   v
Physical Host haco
   |
   | Incus control
   v
Incus daemon
   |
   v
haco-host
```

Environment から `haco-host` へ直接アクセスさせません。将来 Environment から privileged operation を要求する場合も、ambient な trusted Host access にせず Hacocoon の policy / capability / approval boundary を通します。

## Ownership と name collision

Incus instance 名 `haco-host` は infrastructure-owned です。

作成時に `incus init` と同時に Hacocoon ownership marker を付与します。既存 instance を再利用する場合はその marker が完全一致することを要求します。無関係な instance や古い instance が `haco-host` を占有している場合、Hacocoon は takeover、start、delete、設定変更をせず fail closed します。

通常の Environment 名 `host` も provider-local では `haco-host` になるため、Incus adapter は Incus に触る前にその Environment 名を拒否します。

複数の `ensure` が同時に走って create race になった場合、負けた側は exact ownership marker を確認できたときだけ winner の instance を再利用できます。未知の Incus state は推測せず incompatible state として失敗します。

## Storage

`haco-host` は通常の Hacocoon Incus storage integration が選択した root storage pool を使います。Default local backend では rootfs が Hacocoon の sparse-raw Btrfs-backed Incus pool に入り、将来の `/var/lib/containerd` や repository data などを unmanaged な Physical Host filesystem location に依存させずに済みます。

ただし同じ Btrfs 上にあるだけで、将来の `haco-host` data が Seed / Environment と自動的に物理 COW share されるとはみなしません。そのような claim は別途 measurement が必要です。

## WSL の default entry

Supported Windows installer が正常完了すると、専用 WSL distribution の通常 non-root user の login shell を `hacocoon-login` という専用 entry に変更します。

この entry は別 executable name で起動した同じ trusted `haco` binary です。Command なしの interactive launch では次へ delegate します。

```text
sudo -n <system-owned-haco> host shell
```

Installer が passwordless sudo を許可するのは、その WSL user に対する exact な `haco host ensure` と `haco host shell` だけです。`incus-admin` は引き続き default では付与せず、Incus socket も `haco-host` へ公開しません。

そのため通常 UX は次になります。

```powershell
wsl -d Hacocoon
```

```text
Physical Host login entry
    -> haco host shell
    -> haco-host
```

明示的な WSL command は Physical Host command のままです。また root account の login shell は変更しません。したがって緊急時には例えば次で Physical Host へ直接入れます。

```powershell
wsl -d Hacocoon -u root
```

Installer は `haco host ensure` が成功した後にだけ通常 user の login shell を変更します。Bootstrap に失敗した場合、壊れた自動 entry loop に入れず Physical Host recovery path を残します。

`-SkipIncus` を使った場合は backend ready を Hacocoon が保証できないため、自動 `haco-host` entry は設定しません。

## Interactive warning

`haco host shell` は `haco-host` へ入る直前に短い privileged-environment warning を表示します。Japanese locale では日本語、その他では英語を出します。

Warning を出すのは interactive Host-shell path だけで、non-interactive WSL command の output には混ぜません。

## 今後の follow-up

次はこの slice の範囲外です。

- Git/GitHub や selected external-service tooling の標準実行場所を `haco-host` にする
- Host OCI store / containerd を `haco-host` 内で動かす
- reusable credential を通常 Environment に渡さない explicit credential injection / broker
- trusted Host だけに optional WSL / Windows interop を与える
- Incus socket を渡さず `haco-host` 内から Physical-Host-authority の `haco` operation を実行するための Physical Host controller / control channel
- `haco` と `haco-host` の CLI responsibility split を完了する
- repository を永久に `haco-host` 固定と Core に仮定させず、長期的な Workspace / repository location seam を実装する

## Acceptance boundary

Repository test では ownership reconciliation、name collision refusal、stopped/running state、create race、CLI routing、locale warning、login-mode identification を確認します。CI では bootstrap shell syntax も検証します。

ただし、実際の Windows terminal から WSL distribution を起動し、instance 作成、login shell 変更、`haco-host` entry まで成功することは repository CI だけでは証明できません。Real Windows + WSL 2 + systemd + Incus acceptance が完了するまでは host-dependent path を実機確認済みとは扱いません。
