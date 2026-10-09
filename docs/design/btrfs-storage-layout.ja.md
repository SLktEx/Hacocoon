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
  |- Base builder Environments
  |- trusted haco-host rootfs
  |- Environment rootfs volumes
  `- Incus snapshots / clones
```

backing イメージ作成、loop attach、Btrfs 形式、mount/unmount ライフサイクル、対応可能な loop-pool grow は Incus が所有する。Hacocoon が所有するのは desired なプール識別と方針だけで、別の Host-managed block/mount ライフサイクルは持たない。

## sparse file と WSL sparse VHD は別物

Incusのloop-backed Btrfsプールはスパースな **Linuxファイル** を使い、論理サイズ128GiBを最初から全量確保しない。これはWSLの`sparseVhd`/sparse-VHDXモードとは別で、Hacocoonはこのstorage設計のためにWSL sparse-VHDモードを有効化しない。Windows Host側VHDXの回収は、明示的な[容量回収操作](storage-reclamation.ja.md)として扱う。

## なぜ rootfs object を同じ pool で共有するのか

Base image、ビルド用Environment、信頼されたHost、通常のEnvironmentのrootfsデータを同じHacocoon Btrfsプールへ置き、IncusのBtrfs storage driverによる処理をライフサイクル全体へ適用する。

- 圧縮しやすいデータは Btrfs transparent compression で physical バイト列を減らせる。
- Incus Btrfs スナップショット / clone で copy-on-write sharing を維持できる。
- ストレージ maintenance を任意の Host データではなく Hacocoon rootfs データへ限定できる。

隔離のためだけにEnvironmentごとへ別Btrfsファイルシステムやloop imageを作らない。論理隔離は共有プール内のIncus volume/subvolumeが担当する。

## managed mount policy

既定の desired マウント方針は次。

```text
compress=zstd:3,noatime,nodiscard
```

`compress=zstd:3` で透過圧縮を有効にし、`compress-force` は要求しない。`noatime` は読み取りのたびの access-time メタデータ更新と不要な COW churn を避ける。`nodiscard` は continuous discard を無効化し、space reclamation を明示的な batch operation として扱えるようにする。`autodefrag` も既定では有効にしない。自動 defragmentation は extent を書き換え、snapshot/clone 中心の rootfs プールで reflink/COW sharing を減らす可能性があるため。

マウントオプションが主に効くのは新しく書かれる extent で、既存データを一律に自動 rewrite して再圧縮しない。

## Runtime の pool 選択ルール

local 構成はストレージプロバイダーを lazy に設定する。Incus root ストレージを必要としないコマンドを開いただけではプールを作らない。

最初にEnvironment、Baseビルダー、信頼されたHostなどがroot storageを必要とした時点で`haco-local-default`を確認し、存在しなければ指定のサイズとmount optionをIncusへ渡してloop-backed Btrfsプールを作成させる。その後のHacocoon所有rootfsの操作は、Hostの無関係なIncus default-profileプールではなくこのプールを使う。

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

リポジトリの CI は 配布する `haco` と `haco-host` のcontroller clientを通常の利用者として実際の Incus へ接続する。Incus が loop-backed Btrfs プールを作ること、backing イメージが Linux ファイルとしてスパースであること、configured desired 状態が `compress=zstd:3,noatime,nodiscard` であること、稼働中のファイルシステムが zstd 圧縮と `noatime` を持ち稼働中な discard モードと autodefrag が無いことを確認する。また create/exec/delete/run ライフサイクル operation が同じプールを再利用し、旧 compression-only 方針を設定しても次の rootfs operation で desired 方針へ照合・調整されることを確認する。

`findmnt` は negative/default オプションの `nodiscard` token を省略する場合がある。そのため検証は Incus プール設定に `nodiscard` が含まれることを要求し、live behavior では `discard` / `discard=async` が有効でないことを確認する。

これらは hosted environment 上のライフサイクルと方針を検証するもの。physical compression ratio、COW 効率、Windows Host VHDX compaction 効果、すべての対応している Host 設定まで証明するものではない。

## Host OCI コピーの容量計測

`TestRealIncusHostToolingE2E` は既存の標準 Host 試験に容量計測を加えます。
BusyBox を取得し、BuildKit で実際のイメージをビルド・実行してから、正規の
リソースサービスで Host の OCI 領域をコピーし、ネットワークのない独立した
受け手でイメージを再利用します。従来の合成キャッシュ・Store データ試験を
実イメージの受入結果として扱うものではありません。

コピー前の Host 領域、ネイティブコピー後の両領域、保持したコンテナーの
書き込み層へ 8 MiB の乱数を書き込む前後の両領域を計測します。そのコンテナーと
イメージのタグを一つ削除した後にも計測し、Host のイメージと受け手の別タグは
残します。ネイティブクローンの親子関係に加え、コピー先のイメージ内容・BuildKit
ディレクトリに共有 extent があることを要求します。書き込み後は割当量と排他的な
extent が増え、Host のイメージは内容不変かつコピー先の書き込みを含まずに
実行できる必要があります。

`storage_measurement` の各記録は操作と対象領域を識別し、Store 全体、containerd の
コンテンツ blob、BuildKit のディレクトリを分けて報告します。

| 観測項目 | コマンドと意味 |
|---|---|
| `logical_bytes` | `du --summarize --apparent-size --block-size=1` による見かけの容量 |
| `allocated_bytes` | `du --summarize --block-size=1` による参照ブロックの割当量。コピー間の CoW 重複参照を含む |
| `extent_total_bytes`、`extent_exclusive_bytes`、`extent_set_shared_bytes` | `btrfs filesystem du --raw --summarize` の FIEMAP extent 集計。set-shared は各引数内で重なる共有 extent を一度だけ数える |
| `pool_logical_bytes`、`pool_allocated_bytes` | この試験専用の Incus 所有スパースイメージのファイル長と `du --block-size=1` による割当量 |

コピーごとの割当量や共有 extent を足して、重複を除いた物理使用量と見なしては
いけません。FIEMAP の extent 長は圧縮後のバイト数ではありません。プールの観測には
rootfs・メタデータ・ランタイムの活動も含まれ、差分は割当量の変化であり、デバイスの
書き込みカウンターや export/import の書き込み増幅ではありません。他の参照が
残る場合を含め、イメージやタグの消失だけでは実容量の回収を証明できません。
[Btrfs コマンドの契約](https://btrfs.readthedocs.io/en/latest/btrfs-filesystem.html#subcommand)と[検証した出力形式](https://github.com/kdave/btrfs-progs/blob/v6.17/cmds/filesystem-du.c#L496-L508)も参照してください。

観測前に正確なボリューム所有者と利用元を確認し、試験専用インスタンスだけを Incus
経由で一時停止します。ファイルシステムを同期してカウンターを読んだ後、同じ
インスタンスを再開します。書き込みはゲスト・ランタイム操作で行い、観測処理は
backing subvolume・loop デバイス・mount を直接変更しません。失敗時には表示した
project・catalog を残し、一時停止中の利用元も調査用に残る場合があります。
既存環境を取り込んだり、無関係なプールを削除したりしません。

標準 Host ツールの準備と通信が可能な、専用の root Linux/WSL Incus/Btrfs 環境で、
リポジトリのルートから実行します。既存の Incus ワークフローも同じ試験を実行し、
別のベンチマーク用ワークフローは追加しません。

```bash
git rev-parse HEAD
git diff --exit-code
go test -c -o /tmp/haco-host-storage.test ./internal/adapters/incus
sudo env HACO_E2E_HOST_TOOLING=1 /tmp/haco-host-storage.test \
  -test.run='^TestRealIncusHostToolingE2E$' -test.v -test.timeout=25m
```

実際に試験したコミットと全試験・CI 記録を保持してください。試験は追加で、
OS/kernel、Go、Incus、Btrfs-progs、
nerdctl/containerd/BuildKit の版、native snapshotter、ビルド済みイメージの識別子を
報告します。記録した変更のない checkout からビルドしてください。現在のリビジョン
だけでは、古いバイナリーの由来は証明できません。実機の測定値を[受入記録](../status/acceptance-evidence.ja.md#storage)に
残すまでは、計測による受入を完了したと主張しません。Base と複数 Env、管理
Workspace、snapshot restore、archive/publish の増幅、Docker の各 driver、最後の
参照を除いた後の回収、大規模負荷の計測は
[issue #241](https://github.com/SLktEx/Hacocoon/issues/241)に残ります。Btrfs の観測を
他のファイルシステムや Windows VHDX の割当量へ一般化しません。

## 管理 Workspace のライフサイクル計測

既存の必須試験 `TestRealIncusSnapshotAggregateE2E` で、管理リポジトリの
ボリューム一つを対象に、稼働中の Snapshot 作成、公開コマンド
`open --new --snapshot`、停止後の `env copy`、独立した書き込みと削除を計測します。
これは範囲を限定した合成 Workspace 負荷です。実際の OCI 負荷は上記の Host 試験で
扱い、実機での受入結果と実測値は[受入記録](../status/acceptance-evidence.ja.md#storage)に残します。

計測区間は、従来の集約試験による合成 Git/rootfs/OCI データ準備の後から始まります。
新しいデータの書き込みと unlink は所有権を確認したゲスト内だけで行い、Snapshot、
コピー、リソース削除には既存の公開ライフサイクルを使います。8 MiB の乱数ファイルを
保存し、Snapshot を残したまま元の Workspace から削除して、復元・コピーします。
最後のコピーにだけ別の 8 MiB ファイルを書き込みます。保存した Snapshot と復元元を
削除して残るコピーを計測した後、そこから両ファイルを削除します。既存の Git・内容・
所有権・後片付けの検査は維持します。

各段階で、選んだ Workspace ボリューム全体と固定の計測ディレクトリを分け、上記の
`du`・FIEMAP 項目を報告します。上限を設けた SHA-256 計算で内容を結び付けますが、
同じハッシュだけで共有を証明しません。source/saved/restored/copy の各ラベルに、
正確な project/pool/volume/owner の組から得た安定したハッシュを付けます。元の所有権
記録は永続的な試験用 catalog に残ります。保存・復元・コピーには実際の共有 extent を
要求し、独立した書き込みには参照割当量と排他的 extent の増加、保存元と元データの
不変を要求します。

観測は正規の Environment、Workspace の順でライフサイクルをロックし、ボリュームの
所有権と単独の接続先を確認します。稼働中の正確な試験用インスタンスだけを Incus で
一時停止し、停止済みのものは起動しません。物理領域の読み取りと再開の前に、識別・
世代・接続先・一時停止状態を再確認します。不明な状態や観測失敗では推測で後片付けせず、
一時停止したインスタンスを含めて調査用に残します。観測一回は二分、各コマンドは一分を
上限とし、固定ファイルはリンク数一の通常ファイルかつ正確に 8 MiB である必要があります。
backing subvolume、loop デバイス、mount を直接変更しません。

プールの値には `pool_scope=whole_shared_pool` と記録します。同じ管理プールには rootfs、
Image、メタデータ、他の試験や Host の活動もあるため、差分を Workspace 操作だけの
割当量とは見なしません。各観測前にファイルシステムを一度同期しますが、
[Btrfs の仕様](https://btrfs.readthedocs.io/en/latest/btrfs-filesystem.html#subcommand)では、
これは削除済み subvolume の清掃を開始するだけで完了を待ちません。ネイティブリソースの
不在が証明するのは論理的な削除だけです。残るコピーの排他的・共有 extent の変化と、
最後の有効なファイル参照を除いた後の backing 割当量は観測値として記録します。
待ち時間の挿入や成功までの再試行、容量減少の必須条件は設けません。extent の回収完了、
discard、Windows 側の回収を証明するものではありません。

通常の管理 Btrfs プールとキャッシュ済みコンテナーイメージがある専用の Linux/WSL
Incus Host で、記録した変更のないコミットからビルドし、検証済みのプール名と完全な
イメージ fingerprint を渡します。

```bash
git rev-parse HEAD
git diff --exit-code
go build -o /tmp/haco-snapshot-cli ./cmd/haco
go test -c -o /tmp/haco-snapshot-storage.test ./internal/adapters/incus
sudo env HACO_E2E_SNAPSHOT_AGGREGATE=1 \
  HACO_E2E_SNAPSHOT_CLI=/tmp/haco-snapshot-cli \
  HACO_E2E_INCUS_RESUME_POOL=<managed-pool> \
  HACO_E2E_INCUS_RESUME_IMAGE=<full-cached-image-fingerprint> \
  /tmp/haco-snapshot-storage.test \
  -test.run='^TestRealIncusSnapshotAggregateE2E$' -test.v -test.timeout=55m
```

既存の `incus-snapshots` ジョブがこの試験を必須として CLI を渡すため、新しいワークフローや
計測用の有効化フラグは追加しません。リソース作成前に障害復旧用 catalog を表示し、
成功時は正確に所有するリソースだけを削除します。全記録とビルドしたコミットを保持してください。
Base と複数 Env の計測、archive/publish の増幅、物理的な回収の完了、大規模負荷は
[issue #241](https://github.com/SLktEx/Hacocoon/issues/241)に残ります。

## Workspace の境界

通常の管理 Workspace は、管理プール内の独立した Incus カスタムボリュームと正規の利用権を使います。保持している外部パス方式は、利用者が選んだディレクトリを接続するもので、プール内に置く必要はありません。ルートファイルシステムの構成に合わせるためだけに、任意のソースコードを管理ストレージへ移動しません。

## 複数 pool

既定 local プールは `haco-local-default`。将来の明示 configured プールも、同じ single-owner ライフサイクルを維持したまま独自の Incus-managed ストレージ識別を利用できる。
