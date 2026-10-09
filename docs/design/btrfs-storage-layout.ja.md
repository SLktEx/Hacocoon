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
残すまでは、計測による受入を完了したと主張しません。管理 Workspace と rootfs/Image は
以下の節で、保持する archive の計測とともに
[issue #241](https://github.com/SLktEx/Hacocoon/issues/241)の各範囲として定義します。
一般的な Base ビルド負荷、Docker driver、物理回収の完了、大規模負荷は未検証の限界であり、
代表的な計測を求めるこの issue の追加完了条件ではありません。Btrfs の観測を
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
これは Workspace データの計測で、以下の rootfs/Image と保持する archive の計測とは別です。
一般的な Base ビルドの効率、物理回収の完了、大規模負荷は未検証です。論理削除を正確に
報告するために、容量が減るまで待つ必要はありません。

## rootfs の保存と通常 Image の再利用の計測

同じ必須試験 `TestRealIncusSnapshotAggregateE2E` に、独立した rootfs 観測処理を
加えます。小規模な合成 rootfs 負荷を次の二経路で計測し、内容の一致だけから物理的な
共有を推定しません。

1. 稼働中の Snapshot 取得前に、元のゲストから rootfs へ 8 MiB の乱数ファイルを
   書き込みます。元と独立した保存済み rootfs を計測し、元のファイルを unlink しても
   保存済みの内容が残ることを確認します。公開コマンド `open --new --snapshot` は
   保存済み rootfs から直接コピーし、停止後の `env copy` はさらに独立した rootfs を
   作ります。保存・復元・コピーでは、実際の共有 extent を要求します。コピー先だけに
   8 MiB を追加し、参照割当量と排他的 extent の増加、保存済み・復元元の内容不変を
   確認します。
2. Snapshot が生成した通常の Image を完全な不変 fingerprint で固定します。
   通常の `open --new IMAGE` を既存の作成コントローラーで二回実行します。
   読み取り専用の Incus API 観測で、最適化されたイメージキャッシュを fingerprint・
   project・pool に結び付け、実際の cache subvolume が読み取り専用であることも
   確認します。cache と Env のファイルには共有 extent を要求します。一方の Env だけに
   8 MiB を追加し、その参照割当量と排他的 extent の増加、他方と cache の内容不変を
   確認します。

保存済み rootfs と通常 Image の経路は基準値を分けます。Image の公開、export、cache の
展開でデータが書き直される可能性があるため、保存済み rootfs から image cache への
共有は**要求も主張もしません**。cache は Incus の所有物であり、観測によって Hacocoon に
編集権限が生じるわけではありません。この Image は Snapshot が生成したもので、一般的な
Base ビルドや Packer の負荷ではありません。既存試験の合成 OCI データの準備も、
実際の OCI イメージコンテンツの受入確認とは扱いません。

固定された各段階で rootfs 全体と計測用ディレクトリを分け、前述の論理容量・参照割当量・
FIEMAP の値を記録します。SHA-256 は既知の 8 MiB、リンク数一の通常ファイルだけを
上限付きで読み取ります。領域の識別には、確認済みの project・pool と型付きの
runtime/owner または Image 識別のハッシュを使い、正確な所有対象は永続 catalog に
保持します。通常 Image の fingerprint と生成元も明記します。

観測は正規のライフサイクルロックを保持し、rootfs の正確な所有権と現在の状態を確認して、
稼働中の試験用インスタンスだけを Incus で一時停止します。停止済みの保存インスタンスは
起動しません。観測側 Host のマウント名前空間をカーネルの平坦な一覧で確認し、上限を超えた
一覧やプールのマウントが一意でない状態を拒否します。計測 rootfs と重なる追加マウントは、
親・同一パス・子孫のどれでも再帰読み取りの前後に拒否します。同じファイルシステムの
bind mount も対象です。ゲストのデバイス設定を Host のマウント構成の証拠にはしません。
rootfs と親ディレクトリの識別も観測中は固定します。
読み取りと再開の前にも識別・一時停止状態を再確認します。観測失敗や不明な
状態では、一時停止したインスタンスも含めて調査用に残します。ファイルの書き込みと unlink は
所有するゲスト内だけで実行し、backing rootfs/cache subvolume、loop デバイス、mount、
製品の導入処理は変更しません。

元の削除と残るコピーの最後の unlink は論理的な削除の観測です。一回の同期と計測では
extent の回収完了を待ちません。プールの値は `pool_scope=whole_shared_pool` のままで、
他の rootfs、Image、メタデータ、Host の活動を含みます。領域ごとの割当量を足して固有の
使用量を求めたり、プール差分を一操作へ帰属させたりしません。FIEMAP の長さは圧縮後の
バイト数、デバイス書き込み量、公開・export の増幅、discard、Windows VHDX の回収量を
意味しません。

実行には前節と同じ、変更のないコミットからのビルドと集約試験のコマンドを使います。
新しいワークフロー、導入経路、計測フラグは追加しません。成功を主張する前に、実機の値と
正確な試験コミット・run を[受入記録](../status/acceptance-evidence.ja.md#rootfs-image-sharing)に
残す必要があります。リポジトリの検査や従来の Workspace/Host の証拠は rootfs/Image の
受入成功を証明しません。一般的な Base ビルド・Packer の効率、実 OCI 負荷、物理的な
回収完了、大規模負荷、他のファイルシステムは
[issue #241](https://github.com/SLktEx/Hacocoon/issues/241)の今回の限定範囲外です。

## 保持する native archive と import 展開の計測

通常 Image の経路では、Env B に既存の 8 MiB 差分を書いた後、正規の削除前に export します。
通常の停止と公開 CLI の `haco env export`、`haco env import` は、既存の非公開
コントローラーと正規の移送サービスを使います。実際の `.haco` 出力を試験専用の非公開
ディレクトリに保持し、移送先一つを新規作成して計測・削除します。集約試験全体が成功した
場合は既存の試験用ディレクトリ削除で archive も消し、この計測が失敗した場合は復旧用
catalog とともに残します。新しい合成の元環境、
Base ビルド、プール、権限、mount 変更、計測フラグは追加しません。従来の集約 export は
計測用ファイルの作成前に行われる別の機能試験で、そのバイト数を今回の計測へ流用しません。

次の観測を分けて記録します。

- 保持 archive: リンク数一の通常ファイルとファイルシステムの識別ハッシュ、数値の
  filesystem type、論理容量、参照割当量（`st_blocks * 512`）、全体の SHA-256。
  import の前後と移送先削除後に、上限付き読み取りの前後で読み取り専用 descriptor、
  パスと祖先ディレクトリの所有・識別を再確認します。archive は管理 Btrfs プールとは
  別のファイルシステムに置かれる場合があります。
- native component: 既存の bundle 検証器で全体を検証し、manifest の固定 role ごとの
  `bytes` と `sha256`、合計と外側の付加分（ファイル容量から component 合計を引いた値）
  だけを記録します。計数だけのための展開や component コピーは行いません。
- 元と移送先: rootfs 全体、計測用ディレクトリに加え、`base`・`delta` 各ファイルの容量と
  不変のハッシュを確認します。既存の正確な所有識別、Environment → Workspace の
  ロック順、一時停止・読み取り・再開、Host mount 範囲確認を維持し、ゲストから見た
  計測ファイルの数値 UID:GID が `0:0` のままであることも確認します。
- 背景と後始末: 既存の `whole_shared_pool` は背景情報のままです。export の一時
  Snapshot と image 操作記録が通常のライフサイクルで解消され、移送先 Env・Workspace・
  必要なら Store の native/catalog 不在が確認できることを要求します。その間 archive は
  読み取り可能なまま保持します。

`separately_materialized_from_source` は、元と移送先に同じ差分の内容があり、元の差分の
extent が export 前と import 後の両方で完全に排他的な場合だけ記録します。移送先が自分の
import cache と共有していても、この限定的な判断は変わりません。元や基準値に共有が
残る場合は `inconclusive_source_shared` または `inconclusive_baseline_shared` と記録し、
排他的になるまで再試行しません。別々の観測でハッシュが一致し set-shared が非ゼロでも、
その二つが共有している証明にはしません。物理 extent の対応関係は推定しません。

移送は非圧縮の native rootfs・volume archive を公開します。通常の Base 公開は別の
圧縮経路を使うため、今回を Base ビルドや Packer の効率計測とは扱いません。
component archive、bundle の一時保存、メタデータを書き換えた import archive は、
実装上で確認できる一時的な実体化段階です。その割当量やメモリーの最大値、デバイスの
書き込み増幅は計測しません。保持出力と移送先の割当量を足して、一時容量の最大値や
固有の物理使用量とみなすこともできません。

実行には[前述と同じ集約試験のコマンド](#管理-workspace-のライフサイクル計測)を使います。
追加した archive 経路は、PR #753 の検証用マージ
`c9ba3ac33323c091c1c2450beafc3515dc65b1e1`の初回実機試験で成功しました。
[正確な実行に結び付いた受入記録](../status/acceptance-evidence.ja.md#archive-materialization)に、
保持容量、元と移送先の計数、差分の別実体化、物理回収を主張しない論理的な削除を残しています。
issue の範囲は代表的な実体化境界と論理削除・容量回収の区別であり、
普遍的なスケーリング、全 builder、物理回収の成功は追加条件ではありません。

## Workspace の境界

通常の管理 Workspace は、管理プール内の独立した Incus カスタムボリュームと正規の利用権を使います。保持している外部パス方式は、利用者が選んだディレクトリを接続するもので、プール内に置く必要はありません。ルートファイルシステムの構成に合わせるためだけに、任意のソースコードを管理ストレージへ移動しません。

## 複数 pool

既定 local プールは `haco-local-default`。将来の明示 configured プールも、同じ single-owner ライフサイクルを維持したまま独自の Incus-managed ストレージ識別を利用できる。
