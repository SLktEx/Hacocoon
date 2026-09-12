# Environment の持ち出し

状態: **全体として partial** です。Linux 公開 export/import と Windows ファイル共有経由の import は実装済みで、インストール済み controller、更新した鍵での SSH、保持データの再接続を実検証しました。実 OCI runtime の整合性、Git 再接続の実受入、全量退避は未完了です。
[Linux import コマンド](#linux-import-コマンド)を参照してください。

## Incus を土台にする

rootfs と custom volume の独立 archive は Incus の機能を使います。rootfs は1つの image archive として publish/export し、
custom volume は volume export/import を使えます。instance archive には
接続した Workspace／OCI custom volume の内容が含まれません。Hacocoon は archive と用途の対応を管理し、
必要な対象がすべて保存されたことを確認してから export 完了を返します。Base の名前・fingerprint は由来だけで、
追加の Base filesystem component は不要です。
[instance backup の範囲](https://linuxcontainers.org/incus/docs/main/howto/instances_backup/)と
[custom volume backup](https://linuxcontainers.org/incus/docs/main/howto/storage_backup_volume/)を参照してください。

通常のファイル archive は pool 間で持ち出せます。optimized archive は対応する storage driver に依存し、
速度だけの違いとして扱いません。後続の壊れた storage からの退避は別の作業であり、optimized export や
新規 snapshot の成功を前提にしません。

## データと権限

公開 import は canonical lifecycle の所有確認を使って新しい管理資源を作り、Workspace の Git 状態と
保持対象 OCI データを保存します。既存 Environment の上書きや、古い承認・接続・管理権限の復元は行いません。
Host の認証情報と control socket は Environment export の対象外です。archive checksum は変更検出であり、
設定を適用する権限ではありません。import された owner label も元の metadata であり、新しい lease ではありません。

専用実機の Incus 6.0.5 CLI には、最新 upstream 文書にある instance import の config/device 上書き引数がありません。
新しい flags を仮定したり、古い設定のまま起動してから修正したりしません。rootfs archive の検証、現行 security の
再構成、ファイル path の扱い、全対象の公開処理は実装・テストが必要です。

## Native custom volume の受入

`TestRealIncusVolumeTransferE2E` は専用 root Linux/WSL Incus+Btrfs 上で
`HACO_E2E_INCUS_VOLUME_TRANSFER=1` を指定した場合だけ実行します。新しいランダム名の2 pool と合成 `work`／`oci`
volume を使います。runner は Incus の mount を見られる必要があり、daemon が private mount namespace を使う場合は
同じ呼び出し内で確認した現在の namespace に入ります。作成前に `/var/lib` へ正確な所有対象を記録します。記録と archive は両 pool の外に保持します。
失敗時は明示的な確認用に資源を残します。cleanup は試験用所有 marker と pool の空を確認してから pool を削除します。
既存 Workspace／OCI Store／snapshot／共有 image／pool は選択しません。

non-optimized の `--volume-only --compression=none` export と第2 pool への import を使い、未push commit、
未commit・untracked、hardlink、symlink、file mode、独立した書き込み、archive 不変性、保存元削除後の独立性を確認します。
native import が古い user-config marker を引き継ぐことも確認し、Hacocoon で新しい所有権が必要な点を明示します。

公開 rootfs import、UID/GID・拡張属性の網羅、実 Docker/containerd 内容、公開コマンド、Windows への成果物保存、cross-host は
未検証です。volume テストの成功を G1 全体の完了とは扱いません。

専用実機の初回は export 前に失敗しました。fixture が volume path の `default_` prefix を欠き、
daemon の mount namespace 外で実行していたためです。正確な試験資源を保持し、owner marker と空一覧の照合後に削除しました。修正後、現在の daemon namespace 内での
Incus 6.0.5／Btrfs 検証は11.24秒で成功し、両試験 pool の cleanup も成功しました。記録と2 archive は
`/var/lib/haco-volume-transfer-2481101147` に保持し、初回失敗の記録も `/var/lib/haco-volume-transfer-3035986437` に残しています。

## Native rootfs image の受入

`TestRealIncusRootfsTransferE2E` は `HACO_E2E_INCUS_ROOTFS_TRANSFER=1` で実行します。
新しい隔離 project／pool と、Base・cached image を使わない空の停止 instance を作ります。
Incus publish/image export で1つの rootfs archive を保存し、保存元 instance と一時 image を削除します。
project の image 一覧が空であることを確認してから import し、現在の設定と `--no-profiles` を明示して
新しい instance を作ります。古い環境変数 token・instance ID・profile を復元しないこと、guest file 内容、
archive checksum を確認します。cleanup は正確な marker を照合し、archive と記録は残します。
これは rootfs component そのものであり、追加の Base filesystem・Base 登録・名前を変えた Base 保持物ではありません。
既存 snapshot の独立 copy は変更しません。

専用 Incus 6.0.5／Btrfs で14.88秒の検証が成功しました。archive は
`/var/lib/haco-rootfs-transfer-2526041618/rootfs.tar`、SHA-256 は
`9df946096ec6eb1b2a7a999937dd1d78bf79b61db91ebea43a6310f0843abf78` です。
初回は fixture が未対応の `image get` を使って失敗しました。既存と同じ JSON image API へ修正し、
所有 marker の確認後に正確な残骸を削除しました。`/var/lib/haco-rootfs-transfer-430939700/plan.json` は保持しています。

追加確認を含む専用 WSL Incus/Btrfs テストは 22.44 秒で成功しました。import した元 image の削除後、
image 一覧が空であることと、停止中の復元先から再取得したファイルが削除前と一致することを確認します。
保持 archive の checksum も不変です。native image と rootfs の独立性の確認であり、
コンテナが参照中の OCI image 削除の受入ではありません。

この小さなデータ fixture は OS 起動・SSH・管理 network・認証情報・template・全対象 import の受入ではありません。
公開 import は canonical な所有権・作成、archive の検証、現在の接続・security setup が引き続き必要です。
image の property・profile 関連付けは権限ではなく、現在の設定を明示して使います。
公開コマンドや catalog schema はこれらのテストで追加しません。

## 内部 transfer envelope

`internal/environmenttransfer` にストリーム書き込みの `Write` と読み取り専用の `Inspect` を実装しました。
内部 version 1 の manifest は、上限付きの source label、OCI の有無、順序付き role／size／SHA-256 を持ちます。
外側の USTAR は固定名の通常ファイル `manifest.json`、`rootfs.tar`、`workspace.tar`、追加分の連番 `workspace-002.tar`〜`workspace-253.tar`、任意の `oci.tar` です。
Workspace 数は現行 Incus snapshot の上限に合わせ、番号の欠落を認めません。metadata は64 KiB、envelope の
付加部分は512 KiB に制限し、payload の呼び出し元予算とは別に上限を設けます。
Base component、provider path、元の管理 ID、認証情報 map は持ちません。公開交換形式や公開 export/import ではありません。

上限付き canonical JSON で重複・未知フィールドを拒否し、role と順序、サイズ、呼び出し側の総量上限、header の種類・形式、
payload hash、完全な終端、余分な内容がないことを確認します。tar が内部で処理する拡張 header を含め、manifest 前の読み取りも
上限付きです。展開・Incus 呼び出し・全検証前の consumer callback は行いません。内側 archive の安全性や送信元の真正性は証明しません。

`WriteSnapshot` は保護された ready snapshot の全対象と archive を照合し、Workspace role 順に整列します。
component の identity・binding・状態の完全一致を要求し、欠落・余分・重複・不一致を出力前に拒否します。
Workspace／OCI の欠落も対象です。legacy Base record は元 catalog に残し、archive や Base filesystem の照会を要求しません。
未知の保持 role は黙って省略せず拒否します。呼び出し元は canonical な source reservation を保持し、archive 作成中の
native 所有権を検証する必要があります。渡された snapshot object 自体を権限とは扱わず、lifecycle lock や native 確認を
この関数で代替しません。`Inspect` だけでは信頼できない manifest から省略された元データを発見できません。
書き込み成功の場合だけ公開し、失敗した出力は公開しません。削除不明なら cleanup の所有情報を保持します。
後の import は不変の staging bytes を保持するか再検証し、canonical lifecycle と現在の security 設定を使います。
検証は source label や古い設定を適用する権限ではありません。[ADR 0049](../adr/0049-transfer-envelope-authority.md)を参照してください。

関連 race test と vet は成功しました。初回の切断回帰は Go tar reader が終端なしの EOF を受け入れて失敗し、終端の明示確認で修正しました。
payload 欠落・変更、OCI 欠落、Base／余分・順序違いの component、path／link／header 攻撃、重複 JSON、総量・overflow、
manifest 前の読み取り上限を回帰テストで確認しています。

## Linux の内部 staging

`Stage` は Linux／WSL 向けに **implemented** です。controller 所有の非公開ディレクトリを
symlink をたどらず開き、上限付き入力を native `O_TMPFILE` へコピーします。同じ inode を
読み取り専用で開き直し、書き込み handle を閉じてから envelope 全体を検証します。
検証済みの bytes とコピーした metadata だけを返し、reader は descriptor や pathname を公開しません。
入力や staging directory のパスを差し替えても、保持した内容は変わりません。
process descriptor を操作できる Host 管理者に対する封印ではありません。

結果を閉じるか process が終了すると、名前のないファイルは解放されます。入力失敗時に名前付きの
保存物や Incus 資源を残しません。process の寿命内の staging であり、永続的な復旧記録や自動 backup
ではありません。`openat2`／`O_TMPFILE` が未対応なら明示的に失敗し、fallback や権限修復は行いません。
入力の Read が停止した場合は、呼び出し元が transport をキャンセルして解除する必要があります。

Linux の実 filesystem を使う race test と vet が成功しました。パス差し替え、読み取り専用 handle、
不正・過大入力、整数上限、全 bytes 受信後の通信失敗を検証しています。最初の検証起動は shell の
PATH 引用エラーでテスト開始前に失敗し、修正後の起動で成功しました。Btrfs 上の staging と公開 import
への接続は未検証です。先行する native Incus archive の成功から、それらの成功を推定しません。

保存対象照合の回帰テストは、現行上限253個の Workspace、OCI の有無、順序の違い、対象の欠落・不一致、
catalog を変更しない legacy Base 除外を確認します。これは repository test であり、複数 Workspace の native な
全体 export は未検証です。先行する1 Workspace 限定の内部 envelope は公開形式や catalog schema ではありませんでした。
その1 Workspace の encoding は引き続き読み取り可能で、既存保存データの移行はありません。

## export 中の保存元の寿命

`workspace.Service.ReadSnapshot` は保存元を利用する境界として **implemented** です。
`DeleteSnapshot` と既存の Environment → Workspace の lifecycle lock 経路を共有し、
ロック後に catalog を再取得します。元の識別情報が変わった場合や ready でない保存物は拒否します。
保持された rootfs／Workspace／OCI は、既存 runtime adapter による全件の所有確認後に consumer へ渡します。
historical な Base filesystem は照会せず、元 Env の存在も要求しません。

consumer は返る前に元データの読み取りを完了し、lifecycle 操作へ再入してはいけません。
snapshot の値を保持するだけでは reservation になりません。cancel や consumer／検証の失敗では、
保存物を変更せず process lock を解放します。Linux／WSL は既存の process 間 filesystem lock を使い、
非 Linux のテスト実装は process 内のままです。新しい native controller platform の追加ではありません。
backup、永続的な export 状態、自動再開は追加しません。

native archive 作成と Linux 公開 export command を接続しました。この source lock で、
一時資源の厳密な所有管理、出力の完全な公開、復元先の security 再構成を代替しません。

## 保存 volume の native export adapter

Linux／WSL の `incus.Runtime.ExportSnapshotVolume` は、保存された Workspace／OCI volume 向けに
**implemented** です。既存の保護された component binding を検証し、通常の volume-only Incus export の
前後で、未接続の保存 volume の native 所有情報を確認します。呼び出し元は処理中 `ReadSnapshot` を保持します。
rootfs の export や、Environment 全体の bundle 公開はまだ行いません。

CLI は、生存中の親 process の `/proc/<pid>/fd/<fd>` 経由で名前のないローカルファイルへ出力します。
書き込み descriptor を閉じてから hash を計算し、読み取り専用の内容を返します。archive を容量制限付き stdout log
へ流さず、consumer にパスを渡さず、失敗時に名前付きローカル archive を残しません。controller の非公開ディレクトリは
`openat2` と `O_TMPFILE` に対応する必要があり、未対応なら明示的に失敗します。サイズ予算は native 出力後の検査であり、
Incus 書き込み中の disk quota ではありません。descriptor を操作できる Host 管理者に対する封印ではありません。

[Incus 6.0.5 volume export](https://github.com/lxc/incus/blob/v6.0.5/cmd/incus/storage_volume.go) は一時 native backup を作り、
終了時の削除エラーを無視します。adapter は export 前後の backup 名・作成／期限時刻・flags を比較します。
新しい／変化した backup が残る場合や最終確認に失敗した場合は、成功を返さず、確認対象の保存 volume をエラーに示します。
既存 backup を Hacocoon が削除することはありません。名前だけでは cleanup の所有根拠になりません。
出力 bytes が完成していても、native command の失敗を成功にはしません。

関連テストは所有情報の変化、出力後の native 失敗、backup の残存・確認不能・名前の再利用、空／過大出力、
実際の子 process による匿名ファイルへの書き込みを確認します。`HACO_E2E_INCUS_VOLUME_TRANSFER=1` で動く
`TestRealIncusOwnedVolumeExportE2E` は、新しい所有済み Btrfs pool 1個と合成の保存／import volume を使います。
正確な計画と archive を保持し、保存元削除後の独立性を確認して、特定した fixture 資源だけを検証後に削除します。
既存 Incus GHA job にこのテストを追加しました。公開の全体 export/import、OCI daemon 内容、復元先の権限処理は未完了です。

専用 Incus 6.0.5／Btrfs の実行は5.92秒で成功しました。計画と4096 bytes の archive は
`/var/lib/haco-owned-volume-export-778805159` に保持し、archive の SHA-256 は
`33e2785b3d574d504fb104c4a16bd154339d1c675d43241418ab1ecc171d155f` です。
元・import 先の fixture volume と所有 pool は、確認後に削除しました。関連 race test と vet も成功しました。
初期の未使用 import・文字列記述による fixture ビルド失敗は、native 実行前に修正しました。
新しい単一保存 volume adapter の確認であり、公開の全体転送や OCI daemon の受入ではありません。
出力先 filesystem が Btrfs の場合の匿名 staging は引き続き未検証です。

## 保存 rootfs の native export adapter

状態は **内部実装済み、専用 native adapter 受入は成功** です。公開 G1 には停止中 Env の export/import/start/SSH が
必要であり、export のために利用者へ別途 snapshot 作成を必須にはしません。この adapter は、保護済みの
独立した保存 rootfs を使う前提機能であり、公開フローそのものではありません。

Linux/WSL の Incus adapter の `ExportSnapshotRootfs` は公式 Incus 6.0.5 client を使います。
呼び出し元の `ReadSnapshot` reservation 中に保存 component を検証し、非圧縮 unified image として publish、
既存の匿名 `NativeArchive` へ streaming 取得、保存元の再確認、一時 image の削除を行います。
この image は転送中の rootfs component であり、追加の Base filesystem ではありません。
元 Base/image cache の検索、保存元の削除、自動 backup、古い権限の復元は行いません。

Incus CLI config から private Unix remote を選びます。publication 前に socket・project・ランダム owner を
記録し、戻された operation・fingerprint も後続確認より先に永続追記します。最初の local フローでは
HTTPS remote と cluster daemon は明示的に未対応で、local への暗黙 fallback はありません。
SDK metadata の受信上限は 1 MiB、event listener は無効、operation 待機は最終的な成功状態を必須にします。

image endpoint は選択済み Unix transport と明示的な request context を使います。redirect・split image を拒否し、
実際の書き込み量を制限して、archive 全体の SHA-256 と fingerprint を照合します。応答の filename をパスにしません。
これにより SDK 6.0.5 の `GetImageFile` が別の `/dev/incus/sock` を試すことと、download context の非継承を避けます。
archive の展開・実行はしません。

publication や cleanup が不明なら、非公開の `rootfs-export-<owner>.jsonl` receipt を残します。
自動再実行せず、記録された socket/project/operation と正確な `user.hacocoon.export-owner` property を調べます。
所有確認でき、alias のない一時 image だけを削除し、不在確認後に receipt を削除します。
置き換えられた、または共有された receipt は unlink しません。保存 rootfs は変更しません。
[ADR 0050](../adr/0050-native-rootfs-export-ownership.md)を参照してください。

回帰テストは保存元/image の owner 変化、publication 応答喪失、未完了 operation、download 失敗、digest/size 不一致、
cleanup 不明、receipt 置換、split/redirect 拒否、停止した通信のキャンセルを確認します。
`TestRealIncusSnapshotRootfsExportE2E` は既存 GHA の opt-in rootfs transfer gate に追加しています。
隔離 project/pool と合成した保存 rootfs を使い、native export、一時 image の不在、保存元保持、archive の
native 再 import を確認するテストです。起動可能な公開 import・SSH・OCI data の受入ではありません。
native 実行結果は単体テストの結果と分けて記録します。

専用 WSL Incus 6.0.5/Btrfs adapter 検証は 13.44 秒で成功しました。隔離した保存元・project・pool と
import した一時 image は確認後に削除しました。計画と archive は `/var/lib/haco-rootfs-export-1396608668` に
保持し、archive の SHA-256 は `195bb069299c130f187cd1cb814806a8e39966f2b089a86fcb9460e5d3e8da85` です。
関連 race 回帰は 3.614 秒で成功し、package vet も成功しました。この native 結果は component adapter の確認で、
未完成の公開 G1 フローを受け入れたものではありません。

## 停止 Environment export の内部処理

Status: **内部実装済み**。Linux 公開 CLI/controller の転送経路は後述の partial です。
`environmenttransfer.Exporter.ExportStopped` は既存の canonical な
`CaptureStoppedSnapshot`、`ReadSnapshot`、`DeleteSnapshot` を使います。
実行中 Environment を停止せず、利用者に別の snapshot コマンドも要求しません。
今回だけの COW capture は一貫した export 元であり、restore 前の backup ではありません。

capture 前に非公開の出力 directory を開きます。保存元の予約内で rootfs・全 Workspace・
任意 OCI の完全な inventory を確認してから native export を行い、全 archive で一つの
合計 byte 上限を消費します。過去の Base filesystem 記録は catalog に残し、転送から
除外します。native handle は予約内で閉じます。今回作成した canonical capture だけを
削除し、呼び出し元の取消後も期限付き cleanup を実行します。削除が不明なら、その
snapshot ID と catalog の途中状態を保持します。

envelope は既存の匿名 staging file に同期的に書き込みます。完成 bytes の後に producer
や cleanup が失敗した場合も、一部 component が欠けた場合も bundle は返しません。
最後に read-only file 全体を検証してから返します。元 Env の識別・lease・既存 snapshot・
現在データは変更しません。新しい catalog、クラッシュ再開、rollback、import/restore の
自動 backup は追加しません。native copy/export は Incus の component producer に任せ、
この package はデータを束ねる境界を担当します。

回帰テストは最大 253 Workspace と任意 OCI、過去 Base の除外、合計上限、遅い段階の
失敗、取消、descriptor cleanup を対象にします。実 JSON catalog/canonical lifecycle と
fake native adapter の結合テストで、running 拒否・共有削除 lock・既存 snapshot 保持・
cleanup 不明時の記録を確認します。これは実 Incus の aggregate export、公開 artifact
転送、archive import、OS 起動、SSH の受入ではありません。それらは planned または
未検証です。先行する native component テストの証明範囲も各 component に限ります。

既存の Linux `TestRealIncusSnapshotAggregateE2E` に、Base 削除後の routed catalog と
実 native producer を通る export、および元データ変更・元 Env 削除後の全 export bytes
再検証を加えました。専用 WSL Incus 6.0.5/Btrfs 受入は 314.12 秒で成功し、bundle を
返す前の一時 capture cleanup と元 Env 削除後の bundle 全体検証を確認しました。
同じ fixture の既存 snapshot restore・新世代識別・承認状態リセットも成功しましたが、
その restore は export bundle ではなく保存 snapshot を使います。今回所有する全 fixture
資源と recovery directory を cleanup しました。export archive は保持していません。

ローカルでは共有 source image を保持し、CLI binary を指定しなかったため、image 削除と
任意の公開 snapshot/Workspace CLI 経路は SKIP しました。既存 GHA aggregate gate は CLI を
指定します。実 SSH handshake・live OCI 整合性・公開 bundle import は未検証または未実装
です。`bbcf7ea` の全体 local CI（Go test/vet、通知27テスト）と文書チェックは成功しました。
最初の canonical 結合 fixture は native 名の重複で失敗し、capture ごとの固有名に修正しました。
製品の所有チェックや timeout は緩めていません。

## Linux export コマンド

Status: **partial**。trusted controller stream を使う `haco env export` を実装しました。
import の実経路受入は pending です。保存元 Env は停止している必要があります。

```bash
haco env stop dev
haco env export dev
haco env export dev /path/to/dev.haco
```

必須引数は保存元の名前だけで、既定では client の現在 directory に `dev.haco` を作ります。
任意の `--json` は保存先と byte/digest receipt を返します。保存先は client だけで扱い、
controller の filesystem path として送信しません。既存ファイルは、並行して作られた場合も
上書きしません。別の snapshot コマンド、元 Env の削除、restore 前の自動 backup は不要です。

管理専用の `environment.export` stream は source 名だけを受け取ります。controller の非公開
`$HACO_ROOT/transfers` を使い、canonical な停止済み capture を、合計 payload 64 GiB と
上限付き envelope overhead の範囲で export します。取消・切断は capture を取り消し、
cleanup が不明なら既存の所有記録を残します。guest Git や read-only 通知 socket には登録しません。

上限付き canonical JSON frame は最大 64 KiB の data を運びます。client は明示的な完了
count/SHA-256 receipt、cleanup 成功、EOF を要求します。早い EOF、重複 field、余分な
frame、cleanup 失敗、digest 不一致は失敗です。Linux CLI は匿名 file で envelope と
source label を独立に検証して sync し、固定した出力 directory に生きた inode を link します。
既存名は置き換えず、名前付きの途中 file や path による cleanup はありません。
[O_TMPFILE の文書化された公開手順](https://man7.org/linux/man-pages/man2/open.2.html)を使います。

出力先は現在、匿名 file を扱える Linux filesystem（ext4/Btrfs など）が必要です。
Windows native の file 公開と Windows mount への保存受入は未実装・未検証で、暗黙の fallback は
ありません。trusted `haco-host` 内で動く `haco` はその client の filesystem に保存し、Windows
desktop へ自動で保存するわけではありません。公開 bundle import、新しい権限での import、
import 後の起動/SSH、G1 全体の受入は planned です。

Unix stream と Linux filesystem/CLI の race test は成功しました。最初の CLI fixture は
socket mode 引数不足で compile に失敗し、fixture を修正しました。既存 native aggregate E2E は
CLI binary 指定時に shipped export CLI を呼ぶよう拡張しました。先行の 314.12 秒成功は
内部 producer の証明に限り、公開経路の GHA 結果は下記に示します。

最初の専用 shipped CLI 実行では export・元 Env 削除・公開 snapshot create/restore が成功し、
その後の copy が元の fixture の8分期限に達して終了したため、全体は 480.07 秒で失敗しました。
これは gate の失敗であり、成功や SKIP ではありません。正確な catalog と保存 archive は
`/var/lib/haco-snapshot-aggregate-2545909325` に残しています。test Env 2個は世代の一致を
確認して canonical な削除を完了しました。Workspace・OCI・snapshot と失敗 catalog は
明示的な cleanup のため保持しています。追加した archive 全体の転送・検証時間を含め、
fixture は12分、既存 CI の native test 群は15分の期限にします。製品の期限や隔離は変更せず、
修正後の native 受入は未完了です。`081beda` の local Go/vet/docs/通知 CI は成功しました。

修正後の専用実行も、最後の公開 Workspace cleanup 中に 720.06 秒で失敗しました。
その期限までに export・元 Env 削除・公開 snapshot restore/copy・同名での世代更新・
保存物の独立性・管理 SSH key のリセット・native child snapshot/backup の削除拒否は
成功しました。失敗 fixture は `/var/lib/haco-snapshot-aggregate-462967548` に残し、
残りの cleanup を成功とは扱いません。さらに期限を延ばす変更は行いません。
同じ [GHA aggregate gate](https://github.com/SLktEx/Hacocoon/actions/runs/34430493864/job/102724802406) は
`3d0dd9a` で 47.06 秒で成功しました。shipped export、snapshot/restore/copy、Workspace 削除、
所有対象 cleanup を含みます。該当する4 workflow もすべて成功しました。
これは Linux Incus/Btrfs の代替検証であり、local WSL gate 成功や復元後の実 SSH handshake
を証明するものではありません。

事後確認で、両方の失敗 fixture に Environment と Workspace lease が残っていないことを
確認しました。元の9個の instance、保護 sentinel の SHA-256、登録ファイルの mode/link 数は
維持しています。その後、fixture の snapshot 4個は
全 component の所有確認後に canonical API で削除しました。保持 OCI Store 4個・対応 Workspace
記録と export archive 2個は、明示的な cleanup のため残しています。

## import 向けの検証済み component 読み取り

Status: **内部実装済み**。公開 importer の受入は pending です。
`Staged.ComponentReader(role)` は、native archive 1個の seek 可能な read-only view を
返します。位置は全 component と envelope 全体を検証する同じ上限付き parser で記録し、
途中までの検証では公開しません。reader の位置は独立し、隣の component、manifest、
外側の file handle は読み取れません。staged bundle を close すると reader も使えなくなります。
Host directory への archive 展開は行いません。

これは今後の Incus image/volume import adapter へ渡す入力境界であり、元の設定を適用する
権限ではありません。内側 archive の検証、新しい native 所有情報、canonical な Workspace/OCI
登録、新しい Env の作成と現行 security 設定は引き続き必要です。CLI 引数と catalog schema は
追加しません。Linux の実 filesystem component test と既存 transfer suite は race detector
付きで成功し、vet も成功しました。native import は実行していません。

## native OCI volume import

Status: **内部実装済み**。persistent-resource service の import は、既存の新 owner の
`creating`／verify／`ready` 遷移を使います。Linux Incus adapter は、native metadata を
新しい所有情報へ置き換えた非公開の匿名 archive を作り、通常の
`incus storage volume import` を呼びます。投入後の所有情報修復や新しい復旧 catalog は
追加しません。[ADR 0051](../adr/0051-native-import-ownership.md) を参照してください。

初期対応は child snapshot を含まない、無圧縮・非 optimized の Btrfs filesystem volume
archive です。controller の archive 上限内で、index は64 KiB、path は4096 byte、entry は
100万件までです。元の権限情報は破棄し、検証済み idmap 情報は numeric file ID と一緒に
保持します。既存 volume、危険な path/link、重複 metadata、不完全 archive は拒否し、
未対応形式は明示的に失敗します。実際の展開は Incus が担当します。

専用 Incus/Btrfs gate は、新規2 pool と実 catalog を使い0.56秒で成功しました。新しい
owner/config、data、hardlink、symlink、mode、numeric UID/GID、idmap、重複拒否、独立した
変更、保存元削除、canonical な所有対象 cleanup を確認しました。pool は marker と空を
確認して削除し、元 archive と plan は `/var/lib/haco-owned-import-4138767719` に残しました。
`HACO_E2E_INCUS_VOLUME_IMPORT=1` で実行でき、既存 Incus GHA job にも追加しました。

Store/import と native 準備の focused race test は1.052秒／1.057秒で成功し、vet も
成功しました。最初の fixture build は複数行文字列の構文で失敗し、修正しました。
rootfs/Workspace 一式の import、Env 起動、接続時の実 idmap shift、live OCI daemon は
未検証・未実装です。公開 import の実経路受入は pending のままです。

ネイティブ呼び出し境界の回帰テストでは、所有済み・他所有者の対象、不正・切り詰め済みの一覧、
一覧取得失敗、native の非ゼロ終了、応答喪失も確認しています。import 呼び出し時点の新しい
所有 metadata と、成功・失敗の両方で匿名入力を閉じることを確認し、対象 race テストは
2.359 秒で成功しました。PR 初期 head の既存 Incus GHA でも owned volume import step は
成功しましたが、PR 全体の green を意味しません。

下記 version 2 の実装は、不足していた Workspace 登録 metadata を補います。version 1 の
転送形式は順序付き archive を保持しますが、repository 名・remote・branch の対応は持ちません。
現行の管理 Workspace service はこの対応を必要とし、ゲスト内の Git config を暗黙に信頼済みの
broker 接続先として採用してはいけません。これは公開 import の残実装であり、既存 bundle の
喪失や読み取り不能を意味しません。既存の検査・component 読み取りは引き続き利用できます。

## 新しい export の Workspace 接続先情報

Status: 公開 Linux export に **implemented**。一式の import の実経路受入は pending です。
新しい公開 export は envelope version 2 を使い、保護された保存済み binding から順序付きの
Workspace 名・remote・branch を含めます。role が各情報を native archive 1個に対応付けます。
名前の重複、認証情報付き・未対応の接続先は既存 Git validator で拒否し、remote は4096 byte、
manifest 全体は既存の64 KiB上限です。staged metadata のコピーから検証済み状態は変更できません。

これらはデータであり、権限の移譲ではありません。元 owner・承認・認証情報・native path は
追加しません。`file:` remote は保存元 Host の情報であり、復元先 Host のファイルアクセスを
許可しません。古い保存物にない接続先は空のまま保持します。
[ADR 0052](../adr/0052-transfer-routing-metadata.md) を参照してください。

操作は `haco env export <stopped-env> [file.haco]` のままです。既存 version 1 bundle の
完全検査・component 読み取りは維持し、保存済みデータの移行・書き換えは不要です。
version 2 未対応の古い reader は新しい export を明示的に拒否するため、更新した Hacocoon で
読み取ってください。公開 import には新 Workspace 登録、欠落・ローカル接続先の扱い、
rootfs import、Env 起動の実装が残っています。

対象の transfer・Router・Incus metadata テストは0.612秒／0.089秒／0.298秒で成功し、
composition は対象テストなしで compile を確認しました。文書検証も成功しました。最初の
build は Router の中継不足で失敗し、混在 route の回帰を追加してから再検証が成功しました。
version 2 export の全体 CI と native 受入検証は実行待ちです。

## native Workspace 登録

Status: GitHub 接続先または offline を明示した単一 Workspace について **内部実装済み**です。
`RepositoryService.ImportWorkspace` は通常の所有予約、created／inspect／ready 公開を
再利用し、Git によるデータ準備は実行しません。native volume import は OCI と同じ上限付き
Incus archive 準備処理を使い、新しい Workspace config を作成前に設定します。既存対象は
取り込み前に拒否します。clone・checkout・remote 通信・guest hook・認証情報操作は行いません。

この内部登録では元の local-file URL は拒否します。明示的な空の接続先と複数 Workspace の
統合は下記のとおり内部対応済みです。公開 metadata の対応付けと再接続は planned です。
失敗時の記録保持と完成済み単一 volume の cleanup は下記を参照してください。
[ADR 0053](../adr/0053-workspace-native-import.md) を参照してください。catalog schema、
通常の clone／copy、ready Workspace の削除経路は変更しません。

対象 service／native 境界テストは0.427秒／0.536秒で成功し、import 前の所有記録、Git 準備を
呼ばないこと、重複拒否、失敗記録の保持を確認しました。既存 native volume E2E に、保存元
volume 削除後の Workspace 登録、commit・未commit・untracked ファイル、guest Git config の
保持、独立した管理接続先と所有対象 cleanup の確認を追加しました。専用 Incus/Btrfs の
`b7c7ec5` は20.67秒で成功し、canonical な Workspace 登録・削除と隔離2 pool の削除を
確認しました。元 archive と fixture plan は `/var/lib/haco-owned-import-1590527782` に
保持しています。事後確認で既存9 instance、sentinel checksum、登録ファイルの mode／link数は
変更されていません。

同じソースの全 Go テスト・vet・docs／workflow policy・JS 27件は成功しました。対象 Git service
race は1.568秒、native adapter race は2.541秒で成功し、ローカル検証の全呼び出しが正常終了しました。Env 接続時の idmap shift、
boot、SSH、live OCI daemon、複数 Workspace import、公開一式のコマンドの成功は主張しません。

## native Workspace import 失敗時の cleanup

Status: 作成完了済み・未公開の単一 Workspace について **内部実装済み**です。
失敗した import の戻り値と読み直した registry が完全一致し、`created` の場合だけ掃除します。
既存 native 削除経路が所有者・利用者・保存済み子要素を確認し、実体の不在を確認してから
registry を削除します。cleanup はクライアントのキャンセルから独立した期限付き context で
実行します。掃除できても import 自体は失敗として返し、ready Workspace にはしません。

`creating` は native 要求の応答未確定、`ready` は公開エラー後に利用者がいる可能性があります。
どちらも自動削除しません。所有情報の変更や cleanup の不確定時は正確な receipt を保持・返却し、
recovery-required とします。自動再開・隠れた backup・新しい状態は追加しません。
[ADR 0054](../adr/0054-completed-import-cleanup.md) を参照してください。

cleanup 完了・失敗、所有者変更、公開済み、キャンセル、作成応答不明の対象 race テストは
2.072秒で成功し、vet も成功しました。既存実 Incus volume-import gate に、作成後の検証失敗、
cleanup 失敗、native 応答喪失の注入を追加しました。2917714 の専用 Incus/Btrfs 検証は21.65秒で成功し、
全 Go・vet・docs・workflow policy・JS と対象 race（1.928秒）も成功しました。
archive と receipt は /var/lib/haco-owned-import-490401733 に保持しています。
作成不確定・cleanup 失敗の receipt は残し、試験専用 teardown は作成完了を把握した native fixture のみ削除します。
本番の不確定 cleanup の成功は主張しません。既存9 instance・sentinel checksum・登録ファイル属性の不変も確認しました。
作成要求の不確定と公開後 aggregate cleanup は残課題であり、すべての未完了 Workspace を
修復する API ではありません。

## 複数 Workspace の native 登録

Status: 明示的な GitHub 接続先または offline の2〜8 archive について **内部実装済み**です。
通常の collection 作成と同じ遷移で、全 member の新しい所有情報を単一記録へ予約し、
native import の完了を永続記録・検査してから全体を公開します。Git の populate は実行しません。
member 単独の参照用記録は作りません。native 実体は別々であることを要求し、不正な入力は
予約前に拒否します。volume import 自体は Incus に任せます。

部分失敗では全 collection と完了済み・作成不確定 member の所有記録を残します。
一部だけ公開したり、不確定 import を再実行したり、単一 volume 用 cleanup を適用したりしません。
未完了 collection の明示的 cleanup は公開 import 前の残課題です。ready collection の削除は
既存の lease 排除・所有対象確認を使います。catalog・schema・状態・CLI・元の権限は追加せず、
既存データの移行も不要です。ADR 0053 を既存 collection モデルで拡張しています。

b7dca44 の全 Go・vet・docs・workflow policy・JS 27件と対象 race（2.782秒）は成功しました。
専用の実 Incus/Btrfs 検証も29.64秒で成功し、保存元削除後の archive からの独立2コピー、
Git・untracked データ保持、member 単独記録の不在、canonical な collection 削除を確認しました。
collection の部分失敗時の保持は service テストで確認し、native への失敗注入では未検証です。
公開一式の import 受入は未確定です。offline 接続先、rootfs と Env 起動は実装済みです。

## offline Workspace データ

Status: **内部実装済み**です。remote と branch が両方空なら offline Workspace として登録します。
片方だけの指定は不正で、import 元 Host の file URL は引き続き拒否します。架空の Host repository、
承認、認証情報は作りません。混在 collection でも offline member のデータを保持し、Git broker は
接続先のある member だけを扱います。online binding は現在の Host source と remote・branch の
一致を要求します。接続先がない snapshot Workspace のコピーも offline で復元できます。

既存 catalog の項目と元 archive は書き換えません。schema・CLI の変更も不要です。
offline member は無関係な同名 source repository の削除を妨げず、データは通常の所有確認付き削除で守ります。
[ADR 0055](../adr/0055-offline-workspace-routing.md) を参照してください。公開 import での metadata の
対応付けと rootfs import・一式の起動は実装済みで、再接続は planned です。634590d の全 Go・vet・docs・
workflow policy・JS 27件と対象 race（Git 11.451秒、Incus 2.386秒）は成功しました。
実 Incus/Btrfs の混在 collection import と native attachment metadata 検証も29.52秒で成功しました。
offline snapshot copy と broker 拒否は component／service テストで確認し、実機の offline snapshot
restore、接続・起動済み Env、live Git／OCI は未検証です。

## native rootfs image import

Status: **内部実装済み**です。実 Incus/Btrfs transport 検証は22.28秒で成功し、元 instance／image の削除と
一時 image 削除後の復元先データ保持を確認しました。boot・SSH・公開一式の import は未検証です。上限付き匿名 archive で rootfs データを保持し、
image properties は新しい import owner に置き換え、image 作成時 template は除きます。
元 archive は変更しません。Incus が統合 container image を取り込み、同期 consumer が現在の
明示的な設定で独立 instance を作ります。一時 image は export と共通の所有・不在確認で削除します。
作成・cleanup 不確定時は receipt を残し、Base や backup は追加しません。

固定 SDK の context を保持する raw operation で upload・完了待ちを行い、選択済み local Unix
接続・project と応答上限を維持します。現時点では非圧縮の統合 x86_64／aarch64 container image のみ対応します。
[ADR 0056](../adr/0056-native-rootfs-import.md) と次節の canonical 作成を参照してください。公開一式の
import、SSH 実ハンドシェイク、live OCI 整合性は残課題です。

## archive から Environment へ

Status: **内部実装済み・native 起動確認済み**です。Workspace service の CreateFromArchive は、
呼出元が用意した Workspace／OCI を canonical な Env 作成で接続し、現在の Host OCI の自動コピーを省きます。
Incus adapter は所有確認付き一時 image から独立 instance を作り、直ちに所有記録を保存した後で、
現在の sandbox 設定と guest SSH identity の再生成を行います。native init の完了不明時は lease を保持し、
cleanup は接続データを削除しません。CLI・catalog 移行・Base 実体・自動 backup は追加しません。
一式の orchestration、公開利用経路、SSH 実ハンドシェイクは未検証です。

通常の BaseRouter は native archive を共通の所有記録プロトコルで Incus へ渡します。
既存 aggregate E2E に、旧 Env 削除後の export bundle をこの router から import し、
実起動・明示した Workspace／OCI の接続・世代と管理 SSH 権限の更新・削除後のデータ保持を
確認する処理を追加しました。47bf7a8 の専用 Incus/Btrfs 実行は439.76秒で成功し、既存の capture／restore／export と
native data cleanup も確認しました。CLI バイナリを渡さないローカル実行では公開 CLI を SKIP し、GHA は同バイナリを渡します。
共有 image の削除も SKIP しました。公開 import、SSH 実ハンドシェイク、live OCI 整合性はこの検証では証明しません。

## native bundle import の接続

Status: **内部実装済み・native bundle 起動確認済み**です。bundle 全体を変更前に検証し、単一／複数 Workspace、
その新しい Workspace への永続的な対応付けを持つ OCI、canonical lifecycle による新規 Env の作成・起動を
順に行います。既定の復元先は SOURCE-imported で、既存名は拒否します。公開 CLI／controller upload は接続済みです。下記の Linux import コマンドを参照してください。

version 2 は repository 名と GitHub の接続先 metadata を保持しますが、認証や承認は付与しません。
元 Host の file URL は offline として扱います。version 1 は接続先 descriptor がないため component 名で
offline 登録します。guest の Git データと元 bundle は変更しません。Workspace は既存 collection 上限の8件までで、
超過は native 変更前に拒否します。長い repository 名はそのまま保持し、内部 native ID を新しい member owner から決めます。

native 作成不明時は既存の所有記録を保持します。Env 作成失敗時は既存所有 API により、公開済みで lease のない
今回のデータだけを cleanup し、OCI cleanup 不明時は対応する Workspace も保持します。起動失敗時は Env とデータを残します。
schema 変更・自動 backup・Base component・import 専用復旧 catalog は追加しません。
[ADR 0057](../adr/0057-native-bundle-import.md)を参照してください。公開 import、再接続、SSH 実ハンドシェイク、
live OCI 整合性は未完了です。

6360a23 の専用 Incus/Btrfs aggregate は558.35秒で成功しました。保存元削除後の rootfs・2つの
Git Workspace・OCI の独立 import、Env 起動、世代確認、Env 削除後のデータ保持、所有対象の cleanup を
確認しました。ローカル実行は CLI バイナリ未指定のため公開 CLI 検証を SKIP し、共有 image の削除も
SKIP しました。公開 import、SSH 実ハンドシェイク、live OCI 整合性は未検証です。

## 管理接続の import 転送

Status: **Linux 管理接続に実装済み** です。型付き `environment.import` stream は
任意の復元先名と最大64 KiBの data frame を受け取り、明示的な終端の byte 数と SHA-256 を確認します。
既存 importer が全入力を staging・検証してから native 変更を行います。client 指定の Host path、
owner、OCI kind、上限は受け取りません。export と同じ64 GiBの payload 上限と上限付き envelope を使います。

upload 後の切断・追加入力は起動処理を取り消します。応答は失敗時に保持した資源名を含む既存 import result
を返します。成功には転送 count/digest の一致、復元先の running、終端 EOF が必要です。操作は30分、
upload の読み書きは30秒の待機期限を設けます。呼出元の入力 reader は完了または取消可能である必要があります。
自動再試行や backup は追加しません。製品 Linux controller の管理接続に登録し、guest／通知接続には公開しません。

## Linux import コマンド

Status: **partial**。CLI と製品 controller への接続は実装済みです。b7297a3 の専用実行で、製品 CLI と fixture controller
による native import・起動・データ保持・所有 cleanup を確認しました。インストール済み controller／desktop の
import は当時未検証でしたが、後続684e411で成功しました。aggregate 全体の完了は別に判定します。
bundle を読める Linux client から実行します。

```bash
haco env import dev.haco
haco env import dev.haco new-dev
haco env import --json dev.haco new-dev
```

必須入力はファイルだけです。既定名は保存元名に `-imported` を付け、既存 Env や未解消の lease は拒否します。
新しい Workspace／OCI コピーと Env を作り、現在の所有・セキュリティ条件で検証して起動します。
保存ファイルと既存 Env／データは変更しません。Base 実体や事前 backup は不要で、古い承認・接続権限は戻しません。

client は読取専用の通常ファイルを開き、末尾 symlink と特殊ファイルを拒否して bundle を検証し、内容だけを
upload します。controller は変更前に全入力を再検証します。パスの入れ替えで開いた descriptor は変わらず、
内容の同時変更でも controller の全体検証を回避できません。CLI の import 操作は30分の期限、payload は
共通の64 GiB上限を使い、容量やストレージの必須引数はありません。

失敗は非0で終了します。`--json` は保持資源名も返し、通常表示は stderr に表示します。切断によって最終結果が
届かないことがあるため、再試行前に保持資源を確認してください。自動再試行は行いません。file 接続先や旧形式は
 offline で import し、再接続は planned です。Linux／WSL が対象で、Windows native のファイル入力は未対応です。
trusted `haco-host` 内で実行する場合、ファイルはその client から読める必要があります。
SSH 実ハンドシェイクと live OCI daemon の整合性は別の受入項目です。

b7297a3 の初回公開 import aggregate は export・native import・restore に成功し、続く公開 copy で
fixture の12分期限に達して720.07秒で失敗しました。全体は FAIL であり、成功や SKIP ではありません。
所有 catalog と保存データは `/var/lib/haco-snapshot-aggregate-1920048809` に明示 cleanup のため残しています。
共有データは削除対象にしていません。増えた検証量に合わせて fixture を20分、GHA の test process を25分に
設定します。製品の期限・隔離は変更しません。b7297a3 の GHA 実 Incus/Btrfs
[aggregate step](https://github.com/SLktEx/Hacocoon/actions/runs/34455660292/job/102801320149)は、
製品 import CLI と全 aggregate assertion を含めて成功しました。ローカル失敗は保持し、GHA を独立した受入結果として
扱います。延長後の local-budget 版はコンパイル済みですがローカル再実行はしていません。b7297a3 の4 workflow は成功しました。後続の fixture 期限変更は、その最新 head の CI を別に追跡します。

## インストール済み controller と SSH の受入

Incus aggregate の製品 controller subtest は、空の専用 catalog を使って native rootfs／Git／OCI、
新しい世代、管理 SSH 鍵の初期化、Env 削除後のデータ保持、canonical な所有対象の cleanup を
確認します。診断は、通常ファイルのみを許す aggregate の receipt ディレクトリとは別に保持します。

| commit | 実際の結果 |
|---|---|
| a58d553 | controller subtest は20.35秒で成功。診断ディレクトリを最後の receipt 確認が拒否し、aggregate は89.56秒で失敗。 |
| 6d5e027 | SSH 準備で失敗し、ハンドシェイク未到達。aggregate は81.43秒で失敗。後続 cleanup は別 fixture 所有の残存物を正しく拒否。 |
| e598270 | sshd 不在と SSH 導入段階の失敗を確認。aggregate は86.49秒で失敗。 |
| 0cc27a5 | Windows transfer は seed-repository で失敗（exit 127）、VS Code は成功。fixture に通常のパッケージ導入による trusted Host と source Env の Git 準備を追加。再検証は未完了。 |

単独 fixture にはインストール済みの package egress サービスがありません。SSH 継続は、既存の
Windows 製品インストール gate で別の管理 Workspace／OCI を持つ source を作り、既存の限定した
package Policy で source SSH を準備して検証します。Windows OpenSSH で未push・未commit・untracked
の作業を作り、stop・export・source Env の削除後、trusted Host のクライアントとインストール済み
controller から import します。復元先へ source の package 許可は与えず、保存 rootfs の sshd を使います。
controller が返す新しいホスト鍵を固定した実 Windows SSH で Git・rootfs・OCI marker を確認し、
追加作業を保存します。Env 削除後、新しい Env に保持データを再接続し、公開コマンドで検証所有の
データだけを明示的に削除します。

この installed gate は **684e411 で成功**しました（下記の実行記録を参照）。単独 native controller gate
も必須のままです。既存 external-path Windows SSH fixture の対象は維持し、transfer の失敗を記録しながら
独立した desktop probe を続けます。CI を通すための黙った SKIP は行いません。bundle と生のローカル
検証 repo は確認用に trusted Host 内へ保持します。Windows native の bundle ファイル受け渡しと
live Docker／containerd 整合性は未検証です。製品コマンド、backend、Base component、自動 backup、
schema は追加しません。

7517c27 の transfer は Windows SSH 内の Git 導入で失敗しました（exit 100）。
684e411 の [Windows attempt 1](https://github.com/SLktEx/Hacocoon/actions/runs/34471376143/attempts/1)
では Git 導入、VS Code、export、source Env 削除、独立 import、新しい鍵を固定した Windows SSH、
作業再開、保持 Workspace／OCI からの Env 再作成、公開コマンドでの所有 cleanup が成功しました。
bundle と生の検証 repo は trusted Host 内の `/tmp/haco-transfer-4844a07f73644223` に残しています。
OCI は合成した永続 marker の確認であり、実 containerd／Docker workload の検証ではありません。
同 commit の対象 native Incus／Btrfs・通常テストの job はすべて成功しました。

Windows job 全体は、独立した承認待ち probe の
`prepare-python-prerequisite-setup-start-internal` で **失敗**しました（exit 1、cleanup_failed=false）。
直前の project setup と後続 preview／doctor は成功しましたが、原因は未解明です。
attempt 2 でも同じ段階で失敗し、transfer と独立した desktop 検証は再度成功しました。失敗直後に既存の鍵固定 SSH で DNS service の Result だけを読み、許可した定型値のみを出す診断を追加しました。setup の再試行・service 再起動・失敗判定の変更は行いません。
live OCI の整合性と Windows native の bundle 受け渡しは未検証です。

## 既存ドライブ共有を使う Windows bundle ファイル

Status: **実装済み・c4449e1 の installed GHA 受入成功**。[Windows run 34482712957](https://github.com/SLktEx/Hacocoon/actions/runs/34482712957) で確認しました。Windows gate は export 済みの Linux bundle を、
trusted Host の既存ドライブ共有を通して、新しい Windows 一時ファイルへコピーします。
排他的な新規作成で既存ファイルを拒否し、Windows 側で長さと SHA-256 を export receipt と照合します。
source Env を削除した後、通常の Linux import client が共有経由の Windows ファイルを読みます。
import 後の作業・保持データからの再作成後にも Windows 側の digest を確認します。
確認用ファイルは SSH fixture の cleanup ディレクトリ外に保持します。

既存のファイル・管理インターフェースを使います。Windows ネイティブ haco や DrvFS への直接 export
公開の実装ではなく、export は対応 Linux filesystem 上で完了させます。手動では完成した bundle を
Windows フォルダへコピーし、その共有パスを Linux client の入力にする経路を想定しています。

```bash
haco env import /mnt/c/Users/USER/Backups/dev.haco dev-imported
```

Windows native の export／import コマンド、自動コピー、WSL 全体の退避は別の作業です。
ローカルのコピー回帰は既存対象を上書きしない確認であり、ドライブ共有経路は installed GHA で成功しました。保存元 Env の削除、Windows SSH による作業再開、保持データの再接続、最後の bundle 不変確認を含みます。native Windows CLI、直接 DrvFS export、別 WSL への復元の受入ではありません。

## 実 OCI データ転送の受入

Status: **fixture 実装済み・6974272 で native 受入成功**。既存 aggregate で、Store 受入と同じ固定版
containerd／nerdctl 資材を使用する検証を選択できます。今回作った所有確認済み source で
offline image を実行し、名前付きコンテナの書込 filesystem にファイルを書いて sync します。
コンテナの終了後に containerd と source Env を停止し、既存 aggregate export を実行します。

source 削除後、canonical importer と製品 controller の import で保存 image ID と実行 task 不在を
確認し、保持されたコンテナを明示的に起動して以前の内容を要求します。source registry、image pull、
旧 Base 実体、task の移行は使いません。bundle 不変性、所有・新しい世代・データ cleanup の既存確認も
維持します。fixture の native 準備は通常の installed Base／runtime 導入の受入ではありません。
Docker、BuildKit cache、任意のアプリ／DB 整合性は未検証です。この段階は固定版 containerd、
native snapshotter、停止コンテナのデータを対象にします。

ba4dbcd の実 OCI 検証は export 前の source runtime 準備で FAILED。fixture は生の subprocess 出力を出さず、固定の失敗段階と終了コードを示すようになった。所有情報の復旧記録は保持し、転送の検証成功とは扱わない。

オフライン source fixture では containerd transfer service に linux/amd64 の native unpack を明示設定します。標準の unpack 選択は native を含まないためで、source の準備だけに使います。復元先は起動前に現在の Hacocoon 設定へ置き換えます。8103e3f は export 前の image import で失敗し、CLI の platform 指定だけでは解決しませんでした。6974272 の [run 34501951826](https://github.com/SLktEx/Hacocoon/actions/runs/34501951826) では aggregate が 103.36 秒、製品 controller の import が 22.00 秒で成功し、元 Env の削除後も containerd の書込データから作業を再開できました。Windows を含む対象 CI は成功し、任意の authenticated-private-registry job は SKIP です。以前の失敗は失敗として残し、Docker・BuildKit/cache・任意のアプリ整合性の成功とは扱いません。

## 退避対象の native 一覧

G2 は **partial** です。`tools/evacuation_inventory.py` は Incus の project、pool、
image、instance、custom volume、保存済み snapshot を読み取り query だけで一覧化します。
既存の Incus 管理権限がある Physical Host 上で repository から実行する復旧用の補助です。
日常の `haco` コマンドは増やしません。

```bash
umask 077
python3 tools/evacuation_inventory.py > inventory.json
```

JSON は資源名と種類を含みますが、config 本文や認証情報は出力しません。instance の disk 対応には pool・mount 先・単純な volume 名または Host パスの参照を含め、参照先は開いたり追跡したりしません。任意の URI 本文は表示せず要確認とし、全ての対応に所有・外部データの確認を残します。不正 device があればその対応を不明と記録し、他の一覧は保持します。
取得に失敗した query の対象を残し、他の取得結果は保持します。終了コード 1 と
`native_queries_complete: false` は native query の未完了を示します。
project 間で同じ資源が見える場合があり、行数は独立した所有資源数ではありません。
この一覧は削除や復元の権限にはなりません。query は最大 256 回・全体で 5 分を上限に次の実行を判断し、各 query も 30 秒で打ち切ります。上限到達時は取得済みの行を残して未完了とします。

image の行には完全な fingerprint・種類・alias 名だけを記録し、description・properties・更新元 URL は出力しません。`image_source_project` は Incus の image 名前空間であり、Hacocoon の所有権ではありません。[Incus の project features](https://linuxcontainers.org/incus/docs/main/reference/projects/) に従い、`features.images` が未設定または false なら `default`、true なら query 対象 project です。不明な設定は共有元を不明として一覧を未完了にします。不正・重複・取得不能の image 行も、他の資源一覧を保持してエラーを記録します。image の参照は保存済み snapshot component ではなく、独立した保存 rootfs の復元に元 Base image の保持は要求しません。

Linux の回帰テストは inventory 21 件・file inventory 10 件が SKIP なしで成功しました。専用 WSL の実 Incus の読み取り比較も成功し、default の image 1 件・独立 project の 2 件の完全な fingerprint が直接 query と一致しました。default 以外の共有 project は回帰テストのみです。image の export・内容の保存・installation 全体の復元は実行していません。

`backup_complete` は常に false です。catalog の対応関係、controller／Policy 設定、
保護する trusted Host データ、手動追加・未登録ファイル、外部 pool／VHD と Windows の参照、
読み出し可否、整合性を保った保存、復元後の照合は unreviewed に残します。
全ファイルの列挙・export は未実装で、native 一覧の成功は WSL 全体の退避完了ではありません。
旧 WSL とデータは保持し、この処理では snapshot 作成・削除を行いません。
一覧ファイルも後で入替対象 storage の外へ保存する必要があります。

専用 WSL の実 Incus で、2 project、1 pool、instance 13 行・volume 55 行の取得が成功し、
query エラーはありませんでした。private な一覧はその WSL 内の
`/var/tmp/haco-evacuation-inventory-tb_t97dj/inventory.json` に残しています。
外部への backup や snapshot 削除失敗時の退避を実証したものではありません。

拡張後も専用 WSL で instance 13 行・disk 対応 26 件を query エラーなしで取得できました。private な一覧は /var/tmp/haco-evacuation-inventory-hp1r9_bs/inventory.json です。参照先は記録するだけで開かず、外部 backup や所有確認の成功を意味しません。

一覧には instance の disk 対応に加え、pool の保存元参照と volume の `content_type` を含めます。パスは利用者が確認する参照情報であり、この補助は参照先を開いたり、背後の VHD を特定したり、所有権を推定したりしません。block volume を通常の filesystem tree として扱わないでください。URI 形式の参照は埋め込まれた認証情報を出力しないため伏せます。不正な pool 参照があっても他の一覧を残し、不完全と記録します。対象テスト 11 件に加え、専用 WSL の実 Incus 確認も query error なしで成功しました。Btrfs の保存元パス参照 1 件と filesystem の内容種別を確認し、私有の一覧を `/var/tmp/haco-evacuation-inventory-zcdndzfm/inventory.json` に残しています。外付け／block storage は metadata のテストのみで、実 block volume の退避は未検証です。

任意で `--catalog /var/lib/hacocoon/state/environments.json` を指定できます
（実際の controller root に合わせてください）。Linux 専用の reader は既存の通常ファイルを
末尾 symlink を追わずに開き、特殊ファイル・サイズ超過・読み取り中の変更を拒否します。
catalog の移行処理は呼ばず、lock や catalog を書き込みません。参照 projection は schema 10 から 13 を番号変更なしで扱い、
9 を含む他の schema は明示的に不完全と報告します。永続資源・Base asset・Workspace lease・snapshot
component の native 参照と owner／generation の情報だけを抽出します。Incus 側の所有者との
一致は検証せず、復元や削除の権限を与えません。`projection_complete` は選択した項目の
読み取りが完了したという意味だけです。repository catalog・進行中操作・source path・
設定／Policy／認証情報・手動データは引き続き確認が必要です。不正な行があっても他の結果を
残し、exit status 1 とします。native query と catalog 抽出は別の結果を持ち、
`backup_complete` は常に false です。入力の digest は読んだ bytes の識別用であり、
真正性や移行の承認を証明しません。このファイルから Env を構成することはありません。

検証は Linux で 15 件成功しました（Windows は 13 件成功、Linux 専用 2 件を SKIP）。専用 WSL の既存 catalog は schema 4 で、抽出対象の 4 区分を持たないため、実 catalog の読み取り確認は明示的に失敗しました。移行は行わず、失敗の記録を `/var/tmp/haco-catalog-inventory-iuzdd46h/catalog.json` に残しています。その後、実 Incus aggregate テストが残した `/var/lib/haco-snapshot-aggregate-1920048809/state.json` の読み取りは成功しました。snapshot 3 件・永続資源 3 件・Base asset 1 件・Workspace lease 2 件、計 9 件を抽出し、error はありません。私有の記録は `/var/tmp/haco-catalog-inventory-s5o_tdw4/catalog.json` です。実 schema-13 の参照抽出の確認であり、native 所有者との照合や全データの保存ではありません。既定の schema-4 catalog は空で、legacy 移行は行っていません。

`--repositories /var/lib/hacocoon/state/repositories` で別管理の repository 記録を含められます（controller root に合わせてください）。Linux reader は directory を固定して再帰せず、repo／work 記録と collection member の native 参照を抽出します。remote URL と認証情報は出力しません。子 symlink、ファイル名と記録の不一致、未知の項目、サイズ超過、不正な member は明示的な欠落として残し、他の結果を保持します。観測中に directory が変わる可能性があり、全体の atomic snapshot や完全な backup ではありません。全量保存には停止・整合性確認が必要です。一覧の回帰テストは 17 件となり、実運用の repository directory に対する受入は未検証です。

未確認の repository 項目には error 番号と directory 相対のファイル名を残し、内容は開きません。手動ファイル・リンク・不正な記録を利用者が特定できるようにするためで、一覧自体も private な metadata として扱います。保持していた native aggregate directory では repository 3 ファイルから 9 参照を抽出しましたが、他の 4 項目が未確認のため終了結果は失敗です。記録は `/var/tmp/haco-repository-inventory-icbsgg5s/repositories.json` にあります。この部分取得を directory 全体の受入成功とは扱いません。

## snapshot 操作が使えない場合の読み出せるファイル

G2 のファイル退避は **partial** です。Incus 6.0.5 の Btrfs
[BackupVolume 実装](https://github.com/lxc/incus/blob/v6.0.5/internal/server/storage/drivers/driver_btrfs_volumes.go)
は、非 optimized の filesystem volume 保存でも一時的な read-only snapshot を作ります。
`--volume-only` はこの内部依存を除きません。通常の G1 export は Incus backup を利用し続け、
snapshot に依存しない退避経路とは区別します。

opt-in の `TestRealIncusReadableDataEvacuationE2E` は、新規・所有確認済み・未接続の
Btrfs custom volume 二つの間で GNU tar の保存と復元を検証します。既存 volume 転送 fixture を
再利用し、Git commit／status／untracked、内容、hardlink、symlink、mode、数値 UID/GID、
user xattr、archive 不変性、保存元削除後の独立性を確認します。archive は両 pool の外にある
fixture の private directory に保持します。保存には Incus export・snapshot コマンドを使わず、
既存の利用データや pool は選択しません。

専用 WSL の実 Incus/Btrfs で 20.59 秒で成功しました。試験用 pool 二つは cleanup し、archive と所有記録は両 pool の外にある /var/lib/haco-volume-transfer-2051477010 に残しています（WSL 内です）。書込がないファイルのコピーを検証する基本部分です。
snapshot 削除失敗の模擬、全インストールファイルの列挙、live OCI daemon の移行、任意の
非信頼 tar の安全な import、trusted Host データの保全、WSL 外への保存、新 WSL 復元は
未確認です。全量退避を主張する前にこれらを扱い、読めない・変更中の source を完全保存済み
とはしません。

a16f3b1 では対象 CI がすべて成功しました（任意の private-registry job は SKIP）。
実 GHA の直接ファイル退避テストは [run 34498433003](https://github.com/SLktEx/Hacocoon/actions/runs/34498433003) で 0.84 秒で成功しました。
その後の手動確認では、合成データのアーカイブ 2 個を Windows Temp の新規ディレクトリ
`C:/Users/gddro/AppData/Local/Temp/haco-readable-evacuation-egpq6g7c` へ排他的にコピーしました。
元ファイルのコピー前後と保存先の SHA-256 が一致し、Windows 側でもサイズと hash を独立に確認しました。
`work.tar` は 112640 bytes、`oci.tar` は 10240 bytes です。最初の Windows 結果表示は
constrained language mode の制限で失敗し、通常の文字出力による再確認は成功しました。
`receipt.json` をアーカイブと一緒に残しています。これは合成データの WSL 外への保存の確認であり、
全量 backup、保護した認証情報の持ち出し、別 WSL への復元は未検証です。

追加の `TestRealIncusSavedReadableDataEvacuationE2E` は、準備時に所有する native volume snapshot を作り、live volume から marker を削除します。直接 tar 保存は既存 snapshot tree を読み、snapshot にしか残らない marker を新しい所有 volume へ復元します。通常の live volume の検証は別に維持します。snapshot 作成は準備段階だけで、保存処理では作成・export・削除しません。snapshot 削除失敗の模擬や、全 rootfs／Workspace／OCI の保存対応の確認ではありません。専用 Incus/Btrfs で 24.52 秒で成功しました。所有する両 pool は片付け、アーカイブと所有記録を `/var/lib/haco-volume-transfer-2483709670` に残しています（両 pool の外、WSL の内）。保存処理中に元 snapshot は変更していません。全量退避や新 WSL への復元を確認したものではありません。

b8ef557 の native GHA は、直接ファイル退避（0.89 秒）と saved-only 退避（1.87 秒）に成功しました。同 head の Windows SSH は native client の5分タイムアウトで失敗しました。現在の branch は、別途検証済みの main の SSH 進捗診断を含み、最新 head 自身の CI が必要です。以前の Windows 失敗を成功へ変更したり、原因が判明したことにはしません。

## 読み出せるデータの暗号化転送

状態: **過去の任意受入記録（historical）**。以下は以前の暗号化 fixture の記録です。通常 export と現在の退避手順は暗号化しない archive を使い、鍵準備や export 後の暗号化は不要です。[ツリー保存](#明示したデータツリーの保存)を参照してください。

この過去の fixture は、確認済みで書込のない file tree に既存の GNU tar と
[age](https://github.com/FiloSottile/age) を使い、Hacocoon 独自の暗号形式を作らず検証しました。
復号の秘密 identity は trusted storage に保持し、暗号化では公開 recipient だけを使います。
実検証 `tools/test_encrypted_evacuation.py` は合成の認証情報を使い、tar と age の終了状態を
両方確認します。外部保存先に平文 archive は書きません。自分で作った fixture だけを、
復号成功の確認後に展開し、private な mode、誤鍵・暗号文の改変・切詰めの拒否を確認します。
失敗または途中の復号出力を直接 restore へ流してはいけません。

この任意の過去 fixture だけを再現する場合は、停止・確認済みの source tree を SOURCE、入替対象 WSL／pool の外の新規 archive を DEST、
age の公開 recipient を RECIPIENT に設定します。対応する秘密 identity は別途アクセス可能に保持します。

```bash
umask 077
set -o pipefail
set -o noclobber
env -u TAR_OPTIONS tar --one-file-system --acls --xattrs --numeric-owner --sparse -C "$SOURCE" -cpf - . |
  age --recipient "$RECIPIENT" > "$DEST"
```

pipeline 全体の終了状態を確認し、出力ファイルの存在だけで完了としません。
`--one-file-system` が飛ばす mount は別の確認済み保存が必要です。部分出力は調査用に残し、
source は削除しません。trusted な私有 staging で復号を最後まで終え、成功を確認してから
展開を検討します。任意の非信頼 tar の安全な importer ではありません。実認証情報、全量、
WSL 削除後の鍵の確保、暗号化データの新 WSL 復元は未検証です。実 script は
`HACO_E2E_ENCRYPTED_EVACUATION=1` で opt-in し、`HACO_E2E_ENCRYPTED_OUTPUT_ROOT` に
既存の外部保存先 parent を指定すると、その下の新規テスト directory を使います。

専用 WSL では age 1.2.1（配布 package `age_1.2.1-1build1_amd64.deb`）で 1.03 秒で成功しました。直接の package 導入は libc6 の dpkg 設定中断により失敗し、その後 apt で取得した package を私有の tool directory に展開して、システムの package 状態を変更せず検証しました。暗号文と記録は 新規の Windows 出力 directory に残し、Windows 側でも 10440 bytes と SHA-256 `bf25c5334464947c0ea0c8645ecc535195cd930dad98efd18550243323eb5804` を独立に確認しました。合成 source・復元データ・試験 identity は当初 WSL の一時 fixture 内にありました。その後の読取確認では、その正確な directory は通常側にも現在の Incus mount namespace 側にもなく、当該 WSL の `/tmp` は tmpfs でした。Windows の暗号文と digest は保持していますが、元 identity は現時点で確認できません。転送前の確認は失敗し、鍵の転送・復号は未実行です。この暗号文の新 WSL 復元は SKIP とします。削除の正確な時刻・原因を確定した結果ではなく、別の鍵ではこの archive を復元できません。

55be427 の既存 native GHA [job](https://github.com/SLktEx/Hacocoon/actions/runs/34508162748/job/102975354767)
は成功し、実 tar／age テストは 0.029 秒で成功しました。この結果は当該 commit のもので、
rebase 後の最新 head は改めて CI が必要です。

## 新 WSL へのデータ復元の受入

状態は **partial G3** で、暗号化 identity の回復とは別です。公式キャッシュ image の
SHA-256 を現在の Microsoft 配布 metadata と照合し、別名の新 Ubuntu 26.04 WSL を
作成しました。既存 WSL は残しています。標準の
[WSL import](https://learn.microsoft.com/en-us/windows/wsl/use-custom-distro) を使い、
旧 WSL の filesystem や Incus DB を丸ごと戻す方式ではありません。

Incus 6.0.5-8 で、先に Windows へ退避した合成 Workspace／OCI tar archive を、新規の
1 GiB Btrfs pool の明示的に所有する custom volume へ復元しました。展開前に期待する
SHA-256 を照合し、既知の試験 archive だけを使用しました。GNU tar の比較と個別確認で、
内容・数値の所有者・権限・hardlink・symlink・user xattr が一致しました。保存 Git HEAD と
untracked file が残り、復元先で新しい local commit を作成できました。8.04 秒で成功し、
元 archive は変更していません。その後、復元 Workspace volume に新規 snapshot を作成し、
その snapshot の削除と消失確認にも成功しました。復元 volume と所有記録は確認用に残しています。

インストール済み Hacocoon の import、Env の権限・network・credential 再構成、実 OCI
アプリ状態、新 WSL 上の saved-only data、全量の網羅、WSL 入替は証明していません。
暗号化の秘密 identity の移送は未実行です。残る確認を終え、復元結果を確認してから
削除する旧 WSL を選びます。この部分結果を旧データ削除の根拠にしてはいけません。

### インストール済み controller と復元 Workspace

新規 Ubuntu 26.04 WSL・Incus 6.0.5・ローカル ad80acc controller candidate で、合成の外部 Workspace を復元し、通常の `haco env create` により新しい既定 OCI Store 付きで作成できました（40.88 秒）。source guard の拒否を観測した後、通常の stop/start は成功しました。SSH の package 準備は最初 exit 100 で失敗し、`haco config` で試験 Env の Ubuntu 配布先だけを許可した後は通常準備（84.69 秒）と固定 Host 鍵の SSH による Workspace 読書き（1.68 秒）に成功しました。Env 内に管理 socket はありませんでした。

同じ SSH/package 経路で Git を導入し、復元 repo にローカル commit `cafa5fc` を作成しました。続く複合検証は、新 Store に containerd directory が既にあると仮定したため失敗しており、live OCI 成功とは扱いません。別の検証で所有 Store の mount を確認して marker を書き、通常の stop/delete/create 後も Store の同一性と marker、Workspace の同一性、Git commit、modified/untracked、所有権・permission・link・xattr を保持しました。同名 Env の世代は変わり、旧 SSH endpoint は接続を拒否し、旧生成 config と authorized keys は存在せず、Env rootfs 内だけの marker は消えていました。通常 lifecycle/SSH に native readback を併用しています。

新規インストール上の合成外部 Workspace での開発と、新 Store の保持データの検証です。旧 live OCI daemon、全 managed repo/Store 対応、実認証情報、既存暗号 identity の回復は対象外で、旧 WSL 削除を許可する確認ではありません。揮発性 guard より先に Incus が自動起動する問題は別の[起動境界修正](../adr/0060-explicit-environment-start.md)で扱い、その cold-boot 受入は別途記録します。


## snapshot 削除失敗後の読み出し退避

状態: **G2 の一部**。明示実行の
`TestRealIncusFailedDeleteReadableDataEvacuationE2E` は、新規の所有 Btrfs pool と
合成 Workspace／OCI データだけを使います。snapshot の親ディレクトリだけを
immutable にして、実 Incus の snapshot 削除が EPERM で失敗することを確認し、
その状態のまま GNU tar で保存済みファイルを取り出します。既存の比較で Git 状態、
snapshot にだけ残るデータ、リンク、数値所有者、mode、xattr を確認します。
別の新規 pool に復元し、保存元・archive の独立性と所有対象だけの cleanup を確認します。

属性変更前に、private な fixture 記録へ親のパスと inode 識別情報を保存します。
cleanup は同じディレクトリと確認できた場合だけ属性を解除し、失敗時も記録を残します。
これはテストだけの障害注入で、利用者向けの復旧手順や製品の削除保護の変更ではありません。

隔離 Ubuntu 26.04 WSL の実 Incus 6.0.5／Btrfs による手動検証は成功しました。
削除が EPERM で失敗し、保存済みデータの取得・別 pool への復元・独立した編集が成功し、
cleanup 後に両テスト pool の不在も確認しました。10240 byte の archive と記録は
両 pool の外に保持しています。読める状態の削除失敗の一条件だけの検証であり、
Btrfs 破損、全量網羅、WSL 外への転送、旧 WSL 入替は未完了です。
既存 native GHA gate にこの回帰を追加します。その結果は手動検証や通常の package
テストと区別して報告します。

追加した Go 回帰も同じ隔離 Incus／Btrfs で 11.74 秒で成功しました。合成 Workspace と
OCI の両 volume を確認し、所有記録と archive は `/var/lib/haco-volume-transfer-56267304`
に保持しています。adapter 全体のテストは 19.17 秒で成功し、文書整合性と workflow
policy 回帰 25 件も成功しました。最新 head の GHA 受入とは区別します。

## 退避確認のための手動追加ファイル一覧

状態: **G2 の一部**。既存の読み取り専用 `tools/evacuation_inventory.py` に
`--files /absolute/root` を付けると、controller catalog にない可能性のある
ディレクトリ metadata を列挙します。専用 WSL の root は `/`、個別の確認済み
データ領域はその絶対パスを指定します。ファイル名や所有情報自体も機密になり得るため、
結果の JSON は private に保存してください。

Linux reader は相対名、種類、mode、数値 UID/GID、size、inode/device/link 数、更新時刻を
記録します。開くのはディレクトリと kernel の mount 情報だけです。通常ファイルの内容、
symlink の参照先、ACL、xattr 値は読みません。指定 root の途中の symlink も辿りません。
子ディレクトリは descriptor を保持して観測 inode と照合し、descriptor の mount ID で
一覧にまだなかった同一 filesystem の bind mount への進入も防ぎます。

既知の mountpoint は相対パスだけを記録し、切断された外部 storage への stat も行いません。
mount 境界、symlink、特殊ファイルはすべて deferred として記録します。読取エラー、
ディレクトリ/root の置換、観測できたディレクトリや mount 一覧の変更、50,000 entry／
64 階層／60 秒の上限では途中結果を残し、`enumeration_complete=false` と終了値 1 にします。
未処理箇所は個別に確認し、追加の所有 mount root は明示指定して別途列挙してください。
時間上限は entry 間で確認します。kernel の metadata 読取が停止した場合に割り込む
実行時間保証ではありません。
Incus の storage mount が別 namespace にある場合は daemon の mount namespace 内で
実行します。Windows 外部参照のファイル内容を読んだり削除したりはしません。

一覧は atomic snapshot、内容の照合、所有権の付与、backup ではありません。
全 entry に分類・停止後の取得・復元後の内容と属性の照合が必要です。小さな指定領域を
列挙できても `backup_complete=false` を維持します。installation 全体、trusted な秘密情報、
旧 WSL 入替の確認にはなりません。公開 `haco` コマンドや自動復旧 state は増やしません。

検証: Linux のファイル列挙回帰 10 件、統合 inventory 回帰 18 件、workflow policy 25 件が
成功しました。最終版は専用 Ubuntu 26.04 WSL で 20.87 秒に 47,848 entry を観測し、
読取エラーはありませんでした（通常ファイル 39,061、ディレクトリ 5,050、mount 参照 17、
symlink 3,718、特殊ファイル 2）。17 mount 境界、全 symlink、両特殊ファイルは deferred のため、
列挙・backup とも未完了です。private report は WSL 内の
`/var/lib/haco-file-inventory-4kvt5eyb/wsl-root.json` にあり、外部 archive ではありません。
別の小さな合成の手動追加データ領域では内容を出力せずに全 entry を列挙できました。
どちらも installation 全体を分類・保存した結果ではありません。

## 暗号化受入 fixture の保持

root で明示実行する native tar/age テストは、合成 identity・source・復元データ・記録を
新規 mode 0700 の `/var/lib` directory に保存します。揮発性の `/tmp` を保持済みの証拠と
表示しません。鍵は mode 0600 で、内容の出力や転送は行いません。既存 native GHA step は
`tools/test_encrypted_evacuation_retention.py` を実行します。実暗号化テストを
`PrivateTmp=yes` の一時 systemd unit 内で実行し、unit 終了後も表示した private 証拠が
期待した権限で存在することを確認します。旧実装は表示した directory が消失し、
この実 systemd 回帰で失敗しました。

同じ installation 内での fixture 保持であり、以前の未確認 identity の回復、WSL 削除後の
復旧、製品の認証情報 backup ではありません。本番の復号 identity は storage 入替前に
独立してアクセスできる場所へ保管する必要があります。暗号文が残るだけでは復元可能とは
言えず、新しい合成テストによって旧暗号文を置き換えたり成功扱いしたりもしません。
公開 `haco` コマンドは増やしません。

専用 WSL の検証では、旧 fixture は実 PrivateTmp 終了後の確認で失敗し、修正版は
2.31 秒で成功しました。既存の暗号化・復号・異なる鍵／改変／切詰めの拒否と private
ファイルの権限を含みます。一時 unit にも wrapper の timeout より短い実行・停止上限を
設定します。新たに生成した合成 identity は当該 WSL 内にあり、以前の Windows 暗号文の
復号には使っていません。WSL 間の鍵転送は未検証です。

## 復号 identity を WSL 外へ保持する

別の合成受入で、標準の [age の公開 recipient 手順](https://github.com/FiloSottile/age/tree/v1.2.1)
を使いました。Windows age/keygen v1.2.1 は公式 Go module の固定版から checksum database を
有効にしてビルドしています。新規 Windows 復旧ディレクトリの ACL を現在ユーザーと SYSTEM
だけに限定し、秘密 identity はそこで生成・利用しました。専用 WSL へ渡したのは公開 recipient
だけで、既存 identity の読み出し・転送はありません。以前見つからなくなった identity と
暗号文の未解決結果は別に保持します。

WSL age で既知の合成保存 rootfs archive を Windows へ暗号化し、10440 bytes を保存しました。
Windows native age の復号結果は10240 bytes、SHA-256
`ab82a108262f499b89576c218bec974df10e31a56267d25bef7ccefbb2536e7f` と一致しました。
改ざんした暗号文は exit 1 で拒否し、0 byte の部分 staging ファイルは復元していません。
証拠と新しい identity は
`%LOCALAPPDATA%/Hacocoon/RecoveryTests/<fixture-id>`
に保持しています。手動の OS 間暗号化検証であり、全量 backup、実 credential 移行、任意 archive
の安全な import、WSL 削除後の復旧の証明ではありません。storage 入替前に独立してアクセス
できる identity を保護して保持し、完全な復号を確認してから復元します。暗号文の存在だけで
鍵が復旧可能とは判断しません。

## 保存 rootfs ファイルの直接退避

状態: **G2 の一部**。明示実行の `TestRealIncusSavedRootfsEvacuationE2E` は、
既存の保存 rootfs adapter で新規 Btrfs pool に独立した停止 Incus copy を作ります。
元 instance を削除後、保存 rootfs を GNU tar で読み出し、このテスト自身の archive だけを
新しい空 instance に展開します。取得中に image の publish/export や snapshot の作成・削除は
行わず、Base や image cache も使いません。復元先を変更しても保存物が変わらず、復元先が
新しい世代を持ち、元の設定を引き継がないこと、所有対象だけの cleanup と archive／記録の
保持を確認します。

これは合成した停止 rootfs のデータ検証です。任意 archive の安全な importer、起動できる
Env の復元、rootfs 削除失敗、全量退避の検証ではありません。Workspace／OCI の対応関係や
trusted credential は転送しません。既存の volume・削除失敗 gate と区別し、実機実行と
最新 head の CI 結果を記録してから受入を判断します。

専用 Ubuntu 26.04 WSL／Incus 6.0.5／Btrfs の受入は9.37秒で成功しました。
保存 rootfs の archive と所有記録は `/var/lib/haco-saved-rootfs-evacuation-476527331`
に保持し、archive の SHA-256 は
`ab82a108262f499b89576c218bec974df10e31a56267d25bef7ccefbb2536e7f` です。
所有する3 instance と pool は cleanup 後の不在を確認しました。元は新規の空 instance と
合成ファイル1件であり、実 credential やアプリのデータは選択していません。
実起動・任意 archive・全量復元は未検証です。package 回帰19.82秒、文書整合性、
workflow policy 25件も成功しました。

その合成 archive の Windows への別途転送も成功し、10240 bytes と同じ SHA-256 を
確認しました。archive と receipt は
`%TEMP%/haco-saved-rootfs-output-<fixture-id>` に保持しています。
内容は root ディレクトリと合成 `root/retained` ファイルだけです。保存 rootfs fixture の
外部保存を示し、全量 backup や新 WSL の管理対象 Env 復元の証明ではありません。

## 別 WSL への管理対象 bundle 復元

状態: **G3 の一部**。管理対象 Env bundle 1 件の実機受入です。
保持済みの合成 aggregate bundle を元 WSL から private な Windows directory へ複製し、
成分ごとに検証して、新規 installation の受入に使った別の Ubuntu 26.04 WSL へ渡しました。
インストール済み local controller candidate 61a26e3、Incus 6.0.5、Btrfs において、
通常の `haco env import` が171.27秒で成功しました。元 catalog・Incus DB・Base 実体・
承認・Host credential は引き継いでいません。

bundle は550415872 bytes、SHA-256 は
`0aaaf7ab43e183f4bc489b7c8f3b6da9c1d24ff7063e4501b0051ed6d0a47fba` です。
rootfs、Git Workspace 2 個、合成 OCI Store 1 個を含みます。Git 内部ファイルを含む
volume の全94エントリ（44・44・6）について、内容・種類・mode・guest から見える
数値所有者が保存時と一致しました。最初の Host 生 UID/GID との比較は、Incus が guest 0 を
Host 1000000 に対応付けていたため **失敗** しました。その記録を保持し、所有権の変更や
mapping の回避をせず、guest 内で再比較して成功しました。既存 native aggregate にも、
復元 Workspace／OCI の UID/GID・mode を guest 内で確認する検査を追加しています。

新 WSL の再起動後は Env が停止したままで、通常の guarded start が成功しました。
新しい所有者・世代・現在の device・`boot.autostart=false` を確認しています。
保存 rootfs の marker は import 後も残り、旧管理 SSH 鍵は残りませんでした。
Env 内に Hacocoon／Incus 管理 socket や Windows drive は公開されていません。

SSH 準備は最初に package exit 100 で **失敗** しました。通常の revision-bound
`haco config` で、この Env の新世代だけに Ubuntu package の一時許可4件を追加すると、
86.57秒で成功しました。新しい client 鍵と controller が返した host 鍵の固定により、
復元先 WSL から SSH 接続し、Git の導入と復元 Workspace 内でのローカル開発 commit が
成功しました。一時ルール4件は同じ API で削除し、不在を再確認しています。

同名 Env を通常の stop/delete/create/start で再作成し、同じ管理 Workspace collection と
OCI Store を再接続しました。両 Git HEAD、modified／untracked、新 commit、OCI marker
ファイルが残りました。Env 世代は更新され、旧 SSH 設定の取得と旧 port への接続は拒否されました。
通常 Base からの再作成なので旧 rootfs marker は失われます。受入後の新 Env は停止し、
永続データと両 WSL は保持しています。

検証済み bundle を復元先 WSL に配置した後の利用手順は、既存のままです。

```bash
haco env import backup.haco recovered
haco open recovered
```

OCI は合成ファイルであり、実 Docker／containerd のアプリ状態ではありません。
Git fetch/push は **SKIP** です。保存された example route は合成で、この実行には認証済み
検証 repository への接続がないためです。Git push は実行していません。
Windows native の SSH／VS Code、ACL／xattr／link、trusted Host のデータ・credential、
他の管理対象・未登録資源、保存 snapshot、installation 全体の照合は対象外です。
G2／G3 全体と確認後の G4 入替は未完了で、旧 WSL の削除を許可する結果ではありません。
Windows の private 証拠は `%LOCALAPPDATA%/Hacocoon/RecoveryTests/<fixture-id>`、
復元先記録は `/var/lib/haco-managed-cross-restore-<fixture-id>` に保持しています。

## Catalog 参照の照合

`--catalog` または `--repositories` を指定すると、読み取り済みの native inventory と選択した catalog 項目から `associations` を出力します。controller state を所有する Physical Host で実行し、controller root を変更している場合は実際のパスに置き換えてください。

```bash
umask 077
python3 tools/evacuation_inventory.py --catalog /var/lib/hacocoon/state/environments.json > inventory.json
```

該当ディレクトリがある場合は `--repositories /var/lib/hacocoon/state/repositories` を追加できます。schema 10〜13 の参照 projection は Env の runtime 参照、Workspace ID と所在、永続資源との対応、Base 名・revision の由来情報を含みます。所在パスは参照として記録するだけで開きません。Base filesystem、任意の設定本体、認証情報は含めず、URI 形式の Workspace 所在は出力を控えます。元の schema 番号を保持します。schema 9 とその他の未対応形式は拒否し、catalog の移行・書き換えは行いません。この projection は lifecycle state を検証せず、`state_validated` は false のままです。途中の restore、snapshot Workspace-copy、ephemeral-run の各 section は、内容を出さず `unprojected_records` に件数を記録します。これらの所有関係や処理途中のデータは別途確認が必要です。

native instance/volume の設定からは `user.hacocoon.owner` マーカーだけを記録します。マーカーの欠落は不明として扱い、不正形式は native 観測の未完了となります。照合は provider 内の参照と、長さ制限・正規 encoding を確認した既存の `haco-runtime-v1:runtime.incus:<base64url>` 形式を扱います。他 provider は未対応です。参照・マーカーの観測、資源の未観測、マーカーの欠落・不一致、複数 project の候補、native inventory の未完了、未対応参照を区別します。削除途中の snapshot も含め component と上位記録の状態を残し、比較の 4096 行制限に達した場合も明示します。

この観測は所有権、Env の世代、権限の証明にはなりません。保存 Base の instance 参照は native 所有マーカーと照合します。Env と lease の runtime 参照は `runtime-reference-observed` を報告できますが、世代・所有権の検証は未完了で、共有 project の候補をマーカー一致だけで選びません。`authority` は false、`review_required` は true のままです。終了コード 0 は要求した inventory/projection を読み取れたことを示し、全対応の一致や backup の保存を意味しません。報告は private に保持し、データ保存を計画する前に未解決行を確認してください。普段の `haco` コマンドは増やしません。

`associations.native_review` は、観測した instance と custom volume から逆方向にも参照を確認します。各 project の表示について種類、名前、該当する pool、所有マーカーを残し、`reference-and-marker-observed`、`unresolved-reference`、`no-supported-reference` を区別します。対応する参照が一つあっても、別の対応可能な参照に不一致があれば要確認のままです。列挙は4096行までで、省略は明示的なエラーにします。これは孤立資源や削除対象の一覧ではありません。未対応の参照形式、repository catalog の欠如、不完全な inventory により、正当な管理資源にも対応する参照が見つからない場合があります。image、native snapshot の子要素、外部・手動ファイル、filesystem にだけ残った資源は元の inventory と実 storage で引き続き確認が必要です。削除・退避する資源を自動選択しません。

## 明示したデータツリーの保存

状態: **partial**。通常の移行は `haco env export <stopped-env> [file.haco]` と `haco env import <file.haco> [new-env]` を使います。暗号化・recipient・鍵は不要で、export 後にも暗号化を挟みません。

確認済みで書き込みを停止した Linux ツリーを直接退避する保守用途では、`tools/evacuation_capture.py` が GNU tar を使います。普段使う `haco` コマンド、Incus snapshot、catalog state、復元機構は追加しません。すべての書き込み元を停止し、保存元の外に、実行者所有の空の mode-0700 ディレクトリを新しく用意します。

```bash
umask 077
mkdir -m 700 /absolute/private-capture
python3 tools/evacuation_capture.py /absolute/reviewed-source /absolute/private-capture --quiesced
```

通常の `data.tar` と、開始記録・標準形式の `data.tar.sha256`・完了記録を出力します。鍵生成や暗号化は行いません。archive と記録を選択した保管先にコピーし、そこで確認します。

```bash
sha256sum --check --status data.tar.sha256
```

保存元・保存先は symlink 成分を辿らず directory identity を固定し、前後の metadata を上限付きで調べます。不完全な列挙・別 mount・特殊ファイルは拒否します。ファイルの symlink は参照先を辿らず保存します。GNU tar は filesystem の数値 ID・mode・link・ACL・xattr・sparse file 情報を保存しますが、Incus idmap は変換しません。

完了には tar の成功、出力の同期、観測した保存元 metadata と directory／出力 identity の一致が必要です。任意の `--byte-limit` と `--seconds` は出力と tar 実行の上限です（既定値 64 GiB・900秒）。失敗は非ゼロで終了し、部分出力と開始記録を残し、完了記録は作りません。既存ファイルの上書き・自動削除は行わず、再試行には新しい保存先を使います。終了させるのは当該呼び出しが作った子プロセスだけです。

`--quiesced` は操作した人による確認であり、書き込み元の検出や atomic snapshot ではありません。archive 完了はアプリ整合性・外部保管・全量 backup の証明ではなく、`backup_complete` は false のままです。入替前に復元内容を別途照合してください。checksum はコピー破損を検出しますが、archive と checksum 両方の置換には真正性を保証しません。記録には保存元の場所が含まれるため非公開で保持します。保存物の管理権限は引き継ぎません。

過去の暗号化 fixture と既存の暗号文は変更せず、通常 export の前提条件にしません。古い暗号文を読む場合は元の鍵が必要ですが、移行・書き換えは行いません。全対象の分類・書き込み停止の同期・installation 再構築は未完了です。

ネイティブ退避の復元検証では、隔離した所有済み fixture の archive を展開するときに user 名前空間以外の拡張属性も明示的に含め、合成した trusted 属性を直接照合します。GNU tar の --xattrs だけによる既定の展開では user 名前空間しか復元しません。これは同一基盤の fixture 検証であり、任意の保存済みセキュリティ属性を Host に適用したり、旧管理権限を復元したりする許可ではありません。復元データ全体の照合は引き続き必要です。


## 通常の Incus image の保持

状態: **G2／G3 の一部**。Incus 標準コマンドを使います。今後の Env 作成に必要な image を保持する手順であり、独立した snapshot rootfs の復元に元 Base image は不要です。snapshot component、Hacocoon catalog の管理対象、日常のコマンドは追加しません。

保存元の Physical Host で、一覧から確認した完全な fingerprint と実際の image 名前空間を使います。新しい private directory を選び、各コマンドが失敗したら中断してください。

```bash
umask 077
mkdir -m 700 /absolute/new-image-export
incus image export FULL_FINGERPRINT /absolute/new-image-export/image --project SOURCE_PROJECT
ls -l /absolute/new-image-export
```

出力された全ての part を保持します。検証した分割 image では、この prefix から `image`（metadata）と `image.root`（rootfs）が作られました。一体型では `image.tar` が作られる場合があります。prefix だけで出力の欠落を判断したり、既存ファイルへ再 export したりしないでください。実際の各 part の SHA-256 を計算し、保存元 WSL の外の新しい保持ディレクトリへコピーして、import 前にコピー先でも照合します。

復元先の Physical Host で、新しい project 名と一意な所有識別用 description を作成前に記録します。共有名前空間では検証を隔離できないため、独立した image 名前空間を明示的に有効にします。

```bash
incus project create RESTORE_PROJECT --description UNIQUE_RESTORE_DESCRIPTION -c features.images=true
incus project list --format=json
incus image import /absolute/retained/image /absolute/retained/image.root --project RESTORE_PROJECT
incus image list --project RESTORE_PROJECT --format=json
```

import 前に、記録した project description と `features.images` を確認します。一体型の場合は `incus image import` に実際の archive path だけを渡します。import 後の完全な fingerprint と image 種類が保存元と一致し、保持ファイルも変わっていないことを確認します。失敗と作成済み資源の正確な識別情報を残し、推測で cleanup したり、既存 project を置き換えたりしません。保存元と保持 archive は残します。image の import は Hacocoon Base の登録、alias の復元、旧権限の採用、Env 作成、起動の実証ではありません。

別々の専用 WSL による実検証では、分割 container image 2 件を export し、全 part を Windows の新規ディレクトリへコピーして SHA-256 を照合し、新しい独立 Incus image project へ import しました。両方の fingerprint・種類が一致し、Windows 側のコピーも不変でした。最初の project 確認は `incus project show` が `--format` 非対応のため失敗しました。その記録を保持し、import 前の確認を `project list` で続行しました。復元した image と project は保持しています。この検証では一体型 image の import、これらの image から作った Env の起動、installation 全体の入替は未検証です。
