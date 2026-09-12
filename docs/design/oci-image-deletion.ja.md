# OCI image の一覧と削除

日本語 | [English](oci-image-deletion.md)

Status: 部分実装。接続済み Store の実装と専用環境での実実行基盤検証は完了し、インストール済みコントローラー経由の受け入れ検証は未完了です。

## 現在のコマンド

```bash
haco plugin oci image list dev
haco plugin oci image list --runtime docker --json dev
haco plugin oci image delete dev example.local/app:dev
```

既定は nerdctl で、Docker は実行基盤を明示します。一覧には Env 世代、Store の
所有者、イメージ ID・tag・ダイジェスト、container の参照、独立スナップショットの由来を表示します。
削除は表示された正確な ID または tag を受け付け、実行基盤の不変 ID に解決して
対象を表示し、確認を求めます。`--yes` は確認入力だけを省略します。tag だけの
解除ではなく、選択イメージ全体の削除です。複数 tag や container 等による実行基盤
の拒否はエラーとして返し、force で回避しません。

## 責務と保護

OCI プラグインは実行基盤の一覧・削除を使い、別のイメージ catalog を持ちません。
Docker の設定 ID と nerdctl の manifest/index ダイジェストは区別します。
nerdctl の Docker 互換 inspect は設定 ID と container のイメージ名を返すため、
削除対象の native ID へ明示的に対応付けます。CLI の環境をクリアし、固定の
ローカルソケット・containerd 名前空間・snapshotter を指定します。呼び出し元は
ソケットや実行ファイルを選べず、layer ファイルを直接削除しません。不正・不完全な
出力や失敗を成功扱いせず、削除後の一覧で不在を確認できなければ失敗を返します。

`internal/workspace.ExecForResource` は通常の Env ライフサイクル lock 内で、確認済み
世代と接続 Store の所有 ID を比較して通常の実行経路を呼びます。各実行基盤呼び出しで
再確認し、プラグインも現在の ready Store の所有者を照合します。同名 Env の再作成で
待機中の削除が新 Env へ向かうことを防ぎます。guest の通常操作で一覧は変わり得るため、
最後の参照確認は実行基盤の非 force 削除が担当します。

## 範囲と検証

接続済み Store と Host 元データに加え、未接続 Store の経路は後述の部分実装です。候補選択と確認付き削除は実装済みで、後述の実機検証があります。保存スナップショットは独立コピーであり、イメージ削除の影響を受けません。schema 移行や自動 backup は追加しません。

## 管理対象 Host source

部分実装の実装です。専用環境の実実行基盤検証は成功し、インストール済みコントローラー経由の検証は未完了です。

```bash
haco plugin oci image list --host
haco plugin oci image delete --host --runtime docker example.local/app:dev
```

`--host` と Env 名は同時指定できません。確認画面は正確な元データ所有者と、今後の
Store コピーの元を変更することを表示します。既存の独立 Store／スナップショットコピーには
影響しません。旧 Seed 名前空間・削除指定・全 Env 削除は復活させません。
対象は現在の管理元データ `oci-source:host` であり、guest Store を Host に接続するための
指定には使えません。自動 setup・移行・復旧も実行しません。

プラグインは現在の ready 元データ所有者を照合します。Incus アダプターも独立して、固定の
イメージ／container 一覧、限定した inspect template、不変 ID の非 force 削除だけを許可します。
実行ファイルの検索先・daemon ソケットは固定し、CLI の環境をクリアします。任意シェル・
program・daemon オプション・inspect template は Host 境界へ渡せません。

各命令は既存 Host-operation lock を保持し、未完了コピー journal を拒否します。
native ボリューム所有者、単独の接続先、正確なマウント、local Host/source 識別情報、非特権の
container 型、空のプロファイル、running 状態、管理対象 daemon layout を確認します。
停止中 Host を再開したり、復旧記録を消したりしません。所有・layout が不明、または
観測が失敗／切り詰められていれば拒否します。native 命令は2分、全体要求は5分で
制限します。container 参照と削除後の不在確認は接続済み Store と同じ実行基盤の検証を使い、
既存コピー／後始末状態は変更しません。

## 過去の Seed 削除

v0.16 の Host Seed キャッシュ・削除指定・全 Env 操作は隔離された旧実装のものです。
現在の製品コマンドや Store 寿命モデルではありません。過去の仕様は Git 履歴で確認でき、
今回の変更で既存記録を黙って削除しません。release 表の v0.16 リンクは過去のチェックポイント
を示し、今回の部分実装の番号ではありません。[ADR 0046](../adr/0046-reviewed-runtime-image-deletion.md)を参照してください。

## 未接続 Store の実装中の範囲

状態: **部分実装**。既存のイメージコマンドは Env 名の代わりに保持 Store ID を受け付けます。
新しいコマンドや必須オプションは追加しません。

```bash
haco plugin oci image list oci:store-id
haco plugin oci image delete oci:store-id sha256:<displayed-digest>
```

確認対象は正確な Store ID・所有者であり、古い一時 Env の識別情報は保持しません。
一覧・削除はそれぞれ一つの正規の maintenance run を取得し、全実行基盤呼び出しで
現在の Env 世代を確認します。run がキャンセル・Store 排他・後始末を所有し、元の
Workspace 対応や借用 Store を変更・削除しません。後始末が不明なら所有記録を保持します。
Host・Env・Store の混在した対象や古い確認結果は拒否します。未接続 Store の Docker は未対応です。

SandboxProvider の処理記録付き作成は、保持データなしで起動し、現在のネットワーク保護を維持し、
通常 daemon を mask してから Store を接続し、専用の containerd 2.3.3 メタデータサービスを起動します。
task・再起動・CRI・NRI・sandbox サービスは無効にします。保持設定・再起動 label・権限は引き継ぎません。
処理記録なしの作成とスナップショット restore は maintenance を拒否します。
[ADR 0047](../adr/0047-detached-store-maintenance.md) を参照してください。

Linux/WSL amd64 の構成は、保持 Store 接続前に対応ツールを自動配置します。
OCI module が固定した nerdctl 2.3.5 配布物の SHA-256 を検証し、設定済み Haco root 内の
非公開アーカイブキャッシュを再利用します。一時非公開ディレクトリに containerd・ctr・nerdctl
だけを準備します。Incus アダプターは所有・世代・保持マウント不在を再確認し、転送後の hash を
照合して disposable rootfs に配置します。取得したバイナリは Physical Host では実行しません。
準備失敗時は保持 Store を接続しません。利用者の準備コマンドは不要です。

キャッシュは排他制御し、symlink・hardlink・不適切な権限・破損した内容を黙って置き換えず拒否します。
アーカイブ内のパスで Host の出力先を選びません。署名付き download URL や応答 body を
取得エラーに含めません。非 Linux と amd64 以外の自動配置は現在未対応です。
ツール配置の native 検証は成功し、導入済みコントローラー全体の受け入れは未完了です。
schema 移行・自動 backup・任意の実行ファイルやソケットを選ぶオプションはありません。

maintenance は確認済みの既存 Store を指定し、`SkipDefaultResource` は併用しません。明示した Store は既定 Store の自動準備を通りません。矛盾する指定を正規のライフサイクルが拒否する契約を維持します。実 catalog／ライフサイクルの回帰テストで予約、元 Workspace 対応の保持、正常時と操作失敗時の後始末を確認します。

## Controller／CLIの検証

非接続nerdctlの製品構成・単体controller/CLIは`bd1c9a5`で成功しました。
導入済みStandard全体、通常ユーザー・デスクトップ、非接続Dockerは未完了です。
native primitive、自動ツール配備、模擬catalogの試験はそれぞれ異なる範囲です。
[検証証拠](../status/acceptance-evidence.ja.md#storage)が成功・失敗・制約を保持します。

## 未使用イメージ候補の確認

CLI の候補選択は実装済み、実 controller/CLI での一括検証は 9484d06 で成功しました。既存の
コマンドへ任意のフラグを一つ加えます。

```bash
haco plugin oci image list --unused dev
haco plugin oci image delete --unused dev
```

保持 Store ID または `--host` も指定できます。観測した一覧で、実行中・停止中の
どちらのコンテナからも参照されていないイメージを、タグ付きも含めて選びます。
dangling layer、未使用ビルドキャッシュ、回収可能な容量という意味ではありません。
表示した ID・タグを確認して承認します。`--yes` はその確認集合への明示的な同意です。
`--unused` とイメージ ID は同時に指定できません。

選択した不変の ID ごとに既存コントローラーの削除契約を使い、現在の所有者・世代、
最新の参照、非 force 削除、削除後の不在を確認します。確認後に現れたイメージは追加しません。
新しい参照や所有者の変更は削除を拒否できます。最初の失敗で停止し、完了数を報告して
残りを保持します。rollback・隠れた backup はありません。独立スナップショット・Store、
コンテナ、キャッシュはこの GC の対象外です。

779b0e5 の実環境検証は、候補削除の確認後に全体の 720 秒期限で失敗しました。一括削除の成功とは扱いません。拡張した検証用構成は固定の step 番号と所要時間を記録し、20 分の期限を設けます（Go テストは 22 分、job は 45 分）。製品の操作期限・所有確認・拒否／保持の検証は変更せず、修正後は 9484d06 の [run 34493016558](https://github.com/SLktEx/Hacocoon/actions/runs/34493016558) で 975.86 秒で成功しました。候補一覧、拒否時の保持、確認後の削除、参照中イメージの保護、一時資源と試験用 Store の正確な後始末を確認しました。この検証用構成では detached の一覧・拒否操作が各約 95–96 秒、確認後の一括削除が 288 秒かかっており、大量の候補を高速に処理できる実証ではありません。
