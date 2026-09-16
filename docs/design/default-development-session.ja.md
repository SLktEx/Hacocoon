# 通常の開発環境

日本語 | [English](default-development-session.md)

状態: 初回のリポジトリ登録と繰り返しのopenはimplemented。
この一連の操作の導入済みIncus・Windows/WSL・デスクトップ実機確認は未実施です。

## 利用手順

インストール後、作業対象を登録してHacoを開きます。

```bash
haco repo add api https://github.com/OWNER/API.git
haco repo add web https://github.com/OWNER/WEB.git
haco open
```

引数なしのopenは登録済みの全取得元を選び、独立した作業コピーを準備して開発環境を
作成・再開します。既定Baseの解決・取得、設定済みOCI Storeの初期化、マウント、
通信の隔離、Git brokerの接続は既存の担当実装を使います。sshdがない場合は
SSH接続準備が通常のパッケージ取得・Policy経路で導入します。
Baseのbuild、Workspace・Environmentの作成、OCI操作を手動で行う必要はありません。

既定はRemote-SSH導入済みのVS Codeです。`--client ssh`はシェルを開きます。
`--client none --json`は環境を準備・再開して名前を返しますが、デスクトップ接続の
準備・起動は行いません。リポジトリが1個なら`/workspace`、複数なら
`/workspace/<repository>`で編集します。取得元とはファイル・Git情報を共有しません。
再度openしても編集中のファイルを取得元の内容で更新しません。

## 所有権と再実行

クライアントの参照を`~/.haco-default`に保存します。明示的なディレクトリopenと
同じ、ロック・所有者検証・atomic保存・fsyncを使います。最初の変更前にランダムな
準備名と整列済みの取得元IDを保存し、応答後にWorkspaceとOCIの所有者を固定します。
これは認証情報やcontrollerのカタログではありません。ゲストには参照も管理socketも
渡しません。symlink・hardlink・別所有者・書き換え可能な参照ディレクトリは拒否します。

変更は`workspace.workflow`と正規のリポジトリ・Environment lifecycle APIを使います。
同じクライアントの同時openは参照ロックで直列化し、別クライアント間も既存leaseで
制御します。応答が失われても準備名を保持し、同じ操作として再確認します。
未完了のコピー、古い所有者、Base・OCIの不一致、後始末の不明状態は拒否します。
再実行で保存データを削除したり、未確定の所有権を解放したり、既存環境を置換したり
しません。Environmentを明示的に削除した後は、保存された作業から正規経路で再作成します。

## 許可・進捗・失敗

リポジトリ確認、作業ファイル準備、環境準備、接続準備をstderrへ表示し、長時間の
処理中も定期的に案内します。JSONはstdoutに分離します。providerの生出力や
認証情報を進捗として表示しません。キャンセルは現在の要求へ伝えますが、
中断した結果だけでリソースが存在しないとは判断しません。

Policy・Approvalの制御は維持します。承認待ちは別の信頼済み端末で`haco approve`を
実行し、正規の回答後に処理を続行します。既定denyでは承認要求を作らないため、
必要な権限を`haco config --edit`で明示的に確認・変更して`haco open`を再実行します。
open自身がワイルドカードルール・保存済み承認・権限を追加することはありません。
失敗は処理段階と診断・再実行方法を示し、作業を保持します。クライアントソフトの
不足はデスクトップ側で解決してから同じopenを再実行できます。

## 明示的な操作と制限

環境名・ディレクトリを指定するopen、プレビュー、Workspace・Env・Base・ストレージ・
通信・設定の既存コマンドは引き続き使えます。`haco open --select`で既存環境を
対話選択できます。`haco ssh setup`の既存選択動作も維持します。通常のopenにも
`--base`・`--oci`を指定できますが、既存の互換性検証に従います。

現在の構成上限は取得元8個です。未登録ならrepo addを案内し、未完了の取得元を
黙って除外しません。準備済みの構成は不変です。後で登録を追加・削除した場合は
構成の不一致として報告し、編集済みデータを置換しません。変更には
[明示的なfork](workspace-workflow.md#choose-the-copys-repositories)を使います。
構成の自動拡張はplannedです。

実バイナリとUnix RPCによる複数repoの登録・open、独立した所有権、既定Baseの選択、
編集を保持した停止・再開、同時open、キャンセル、応答喪失、不正な参照を
リポジトリテストで検証します。Incus実機、パッケージ取得・承認、デスクトップ起動は
別途確認が必要です。[ADR 0109](../adr/0109-default-development-session.ja.md)を参照してください。
