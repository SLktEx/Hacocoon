# Btrfs ストレージレイアウト

状態: **対応しているな local ストレージパスは Incus-owned loop-backed Btrfs のみ。**

Milestone: **v0.20 Managed Btrfs Rootfs Storage** / **v0.21 Managed Btrfs Transparent Compression** / **v0.25 Incus-owned Btrfs Storage Acceptance**。

## 既定の local layout

実行基盤が受け入れるattachmentはlocal `incus_pool` 識別だけです。削除した `driver`/`source` は既存プールがあっても拒否し、検査失敗時に代替プールを作りません。マウント方針のread/readback失敗も安全側で拒否です。実際の Incus ストレージ CIには方針照合中の既存rootfs・Workspace sentinel データ保持も含めます。

local 構成は `source=` を指定せず、Incus へ既定プールを lazy に作成させる。

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

backing イメージ作成、loop attach、Btrfs 形式、mount/unmount ライフサイクル、対応可能な loop-pool grow は Incus が所有する。Hacocoon が所有するのは desired なプール識別と方針だけで、別の Host-managed block/mount ライフサイクルは持たない。

## sparse file と WSL sparse VHD は別物

Incus の loop-backed Btrfs プールはスパースな **Linux ファイル** を使い、logical 128GiB を最初から全量 physical allocate しない。これは WSL の `sparseVhd` / sparse-VHDX モードとは別で、Hacocoon はこのストレージ design のために WSL sparse-VHD モードを有効化しない。Windows Host 側 VHDX の reclamation は明示的な Host/operator 運用として扱う。

## なぜ rootfs object を同じ pool で共有するのか

Base、Tooling、Seed、信頼された Host、Environment の rootfs データを同じ Hacocoon Btrfs プールへ置き、Incus の Btrfs storage-driver behavior をライフサイクル全体へ適用する。

- 圧縮しやすいデータは Btrfs transparent compression で physical バイト列を減らせる。
- Incus Btrfs スナップショット / clone で copy-on-write sharing を維持できる。
- ストレージ driver が共有できる範囲では Seed 由来 Environment が unchanged extent を共有できる。
- ストレージ maintenance を任意の Host データではなく Hacocoon rootfs データへ限定できる。

隔離のためだけに Environment や Seed ごとへ別 Btrfs ファイルシステム / loop イメージを作らない。論理隔離は共有プール内の Incus ボリューム / subvolume が担当する。

## managed mount policy

既定の desired マウント方針は次。

```text
compress=zstd:3,noatime,nodiscard
```

`compress=zstd:3` で透過圧縮を有効にし、`compress-force` は要求しない。`noatime` は読み取りのたびの access-time メタデータ更新と不要な COW churn を避ける。`nodiscard` は continuous discard を無効化し、space reclamation を明示的な batch operation として扱えるようにする。`autodefrag` も既定では有効にしない。自動 defragmentation は extent を書き換え、snapshot/clone 中心の rootfs プールで reflink/COW sharing を減らす可能性があるため。

マウントオプションが主に効くのは新しく書かれる extent で、既存データを一律に自動 rewrite して再圧縮しない。

## Runtime の pool 選択ルール

local 構成はストレージプロバイダーを lazy に設定する。Incus root ストレージを必要としないコマンドを開いただけではプールを作らない。

最初に Environment、Tooling Base ビルダー、Seed ビルダー、信頼された Host などが root ストレージを必要とした時点で `haco-local-default` を確認し、存在しなければ desired size とマウントオプションを Incus へ渡して loop-backed Btrfs プールを作成させる。その後の Hacocoon-owned rootfs operation は Host の無関係な Incus default-profile プールではなくこのプールを使う。

既存の `haco-local-default` がある場合は、再利用前に `btrfs.mount_options` を `compress=zstd:3,noatime,nodiscard` へ照合・調整する。populated プールを破壊・再作成せず Incus プール設定を更新し、lifecycle/remount 所有権も Incus に残す。

実行基盤はこの Incus-owned プール shape だけを前提とし、別の Host-managed ストレージ所有権パスは持たない。

## 読み取り専用のmount診断

`haco doctor` はIncus設定の検査（`storage`）と稼働中ファイルシステムの検査（`storage_mount`）を区別する。設定の一致だけではlive マウントへの反映を証明できない。

| 設定方針 | live観測 | 結果 |
|---|---|---|
| 一致 | 検証済みBtrfs root マウントに方針適用済み | `storage_mount: ok` |
| 一致 | 検証済みBtrfs root マウントのオプションが異なる | `storage_mount: pending`、終了1 |
| 一致 | identity/mountが欠落・不正・曖昧、または観測中に変化 | `storage_mount: failed`、終了1 |
| 不明または不一致 | live検査をskip | 設定を解決してからlive 方針を判断する |

Incus アダプターはlocal daemonのプール元データを読み、Incus所有の `disks/<pool>.img` layoutを検証する。そのdaemonのプール mountpointを導出し、読み取り専用の `stat`・`losetup --list --associated`・`findmnt --kernel` を使う。backing objectは通常のファイルであり、device/inodeが単一の全イメージ loop関連付けと一致する必要がある。マウントはそのloop デバイスを使うBtrfs ファイルシステムのrootに限定する。backing 識別と関連付けを再照会し、収集中に観測した変化を検出する。別WSL distributionでfilenameが同じだけでは同一としない。

live一致には書き込み可能なBtrfs、`noatime`、`compress=zstd:3` を要求し、稼働中 discard・autodefrag・forced compressionは拒否する。否定形の `nodiscard` はkernel出力になくてもよい。reportは選択した結果だけを返し、生のパス・マウントオプション・subprocess出力を公開しない。項目欠落、関連付け/mountの重複、出力切断、キャンセル、不明な観測を `ok` / `pending` にしない。

`pending` はdesired設定が記録されているがlive反映を確認できていない状態を表す。maintenanceを予約せず、再起動の安全性も保証しない。作業を保持し、maintenance時にIncus所有のプール remountを行ってから再診断する。診断の修復としてHacocoonがattach・detach・remount・形式を実行することはない。この時点の観測は所有権leaseや後続mutationの前提条件ではない。

common インストーラーは完了表示の前に同じ製品doctorを実行する。live マウントがpending/failedなら診断の次の操作を示してinstallを停止し、再実行でも検査を迂回しない。

## Acceptance coverage

リポジトリの CI は CLI 移行中の temporary 旧実装実行基盤 CLI（`cmd/haco`、release では `hacoq` として packaging）を通常の利用者として実際の Incus へ接続する。Incus が loop-backed Btrfs プールを作ること、backing イメージが Linux ファイルとしてスパースであること、configured desired 状態が `compress=zstd:3,noatime,nodiscard` であること、稼働中のファイルシステムが zstd 圧縮と `noatime` を持ち稼働中な discard モードと autodefrag が無いことを確認する。また create/exec/delete/run ライフサイクル operation が同じプールを再利用し、旧 compression-only 方針を設定しても次の rootfs operation で desired 方針へ照合・調整されることを確認する。

`findmnt` は negative/default オプションの `nodiscard` token を省略する場合がある。そのため検証は Incus プール設定に `nodiscard` が含まれることを要求し、live behavior では `discard` / `discard=async` が有効でないことを確認する。

これらは hosted environment 上のライフサイクルと方針を検証するもの。physical compression ratio、COW 効率、Windows Host VHDX compaction 効果、すべての対応している Host 設定まで証明するものではない。

## Workspace の境界

通常の管理 Workspace は、管理プール内の独立した Incus カスタムボリュームと正規の利用権を使います。保持している外部パス方式は、利用者が選んだディレクトリを接続するもので、プール内に置く必要はありません。ルートファイルシステムの構成に合わせるためだけに、任意のソースコードを管理ストレージへ移動しません。

## 複数 pool

既定 local プールは `haco-local-default`。将来の明示 configured プールも、同じ single-owner ライフサイクルを維持したまま独自の Incus-managed ストレージ識別を利用できる。
