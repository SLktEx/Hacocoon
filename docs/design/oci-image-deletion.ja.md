# OCI image の一覧と削除

日本語 | [English](oci-image-deletion.md)

Status: partial。接続済み Store の実装と専用環境での実 runtime 検証は完了し、インストール済み controller 経由の受け入れ検証は未完了です。

## 現在のコマンド

```bash
haco plugin oci image list dev
haco plugin oci image list --runtime docker --json dev
haco plugin oci image delete dev example.local/app:dev
```

既定は nerdctl で、Docker は runtime を明示します。一覧には Env 世代、Store の
所有者、image ID・tag・digest、container の参照、独立 snapshot の由来を表示します。
削除は表示された正確な ID または tag を受け付け、runtime の不変 ID に解決して
対象を表示し、確認を求めます。`--yes` は確認入力だけを省略します。tag だけの
解除ではなく、選択 image 全体の削除です。複数 tag や container 等による runtime
の拒否はエラーとして返し、force で回避しません。

## 責務と保護

OCI plugin は runtime の一覧・削除を使い、別の image catalog を持ちません。
Docker の config ID と nerdctl の manifest/index digest は区別します。
nerdctl の Docker 互換 inspect は config ID と container の image 名を返すため、
削除対象の native ID へ明示的に対応付けます。CLI の環境をクリアし、固定の
ローカル socket・containerd namespace・snapshotter を指定します。呼び出し元は
socket や実行ファイルを選べず、layer file を直接削除しません。不正・不完全な
出力や失敗を成功扱いせず、削除後の一覧で不在を確認できなければ失敗を返します。

`internal/workspace.ExecForResource` は通常の Env lifecycle lock 内で、確認済み
世代と接続 Store の所有 ID を比較して通常の実行経路を呼びます。各 runtime 呼び出しで
再確認し、plugin も現在の ready Store の所有者を照合します。同名 Env の再作成で
待機中の削除が新 Env へ向かうことを防ぎます。guest の通常操作で一覧は変わり得るため、
最後の参照確認は runtime の非 force 削除が担当します。

## 範囲と検証

接続済み Store と Host source に加え、未接続 Store の経路は後述の partial 実装です。候補選択は後述の implemented 機能で、実 runtime の一括検証は未完了です。保存 snapshot は独立コピーであり、画像削除の影響を受けません。schema 移行や自動 backup は追加しません。

## 管理対象 Host source

partial の実装です。専用環境の実 runtime 検証は成功し、インストール済み controller 経由の検証は未完了です。

```bash
haco plugin oci image list --host
haco plugin oci image delete --host --runtime docker example.local/app:dev
```

`--host` と Env 名は同時指定できません。確認画面は正確な source owner と、今後の
Store コピーの元を変更することを表示します。既存の独立 Store／snapshot コピーには
影響しません。旧 Seed namespace・tombstone・全 Env 削除は復活させません。
対象は現在の管理 source `oci-source:host` であり、guest Store を Host に接続するための
指定には使えません。自動 setup・移行・復旧も実行しません。

plugin は現在の ready source owner を照合します。Incus adapter も独立して、固定の
image／container 一覧、限定した inspect template、不変 ID の非 force 削除だけを許可します。
実行ファイルの検索先・daemon socket は固定し、CLI の環境をクリアします。任意 shell・
program・daemon option・inspect template は Host 境界へ渡せません。

各命令は既存 Host-operation lock を保持し、未完了 copy journal を拒否します。
native volume owner、単独の接続先、正確な mount、local Host/source marker、非特権の
container 型、空の profiles、running 状態、管理対象 daemon layout を確認します。
停止中 Host を再開したり、復旧記録を消したりしません。所有・layout が不明、または
観測が失敗／切り詰められていれば拒否します。native 命令は2分、全体 request は5分で
制限します。container 参照と削除後の不在確認は接続済み Store と同じ runtime の検証を使い、
既存 copy／cleanup 状態は変更しません。

## 過去の Seed 削除

v0.16 の Host Seed cache・tombstone・全 Env 操作は隔離された旧実装のものです。
現在の製品コマンドや Store 寿命モデルではありません。過去の仕様は Git 履歴で確認でき、
今回の変更で既存記録を黙って削除しません。release 表の v0.16 リンクは過去の checkpoint
を示し、今回の partial 実装の番号ではありません。[ADR 0046](../adr/0046-reviewed-runtime-image-deletion.md)を参照してください。

## 未接続 Store の実装中の範囲

状態: **partial**。既存の image コマンドは Env 名の代わりに保持 Store ID を受け付けます。
新しいコマンドや必須 option は追加しません。

```bash
haco plugin oci image list oci:store-id
haco plugin oci image delete oci:store-id sha256:<displayed-digest>
```

確認対象は正確な Store ID・owner であり、古い一時 Env の識別情報は保持しません。
一覧・削除はそれぞれ一つの canonical maintenance run を取得し、全 runtime 呼び出しで
現在の Env 世代を確認します。run がキャンセル・Store 排他・cleanup を所有し、元の
Workspace 対応や借用 Store を変更・削除しません。cleanup が不明なら所有記録を保持します。
Host・Env・Store の混在した対象や古い確認結果は拒否します。未接続 Store の Docker は未対応です。

SandboxProvider の receipt 付き作成は、保持データなしで起動し、現在の network 保護を維持し、
通常 daemon を mask してから Store を接続し、専用の containerd 2.3.3 metadata service を起動します。
task・restart・CRI・NRI・sandbox service は無効にします。保持設定・restart label・権限は引き継ぎません。
receipt なしの作成と snapshot restore は maintenance を拒否します。
[ADR 0047](../adr/0047-detached-store-maintenance.md) を参照してください。

Linux/WSL amd64 の composition は、保持 Store 接続前に対応ツールを自動配置します。
OCI module が固定した nerdctl 2.3.5 配布物の SHA-256 を検証し、設定済み Haco root 内の
private archive cache を再利用します。一時 private directory に containerd・ctr・nerdctl
だけを準備します。Incus adapter は所有・世代・保持 mount 不在を再確認し、転送後の hash を
照合して disposable rootfs に配置します。取得したバイナリは Physical Host では実行しません。
準備失敗時は保持 Store を接続しません。利用者の準備コマンドは不要です。

cache は排他制御し、symlink・hardlink・不適切な権限・破損した内容を黙って置き換えず拒否します。
archive 内の path で Host の出力先を選びません。署名付き download URL や response body を
取得エラーに含めません。非 Linux と amd64 以外の自動配置は現在未対応です。
ツール配置の native 検証は成功し、導入済み controller 全体の受け入れは未完了です。
schema 移行・自動 backup・任意の実行ファイルや socket を選ぶ option はありません。

maintenance は確認済みの既存 Store を指定し、`SkipDefaultResource` は併用しません。明示した Store は既定 Store の自動準備を通りません。矛盾する指定を canonical lifecycle が拒否する契約を維持します。実 catalog／lifecycle の回帰テストで予約、元 Workspace 対応の保持、正常時と操作失敗時の cleanup を確認します。

## Controller／CLI の受け入れ検証

製品 controller／CLI を使う gate は、[bd1c9a5 の GHA](https://github.com/SLktEx/Hacocoon/actions/runs/34417051340/job/102684134054) の
実 Incus/Btrfs で成功しました（588.51 秒）。production composition と、
試験が新規作成した合成 Store だけを登録した非公開の実 catalog を使います。
未接続 Store の一覧、参照中画像の削除拒否、確認後の未使用 digest の削除と不在、
container metadata の保持、一時 Env／lease の正確な cleanup、試験用 Store の
明示削除を確認しました。この commit の全 4 GHA workflow が成功しています。

使い捨ての GitHub-hosted runner に限定した fixture です。root controller は
専用の 0700 TMPDIR を使い、先行する通常ユーザーの lifecycle lock を引き継ぎません。
製品の lock 所有者確認は変更していません。fixture と明示 Store 作成の修正前に
失敗した gate の結果は PR に記録しています。

確認したのは bare controller と private socket の経路です。インストール済みの
Standard egress 全体、通常ユーザー／desktop での利用、未接続 Docker、候補選択型 GC は
未検証または未実装です。接続済み Store／Host source の検証範囲を広げたとは扱いません。

## 未接続 containerd metadata service

独立した Incus 6.0.5/Btrfs primitive は179.66秒で成功し、task API 拒否、restart 設定付き
container metadata の不変性、使用中画像保持、未使用 alias 削除、mask 付き再起動、
所有対象だけの cleanup を確認しました。同じ socket 上の製品画像操作は別の受け入れ検証です。
fixture は表示タグ・不変 digest・実際の container 参照を区別します。同じタグであることは、
各 digest が参照されている証拠にはなりません。fixture の catalog・lifecycle adapter は、
controller 全体の作成とツール配置を表していません。

拡張した native fixture は224.64秒で成功しました。製品の一覧、実際の container 参照による削除拒否、
選択した未使用 digest の削除と不在、container metadata 保持、mask 付き再起動、Env 削除後の
Store 保持、所有対象だけの cleanup を確認しました。lifecycle・catalog adapter は fixture のままです。

製品の準備処理と Incus adapter による自動配置を含む専用 native fixture は237.37秒で成功しました。
後続の画像操作、metadata・Store の保護、所有対象だけの cleanup も確認しました。
空 cache からの実 HTTPS 取得・展開は別に70.71秒で成功し、取得したバイナリは Host で実行していません。
OCI・Incus・composition 全体の race suite と vet も成功しました。

## 未使用画像候補の確認

CLI の候補選択は implemented、実 runtime での一括検証は未完了です。既存の
コマンドへ任意のフラグを一つ加えます。

```bash
haco plugin oci image list --unused dev
haco plugin oci image delete --unused dev
```

保持 Store ID または `--host` も指定できます。観測した一覧で、実行中・停止中の
どちらのコンテナからも参照されていない画像を、タグ付きも含めて選びます。
dangling layer、未使用 build cache、回収可能な容量という意味ではありません。
表示した ID・タグを確認して承認します。`--yes` はその確認集合への明示的な同意です。
`--unused` と画像 ID は同時に指定できません。

選択した immutable ID ごとに既存 controller の削除契約を使い、現在の所有者・世代、
最新の参照、非 force 削除、削除後の不在を確認します。確認後に現れた画像は追加しません。
新しい参照や所有者の変更は削除を拒否できます。最初の失敗で停止し、完了数を報告して
残りを保持します。rollback・隠れた backup はありません。独立 snapshot・Store、
コンテナ、cache はこの GC の対象外です。
