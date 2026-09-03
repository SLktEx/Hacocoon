# Btrfs ストレージレイアウト

状態: **通常のlocal Incus経路ではloop-backed Btrfs lifecycleをIncusへ委譲する。従来のHacocoon-managed raw/helper実装はfocused compatibility test向けに残す。**

Milestone: **v0.20 Managed Btrfs Rootfs Storage** / **v0.21 Managed Btrfs Transparent Compression**。

## 既定のlocal layout

通常のlocal compositionはHacocoon自身のraw imageを作成・mountしない。代わりに、`source=` を指定せずIncusへ既定poolをlazyに作成させる。

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
- filesystem-level maintenanceをHostの無関係なdataではなくHacocoon rootfs dataへ限定できる。

隔離のためだけにEnvironmentやSeedごとへ別のBtrfs filesystem / loop imageを作らない。論理的な隔離は共有pool内のIncus volume / subvolumeが担当する。

## mount policy

通常のIncus-owned poolは次をdesired mount policyとする。

```text
compress=zstd:3,noatime,nodiscard
```

- `compress=zstd:3`: 透過圧縮を有効にする。`compress-force` は使わず、圧縮しにくいdataはBtrfsの通常heuristicsに任せる。
- `noatime`: read-heavyな開発workloadでaccess time更新によるmetadata writeと不要なCOW churnを減らす。access time semanticsへ明示依存するapplicationは既定Hacocoon rootfs policyの対象外で、必要なら将来別policyを用意する。
- `nodiscard`: 通常I/O pathからdiscard/reclamationを外し、trim/compactionは明示的なmaintenance flowで扱う。

`autodefrag` も既定では無効。automatic defragmentationはextentを書き換え、既存のreflink/COW sharingを減らす可能性があるため、Incus snapshot/clone中心のrootfs poolには良いdefault trade-offではない。`nodatacow`、`nodatasum`、custom `commit=`、手動のSSD heuristic固定も既定policyには入れない。

mount optionはpoolのmount stateへ作用し、compressionは主に新しく書かれるextentへ効く。既存dataを一律に自動rewriteして再圧縮しない。

## Runtime のpool選択とreconcile

local compositionはstorage providerをlazyに設定する。Incus rootfsを必要としないcommandのためにlocal applicationを開いただけでは既定poolを作らない。

最初にEnvironment、Tooling Base builder、Seed builder、trusted hostなどがroot storageを必要とした時点で、providerが `haco-local-default` の存在を確認する。なければHacocoonはIncusへdesired sizeとmount optionsを渡してloop-backed Btrfs poolを作成させる。その後はHacocoon所有rootfs operationで同じpoolを再利用し、HostのIncus default-profile poolへfallbackしない。

既存のIncus-owned `haco-local-default` poolがある場合は、Incusのstorage API/CLI経由で `btrfs.mount_options` を読み取る。desired valueと違えば `incus storage set` でreconcileし、その設定値を再確認する。通常pathではIncusの裏側でHacocoonが勝手にremountしたり、privileged storage helperを使ったりしない。

既存のpopulated legacy external-path poolを通常startupで破壊・再作成しない。古いownership modelのmigration/recoveryは別のfail-safe compatibility operationとして扱う。

## `metadata_ratio` policy

Hacocoonはcustom Btrfs metadata ratioを既定では設定しない。これは上記mount-option reconciliationではなくfilesystem creation policyの領域。将来non-default ratioを採用する場合はsnapshot/clone/COW-heavy workloadでfocused benchmarkを行い、space overheadを許容できる範囲でmetadata allocation failure riskが再現性を持って改善することを確認してから決める。

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
2. normal CLI jobは、その経路のためにstorage helperをinstallせず、actual ordinary-user `haco` をreal Incusへ接続する。Incusが `/var/lib/incus/disks/haco-local-default.img` を作ること、Linux fileとしてsparseであること、real loop deviceがpoolをbackingしていること、configured/live Btrfs policyにzstd compression・`noatime`・`nodiscard` があり `autodefrag` がないこと、legacyな `$HACO_ROOT/images/local-default.raw` / `$HACO_ROOT/mounts/local-default` が作られないこと、`haco create` / `exec` / `delete` / `run` が同じpoolを正しく再利用することを確認する。

これらはhosted environment上でlifecycleとpolicyを検証するもの。physical compression ratio、COW効率、Windows Host VHDX compaction効果、すべてのsupported Host configurationまで証明するものではない。

## Workspace の境界

Host WorkspaceはEnvironmentへbind mountされるため、Hacocoon Btrfs pool内に置く必要はない。このlayoutが対象にするのはHacocoon所有のIncus rootfs / image-volume dataであり、任意のuser source treeではない。

## 複数pool

ルールは「設定されたHacocoon storage poolごとに1個の共有Btrfs filesystem」であり、全deploymentに対するhard global singletonではない。既定local poolは `haco-local-default`。将来の明示configured poolもHost default poolへfallbackせず、それぞれのIncus-managed storage identityを使える。
