# データ退避と移行の確認

[English](data-evacuation.md) | 日本語

状態: **部分実装（保守手順の一部を実装）**。環境全体の取得、再構築、比較、置換は未完了です。
この文書はPhysical Hostの管理者向けです。通常の開発では
[Envのexport/import](../design/environment-transfer.ja.md#commands)を使います。

## 保持が必要なものを決める

管理Workspace（collection全メンバーとGit メタデータを含む）、OCI Store、
保存rootfs・スナップショット、コントローラーの関連付け・ポリシー・設定を調べます。
信頼済みHostのファイル・資格情報、手動・未登録データ、外部volume/pool/VHD、
Windows側の参照も別途確認します。Baseの参照はBase実体の保存ではありません。

読み取れるデータの退避にスナップショットやexportの成功は必須ではありません。
スナップショットの作成・削除が失敗しても、所有記録と元データを保持してください。
残存スナップショットが領域を参照して容量回収を妨げても、ファイルが読めなくなるとは限りません。
移行を進めるためだけに記録を削除したり、権限を変更したり、スナップショットを追加作成したりしません。

## 読み取り専用の棚卸し

現在のロードマップでは必要な現行データを保持します。古いバージョンの再構築や旧WSLの
置換は対象外です。以下の手順は、環境全体の移行を前提にせず、個別に確認したデータにも使えます。

既存のIncus管理権限を持つPhysical Host上で、リポジトリのルートから実行します。
パス・所有情報も機密になり得るため出力を保護します。
コントローラーのrootを変更している場合は実際の場所に置き換えてください。

```bash
umask 077
python3 tools/evacuation_inventory.py --catalog /var/lib/hacocoon/state/environments.json --repositories /var/lib/hacocoon/state/repositories > inventory.json
```

存在しないカタログのオプションは省き、その分は未確認事項として残します。
helperはproject、プール、イメージ、instance、custom ボリューム、スナップショットを照会し、
一部の照会失敗があっても取得済みの結果を保持します。
native照会は最大256回・照会間の合計5分、個別照会の期限は30秒です。
終了値1または`native_queries_complete: false`は取得未完了を表します。

投影するのは選択した参照・所有者識別情報だけで、任意の設定・資格情報は出力しません。
URI形式の参照は伏せ、ファイル参照は追跡しません。イメージは完全なfingerprint・種類・alias・
nativeの元projectを記録します。共有projectの表示を別々の所有とみなしません。
現行カタログschema16を変更せず読み取ります。既存の10–13の参照読み取りは変更せず、その他のschemaは非対応です。`state_validated`はfalseのままです。

現行の追加データ配置、Env専用領域と元の世代、選択中のキャッシュ世代、公開元・生成元の参照、コピー完了・import待ち・実行実体不在の記録を含みます。`catalog_links`は台帳内の参照を実ボリュームの観測と分けて照合します。削除済みの生成元や以前の世代は正当な履歴の場合があるため、見つからない参照を自動的な破損判定・採用・削除許可に使いません。ファイル内容・任意設定・資格情報は出力しません。
投影しない復元・コピー・runの途中記録は件数を示し、別途の確認が必要です。

関連付け比較は最大4096行で、識別情報の存在・不足・不一致、非対応参照、
曖昧なproject表示、照会失敗を区別します。native側から参照が見つからなくても、
**孤立resourceや削除候補の一覧ではありません**。世代、所有、保存child、
未知の参照は別途確認します。終了値0でも`authority`はfalse、
`review_required`はtrue、`backup_complete`はfalseです。

手動ファイルには`--files /absolute/reviewed-root`を追加します。
Linux走査はメタデータだけを制限付きで記録し、通常ファイルの内容、ACL/xattr値、
symlink先を読みません。パス中のsymlinkを追わずディレクトリとマウント IDを固定・照合します。
マウント境界、symlink、特殊ファイルは保留にします。読取り失敗・観測中の変化・
50,000項目/64階層/60秒の上限で部分結果を保持します。
全ファイルの列挙も、内容取得や旧WSL削除の許可ではありません。

## 確認したファイルツリーを取得する

### 名前付きの保存対象一覧を作る

棚卸しの後、保持するものを非公開の一覧へ記録します。LinuxまたはWindows上の
リポジトリから雛形を出力します。Windowsで保存する場合はUTF-8を指定してください。
以下はLinuxでの例です。

```bash
umask 077
python3 tools/evacuation_selection.py template > selection.json
```

JSONを編集し、`managed-data`（管理対象）、`host-settings`（Host設定）、
`manual-files`（手動ファイル）、`external-data`（外部データ）を確認します。
対象を列挙した分類は `status: "listed"`、必要なデータがない分類は理由付きの
`status: "none"` にします。`pending` は未確認のまま残ります。
ツリーごとに重複しない分かりやすい `name` と、場所・内容を示す `source` を記入します。
`decision` は保持する `retain`、作り直す `recreate`、対象外の `exclude` から選びます。
作り直しと除外には理由が必要で、結果にも残ります。資格情報や設定の中身は記入しません。

保持対象を既存手順で保存・復元し、後述の `evacuation_compare.py scan` で両側を調べます。
結果のファイルを `source_manifest` と `restored_manifest` に指定します。
絶対パス以外は `selection.json` のある場所からの相対パスです。
両方のHostが同時に動いている必要はありません。次に照合します。

```bash
python3 tools/evacuation_selection.py report selection.json > selection-report.json
```

一覧には指定した名前ごとに、一致 `matching`、差分 `different`、調査不足 `incomplete`、
未作成・読めないマニフェスト、未確認、作り直し、除外と次の操作が出ます。
一項目が読めなくても他の比較結果を残します。既存の上限付き読込・照合処理で一組ずつ処理し、
一覧の上限は1 MiB・4,096項目です。一覧に書かれたコマンドの実行や元ツリーの読込はしません。

終了コード0は全分類と判断を確認済みで、保持対象が一つ以上あり、その選択範囲の観測が
すべて一致した場合です。条件を満たさなければ1、一覧を読めなければ2です。
0でも、作り直し・除外したデータが検証済みのバックアップになるわけではありません。
同じファイルを両側に指定した場合は拒否しますが、別ファイルも単なるコピーかもしれず、
独立した取得の証明にはなりません。`authority: false` と `backup_complete: false` を常に残します。
対象選択の網羅性、所有者の対応、独立した保存、通常のアプリ利用は別途確認します。
このヘルパーはリポジトリから使う保守用で、インストール済みの `haco` コマンドではありません。
データの取得・復元・削除は行いません。

### 選択した元データを取得する

すべての書込み元を停止します。取得元の外に、実行者所有の新しい空ディレクトリを
モード 0700で用意します。置換後も残る保存先を選び、Linuxのリポジトリルートで実行します。

```bash
umask 077
mkdir -m 700 /absolute/private-capture
python3 tools/evacuation_capture.py /absolute/reviewed-source /absolute/private-capture --quiesced
```

両方の絶対パスを置き換えてください。helperはGNU tarを直接使い、
`data.tar`、開始記録、`data.tar.sha256`、完了記録を作ります。
暗号化、鍵作成、Incus スナップショット、カタログ変更は不要です。
アーカイブと記録を保持先へコピーし、そのディレクトリで確認します。

```bash
sha256sum --check --status data.tar.sha256
```

完了にはtar成功、出力sync、観測した元メタデータと固定したディレクトリ・出力IDの不変が必要です。
パス中のsymlink、列挙不足、別マウント、特殊ファイルは拒否します。
ファイルsymlinkは参照先を追わず保存します。数値UID/GID、モード、link、ACL、xattr、
スパース情報を保持しますが、**Incusのidmapは変換しません**。

`--byte-limit`と`--seconds`の既定は64 GiBと900秒です。
失敗時は途中出力と開始記録を保持し、完了記録を作りません。
既存ファイルの上書き・自動削除はせず、再試行には新しい出力先を使います。
終了させるのはこの呼出しが作った子プロセスだけです。

`--quiesced`は作業者の確認であり、書込み検出や不可分なスナップショットではありません。
checksumは転送破損を検出しますが、アーカイブとchecksum両方の差替えは防げません。
記録は非公開に保ちます。過去の任意の暗号化取得を開くには元の鍵が必要で、
新しいアーカイブ・試験鍵では失った鍵を復元できません。
[復号鍵を失った試験の記録](../status/acceptance-evidence.ja.md#transfer)も参照してください。

## 復元・比較してから置換を判断する

通常のツリーは適切なnativeツールで新しい非公開な場所へ復元します。
不正なアーカイブを稼働中のコントローラー状態へ展開したり、旧管理設定を有効化したりしません。
現在の所有、設定、許可を意図して再構築します。管理bundleは公開importと新しい接続を使います。

必要な内容、種類、モード、ゲストから見えるUID/GID、Git履歴・未コミットファイル、
アプリデータを比較します。必要に応じてhardlink、symlink、ACL/xattr、外部マウントも調べます。
Incusのidmapが異なるとHostの生UID/GID比較は正しくありません。
必要な再起動・同名Env再作成後にも動作を確認します。

管理bundleのWSL間移送一構成と、スナップショット失敗・読み取り可能rootfsの隔離試験は成功しましたが、
[失敗と制約](../status/acceptance-evidence.ja.md#transfer)が残ります。
環境全体の確認とG4の最終置換は未完了です。必要な比較が完了するまで旧WSLと独立保存データを保持し、
棚卸しや一度のimport成功だけを削除の根拠にしないでください。

## 通常のIncusイメージを退避する

状態は**G2/G3の部分実装**です。今後の環境作成に必要なイメージは、Incus標準のexport/importで保持できます。独立保存したスナップショットのrootfsは元のBaseイメージに依存しません。この手順はHacocoonのカタログ項目やスナップショット構成要素を追加しません。

元のPhysical Hostで、棚卸しから確認した完全なfingerprintと実際のイメージ所属projectを指定します。新しい非公開ディレクトリを選び、途中で失敗したら中断してください。

```bash
umask 077
mkdir -m 700 /absolute/new-image-export
incus image export FULL_FINGERPRINT /absolute/new-image-export/image --project SOURCE_PROJECT
ls -l /absolute/new-image-export
```

出力されたすべてのファイルを保持します。確認済みの分割形式では`image`がメタデータ、`image.root`がrootfsです。単一形式では`image.tar`などの出力になります。接頭辞からファイル構成を推測せず、既存出力へ再exportしないでください。実際の各ファイルのSHA-256を計算し、元のWSLの外にある新しい保存先へコピーして、import前にそこで照合します。

移送先のPhysical Hostでは、新しいproject名と固有の所有説明を作成前に記録します。イメージを他projectと共有しないよう、専用の名前空間を明示します。

```bash
incus project create RESTORE_PROJECT --description UNIQUE_RESTORE_DESCRIPTION -c features.images=true
incus project list --format=json
incus image import /absolute/retained/image /absolute/retained/image.root --project RESTORE_PROJECT
incus image list --project RESTORE_PROJECT --format=json
```

import前に、記録した説明と`features.images`を一覧で検証します。単一アーカイブなら、その実際のパスだけをimportに渡します。移送後の完全なfingerprintとイメージ種別が元と一致すること、保持したファイルが変わっていないことを確認してください。

失敗時は結果と作成済みの正確なリソースを記録し、削除対象を推測したり既存projectを置き換えたりしません。元データと退避アーカイブは保持します。この操作はHacocoonのBase登録、alias復元、旧権限の引継ぎ、Env作成や起動確認を行いません。分割イメージ2件の移送結果と未確認範囲は[検証証拠](../status/acceptance-evidence.ja.md#transfer)に記録しています。

## 復元したツリーを照合する

保守用の補助ツールで、Linux/WSLそれぞれの停止済みデータから照合用一覧を作成できます。
出力は対象ツリーの外へ置き、非公開で保持してください。パスや内容のハッシュも機密情報に
なり得ます。それぞれのHostで、実際の絶対パスを指定します。

```bash
umask 077
python3 tools/evacuation_compare.py scan /absolute/source-tree --quiesced > /absolute/private/source.json
python3 tools/evacuation_compare.py scan /absolute/restored-tree --quiesced > /absolute/private/restored.json
```

2つの一覧を同じ非公開の場所へ移し、LinuxまたはWindowsで比較します。

```bash
python3 tools/evacuation_compare.py compare source.json restored.json > comparison.json
```

終了コード0は選択した観測の一致、1は差分または走査未完了、2は比較を読み取れない状態です。
JSONで追加・欠落した相対パスと変わった項目を確認できます。ファイル内容、たどらない
symlinkの参照先、mode、数値UID/GID、ツリー内hardlinkの関係、通常ファイル・ディレクトリの
ACL/xattrを比較します。リンク先やxattrの値そのものは出力せず、ハッシュで照合します。
ツリー内のGit情報・未コミット・未追跡ファイルも通常のファイルとして含めます。

所有者は同じ名前空間で比較してください。Incusのidmapが異なるとHost側の数値IDは正当に
変わる場合があります。適切なguest側で比較するか対応を確認し、一致させるためだけに
所有者を書き換えないでください。

既存の安全な一覧処理を再利用し、ファイル・ディレクトリを開いて保持します。symlinkや
別mountをたどらず、読み取り不能・途中変更は未完了として残します。既定の範囲は
50,000項目・ファイル読み取り64 GiB・処理間で確認する900秒で、停止したfilesystem呼出しは
期限を超える場合があります。書き込み停止は操作者が行う必要があり、原子的なsnapshotでは
ありません。時刻精度、filesystem固有flags、symlinkのACL/xattr、ツリー外hardlink、
実行中アプリの整合性は未確認です。一致は独立保存・認証の利用・元データの削除許可を
意味しません。エディタ・ビルド・OCI・Gitでの通常利用も別に確認してください。
この2つの操作はデータの修復・削除を行いません。
