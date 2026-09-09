# ストレージの空き領域回収

日本語 | [English](storage-reclamation.md)

状態: **partial、内部実装**。Linux の実体照合・割当量測定、Btrfs trim と外側 ext4
への discard を実装し、隔離した実環境で検証しました。信頼できる対象選択、一つの
入口で全層を回収する操作、Windows の停止・圧縮・再開の連携は未実装です。Windows の
ファイル測定と native 圧縮は以下の内部実装まで進んでいます。F1 は
未完了であり、データの明示削除・GC・移行とは別の機能です。

## 必要な結果

管理対象 Btrfs pool、Incus loop backing file、外側 WSL filesystem、Windows VHDX
の未使用割当を回収します。論理サイズ、前後の実割当量、削減量、失敗・SKIP を区別し、
削減量0も正しく表示します。kernel の trim 報告量を Windows の実回収量とは扱いません。
backing file の切り詰めや pool 容量の縮小、Workspace・Git・OCI・snapshot の削除は
行いません。空き extent は filesystem が判断し、保存物の参照を維持します。

## Incus と実体の照合

pool 作成・接続・mount の寿命は Incus が担当します。利用者指定の Host path や
loop 番号ではなく、信頼できる設定から対象を選択し、実体照合も行う必要があります。
内部 primitive だけでは pool の操作を許可しません。

backing・mount・loop の handle を保持し、全 symlink 要素と hardlink を拒否します。
単一 device の Btrfs と loop の backing device/inode、offset/size limit が0であることを
照合します。名前の再照合と handle 保持により置換を検出し、操作対象のすり替えを防ぎます。
既存 doctor の観測だけを変更の前提にしません。必要な openat2 がない kernel は未対応です。

cold WSL では pool が未 mount の場合があります。Incus 外で独自 mount は行わず、
通常の Host entry または正確に所有する Incus runtime により使用状態を保ちます。
操作 handle を閉じてから使用状態を解除します。

## 外側 filesystem と Windows

ext4 discard は照合済み backing file の handle が属する filesystem に対して行います。
[Linux ext4 FITRIM](https://github.com/torvalds/linux/blob/v6.6/fs/ext4/ioctl.c) を使い、
別の mount path は選択しません。pool の許可とは別に管理対象 WSL 全体への操作許可が
必要です。他の外側 filesystem は未対応です。両段階とも公開経路には未接続です。

同期と前後の実体確認を行い、kernel 内でキャンセルされた場合も実行を試みた記録と
取得できた結果を保持します。キャンセルを未実行の証拠とは扱いません。

Windows 側は planned です。正確な一つの WSL 登録・VHDX を照合し、その WSL の
終了を越えて処理を継続し、圧縮・実割当量測定・再開を行います。全 WSL の停止や
drive 全体の処理、隠れた backup、Env 完全復旧の仕組みは追加しません。
[CompactVirtualDisk](https://learn.microsoft.com/en-us/windows/win32/api/virtdisk/nf-virtdisk-compactvirtualdisk)
の成功だけで実回収を推測せず、各段階の結果を保持します。

## 検証

sparse 割当、名前置換、symlink/hardlink、不正対象・キャンセルの拒否回帰は成功しました。
最初の実体照合は未 mount のため unsupported で失敗し、trim は未実行です。
Incus で mount を維持した検証は22.28秒で成功し、専用 instance を cleanup しました。

専用 WSL の内側 trim は23.58秒、Btrfs と ext4 の連続 trim は19.81秒で成功しました。
隔離1GiB pool の割当量は filler 削除前72,523,776 byte から trim 後1,417,216 byte に
減り、論理サイズ1,073,741,824 byte と volume・snapshot の内容を保持しました。
ext4 の kernel 報告は1,075,829,817,344 byte ですが、Windows 割当量は未測定です。
検証自身の filler だけを除去し、確認後に正確な所有 fixture を cleanup しました。
所有記録は残しています。

専用 root Incus fixture は `HACO_E2E_RECLAIM_TRIM=1` で実行します。
`HACO_E2E_RECLAIM_OUTER_TRIM=1` を追加した場合だけ、専用 WSL の外側 filesystem
も discard します。無関係な共用 Host に検証目的で指定しないでください。公開の全層操作は未検証です。
別途実行した Windows 圧縮の結果は後述します。[ADR 0048](../adr/0048-storage-reclamation-identity.md)を参照してください。

## Windows のファイル実体と割当量

`internal/wslreclaim` に Windows 専用の内部測定を実装しました。VHDX と全親 directory
を handle で保持し、reparse point・複数 hardlink を拒否し、共有制約で rename を防ぎます。
現在はローカル drive path に対応し、UNC・device path・別 stream・曖昧な Win32 正規化を
拒否します。公開呼び出しは未接続で、この観測だけで WSL 停止や圧縮を許可しません。

native `FILE_STANDARD_INFO` で実割当量とファイル長を測定します。ファイル長は VHDX 内の
仮想 filesystem 容量とは別です。実 Windows で32MiBの sparse fixture を実割当64KiBと測定し、
内容保持・ファイルと親の rename 拒否・hardlink 拒否・junction 祖先の拒否を確認しました。
最初の属性読み取りだけの実装は rename テストに失敗し、`GENERIC_READ` で修正後に成功しました。
symlink fixture の生成は Windows 権限不足で SKIP です。junction 検証の成功を、その未実行
fixture の成功とは扱いません。

専用 WSL の初回読み取り測定では、ファイル長・実割当量とも8,373,927,936 byte
でした。その後の圧縮検証は以下に記載します。公開利用には信頼できる distribution
選択と停止・圧縮・再開の連携が必要です。実体の固定は維持します。

## native 圧縮の検証

内部の Windows 圧縮は、保持した file から volume GUID のパスを取得し、親ディスクを
辿らず VHDX を開き、動的かつ未接続であることを要求します。file・親 handle の固定を
維持し、native 完了と実割当量を分け、圧縮後に仮想容量・識別子を照合します。
キャンセルは同期処理を取り消しません。native open の共有違反だけを30秒の予算で
再試行します。個々の同期呼び出しは予算を超える場合があります。他のエラーは即時失敗し、
圧縮自体は再試行しません。状態は **partial、内部のみ** で、公開連携は未実装です。

初回の専用 VHDX 検証は4.27秒で `OpenVirtualDisk` の共有違反により失敗し、圧縮は
未実行でした。その後、隔離した native VHDX と実 WSL の圧縮は同じ固定を保持したまま
成功しました。「固定と native open が衝突する」という以前の断定は誤りで、初回の原因は
未確定です。検証した native 経路では file の固定解除や handle 引き渡しは不要です。

専用 WSL の実圧縮は17.66秒で成功しました。Windows のファイル長・実割当量は
8,373,927,936 から6,719,275,008 byte に減り、1,654,652,928 byte を回収しました。
仮想容量1TiBと識別子は不変です。同じ登録 ID の WSL を再開し、確認ファイルの hash と
9件の停止中 instance の名前・状態が一致しました。全 Workspace・OCI・snapshot の
保存内容すべてを照合したという意味ではありません。

続く停止直後の検証では、期限付き open 待機を含め34.21秒・69回の試行で失敗しました。
共有違反が期限まで続き、圧縮は未実行です。同じ登録 ID の再開と確認ファイル・instance
照合は再び成功しました。自動停止から圧縮可能になるまでの条件は未解決です。
先の圧縮成功で、この失敗を成功扱いにはしません。確認ファイルは
`/var/lib/haco-reclaim-compact.7NTvx7` に残しています。

隔離 native VHDX の圧縮、実体保持・rename 拒否、キャンセル、通常ファイル拒否、
期限付き共有待機の回帰は成功しました。公開の全層操作、自動停止・再開、その経路を
通じた保存データ全体の検証は未完了です。
