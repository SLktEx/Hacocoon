# ストレージの空き領域回収

日本語 | [English](storage-reclamation.md)

状態: **partial、内部実装**。Linux の実体照合・割当量測定、Btrfs trim と外側 ext4
への discard を実装し、隔離した実環境で検証しました。設定済み Incus pool の選択は内部実装済みです。
一つの入口で全層を回収する操作は未実装です。Windows の停止・圧縮・再開、
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
キャンセルは同期処理を取り消しません。native open の共有違反だけを90秒の予算で
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
照合は再び成功しました。これは30秒予算で実行した当時の失敗です。後述の90秒予算の成功と区別します。
先の圧縮成功で、この失敗を成功扱いにはしません。確認ファイルは
`/var/lib/haco-reclaim-compact.7NTvx7` に残しています。

隔離 native VHDX の圧縮、実体保持・rename 拒否、キャンセル、通常ファイル拒否、
期限付き共有待機の回帰は成功しました。公開の全層操作、自動停止・再開、その経路を
通じた保存データ全体の検証は未完了です。

## distribution の解放待ちと登録照合

WSL 2.7.13 では停止表示だけで native disk の解放を確認できませんでした。個別停止直後の
読み取り open は4.10秒で失敗しました。その後の期限付き読み取り観測は61.62秒で成功し、
204回の open と57.70秒の待機を経て未接続状態を確認しました。同じ登録 ID の再開、
確認ファイル hash、9件の停止中 instance 記録も一致しました。ディスク変更は未実行です。
30秒の予算は実測した解放時間より短かったため、native open は90秒まで待ちます。
他の WSL 利用者が共有 VM を稼働させている場合や、アイドル終了が無効・長時間の場合の
解放を保証するものではありません。

導入済み版の [WSL session 実装](https://github.com/microsoft/WSL/blob/2.7.13/src/windows/service/exe/LxssUserSession.cpp)
は distribution 停止と共有 VM のアイドル終了を分けています。成功させるために全体設定を
変えたり、無関係な distribution を停止したりしません。導入済み CLI に native compact
コマンドはなく、新しい上流実装を利用可能な機能と取り違えません。

内部の登録照合は、現在の Windows ユーザーの WSL registry に対し、非ゼロの明示 GUID、
リテラルの名前・base・VHD filename、WSL 2 を要求します。項目欠落、曖昧なパス、既定や
名前へのフォールバックを拒否し、再照合では登録値の変更を拒否します。変更境界ごとに、
許可済み管理対象と保持したファイル実体への結び付けが別途必要です。この観測自体は
権限の根拠ではなく、公開の継続処理も未完成です。

調整後、正確な登録・固定ファイルの事前照合は4.42秒で成功し、停止直後からの圧縮は
144回の native open を経て79.16秒で成功しました。Windows のファイル長・実割当量は
6,768,558,080から6,747,586,560 byte に減少し、20MiBを回収しました。圧縮前後の仮想容量
1TiBと識別子は一致しました。同じ登録 ID の再開と確認ファイル・9件の instance 照合も
成功です。停止・再開は検証 driver が担当し、製品としての連携は planned のままです。
登録の型・パス拒否の native 回帰と amd64／arm64 ビルドも成功しました。
既存 catalog・snapshot の移行、WSL 登録・設定の書き換えはありません。将来の公開操作を
通じた Workspace・OCI・snapshot の保存内容全体の検証は未完了です。

## 設定済み Incus pool の入口

`Runtime.PrepareStorageReclamation` は、信頼済みのローカル設定と既存 pool、保持した
filesystem・loop・image の実体を結び付けます。Created 状態の Btrfs pool が一件だけ一致し、
設定した mount policy と標準パス `/var/lib/incus/disks/<pool>.img` が一致することを
要求します。任意の data directory・独自 Incus 配置は現時点で未対応です。backend の
応答から別の Host directory を選びません。固定後と各 discard の直前に pool の対応を
読み直し、欠落・不正・重複・切り詰め・失敗・不一致を変更前に拒否します。native 操作と
close は直列化しています。

この入口は pool を作成せず、独自 mount もしません。cold pool は通常の Incus 利用によって
mount を保持する必要があります。構成層の入口は引数なしで既存設定の pool を選び、呼び出し側の
pool・path 指定を受け付けません。外側 filesystem の権限と Windows 継続処理には
別途管理対象との結び付けが必要です。公開の全層操作は planned のままで、Linux だけを
回収する公開コマンドは追加していません。

既存の専用 Incus/Btrfs fixture をこの入口経由へ変更し、26.52秒で成功しました。
隔離1GiB pool の volume・native snapshot の内容を保持し、filler 削除と内外 discard 後、
backing 割当量は72,523,776から1,417,216 byte に減りました。所有 fixture の cleanup 後は
Incus pool・backing image の不在も確認し、所有記録を残しています。この Linux 検証では
Windows 割当量を測定していません。選択・拒否の回帰と race 検証も成功しました。
この段階では catalog 移行や利用者向けコマンドの変更はありません。

## 登録 GUID に結び付けた Windows 処理

導入済み WSL 2.7.13 の [terminate 実装](https://github.com/microsoft/WSL/blob/2.7.13/src/windows/common/WslClient.cpp)
は指定名を改めて解決します。内部処理では `wsl.exe --distribution-id <GUID>` を使い、
固定した `systemctl --no-block poweroff` を実行します。対象 distribution 内の終了は
[WSL の systemd](https://learn.microsoft.com/en-us/windows/wsl/systemd) が担当し、全体の
shutdown・unregister・名前へのフォールバックは行いません。この停止経路の読み取り専用
実機確認は68.82秒で成功し、同じ登録の再開後、確認ファイル・9件の instance が一致しました。

Windows 側は OS の System32 にある `wsl.exe` と固定引数・root 作業 directory を使い、
呼び出し元の環境変数を除去します。停止・圧縮・再開の間は VHDX と親を固定し、WSL 呼び出しの
前後で登録値を再照合します。停止要求の受理だけを終了完了とは扱いません。停止を試みた後は、
失敗・キャンセル時も独立した2分の制限内で同じ GUID の再開を試みます。圧縮と再開の失敗は
それぞれ残し、再開成功で圧縮失敗を成功扱いにしません。

これは内部処理です。`/usr/bin/true` の起動成功は WSL のプロセス再開だけを確認し、
Incus・controller の readiness は証明しません。公開前には、実行意図・結果の永続記録、
操作の排他、導入済み Physical Host に結び付く認可、呼び出し元 WSL が終了しても存続する
Windows 子プロセスが必要です。自動クラッシュ再実行や backup は追加していません。

続いて内部処理の実機検証が193.47秒で成功しました。停止要求、91回の native open、
圧縮、同じ GUID の再開を確認しました。Windows 割当量は6,883,901,440から
6,776,946,688 byte（102MiB減）になり、圧縮前後の仮想容量1TiB・仮想ディスク識別子は
一致しました。再開後の確認ファイルのハッシュと停止中の Incus instance 9件も一致しました。
Workspace・OCI 全内容や controller readiness の検証ではありません。Windows の単体・
拒否・失敗／キャンセル検証と amd64・arm64 ビルドは成功しました。symlink fixture は
Windows アカウントの作成権限不足で SKIP、junction 拒否は成功しました。
catalog・保存済みデータの移行はありません。

## 実行中の処理の排他

native 処理は WSL の固定・停止前に、実行ユーザーの SID と登録 GUID に対応する
[Windows の名前付き mutex object](https://learn.microsoft.com/en-us/windows/win32/api/synchapi/nf-synchapi-createmutexexw)
を global namespace に排他的に作成します。保護した DACL で実行ユーザー・SYSTEM のみ
アクセスを許可し、handle は継承しません。既存 object や作成エラーは処理を拒否します。
待機・thread による mutex 所有・自動再試行・引き継ぎはありません。再開とディスクの
固定解除まで handle を保持し、close でデータを削除しません。

これは Hacocoon の処理の同時実行だけを防ぎます。登録の認可、Windows 所有者による
直接の WSL 操作の禁止、中断した処理の結果保持は担当しません。名前の先取りでサービスを
妨害できても、操作の許可にはなりません。公開前には実行意図・結果の永続記録と中断時の
扱いが必要です。Windows object が消えたことを前回の成功の証拠にはしません。

Windows 実機で別プロセスの競合拒否・解放後の再取得、異なる GUID の独立性、
不正識別子の拒否、close の冪等性が成功しました。既存の native file・空 VHD・失敗系
検証と amd64・arm64 ビルドも成功しました。別の Windows ログインセッション・ユーザー
での実行は未検証です。symlink 作成は権限不足で SKIP、junction 拒否は成功しました。
上記の専用 WSL の停止・圧縮・再開検証はこの排他追加前のものであり、今回その全体処理は
再実行していません。
