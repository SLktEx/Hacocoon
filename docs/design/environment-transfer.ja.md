# Environment の持ち出し

状態: Linux 公開 export は **partial**、公開 import は **planned** です。native rootfs／volume の受入テストは内部の前提確認であり、
利用可能な Hacocoon importer ではありません。

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

将来の公開 import は canonical lifecycle の所有確認を使って新しい管理資源を作り、Workspace の Git 状態と
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
import は planned です。保存元 Env は停止している必要があります。

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
