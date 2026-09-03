# Btrfs ストレージレイアウト

状態: **通常のlocal Incus経路ではloop-backed Btrfs lifecycleをIncusへ委譲する。従来のHacocoon-managed raw/helper実装はfocused compatibility test向けに残す。**

Milestone: **v0.20 Managed Btrfs Rootfs Storage** / **v0.21 Managed Btrfs Transparent Compression**。

## 既定のlocal layout

通常のlocal compositionはHacocoon自身のraw imageを作成・mountしない。代わりに、`source=` を指定せずIncusへ既定poolをlazyに作成させる。

```text
Incus pool: haco-local-default
  driver=btrfs
  size=128GiB
  btrfs.mount_options=compress=zstd:3
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
  |- Environment rootfs volumes
  `- Incus snapshots / clones
```

backing image作成、loop attach、Btrfs format、mount/unmount lifecycle、対応可能なloop-pool growはIncusが所有する。Hacocoonはpool identityとpolicyを決めるが、通常local pathではIncusが使うblock-device lifecycleを重複実装しない。

このownership boundaryはWSLで特に重要。`incusd` 起動後にHost側でmanaged mountを復元する設計ではIncusのstorage initializationとraceし、poolが一時的にunavailableになる可能性がある。Incus自身にbacking imageとmountを所有させることで、mount namespaceやservice起動順のworkaroundを増やすのではなくcross-owner dependency自体をなくす。

## sparse file と WSL sparse VHD は別物

Incusのloop-backed Btrfs poolはsparseな **Linux file** を使う。Incusはloop imageのlogical sizeだけを設定するsparse-file pathで作成し、128GiBを最初から全量physical allocateしない。repository acceptanceでも作成後にallocated bytesがlogical 128GiBより小さいことを確認する。

これはWSLの `sparseVhd` / sparse-VHDX modeとは別。Hacocoonはこのstorage designのためにWSL sparse-VHD modeを有効化しない。Windows Host側VHDXのspace reclamationは明示的なmaintenanceとして扱い、`haco maintenance compact` の別workで管理する。

## なぜrootfs objectを同じpoolで共有するのか

Base、Tooling、Seed、Environmentのrootfs dataを同じHacocoon Btrfs poolへ置き、storage-level optimizationをlifecycle全体へ適用する。

- Btrfsの透過圧縮をmanaged rootfs data全体へ適用できる。
- Incus Btrfs snapshot / cloneでcopy-on-write sharingを維持できる。
- storage driverが共有できる範囲では、Seed由来Environmentは変更されたextentだけ追加消費すればよい。
- filesystem-level maintenanceやoptionalな外部deduplicationを、Hostの無関係なdataではなくHacocoon rootfs dataへ限定できる。

隔離のためだけにEnvironmentやSeedごとへ別のBtrfs filesystem / loop imageを作らない。論理的な隔離は共有pool内のIncus volume / subvolumeが担当する。

## 圧縮・defragmentation policy

既定poolは `compress=zstd:3` を使う。`compress-force` は意図的に要求せず、圧縮しにくいdataはBtrfsの通常heuristicsに任せる。

`autodefrag` も既定では無効。automatic defragmentationはextentを書き換え、既存のreflink/COW sharingを減らす可能性があるため、Incus snapshot/clone中心のrootfs poolには良いdefault trade-offではない。将来使う場合もworkload-specificな明示判断にする。

compression mount optionが主に効くのは新しく書かれるextentであり、既存dataを一律に自動rewriteして再圧縮しない。

## Runtime のpool選択ルール

local compositionはstorage providerをlazyに設定する。Incus rootfsを必要としないcommandのためにlocal applicationを開いただけでは既定poolを作らない。

最初にEnvironment、Tooling Base builder、Seed builder、trusted hostなどがroot storageを必要とした時点で、providerが `haco-local-default` の存在を確認する。なければHacocoonはIncusへdesired sizeとmount optionsを渡してloop-backed Btrfs poolを作成させる。その後はHacocoon所有rootfs operationで同じpoolを再利用し、HostのIncus default-profile poolへfallbackしない。

既存の `haco-local-default` poolがある場合は再利用する。通常startupで既存のpopulated legacy poolを破壊・再作成しない。legacy pool contentsのmigrationは別のfail-safe operationとして扱う。

## legacy Hacocoon-managed storage path

repositoryには従来の `modules/storage/btrfs` と `haco-storage-helper` も残す。このpathは次をHacocoon側で管理する。

```text
HACO_ROOT/images/<storage-id>.raw
  -> loop device
  -> Hacocoon-managed Btrfs mount
  -> Incus pool source=<mountpoint>
```

storage-helper、block backend、shrink/compact、hardening、compatibilityのfocused testに引き続き使える。local compositionで明示的に `HACO_STORAGE_PRIVILEGE_MODE` または `HACO_BLOCK_BACKEND` を設定した場合だけこのcompatibility pathを選ぶ。通常installationはどちらも設定しないためIncus-owned poolを使う。

helperは引き続きfail-closedなtyped interfaceであり、任意のroot command executionを公開しない。専用acceptance coverageも通常CLI storage pathとは独立して残す。

## Acceptance coverage

repository CIはdisposableなUbuntu 26.04上で独立した2種類のacceptanceを行う。

1. storage-helper jobは、残しているHacocoon-managed raw/loop/Btrfs helper boundaryとhardening ruleを実機能で検証する。
2. normal CLI jobは、その経路のためにstorage helperをinstallせず、actual ordinary-user `haco` をreal Incusへ接続する。Incusが `/var/lib/incus/disks/haco-local-default.img` を作ること、Linux fileとしてsparseであること、real loop deviceがpoolをbackingしていること、live mountがzstd圧縮付きBtrfsかつautodefrag無しであること、legacyな `$HACO_ROOT/images/local-default.raw` / `$HACO_ROOT/mounts/local-default` が作られないこと、`haco create` / `exec` / `delete` / `run` が同じpoolを正しく再利用することを確認する。

これらはhosted environment上でlifecycleとpolicyを検証するもの。physical compression ratio、COW効率、Windows Host VHDX compaction効果、すべてのsupported Host configurationまで証明するものではない。

## Workspace の境界

Host WorkspaceはEnvironmentへbind mountされるため、Hacocoon Btrfs pool内に置く必要はない。このlayoutが対象にするのはHacocoon所有のIncus rootfs / image-volume dataであり、任意のuser source treeではない。

## 複数pool

ルールは「設定されたHacocoon storage poolごとに1個の共有Btrfs filesystem」であり、全deploymentに対するhard global singletonではない。既定local poolは `haco-local-default`。将来の明示configured poolもHost default poolへfallbackせず、それぞれのIncus-managed storage identityを使える。
