# Packerの構築処理をEnv内で実行する

状態: 採用、実装候補。 [English](0075-guest-packer-provisioning.md)

## 決定

実PackerのHCL2評価と、すべてのplugin・provisioner・post-processorを、共通のライフサイクルで
所有する通常の一時ビルドEnv内で実行します。任意のPackerアダプターにはleaseに結び付いた実行関数だけを
渡します。CoreにはPacker SDK・HCL解釈・plugin管理を持ち込まず、構成処理がアダプターを選びます。

組み込みのnull builderは、Env内で生成した一時鍵で同じEnvのループバックSSHへ接続します。
依存ツールとPackerの取得には通常のEnv通信ポリシーを使います。CLIは上限付きの通常ファイルの内容を
非公開の制御経路で送り、Hostマウントを渡しません。既存の非公開Workspace／OCIデータ、再利用可能な
Host認証情報、制御ソケット、Incus管理権限も渡しません。Envのrootは自分の内容や成功結果を偽装できますが、
それによってHost権限を取得することはできません。作ったBaseも次回の通常作成時に未信頼として扱います。

作成・正確な所有記録とlease・停止・公開・後片付けは、既存のBaseサービスが管理します。
Packerの責務はEnv内の構築だけです。Incusのイメージ所有情報とaliasの照合を正とし、別カタログは作りません。
入力ファイルと一時鍵は公開前に削除し、後片付け・公開の成否が不明な場合は従来の復旧状態を維持します。

## 採用しない方法

特権HostでHCLを実行すると、主なshell処理がEnv向けでも、`shell-local`・plugin・post-processorが
Hostの権限を使えます。EnvへIncusソケットや再利用可能な認証情報を渡す方法も同じ境界を壊します。
既存のコミュニティ版Incus Packer builderはインスタンスの作成・公開を直接行うため、そのままでは
Hacocoonの共通所有処理を迂回します。HCLを旧JSONの`run`へ変換すると実Packerの意味を失います。
新たなHost側builderやカタログを設ける方法も、ライフサイクルの判断を重複させます。

対象は同一PCのLinux／WSL開発です。AMI・QEMU・cloudへの移植性は追加しません。
[コマンド・入力・失敗時の契約](../design/packer-base-builds.ja.md)を参照してください。
