# Btrfs ストレージレイアウト

状態: **supportedなlocal storage pathはIncus-owned loop-backed Btrfsのみ。**

Milestone: **v0.20 Managed Btrfs Rootfs Storage** / **v0.21 Managed Btrfs Transparent Compression** / **v0.25 Incus-owned Btrfs Storage Acceptance**。

## 既定のlocal layout

local compositionは `source=` を指定せず、Incusへ既定poolをlazyに作成させる。

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

backing image作成、loop attach、Btrfs format、mount/unmount lifecycle、対応可能なloop-pool growはIncusが所有する。Hacocoonが所有するのはdesiredなpool identityとpolicyだけ。

Hacocoon側に別のHost-managed block/mount lifecycleは持たない。installとruntimeはこの1つのstorage shapeだけを対象にする。

所有権と却下した代案は [ADR 0003](../adr/0003-incus-owned-storage.md) に記録する。installerは対象外の旧layoutを移行・削除しない。

## sparse file と WSL sparse VHD は別物

Incusのloop-backed Btrfs poolはsparseな **Linux file** を使う。logical 128GiBを最初から全量physical allocateしない。

これはWSLの `sparseVhd` / sparse-VHDX modeとは別。Hacocoonはこのstorage designのためにWSL sparse-VHD modeを有効化しない。Windows Host側VHDXのspace reclamationはHacocoon CLIの責務にせず、必要な場合はWindows/WSL側の運用として扱う。

## なぜrootfs objectを同じpoolで共有するのか

Base、Tooling、Seed、trusted host、Environmentのrootfs dataを同じHacocoon Btrfs poolへ置き、IncusのBtrfs storage-driver behaviorをlifecycle全体へ適用する。

- 圧縮しやすいdataではBtrfs transparent compressionでphysical bytesを減らせる。
- Incus Btrfs snapshot / cloneでcopy-on-write sharingを維持できる。
- storage driverが共有できる範囲ではSeed由来Environmentがunchanged extentを共有できる。
- storage maintenanceを任意のHost dataではなくHacocoon rootfs dataへ限定できる。

隔離のためだけにEnvironmentやSeedごとへ別Btrfs filesystem / loop imageを作らない。論理隔離は共有pool内のIncus volume / subvolumeが担当する。

## 圧縮・defragmentation policy

既定poolは `compress=zstd:3,noatime,nodiscard` を要求する。`compress-force` はdesired stateにせず、圧縮しにくいdataはBtrfsの通常heuristicsへ任せる。

`noatime` はaccess-time更新によるmetadata writeとCOWを避ける。atimeに依存するapplicationには将来別policyが必要となる。`nodiscard` は通常I/Oからdiscardを外す。batch trimとWindows VHDX回収は明示的maintenanceとし、installerでは自動実行しない。

`autodefrag` も既定では無効。automatic defragmentationはextentを書き換えてreflink/COW sharingを減らす可能性があり、snapshot/clone中心のrootfs poolには良いdefaultではない。

compression mount optionが主に効くのは新しく書かれるextentであり、既存dataを一律にrewriteして再圧縮しない。

## Runtime のpool選択ルール

local compositionはstorage providerをlazyに設定する。Incus root storageを必要としないcommandを開いただけではpoolを作らない。

最初にEnvironment、Tooling Base builder、Seed builder、trusted hostなどがroot storageを必要とした時点で `haco-local-default` を確認し、存在しなければdesired sizeとmount optionsをIncusへ渡してloop-backed Btrfs poolを作成させる。その後のHacocoon-owned rootfs operationはHostの無関係なIncus default-profile poolではなくこのpoolを使う。

runtimeは現在のIncus-owned pool shapeだけを前提とし、別のstorage ownership pathは持たない。runtime attachmentは `incus_pool` のみとし、外部 `driver`/`source` はIncusへアクセスする前に拒否する。指定poolが利用不可なら、Host pathから代替poolを作らず失敗させる。

既存poolではIncusから `btrfs.mount_options` を取得し、完全一致なら書き込まない。古い値は `incus storage set` で変更して読戻す。read・writeの失敗や読戻し不一致ではrootfs operationを失敗させる。HacocoonはIncusの外からremountしない。設定読戻しはdesired configurationの証拠であり、kernelのlive mount反映は別の実機受入で確認する。Host側remountで受入を補わない。

## Acceptance coverage

既存Incus workflowの `incus-owned-btrfs` jobは一般ユーザーの一時的な `hacoq` 実装をreal Incusへ接続する。sparse backing、live zstd/noatime、discard・compress-force・autodefragの不在、古いpolicyの修正と既存rootfs/Workspace data保持を確認する。これは基盤検証であり、新製品 `haco` journeyの証拠ではない。unit testは一致時の再利用、古い値の更新、read/write/readback失敗を検証する。

これらはhosted environment上のlifecycleとpolicyを検証するもの。physical compression ratio、COW効率、Windows Host VHDX compaction効果、すべてのsupported Host configurationまで証明するものではない。

## Workspace の境界

Host WorkspaceはEnvironmentへbind mountされるため、Hacocoon Btrfs pool内に置く必要はない。このlayoutが対象にするのはHacocoon-owned Incus rootfs / image-volume dataであり、任意のuser source treeではない。

専用 `.git` を持つ独立COW repo cloneと、Environment削除・Base変更後も残るWorkspace volumeは **planned**。Seed実装はまだこのpoolを使う。Base依存を分離してから別変更で撤去する方針は [Seed設計](oci-seed-and-cow.ja.md) を参照する。

## 複数pool

既定local poolは `haco-local-default`。将来の明示configured poolも、同じsingle-owner lifecycleを維持したまま独自のIncus-managed storage identityを利用できる。
