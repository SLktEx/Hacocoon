# ストレージの空き領域回収

日本語 | [English](storage-reclamation.md)

状態: **implemented、native Windows/WSL CI の受入は成功**。公開の起動・結果照会・
明示的な失敗確認は、設定済み Incus pool と登録済み WSL の実体を使います。
データの明示削除・GC・移行は別の機能です。既存の手元環境での受入は別途記録します。

## 現在の native 受入

commit 5100d86 では、通常の Host 入口から haco reclaim --yes を実行し、再接続後の
haco reclaim --status まで通しました。Windows VHDX の実割当量は
7,964,983,296 bytes から 4,224,712,704 bytes へ減り、**3,740,270,592 bytes を回収**しました。
仮想容量は 1 TiB、Incus pool の容量は 128 GiB のままです。Linux discard、
対象 WSL の停止・圧縮・再開、Host に保持した marker、および切離し済み fixture の
Workspace・OCI・snapshot の復元を
[Windows の利用経路の job](https://github.com/SLktEx/Hacocoon/actions/runs/34623036552/job/103341362151)で確認しました。

これは記録した GHA の Windows/WSL 構成での結果であり、すべての手元環境や disk・platform の
受入を示すものではありません。以下の過去 commit ごとの結果は、失敗を含む歴史的な
checkpoint です。現在の結果は上記を参照してください。

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
必要です。他の外側 filesystem は未対応です。両段階は公開の回収操作に接続されています。

同期と前後の実体確認を行い、kernel 内でキャンセルされた場合も実行を試みた記録と
取得できた結果を保持します。キャンセルを未実行の証拠とは扱いません。

Windows 側は内部実装済みです。正確な一つの WSL 登録・VHDX を照合し、その WSL の
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

## 最後の操作の永続記録

内部処理は停止要求の前に、現在の Windows ユーザーの
`Software\Hacocoon\Reclamation\<登録 GUID>` レジストリ key へ version 付きの実行意図を
書きます。排他を保持し、ランダムな操作 ID・登録値・固定したファイルの識別子を記録します。
上限付きの一つの binary value に正規化した JSON を保存し、不正形式・未知 version／field・
重複 field・過大サイズ・不正な型は上書きせず拒否します。
[RegFlushKey](https://learn.microsoft.com/en-us/windows/win32/api/winreg/nf-winreg-regflushkey)
で永続化が完了してから停止を要求します。この flush はユーザーの registry hive に影響するため、
実行意図と最終結果に限定し、進捗のポーリングでは使いません。

最終記録は完了と失敗を区別し、各段階の観測結果を残します。認証情報・子プロセス出力は保存しません。
未完了・失敗の記録は次の実行を拒否し、完了記録だけが同じ登録・ディスクの新しい実行意図で
置き換えられます。自動再実行・確認済み扱い・記録削除は行いません。中断時の明示的な確認、
保護された導入先との対応付け、公開操作は引き続き planned です。既存 catalog・snapshot の形式や
Linux の名前だけの interop 記録は変更しません。記録対象は停止・圧縮・再開であり、その後の
handle close の失敗も呼び出し元に返します。

`e263f89` で、排他と永続記録を含む専用 WSL の native 全体処理が127.22秒で成功しました
（native open 7回）。Windows 割当量は6,827,278,336から6,787,432,448 byteへ38MiB減少し、
仮想容量は1TiBを維持しました。実行中に pending 記録を確認し、プロセス完了後の complete
記録は、操作・登録の GUID 全体、固定ファイルの識別子、測定値、仮想ディスク識別子と一致しました。
同じ WSL が再開し、確認ファイルの SHA-256 と instance 9件の一覧も一致しました。
完了記録は残しており、検証のために記録や既存データを削除していません。

これにより上記の記録・排他追加後の全体処理の未検証状態を更新します。電源断からの復旧、
別ユーザー・セッション、controller readiness、Workspace・OCI 全内容、今後の公開の全層一括操作を
証明するものではありません。既存の root 所有の Linux interop は distribution 名だけを保存するため、
現在の利用者向けの形式を維持し、Windows の変更操作の公開前に導入済み登録・ディスクとの
明示的な対応付けを追加する必要があります。

## Installer の登録対応付け

通常の Windows 導入は共通 Ubuntu setup 前に、一意のリテラルな WSL 2 登録 GUID を解決し、
その GUID を対象に共通 setup を実行して、終了後に名前との対応を再照合します。GUID 指定で
導入済み interop helper を呼び、root として `/etc/hacocoon/windows-registration.json` を作成します。
別の version 付き記録に正規形式の登録 GUID とランダムな導入 ID を保持します。既存の
`windows-distribution.json` は接続・通知用の名前文字列の JSON として維持します。
`-SkipIncus` は managed Host の登録を行いません。

記録作成は symlink をたどらず directory handle で進み、root 所有かつ他者が書けない親を要求します。
並行処理を直列化し、flush した mode 0600 のファイルを既存名の置き換えなしで公開します。
同じ GUID の再実行は導入 ID を保持し、GUID の変更、不正な所有者・mode・link、過大・未知・
不正形式の記録は上書きせず確認用に残します。クラッシュ時に公開途中の状態が残る場合も、
自動復旧・上書きはしません。既存導入は通常の installer の再実行で記録を取得し、追加オプションは不要です。

これは Linux 側の導入記録であり、Windows の変更操作の認可ではありません。Windows 側では
固定した VHDX・Windows 所有者を導入済み Host に対応付け、強制する必要があります。
Linux の記録のコピーだけで別の登録・ディスクを認可しません。公開操作と独立した Windows 子プロセスは
未完了です。Core の新しいストレージ interface は追加していません。

Linux native interop 15件（並行作成、不正・変更された記録の拒否を含む）、Windows installer
部品テスト、専用 WSL の実登録の解決が成功しました。実際の root 所有記録の作成も成功しました。
最初の再実行・既存ファイル比較プローブは、その WSL に `windows-distribution.json` がないため
失敗し、再実行の assertion に到達していません。既存名前ファイルの内容比較はこの fixture では
SKIP とし、成功扱いにしません。installer 全体と Windows の変更操作の認可の検証は未完了です。

別の native 再実行プローブでは、root 所有・mode 0600・単一リンクと、2回の作成呼び出し後も
内容・inode が同じことを確認し成功しました。既存名前記録は未作成のままで、上記の最初の
プローブ失敗は記録として残しています。

## Windows からの導入識別情報の読み取り

導入済み interop helper に読み取り専用モードを追加しました。作成時と同じ保護された
ディレクトリ・ファイル・正規形式の検査を使いますが、記録がなければ作成せず失敗し、正常な記録も
書き換えません。Windows 側は正確な GUID でこの固定モードだけを呼び、既存の環境変数を消した
実行環境と2分の制限を使います。出力は4KiBまでで、正規形式の schema・UUID と対象 GUID の一致を
要求し、未知・重複 field、余分な出力、切り詰め・上限超過を拒否します。helper の生出力はログに
記録せず、stderr は破棄します。

観測中は処理の排他と VHDX・親の handle を保持し、ファイル実体と登録の一致を再確認します。
導入識別情報・登録・固定したファイルの識別子・現在の Windows プロセスのユーザー SID をまとめますが、
認可の保存・付与・圧縮は行いません。同じ排他を使うことで、この読み取りが Hacocoon の圧縮中に
WSL を起動することを防ぎます。認可された対応の保存と変更時の強制は公開前の残課題です。

Linux native interop 16件（読み取りで未登録状態を変更しないこと、内容・inode の維持を含む）、
Windows の出力・識別子・拒否回帰、amd64・arm64 ビルドが成功しました。専用 WSL の native 観測は
26.87秒で成功し、想定した VHDX のファイル識別子と現在の Windows 所有者を確認しました。
専用 WSL の Hacocoon helper を更新し、直接の読み取りでも記録が維持されました。helper setup 中に
root の systemd ユーザーセッションの起動警告が出ましたが、helper 検証は正常終了しました。
controller・systemd ユーザーセッションの readiness 検証ではありません。Windows の symlink fixture は
権限不足で SKIP、Linux symlink・Windows junction の拒否は成功しています。観測処理は登録・圧縮を
行わず、installer 全体と公開の全層一括操作の検証は未完了です。

## 保存した導入情報と変更時の照合

Windows の内部 installer entry は、管理対象の登録 key の `Operation` とは別に schema 1 の
`Installation` を保存します。既存の排他とファイル固定を保持したまま、登録値・Linux の導入 ID・
Windows プロセスのユーザー SID・VHDX のファイル識別子を取得して永続化します。明示的な登録だけが
この値を作成し、同じ内容の再実行は許可しますが、変更・不正な記録は上書きしません。操作履歴は
別の値であり、登録によって確認済み扱いや置き換えはしません。

通常の内部処理は実行意図の記録・停止要求前に保存済みの対応を要求します。登録・ファイル・
Windows ユーザーの違いは Linux へ問い合わせる前に拒否し、その後に導入 ID を照合します。
停止と圧縮の直前に保存した対応を再読込し、圧縮前にも native 登録を再照合します。記録がなければ
自動登録しません。既存の操作記録・catalog・snapshot の schema は変更していません。
Linux の識別情報のコピーだけでは Windows のファイル・所有者の対応を満たしません。

内部の `haco-wsl.exe enroll <registration-guid>` を Windows amd64/arm64 用パッケージに同梱します。
通常の管理対象インストールは Linux の識別情報取得後、同梱チェックサムを検証して一意に解決した GUID で
呼び出します。Windows 呼び出し元の権限で動き、昇格・圧縮・汎用コマンド実行は提供しません。
SkipIncus は管理対象の登録に含めません。公開の変更入口・暗黙の登録 fallback はありません。
中断記録の確認、呼び出し元 WSL の終了後も生きる Windows 子プロセス、公開の全層一括操作は未完成です。
所有ユーザーによる直接の Windows 管理はこの排他の対象外であり、controller のリクエスト認可の代わりにはなりません。

Windows native の保存・拒否テスト（暗黙登録の禁止、同名での登録置換、別の導入・ユーザー・
ファイル、未知 schema／field、拒否時の元バイト保持）と amd64・arm64 ビルドは成功しました。
専用 WSL の明示的な登録・同じ内容の再登録は53.48秒で成功し、以前の操作記録も保持しました。
登録済みの停止・圧縮・再開は128.69秒で成功し、native open 6回、割当量は6,840,909,824から
6,817,841,152 byteへ22MiB減少、仮想容量1TiBは維持しました。再開後の確認ファイルのハッシュ・
instance 9件・保存した導入情報のバイト列も一致しました。公開操作・controller・全データ・電源断の
検証は未完了です。

PR #511 の head `81d105f` では GHA test 全10ジョブ・Ubuntu・Incus が成功し、認証付き private
registry は gate により SKIP でした。Windows の導入・再導入、Environment egress、native interop・
SSH、setup、preview、doctor、通知は成功しましたが、Remote-SSH 拡張インストールと承認レビューの
前提 setup-start（internal）で workflow は失敗しました。取得したログでは両方の原因を確定できません。
ローカルの回収検証でこれらを確認済みとしたり、成功扱いにしたりしません。

## 配布用登録処理の検証

状態: **partial**。native helper の引数・失敗・ログ秘匿テストと Windows PowerShell installer
fixture が成功しました。チェックサムの重複・欠落・不一致と helper の非ゼロ終了を拒否します。
パッケージ検証ではアーキテクチャと両実行ファイルのチェックサムを確認し、Windows 両アーキテクチャの
ビルドが成功しました。既存 Windows GHA はパッケージ作成前に native 拒否テストも実行します。
チェックサムは同梱内容の確認であり、guest から渡された実行ファイルを信頼する仕組みではありません。
固定 entry は installer パッケージのみが供給します。利用者のコマンド・option は増えません。
既存導入は通常の管理対象 installer 再実行で登録できますが、対応が変わっていれば拒否し、自動移行しません。

直前の head `d85df9a` は GHA 4 workflow すべてが成功しました。上記 `81d105f` の Windows 失敗を
取り消したり、その原因を確定したりするものではありません。同梱 helper を含む installer 全体の GHA は
未確認です。公開の容量回収・独立 Windows 子プロセス・中断記録の確認は planned のままです。
この変更に Env・Workspace・OCI・snapshot・catalog の schema 変更はありません。

実際の installer 関数からビルド済み helper を専用 WSL に対して呼び出し、47.54秒で成功しました。
Installation と Operation の既存バイトは不変でした。実登録への呼び出し検証であり、
新規インストール全体や容量回収の検証ではありません。

`8c8a543` の GHA は test・Ubuntu・Incus が成功しました。Windows の native テスト・release build・
package 作成・installer component の assertion は成功しましたが、別の BAT 終了コードテストが
空の fixture ディレクトリ削除時に共有違反で失敗しました。その後の新規導入・再導入などは SKIP です。
テストは正確な空 fixture に対する共有・ロック違反だけ最大10秒待機し、実際の子プロセスによる
ロック解放と非空ディレクトリの拒否を回帰検証します。インストールの再試行や失敗の成功扱いはしません。
元の CI でロックを保持した主体は未確定であり、製品の不具合とは断定しません。修正 head の実導入 GHA は未確認です。

専用 WSL の読み取り用 prototype では Windows 親が Job に所属することを観測し、明示的な
job breakaway とクリアした実行環境で独立した子を起動できました。子は呼び出し元終了後に隔離マーカーを
作成しました。WSL 停止・圧縮は行っていません。このホストでの起動方式の検証であり、WSL 停止を越えた
継続や公開の引き継ぎの検証ではありません。[Windows の要件](https://learn.microsoft.com/en-us/windows/win32/procthread/process-creation-flags)
により親 Job の breakaway 許可が必要です。未対応の Job では停止前に拒否し、親に拘束される子へ黙って切り替えません。

新しい共有 fixture の初回ローカル連続実行は子の準備前に検査して失敗しました。明示的な
ready/release 合図でテストの競合を除き、同じ PowerShell 5.1 の component と BAT の連続実行は
成功しました。この fixture 修正で製品 installer の動作は変更していません。

## 準備した操作の明示的な引き継ぎ

状態: **partial、内部のみ**。準備処理は排他・実体固定の下で保存済み登録を検証し、既存の pending
操作記録を永続化します。実行処理は正確な操作 ID を受け取り、排他・実体固定を取り直して登録・
Windows 所有者・ディスク・導入識別情報を再照合します。同じ pending 記録を要求したうえで、共通の
停止・圧縮・再開を実行します。別の操作・古い操作・完了済み・失敗済み・不正な記録は書き換えず拒否します。

同期の内部入口も同じ認可・実行処理を共有します。操作 schema と既存データは変更しません。
暗黙の採用・記録走査からの再実行・新しい復旧 coordinator・公開 resume コマンドは追加しません。
操作 ID は準備済みの要求を選ぶ識別子であり、導入や controller の権限ではありません。
準備・起動時のクラッシュは pending 記録を残して明示的な確認を要求します。独立 Windows worker と
公開の全層要求への接続は planned です。同じ Windows プロセスで二度呼ぶ検証は別プロセス引き継ぎの証明ではありません。

`7f4d7f4` は GHA 4 workflow がすべて成功し、同梱 helper を含む導入・再導入と
後続の Windows E2E も成功しました。修正 head の installer 検証待ちは解消しましたが、
以前の失敗 run を成功扱いにはしません。

準備済み native 実行は165.68秒で成功しました。open は70回、実割当量は6,833,569,792から
6,832,521,216 byteへ1MiB減少し、仮想容量1TiBは不変でした。保存された complete 結果は準備時の
正確な操作・登録・ファイル識別子を保持し、実測した各段階と一致しました。確認ファイルのハッシュ・
instance 一覧・保存済み登録のバイトも不変でした。Windows 拒否・単体検証と両アーキテクチャの
ビルドが成功し、Windows symlink fixture は権限不足で SKIP です。同じプロセスでの準備・再読込・実行の
検証であり、独立 worker・公開操作・controller・Workspace/OCI 内容全体の検証ではありません。

## 内部 Windows worker と結果の照会

状態: **partial。ローカルの Job context で native worker 検証は失敗**しました。
`cmd/haco-wsl` に正確な登録・準備済み操作 ID を扱う内部 `_launch`・`_continue`・`_status` を
追加しました。通常の `haco` コマンドではなく、transport・診断用の入口です。公開の準備・全層要求は
提供していません。worker は共通処理で保存済み登録と同じ pending 対象を引き続き要求します。

launcher はディスク観測と共通の native 検査で、自分自身の実行ファイルと親を固定します。
実行ファイルの書き込み・削除共有を禁止し、VHDX に必要な書き込み共有は維持します。起動するのは自分自身の
固定 worker mode だけで、detached・明示的な job breakaway・NUL stdin/stderr・OS 由来の作業ディレクトリ・
クリアした環境を使います。呼び出し元からの path・command や起動の自動再試行は受け付けません。
起動成功には専用 stdout pipe の準備通知と EOF が必要です。PID は完了を意味しません。worker は WSL アクセス前に console 接続を拒否します。明示的な breakaway 後に残る外側の Job は
許容します。その Job の終了で worker も終了し得ます。既存の Job 制限は変更せず、pending 記録と
正確な登録・操作の照合を維持します。

`_status` は既存 registry key を読み取り専用で開き、操作・登録の一致を確認します。WSL 起動・登録作成・
記録の確認済み扱い・worker の引き継ぎは行いません。pending は結果不明のため段階別観測を出しません。
complete/failed は保存済み観測だけを返します。起動前段の失敗ではプロセス終了後も pending が残り得ます。
成功や native 操作未実行の証拠にはなりません。中断記録の確認操作は未実装です。

Windows command/library テストと amd64/arm64 ビルドは成功しました。共通ファイル処理への整理で既存の
nil disk テストが一度失敗しましたが、nil の安全な拒否を戻し、native suite は成功しました。実行ファイルの
書き込み・rename 拒否と、実際の pending 記録を含む読み取り時のバイト保持も成功しました。
Windows symlink fixture は権限不足で SKIP です。

専用 worker の初回起動は exit 1 で失敗しました。同じ準備済み操作に対する明示的な診断起動も exit 1 で終了し、
WSL アクセス前に `worker is bound to a Windows Job` を報告しました。両プロセスは終了済みで、pending 記録を
保持しています。worker の停止・圧縮・再開経路は未検証です。別の pending 記録への置き換えや保存データの削除は
行っていません。外側 Job による終了リスクの明示的な承認を受け、Job 所属の一律拒否を解除しました。変更後は停止要求・再開に成功しましたが、native disk open の待機上限（320回）で圧縮前に失敗しました。正確な操作記録を failed として保持し、再試行・記録の置き換え・他 distro の停止は行っていません。standalone・nested Job の実機検証、
中断記録の確認、公開 controller 連携、全層一括操作は未完了です。

準備通知は4 byte（`RDY\n`）と EOF だけです。worker は排他・native pin の下で保存済み対応と
正確な pending 操作を確認した後に通知し、WSL 停止前に pipe を閉じます。launcher は最大3分待ち、
通知の欠落・不足・余分な出力を失敗として返して process handle を解放します。キャンセルは pipe
だけを閉じ、worker の kill・再試行・引き継ぎは行いません。タイムアウトと native 操作が競合し得るため、
保存記録を保持し結果不明として扱います。command・guest 出力・認証情報は通さず、最終結果は既存の
操作記録で確認します。公開コマンドや記録 schema は追加しません。

Windows native pipe の通知・EOF・キャンセルと command/library テストは成功しました。存在しない登録を
使った実 helper 起動は dispatch 成功でなく exit 1 を返し、既存 pending 記録のバイトは不変でした。
これは起動拒否の検証であり、worker の停止・圧縮・再開の検証ではありません。最初のテストコマンドは
PowerShell の引数解析で失敗し、引用符を付けた再実行は成功しました。`ee5e017` は GHA 4 workflow が
成功しました。準備通知の変更は別途 CI 確認が必要です。

## 最新 main との統合確認

`98a5626` で `55b0377` までの main を取り込み、回収処理の動作は変更していません。
Windows native library/helper、installer component、配布 package、PowerShell 5.1 の
BAT 終了コードテストは成功しました。新規所有の空256MiB VHDX は0.22秒・open 1回で圧縮が
成功し、割当4MiBは不変でした。専用 WSL gate は有効化せず、symlink fixture は権限不足で
SKIP です。以前の worker 圧縮失敗は失敗のまま保持します。Windows の package 初回実行は
子の `python3` 不在で失敗し、Ubuntu で成功しました。BAT テストは PowerShell Core で失敗し、
必要な5.1で成功しました。最新 head の GHA は確認待ちで、公開の全層受入ではありません。

## 終端の失敗結果を明示確認する

内部 Windows helper は、正確な `_status` 結果を確認した後の
`_review-failed REGISTRATION_GUID OPERATION_GUID` を受け付けます。
既存の continuation 排他、導入済み対応、Windows 所有者、Host 識別、disk の固定と照合を
維持します。導入識別の読み取りで選択した WSL を起動する場合はありますが、停止要求、
disk 圧縮、別 worker の起動は行いません。

確認できるのは現在の終端 `failed` 結果だけです。canonical な記録全体を
同じ private registry key の `ReviewedFailed-<operation GUID>` へ保存し、flush と
読み戻しを確認してから将来の新規 intent に進めます。確認自体では現在の結果を書き換えず、
旧操作は `failed` のままです。新しい intent は別 ID を持ち、その後も旧 ID の `_status`
で保存した失敗を読めます。同じ現在の失敗の再確認は冪等で、不明・競合する保存証拠を
上書きしません。

pending／結果不明の操作には使えません。launcher が後から worker を起動し得るためです。
成功済み、古い対象、別対象も拒否します。自動確認・再実行・新 schema・新復旧状態は追加
しません。公開の全層操作と pending の中断確認は未完了です。新しい回帰テストは、以前の
実 WSL 失敗の確認解除や再試行を行いません。

検証: Windows native registry の確認回帰は0.04秒で成功し、原文保持、冪等性、
pending・別対象・不正記録の拒否、新試行後の旧 ID 照会を確認しました。
native library 全体、helper テスト、vet も成功しました。専用 WSL gate は有効化せず、
symlink 作成は権限不足で SKIP です。実登録の失敗は確認解除・再試行しておらず、
その受入は未完了です。

## WSL 停止後に現在の結果を読む

内部 helper は `_status REGISTRATION_GUID` でも現在の保存済み結果を読めます。
停止中に呼び出し元が操作 ID を失った場合の読み取り専用経路です。WSL 起動、
登録、失敗の確認扱い、再実行、ロック引き継ぎは行いません。結果には保存された
実際の ID を返します。旧 ID を指定した失敗記録の読み取りも維持します。
現在の記録が不正なら履歴を探索せず拒否します。起動・継続・失敗の明示確認には
引き続き正確な操作 ID が必要です。

Windows native library/helper テスト、vet、amd64/arm64 build は成功しました。
registry 回帰は、不在 key を作らないこと、キャンセル、不正・別対象の識別、
不正な現在記録の拒否、元の bytes 保持を確認します。実際の完了結果の読み取りでも
記録は不変で、旧 ID の失敗を引き続き読めました。

## 専用 WSL の worker 実機確認

その後、以前の登録済み対象の失敗を明示確認しました。現在記録と保存した失敗の
bytes は一致し、元の結果も不変でした。通常の prepare で別 ID の pending を作成し、
一度の detached launch で停止・disk 圧縮・同じ登録の再開が完了しました。
worker process の終了と保存結果 complete を確認しています。他の WSL は停止していません。

native disk open は131回目で成功しました。ファイル長と割り当てはともに
10565451776 bytes から9626976256 bytes へ減り、**938475520 bytes（895 MiB）**
を回収しました。仮想容量1099511627776 bytes（1 TiB）と native 識別確認は維持しました。
以前の320回の open 失敗は失敗のまま保存しています。今回の成功だけで以前の原因や
任意の Job 下での生存を証明したとは扱いません。

既知の550415872-byte 合成 bundle の SHA-256 は不変でした。13 instance と54 volume
の識別一覧も前後で一致し、controller 起動と通常の Env 一覧取得は成功しました。
対象 bundle と一覧の確認であり、永続データ全体の bytes 比較ではありません。
導入済みの過去の v0.45 開発 build（commit 093ed15）では新規 Env の create/start、
Workspace 読み書きは成功しましたが、OCI directory の assertion は**失敗**しました。
作成結果の persistent-resource 対応は空でした。現行の OCI 自動構成の受入ではありません。保存した作成識別と照合した後、
通常の stop/delete は成功し、対象 Env の不在と Workspace の両 marker 保持を確認しました。

今回は Linux trim と Windows 圧縮を一つの公開入口から実行していません。
公開の全層操作、pending 中断の扱い、現行導入一式での Workspace/OCI 受入は未完了です。
日常コマンド、schema、backup、独自ストレージ lifecycle、権限は増やしていません。

## 導入する helper から操作を準備する

内部 helper は `_prepare REGISTRATION_GUID` を受け付けます。実機検証と同じ
canonical 準備処理を呼び、保存した pending の ID を JSON で返します。
登録済み対応、正確な disk・所有者・導入識別、排他、永続記録の規則は不変です。
worker 起動や WSL 停止は行いません。出力失敗は非ゼロ終了とし、保存済み intent
を残します。読み取り専用 status で確認し、失敗だから記録がないとは判断しません。
pending または未確認の失敗があれば、新しい intent は引き続き拒否します。

Windows の準備境界でテスト用 binary を使う必要がなくなります。公開の全層入口では
なく、controller の権限、Linux 段階との接続、通常利用者への結果表示は未接続です。
既存の実機 handoff gate もこの準備 API を呼びます。

導入用 helper API からの実機確認は成功しました。prepare が一つの pending ID を返し、
その ID の一度の起動で停止・圧縮・同一登録の再開が完了しました。worker の終了と
保存結果 complete を確認しています。open は95回目で成功し、ファイル長・割り当ては
9899606016 bytes から9865003008 bytes へ **33 MiB** 減少しました。仮想容量は1 TiB
のままです。既知 bundle の hash と保持済み Workspace の両ファイルは一致し、
通常の controller 起動・一覧取得も成功しました。準備にテスト binary は使わず、
worker 生存中に他の WSL 操作を差し込んでいません。Linux trim との統合や
現行アプリ一式・OCI の受入は依然としてこの確認に含みません。

## Linux 段階の controller 接続

状態: **内部接続は実装済み、導入済み環境での Linux 実機受入は成功**。
管理 endpoint 専用の `storage.reclaim-linux` は、正確な canonical 形式の
registration/installation 識別を受け取ります。pool、file、mountpoint、loop device、
shell command は受け付けません。controller は既存の root 所有の導入記録を
固定 reader で確認し、設定済み Incus pool の trim、導入識別の再確認、
外側 ext4 の discard の順に実行します。pool/mount の所有と native handle 検証は
引き続き Incus adapter が担当します。

server は5分の上限と同時呼び出し拒否を設けます。client 中断は再実行の許可には
なりません。失敗時も実行済み段階と観測結果を残し、後続段階は SKIP とします。
先行する trim 失敗と後片付け失敗が重なっても両方を保持します。失敗 report が
返ったことは RPC 応答の完了であり、容量回収の成功ではありません。内部 client は
report を出力して exit 1 を返します。不正な結果や通信断はエラーです。

| 配置 | 責務 |
|---|---|
| `modules/runtime/incus` | native pin、対応照合、FITRIM、計測 |
| `internal/composition` | 導入 WSL の照合と設定済み二段階操作 |
| `internal/reclamation` | 上限を持つ識別・結果の形式と検証。永続状態は持たない |
| `internal/controlapi` | 管理専用 request、排他、deadline、typed client |
| `cmd/haco-product` | Windows continuation 用の固定内部 `_reclaim-linux` 経路 |

composition の狭い target interface は失敗・順序のテストに使い、Incus adapter を
汎用 storage backend に変えるものではありません。観測型をこの境界で公開しています。
catalog schema、所有 ledger、backup、lifecycle transition は増やしていません。
通常の help・日常 CLI は不変です。Windows 側の Linux report 受け取りは下記のとおり
接続しました。公開の全層入口は未完了です。

固定した Btrfs/ext4 descriptor の `statfs` から前後の容量・使用量を取得します。
Incus loop file の長さ・割り当て、kernel の discard 報告量とは別項目です。
filesystem 使用量には内部の集計・予約も含まれ、正確な物理 file 使用量や Windows
の回収量ではありません。[Linux Btrfs statfs 実装](https://github.com/torvalds/linux/blob/v6.6/fs/btrfs/super.c)
を参照してください。未計測は0で埋めず省略します。不正カウンタ、演算 overflow、
filesystem 種別や観測容量の変化は成功扱いしません。

既存 Windows installation workflow に別の内部実機 gate を追加しました。
通常の setup で Incus 所有の Host/pool 利用を確立し、別 installation ID の拒否、
導入済み client/controller 経由の両 Linux 段階、既存 Host sentinel の保持を確認します。
正規の日常利用経路の gate は変更せず、WSL 停止や VHDX 圧縮、Workspace/OCI 全体の
保持までは検証しません。`29886ed` の Windows workflow とこの導入済み Linux gate は
成功し、Incus の Btrfs volume/snapshot 保持 trim step も成功しました。これは各 gate の
範囲の証拠であり、その後追加した Windows worker との一連の実行の証明ではありません。

既存 Incus-owned Btrfs workflow でも、専用の合成 pool を使う volume/snapshot 保持 trim gate を実行します。file 割り当てと filesystem 集計を検証しますが、runner の外側 filesystem 操作は許可しません。その outer discard は明示的に SKIP とし、専用 WSL gate が担当します。

## Windows worker による Linux 結果の保持

状態: **内部の実行順序は実装済み、一連の実機受入は確認待ち**。
通常の `_prepare` は version 2 の intent を作ります。独立 worker は保存済み enrollment と
正確な導入識別を検証し、Linux 実行開始を記録してから、固定した導入済み
`/usr/local/bin/haco _reclaim-linux` へ対応済み registration/installation ID だけを渡します。
Windows のコマンド環境は消去します。Linux stdout は4 KiB以下の canonical な型付き report
に限定し、未知・重複 field、不正な測定値、未対応 protocol は拒否します。
子プロセスの生出力や認証情報を失敗記録として保存しません。

検証した段階別 report を保存した後で WSL 停止を要求します。結果欠落、失敗終了、中断、
Linux 段階の失敗時は Windows 停止・圧縮へ進みません。終了確認に失敗しても取得済み report
は保持します。開始済みで report がない状態は結果不明であり、SKIP や回収量0ではありません。
開始済み pending 操作の再実行は拒否します。停止直前に導入識別も再読取します。
Windows disk pin、保存済み enrollment、native 排他、同一 registration の再開を維持します。
worker の上限は10分、controller 起動待ちを含む Linux 子プロセスの上限は6分です。

記録は既存の最終操作結果であり、自動復旧 journal にはしません。version 1 は元の canonical
バイト列と disk-only の意味を維持して読み取り・保持し、新規準備だけ version 2 を使います。
一括移行や catalog 変更は不要です。version 2 の読取には更新した helper を使ってください。
旧 helper は field を捨てず拒否します。明示的な失敗確認では、どちらの版も正確な操作 ID で
保持してから新しい試行を許可します。pending の黙った消去・確認済み化・再実行はしません。

Windows の引き継ぎと native 記録は `internal/wslreclaim` が担当します。controller、
Incus adapter、結果型 package の責務は維持します。内部 helper の引数は不変です。
Windows helper の常設と公開操作は下記のとおり接続しました。結果不明の pending を
明示確認する経路は未完了で、F1 完了とは扱いません。

既存 Windows CI gate に、配布 helper の準備・一度の起動、同じ Windows worker の終了待ち、
保存済み Linux/Windows 完了確認、再開後の通常 setup・Host sentinel 確認を追加しました。
worker 終了までは WSL 呼び出しを差し込まず、失敗・不明記録を保持して自動再試行しません。
新しい head の結果は確認待ちです。Workspace/OCI 内容全体はこの gate の対象外です。
native 記録・順序・拒否テストは、停止前の永続化、結果欠落、再実行拒否、旧記録と失敗記録の
同一バイト列保持を検証します。初回 native 実行は旧「version 2 は未知」の assertion で失敗し、
その assertion を引き続き未知の version 3 へ更新しました。専用 WSL の opt-in 実機検証は
これらのローカル native 回帰検証とは区別します。

今回の接続の検証では、Windows native library/helper 回帰と vet、Windows amd64/arm64
helper build、文書整合、workflow policy、gate 構文確認が成功しました。ローカルの専用 WSL
opt-in gate は有効化せず、symlink fixture は権限不足で SKIP です。上記の旧 version
assertion による初回失敗は失敗のまま残し、その後の修正版実行は成功として区別します。

## Windows helper の常設

状態: **実装済み、常設 helper からの worker 実機受入は確認待ち**。
通常の管理対象 Windows インストールで同梱 helper の checksum を確認し、Windows ユーザーの
アプリ用フォルダー配下の
`Hacocoon/reclamation/<括弧・ハイフンなしの registration UUID>/haco-wsl.exe`
へ導入し、その実体から enrollment を行います。追加 option、PATH 変更、昇格は不要で、
展開した package フォルダーを保持する必要もありません。更新も通常の installer を使います。

実行ファイルのコピー前に正確な登録を `installation.json` へ記録します。これは導入ファイルの
所有記録であり、WSL/disk 操作の認可ではありません。helper の別の native enrollment・操作照合
は引き続き必要です。所有不明・記録欠落・別登録・パス転送を拒否します。
排他的な一時ファイルへコピーし、flush・checksum 確認後に backup なしで原子的に置換します。
実行中 worker の native pin が置換を拒否した場合、installer は失敗を報告して既存実体を保持します。
後片付けは自分で作った一時ファイルだけです。途中導入は所有記録を保持し、明示的な installer
再実行で続けられますが、回収の自動起動・再実行は行いません。既存の操作・enrollment 記録は
このファイル導入処理では変更しません。

PowerShell 5.1 の component テストは隔離フォルダーの実ファイルを使い、新規導入・更新、
checksum/所有の拒否、実行ファイルのロック、junction 転送を検証します。初回更新は PowerShell が
`$null` を空の backup path に変換して失敗しました。明示的な .NET null へ修正し、修正版の
component 実行は成功しました。既存 native CI gate も展開 package のコピーではなく常設 helper を
使います。新しい head の結果は確認待ちです。

## 管理対象の識別取得

管理専用の読み取り `storage.reclamation-target` は、この controller の検証済み
registration/installation ID を15秒の上限で返します。呼び出し側の対象選択・path・pool は
受け付けません。型付き client は不正な応答や protocol 不一致を拒否し、service のエラーも
backend の生出力を公開しません。取得だけでは変更対象 storage の選択や Windows 操作の
認可を行いません。公開 CLI はこの API を使い、利用者へ GUID を要求しません。関連 controlapi/composition/controller テストと vet は成功しました。

`4369fdb` の一連の worker gate は **失敗** しました。導入済み Linux 両段階は成功しましたが、
`_launch` が exit 1 を返しました。取得結果から原因や worker 未起動までは断定できません。
その run の後続通知確認は成功しましたが、回収全体の証明ではありません。
gate の診断は固定した起動段階と native Windows エラー番号だけを記録し、子プロセスの生ログは
出しません。Job breakaway・console 分離・native pin は維持します。回収 gate が失敗した場合は
worker 完了が不明な可能性があるため、その後の通知 gate から WSL にアクセスしないようにしました。
常設 helper と診断を加えた追試の受入は確認待ちです。

現在の helper の起動診断を、別途生成した存在しない registration/operation ID で
実 Windows 上でも確認しました。期待する exit 1 が固定の `readiness` 段階として記録され、
stdout は空、該当 probe process は残らず、登録・操作記録も作成されませんでした。
これは起動拒否 probe の成功であり、GHA の起動失敗原因や停止・圧縮・再開の成功を
示すものではありません。この probe は実在する WSL を選択していません。

## 公開の起動と結果照会

状態: **CLI 実装済み、一連の導入済み実機受入は未完了**。
管理対象の trusted Hacocoon Host で作業を保存して実行します。

```sh
haco reclaim
# Hacocoon 再開後に Host を開き直す:
haco reclaim --status
```

最初の操作は停止・再開を確認し、`--yes` はこの質問だけを省略します。
GUID・disk path・pool の必須引数はありません。controller の読み取りで導入先を識別し、
既存 Host Windows interop を通して常設 helper が操作を準備し、一度だけ起動します。
起動の exit 0 は worker の受付であり、**回収完了ではありません**。出力欠落・起動失敗を
理由に再試行せず、保存結果を確認してください。`--status` は起動・再試行・記録消去・
確認済み化をしません。pending は完了不明、failed は非ゼロ終了で証拠を保持します。
中断 pending の明示確認には `haco reclaim --review` を使います。再試行のために記録を削除しないでください。
現行 controller と常設 Windows helper が必要で、旧導入先は通常の installer で更新します。

`cmd/haco-product` は引数・確認・表示、`internal/reclaimclient` は上限付きの具体的な
PowerShell 呼び出しを担当します。後者に backend interface や storage 権限はありません。
`internal/reclamation` は識別と結果の値、管理 endpoint は導入先の識別、Incus adapter は
pool の接続・選択と native discard、`internal/wslreclaim` は Windows enrollment・pin・
排他・停止解除と結果の永続記録を担当します。CLI の関数引数はテスト用の差し替え点であり、
別 provider のモデルではありません。catalog schema・Base 保存・snapshot backup・
自動復旧は追加せず、保存データの移行も不要です。

表示は Linux filesystem 使用量・loop file 割当・kernel discard・Windows VHD 割当を
区別します。欠けた測定値は不明のまま、過去の Windows-only 完了はそう明記します。
起動失敗でも pending 証拠を消しません。既存の Job breakaway・console 隔離は維持します。

公開 CLI の回帰検証は確認・中断・固定対象・再試行なし・出力不能・不正結果・測定値の表示を
扱います。native PowerShell 5.1 の起動・照会・準備拒否 fixture は隔離した通常ファイルで
成功しました。この fixture は実 WSL worker を動かしません。

`d675c5a` の一連の GHA 追試は `process_start`、native error 5 (Access denied) で
**失敗** しました。保存記録は pending、`linux_started=false` でした。先行する導入済み
Linux 両段階は成功しています。エラーだけで原因の Job・process 制約は断定できません。
公開 Host → Windows 起動と一連の Workspace/OCI 保持は **未検証** です。native protocol
fixture をその受入の代わりにはしません。[失敗した Windows run](https://github.com/SLktEx/Hacocoon/actions/runs/34555588035)を参照してください。

## 通常の Host 入口による受入

Windows workflow は導入済み Linux の識別・discard 直接検証を残し、その後は既存の
ConPTY driver で通常の `wsl -d Hacocoon` terminal を開きます。`haco reclaim --yes` を
入力し、表示された操作 ID の結果が complete かつ常設 helper の process が存在しなくなる
まで、Windows の読み取り専用 status・process 照会だけを行います。process 識別不能、
worker 不在の pending、失敗・timeout は、再試行や記録消去をせず gate の失敗にします。
CLI に表示する操作 ID は診断用であり、必須入力ではありません。

完了と process 不在の両方を確認した後だけ通常 Host を開き直し、`haco reclaim --status`
と installer の Host sentinel 保持を確認します。後続の通知受入にも回収成功が必要です。
Job・console 隔離と worker の起動 flag は変更しません。従来の runner 直接起動の失敗は
失敗のままで、この通常経路 gate の native 成功はまだ未確認です。照会の拒否回帰テストは
WSL 操作なしで成功しました。Workspace/OCI 内容全体は対象外で、F1 完了とは扱いません。

## 中断した操作の明示確認

状態: **helper・公開 CLI は実装済み、実登録 WSL の受入は未完了**。
`_review-interrupted` は正確な registration・operation ID を受け取り、準備時と同じ
continuation guard、enrollment の Windows ユーザー、登録・disk pin、導入識別の照合を
使います。動作中 worker があれば WSL に触れる前に拒否します。識別確認で選択した登録済み
WSL を開き直す場合はありますが、trim・圧縮・次の準備・起動は行いません。

最初に元の canonical pending 記録を操作 ID ごとに永続保存し、その後に現在記録の state
だけを `interrupted` にします。開始済み・結果欠落を含む元の Linux 観測は保持し、Windows
結果や成功を作りません。同一の明示確認は繰り返せますが、証拠欠落・変更・不正形式・別所有は
拒否します。新しい準備には元の pending 証拠との完全一致が必要で、操作 ID は新しくします。
次の準備前でも、遅れた旧 worker の実行・結果保存を拒否します。

この一つの state は旧 handoff の無効化に必要です。別の確認済み印だけでは旧 helper が
pending のままの操作を実行できてしまいます。既存 version 1/2 の encoding は維持します。
旧 helper は未知の `interrupted` を拒否するため、その読取・確認には更新済み helper を
使います。catalog 移行・記録削除・暗黙の再試行・全クラッシュ地点の復旧機構は追加しません。
元の pending バイト列は保存し、読み取り status だけ中断・結果不明として表示します。
回収成功にはしません。保存確認ができない場合、新しい操作への置換は許可しません。

native Windows の回帰は一意な一時 registry key で、元バイト列保持、別所有・disk・操作の
拒否、不正・欠落 archive、反復確認、遅延実行・結果の拒否、新規操作 ID を検証しました。
helper dispatch と公開の結果不明表示も成功しました。これらの隔離テストは、実登録 WSL の
動作中 worker を中断した受入の証拠ではありません。日常の確認操作への接続は下記のとおりです。記録削除で回避しないでください。

## haco から明示確認する

同じ trusted Host で `haco reclaim --status` を確認し、明示的に実行します。

```sh
haco reclaim --review
```

CLI が導入先・保存済み操作を取得するため GUID・path 入力は不要です。状態を表示し、
確認前の正確な対象・操作を固定します。`--yes` は質問だけを省略します。動作中 worker や
記録変更は拒否し、新しい別操作へ同意を流用しません。導入識別のため登録済み WSL を
開き直す場合はありますが、discard・圧縮・自動再試行は行いません。必要なときに別途
`haco reclaim` で新しい試行を始めます。完了済み・既に中断確認済みなら変更は不要です。
結果不正・取得不能・中断・確認文の出力不能では確認しません。enrollment・排他・pin・
証拠の照合は引き続き native helper が担当します。

公開 CLI のテスト・vet と native PowerShell 5.1 fixture は、正確な pending/failed の
確認選択・確認拒否を含め成功しました。これは protocol/component 検証で、実 worker 中断の
確認を証明するものではありません。`de72119` の公開 E2E は通常 Host interop からの
worker 起動に成功しましたが、保存結果が failed となり **失敗** しました。以前の runner
直接起動の Access denied は今回の原因を示しません。失敗段階は当時の出力にないため、
gate に保存証拠の固定 state・failure 語彙と真偽値だけを報告する処理と秘密値の除外回帰を
追加しました。任意の子出力は表示しません。[native run](https://github.com/SLktEx/Hacocoon/actions/runs/34559101015)を参照してください。
