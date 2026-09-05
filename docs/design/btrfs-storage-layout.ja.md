# Btrfs ストレージレイアウト

状態: **supported な local storage path は Incus-owned loop-backed Btrfs のみ。**

Milestone: **v0.20 Managed Btrfs Rootfs Storage** / **v0.21 Managed Btrfs Transparent Compression** / **v0.25 Incus-owned Btrfs Storage Acceptance**。

## 既定の local layout

runtimeが受け入れるattachmentはlocal `incus_pool` identityだけです。削除した `driver`/`source` は既存poolがあっても拒否し、検査失敗時に代替poolを作りません。mount policyのread/readback失敗もfail closedです。real-Incus storage CIにはpolicy照合中の既存rootfs・Workspace sentinel data保持も含めます。

local composition は `source=` を指定せず、Incus へ既定 pool を lazy に作成させる。

```text
Incus pool: haco-local-default
  driver=btrfs
  size=128GiB
  btrfs.mount_options=compress=zstd:3,noatime,nodiscard
        |
        v
/var/lib/incus/disks/haco-local-default.img
  (Incus-owned sparse Linux file)
        |
        v
     loop device
        |
        v
  Btrfs filesystem
        |
        v
/var/lib/incus/storage-pools/haco-local-default
  |- cached Base image volumes
  |- Tooling Base builders
  |- Seed builders / cached Seed image volumes
  |- trusted haco-host rootfs
  |- Environment rootfs volumes
  `- Incus snapshots / clones
```

backing image 作成、loop attach、Btrfs format、mount/unmount lifecycle、対応可能な loop-pool grow は Incus が所有する。Hacocoon が所有するのは desired な pool identity と policy だけで、別の Host-managed block/mount lifecycle は持たない。

## sparse file と WSL sparse VHD は別物

Incus の loop-backed Btrfs pool は sparse な **Linux file** を使い、logical 128GiB を最初から全量 physical allocate しない。これは WSL の `sparseVhd` / sparse-VHDX mode とは別で、Hacocoon はこの storage design のために WSL sparse-VHD mode を有効化しない。Windows Host 側 VHDX の reclamation は明示的な Host/operator 運用として扱う。

## なぜ rootfs object を同じ pool で共有するのか

Base、Tooling、Seed、trusted host、Environment の rootfs data を同じ Hacocoon Btrfs pool へ置き、Incus の Btrfs storage-driver behavior を lifecycle 全体へ適用する。

- 圧縮しやすい data は Btrfs transparent compression で physical bytes を減らせる。
- Incus Btrfs snapshot / clone で copy-on-write sharing を維持できる。
- storage driver が共有できる範囲では Seed 由来 Environment が unchanged extent を共有できる。
- storage maintenance を任意の Host data ではなく Hacocoon rootfs data へ限定できる。

隔離のためだけに Environment や Seed ごとへ別 Btrfs filesystem / loop image を作らない。論理隔離は共有 pool 内の Incus volume / subvolume が担当する。

## managed mount policy

既定の desired mount policy は次。

```text
compress=zstd:3,noatime,nodiscard
```

`compress=zstd:3` で透過圧縮を有効にし、`compress-force` は要求しない。`noatime` は read のたびの access-time metadata 更新と不要な COW churn を避ける。`nodiscard` は continuous discard を無効化し、space reclamation を明示的な batch operation として扱えるようにする。`autodefrag` も既定では有効にしない。automatic defragmentation は extent を書き換え、snapshot/clone 中心の rootfs pool で reflink/COW sharing を減らす可能性があるため。

mount option が主に効くのは新しく書かれる extent で、既存 data を一律に自動 rewrite して再圧縮しない。

## Runtime の pool 選択ルール

local composition は storage provider を lazy に設定する。Incus root storage を必要としない command を開いただけでは pool を作らない。

最初に Environment、Tooling Base builder、Seed builder、trusted host などが root storage を必要とした時点で `haco-local-default` を確認し、存在しなければ desired size と mount options を Incus へ渡して loop-backed Btrfs pool を作成させる。その後の Hacocoon-owned rootfs operation は Host の無関係な Incus default-profile pool ではなくこの pool を使う。

既存の `haco-local-default` がある場合は、再利用前に `btrfs.mount_options` を `compress=zstd:3,noatime,nodiscard` へ reconcile する。populated pool を破壊・再作成せず Incus pool config を更新し、lifecycle/remount ownership も Incus に残す。

runtime はこの Incus-owned pool shape だけを前提とし、別の Host-managed storage ownership path は持たない。

## Acceptance coverage

repository CI は CLI 移行中の temporary legacy runtime CLI（`cmd/haco`、release では `hacoq` として packaging）を ordinary user として real Incus へ接続する。Incus が loop-backed Btrfs pool を作ること、backing image が Linux file として sparse であること、configured desired state が `compress=zstd:3,noatime,nodiscard` であること、live filesystem が zstd 圧縮と `noatime` を持ち active な discard mode と autodefrag が無いことを確認する。また create/exec/delete/run lifecycle operation が同じ pool を再利用し、旧 compression-only policy を設定しても次の rootfs operation で desired policy へ reconcile されることを確認する。

`findmnt` は negative/default option の `nodiscard` token を省略する場合がある。そのため acceptance は Incus pool config に `nodiscard` が含まれることを要求し、live behavior では `discard` / `discard=async` が有効でないことを確認する。

これらは hosted environment 上の lifecycle と policy を検証するもの。physical compression ratio、COW 効率、Windows Host VHDX compaction 効果、すべての supported Host configuration まで証明するものではない。

## Workspace の境界

Host Workspace は Environment へ bind mount されるため、Hacocoon Btrfs pool 内に置く必要はない。この layout が対象にするのは Hacocoon-owned Incus rootfs / image-volume data であり、任意の user source tree ではない。

## 複数 pool

既定 local pool は `haco-local-default`。将来の明示 configured pool も、同じ single-owner lifecycle を維持したまま独自の Incus-managed storage identity を利用できる。
