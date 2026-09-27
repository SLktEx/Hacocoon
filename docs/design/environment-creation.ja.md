# Environment作成と変更できない構成

日本語 | [English](environment-creation.md)

`haco open`は最後に開いたEnvironmentを再開し、停止中なら起動します。
Environmentが0件なら既定Imageから作成します。Repositoryが0件でも空のWorkspaceで
開始できます。`haco open --new [IMAGE]`は常に新規作成し、`haco create IMAGE`は
停止状態で作成するだけです。`--name`を省略すると名前を生成します。

## 作成元の選択

共通の作成サービスが次の優先順位で選択します。

1. `--snapshot SNAPSHOT`：保存済みImage・Workspace・OCIデータ・構成を使います。
   現在のRepository登録や既定Imageは使いません。
2. 位置引数`IMAGE`：指定Imageを使います。既定設定は変更しません。
3. どちらもない場合：設定済み既定Imageを使います。

Snapshotと明示Image・Volumeの併用は拒否します。`create`はImage必須です。
両コマンドは共通のEnvironmentライフサイクルを使い、`open`のみ起動・接続を続けます。
`--client none`でクライアント起動を省略でき、その場合は`--json`も指定できます。

`haco image default`で確認し、`haco image default IMAGE`で存在確認後に変更します。
Host初回セットアップは`haco/ubuntu-26.04`を取得し、未設定の場合のみ既定にします。
保護されたEnvironment JSON catalogに`default_image`と`last_opened`を保存します。
新しいresource種別ではなく参照設定です。既存EnvironmentのImageは変わりません。


Catalog version 17で参照と所有フィールドを追加します。Version 16からの更新では
既存Environmentのデータ寿命を変更しません。既定Imageが未設定の既存環境では
setupまたは`haco image default IMAGE`を実行します。旧バイナリは新catalogを拒否し、
新フィールドを失う書き戻しを防ぎます。

## Workspaceの所有とVolume

自動Workspaceは登録Repositoryの独立コピーを持ち、各remote default branchから
開始します。登録変更は今後の作成にのみ反映します。以後のbranch状態はGitが管理します。
EnvironmentはImage・Workspace identity・所有関係・任意のVolume名を固定します。

`haco volume create NAME`は空データ、`--container ENV`は現在のWorkspaceの独立コピーを
作成します。`open --new --volume NAME`または`create IMAGE --volume NAME`で選択し、
通常Workspaceは別に作りません。配置先は`/workspace`で、複数repositoryの相対配置は
維持します。停止中も含め排他的leaseで二重利用を拒否し、作成後の接続変更は提供しません。

`haco env delete ENV`は実行中なら拒否し、`-f`は停止後に削除します。自動Workspaceは
所有Environmentとともに削除します。明示Volumeは保持し、未使用時に`haco volume rm`で
削除します。旧来の明示管理Workspaceは従来の保持期間を維持します。

## 失敗時の境界

新規作成失敗時はprovider instanceの不在を確認してから新規所有データを削除します。
不明な所有関係・削除結果では予約を保持し、復旧が必要と報告します。作成済みEnvironmentは
起動・エディタ失敗でも保持し、`haco open`で再試行できます。壊れた既存Environmentを
自動再作成したりWorkspaceを差し替えたりしません。

細かな進捗state machineは追加せず、所有証跡とresourceの対応を保持します。
[所有原則](../adr/0002-environment-lifecycle-ownership.md)と
[設計判断](../adr/0113-unified-environment-creation.ja.md)を参照してください。

## 名前・実行・再試行

Snapshotは`haco snapshot create --name NAME ENV`で名前を指定できます。Snapshot・
Image・Volumeの同名を拒否します。名前予約のロックでVolume作成との競合を防ぎ、
Image公開はIncusの原子的なalias作成に任せます。

`exec`と`env exec`は起動中Environmentだけで実行し、終了値を返します。`commit ENV IMAGE`
はWorkspace・Volumeを含まないrootfs Imageを作り、元Environmentの状態を変えません。
`image tag SOURCE TARGET`は同じImageへの名前を追加し、既存Environmentのrevisionを変えません。

自動Workspaceの削除失敗時は、削除済みEnvironmentとWorkspaceの所有関係をcatalogの
`owned_workspace_cleanup`に残します。同じ`rm`で再試行でき、完了まで名前・データを再利用しません。
Repositoryの再登録はremote default branchをfetchします。Haco所有の取得材料が壊れている場合、
元Gitメタデータを退避して再取得します。ネットワーク・認証失敗はエラーとして返します。
Collectionは既存provider inventoryの上限253件までで、8件制限はありません。

旧 in-place 復元準備 API と新規 staging 作成経路は削除した。旧カタログの所有関係を失わないため、legacy staging metadata の読み取りと所有者を検証する cleanup のみ移行用として残す。

Snapshot のデータは旧 Repository 登録がなくても開ける。元 source が存在しなければ Git broker 接続は提供しない。open が代わりの source を登録したり、別 Repository の認証権限を付与したりすることはない。
