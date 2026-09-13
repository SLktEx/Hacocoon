# Environmentの移送

[English](environment-transfer.md) | 日本語

状態: **部分実装**。Linuxのexport/import、導入済みコントローラー、
Windows投影ファイルからのimport、管理データを別WSLへ移す一構成を確認済みです。
停止したcontainerdのイメージID・書込みデータの移送も実Incus/Btrfsで確認しました。
保存したコンテナーの再開には明示的な起動が必要で、稼働プロセスは移送しません。
環境全体の退避、import後の認証Git、広いOCI・アプリの整合性は未完了です。
[成功・失敗の検証記録](../status/acceptance-evidence.ja.md#transfer)は設計と分けて管理します。

<a id="commands"></a>

## exportとimport

選択したファイルを読み書きできるLinux側（通常は信頼済み`haco-host`）で実行します。
まずアプリの書込みを止め、元のEnvを停止します。

```bash
haco env stop dev
haco env export dev dev.haco
haco env import dev.haco recovered
haco open --client ssh recovered
```

exportの既定出力は実行ディレクトリの`<source>.haco`、importの既定名は
`<source>-imported`です。どちらも`--json`に対応します。既存ファイル、
既存Env、未解決の作成先リースは拒否します。exportは停止済みデータの取得を
内部で行うため、事前のスナップショットコマンドは不要です。

importは新しい管理Workspace/OCIと新しい稼働Envを作成します。アーカイブ、
元Env、既存データは変更しません。Baseは由来を表すだけで、Base実体やimport前の
backupは不要です。以前の承認・接続・管理権限は復元しません。SSHを新規設定し、
[Gitガイド](../guides/git-workflow.ja.md)に従って接続先・許可を確認してください。

exportの確定にはext4/BtrfsなどLinuxの`openat2`/`O_TMPFILE`対応が必要です。
非対応なら明示的に失敗します。Host内での出力はWindowsデスクトップへの出力ではありません。
検証済みbundleを既存のドライブ投影経由でWindowsへコピーし、サイズとSHA-256を確認後、
読み取り可能な投影パスからimportできます。

```bash
haco env import /mnt/c/Users/USER/Backups/dev.haco recovered
```

`USER`とファイル名を実際の保持先へ置き換えてください。投影ファイルでの成功は、
直接DrvFSへのexport確定やWindowsネイティブCLIの合格を意味しません。
[退避・復元比較](../guides/data-evacuation.ja.md)で必要データを独立して復旧できると
確認するまでは、元の環境を保持してください。

対応基準のIncus 7.0 LTSでは、native volume exportの出力先が既存扱いになるため、
controller自身が所有する生存中の匿名FDだけに`--force`を使います。利用者の出力パスは
このコマンドへ渡さず、既存の出力ファイルは引き続き拒否します。
[ADR 0049](../adr/0049-transfer-envelope-authority.md)を参照してください。

失敗時は非ゼロで終了し、保持したresource名をJSONまたはstderrへ示します。
応答が失われても処理が完了または継続している可能性があるため、再試行前に記録を確認します。
自動再実行、上書き、カタログ編集による復旧は提供しません。

## Incusの機能と権限

Incusはrootfsを単一イメージ、接続したcustom ボリュームを別々に出力します。
instanceアーカイブだけにはWorkspace/OCIの内容は入りません。
通常形式はプール間で移せますが、optimized形式は対応するストレージ driverが必要です。

Incus 6.0.5には新しい版のinstance-import設定上書きフラグがありません。
現行アダプターは一時的な所有イメージをimportし、現在の明示的な設定と空のプロファイルで
独立instanceを作成します。データを検証し、init直後に所有記録を永続化してから
ネットワーク・送信元ガード・SSH IDを再構成し、起動します。元の管理設定では起動しません。

取り込んだラベルやchecksumはデータの説明であり、リース・ポリシー・Hostパス・
資格情報・設定を適用する権限ではありません。管理用コントローラー接続先は
ゲストGit・通知ソケットへ公開しません。内部アーカイブを任意のHostパスへ展開しません。

## bundle形式と検証済みの一時保存

`internal/environmenttransfer`は制限付きUSTARと正規形JSONを使います。
項目は`manifest.json`、`rootfs.tar`、`workspace.tar`、連番の
`workspace-002.tar`～`workspace-253.tar`、任意の`oci.tar`です。
メタデータは64 KiB、外側の付加領域は512 KiB、公開操作のpayload合計は64 GiBが上限です。
これは検証上限であり、Incus書込み中のディスクquotaではありません。

version 2はリポジトリ名とGitHub接続情報を保存します。version 1は引き続き読め、
オフラインでimportします。古いreaderはversion 2を拒否します。
元のローカルファイル接続もオフラインになります。情報不足から移行先Hostへの権限は導出しません。
現行importはWorkspaceメンバー**最大8個**で、それより大きい有効bundleも変更前に拒否します。
接続情報の一部だけが存在する状態は無効です。オンライン接続には現在のHostに登録した
リポジトリのremote・branchとの完全一致が必要です。

検証ではJSONの重複・未知項目、役割の不足・余分・順序違い、サイズ・hash不一致、
overflow、link・拡張header、終了block不足、末尾データを拒否します。
consumerやnative変更を呼ぶ前に全体を検証します。writerは保護したスナップショット全体と照合し、
不正なmanifestだけで取得の完全性を判断しません。旧Base記録はカタログに残りますが
Baseアーカイブは不要です。未知の役割は拒否します。

Linuxの一時保存はコントローラー専用ディレクトリを固定して匿名ファイルを作り、
書込みdescriptorを閉じて全体を検証してから、範囲を制限した読み取り専用readerを返します。
各構成要素のcursorは独立し、隣接データは読めません。bundleを閉じるとreaderも無効です。
パス置換で固定済みデータは置換できません。ただし特権によるプロセス descriptor操作は
この保証の外です。弱いファイルシステムへの代替経路、権限修復、永続的な一時保存復旧台帳はありません。

## 取得・通信中の寿命

`ReadSnapshot`はEnv→Workspace順の正規ロックを読み終わるまで保持し、
所有IDとready状態を再検証します。返されたスナップショット値を保持するだけでは予約になりません。
consumerは返る前に読み終え、ライフサイクル操作へ再入できません。
キャンセルは保存元を削除せずロックを解放します。

ボリューム exportは前後に非接続状態と正確な所有を確認します。Incusが後始末のbackup削除エラーを
無視し得るため、前後のnative backupのID・時刻・フラグを比較し、新規・変更済みの残存や
最終確認失敗があれば成功にしません。既存backupを名前だけで削除しません。

rootfsは選択した非公開 Unix Incus remote/projectを使い、暗黙の接続先へ切り替えません。
HTTPS・cluster remote・分割イメージは非対応です。メタデータと転送を制限し、
redirectを拒否してfingerprint/hashを照合します。rootfs importは非圧縮の単一
x86_64/aarch64 container イメージを対象に、propertyを新しい所有IDへ置換し、
作成templateを取り込みません。元アーカイブは変更しません。

一時イメージの公開前にソケット・project・ランダムな所有者を非公開処理記録へ記録します。
native operationとfingerprintを後続検証前に永続追記します。aliasがなく所有が一致する
イメージだけを削除でき、不存在確認後に処理記録を削除します。不明な作成・後始末は記録を残し、
保存元データを削除しません。

管理ストリームは最大64 KiBの正規frameと、最後のバイト数・SHA-256を使います。
export成功には全体検証、後始末成功、EOFが必要です。クライアントは匿名ファイルをsyncし、
固定した出力ディレクトリへ同じinodeを上書きなしで公開します。
名前付きの途中ファイルや、パス名に基づく後始末はありません。

importは通常ファイルを読み取り専用で開き、末尾symlink・特殊ファイルを拒否します。
upload後、コントローラーでも全体を再検証します。処理期限は30分、uploadの無通信期限は30秒です。
切断・追加入力はactivationをキャンセルします。成功にはバイト数・hash一致、作成先の稼働、
EOFが必要です。

## 登録と失敗時の後始末

importはnative作成前に新しいIDを予約し、各作成完了を記録してメタデータとidmapを確認後、
Workspace/collection全体をreadyにします。元Gitのhook、clone、checkout、資格情報操作は
実行しません。collectionは一つのリースを持ち、メンバーは個別に解決できません。

OCIは新しいWorkspaceへ関連付けます。Env作成は明示したデータを正規ライフサイクルへ渡し、
現在のHostの既定OCIを追加しません。ボリュームメタデータは新しい所有者用に構成し直します。

作成完了・未公開の単一Workspaceを片付けられるのは、再読込した処理記録が一致する場合だけです。
native削除は所有、利用者、保存childを確認し、クライアントキャンセルとは独立した期限で行います。
完了不明の`creating`、公開済み`ready`、ID変更、collection部分作成は保持します。
後始末が成功しても元のimport失敗は成功になりません。

Env作成失敗時は公開済み・未利用の新規データだけを既存APIで片付けます。
OCIの後始末が不明ならWorkspaceも保持し、起動失敗ならEnvとデータを保持します。
削除を試みただけでリースを解放しません。

## 判断理由と検証

[ADR 0049](../adr/0049-transfer-envelope-authority.md)、[ADR 0052](../adr/0052-transfer-routing-metadata.md)、
[ADR 0053](../adr/0053-workspace-native-import.md)、[ADR 0054](../adr/0054-completed-import-cleanup.md)、
[ADR 0056](../adr/0056-native-rootfs-import.md)、[ADR 0057](../adr/0057-native-bundle-import.md)
が境界の判断理由と却下案を保持します。

リポジトリ試験は不正入力、読取り制限、パス置換、構成不足、ロック、永続処理記録、
新規権限、後始末失敗を扱います。native 構成要素、公開操作、WSL間移送の各試験は
確認範囲が異なり、[検証証拠](../status/acceptance-evidence.ja.md#transfer)に失敗・スキップも残します。
停止したcontainerdの試験は任意の稼働アプリの整合性を保証せず、
管理bundleは環境全体のbackupではありません。

## rootfs archive の Incus CPU 表記

実装済み: rootfs import は固定済み Incus SDK で CPU 名を解決し、既存の x86_64／aarch64
という対応 CPU の制限を保って、一時 transport image に正規名を書きます。amd64／arm64
など Incus の別名は同じ CPU を表し、不明または他の CPU は引き続き fail-closed で拒否します。
元 archive、所有確認、template 除去、資源の寿命は変えません。


[限定された実機検証](../status/acceptance-evidence.ja.md#development-branch-integration)を参照してください。
