# Hacocoon 管理 Btrfs ストレージレイアウト

状態: **repository実装あり。GitHub-hosted上のnormal-user helperとreal CLI acceptanceを自動化済み。physical compression/COW/compaction acceptanceはhost-dependentです。**

Milestone: **v0.20 Managed Btrfs Rootfs Storage** / **v0.21 Managed Btrfs Transparent Compression**。

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

storage ID と managed `.raw` backing path が永続的な identity であり、具体的な `/dev/loopN` 名は runtime 中だけ有効な一時 attachment として扱う。detach/reattach や Host 再起動後には番号が変わり得るうえ、同じ `/dev/loopN` が別の backing file に再利用されることもある。そのため、破壊的な loop operation の前には cached device number を正しいものと仮定せず、managed backing image から現在の loop を再発見する。

## なぜ同じ filesystem を共有するのか

この storage boundary は意図的なもの。Base、Tooling、Seed、Environment の rootfs data を同じ Hacocoon 管理 Btrfs filesystem に置くことで、storage-level の最適化を lifecycle 全体へ適用できる。

- Btrfs の透過圧縮を、管理対象の rootfs data 全体へ適用できる。
- Incus の Btrfs snapshot / clone で copy-on-write の共有を維持できる。
- storage driver が共有できる範囲では、Seed 由来 Environment は変更された extent だけ追加消費すればよい。
- filesystem-level maintenance や optional な外部 deduplication を、Host の無関係な data に触れず Hacocoon filesystem だけへ適用できる。
- compaction によって未使用 extent を sparse raw backing file の hole に戻せる。

隔離のためだけに Environment や Seed ごとへ別の Btrfs filesystem / sparse image を作ってはいけない。論理的な隔離は、共有 storage pool 内の Incus volume / subvolume が担当する。

## 圧縮ポリシー

Managed Btrfs filesystem は標準で `compress=zstd:3` を使う。`compress-force` は意図的に使わず、圧縮しにくいdataはBtrfsの通常heuristicsに任せてuncompressedのまま扱えるようにする。

既にmount済みのHacocoon-managed filesystemが期待するcompression optionを持たない場合は、managed filesystemを `compress=zstd:3` でremountする。`compress-force` が付いているmountはdesired stateを満たしたものとして扱わない。

compression mount optionが効くのは新しく書かれるextentです。既存dataを自動defrag/recompressするとreflink/COW sharingを減らす可能性があるため、Hacocoonは自動再圧縮しない。physical compression ratio、CPU cost、supported-host behaviorはrepository testだけで証明したことにしない。

## Runtime の pool 選択ルール

local composition は Hacocoon Btrfs storage provider を lazy に設定する。Incus rootfs を必要としないコマンドのために local application を開いただけでは、loop image の attach、Btrfs mount、Incus storage pool 作成を行わない。

最初に Environment、Tooling Base builder、または Seed builder が root storage を必要とした時点で、Incus runtime が設定済み provider を解決し、sparse-raw Btrfs storage と対応する `haco-<storage-id>` Incus pool を作成・確認する。その pool を記録し、以降の Hacocoon 所有 rootfs operation でも再利用する。したがって、これらの経路は Host の Incus default profile pool を継承しない。

Hacocoon local composition を通さず低レベル Incus runtime を直接利用する経路については、互換性のため従来の default-profile 挙動を残す。この互換経路は Hacocoon の通常ローカル storage architecture ではない。

## Host 特権境界

通常の `haco` process は非rootのまま動かす。sparse backing file の作成・size変更、state/lock file、その他 Host 特権を必要としない処理はordinary-user process側に残し、Host権限が必要な固定storage operationだけを専用の `haco-storage-helper` へ委譲する。

release installerはhelperを通常PATH外の `/usr/local/libexec/hacocoon/haco-storage-helper` へ置き、root所有・group/other非writableにする。委譲前にHacocoonはhelper本体について、root所有・実行可能なregular non-symlink fileで、root所有かつ書き換え不能なparent directory配下であることを要求する。`/usr/bin/sudo` のような固定OS toolはdistribution上symlinkである場合があるためcanonical targetへ解決し、そのtargetとparent chainがroot所有・非writableであることを検証する。Hacocoonは**passwordless sudo ruleをinstallしない**。sudoがpromptするか、既存credential cacheを使うか、拒否するかはHost/operator policyであり、CLI全体をrootへ昇格させる設計ではない。

helperは任意のexecutable/argv forwarding APIではなくtyped operationだけを公開する。権限範囲はHacocoon-managed storage objectと固定command shapeに限定し、loop discovery/attach/detach/rescan、filesystem type probe、Btrfs format、mount/remount/unmount、usage/resize/minimum-size/balance、trimのみを扱う。任意shell execution、任意mount option、任意block device format、任意loop device、任意Host path、任意Btrfs subcommandは提供しない。

すべてのprivileged requestはcaller-side validationを信用せずroot helper内でも再検証する。特に次を保証する。

- `HACO_ROOT`、`images`、`mounts` はcanonicalなreal directoryで、ordinary-user ownershipが必要な箇所はinvoking UID所有かつgroup/other非writableであること。
- backing imageは正確に `<HACO_ROOT>/images/<storage-id>.raw` のregular fileで、invoking UID所有、non-symlink、group/other非writable、hard link数が1であること。
- loop deviceは `/dev/loopN` だけを許可し、期待するmanaged `BACK-FILE` に加えて、現在のmanaged raw file inodeと同じ `BACK-INO` を報告すること。
- 新規attachしたloopは直後に再検証し、path/inode identityが一致しなければ即detachすること。
- `mkfs.btrfs` はhelper自身の `blkid` が明示的なno-signature stateを返した場合だけ許可し、format直前にもloop identityを再検証すること。
- mountpointは正確に `<HACO_ROOT>/mounts/<storage-id>` に制限し、loopとmountpointのstorage identity一致を要求すること。新規mount後にもsourceを再検証し、postconditionが崩れていれば即unmountすること。
- mount optionは `compress=zstd:3`、balance filterは固定のtargeted filter、resize targetは検証済みの正整数または `max` だけを許可すること。

storage lifecycleのserializationは引き続きordinary storage layerのper-storage leaseが担当する。helper側validationはdirect invocation、stale state、partial failure、confused-deputyに対する独立したdefense-in-depthです。cleanupもfail closedで、loop detachに失敗した場合はbacking imageを削除せず、mount/loop identityが曖昧なら破壊的targetを推測せず拒否する。

`HACO_STORAGE_PRIVILEGE_MODE=direct` はfake/test/development environment専用で、callerが元々持っている権限のままcommandを直接実行するだけです。権限を付与する仕組みではなく、通常のmanaged Btrfs operationで使うprivileged shortcutではない。

repository CIはdisposableなGitHub-hosted Ubuntu 26.04上で二段のacceptanceを順番に実行する。第一段ではGo test processをordinary runner userのまま実行し、installed root-owned helperを経由してreal sparse image作成、loop attach、Btrfs format、`compress=zstd:3` mount、inspect、idempotent reuse、unmount、loop detach、backing image deleteまでを確認する。第二段ではfresh runnerで同じhelper境界とreal Incusを組み合わせ、actual ordinary-user `haco` binaryを実行してlazyな `haco-local-default` pool作成、`haco create` / `exec` / `delete`、`haco run` によるmanaged pool再利用、ephemeral cleanup、pool/mount/loopのexact cleanupまで確認する。これらはそのhosted environment上で通常local CLI compositionが機能することを示すが、physical compression ratio、COW効率、compaction効果、すべてのsupported Host configurationまで証明するものではない。

## Workspace の境界

Host Workspace は Environment へ bind mount されるため、Hacocoon Btrfs pool 内に置く必要はない。この storage layout が対象にするのは Hacocoon 所有の Incus rootfs / image-volume data であり、任意のユーザー source tree ではない。

## 複数 pool

ルールは「設定された Hacocoon storage pool ごとに 1 個の共有 Btrfs filesystem」であり、すべての Hacocoon deployment に対する hard global singleton ではない。Runtime Prepare または設定済み storage provider が storage attachment の `incus_pool` を選択するため、別の storage ID は別の `haco-<storage-id>` pool へ対応でき、Host の default pool へ戻る必要はない。
