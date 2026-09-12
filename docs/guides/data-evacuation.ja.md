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
カタログschema 10–13は移行せず読み、9は非対応です。`state_validated`はfalseのままです。
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
