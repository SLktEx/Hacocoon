# ADR 0113: 共通作成サービスと固定Workspace構成

状態: 採用

Issue #728では`open --new [IMAGE]`を作成・起動・接続の通常経路とし、
`create IMAGE`を停止状態の作成primitiveとして維持します。別の一時実行CLIや
作成後のWorkspace差し替えは、このモデルと整合しません。

両者は共通の作成元解決と正規ライフサイクルを使います。Snapshot指定時は保存済み
構成を使い、それ以外は明示Image・設定済み既定Imageの順です。設定は保護catalogの
参照であり、既存Environmentを変更しません。provider作成は起動せず完了でき、
初回startでcreation identityに基づく再実行可能なguest初期化を行います。

自動WorkspaceはEnvironment所有、明示Volumeは独立した寿命を持ちます。
既存の排他的leaseを共用し、データ欠損やエディタ失敗で完成済みEnvironmentを置換
しません。所有証跡と不明時に拒否する削除を維持し、細かな進捗state machine、
暗黙のVolume差し替え、Snapshotによる既存環境の上書き復元は採用しません。

Snapshotは独立したWorkspace・OCIデータとともに通常の再利用可能Imageも作成します。
元の起動状態は変えません。アプリケーション整合性が必要な場合は利用者が書き込みを
停止します。[意味仕様](../design/environment-creation.ja.md)を参照してください。

既存Environmentのexecは削除と共通のlifecycle排他と、起動中のprovider観測を要求します。
一時Environmentの作成・削除権限は持ちません。Workspace削除はEnvironment終了の原子的な
処理で正確な所有関係を未完了cleanup参照へ移します。途中失敗時に所有関係を失う設計や、
不在確認前に名前を再利用する設計は採用しません。
