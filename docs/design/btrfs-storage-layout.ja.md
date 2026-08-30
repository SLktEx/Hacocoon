# Hacocoon 管理 Btrfs ストレージレイアウト

Hacocoon のローカルストレージは、設定されたストレージ pool ごとに 1 個の sparse raw backing image を持ち、その image を Btrfs でフォーマットする。Incus は、その Btrfs filesystem の mount point を対応する Hacocoon 管理 storage pool の source として利用する。

```text
HACO_ROOT/images/<storage-id>.raw   (sparse raw)
        |
        v
     loop device
        |
        v
  Btrfs filesystem
        |
        v
Incus pool: haco-<storage-id>
  |- cached Base image volumes
  |- Tooling Base builders
  |- Seed builders / cached Seed image volumes
  |- Environment rootfs volumes
  `- Incus snapshots / clones
```

既定のローカル storage ID は `local-default` なので、既定の Incus pool は `haco-local-default` になる。

## なぜ同じ filesystem を共有するのか

この storage boundary は意図的なもの。Base、Tooling、Seed、Environment の rootfs data を同じ Hacocoon 管理 Btrfs filesystem に置くことで、storage-level の最適化を lifecycle 全体へ適用できる。

- Btrfs の透過圧縮を、管理対象の rootfs data 全体へ適用できる。
- Incus の Btrfs snapshot / clone で copy-on-write の共有を維持できる。
- storage driver が共有できる範囲では、Seed 由来 Environment は変更された extent だけ追加消費すればよい。
- filesystem-level maintenance や optional な外部 deduplication を、Host の無関係な data に触れず Hacocoon filesystem だけへ適用できる。
- compaction によって未使用 extent を sparse raw backing file の hole に戻せる。

隔離のためだけに Environment や Seed ごとへ別の Btrfs filesystem / sparse image を作ってはいけない。論理的な隔離は、共有 storage pool 内の Incus volume / subvolume が担当する。

## Runtime の pool 選択ルール

local composition は先に Hacocoon Btrfs storage を作成・確認し、その storage attachment で Incus runtime を Prepare する。Environment、Tooling Base、Seed の作成は、Host の Incus default profile pool を継承せず、Prepare 済みの pool を利用する。

Hacocoon local composition を通さず低レベル Incus runtime を直接利用する経路については、互換性のため従来の default-profile 挙動を残す。この互換経路は Hacocoon の通常ローカル storage architecture ではない。

## Workspace の境界

Host Workspace は Environment へ bind mount されるため、Hacocoon Btrfs pool 内に置く必要はない。この storage layout が対象にするのは Hacocoon 所有の Incus rootfs / image-volume data であり、任意のユーザー source tree ではない。

## 複数 pool

ルールは「設定された Hacocoon storage pool ごとに 1 個の共有 Btrfs filesystem」であり、すべての Hacocoon deployment に対する hard global singleton ではない。Runtime Prepare は storage attachment の `incus_pool` を記録するため、別の storage ID は別の `haco-<storage-id>` pool へ対応でき、Host の default pool へ戻る必要はない。
