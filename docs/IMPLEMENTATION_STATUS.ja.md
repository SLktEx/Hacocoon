# 実装状況

7517c27 の対象 Incus・通常テストの全 job と Windows VS Code は成功しましたが、transfer は SSH 内の Git 導入で失敗しました（exit 100）。SSH 準備時に現在の管理対象 proxy を sshd セッションへ設定する修正を追加し、684e411 でローカル tests／vet と実 Windows SSH 内の Git 導入が成功しました。[ADR 0058](adr/0058-ssh-session-egress-environment.md)を参照してください。

GitHub の接続情報を持つ import は既存の source clone と Git connect を使います。一致する接続、不一致・offline・同名再作成の拒否の component テストと package の race・vet は成功しました。import 後の実 Git fetch・push は未検証です。[契約](design/git-and-github-capability.md#reconnect-an-imported-github-workspace)を参照してください。

0cc27a5 の Windows transfer は seed-repository で失敗（exit 127）、VS Code は成功しました。fixture に通常のパッケージ導入経路で trusted Host と source Env の不足する Git を準備する処理を追加しました。後続684e411でtransferは成功し、独立した承認probeは失敗しました。

## 公開 Environment import の作業状況

Status: **partial** です。Linux の `haco env import <file.haco> [new-env]` を、client のファイル読取、
管理 upload、native Workspace／OCI の所有管理、canonical Env 作成・起動へ接続しました。
既定名は SOURCE-imported で既存名は拒否します。入力は変更しません。version 1 と元 Host の file 接続先は
 offline で import し、GitHub descriptor は接続先だけを保持して承認・認証情報は引き継ぎません。

製品構成で private staging と64 GiBの共通 payload 上限を全 native import adapter へ設定します。
新しい資源 owner・Env 世代・現在の sandbox／管理 SSH 設定を維持します。Base 実体、自動 backup、
現在データの置換、import catalog、schema 移行は追加しません。失敗 receipt は保持資源を示し、
起動失敗時はデータを残して確認できるようにします。

公開 CLI 接続前の内部 Incus/Btrfs aggregate は6360a23で558.35秒成功しました。保存元削除後の rootfs・
2つの Git Workspace・OCI の独立 import、Env 起動、所有 cleanup を確認しました。転送 race は4.808秒で
成功し、2992c47の全 Go・vet・JS 27件・文書・workflow policy も成功しました。全体 local CI は Ubuntu の
pwsh 不在で失敗し、その後の all-entry 項目は未実行です。最新 head の GHA とは区別します。

b7297a3 の専用 Incus/Btrfs 実行では、製品 import CLI・管理 stream・canonical importer による
rootfs／Git／OCI の独立復元、実起動、旧世代の拒否、管理 SSH 更新、Env 削除後の保持と所有 cleanup が成功しました。
これは fixture controller の実検証です。後続 684e411 でインストール済み controller／desktop からの import も成功しました。
aggregate 全体の後続 snapshot／copy の完了判定は別に扱います。SSH 実ハンドシェイクは684e411で成功しました。live OCI 整合性、Git 再接続、未完了 collection の cleanup、Windows native
ファイル入力は未完了です。[Environment transfer](design/environment-transfer.ja.md#linux-import-コマンド)を参照してください。

b7297a3 の初回公開 import aggregate は export・native import・restore に成功し、続く公開 copy で
fixture の12分期限に達して720.07秒で失敗しました。全体は FAIL であり、成功や SKIP ではありません。
所有 catalog と保存データは `/var/lib/haco-snapshot-aggregate-1920048809` に明示 cleanup のため残しています。
共有データは削除対象にしていません。増えた検証量に合わせて fixture を20分、GHA の test process を25分に
設定します。製品の期限・隔離は変更しません。b7297a3 の GHA 実 Incus/Btrfs
[aggregate step](https://github.com/SLktEx/Hacocoon/actions/runs/34455660292/job/102801320149)は、
製品 import CLI と全 aggregate assertion を含めて成功しました。ローカル失敗は保持し、GHA を独立した受入結果として
扱います。延長後の local-budget 版はコンパイル済みですがローカル再実行はしていません。b7297a3 の4 workflow は成功しました。後続の fixture 期限変更は、その最新 head の CI を別に追跡します。

a58d553 の単独製品 controller は native import・データ・所有 cleanup を20.35秒で確認しましたが、
全体 gate は診断ディレクトリの配置で失敗し、配置を修正しました。6d5e027／e598270 の SSH は失敗し、
後者で sshd 不在と SSH 導入段階の失敗を確認しました。SSH 継続は、別の管理 source と通常の限定
package Policy を使い、公開 export／import、新しい鍵を固定した Windows SSH、保持データからの
Env 再作成を行う既存 installed Windows gate へ接続しました。684e411 でこの gate と単独 native
controller 確認は成功し、両方を必須のまま維持します。[受入記録](design/environment-transfer.ja.md#インストール済み-controller-と-ssh-の受入)を参照してください。

## 公開 Environment export の作業状況

Status: **partial**。Linux の `haco env export <stopped-env> [file.haco]` は管理 stream と
検証後の上書きしない client 公開を使います。既定は `<env>.haco` で、別の snapshot コマンドや
controller path は不要です。Unix stream と実 filesystem の CLI race test は成功しました。
local shipped CLI 全体 gate は export 成功後に fixture 期限で失敗し、同じ GHA aggregate gate は
`3d0dd9a` で47.06秒で成功し、該当4 workflow も成功しました。公開 import と Windows native 出力は planned です。
[契約](design/environment-transfer.ja.md)を参照してください。

停止 Env の内部 exporter は canonical capture/read/delete、native component producer、
匿名の一式 staging を接続しました。Linux 公開 CLI/controller の artifact 転送は partial です。
native aggregate export 受入は 314.12 秒で成功し、公開 bundle import/SSH の証明ではありません。[Environment 持ち出し](design/environment-transfer.ja.md)を参照してください。

ca5ba79 は controller 起動後、最初の画像一覧で失敗しました（native fixture 112.27秒）。maintenance が既存 Store の明示指定と `SkipDefaultResource` を併用し、canonical create に拒否されていました。不要な指定を削除しました。実 catalog／lifecycle の回帰テストで修正前の失敗を再現しています。修正後の native 操作は未確認です。

ca6e5fb の controller gate は起動準備前に失敗しました。fixture が登録済みの `runtime.incus` ではなく `incus` を指定していました。正規の定数参照に修正しました。native のツール配置・画像操作は成功し、失敗後は保持 Store が残るため pool cleanup も失敗しました。controller 全体の受け入れは引き続き未確認です。

controller／CLI の受け入れ経路を使い捨て GHA 限定 gate として追加しました。実行結果は未確認です。製品 composition と実 catalog／lifecycle を使い、Store の内容だけを合成 fixture が供給します。

## 未接続 Store maintenance の実装中の範囲

状態: **partial**。既存 image list/delete は保持 Store ID を受け付けます。OCI module が
正確な owner を確認し、canonical run service が操作全体と cleanup の間、一つの予約を保持します。
各 runtime 呼び出しは新しい一時 Env の世代を照合します。元の Workspace 対応と借用 Store は保持します。
混在・古い識別情報、source Store、未対応の未接続 Docker は拒否します。
新しいコマンド・schema・隠れた backup・復旧状態は追加しません。

OCI・control API・製品 CLI・run の race suite は成功しました（3.402秒、47.590秒、6.456秒、2.794秒）。
native metadata 起動は以前179.66秒で成功しました。拡張した製品画像操作の native fixture は別の検証です。
catalog・lifecycle の識別情報は fixture が供給するため、導入済み controller の受け入れとは扱いません。
最初の試行は digest 件数と同じ表示タグについての fixture の仮定で失敗しました。
その正確な所有 fixture は cleanup し、receipt は保持しています。

Linux/WSL amd64 の composition は保持 Store 接続前に固定 OCI ツールを自動配置します。
private cache、上限付きの固定 member 展開、hash 照合付き Incus 転送、一時ファイル解放を実装しました。
cache の race test は2.158秒、adapter・作成の拒否 test は1.956秒で成功しました。
その focused run の composition はコンパイルのみで、テスト実行ではありません。
native 配置は成功し、controller 全体の作成は未検証です。amd64 以外のツール準備は未対応です。
候補選択 GC と未接続 Docker は未実装です。[契約](design/oci-image-deletion.ja.md#未接続-store-の実装中の範囲)を参照してください。

d3013a3 の test・Ubuntu installer・Incus GHA は成功しました。Windows installer は
pending approval の Python 前提 setup で失敗しました。private registry は workflow_dispatch gate により SKIP です。
失敗を承認待ちとは扱いません。

修正後の拡張 native fixture は224.64秒で成功しました。製品の一覧、実参照による削除拒否、
選択 digest の削除と不在、container metadata 保持、mask 付き再起動、Store 保持、
所有対象だけの cleanup を確認しました。

製品の準備処理・Incus adapter による自動配置を含む native fixture は237.37秒で成功しました。
画像操作、metadata・Store 保護、所有対象だけの cleanup まで確認しました。空 cache からの
実 HTTPS 取得・固定 member 展開は別に70.71秒で成功し、取得バイナリは Host で実行していません。
OCI・Incus・composition 全体の race suite は4.332秒・22.135秒・1.856秒で成功し、vet も成功しました。
先行する71a40e0の GHA は4 workflow すべて成功しました。controller 全体や他 architecture の
受け入れを証明する結果ではありません。

## Environment 持ち出しの前提確認

Linux／WSL の Incus adapter は所有済みの保存 Workspace／OCI volume を匿名・読み取り専用 archive へ export し、
native 所有情報と backup cleanup を確認します。専用 Incus 6.0.5／Btrfs の adapter 検証は5.92秒で成功し、
関連 race test と vet も成功しました。Linux 公開 export は接続済みで、公開 import の実経路受入は未確定です。内部 rootfs producer は固有所有の native image と匿名 archive を使う実装を追加し、専用 Incus 6.0.5/Btrfs adapter 受入は 13.44 秒で成功しました。
[所有文書](design/environment-transfer.ja.md)を参照してください。

公開 G1 は **partial** で、Linux export を実装し import は **planned** です。内部の snapshot／archive 照合は現行上限までの全 Workspace と任意の OCI を扱います。
保存元の読み取り境界は canonical な削除ロックを共有し、保持 component を検証します。
native archive 作成と Linux export を接続し、公開 import の実経路受入は pending です。native Incus rootfs／volume archive の opt-in テストと既存 GHA への追加を実装しました。
fixture の path／namespace の想定を修正後、専用 Incus 6.0.5／Btrfs で11.24秒の検証が成功しました。
保存元・復元先の独立性、Git 状態、リンク、mode、archive 保持を確認しました。rootfs と公開 import の権限処理は未実装です。
別の空 rootfs image 検証は14.88秒で成功し、Base/image を使わない作成、import 前の保存元 instance/image 削除、
現在の明示設定を確認しました。OS／SSH／公開 import の受入ではありません。
[所有文書](design/environment-transfer.ja.md)を参照してください。

固定 role の内部ストリーム書き込み／検証を追加し、展開や Incus 操作なしで完全な内容を確認します。
関連 race test と vet は成功しました。公開 lifecycle への接続は planned です。

Linux／WSL staging は検証済み bytes を名前のない読み取り専用ファイルに保持します。
実 filesystem の race test と vet は成功しました。Btrfs 上の staging は未検証、公開 lifecycle 接続は planned です。

## 現在の Incus-first snapshot 契約

状態: **保存と新しい Environment への restore は implemented** です。
現在のコマンドは `haco snapshot restore <snapshot-id> [new-env]` で、既存 Env の
置換は **planned** です。以下の古い checkpoint は各 revision 当時の検証記録です。
当時の「公開 restore は planned」という記述は、現在の実装状態を上書きしません。
[現行の契約](design/environment-snapshots.md#restore-into-a-new-environment)を参照してください。

Incus が独立した rootfs・volume のコピーと runtime 操作を担当します。Hacocoon は
保存全体の整合性、永続データの所有確認、新しい権限世代を加えます。新しい保存物は
rootfs・Workspace・任意の OCI・metadata で構成され、Base filesystem と自動
pre-restore backup は作りません。schema 13 は既存の Base・backup 所有記録と
保存元の予約を保持し、通常の更新に保存データの手動書き換えは不要です。
既存の記録を黙って破棄しません。

[PR #493](https://github.com/SLktEx/Hacocoon/pull/493)に、元 Base・image cache に
依存しない実 Incus/Btrfs の保存・復元準備を記録しています。
[PR #501](https://github.com/SLktEx/Hacocoon/pull/501)には、保存元削除後の公開 restore、
新しい世代、Git・OCI の内容保持、所有対象だけの cleanup の実検証を記録しています。
これらは各 revision で実行した結果で、以後の全変更を再検証したという意味ではありません。
復元後の SSH handshake と稼働中 OCI DB の整合性は未検証です。容量回収、未接続 Store
の image 操作、移行は別の未完了作業です。

## Host source の image 操作

partial の実装です。`image list/delete --host` は正確な管理対象 Host source のみを選びます。現在の plugin／controller／CLI と Incus adapter が、source 所有者、local role／mount／layout、非特権で running の状態、既存 Host-copy operation lock、固定命令／template の制限を確認します。guest Store を Host source として指定できません。関連5 package が成功し、security／CLI 回帰を追加しています。共有 Host-copy lock の保護を含む関連4 package の race と、大文字 tag の追加回帰は成功しました。専用 WSL Incus/Btrfs の実検証は487.19秒で成功しました。両 runtime で Host source 一覧、停止 container の参照による拒否、選択 image のみの削除、他 image の保持を確認し、その後の独立 Store copy／削除と所有 fixture の cleanup も成功しました。実 Host adapter と fixture catalog を使う検証であり、インストール済み controller／公開 CLI の実接続は未検証です。既存の独立コピーは保持し、未接続 Store と GC は planned です。[契約](design/oci-image-deletion.ja.md#管理対象-host-source)を参照してください。

## 接続済み Store の image 操作

partial です。image list/delete を現在の OCI plugin・controller・製品 CLI に接続しました。旧 Seed 選択ではなく runtime の一覧・削除を使い、確認済み Env 世代・Store 所有 ID で実行を保護します。初回の controller 配線・正規表現・無効 RPC エラー分類の不具合を修正し、関連7 package と文書チェックが成功しました。専用 WSL Incus/Btrfs の native COW 検証は417.80秒で成功し、Docker・nerdctl の一覧、停止済み container の参照拒否、不変 runtime ID の削除、不在確認、元 image の独立性を確認しました。専用 project・pool・不要になった catalog は cleanup 済みです。関連4 package の race と、Env 削除の割り込みを防ぐ個別回帰も成功しました。この fixture は実行用 test adapter を使うため、インストール済み controller／公開 CLI からの native 操作は未検証です。PR #508 は `4d9038b7` の4 workflow 成功後、`3aa8b07f` にマージしました。固定したファイルを使う WSL ext4 のローカル docs・policy・Go test／vet・JS・E2E は成功しました。systemd は古い Ubuntu tool で最初に失敗し、専用 Hacocoon WSL では成功しました。初回失敗は記録に残しています。ローカル packaging は必要 tool 不在で SKIP し、GHA release-config で成功を確認しました。未接続 Store・候補 GC・容量回収は planned です。schema 移行はありません。[契約](design/oci-image-deletion.ja.md)を参照してください。


## source repository の明示的 cleanup

E5 の partial です。`haco repo list [--json]` と `haco repo delete [--yes] <id>` を実装しました。Workspace の参照が現在の Git 経路を保護し、既存 registry・Host operation lock が所有 ID・未完了 Host copy・native 保存物を守ります。schema 13 と独立データは保持します。関連 package test は成功しました。専用 WSL Incus/Btrfs の公開 source CLI 検証は30.43秒で成功し、Workspace 参照拒否・子 snapshot と Host mount 保持・旧 owner 拒否・正しい detach/delete と不存在を確認しました。専用 project は削除し、共有 image・pool・所有記録は保持しました。追加で、待機した Git 操作を registry lock 内で再照合する保護を加えています。最終 race/CI 結果は実装 PR で追跡します。[契約](design/git-and-github-capability.md#explicit-source-repository-deletion)を参照してください。OCI cleanup PR #506 は `73175b4` の4 workflow 成功後に `6903319` へマージし、新しい native 回帰は GHA で0.71秒で成功しました。


source cleanup の最初の2つの GHA 候補は、削除処理より前の fixture instance 作成で失敗しました。元 image の project を明示した版は専用 WSL で28.05秒で成功しましたが、2回目の GHA 失敗を解消できず、image project だけが原因とは確定していません。fixture は空の停止した Incus instance を作る形へ変更しました。このテストに必要なのは管理対象 Host の接続情報で、image や稼働中 guest は不要です。両失敗 job の後続 Base・OCI 検証は SKIP であり、成功扱いにしません。空 instance 版は専用 WSL Incus/Btrfs で13.82秒で成功し、公開 CLI・参照／子 snapshot／旧 owner の拒否・所有対象の cleanup を確認しました。その前のローカル起動は PowerShell の引数分割でテスト実行前に失敗し、引数を引用して再実行しました。関連 package test と文書チェックも成功しています。PR #507 は `c4842c2` の4 workflow 成功後、`19c4bdd9` にマージし、native source fixture は GHA で0.80秒で成功しました。

## OCI Store の明示的 cleanup

E5 の partial です。OCI Store の一覧・確認付き削除を実装しました。既存 catalog が所有者と予約を管理し、Incus が volume を削除します。native snapshot・backup・schedule、Host の元 Store、作成途中・利用中の資源は削除を拒否します。schema 13 と独立 snapshot は変わりません。関連6 package test は成功しました。専用 WSL Incus/Btrfs 検証は11.21秒で成功し、独立 COW・元 Store 削除・子 snapshot による削除拒否と ready/data 保持・旧 owner 拒否・正確な削除と不存在を確認しました。初回は fixture の snapshot show 引数誤りで失敗し、修正後に成功しました。初回の残骸は所有確認付きで削除し、専用 pool が空であることも確認しました。Docker 互換性と個別 OCI image 操作は未検証です。[所有文書](design/persistent-oci-store.md#explicit-retained-store-deletion)を参照してください。

## build 済み Base image の明示 cleanup

E5 は partial です。`haco base list --all [--json]` と `haco base delete [--yes] <name-or-fingerprint>` で保持中の build revision を確認・個別削除します。Incus が image・alias を管理し、service は catalog 参照と確認した所有 ID を照合します。native create・publication と削除を同期し、Env・保護対象 alias の利用中は拒否します。独立 snapshot の由来情報は元 image の保持を必須にしません。schema 変更・保持オブジェクト追加・移行はありません。[Base 契約](design/base-images-and-custom-environments.md#explicit-built-image-cleanup)を参照してください。

Base cleanup PR #505 は `9d8ff82` の適用対象4 workflow が成功し、`32dd1e4` にマージしました。実 Incus/Btrfs の Base 削除と保存 rootfs の独立性は111.30秒で成功しました。local CI は Windows mount 上の repository-copy test が10分で失敗した後、同一 commit を WSL ext4 に置いて成功しました。Windows 初回は生成 alias の SSH で失敗し、同一 commit の再実行で成功しました。初回の失敗記録は PR #505 に残しています。Workspace PR #504 は適用 CI 成功後に `4adfa81` へマージ済みです。

## managed Workspace の明示的削除

E5 は partial です。`haco workspace list [--json]` と `haco workspace delete [--yes] <id>` で保持中の managed Workspace を確認・個別削除します。既存 lifecycle lock で Env・途中 lease の利用を拒否し、表示した所有 ID を照合して registry に `deleting` を記録します。途中失敗では member の正確な所有情報を保持します。Incus の volume 所有・使用中・不在確認を再利用し、OCI Store・元 repository・独立 snapshot は残します。create は lock 取得後にも Workspace を解決し直し、同名の所有者入れ替えを拒否します。schema 変更や別の cleanup catalog はありません。[契約](design/workspace-abstraction-and-lease.md#explicit-retained-workspace-deletion)を参照してください。

初期候補では関連 package・race test、maintained local CI、local E2E が成功しました。実 Incus/Btrfs GHA run 34301447147 の公開 Workspace CLI fixture は31.19秒で成功し、利用中の拒否、Env 削除後の Git 保持、member の個別削除、独立 OCI・snapshot の保持を確認しています。その候補の適用対象4 workflow は成功しました。後から追加した native 保存物の保護は、対象 package test と専用 WSL Incus/Btrfs test（23.23秒）で成功しました。子 snapshot・backup による削除拒否、親子の保持、所有対象の明示 cleanup を確認しています。native snapshot schedule、不正・不明な応答も拒否します。全 member を `deleting` 記録前に確認し、削除直前にも再確認します。最終候補の workflow 結果は PR #504 に記録します。初期候補の CI 成功で、後から追加した保護まで検証済みとは扱いません。E5 の Base・元 repository・OCI image 整理、F の容量回収、G の移行は残作業です。

Base builder PR #503 は候補 `15fed95` の適用対象4 workflow 成功後、`2ba5434` にマージ済みです。作成コマンドの正しい表記は `haco env create` です。

## Base builder

Base builder 検証: 関連5パッケージの通常テストと race テストは成功しました。実 WSL の1回目は稼働中の machine-id 初期化で失敗し、2回目は guest exec 専用の stdin 経路で管理操作が拒否されました。停止後に通常の検証付き runner で行う Incus file 操作は専用 probe で成功しています。3回目は最初の Base 公開・作成・ツール実行まで成功しましたが、再 build 中に600秒のテスト制限で失敗しました。実検証全体の成功とは扱いません。GHA run 34297739368 の実 Incus/Btrfs build・再 build は70.69秒で成功し、Windows run 34297739417 は installed build → SSH での追加ツール実行と VS Code 接続に成功しました。local CI test も成功しました。最初の test workflow は Incus なしの Base 一覧 fixture で失敗し、その fixture を修正しました。後続候補の結果は PR #503 に記録します。ローカル timeout fixture の cleanup は Incus が停止中と実行中を矛盾して返したため一度失敗しました。所有確認付きの native force-stop 後、Env 2個の canonical 削除と専用 image 2個の削除・不在確認に成功しました。診断 catalog と共有元 image は保持しています。



implemented（代表的な E4 フロー）: `haco base build <definition.json>` が、通常の一時 Env での定義実行と、停止後の Incus image 公開を組み合わせます。所有情報は Incus image に記録し、検証後の alias を通常の revision 固定 create から選択します。旧 revision・snapshot・catalog は保持します。対象テスト・実 Incus・SSH の検証結果は変更単位で記録し、この実装記載だけで実機成功とは扱いません。[Base 契約](design/base-images-and-custom-environments.md)を参照してください。


## Environment copy

修正後の専用 WSL Incus/Btrfs 検証は 357.13 秒で成功しました。停止済み元 Env の copy、名前の前方一致、新しい世代、元削除後の rootfs/Git/OCI の独立性、所有 cleanup を確認しました。fixture `haco-aggregate-1a17295b1a6f50e1` は完全に片付けました。項目が増えた aggregate fixture の時間枠は 8 分とし、製品の timeout は変更していません。

実 Incus 検証は最初、名前の前方一致で元 Env と copy の両方が返り、停止状態確認に失敗しました。名前と状態の CSV から厳密に対象を選ぶよう修正し、不正・重複応答の回帰テストを追加しました。初回ローカル CI はこの修正前に成功しています。最終検証は PR に記録します。

実装済み: `haco env copy <stopped-env> [new-env]` は既存の Incus COW 保存と通常の復元経路を組み合わせ、独立データと新しい権限世代を作ります。実行中の元 Env と既存の宛先は拒否します。一時保存は処理後に削除し、削除不明時は ID を返します。schema 追加や restore 前の自動 backup はありません。検証結果は変更に記録します。[仕様](design/environment-copy.md)を参照してください。

## 公開 snapshot restore

implemented: `haco snapshot restore <id> [new-env]` が保存 Workspace／OCI のコピー、
独立 rootfs からの正規 Env 作成と起動を行います。省略時の名前は `<source>-restored`、
既存名は拒否します。作成失敗時は Workspace lifecycle lock の下で今回所有する未使用
コピーだけを cleanup。不確実な lease があればデータを残し、起動だけの失敗なら
公開済み Env を残します。Base 依存、backup、新しい catalog 状態、準備済み binding の
CLI 必須引数は追加しません。対象・関連 race テストは成功し、キャンセル、別 owner の
OCI、作成中 lease、公開コピーの不完全 cleanup を確認しました。隔離 WSL の初回実
Incus/Btrfs 公開 restore aggregate は 237.69 秒で成功。fixture は
`haco-aggregate-9b6e7be3f4718210`、公開保存は
`snap-9a0e62a7d6e7ee1ad2b430e06658ceaa`、Workspace は
`restore-7a43b3413a89ad32` です。保存元削除後の実 CLI 復元、新世代の識別、guest の
データ、所有試験資源の完全 cleanup を確認しました。最終 build と全 CI の結果は
この変更の PR に記録します。専用 image でないため共有 image 削除は SKIP です。
既存 Env の置換、復元先への実 SSH 接続、live OCI 整合性は未検証です。
[契約](design/environment-snapshots.md)を参照してください。

## Workspace snapshot コピーの保存元保護

implemented: Workspace コピー中は共有 catalog で保存元を予約します。
cleanup が不確実なら所有記録と予約を保持し、公開済みコピーで予約解除だけが
失敗した場合は解除だけを再試行します。schema 13 は schema 12 の実行環境の
保存元予約、schema 11 の OCI 記録と既存保存物を維持します。CLI コマンド、
Base component、backup、実行環境の復旧状態は追加しません。state・registry・lifecycle・composition の race テスト、文書・cleanup helper 検査は成功。
隔離 WSL の実 Incus/Btrfs aggregate は 184.64 秒で成功しました。fixture は
`haco-aggregate-e742fe53ad8dc2db`、保存 ID は `snap-6331fdeac4c8f8350a8604af677cdbbd`、
公開 CLI の保存 ID は `snap-4dd65c2c8d0da67de8dc358c24689332` です。保存元から独立した
Workspace/OCI の登録、同名 Env の新世代作成、公開 snapshot CLI、データ保持、
所有試験資源の完全 cleanup が成功しました。共有 image の削除は専用資源でないため
SKIP。公開 restore・復元先への実 SSH 接続・live OCI 整合性は未検証です。全 CI の結果は
この変更の PR に記録します。

## 公開 snapshot の保存・管理

実装済み：`haco snapshot create <env>`、`list [env]`、`delete <id>` は既存の
controller・lifecycle・catalog と Incus copy を利用します。実行中なら停止して
保存完了後だけ再開し、停止中なら停止を維持します。保存失敗・部分保存では ID を
保持して失敗を返します。元 Env 削除後も一覧に残り、内部 binding は公開しません。
schema 変更、Base 実体の追加、自動 pre-restore backup はありません。公開 aggregate
restore は planned です。検証結果は本変更の PR に記録します。
[使い方・契約](design/environment-snapshots.md)を参照してください。

PR #498 は対象の 4 workflow が head `5cf9bb7` で成功し、`d54618b` として
マージ済みです。実 Incus aggregate は 10.62 秒、Windows の実 SSH と VS Code
編集・terminal も成功しました。private registry、共有 image 削除、VPN/NRPT、
新しい人手の通知判断は、それぞれの既存 gate により SKIP のままです。


実 WSL の初回・2 回目の公開保存は 116.15 秒・138.25 秒で失敗し、部分保存 ID を
保持しました。限定した native 検証で、`volatile.last_state.ready` を空文字に
消去すると Incus の boolean 検証に拒否されると判明しました。保存・保存 rootfs の
起動用 copy・復元 staging は `false` に戻すよう修正し、native 要求の回帰テストを
追加しています。別の start 検証で見つかった fixture のネットワーク所有 wrapper
不足も production と揃えました。最初の fixture `haco-aggregate-52a680aada6ca3e6` の
所有対象 cleanup は成功し、証跡 metadata だけを
`/var/lib/haco-snapshot-aggregate-1076841042` に保持しています。

修正後の native 検証では 2 番目の失敗 fixture の保存・再開に成功し、
`snap-bc49bb612ad51ab361840aa59488913f` が完成しました。保存物・runtime・所有 network・
Workspace・OCI の cleanup も成功し、metadata のみを
`/var/lib/haco-snapshot-aggregate-4240550795` に保持しています。application・state・API・
CLI の関連 race テストと、native 保存・staging・起動用 copy の回帰テストは成功です。

修正後の新規 WSL Incus/Btrfs aggregate は 211.58 秒で成功しました。fixture は
`haco-aggregate-53f70c5d1b1341e5`、最初の保存は
`snap-2e55b4514894de3adb8f71e2ddac39fa`、公開 CLI 保存は
`snap-5f11941d2fdab09a0049716565d86d29` です。実ビルドした `haco` と専用 controller
socket で create/list/delete を実行し、実行中保存元の停止・保存・再開、元 Env
削除後の保存物検証・一覧、明示削除、現在の Workspace／OCI 保持に成功しました。
fixture 全体の cleanup も成功です。共有 cache image 削除は専用 image 許可がないため
SKIP、公開 aggregate restore・復元 Env の SSH 実接続・稼働中 OCI DB の整合性は
未検証です。全体 local CI と exact-head GHA の結果は実装 PR に記録します。

## 保存 rootfs の正規作成経路

内部実装済み：保存 rootfs の作成を通常の lifecycle、保存元予約、参照付き receipt、
所有対象だけの期限付き cleanup に接続しました。schema 12 は schema 11 の保存物と
OCI の作成記録を維持します。environment／state／workspace 全体の race、追加の
失敗・競合・移行回帰、文書と CI cleanup helper は成功しました。専用 WSL Incus/Btrfs
aggregate は 129.46 秒で成功しました。fixture は `haco-aggregate-b9c627e16c2da2be`、
snapshot は `snap-0b4153b66307d5bce49b46befe0ee764`。削除した元 Env と同じ名前を正規経路で
新世代として作成し、receipt 後の公開と保存元予約解除、通常削除後の Workspace／OCI
保持、fixture 全削除を確認しました。共有 image 削除は SKIP、復元先の実 SSH 接続、
公開 aggregate restore、live OCI database 整合性は未検証です。全体 local CI と同一 head
GHA は実装 PR に記録します。データコピー全体の調整と公開 restore は planned です。

PR #497 は適用対象 GHA の全成功後、`62e9947` にマージしました。実 Incus aggregate は
10.30 秒で成功し、Windows の通常 SSH／VS Code も成功しました。Ubuntu installer は
変更パスの対象外、記載済み private registry／image／VPN／人間の UI 操作は SKIP です。

## 保存 rootfs の実行用コピー

内部実装済み：Base／image／default profile の解決を通さず独立 rootfs をコピーし、
直後に所有記録を保存して、通常作成と共通の現在の sandbox 設定を適用します。
世代 ID と管理 SSH 情報を更新します。aggregate の起動調整と公開 CLI は planned
です。対象と Incus package 全体の race／vet は成功しました。全体 local CI
（Go tests／vet、文書／helper、JavaScript 27/27）は最後の device 修正前に成功しました。
実 Incus の初回は、コピーされた `none` device と現在の接続名が衝突して 93.76 秒で失敗。
作成記録後に新しい実行用 instance の mask だけを除去する修正と、名前衝突・除去失敗の
低レベル回帰テストを追加し、最終の対象テストと文書チェックは成功しました。

修正後の専用 WSL Incus/Btrfs aggregate は 135.24 秒で成功しました。fixture は
`haco-aggregate-8ce0bd921c0adafb`、snapshot は `snap-d401c8f07e734b02f8c14fd03293a8c1`。
元 Env／volume と Base がない状態で、実起動、新世代と source guard、guest 内の
root／Workspace／OCI データ、管理 SSH 登録の更新、通常 Env 削除後のデータ保持、
fixture 全削除を確認しました。失敗 fixture の実資源も正確な所有確認と正規 API で
削除し、記録 `/var/lib/haco-snapshot-aggregate-4256480528` だけ保持しています。
共有 image 削除は専用 image 条件により SKIP、復元先への実 SSH 接続、公開 restore の
調整処理、live OCI database 整合性は未検証です。同一 head の GHA は実装 PR に記録します。

OCI 登録の PR #496 は適用対象４ GHA workflow がすべて成功し、`4b06b5f` に
マージしました。実 Incus/Btrfs のデータコピーと Windows SSH／VS Code は成功、
private registry、共有 image 削除、VPN/NRPT、人間による通知判断は fixture 条件により
SKIP です。

## 保存 OCI の再登録

内部実装済み：保存 OCI volume を同じ Btrfs pool の通常 Store に独立コピーし、
新しい所有者と任意の Workspace 対応を登録します。catalog は保存元を予約し、
検証前にコピー完了を記録します。公開または所有対象の不在確認後に予約を解除します。
schema 11 は schema 10 と既存の対応済みデータを保持し、旧 controller は新形式を
拒否します。所有記録・予約・schema の対象 race テストは成功しました。
専用 WSL Incus/Btrfs aggregate は 108.40 秒で成功しました。fixture
`haco-aggregate-87d745d6b76c16c0`、snapshot
`snap-cfc7eae81e93e176a380b5bd171c3e77` で、元 Env／volume 削除後の Workspace／OCI
登録、新しい所有者・Workspace 対応、予約解除、再読み込み、保存データ、独立した
変更と正規 cleanup を確認しました。共有 image 削除は SKIP です。
全体 local CI 成功後、追加回帰でコピー中の通常削除ガード不足を失敗として再現し、
修正しました。最終差分の state／persistentresource／Incus 全 package の race・vet、
文書・cleanup helper は成功し、同一 head の GHA は PR に記録します。
Workspace 登録の PR #495 は
適用対象 GHA の全成功後に 239b3e6 へマージ済みです。Env 起動と公開 CLI は planned、
live OCI database の整合性は未検証です。


## 保存 Workspace の再登録

内部実装済み：Incus/Btrfs の同一 pool 内コピーを、新しい所有者と管理側の
remote／branch 情報を持つ通常の Workspace として登録します。検証前に作成完了を
記録し、途中失敗は今回の所有対象だけを限定 cleanup します。Git の由来情報がない
旧 manifest は保持しますが、自動再登録は未対応です。Env 起動と公開
snapshot／restore CLI は planned、OCI 登録は上記の段階で内部実装済みです。

対象 race テストと専用 WSL Incus/Btrfs aggregate は成功しました（4.09 秒）。
fixture `haco-aggregate-42d34d51baa33c72`、snapshot
`snap-a0b22312747180b9891e790f8427c78c` で、元 Env／volume 削除後の再登録、
registry 再読み込み、Git commit・未 commit・untracked の保持、独立した変更と
正確な cleanup を確認しました。初回は不要な default profile 参照で失敗し、
保存 pool だけを使う修正と回帰を追加しました。失敗 fixture の snapshot は正規 API
で cleanup し、元 Base／Workspace／OCI の不在も確認しました。所有記録は
`/var/lib/haco-snapshot-aggregate-666738198` に保持しています。全体 local CI
（Go tests/vet、文書・helper、JS 27/27）は成功し、GHA は PR に記録します。
共有 image 削除は SKIP、復元 Env 起動と live OCI 整合性は未実行です。
PR #494 は local CI と適用対象
GHA の全成功後に main a66035d へマージ済みです。


## 構成前の runtime 所有記録

実装済み：本番 Incus 作成は init の直後、device／network／resource 設定と起動の
前に正規 lifecycle API で runtime を記録します。全構成が完了するまで lease は
acquiring のままです。共通の構成処理を保存 rootfs の起動にも使えるように分離しましたが、
保存物からの起動とデータ所有権の引き渡しは planned です。receipt の欠落・重複・参照変更、
cleanup 失敗の race テストは成功。schema・CLI・保存形式の変更はありません。

この段階の全体 local CI は成功、実 Incus 検証結果は実装 PR で追跡します。PR #493 は final local CI と
適用対象 GHA の全成功後に main 632484a へマージ済みです。

## Incus-first の snapshot 整理

内部実装済み：新規保存は独立 rootfs、管理 Workspace の全メンバー、接続 OCI とし、
Base は由来 metadata のみにしました。通常 create/run の Base instance 自動保持と、
restore 前の自動 backup を削除しました。復元用コピーの準備は現在データを変更しません。
失敗時は所有確認付き cleanup を試し、独立した対象の処理を続け、
不在を確認できない場合だけ復旧記録を保持します。作成完了記録、書き込みの排除、所有確認、削除完了確認、権限の世代境界は維持します。

schema 10 は schema 8 の復元対象識別を移行し、古い backup／Base の所有記録を
すべて保持します。更新による資源削除はありません。未公開 schema 9 の試作は
明示的に拒否します。Incus adapter／Workspace の調整／state の所有管理／routing の
分離を維持し、本番 Base 保持 callback は削除しました。
[ADR 0040](adr/0040-incus-first-snapshots.md)を参照してください。

state/service/router/provider/通常作成と race の関連テストは成功しました。
全体 local CI（`bash tools/ci-local.sh test`）も成功しました。Go tests/vet、
文書・helper 検証、JavaScript 27 件すべてを含みます。PR #493 の対象 GHA も成功しました。
専用 WSL の Incus 6.0.5/Btrfs 検証は 5.95 秒で成功しました。
fixture は `haco-aggregate-822d7154c3c2f6dc`、snapshot は
`snap-d849c57477844eef6f02e54e9c9b4364` です。元 Base と専用 Ubuntu image を
削除後に 4 component を保存し、自動 backup なしの独立コピー、現在データの不変、
Git/rootfs/OCI の保存内容、元 Env／データ削除後の独立性と所有 cleanup を確認しました。
初回は別 fixture が参照する image の削除を拒否して失敗しました。その回の所有資源は
削除・不在確認済みです。証跡は `/var/lib/haco-snapshot-aggregate-3029918541` に保持します。

公開 save/restore と起動可能な復元先への切り替えは planned、稼働 OCI DB の整合性は
未検証です。PR #492 は local CI と対象 GHA 成功後にマージ済みです。その 5 component／
自動 backup の動作は過去のもので、今回の方針に置き換わります。

## 外部 Workspace 再作成の実機検証

E1 の基本構成が専用 WSL の product 093ed159b80e で成功しました。通常 API の stop/start
では Workspace と永続的な guest ファイルを保持し、delete/create では guest が編集した
外部 Workspace を保持して guest 内だけの状態を除去しました。修正版 fixture
m1-egress-708dfbc120260908 と、最初に /tmp marker で失敗した fixture は両方とも
片付け済みです。各段階・identity 判定と local CI は成功しました。Windows installed E2E
にも追加し、b73f965 の GHA は既定 OCI を無効化しなかった fixture の前提条件で失敗しました。
create/recreate で明示的に opt-out する修正後の Windows GHA は未確認です。managed Git/OCI の組合せと E2-E5 は別の残作業です。
[Workspace lifecycle](design/workspace-abstraction-and-lease.md#resume-and-recreate-an-external-workspace)を参照してください。


## 実 guest AWS 拒否検証

専用 WSL の product 093ed159b80e で、通常ユーザーによる作成、guest haco 自動選択、
送信元固定の未設定 profile 拒否、失敗時のファイル保持、canonical 削除が成功しました。
controller 稼働と検証用 Environment/Workspace の不在も確認済みです。既存 Windows
installer E2E に追加し、新 HEAD の GHA は未完了です。local CI と判定テストは成功です。
認証済み AWS／正の取得は SKIP で、成功扱いにしません。
[AWS 検証](design/aws-operations.ja.md)を参照してください。


## guest の通常 AWS CLI

実装済み: Standard の作成・start で通常 haco を配置し、AWS list/cp は --env や認証情報なしで
隔離付き送信元固定入口へ接続します。取得は検証済み private 保存を再利用します。focused race、
実 HTTP queue／Policy／audit と setup 再利用・競合拒否が成功しました。installed guest は
未検証、実 AWS は前提不足により SKIP です。[AWS 操作](design/aws-operations.ja.md)を参照してください。


## guest AWS server 境界

server 側実装済み: 隔離された Standard listener で、送信元 Environment の正確な
作成 ID に紐付く AWS list/get だけを受け付けます。管理・承認決定は公開しません。
送信元・再作成・偽装・frame サイズの回帰が成功しました。guest CLI は上記で実装済みです。installed guest／
AWS 検証は未完了です。[AWS 操作](design/aws-operations.ja.md)を参照してください。

## AWS アカウント表示名

実装済み: 任意の Host AWS profile 表示名を実 STS アカウント ID に紐付け、
承認と保存範囲へ含めます。ID 不一致や名前変更では実行を拒否します。
local CI、focused race と SDK/config 15 テストが成功しました。
通常の controller 承認・保存許可・失効経路でも、一覧・取得の表示名を確認しました。認証済み AWS と desktop 表示は
Host の前提不足により SKIP です。[AWS 操作](design/aws-operations.ja.md)を参照してください。


## AWS オブジェクトのストリーム取得

repository 実装です。haco aws s3 cp は承認済み current object を trusted Host・controller
経由で転送し、サイズ・SHA-256・最終の実行／監査 receipt を確認して atomic に保存します。
公開前に失敗した場合は既存ファイルを保持します。実 controller stream の 20 MiB 転送を含む
scope・SDK 応答・filesystem の回帰が成功しました。
[AWS 操作](design/aws-operations.ja.md) と [ADR 0035](adr/0035-streamed-aws-downloads.md) を参照してください。
local CI と SDK 11 テストも成功しました。現在の所有確認付き Host streaming adapter は
専用 WSL で AWS 通信なしに 20 MiB を転送できました。
Host の前提不足による実 AWS の SKIP は継続します。guest 要求経路、native Windows
filesystem と AWS desktop 判断の受入は別の残項目です。

## 承認を経由する AWS S3 一覧

D3 の部分実装です。trusted Host の S3 一覧取得を製品 CLI・controller・共通の
Policy／承認／監査と任意の AWS plugin に接続しました。Host 内で固定した credential を使い、
account／principal を再確認します。署名 region だけが変わる場合を含め、SDK redirect を
送信前に拒否します。準備・範囲・上限は [AWS 操作](design/aws-operations.ja.md) を参照してください。
focused race、通常 controller review と config 撤回、合成 credential を使う実 SDK の
8 ケースが成功しました。署名 region だけが変わる redirect の回帰を含みます。
ファイル取得、guest 要求経路、account 名、実 AWS／desktop 受入は planned です。
SDK の transport 置換テストから実 AWS 成功を推測しません。

現行 Host adapter は専用 WSL でも実行でき、AWS 未設定を検出して外部要求なしで拒否しました。
その所有確認済み Host に AWS CLI・botocore・AWS config がないため、実 AWS は SKIP です。

## 完了確認済み OCI コピーの復旧

実装済み: canonical resource lifecycle は Host 再開より先にコピー完了の記録を
永続化します。通常 setup・Host entry・Environment 作成の再試行で、同じコピーを
再コピーせずに再開・公開できます。所有権・journal・再起動 guard を検証し、完了記録が
ない場合や完了不明の場合は recovery-required を維持します。
[ADR 0033](adr/0033-completed-copy-recovery.md) を参照してください。

対象の race 回帰で state 再読込、予約の保持、壊れた・異なる所有者の journal 拒否、
冪等な再開が成功しました。専用 WSL の実復旧・イメージ検証は成功しました（355.73 秒、project
`haco-area-d6ad75cf558f514e`）。完了直後の再開失敗を注入し、guard を保持したまま
state 再読込で同じコピーを復旧、再コピーなしで両 runtime のオフライン実行と全所有
リソースの cleanup を確認しました。Installed CLI での復旧と完了不明のコピー復旧は
まだ証明していません。`ae0c245` の全 4 GHA は成功し、Btrfs job
101949881165 では Docker/nerdctl の同一 ID・オフライン実行・全 cleanup も実行して
成功しました（109.79 秒）。Private registry は workflow_dispatch 専用 job が
実行されなかったため SKIP です。


## Docker Store の root 設定と Host 実イメージ検証

実装済み: Store 接続時に Docker の永続・一時 root を設定し、他の option を保持します。
既存の default data、競合する root、稼働 Docker unit、不安全な設定ファイルは拒否し、
一致する設定は再利用します。対象 race 回帰・vet は成功しました。Host で Docker/
nerdctl の実イメージを build し、コピー先の同一 identity・オフライン実行を調べる
E2E を追加し、維持済み Btrfs GHA に組み込みました。

初回実検証は両方の Host イメージを build・実行後、コピー先の Docker image inspect
で失敗しました（336.37 秒）。Host は `/var/lib/hacocoon-oci/docker`、コピー先は
`/var/lib/docker` を使用していました。確認後、canonical resource deletion で
fixture を全削除しました（11.75 秒）。修正後の新規検証は成功しました（376.60 秒、
project `haco-area-3147b9dd5920fb2c`）。COW 後の両イメージの同一 ID・オフライン
実行、コピー先イメージ削除後の Host 実行、Btrfs ancestry、双方向の領域変更・削除、
全 fixture cleanup を確認しました。Docker 28.5.2/vfs と nerdctl 2.3.5/containerd
2.3.3/native の provider fixture の結果であり、全 driver/version や installed CLI
での再作成を証明しません。f3f5557 の全 4 GHA workflow は成功し、実 image copy と
完了済み copy の復旧も確認しています。初回の失敗は記録に残します。

`470a2b8` は Windows run 34188963290 を含む全 4 GHA workflow に成功しました。
実 Remote-SSH の editor 読み書き・terminal・trusted review、Host customization の
cleanup、installed notification subscription が成功しました。人間の fresh toast
判断と VPN/NRPT は明示的 SKIP のままで、workflow の成功をそれらの成功とは扱いません。


## 所有確認済み Host の nesting

実装済み: 通常の OCI setup は、所有権・非特権 instance・profile・source・
lifecycle を確認してから nesting を有効にします。設定は永続化され、再 setup
で確認して再利用します。[ADR 0032](adr/0032-owned-host-nested-runtime.md) を参照。
対象の race test・vet と composition/OCI lifecycle テストは成功しました。
専用 WSL `Hacocoon-Review-6771f2f` の実検証で nesting 設定・再利用、mount
namespace、Host pause/COW/resume、独立した変更・削除、所有 fixture の全後片付け
に成功しました（58.56 秒、project `haco-area-e8168370b7f8d3f8`）。検証設定は
残っていません。初回は PowerShell の引数解釈でテスト開始前に失敗し、修正後に
成功しました。後続の Docker/nerdctl 実データ結果は上記に記録しています。
namespace だけの成功を実イメージの受け入れ成功とは扱いません。


## デスクトップ接続時の Environment 選択

implemented: 対話端末の `haco open` と `haco ssh setup` は、複数の Environment が
あると Environment／Workspace の一覧を示し、その場で番号選択できます。一つなら入力不要です。
空入力は setup 前に取り消し、非対話で対象が曖昧なら stdin を読まず名前の指定を求めます。
SSH クライアントは選択した作成時刻・runtime・Workspace・access mode を setup 中に再確認します。

component 回帰と実 PTY 上の製品プロセステストで、選択と取り消しが成功しました。
非公開の fixture controller と一時 desktop directory を用いたもので、複数 Environment からの
Windows／VS Code 実接続の証明ではありません。既存の単一 Environment の実績と区別し、
新しい GUI 受け入れは pending です。

`711005a` の GHA test／Ubuntu／Incus は成功しました。Windows run 34185304876 は
インストール・再起動・再インストールと実 SSH 設定／再利用を通過しましたが、Host customization
の後片付けと通知サービスの稼働確認で失敗しました。連続 setup による systemd 起動制限への
到達を再現し、健康で同じサービスを再利用するよう修正しています。実行ファイルや設定の変更時は
再起動します。Python 回帰 12 件と実 systemd の連続 8 回の再設定・後片付けは成功しました。
途中の編集で Python indentation error があり、修正後に上記検証を通しました。
Windows 全体の後続検証は上記 `470a2b8` で成功しました。人間の toast 操作と VPN は SKIP のままです。

## 新規通知サービスの起動

Windows run 34181502807 は通知サービス設定で失敗し、後続の接続・native 検証は SKIP です。
専用 WSL で、新規の未ロード unit に無条件で `reset-failed` を実行すると失敗する不具合を
再現しました。明示ロードだけの修正案も、systemd が再び解放するため実測で失敗しました。
失敗状態を確認できた unit だけを reset し、不明な結果では停止する形に修正しています。
Python 回帰 11 件と実 systemd の新規起動・再設定・所有物の後片付けは成功しました。
これだけで Windows installer 全体の失敗が解消したとは扱いません。新しい GHA と
新規通知からの実操作は未検証です。

## 新規 Host の OCI 保存領域設定

partial: 通常の `haco setup` が新規 Host の所有確認済み保存領域を作成・接続し、
containerd/Docker の保存先を設定して、再実行時に接続と設定を確認します。
既存データ・symlink・独自設定は移行待ちとして拒否し、準備失敗時は所有記録を残します。
日常コマンドや必須 runtime は増やしません。既存データの
移行、実 runtime の復旧は未完了です。Host の nesting は上記の所有確認付き setup で扱います。

専用 Incus/WSL で設定・再確認・領域 COW・独立した変更と削除・正確な後片付けが
成功しました（53.19 秒）。合成データであり Docker/nerdctl image の検証ではありません。
先行 provider commit `29fd6d1` の GHA test・Ubuntu installer・Incus E2E は成功し、
Windows run 34181502807 は通知サービス設定で失敗し、後続の受け入れ検証は SKIP です。

最初の設定回帰は Python に `tomllib` がなく失敗しました。この依存を除いた最終実装で、対象 race テストと実 provider E2E は成功しました。

保守されている local CI で Go tests/vet、WSL の Python 11 件・承認の Python 3 件、JS 27 件が成功しました。文書整合性とその回帰 7 件も成功しました。

コピー開始時も journal・一時停止の直前に Host 保存領域の準備と設定を再確認します。
setup 時の確認だけでは、その後の設定変更で古い領域をコピーできるためです。
準備記録の欠落・設定不一致・確認の失敗／切り捨てを拒否し、Host を停止もコピーも
しない回帰を追加しました。失敗時は正規の source／destination 復旧記録を保持します。

## Host 領域コピーの provider

partial: Incus backend は、専用 source-only volume を持つ所有確認済み Host を一時停止し、
既存の領域 COW コピーを行い、完了を確認して再開できます。永続のコピー記録により
自動起動を無効にし、不明な状態では通常の Host entry を拒否します。他所有者・複数の利用者・
異なる接続先・既存の一時停止・コピーや再開や後処理の未確認を拒否する component テストは
成功しました。実 Incus E2E はローカルで成功し、既存 Btrfs job に組み込み、`29fd6d1` の GHA でも成功しました。

既存 Host データの移行、Docker/containerd のアプリケーション復旧、途中コピーの運用復旧は
未完了です。プロセスの一時停止は daemon の正常終了ではありません。イメージ列挙や
export/import は使用しません。[手順と限界](adr/0031-host-oci-area-copy.md#provider-pause-and-restart-guard)を参照してください。


専用 WSL の project `haco-area-23ef9c90488e244b` で provider の実機検証が
成功しました（39.15 秒）。一時停止・COW・再開、Btrfs parent UUID、双方向の変更の独立性、
コピー元削除、正確な後片付けを確認しました。初回はコピーと独立性の検査後、download した
Base image が残って project 削除で失敗しました。その正確な fixture image・project・pool は
削除済みで、E2E に記録した Base ID の削除を追加しました。それより前の PowerShell 起動も
引数解析で失敗しましたがテスト開始前です。いずれも成功扱いしません。データは合成データであり、
OCI runtime の image ではありません。対象 race・vet・workflow policy・文書検査は成功しました。

維持されている local CI test は Go tests/vet、WSL の Python 11 件・承認の Python 3 件、JS 27 件が成功しました。その後の Host 起動／コピーのプロセス間ロック変更は、対象回帰と新しい provider E2E で別途検証します。

プロセス間ロックと正確な autostart 復元を含む最終 provider コードは、`haco-area-a7d74034ed65d7d4` の専用 E2E で成功しました（37.98 秒）。所有する image・Host・volume・project・pool・非公開の復旧 catalog は後片付け済みです。最終の対象 race／vet も成功しました。

## 実際の Host OCI 領域のコピー

B4 の要件は実際の Host イメージ保存領域をそのまま Btrfs COW でコピーすることです。
イメージ選択や export/import による再構成ではありません。方向の異なる一覧処理は
取り消しました。既存の独立 volume コピーは一致していますが、実 Host 保存領域と
書き込み停止の provider と新規領域の接続までの partial です。合成データの検証を配布完了と扱いません。
[判断](adr/0031-host-oci-area-copy.md)を参照してください。

Windows `4bb8dad` の run 34176272125 は native review が使用できない Get-FileHash を
要求して失敗しました。`8d7a2ea` で .NET SHA-256 に置き換え、PowerShell 5.1 component は
成功しました。修正後の Windows 実受け入れは pending です。取り消した OCI 一覧処理の
対象テストは成功しましたが、実装と一緒に削除しました。その全体 local CI は一部 Go package の成功出力後、
実行中に中断しました。Go 全体・全 CI の完了は確認しておらず、全 CI 成功や、訂正後の領域コピーの検証結果とは扱いません。


## Git と network の承認の一致

実装済みの共通承認について、CLI と Policy／監査の capability 間比較テストを追加しました。
単発判断、保存 6 種、正確な対象範囲、再評価、同名 Environment の再作成を確認します。
対象の race テストと vet は成功しました。最初のテストは保存拒否を approval-denied と期待して
失敗しましたが、再評価後の正しい policy-denied に修正し、両 provider で一致を確認しました。
この回帰検証では新たな実 Git push や HTTPS 接続を行っていません。
[共通の契約](design/pending-approval-review.ja.md)を参照してください。


## Desktop 自動通知の追加

作業ブランチで実装済み: Windows の登録後に、所有する Host 通知サービスを有効化します。
`-SkipDesktopReview` は停止・無効化します。初回は過去の表示をスキップし、既存の再開位置は
通常通り使います。バイナリは検証した atomic 置換により、notifier 動作中も更新できます。
unit parser・所有権・opt-out・from-now と race 回帰は成功しました。
インストール済みの自動サービス受入は pending です。
`4bb8dad` は test・Ubuntu・Incus E2E が成功し、Windows は失敗しました。上記の修正記録を参照してください。


## Host 通知の受入と状態保存

`213fb2b` の Ubuntu installer run 34173412741 で、インストール済み Host の通知購読と
待受の後片付けが成功しました（job 101898000285）。test workflow も成功しました。
Incus run 34173412776 は Host setup で失敗しました。独立した CLI fixture が、setup の
必要条件になった通知バイナリをビルドしていません。fixture にビルドと配布後の
ダイジェスト・所有権検証を追加し、修正後の `6d516d3` の Incus run 34174437698 は成功しました。
Windows run 34173412761 は実行中です。

Native の再開位置保存は、リンク・特殊ファイルを拒否し、読み取りを制限し、所有する
親ディレクトリを固定し、同期した atomic 保存とプロセス存続中の単一 writer ロックを
実装しました。別プロセスの競合と親の差し替えを含む対象テスト・vet は成功しました。
Windows 自動起動は実装済みです。native 経由の新規判断の受入は引き続き未完了です。


専用 Hacocoon-Review-6771f2f の実 Host でも、作業中の通知バイナリを所有する一時
ディレクトリに置いた component 検証が成功しました。監査の投影なしで既存 controller
を購読し、公開スキーマと待受の終了を確認しました。一時実行ファイルは削除済みです。
最初の Windows マウントからの直接実行は、配布後の権限を要求するテストで失敗し、
0755 の一時コピーで再検証して成功しました。通常インストーラーの配布と native 起動の
証明ではありません。


ローカルの release-provenance 検証は **失敗** しました。検証用 Ubuntu が 22.04 で、インストーラーは 26.04 以上を要求するためです。その後、専用 Ubuntu 26.04 で Git の対象作業ツリーをコマンド内だけ指定し、同じ release-provenance 検証が成功しました。永続的な Git 設定は変更していません。22.04 での失敗は記録に残し、通常の通知パッケージ受入は pending とします。


## 通常 Host の通知経路の追加

作業ブランチで実装済み: 同じリリースの通知バイナリ配布、ローカルへのフォールバックを
しない controller 購読、検証済み Windows ディストリビューション名の投影。
対象 Go テストと WSL 名の回帰検証は成功しました。Windows・Ubuntu のワークフローへ
インストール済み Host の購読 E2E を追加しましたが、新しいパッケージでの実行は pending です。
以前の Physical Host での成功は今回の通常 Host 経路の証明ではありません。
通知から人が新規要求へ回答する操作も未検証です。



追加の読み取り確認で、installed haco-host には haco-notify と監査ファイルの両方がないことを確認しました。
上記 native の証拠は Physical Host からのものです。日常の D2 利用を完了扱いにする前に、
通常 Host からの通知購読を優先して整える必要があります。


## Windows 通知からの承認確認

状態: **adapter の一段階を実装済み。ロードマップ D2 は partial**。
Windows package に native helper、checksum、distribution ごとのユーザー protocol 登録を含めます。
通知は正確な要求 ID だけで既存の承認 console を開き、回答や管理 endpoint の公開は行いません。
[契約](design/pending-approval-review.ja.md)と [ADR 0030](adr/0030-windows-notification-review.ja.md)を参照してください。

実機 Hacocoon-Review-6771f2f で、登録・古い要求／不正リンク拒否・Windows protocol から
正しい helper の起動・期待する URI を持つ通知履歴を確認しました。helper SHA256 は
e79df7c870f6218440479ea0d833e3c3d398a2499fff0eae4cc6bfd902acfc0b です。
後続の通知配送は、interop 全体は有効なのに native WSL 実行登録が欠けており、
PowerShell 起動前の system error 8 で失敗しました。installed の正規 WSL setup が
既存の検証後に登録を復旧し、最終の配送と正しい通知履歴確認は成功しました。
/init 迂回や通知側の binfmt 変更は追加していません。登録が消えた原因自体は未確定です。
最初の helper 試行は trusted Host の起動完了前で失敗しました。
維持されている local CI、関連 native テスト、package、PowerShell 構文、
文書、GoReleaser 設定検査は成功しました。
画面上の通知 click と新規回答、Linux の起動導線は未確認です。

先行する 05c8206 は GHA の test 34166655131、Ubuntu 34166655270、
Incus 34166655133、Windows 34166655142 の全てが成功しました。
Windows では local review の古い要求拒否、通常 HTTPS の ask 保存・許可・再確認拒否、
実 VS Code、preview/Edge、doctor の成功を明示的に確認しました。新しい native adapter の証拠とは分けます。

修正 observer `05c8206` と installed `6771f2f` の組合せで、実機 VS Code 1.136.1 の確認に成功しました。Environment `win-ssh-33848c2759174f10`、Windows loopback port 40429 で、リモートのファイル読み書き・terminal 実行・ローカル承認 terminal・installed controller の古い要求拒否を確認しました。通常の実 HTTPS ask 保存・今回拒否／単発許可／再確認・拒否も再度成功しました。通常 fixture は exit 0 で完了し、一時 Policy・SSH 接続・Environment・Workspace・鍵・observer ファイルを削除、listener 不在と Windows 接続拒否も確認しました。手動 SSH 設定と Remote-SSH による実機結果であり、UI で人間が新規承認する操作や OS toast 起動の証明ではありません。

`5283705` の test 34165137831、Ubuntu 34165137686、Incus 34165137697 は成功しました。Windows 34165137705 はローカル承認画面と承認テスト前提の project setup で失敗しました。リモート編集・terminal、通常 setup、preview/Edge、doctor は成功しました。

修正した snapshot を専用ローカル WSL `Hacocoon-Review-6771f2f` に導入できました（installed commit `6771f2f38f8c036a2fb16e8f9640377229b11c65`）。実機 Environment `win-ssh-d5dc5903cceb456a` で Windows SSH（port 37713）、VS Code 1.136.1 のリモート編集・terminal、通常の `haco approve` による実 HTTPS の ask 保存・今回拒否／単発許可／再確認・拒否に成功しました。ローカル承認 terminal の確認は失敗し、固定診断と失敗時の検証ファイル cleanup の回帰テストを追加しました。接続・Environment 削除と listener の接続拒否は成功しました。最初の cleanup は observer ファイルの残留で失敗し、その後、作成確認済みファイルと空ディレクトリだけを削除しました。先行する既存 instance 更新は自動承認審査で拒否され未実行です。専用 instance の導入は別途承認されています。

`6771f2f` の test 34163005164、Ubuntu 34163005175、Incus 34163005206 は成功しました。
Windows 34163005171 は VS Code 全体の受け入れ（段階はログ未表示）と承認の Python 準備が失敗し、
preview/Edge・doctor 全 4 項目は成功しました。editor timeout／remote 確認／local review、
準備の実行／保存レシピ解除を固定 phase で区別する診断を補いました。新しい UI の成功は未確認です。

ローカル snapshot build と修正 package の専用 instance 導入は成功しました。先行する version の先頭 v がない package は導入途中で失敗しました。現在の実機結果は上記を参照してください。

## VS Code の信頼された承認画面

状態: **repository 実装済み、ロードマップ D2 は partial**。
任意の UI 拡張は Review または command palette から通常の承認 CLI をローカル専用 terminal で開きます。
実行先と環境を固定し、remote/web・信頼しない window を拒否、回答は既存 CLI で入力します。
VSIX 作成に npm download は不要です。関連 JavaScript 26 件は成功しました。
実 VS Code GHA に local terminal → installed controller の古い要求の拒否を追加しましたが結果は未確認です。
OS 通知からの起動と、新規要求への人間の実回答はこの段階では証明していません。
[契約](design/pending-approval-review.ja.md) と [ADR 0029](adr/0029-local-desktop-approval-review.ja.md) を参照してください。

`0754280` の test 34161070477、Ubuntu 34161070466、Incus 34161070522 は成功しました。
Windows 34161070471 は実 VS Code、preview/Edge、doctor 全 4 項目が成功しました。
承認受け入れだけが review 前の Python 準備で失敗し、cleanup は成功しました。
setup unit・package・DNS 等を生出力なしの固定分類で識別する診断を追加しました。
原因は未確定で、これは SKIP ではなく FAIL です。診断を追加した再実行は未確認です。


生成した任意拡張の VSIX は archive/manifest 検証と、実ローカル VS Code の独立 profile への
インストールに成功しました。これは package の確認であり、新規承認や local terminal/controller の往復の証明ではありません。

## 承認待ちの確認

状態: **repository の一段階を実装済み。ロードマップ D2 は partial です。**
haco approve は候補が一つなら直接表示し、複数なら番号で選択できます。
Standard queue が background の待機を制限し、共通の private review API は
元の Git 要求も扱います。単発回答、6 種類の保存、キャンセル、期限切れ、
二重回答、session の完了所有権、Policy 変更、失敗時の安全な receipt、
実際の local Git helper 経路は関連 race／vet で成功しました。
通知から開く操作は planned です。[契約](design/pending-approval-review.ja.md)を参照してください。

installed GHA に、通常の設定操作と実際の HTTPS を使う ask 保存・今回拒否・
単発許可・再確認／拒否・対象を限定した cleanup を追加しました。実行結果は下記のとおりです。preview／doctor の失敗は、生出力を使わず固定 phase と数値を記録します。

f6d193b の test 34154746874、Ubuntu 34154746842、Incus 34154746852 は成功しました。
Windows 34154746844 は実際の VS Code、SSH、設定、project setup、doctor が成功し、
HTTP preview が失敗しました。正確な原因は未確定です。

`5ad8c3e` の Windows run 34159087435 は ask 保存・今回拒否、preview の
setup/open、doctor 呼び出しで失敗しました。実 VS Code、SSH、設定、project setup は成功。
test 34159087438、Ubuntu 34159087434、Incus 34159087447 とローカル test/E2E・docs は成功。
承認 fixture は通常の setup で Python を準備し、正常な完了行を受け入れるよう修正しました。
この修正の installed 再検証は未完了です。

ローカルの installed `71dbb4f` で Windows native SSH、接続先キー不一致の拒否、
cleanup が成功しました。port 33105、Environment `win-ssh-67210d9ab7994c7d` を使用し、
一時 Policy・接続・Environment・Workspace を削除、listener 不在と再接続拒否を確認しました。
先行する 2 回は Host 停止により失敗し、成功した実行は通常 Host ターミナルを開いたまま行いました。
自動 desktop setup は disposable GHA profile 用のためローカルでは SKIP です。
これは installed snapshot の SSH 検証であり、新しい承認 review や新たなローカル VS Code の成功ではありません。

## 承認要求の照合

状態: **照合の基礎は実装済み。ロードマップ D2 は partial です。**
承認画面・信頼された controller の応答・Git の承認待ち情報に、
capability の監査・実行結果と同じ request ID を渡します。
コマンド・承認権限・通知からの操作 endpoint は追加していません。
[Interaction Event](INTERACTION_EVENTS.ja.md) を参照してください。

`2584ec6` の GHA は test 34152700790、Ubuntu 34152700745、
Incus 34152700884 が成功しました。Windows 34152700897 は native SSH、
実際の VS Code 接続、設定 round-trip、project setup が成功し、HTTP preview と
Environment doctor が失敗しました。失敗の正確な原因は未確定です。
以前の installed 成功は別の検証結果として扱い、失敗した項目の成功とはしません。

## 承認方針の設定編集

状態: **repository の一段階を実装済み。ロードマップ D は partial のままです。**
`haco config` と任意の `--edit`／`--file` で、通常の承認保存と同じ Policy を扱います。
revision の確認と共通の private writer により、同時に保存された変更を上書きしません。
監査には操作・revision の情報だけを記録します。関連 test／race／vet と製品 CLI／
controller E2E は成功しました。設定 round-trip の installed 受け入れは 2584ec6 で成功しました。
[設定](reference/configuration.ja.md)を参照してください。

ローカル `71dbb4f` の通常インストールは Host 診断 6 項目が成功し、config の取得・反映・
receipt・実ファイル revision・監査を照合しました。default deny と元の 8 ルールは維持しました。
空の saved_decisions 配列が保存時に省略され、JSON 表示の一致は FAIL でした。
snapshot 表示の正規化と component／CLI 回帰テストを追加して成功していますが、修正の
installed 受け入れは未確認です。[正確な観測](reference/configuration.ja.md#ローカル-installed-での観測)
を参照してください。

`71dbb4f` は GHA 全 4 系統が成功しました。test 34151576434、Ubuntu 34151576429、
Incus 34151576447、Windows 34151576493 です。Windows は通常 config の往復、
実際の VS Code・project setup・Edge preview・Environment doctor を含みます。

別のローカル `71dbb4f` 検証では、`haco config --file` で `preview-71dbb4f` だけに
Ubuntu archive の一時ルール 4 件を追加・削除しました。通常の project setup で
loopback HTTP server を起動し、Windows が port 36059 で正確な Workspace marker を取得しました。
preview の再利用・閉鎖後の拒否と、runtime／Workspace／DNS の doctor が成功しました。
このローカル probe には SSH 接続を用意していません。marker・recipe・Environment・
listener を削除し、default deny・元の 8 ルール・保存方針 0 件を確認しました。
既存 Workspace `git-save-eb16300` は保持しています。過去の preview／doctor 失敗は
この実行では再現せず、元の原因は未解明のままです。

`729f008` の GHA test 34149690153、Ubuntu 34149690280、Incus 34149690192 は PASS。
Windows 34149690178 は DNS・desktop SSH／再開・実際の VS Code・project setup が成功し、
preview と Environment doctor が失敗しました。変更した fixture は両方を実行し、
最終 job を失敗に保ちました。正確な原因は未解明です。

## 通常 Git 承認の方針保存

状態: **repository の一段階を implemented。D1／D2 は partial**。既存 Git
approve／deny に任意の --save で env／全 env の allow・deny・ask を接続しました。
pending は今回の exact commit と provider が定義する再利用範囲を分けます。
OID と operation ID だけを wildcard にし、repository・remote・ref・fast-forward
update kind と属性名完全一致を維持します。永続化・監査済み応答を確認し、非対応 peer
は拒否します。実ローカル Git helper で次 commit、ask、deny、history rewrite 拒否を
確認しました。`eb16300b6700` の installed Windows／WSL で通常 SSH、ask 方針の
保存、GitHub push、次 commit の再確認、拒否時の remote 不変を確認しました。
正確な commit と保持資源は[管理 Git の検証](reference/managed-repository-workflow.md#installed-saved-approval-acceptance)
に記載しています。
[ADR 0026](adr/0026-reusable-git-approval-scope.ja.md) を参照してください。

`eb16300` の GHA test 34146281274、Ubuntu 34146281278、Incus 34146281289 は
PASS。Windows 34146281264 は FAIL です。DNS・通常 SSH・再開は成功しましたが、
VS Code が 10 分以内に完了しませんでした。後続の setup／preview／doctor は未実行です。
失敗を SKIP や現行 editor の受け入れ成功として扱いません。
検証 fixture を変更し、editor 失敗後も独立した setup・preview・doctor を実行します。
各失敗は最終 job 結果に保持します。ローカルの PowerShell 構文確認は成功しましたが、
変更した fixture の GHA 実行結果は未確認です。

953d1e5 は全 4 GHA workflow が PASS しました。修正した orchestrator／crash fixture
も含みます。それ以降の変更の受け入れを証明するものではありません。

## Policy に従う名前解決

状態: **ロードマップ C3 は partial**。installed Standard mode では canonical な
Environment 作成・再開時に guest loopback DNS service を自動導入します。
UDP/TCP の bind 後に readiness を通知し、導入・起動失敗時は成功を返しません。
bare controller mode では component は optional のままです。自動導入のために
新しい command、nameserver 引数、allow rule は不要です。名前解決そのものには
専用の Policy 許可が必要です。

既存の隔離された listener が永続化された送信元を識別し、Capability Policy と監査の
後で Physical Host resolver を使います。接続権限は別です。Windows、WSL、
trusted Host、Environment の通常 getaddrinfo と default DNS 拒否を比較する GHA
fixture は `c05528a` の Windows run 34132173483 で成功しました。
VPN/NRPT、DNS 変更・再起動後の反映は未検証です。[名前解決](design/name-resolution.ja.md)を参照してください。


`72096d8` のローカル test/vet/docs/e2e と関連 race は成功しました。
GHA の test と Incus は成功し、Ubuntu run 34121278716 と Windows run 34121278578 は
Environment DNS service の設定中に失敗しました。新しい DNS fixture には未到達です。
古い installed substrate 上の独立したローカル probe では同じ DNS unit が起動しましたが、
GHA の失敗の再現・原因の説明にはなりません。probe は canonical に削除し、
空の Workspace も削除しました。adapter は任意の guest 出力を公開せず、
許可した処理段階と数値の service 終了コードだけを返す診断を追加しています。
`a1d084b` は Ubuntu installer・Incus・test が成功し、Windows は job 制限時間で CANCELLED になりました。
`c05528a` も test・Ubuntu installer・Incus は成功しました。Windows run 34132173483 は
DNS の一致・default 拒否と VS Code 実接続に成功し、その後の project setup 検証で失敗しました。
PowerShell で生成した Bash script の CRLF により、setup 実行前の `set` が終了コード 2 を返しました。
setup と preview の script を LF に正規化しています。setup・preview・Environment doctor の実機検証は再実行待ちです。

C4 の[プロジェクト setup](design/project-setup.ja.md)は Workspace ごとの recipe を
`haco setup --script <path> <environment>` で保存・実行し、再実行・削除する部分を実装済みです。
Host recipe は既存の挙動を保ちます。起動前と実行 lifecycle lock 内で対象 identity を確認し、
script は上限付き stdin で渡します。関連 race test は成功しました。
installed GHA は `c05528a` で setup 実行前の検証スクリプトが失敗しました。
package 導入と実際の cancel cleanup は未検証です。

## 現在のdesktop開発checkpoint

状態: **ロードマップ C は partial**。desktop SSH の準備、`haco open [--client vscode|ssh] [environment]`、
保持した環境の再開、読みやすい対象一覧を提供します。Host の保存手順は `haco setup --script <path>`、
再実行、`--clear-script` で implemented で、bcc1baf のインストール済み GHA も成功しました。
一時実行 CLI は実装済みで、4adfe19 の実 Incus 検証も成功しました。より広い C1 の対象選択、C3–C5 と後続段階は未完了です。

`4f1f512` では4つの GHA workflow が成功しました。Windows job は通常の `haco open` から実際の VS Code
1.136.1 Remote-SSH に接続し、document の読み書き、terminal 実行、検証用ファイルの削除を確認しました。
使い捨て portable profile で Linux platform を保存し、Workspace trust prompt を無効化した構成です。
通常の desktop prompt や全 Windows/VPN 構成の確認ではありません。
以前の `703ec76` は executable の探索で失敗し、`506c38f` は起動後の editor 検証待ちで timeout しました。
これらの失敗と、その後の成功は区別します。

ローカルの通常 installer による最終更新は `8752431`（v0.33、build `2026-09-07T06:44:17Z`）です。
doctor の6項目が成功し、`stage-b-git-dev` と Workspace の登録を保持しました。Installer ZIP SHA-256 は
`c2c5b720643d98e586996e2d2413d1af196d764331e2b160a5bda647c76946a9` です。
この installation は現在の DNS 変更を含みません。2026-09-07 にユーザーが
`desktop-8752431` の一時 package-egress rule を許可した後、ローカル検証が成功しました。
Windows 標準 OpenSSH と VS Code 1.136.1 Remote-SSH で Workspace marker、
editor の読み書き、remote terminal、probe 削除を確認しました。専用の別 profile と
明示した Remote-SSH URI を使った検証であり、ローカルの通常 `haco open` の検証では
ありません。通常の `haco open` は別途 GHA で確認済みです。
追加した 4 rule は削除済み（残り 0）、SSH 接続 `ssh-39493` は解除済み、
Windows の専用 key は削除済み、検証 Environment は停止済みです。
`stage-b-git-dev` は停止状態を維持しました。最初の再開は WSL 再起動後に volatile
source guard が消えていたため失敗し、canonical な削除・作成で検証環境を作り直しました。
古い /tmp Workspace も存在せず、新規の検証 directory を作りました。
この失敗と、その後の接続成功は区別します。欠落 guard の再開修正には回帰テストを
追加しましたが、その修正の実際の再起動検証は未完了です。

## 永続Storeの独立コピー

状態: **storageの実装単位はimplemented、改訂B4全体はpartial**。
最新main `3b2d0b6`（PR #481 merge）を基準に、既存の
`haco plugin oci store create dev --from shared` で未接続Storeを独立複製する。
コマンド群は増やさない。正規catalogでコピー元を予約し、provider完了と検証後の
公開まで接続・削除を拒否する。失敗時は所有権を保持し、手動recoveryが必要。
中断コピーの自動回復は未実装。[Store契約](design/persistent-oci-store.md#independent-offline-copies)参照。

ローカルの実Incus 6.0.5-8 / WSL / Btrfsで合成データの受入が成功。
Parent UUID一致、両方向の書込み独立、元Store削除後のコピー保持、検証用volume・
project・poolの削除を確認した。OCI image/runtime、配布済みCLI、trusted Hostからの
publicationの成功は意味しない。初回E2Eはpool確認前にテストprojectを作っていない
fixtureの問題で失敗し、修正後に成功した。RPC回帰では不正引数がinternal errorに
分類される問題を検出し、handlerを修正した。

A/B成果は保持。PR #481は`3b2d0b6`としてmerge済み。最終`0b79cac`の
[test](https://github.com/SLktEx/Hacocoon/actions/runs/34081379821)、
[Ubuntu installer](https://github.com/SLktEx/Hacocoon/actions/runs/34081379810)、
[Incus](https://github.com/SLktEx/Hacocoon/actions/runs/34081379802)、
[Windows installer](https://github.com/SLktEx/Hacocoon/actions/runs/34081379870)
は成功済み。今回のコピー変更のCI結果とは区別する。新しいstorage testは既存Incus CIへ追加。

今回のSKIP: trusted Hostからのimage取得/publicationとコピーしたimageのcontainerd/Docker
実利用（その製品経路は未完成）、VS Codeでの実開発（今回IDE操作なし）、今回のWindows
配布物受入（installed productは前のB候補のまま）、新しいGit実push（Git・認証・refの
変更はなく、storage受入にrepoを使わない）。下記Bのpush OIDは過去に照合済みの結果であり、
今回pushしたものではない。C-Gは更新した[ロードマップ](status/architecture-and-roadmap.md#user-facing-development-order)に沿うplanned項目。


## Storeコピー実装単位の検証

ローカル成功: 維持CIの`test`（Go test/vet・installer component・JavaScript）、
`e2e`のcommand/capability/Git/orchestrator assertion、docs/workflow policy、
実Incusの合成データCOW検証。`forwarding`は最初sudoの認証で失敗したが、開発用WSLで
同じ隔離namespaceテストをrootとして実行して成功した。初回E2Eは本体assertion成功後、
一時Go module cacheの権限でcleanupエラーが出た。製品assertionの失敗とは分ける。
既存`GOMODCACHE`を明示してcommand E2Eだけ再実行した結果、cleanupエラーなく成功。
以前の一時パスが存在しないことも別途確認した。

初回全体`race`は既存CONNECTのupstream-prefix停止テストで失敗した。
serve側がCONNECT上流を同期closeし、停止後に完了したdialをwrite前に拒否するよう修正。
proxy packageのrace反復100回と、その後の全体`race`が成功した。
[egress契約](EGRESS_AUTHORIZATION.ja.md)参照。認可Policyや利用コマンドは変更していない。

`bash tools/ci-local.sh`全体実行は、開発用Ubuntu WSLに`pwsh`がないため
release-configで**失敗**した。Windows PowerShellでinstaller component単独検証は成功。
配布由来の検査も最初Ubuntu 22.04の最低OS条件で失敗したが、Ubuntu 26.04でfixture限定の
検査を行い、由来・installer package契約が成功した。rootとWindows所有者の違いは、その
検証プロセスに限り既知worktreeだけをsafe.directoryとして扱って解消。GoReleaser設定検査も成功。
全release archive buildとfresh package installは今回は**SKIP**。

新しいGHA実行は**SKIP／公開阻害**。自動承認レビューが、push検証の明示許可先は
`SLktEx/Hacocoon-test`だけとして、実装branchの本体`SLktEx/Hacocoon`へのpushを拒否した。
sourceのpushもPR作成も行っていない。新しいCOWテストは既存Incus workflowに追加済みで、
公開が承認されれば実行可能。過去のBのCI成功を今回のrevisionの成功として扱わない。

## Incus起動時のPID再利用防止

Status: **implemented。repository回帰とhosted Ubuntu/WSL配布packageの受入は成功**。
共通Ubuntu/WSL installerはroot専用のIncus ExecStartPre guardを導入する。
前namespaceのdnsmasq/proxy PID記録をdaemon起動前に退避し、同一namespaceの記録と
resourceデータを保持する。[Host契約](design/trusted-host.ja.md#incus起動時のpid記録)と
[ADR 0013](adr/0013-incus-pid-record-boot-identity.md)を参照。

19件のcomponent回帰で再利用PID、WSL/native起動、service再起動、初期導入、同時実行、
中断復帰、不正metadataを確認した。installer/packageとWindows driver回帰も成功した。
Windows配布gateには再起動後のmarker更新とdnsmasq記録の退避確認を追加した。
同一namespace内のhelper PID再利用と任意device familyは上流側の残課題とする。

`1b2d6ae`の[Windows配布gate](https://github.com/SLktEx/Hacocoon/actions/runs/34051931616)で、
install、通常のWSL終了・再接続、marker更新、dnsmasq記録の保持、installer再実行、
install済みegress制御が成功した。
[Ubuntu配布gate](https://github.com/SLktEx/Hacocoon/actions/runs/34051931562)、
[実Incus gate](https://github.com/SLktEx/Hacocoon/actions/runs/34051931583)、
[repository CI](https://github.com/SLktEx/Hacocoon/actions/runs/34051931607)も成功した。
任意のauthenticated-private-registry jobはskipであり、private-registryの受入実績は追加しない。
最終revisionとmerge状況は[PR #480](https://github.com/SLktEx/Hacocoon/pull/480)に記録する。

当時のhosted受入はローカル環境を更新していなかった。下記の改訂Stage B fresh受入には、
main由来のこのPID guardも含まれる。

## WSL起動失敗の調査 — 2026-09-07

Status: **historical調査。起動失敗を再現し、古いPIDの再利用が原因であることを強く裏付けた**。
namespaceをまたぐ再発防止は上記のとおりimplemented。
対象は手元のWSL 2.7.12、製品 `029ff08e34c98e075b7b0b3d3a7fc7f639e89323`、
Ubuntu package `incus 6.0.5-8`。別Windowsアカウント・別端末の原因を確定するものではない。

02:32:21 JSTの成功起動では `haco-host0` のdnsmasqにPID 424が割り当てられていた。
通常のWSL終了・再起動後、02:33:13にkernel traceで
`kill(424, SIGKILL)` の対象が `libuv-worker` になっていることを捕捉し、
直後にIncus本体PID 248がsignal 9で終了した。traceの対象kernel PIDは9468であり、
424は呼出側namespaceのIDなので番号空間を混同しない。
独立したprocess一覧でlibuv workerがIncus本体のthreadであることを確認した。
OOMの記録は確認できていない。

上流の[v6.0.5 dnsmasq終了処理](https://github.com/lxc/incus/blob/a87f49a2491fa3a0e74896c1f2322bd356c59ddc/internal/server/dnsmasq/dnsmasq.go)は
保存済み `dnsmasq.pid` を読み、
[`Process.Stop`](https://github.com/lxc/incus/blob/a87f49a2491fa3a0e74896c1f2322bd356c59ddc/shared/subprocess/proc.go)を呼ぶ。
数値PIDの存在確認後に終了させ、boot・process開始時刻・実行ファイルの同一性は検証しない。
以前のdnsmasq PIDがIncusのthread IDとして再利用される経路と一致する。
失敗したsignalのuser-space呼出stackそのものは未採取であり、
PID・対象thread・直前のdnsmasq identity・失敗時刻を照合した結果である。
正常なforkproxy/helper終了でもSIGKILLが出るため、それだけで本体の障害と判定しない。

テスト専用の子processだけを使った独立検証では、
`pidfd_open(worker_tid)` はENOENT、数値IDへのsignal-0は成功し、
同じTIDへのSIGKILLでprocess全体が終了した。
これはOS上の誤終了の仕組みの検証であり、Incus修正の適用や上流packageのE2E回帰検証ではない。

起動中にIncus本体が終了すると、600秒の `waitready` start-post processが残り、
controllerの `After=incus.service` が待機を続け得る。
その結果、製品loginの2分の期限が切れる。login clientは最後のtransport errorも捨てるため、
timeout表示だけでは本原因やsocket権限の失敗を区別できない。

一時kernel trace・uprobeは全て解除した。時間限定の観測でIncus serviceを明示的に1回再起動し、
後続のinstall済み `haco doctor` は全6項目成功した。
回復は間欠障害の解消を意味しない。provider binary、PID file、storage、network、
install済みservice設定へのpatchは行っていない。
正しいprocess所有identityの検証はproviderのlifecycleの責務であり、
CoreによるPID fileの自動削除やlogin timeout延長は根本修正ではない。

## 現在のStage B改訂

状態: **implemented。改訂後のローカル配布物・実機受入は完了**。mainの基準は
`0665ba9`。これは作業branch候補の確認であり、公開releaseや他の未merge PRの完了を
意味しない。対象はnative WSL Interop、実在する複数drive、Persistent OCI Store、
Windows標準OpenSSH。`switch-base`は公開CLIで無効、Stage D以降で再検討し、A-Cを
blockしない。過去のコード・ADR・証拠は残す。[roadmap](status/architecture-and-roadmap.md#current-stage-b-scope)、
[OCI Store](design/persistent-oci-store.md)、[Windows SSH手順](reference/windows-environment-ssh.md)を参照。

**実機の配布物:** `c86c43e4f2702c5fccadd91f542d82bd8b733706`、checkpoint `v0.29`、
`0.27.0-SNAPSHOT-c86c43e`、build 2026-09-07 10:38:30 JST。Windows ZIP SHA-256は
`39205fea38aa7b38f8474edb6f957363b3565a22a00e6957d64ba542217566db`。
後続commitはテスト・文書の改善であり、実際に導入した製品はこの候補である。
構成はWindows 10.0.26200.9278、WSL 2.7.12、kernel
`6.18.33.2-microsoft-standard-WSL2`、Ubuntu 26.04、Incus `6.0.5-8`、
Incus所有Btrfs pool `haco-local-default`。既定Baseのrevisionは
`sha256:297ce79fb308c09126222dd6e64c260003c5d1e1ea1ce46ea43e80a419941636`。

**fresh手順:** UbuntuとUbuntu-24.04だけがありHacocoonがない状態を確認し、branchから
作ったZIPを展開して通常の`install-windows.bat`を実行。`wsl -d Hacocoon`からHostへ入り、
doctor全6項目が成功。既存Windows installer gateでWSL終了・再入場、Host内データ保持、
同じBATの再実行、cold doctorも成功した。追加でHostだけの
`incus restart haco-host --project hacocoon`直後を再設定なしで確認し、実際の
`wsl --shutdown`後の通常入口と、その後の通常`haco setup`再実行・doctorも確認した。
最終配布物の受入では、source由来binary、mount、PATH、socket、serviceの手動修復を
注入していない。Windows/WSL機能自体は導入済みであり、Hacocoon distributionのfresh
installであって、OS再インストールやWindows OS再起動の試験ではない。

| 項目 | 最終候補で確認した結果 |
|---|---|
| B1 drive | 実在するDrvFsからCとQを検出し、`/mnt/c`と`/mnt/q`へ投影。両NTFS driveでWindows作成ファイルをHostからread、HostのwriteをWindowsからreadし、空白入りpath/argumentも成功。WSL削除前のファイルもfresh install・Host restart・WSL shutdown後に保持された。同じWindows filesystemを参照しており、Environment内のコピーではない。 |
| B1 exe/PATH | 明示的な`/init`なしの絶対path cmd.exe、`cmd.exe /c ver`、`powershell.exe -NoProfile -NonInteractive`、Windows PATH上のwhere.exe/findstr.exeが成功。stdoutのhello、stderr marker、終了23を確認。検出drive下にあるWSL変換済みWindows PATHだけを継承。OCI/SSHのEnvironment作成・削除後も、開いたままのHostでInteropが動いた。 |
| 旧B3 | 配布物の`haco env switch-base`はcontroller操作前に終了2、currently disabledとStage D案内を返す。通常のEnvironment作成時のBase選択は維持。 |
| B4 | 別々のprovider identityを持つ2 Storeを作成。Aをattachし、使用中deleteを拒否。BusyBox pull、local image build/run、Environment削除、Store保持、別Environmentへの再attach、registry接続なしのimage再利用、buildのCACHEDを確認。別Storeは空でWorkspaceは保持。Environment deleteはStoreを残し、明示Store delete後のinspectはnot found。 |
| B5 | Windows標準System32/OpenSSH/ssh.exeから127.0.0.1:22229、WSL Physical Host、Incus loopback proxy、Environment sshdへ実接続。生成configはstrict checking、dedicated known_hostsをtrusted provider由来の公開host keyでpin。/workspaceを利用でき、鍵不一致は実行前に失敗。disconnect/delete後はWSL listenerが消え、Windows再接続も失敗。終了後のWindowsエラーはrefusedではなくtimeoutになる場合がある。 |
| B2/A/B6 | 一つのstage-b-git-devで独立した2つの.gitを確認し、commondir/alternates共有なし。Windows SSHでfetch/pull、編集、commit、製品helperの承認付きpushが両repoで成功。disconnect/stop後のstatusはEnvironment・Workspace・Base・保持状態を表示。通常WSL再入場後にもEnvironmentが停止状態で、Workspaceの未追跡noteが残ることを確認した。 |

OCIの実機版はcontainerd `2.2.2-0ubuntu1.1`、nerdctl `2.3.5`、BuildKit `0.33.0`。
各Environmentへ必要なruntimeとdaemon proxy設定を通常導入し、download Policyは
対象Environment・宛先に限定した。`pull --unpack=false`でcontentを保存し、
`run --snapshotter native`でsnapshotを用意する。imageとBuildKit cacheはStore、
process/socketは`/run`に分離。再attach前後でBusyBoxのIDは
`sha256:c6348fa86ba0fb2108c9334f5fe913ddc6d853313e655891f133a0127c30099f`、
local imageのIDは`sha256:475bcd7010f2b330b1b82f7a43a911baeb6be801dfd1d2fb2d6b7b498a99c7bb`
で一致した。Docker Storeの互換性は**未確認**。下記の旧Docker配布受入は過去の証拠である。

Git成果の送信先は`https://github.com/SLktEx/Hacocoon-test.git`のみ。
指定に従い同じURLの異なるbranchを2 repoとして登録した。異なるremote URLの動作は
local real-Git回帰で扱う。両pushともrepo・Environment・URL・ref・操作・旧OID
`f4ff6e33588a7183b0c7d3db2f4c2214a527678f`と固定新OIDを照合して`haco git approve`を実行し、
Windows側の独立した`git ls-remote`で一致を確認した。

| repo / branch | 確認したremote commit |
|---|---|
| stage-b-first / codex/stage-b-20260907-first | `7f9f9ecaaae1cc332c3a42d9724eeddbb9701f4d` |
| stage-b-second / codex/stage-b-20260907-second | `98168553a91e20f2f97b0658bfd305ab4ed488e6` |

Btrfsの観測ではsource UUID `8dbe4029-79b1-5f46-96d1-522b9cf9fd6a`と
`d84cae46-8f22-2144-99c8-d7d6ac5a6c9f`が、それぞれWorkspace
`a558d778-1ac5-1047-9c04-d72568d530ff`と`8b7b06f4-8b40-7d4d-92e8-4643074ca769`の
parent UUIDに一致した。独立COW関係の証拠であり、性能評価ではない。
`managed:stage-b-both`と停止済みEnvironmentは利用者向けに保持している。

**境界・検証:** Environmentに/init・WSL socket・Windows drive・Windows exe権限がない。
client秘密鍵はWindowsでのみ作成・削除し、公開鍵だけをHacocoonへ渡した。Git資格情報は
trusted Hostのroot専用標準gh storeだけに置いた。既存Windows Git tokenの本人性と
テストrepoのpush権限を公式gh apiで確認し、stdin経由で設定。tokenのログ出力や
Environmentへの保存はない。gh auth login --with-tokenは追加OAuth scope不足で失敗したが、
権限を拡張していない（[GitHub CLI仕様](https://cli.github.com/manual/gh_auth_login)）。
これは手動の資格情報設定であり、新しい製品credential brokerではない。

installed egress検証は許可HTTPS、拒否proxyの403、直接TCP拒否、管理socket非共有が成功。
local CIのdocs、workflow policy、Go test/vet、JavaScript、race、E2E、隔離namespaceの
kernel forwardingが成功した。release/package、native interop、lifecycle ownership、
guest systemd readiness回帰も成功。対応systemdが必要なrelease checkはUbuntu 26.04、
Windows installer/BAT componentは実PowerShell 7/5.1で確認。開発用Ubuntu 22.04では
release phase全体をそのまま実行できず、対応platformで各componentを分けて実行した。
新しいhosted CIやprivate registryの受入を意味しない。

再実行用driverは[Windows installer](../tools/windows-installer-user-path-e2e.py)、
[native access](../tools/windows-native-access-e2e.py)、
[Windows SSH](../tools/test_windows_environment_ssh.ps1)、
[OCI lifecycle](../tools/test_persistent_oci_store.py)、
[installed egress](../tools/installed-egress-check/main.go)。
ローカル証拠はbin/stage-b-c86-*、特にfresh-package-gate、persistent-oci、
native-ssh-cleanup、approved-git-standard-credentials、two-repository-cow、
retained-workspaceのログに保存した。ログや資格情報はsource archiveに含めない。

Git認証・Policy、SSH key/config/pin、任意OCI runtime/proxy設定は手動。
Windows/image/runtimeの広い互換性、drive着脱、開いたsession中の外部操作によるWSL
binfmt登録削除、異常切断、upgrade、汎用復旧は未確認。以前観測したhandler消失のtriggerは
未特定であり、通常setup/entryはhandlerがない場合だけWSL自身の生成serviceで補完する。
D+の自動化、switch-base再検討、registry/broker、Store同時共有、live migrationは
[後続課題](status/development-follow-ups.md)に残す。

## 過去の第二段階（対象範囲を変更済み）

以下は以前の依頼に対するcommit固定の実行証拠。旧B3と配布専用B4は現行要件ではない。


状態は**implemented・以下のWindows/WSL構成でB1〜B6を受入済み**。
Dockerとnerdctlの両方で一方向配布・独立起動が成功した。
選択したB5/B6改善と、影響を受けるAの基本導線も確認済み。
[利用手順](reference/managed-repository-workflow.md)、
[OCI契約](design/persistent-oci-store.md)、
[残課題](status/development-follow-ups.md)を参照。

| 段階 | 実装と確認結果 |
|---|---|
| B1 | trusted Host限定の明示setupで既存DrvFsドライブを検出。PowerShellへ独立した空白付き引数を渡し、stdout/stderr・終了23を確認。利用者所有`/mnt/c`の読み書き成功。最終候補029ff08でも再確認。実機はCのみ。追加ドライブは解析回帰のみで実機未検証。EnvironmentへWindows device・`/init`・interop環境変数は渡っていない。 |
| B2 | 配布物087e7e2でb-dev内の`/workspace/b-first`・`/workspace/b-second`を確認。独立Btrfs copyと専用.git、alternates/commondirなし。SSH fetch/pull・編集・commit・固定内容承認付きpushが両repoで通った。GitHub側OIDは`be34f60c2c3d1ab5761e821fbdaada5e4d5802dc`と`b834ee67dbc8f5e37e73656f13872d42ceda40f3`。異なるremote URLもローカル実Git回帰で確認。 |
| B3 | 配布物3747baeでBase一覧と26.04→24.04切替。全54ファイル（.gitを含む）のハッシュ一致。未push commit bce47b9 / 6d5fc53、未コミット変更、未追跡notesを保持。Git/SSH再接続、新host key固定、Ubuntu24.04.4上のSSH編集も成功。 |
| B4 | 配布物029ff08の`haco plugin oci distribute --runtime <runtime> --image hacocoon-b4:smoke b-dev`でDockerとnerdctlを個別に確認。通常SSHからguestコンテナを起動・ファイル変更・停止し、対応するHostコンテナが元の内容で稼働し続けることを確認した。両driver、export失敗、入力・サイズ上限・instance内固定socketの回帰も成功。 |
| B5 | 配布物029ff08の`haco env ssh-config b-dev`で生成した設定から通常SSH接続が成功。host/port/userの手動転記を削減。Incus6.0.5にないconfig show --formatをJSON query APIへ修正し、回帰を追加。 |
| B6 | 配布物029ff08のenv statusで対象Environment・状態・Workspace・access・Baseを読みやすく表示。停止時はWorkspace保持を明示。機械可読出力は--jsonで取得できる。 |

実push先は指定の`https://github.com/SLktEx/Hacocoon-test.git`だけ。
branchは`codex/stage-b-b-first-20260906`と`codex/stage-b-b-second-20260906`。
実機の2登録は同じ許可済みURLの別branchを使用し、別URLの振分けはrepository回帰で確認した。

Btrfs source UUID `411102dc-d913-264a-96a0-b09d079eb898` /
`58ccd7df-d8df-3444-98b4-67b35d85018e`が、それぞれWorkspace volume
`49eff338-40d8-244b-9276-e35952b475b2` / `a23fadbc-77af-be4a-b7a9-f9829e96e613`
のparent UUIDに一致した。実際のCOW関係の確認であり、性能計測ではない。

最終導入候補は`029ff08e34c98e075b7b0b3d3a7fc7f639e89323`、checkpoint v0.28、
snapshot `0.27.0-SNAPSHOT-029ff08`、build時刻`2026-09-06T10:25:40Z`。
Windows ZIP SHA-256は`20f308cb5bcccfdaef1f0c76914bdae65834c957afd6c446fd0effdda26717fe`。
各候補のbranch commitから配布物を作り、通常BATで既存Hacocoon WSLへ適用した。
製品の検証用overrideや内部state修復は使っていない。fresh導入は再実行していない。
保持したA構成はWindows26200.9278 / WSL2.7.12 / Incus6.0.5、Incus所有Btrfs
pool haco-local-default。

B3の26.04 revisionは`sha256:d071290fb40659981198baf0161a8bcc9910ebae79a15f5ef5d9c06dbdb2ea4c`、
切替先24.04は`sha256:f38ca805517f5b6e301f33b0f44523386c5a050847564c1233e586106b31dbc9`。
後の26.04明示作成では`sha256:297ce79fb308c09126222dd6e64c260003c5d1e1ea1ce46ea43e80a419941636`
へ解決された。先に作成したEnvironmentの固定revisionは変わっていない。

最終候補のA回帰では単一repo b-a-work / b-a-devを通常作成。生成SSH設定、
fetch・f4ff6e3からbe34f60へのfast-forward pull、Python compile/assert、commit、
push拒否（remote不変）、続く承認pushが成功。GitHub側で第一branchの
`145fd7fce49a5a8771e39e7b142d47aa49c910c3`一致を確認。disconnectと正常stop後も
全28ファイル（未コミット・未追跡・Git状態）とcanonical lease・volumeを保持。
内外clientとcontrollerのbuild一致、doctor6項目も正常。元のA資源は保全した。
OCI導入後も両repoのSSH/helper fetchが成功し、許可proxy通信・外部直接TCP拒否・
guestへのWindows interop/controllerパス非公開を再確認した。通常の`env stop b-dev`後も
全57Workspaceファイルのハッシュとcollection所有権を保持し、Hostの両コンテナは稼働継続。
b-a-devとb-devは停止している。

B4構成・結果（2026-09-06）：所有確認した非privilegedのhaco-hostとhaco-b-devに
明示的にnestingを設定し、既存deviceとnetwork guardを保持した。両側へUbuntuの
docker.ioを独立導入し、Docker29.1.3、containerdはHost2.2.2・guest2.2.1。
公式最小nerdctl2.3.5配布物のSHA-256は
`de3206aeb7cbd5f20f5fb1f55c1e3bf2db1be567812a8a3f5e65eba2488347ee`。
full bundle・privileged化・AppArmor無効化・runtime device共有は不要だった。
イメージはUbuntuのbusybox-static、shell symlink、固定/data/messageだけを含み、IDは
`sha256:9bafa1f9ed06b9fcc33ef5b6674ef3c4d79ae819b7724d5b228923712112b46f`。
両方の製品配布は1,183,232 bytes、archive SHA-256は
`a2ea9ac81b39572d424bd2b63461ac659c2b0a4c327ccb963e110f08ed553c57`。
両方とも--network noneで起動。Docker guestはguest-only、nerdctl guestは
nerd-guest-onlyへ変更したが、Host側は両方host-originalのままだった。
[再現手順](design/persistent-oci-store.md)を参照。

検証はci-local.shのdocs・workflow-policy・test（Go/vet/JS）・race・e2eが
B5/B6変更後に通過。関連するlifecycle/Git/collection mount/OCI/SSH設定回帰、
GoReleaser check/buildと配布checksum、独立Linux network namespaceのforwarding jobも成功。
ローカルGoは1.27.1。release-configとinstaller/provider jobを含むhosted CI結果は
[PR #473](https://github.com/SLktEx/Hacocoon/pull/473)に記録する。
広い実機runtime/network matrixの受入は主張しない。

手動操作はB1のPhysical Host設定、trusted側認証と限定Policy、client所有SSH鍵と
host key固定。別WSL distroのloopbackから届かない構成があり、controllerの
Physical HostまたはWindows loopbackを使う。SSH内proxy exportは既存#469として残る。
Base切替ではroot filesystem/packagesを破棄しGit/SSH再接続が必要。
B4は各instanceへの明示nesting/runtime設定が必要で、Base交換後は再設定する。
以前の自動実行レビュー拒否はB完了の再依頼後に解消し、対象instanceを固定した設定と
既知の検証image配布が実行・成功した。追加Windowsドライブは実機になく未確認。
広いWindows/image互換性、中断処理、汎用復旧は未検証として残る。

## 管理対象repoのWSL利用経路 — 2026-09-06

**implemented・以下のローカルWindows/WSL構成でA1〜A6を受入済み**。v0.27候補は、新hacoのrepo登録、
独立したIncus Btrfs Workspace copy、controller経由のEnvironment作成・SSH、
Git専用remote helper、Workspace所有権を保持する正常停止を実装する。
認証付きGitはtrusted `haco-host` 内で実行し、Policy・承認・state・Incus権限は
Physical Hostのcontrollerに置く。実Gitを使うローカル回帰では通常fetch・競合のないpull・
push拒否・旧/新OIDを固定した承認付きpushが成功し、承認待ち中のlocal branch変更でも
送信対象が変わらないことを確認した。repository検証と以下の実機観測は区別する。
[利用手順](reference/managed-repository-workflow.md)と
[所有権の決定](adr/0008-managed-repository-workspaces.md)を参照。

**配布物の受入:** commit `7a4d1227c95642f27cb118c3d20d2cd554e8be32`、
version `0.27.0-SNAPSHOT-7a4d122`、build `2026-09-06T07:57:54Z`。
Windows ZIPのSHA-256は
`0468c8f95c5b431c5d4160aead860deb152ed8d8e381b321c6b85b2f650d1a80`。
Windows build `26200.9278`、WSL `2.7.12.0`、kernel `6.18.33.2-2`、
Ubuntu 26.04、Incus `6.0.5`、Incus所有Btrfs pool `haco-local-default`。
既存Hacocoon distributionへ同梱BATを通常実行し、doctor全6項目が成功して終了0。
CI専用の製品設定や内部資源の修復は使っていない。この候補の未登録distroからのfresh導入は
**未再実行**であり、下記の過去installer受入とは区別する。

| 段階 | 観測結果 |
|---|---|
| 入口・controller | 通常の `wsl -d Hacocoon` でtrusted Hostへ入り、内外のhacoが同じbuildと `poc-dev` を返した |
| repo・COW | `https://github.com/SLktEx/Hacocoon-test.git` の `codex/wsl-poc-20260906` を `poc` として登録。独立copy `poc-work2` を `poc-dev` の `/workspace` に配置 |
| 既定Base | `haco/ubuntu-26.04`、revision `sha256:d071290fb40659981198baf0161a8bcc9910ebae79a15f5ef5d9c06dbdb2ea4c` |
| 開発 | client所有鍵と固定host keyによる標準OpenSSHで編集、Python byte compile・unittest 2件・commitが成功。専用.gitを持ち、commondir/alternates・Host gh認証ファイル・管理socketはなく、trusted元worktreeも未変更 |
| fetch/pull | 同じWorkspaceで通常helper fetchと `pull --ff-only` が `f4ff6e3` からremoteで作った `19caa79e123b981227d1c0b58783c7a6af80e930` へ進んだ |
| 拒否 | `haco git deny` 後のremoteは `19caa79` のまま |
| 承認 | proposalの登録URL/ref・操作・`19caa79` → `c18cbb8e202cecc0d6c80b29a8cd700dc1c0558f` を確認してapprove。通常git pushが終了0となり、GitHubからも同じOIDを取得。auditの拒否・承認・成功を確認 |
| 終了 | SSH commandが終了し、`env disconnect poc-dev ssh-2222` と `env stop poc-dev` が成功。内外statusはstopped、再接続は拒否 |
| 保持 | canonical leaseとcustom volumeを保持。未push HEAD `5650953d591fc6294a0db8db5f71a408e7917555`、変更済greeting.py、未追跡notes、branch refのSHA-256は停止前後で一致。remoteはc18cbb8のまま |

元repoのBtrfs UUID `760d7b7e-0e0e-7f4c-9f88-7303ad96f55c` と、Workspace
`f3326bfe-8cb0-684e-a3a3-d437dd3b817e` の親UUIDが一致した。COW関係の観測であり、
性能計測ではない。最初の候補 `c116307` はrepo登録後、copyのIncus ID-map履歴を
落としてEnvironment書込み検証に失敗した。provider回帰と `7a4d122` の修正で履歴を保持する。
失敗copy `poc-work` は保全し、同じ登録repoから通常workspace createで作った新copyで
受入した。chownや特権Environmentによる回避はしていない。

**残る手動setup:** trusted Hostにgit/ghを導入して認証、Physical Hostで対象Git/Ubuntu
取得だけをPolicy許可、SSH公開鍵の準備・host key固定、SSH shellでcredentialを含まない
Standard proxy URLをexport。既存の許可済gh credentialは標準入力でtrusted Hostだけへ
渡した。適用前のHost HTTP/HTTPSはtimeoutしたが、通常BATのsetup後はreadinessが成功した。
別途の修復は行わず、最初の失敗原因は未確定。

**repository検証:** `ci-local.sh docs`・`workflow-policy`・`test`（全Go、vet、JS）・
`race`・`e2e` が成功。env未実装を前提にした既存E2Eを更新し、一時HOME外の既存Go cacheで
実行した。ID-map修正は関連race回帰、配布物はGoReleaser check/buildとchecksumが成功。
完全なrelease-config/forwarding jobや新しいhosted CI実行の成功は主張しない。

**deferred・未検証:** SSH proxy自動設定は [#469](https://github.com/SLktEx/Hacocoon/issues/469)、
承認中断・push結果不明・retryは [#470](https://github.com/SLktEx/Hacocoon/issues/470)。
次の依頼のB1は既存 [#275](https://github.com/SLktEx/Hacocoon/issues/275) の境界に沿った
trusted haco-hostのWindows exe実行・利用可能なWSLドライブmountであり、Environmentへ自動公開しない。
B2/B3の複数repo・Base変更、大きなpack・他認証方式・force/複数ref・LFS/submodule、
汎用復旧/削除・resume UX・広いhost matrixは未受入。test Environmentは停止し、両copyと
test branchを意図して保持した。B/Cの実装は今回に含めない。

以下のM1記録は各記載buildに対する過去の観測として保持する。

## WSL向け更新 — 2026-09-06

この候補branchのWSL向け実装を以下に示す。今回指定されたWSL M0–M1の範囲は **implemented、受入済み**。更新main `e8974ef` は#441/#442/#453/#456と#458/#459を含む。後続のrelease準備・取消2commitの最終ファイル差分はなく、取込merge `b58f82c` の製品treeは受入対象 `c749ff9` と同じ。merge済みPRや過去のgreenを後続製品変更の受入としない。

- **storageはimplemented:** 配布/runtimeはIncus所有Btrfsだけを使う。外部 `driver`/`source` attachmentや不確実な検査はfail closed。desired policyは `compress=zstd:3,noatime,nodiscard`。[読み取り専用mount診断](design/btrfs-storage-layout.ja.md#読み取り専用のmount診断)は設定・検証済みlive反映・反映待ち `pending` を区別する。backing device/inode、単一の全image loop関連付け、Btrfs root mountの一致を要求し、不明・不正・観測中の変化を成功としない。独自のimage/loop/mount lifecycleや診断修復は追加しない。
- **installerとtrusted Hostはimplemented:** 既定はnon-root `hacocoon`、passwordはlocked。`-InteractiveUserSetup` は任意。現在版再実行はaccount識別/password状態を保持し、sudo policyを書かない。controller所有の `haco setup` が所有trusted hostと限定endpointを準備し、common installerは製品doctorの全項目成功を完了条件とする。fresh hostはprofileを継承せずdeviceを明示する。所有 `haco-host0` はtrusted基盤向けDNS/DHCP/NATを提供し、Docker転送許可はそのbridgeと戻り通信だけに限定する。[bootstrap](WINDOWS_WSL_BOOTSTRAP.ja.md)、[trusted Host](design/trusted-host.ja.md)、ADR [0004](adr/0004-wsl-installer-authority.md)・[0005](adr/0005-trusted-host-network-ownership.md)・[0006](adr/0006-controller-owned-host-setup.md)を参照。
- **製品CLIはpartial:** 新 `haco` はhelp/version・`setup`・`doctor`・controller経由WSL login aliasを提供し、`hacoq` を呼ばない。controller state・Policy・provider・Incus権限はPhysical Hostが所有し、guestにcontrollerやIncus daemonを置かない。[診断](design/controller-client-transport.ja.md#host診断)は順序付き6項目と長さを制限した失敗/pending actionを返す。controller待機とguest DNS/routeの読み取り専用起動待機は、失敗した外部検査の再試行やresource修復をしない。広いlifecycle/Base/SSH CLI移行は別件で、#456のcontroller adapterは再利用できる。
- **Standard proxy lifecycleはimplemented:** install済みcontrollerは固定proxy listenerを所有し、bind前に共有guardを検証する。control/proxyの停止を連動させ、hijack済みCONNECTも閉じる。daemonはambient approval providerを持たず、exact allowのauditを維持し、require-approvalはfail closed。同PID listenerと未管理元拒否はEnvironmentの許可通信と区別する。[ADR 0007](adr/0007-controller-owned-standard-egress.ja.md)を参照。
- **repository検証 — `c749ff9`:** 対象race/vet、pendingのCLI/API回帰、Windows assertion 9件、installer実shell 5件、shell構文、文書検査が成功した。維持する `ci-local.sh test` から全Go shuffle test・vet・JavaScript構文2件・notification test 5件が成功した。先行local vetは `bin/` に取得した調査用sourceを含めて停止したが、その観測資料を `.txt` に直してentry point全体を再実行し成功した。製品環境変数のoverrideやinstall済みresourceの修復は与えていない。
- **Seed撤去はplanned:** codeは残り、[Base/任意OCIとの依存](design/oci-seed-and-cow.ja.md)を保持する。Base選択と任意Pluginの境界は維持する。
- **登録時の続行はimplemented、Windows package受入済み:** WSL一覧取得失敗を不存在とせず、native作成成功後も対象名の登録を読戻し確認する。作成/読戻し失敗時は手動で現在版BATを再実行するための段階/option記録を保存する。記録は権限を与えず、実行もしない。明示的な終了3010は再起動待ちとして伝え、終了0でも未登録なら未完了とし、再起動案内は条件付きにする。PowerShell 5.1 component testと実BATの終了code伝達testは成功した。これらはWindows機能installやOS再起動の受入ではない。[bootstrap続行](WINDOWS_WSL_BOOTSTRAP.ja.md#登録の中断とwindows再起動)を参照。

package受入の対象は **`c749ff9033b33c3526e108f60ce2009638075152`**:

| 環境 | 実測した受入 |
|---|---|
| [Windows gate](https://github.com/SLktEx/Hacocoon/actions/runs/34008408570) | 正規cached BATのfresh作成、通常入口、停止/再入場、同版再実行、cold doctor、build識別、保持、proxy所有、未管理元403が成功 |
| [Ubuntu installer](https://github.com/SLktEx/Hacocoon/actions/runs/34008411207) | 配布物からのordinary-user installとtrusted-host検査が成功 |
| [Incus gates](https://github.com/SLktEx/Hacocoon/actions/runs/34008410296) | standalone・owned Btrfs・authenticated private registry・Coreの全jobが成功 |
| 現在のWindows実機 | 未変更ZIPの適用と同版BAT再実行はreadiness全6項目成功後に終了0。通常入口、両clientのbuild全体一致、UUID/file/account/sudo policy保持、Btrfs状態、proxy検査が成功。distro停止確認後のdoctorは51.906秒で終了0 |

実機ZIPは `0.26.1-SNAPSHOT-c749ff9`、build日時 `2026-09-06T03:12:38Z`、SHA-256 `f638379fb293cf249f32ef46b5576b95906ff775bc2f00f96ae3ed602724d3f9`。fresh Windowsはrunnerのcurrent WSL基盤でHacocoon distributionがない状態を意味し、Windows機能無効状態やWindows OS再起動の受入ではない。実機の保持証拠はtrusted-host sentinelと基準値であり、Workspaceの未commit・未追跡・未push作業保持の証明ではない。

**未解決の起動失敗:** `42e2fb3` の通常入口で11:33:30 JSTにIncus本体PID 282がSIGKILLを受け、標準600秒start-post待機とcontroller依存が残った。signal送信元は未確定で、得られたkernel記録はOOMを示していない。手動service/mount修復なしで11:43:16にIncus標準の自動再起動が始まり、後の入口/保持検査は成功した。guest DNS/DHCP起動の競合は別途修正・受入済みであり、その修正や後の `c749ff9` 成功からSIGKILL送信元や以前の独立したWSL終了9の原因を確定しない。

**登録package受入 — `4df465a71aedcdc70c28b543220b79b2465808ab`:** [Windows run 34010791925](https://github.com/SLktEx/Hacocoon/actions/runs/34010791925)、job `101426135649` で正規fresh cached BAT、通常入口、停止/再開、同版再実行、データ保持、doctor 6項目、PowerShell/BAT回帰が成功した。手元のPS5.1実一覧/引数伝達、配布/provenance、`ci-local.sh docs` / `workflow-policy`、native文書検査も成功。provenanceの最初のUbuntu 22.04実行は26.04以上の条件で正しく停止し、製品条件を変えず対応基盤で成功した。実機向けZIPのSHA-256は `439dfc8a0a4dab5ef4adf05f1b1ed9b3e02883a5009b66dca7513c528d0d3105`、version `0.26.1-SNAPSHOT-4df465a`、build `2026-09-06T04:10:02Z`。build/checksum確認まで行い、手元で再installは繰り返していない。現在の実機installは受入済み `c749ff9` のままで、変更したfresh登録/再実行はCIで確認した。
**現在のM1範囲:** 最新のユーザー方針により、実Windows OS再起動の実装/受入と続行案内の追加作り込みは対象外。具体的な変更や失敗に見合う検証に絞り、追加で維持する回帰はCIへ置く。新しい根拠なしに成功済み検証を繰り返さない。必須だった既存controller/provider境界を使うinstall済みEnvironmentの許可proxy通信/直接通信拒否の受入は成功した。原因未確定の起動事象は記録に残し、後の限定signal観測でもその原因は特定できていない。診断機能の拡大、firewall起動順の網羅、CLI/SSH開発導線、Workspace保持は後続とし、追加の完了条件にしない。

**Environment接続元の修正:** `f373cfc` のWindows gateは正規BAT経路に成功したが、許可HTTPS probeがproxy 403になった。永続接続元resolverがprovider-local参照とEnvironment作成のroute付き参照を比較していたため、正規router decoderでproviderとnative参照の両方を照合するよう修正した。実際のBase routerの作成結果を使う最小回帰で失敗を再現し、別providerの同一native参照は拒否する。

**M1受入 — `81c0d160722b96864daa8d6f5f3b9ea86423ff48`:** [Windows run 34013409969](https://github.com/SLktEx/Hacocoon/actions/runs/34013409969)、job `101432997324` でfresh cached BAT install、通常入口、停止/再開、同版再実行、trusted-hostデータ保持、doctor 6項目が成功した。install済みcontrollerのEnvironment検証でも、証明書を検証する許可HTTPS、未承認hostnameの403、直接TCP拒否、管理socket非公開、controller cleanupが成功した。CIの対象はPR merge commit `9049df39f8000e32103b6a2f3939ea3d14fc5ffe` で、candidate `81c0d16` と全treeが一致することを確認した。route付き参照の回帰は修正前に失敗し、修正後の手元egress・Environment router・composition・Standard proxy testはすべて成功した。文書整合性検査も成功。

新しい手元ZIPは `0.26.1-SNAPSHOT-81c0d16`、build日時 `2026-09-06T05:12:08Z`、SHA-256 `4938622b994a66b71d5647086819db63e7ee7a7a8ea1189e3b2ad964ccb69c6b`。GoReleaser配布物作成と全checksum検証が成功した。このZIPは現在のWindows実機に再installしておらず、実機のinstall版は `c749ff9` のまま。上記candidateのWindows受入はCIでの結果である。実Windows OS再起動は対象外のままとする。

**次の具体的な一件:** M2として、既存controller-backed adapter経由のEnvironment作成を新 `haco` から利用できるようにする。

以下の表は元のcheckpoint時点の履歴文脈を保持する。

[English](IMPLEMENTATION_STATUS.md) | **日本語**

> 現在の `main` の code reality を示す companion です。番号の正本は [`status/versioning-and-release-status.ja.md`](status/versioning-and-release-status.ja.md) です。

Hacocoon は pre-1.0 です。現在のmilestone位置は **v0.57** です。milestoneは軽量なdevelopment checkpointとして扱い、v0.17のacceptance残件のようなpartial状態があっても、後続の実装済みcheckpointへ進めます。repository実装は、明示的に名前を付けたacceptance checkを除き、すべてのreal-host supportを意味しません。

| 領域 | 現在の状態 | Milestone |
|---|---|---:|
| Runtime / Workspace | Incus Environment lifecycle、Workspace identity、RO/RW lease | v0.1-v0.2 |
| Client access | status、loopback forwarding、SSH prepare/revoke | v0.3 |
| Policy / Capability | fail-closed policy、approval、audit | v0.4 |
| Git push | trusted Host がbrokerし、reusable Host credentialをEnvironmentへ渡さない | v0.5 |
| Agent integration | `haco run`、machine output、events。orchestrationはCore外 | v0.6 |
| Client-neutral interaction events | public `pkg/interaction` がcapability auditを最小化済みeventへprojectionし、stable ID、resume cursor、bounded batch、recovery/attention flag、public corruption errorを提供。観測はcapabilityを承認・実行しない | v0.6 / cross-cutting |
| Environment routing | provider-neutral seamは維持。**具体的なcloud implementationは現在deferred**で、EC2/AWS/EBS実装はactive treeにない | v0.7 |
| Reusable client adapter contract | public `pkg/clientadapter` がexact Environment ensure/reuse、status、loopback SSH/TCP、revoke/delete、`/workspace` discovery、`pkg/interaction` batchをpackage-owned DTOで公開。通常の `haco ssh` がnon-VS-Code proof path | v0.8 / cross-cutting |
| VS Code / Agent Host | `haco-vscode`、per-agent binding、`haco-agent-host` | v0.8-v0.10 |
| Base | `haco base list` / `inspect`、immutable Base revision | v0.11 |
| Resource budget | CPU / memory / PID / root storage | v0.12 |
| Managed Sandbox Network | `haco-sandbox0`、proxy-only ACL transport guard、`haco-sandbox` profile。DHCPを残してbridge DNSを停止し、driftはfail closed | v0.13 / cross-cutting |
| Git Fetch Plugin | `haco plugin git fetch`、Host `gh auth git-credential` | v0.14 |
| OCI Seed Recommendation | `haco plugin oci seed sample` / `recommend`、top 10%を `auto_promote=true` | v0.15 implemented |
| OCI Image Deletion | `haco plugin oci image delete`、deletion tombstone、exact immutable identityの明示reenable | v0.16 implemented |
| OCI Seed Builder / Btrfs COW | `seed build/current`、Base単位pin、保守的GC/recover、trusted Host acquisition、managed Environmentからのcredential-free exact-image harvest、offline no-NIC build、immutable publish/current pointer、exact-parent resolutionを実装。real-host/authenticated-registry/COW acceptanceはpending | v0.17 partial |
| Docker Compatibility | `haco plugin oci docker status/prepare`。Base提供profileとpinned systemd unitを検証し、active vendor daemonを勝手に停止せずEnvironment-local socket activationだけを有効化 | v0.18 implemented |
| Domain-aware egress authorization | Core `network.egress/connect`、Standard HTTP/HTTPS proxy、Host DNS pinning、private-address reject、CONNECT/SNI検証、trusted Incus source-IP mapping、`haco egress serve` を実装 | v0.19 implemented |
| Managed Btrfs rootfs storage | local compositionが `haco-local-default` Incus-owned loop-backed Btrfs poolをlazyにensureし、Hacocoon所有のBase/Tooling/Seed/Environment/trusted-host rootfsをそのpoolへ配置 | v0.20 implemented |
| Managed Btrfs transparent compression | default Incus pool作成時に `compress=zstd:3` を要求する。`compress-force` と `autodefrag` はdesired defaultにせず、mount lifecycleはIncusが所有 | v0.21 implemented |
| Interaction notification clients | `haco-notify` がloopback interaction deliveryをbrowser/native OS向けに提供し、optional VS Code notification extensionも同じinteraction streamを利用。replay/dedup behaviorをtest済み | v0.22 implemented |
| Real Incus E2E acceptance | GitHub-hosted Ubuntu 26.04でstandalone real Incus system-containerを先に検証し、その後fresh runnerでHacocoon Core lifecycle E2Eを実行。systemd/exec、network、hotplug、storage/snapshot、diagnostics、guarded cleanupをphased gateで検証 | v0.23 implemented |
| Structured logging | shared `log/slog` foundation、INFO-default text/JSON output、Environment lifecycle operation field、sanitize済みDEBUG Host-command trace、egress authorization trace、secret redactionをmaintained executableへ実装 | v0.24 implemented |
| Incus-owned Btrfs storage acceptance | actual ordinary-user `haco` をreal Incusへ接続し、lazy pool creation、Incus-owned sparse backing image、loop attach、Btrfs mount、zstd policy、writable Workspace、pool reuse、guarded cleanupまで自動検証 | v0.25 implemented |
| Trusted `haco-host` / default WSL entry | local Incus runtimeがpersistent trusted logical `haco-host` をensure/shellでき、exact ownership markerとreserved-name collision拒否で境界を守る。managed storageを使い、WSL interactive entryはdefaultで`haco-host`へ入り、Physical Host rootは明示recovery pathとして残す。raw Incus controlは`haco-host`へ公開しない | v0.26 implemented |
| OCI plugin boundary | `HACO_PLUGIN_OCI=nerdctl|docker` の明示opt-in。未設定でもCoreは動作する | cross-cutting |
| Optional Local OCI Registry | optional。通常pullやSeed constructionの必須経路ではない | unversioned optional / deferred |

## Domain-aware egress境界

ordinary HTTP/HTTPS egressはDNS-to-IP ACL近似ではなくStandard proxyでenforceします。Incus NICはdefault denyを維持し、managed bridge gatewayのStandard proxy portへのTCPだけをallowします。bridgeはDHCPを残しつつ `raw.dnsmasq=port=0` でDNS listenerを停止し、unmanaged DNS/ACL configはfail closedです。

managed profileがEnvironmentへHTTP(S) proxy discoveryを提供します。proxyはtrusted Incus source-IP stateからEnvironment identityを導出し、hostname / port / protocolごとに既存Policy / Approval / Capability / audit経路を通し、authorization後だけHost DNSを解決してpublic answer setをconnection単位でpinします。HTTPS CONNECTはTLS bytesをupstreamへ流す前にClientHello SNIとauthorized hostnameの一致を検証します。`haco egress serve` はtrusted Host foregroundの起動経路です。詳細は [`EGRESS_AUTHORIZATION.ja.md`](EGRESS_AUTHORIZATION.ja.md) を参照してください。

## Notification clients

v0.22ではclient-neutral interaction streamをuser-visible notification adapterへ接続しますが、approval authorityはclientへ移しません。`haco-notify` がbrowser/native向けloopback bridgeを提供し、`clients/vscode-notify` がoptional VS Code consumerを提供します。cursor persistence、replay、dedup、corruption handling、browser behavior、VS Code behaviorをrepository testで検証します。notificationの表示は観測だけであり、Capabilityを承認・実行しません。詳細は [`INTERACTION_EVENTS.ja.md`](INTERACTION_EVENTS.ja.md) を参照してください。

## Real Incus E2E acceptance

v0.23は新しいCore APIではなくsupport-confidence checkpointです。GitHub ActionsのUbuntu 26.04上でまずIncus substrateだけを独立に検証し、その後Hacocoon Core lifecycleをfresh runner上のreal Incusで検証します。standalone stageはreal system container、systemd/exec、network、device hotplug、storage/snapshot、diagnostics、exact cleanupを確認し、Core stageはその成立済みsubstrateに対してHacocoon lifecycleを確認します。これによりIncus側のfailureとHacocoon regressionを切り分けやすくし、fake-only E2Eを十分なacceptanceとは扱いません。

## Structured logging

v0.24ではstructured loggingを独立したmilestoneとして扱います。maintained executableは `HACO_LOG_LEVEL` / `HACO_LOG_FORMAT` から1つのshared `log/slog` rootをconfigureします。defaultはINFO/textで、JSONへ切り替えてもstdoutのcommand resultは変えません。Environment create/exec/shell/deleteは `operation`、`environment_id`、duration、result/error fieldをcontext経由で持ち回ります。trusted Host runnerはsanitize済みcommand metadataをDEBUGで追加し、Incus/network/storage/Git/OCIをcomponent分類しますが、subprocess stdout/stderrを自動logしません。

shared handlerはpassword/token/API key、authorization/cookie、credential-bearing URL、secret assignmentのknown patternをDEBUGを含めてdefense-in-depthでredactします。ただしcall site側でもarbitrary header、environment、config object、private key、request body、untrusted outputを渡してはいけません。詳細は [`reference/logging.ja.md`](reference/logging.ja.md) を参照してください。

## Trusted `haco-host` / WSL entry

v0.26ではlocal Incus pathにpersistent trusted logical Hostを導入します。`haco host ensure` が `haco-host` を作成・reconcileし、`haco host shell` がensure後に入ります。Hacocoonはexact ownershipをmarkし、非owned instanceとのname collisionを拒否し、managed storageへ配置し、raw Incus control socketを `haco-host` の外に残します。WSL login shimにより通常のinteractive distro entryは `haco-host` を開き、明示的なPhysical Host root entryはrecovery escape hatchとして残ります。

real Incus acceptanceはtrusted-host creation、ownership、idempotent ensure、stopped-state recovery、managed-storage behavior、control-socket non-exposureをcoverします。real Windows/WSL interactive-login acceptanceはhost-dependentです。現在のsliceはlifecycle/default-entryまでで、Git/OCI/credential/control-channelの全面移行はfollow-upです。詳細は [`design/trusted-host.ja.md`](design/trusted-host.ja.md) と [`WINDOWS_WSL_BOOTSTRAP.ja.md`](WINDOWS_WSL_BOOTSTRAP.ja.md) を参照してください。

## Client adapter境界

`pkg/clientadapter` がVS Codeに依存しないreusable adapter-facing contractです。canonical Host Workspaceとrequested access modeが完全一致する場合だけEnvironmentをensure/reuseし、guest内Workspaceは `/workspace` として公開します。connection metadataのreconcileとpublic `pkg/interaction` event contractも同じ境界から利用できます。

SSH prepareが受け取るのはpublic-key materialだけで、private keyとIDE configはclientが保持します。返却/reconcileされたSSH/TCP connectionはloopback-onlyか再検証し、provider outputがcontract違反ならrejectします。既存の `haco create` + `haco ssh` + 通常の `ssh` がnon-VS-Code proofです。詳細は [`CLIENT_ADAPTER_CONTRACT.ja.md`](CLIENT_ADAPTER_CONTRACT.ja.md) を参照してください。

## Core と OCI plugin

containerd / nerdctl / Docker は Hacocoon Core の必須要件ではありません。project-maintained OCI plugin profile が必要に応じて containerd + nerdctl や Docker compatibility を提供します。Base lifecycle は `haco base ...`、OCI workload tooling は `haco plugin oci ...` に分離します。

## OCI Seed / storage

v0.17はbuild/publish、operations-hardening、credential-free managed-Environment harvestのrepository sliceを実装済みです。trusted Host acquisition/cache → offline no-NIC Seed Builder → immutable Seed revision/current pointer → exact-parent resolution → normal Incus/storage-driver clone の経路を維持し、複数Environmentで一つのwritable `/var/lib/containerd` を共有しません。

v0.20ではlocal rootfs storageをIncus-owned Btrfsへ統一します。Environment、Tooling Base builder、Seed builder、trusted hostがroot storageを必要とした時点でlocal compositionが `haco-local-default` をlazyにensureします。sparse backing file、loop device、Btrfs filesystem、mount lifecycleはIncusが所有し、Host Workspaceはpool外からbind mountします。

v0.21ではtransparent compressionのdefault policyとしてIncusへ `btrfs.mount_options=compress=zstd:3` を渡します。`compress-force` と `autodefrag` はdesired defaultにせず、既存extentを自動rewriteしてrecompressしません。

v0.25はこのstorage pathのreal ordinary-user acceptance checkpointです。GitHub-hosted Ubuntu 26.04でactual `haco` をreal Incusへ接続し、lazy `haco-local-default` creation、Incus-owned sparse backing image、loop attachment、live Btrfs mount、zstd policy、writable `haco create` / `exec` / `delete` / `run`、pool reuse、guarded cleanupまで確認します。詳細は [`design/btrfs-storage-layout.ja.md`](design/btrfs-storage-layout.ja.md) を参照してください。

Local Registryはprerequisiteではなくroadmap versionも予約しません。残件はauthenticated/private-registry combination、physical Btrfs compression ratio / CPU cost / COW measurement、compaction behavior、broader real-host failure injection、Windows/WSL behaviorなどです。

## Docker compatibility

v0.18のrepository gateは実装済みです。`HACO_PLUGIN_OCI=docker` で `haco plugin oci docker status <environment>` / `prepare <environment>` を使えます。`prepare` はpackage installやHost socket mountをせず、Base/Seed側にDocker CLI、dockerd、containerd、systemd、docker group、Hacocoon-pinned socket/service unitがあることを要求し、unit driftやactive vendor Docker daemonではfail closedします。

## Cloud status

v0.7のprovider-neutral Environment routing seamは維持します。以前のconcrete EC2/AWS/EBS implementationはactive treeから削除済みで、**cloud implementationは現在deferred**です。

## Acceptance gaps

v0.23でGitHub-hosted Ubuntu 26.04上のphased real-Incus substrate + Core lifecycleを、v0.25でordinary-user Incus-owned Btrfs CLI behaviorを、v0.26でtrusted-host lifecycle/control-socket isolationをreal Incusで自動証明するようになりました。ただしproxy-only bridge ACL/dnsmasqを含む全network/resource behavior、Windows/WSL + VS Codeとinteractive `haco-host` entry、private-registry credential、Docker compatibility、physical Btrfs compression/COW/compaction、broader storage failure injection、desktop notification delivery、future cloud adapterなどは引き続きenvironment-dependentです。前のmilestoneにacceptance残件があっても、後続minor checkpointへ進むことは妨げません。

## 保持したEnvironmentの再開

Status: **コマンドとcomponentの範囲はimplemented、ロードマップC/Eはpartial**。
`haco env start <name>` は必須flagを増やさず既存runtimeを再開します。
起動前にactive leaseの同一性とIncusのネットワーク隔離を検証し、Linux/WSLでは
create/start/stop/deleteをcontrollerプロセス間で直列化します。
[ADR 0016](adr/0016-resume-owned-environments.md)を参照してください。
ローカルCIのtest・race段階は全体で成功しました。独立した実Incus 6.0.5-8 / WSLで
停止・再開・再度start・root filesystemとWorkspace内容・保存済みleaseの保持・正規削除が成功しました。
初回fixtureはJSON保存でGoの単調時計情報が失われるためtimestamp比較だけ失敗し、
保存済みlease同士の比較へ修正後の再実行は成功しました。両試験runtimeは除去され、
既存のユーザーEnvironmentは停止状態を維持しています。既存Incus E2Eにもtrusted Hostからの
製品stop/startとWorkspace保持の検証を追加し、`f8517ba`のGHAで成功しました。
SSH setup自動化とHost再起動後に欠けたguardを復元する処理は未実装です。

## 通常APIからのSSH公開ホスト鍵取得

Status: **protocolの範囲はimplemented、SSH setup自動化はplanned**。
IncusのSSH準備は構造検証済みEd25519 `host_public_key` を通常のcontroller応答で返します。
native clientは別途管理者としてIncusを呼ばず鍵を固定できます。不正な鍵では管理対象の鍵と
proxyを撤回し、後始末の失敗はrecovery-requiredとします。公開adapterも任意fieldの鍵を再検証します。
関連race testは成功しました。Windows native受け入れscriptもこのfieldを使うよう更新しましたが、
インストール済みWindows経路も`f8517ba`のGHAで成功しました。

利用者の補足: 開発sourceはHacocoonの作業branchへpushしてPRを出し、Git push機能の
検証先は引き続きHacocoon-testに限定します。PR #482でv0.30/v0.31とSSH公開鍵の範囲を
`f8517ba`として公開しました。以前の公開拒否は本体pushの明示許可により解消しました。
B4の必須defaultを明確化しました。Environment作成時にOCIイメージの公開・COWコピーを
自動実行し、任意のOFF指定だけを設けます。公開済みsourceのコピーは実装済みですが、Hostイメージ公開は
完了していません。詳細はStoreの所有文書に記録しています。

## Workspace Storeの自動初期化

Status: **公開済みsourceのコピー・再利用・OFF指定はimplemented、B4全体はpartial**。
通常のEnvironment作成で任意OCI連携の既定resolverを呼びます。readyかつsource-onlyの
`oci-source:host`をWorkspaceに永続的に対応付けたStoreへコピーし、再作成では再利用します。
`--no-oci`で省略できます。公開元がない場合も非OCI作成は利用可能です。公開元の直接接続、
別Workspaceへの流用、不完全な公開/コピー、失敗を空データで成功扱いする経路は拒否します。
HostのDocker/nerdctlイメージproducerは未実装で、自動イメージ配布全体の完了ではありません。
Docker/runtime互換性も未検証です。関連回帰/race testが成功しました。実Incus/Btrfsの合成データ
fixtureも既定resolverを通し、COW親子関係・独立書込み・source削除・後始末を確認しました。
イメージ取得やruntime利用を証明する試験ではありません。PR #482の`f8517ba`ではWindows
native SSHを含む4つのGHA workflowが成功しました。以降の変更には別のCI結果が必要です。
private-registry E2Eはworkflow-dispatch限定のためSKIPです。
追加依頼の `docker run --rm` 相当は VS Code 接続確認後に製品 CLI へ接続しました。
実 Incus の受入結果は下の一時実行節で区別します。

## runtime側でのSSH自動ポート選択

Status: **implemented、Windows GHA bcc1baf の受入は成功**。
`haco env ssh --key <public-key-file> <name>`はポート引数が不要になりました。
SSHポート0をIncus runtimeへ渡し、Physical Hostで選択してからguestの鍵変更前に
proxyを確保します。Windows native E2Eもこの通常defaultを使い、実際のproxyを
確認し、bcc1baf で成功しました。鍵・config 自動設定と実際の VS Code 接続も確認済みです。

## Desktop SSH setupとVS Code起動

状態: **command は implemented、Windows GHA の editor 接続は成功**。
`haco ssh setup [name]` は desktop 所有の鍵と厳密な host-key pin を準備します。
`haco open [--client vscode|ssh] [name]` で client を選択し、Environment が1つなら名前を省略できます。
インストール済み GHA は native SSH、停止からの再開、接続の再利用に加え、`4f1f512` で実際の editor と terminal 接続を確認しました。
[client の契約](design/client-adapters-and-vscode-integration.md#desktop-ssh-setup-and-vs-code-opening) を参照してください。

native Windows fixture では鍵・config 作成と、インストール済み trusted Host 経由での editor 探索も確認しました。
別の開発用 Ubuntu からの試行は PowerShell の `exec format error` で失敗しました。
cold entry 後の raw Incus fixture は Host 停止中で失敗し、通常の対話 entry 後に成功しました。
これらの準備 fixture だけでは接続成功を証明しません。ローカルの version/許可の残件と GHA の正確な範囲は文書冒頭に記録しています。

日常 CLI の確認は **C6 の一部** です。`haco env list` は登録された名前・Workspace・Base を表示し、
スクリプトでは `--json` を使えます。list/status は端末制御文字を escape します。広い DNS・接続診断は未完了です。

## 保存した Host カスタマイズ

状態: **明示 setup/replay は implemented、Windows GHA は bcc1baf で成功**。
利用者が選んだ UTF-8 Bash 手順を controller が private に保存し、所有権を確認した trusted Host だけで実行します。
通常の setup で保存内容を再実行し、明示した script 更新で置き換え、clear で実行せず解除します。
Environment へ渡しません。回帰テストは file/link 保護、直列化、script 失敗前の保存、service 再作成後の replay、
対象の所有権、標準入力での受渡し、秘密を含まない失敗通知を確認します。
Windows GHA bcc1baf（run 34103036390、job 101681633357）で通常の保存・再実行・更新・解除が成功しました。
test・Ubuntu・Incus workflow も成功しました。
[Host カスタマイズ](design/trusted-host.ja.md#保存したカスタマイズ手順) を参照してください。

controller setup 外の暗黙の Host 再作成は未検証です。ユーザーの installation では、このカスタマイズや任意の package/dotfile 手順を実行していません。
当初のソース編集の自動レビュー拒否は、ロードマップ C2 の明示要件を確認し、同じソース編集をその根拠で再審査して解消しました。
保留中のローカル package policy の許可とは別の事項です。

## 一時実行

状態: **product CLI は implemented、実 Incus の検証は 4adfe19 で成功**。
`haco run [--rm] -- <command>` は既定で所有権付きの一時 Workspace を作り、
/workspace から実行し、runtime と自動 OCI copy を削除します。
`--workspace` は既存 Workspace と Store を保持し、`--no-oci` は自動コピーを無効化します。
一時 identity を作成前に記録し、canonical deletion が lifecycle lock 内で照合します。
resource cleanup も Workspace binding を原子的に確認します。失敗時は回復証拠を残します。
非ゼロ終了、片付け失敗、中断を区別し、stdin/TTY は未実装です。

race 回帰は既存作業の保護、片付け失敗と回復、既定の一時 source、OCI 公開元の保持、
provider の明示対応と argv の保持を検証します。実 Incus GHA の 4adfe19（run 34115004878、job 101719650209）で通常 CLI の成功、exit 17、
既存ファイルへの書き込み、中断後の実体不在を確認しました。
内容入り OCI image 実行とローカル installed acceptance は未検証です。
[一時実行](design/temporary-execution.ja.md) と
[ADR 0020](adr/0020-runtime-owned-temporary-workspaces.md) を参照してください。

`5f824b4` は Ubuntu・Incus が成功しました。Go 1.26/1.27 test/vet・race・docs も成功しましたが、
test workflow は失敗しました。Capability E2E が古い承認表示を期待し、別ジョブでは
GoReleaser 導入が HTTP 504 でした。表示期待値を更新し、ローカルの保存・再利用・
Environment 範囲の E2E と製品 CLI E2E は成功しました。
Windows run 34133686648 は DNS・通常 SSH の再利用/再開・VS Code が成功し、
project setup で失敗しました。実運用 runner decorator が Incus exec の optional な
stdin 契約を引き継ぐ修正を加えています。修正後の installed 検証は再実行待ちです。

保存 Policy に Environment 単位・全 Environment の毎回承認を追加しました。terminal で今回の許可・拒否を別に確認し、ask の保存から allow を作りません。通常の Git/通知への統合と、それらを不変の Environment identity に結び付ける作業は未完了です。

`347ca50` は test・Ubuntu・Incus workflow が成功しました。Windows run 34135390824 では DNS・VS Code と project setup の保存・再実行・非ゼロ終了・更新・削除が成功し、preview server recipe で失敗しました。fixture は Python がなければ準備し、loopback listener の起動を待つようにしました。前回の失敗原因はまだ確定していません。preview・Edge・Environment doctor の実機検証は未完了です。単発承認・最初から許可された要求も実行直前に Policy を再評価する修正は、関連 race test が成功しました。

Environment 作成時に canonical lease へランダムな instance ID を予約します。既存の整合した ready 状態には catalog lock 内で一度だけ付与します。保存 Policy・承認表示・監査で ID を扱い、実運用 Git は取得後、実行直前にも再確認します。同名再作成に識別済み保存方針を引き継ぎません。State/Workspace/Core と Capability/controller/Git の race test は成功しました。通常の Git 保存範囲/UI と network identity 統合は partial です。[ADR 0025](adr/0025-environment-approval-identity.ja.md)を参照してください。

`bffc3fd` は test・Ubuntu・Incus が成功しました。Windows run 34136858725 は VS Code と project setup が再度成功し、拡張子のない preview marker が PowerShell に byte 列で返ったため、内容確認で失敗しました。text/plain fixture への修正は installed 検証待ちです。Git pending には追加引数なしで trusted な作成識別子を表示します。

d4aef8d では 4 workflow が成功しました。Windows run [34139245378](https://github.com/SLktEx/Hacocoon/actions/runs/34139245378) で VS Code の実接続、project setup の保存・再実行・非ゼロ終了・更新・削除、Edge headless の preview 描画、HTTP preview の再利用・終了・接続拒否、Environment doctor の前提確認が PASS です。上記の preview 受け入れ待ちは解消しました。既定ブラウザの起動や物理端末の受け入れを証明するものではありません。VPN／NRPT は VPN と private name の fixture がないため SKIP です。

実運用の Capability service は全ての名前付き要求を trusted catalog の作成 ID に結び付け、実行直前にも照合します。env 限定の保存には ID が必須ですが、利用者の引数は増えません。通常の Git 保存範囲・UI と実 network/provider の受け入れ確認は partial です。

5272434 の GHA では Go 1.26／1.27 の tests・vet、race、release-config、docs、Ubuntu、Incus が PASS です。test workflow は orchestrator E2E で未作成の名前を承認元に使っていたため失敗しました。fixture を通常の create／delete に直し、ローカル E2E は PASS しました。Capability の保存範囲・再作成と Git transport 拒否の E2E も PASS です。

local CI 全体は docs／workflow 検査後、WSL の pwsh 不在で失敗し、それ以降の工程はその呼び出しでは未実行です。Go 工程の個別実行では、空の select が SIGKILL 用 helper を deadlock 終了させ、親が生存中の lock を確認する前に解放するテスト不具合が見つかりました。制限時間付き timer で親からの kill まで生存させ、実 subprocess／SIGKILL の回帰 20 回と run package の race 検証が PASS です。cleanup の権限を変える修正ではありません。


実 Git push CI は trusted main での手動実行と固定の SLktEx/Hacocoon-test 送信先に限定します。専用 credential がない場合は SKIP で、push 受入成功ではありません。旧 fixture は製品インストール後の import や対話承認を検証しません。[ADR 0059](adr/0059-dedicated-git-push-test-target.md)を参照してください。

Windows 39b5ce4 で承認 setup の失敗が再現し、DNS start-limit-hit を確認しました。同じ DNS 構成では冪等な systemd start を使い、companion／unit 変更時は restart します。修正後の installed Windows 検証は 226991b（run 34479510230）で成功しました。[service 起動](design/name-resolution.ja.md#繰り返す-setup-と-service-起動)を参照してください。

G1 の Windows ファイル受入は c4449e1 の [Windows run 34482712957](https://github.com/SLktEx/Hacocoon/actions/runs/34482712957) で成功しました。完成した Linux bundle の排他的コピー、Windows 側の長さ／hash 照合、既存共有 drive から Linux client での import を確認します。Windows native CLI／直接 export 公開は未実装です。[経路](design/environment-transfer.ja.md#既存ドライブ共有を使う-windows-bundle-ファイル)を参照してください。

E5 OCI image list/delete は、コンテナの参照がないタグ付き画像も含め、--unused で候補を確認・選択できます。画像ごとの所有・参照・不在確認を維持します。実 controller/CLI の一括検証は 9484d06（run 34493016558）で成功し、cache・他資源の GC は planned です。[候補選択](design/oci-image-deletion.ja.md#未使用画像候補の確認)を参照してください。

G1 の実 containerd 転送 fixture は 6974272（run 34501951826）で native 受入に成功し、Windows を含む対象 CI も成功しました。source 削除後に両 import 構成で offline image と停止コンテナの書込データを確認します。Docker／cache／アプリ整合性は未検証です。[対象範囲](design/environment-transfer.ja.md#実-oci-データ転送の受入)を参照してください。

G2 に読み取り専用の native 退避一覧補助を追加し、専用 WSL の実 Incus で確認しました。全量のデータ列挙・外部 backup・復元後照合は未実装です。[一覧の範囲](design/environment-transfer.ja.md#退避対象の-native-一覧)を参照してください。

G2 の native 一覧は pool の保存元参照と volume の内容種別も示し、参照先を開かず URI の認証情報を出力しません。対象テスト 11 件は成功しました。これらの参照情報だけで全量を把握したことや、削除権限があることにはなりません。

G2 に任意の Linux 読み取り専用 schema-13 catalog 参照抽出を追加しました。catalog 移行・認証情報の出力・所有権の付与は行いません。全対応関係の確認と全量退避は引き続き partial です。

G2 一覧は任意の repository 個別記録・collection member 参照も扱います。remote URL は出力せず、不完全／変動中の directory を完全な backup と扱いません。

## Windows SSH タイムアウトの診断

b8ef557 のインストール済み Windows 検証は、SSH 準備と DNS 確認の成功後に
ssh.exe が5分を超えたため失敗しました。ラッパーは成功マーカーの確認より先に
子プロセスの非ゼロ終了を報告します。最初の SSH 検証はタイムアウト時に許可リスト内の
進捗だけを残します。期限・ホスト鍵固定・隔離は変更していません。
これは診断の改善であり、SSH の失敗が修正済みであることを示しません。

## 読み出せるデータの退避

G2 の Btrfs 直接ファイル保存は専用 WSL で 20.59 秒で成功し、Git 状態・リンク・数値 owner・user xattr を確認しました。Incus の非 optimized Btrfs backup も内部 snapshot を使います。全量退避は未完了です。[読み出せるファイルの範囲](design/environment-transfer.ja.md#snapshot-操作が使えない場合の読み出せるファイル)を参照してください。

G2 の snapshot にしか残らないファイルの保存は、専用 Incus/Btrfs で 24.52 秒で成功しました。準備済みの合成 snapshot データと独立した復元 volume を使っています。全保存データの把握と snapshot 削除失敗時の全量退避は未完了です。

G2 の暗号化ファイル転送は標準 tar／age コマンドと opt-in の合成データ受入で扱います。実 Host 認証情報と全量復元は未検証で、製品の暗号 backend や日常 CLI は追加しません。

G3 の部分受入として Windows 上の合成 Workspace／OCI archive を新 WSL・Btrfs pool に復元し、内容・属性の照合と Git 作業再開に 8.04 秒で成功しました。native snapshot の作成・削除も成功しました。暗号 identity の回復、インストール済み Hacocoon の再構成、全量入替は未検証です。[範囲](design/environment-transfer.ja.md#新-wsl-へのデータ復元の受入)を参照してください。
