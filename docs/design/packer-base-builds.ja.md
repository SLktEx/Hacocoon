# PackerでBaseを作る

日本語 | [English](packer-base-builds.md)

状態: 開発候補で部分実装。実Packerを通常の使い捨てEnvで動かす経路を実装しています。
導入済み環境での受入は別に確認します。公開・revision・保持・確認付き削除は、既存の
[Baseのライフサイクル](base-images-and-custom-environments.md)が管理します。

## 再利用するツールを作る

サンプルのフォルダが手元にあるLinux／WSLのクライアントから実行します。

```bash
haco base build --name my-tools --from haco/ubuntu-26.04 examples/packer
haco base inspect my-tools
haco env create --base my-tools --workspace managed:my-project dev
haco ssh setup dev
ssh haco-dev my-tool
```

[サンプル](../../examples/packer/base.pkr.hcl)は実際のHCL2と
`provisioner "shell" { script = "setup.sh" }`を使います。shellは別ファイルのまま扱い、
Hacocoon専用JSON定義は不要です。`--from`を省略すると既定のBaseを使います。
オプションはフォルダより前に指定してください。作成したツールは`hello-from-packer`を
表示します。再ビルドで`my-tools`の参照先が更新されても、既存EnvのBase revisionは変わりません。

ビルドEnv内でPacker 1.16.0の`fmt`、`init`、`validate`、`build`を順番に実行します。
整形するのは転送したコピーだけです。HCLの評価・変数・plugin・外部shell・`shell-local`・
post-processorもすべてEnv内の権限で動きます。変数値には通常の`variables.auto.pkrvars.hcl`を使えます。
サンプルのSSHポートと鍵は変数の初期値から取得します。Packerの
[env関数](https://developer.hashicorp.com/packer/docs/templates/hcl_templates/functions/contextual/env)は
この初期値で使い、sourceブロックへ直接書きません。

組み込みの[null builder](https://developer.hashicorp.com/packer/docs/builders/null)が、
同じEnvのループバックSSH経由で構築します。Packer自体が成果イメージを返さないのは正常です。
Hacocoonが所有する同じEnvを停止し、既存のIncus Base処理でrootfsを公開します。
別のbuilderやpost-processorを記述しても、Hostの権限や公開対象は変わりません。

単独の`packer build`もEnv内で動かす通常のPacker操作です。この例を独立して使う場合は、
自分で準備したEnv内のSSHポートと秘密鍵のパスを`haco_packer_port`／`haco_packer_key`変数に
指定します。これはEnvの内容を構築するだけで、HacocoonのBaseとして登録しません。
管理されたBase公開の入口は`haco base build`です。Host側で単独Packerを起動する機能は設けません。

## 必要なツールと入力ファイル

通常のBase選択・Env作成にPackerは不要です。Packerビルドでは、Python／OpenSSHがなければ、
使い捨てのUbuntu Env内で通常の`apt-get`を使い、Python 3・CA証明書・OpenSSHを準備します。
カスタムBaseはこれらを事前に含められます。不足時にaptもなければ依存ツールの段階で失敗します。
このpluginがHostへツールを導入することはありません。

Packerの公式Linux amd64／arm64アーカイブはEnvが`releases.hashicorp.com`から取得し、
固定のSHA-256を検証してから所定の実行ファイルへ書き込みます。取得量・展開量を制限し、
アーカイブが指定するパスは展開しません。パッケージ・Packer・pluginの取得には通常のproxyと
承認経路を使います。拒否された取得は失敗し、許可ルールの追加や通信制限の回避は行いません。
ビルドごとに新しいEnv内の領域へ取得します。世代をまたぐパッケージ／Packerのキャッシュは
この変更には含まれません。

入力は通常ファイル128個・合計512 KiBまでです。パスはASCII英数字・下線・ドット・ハイフンを
使い、各要素の先頭は英数字または下線にします。深さは8要素、長さは240文字までです。
フォルダ直下に少なくとも一つの`.pkr.hcl`が必要です。`.git`・`.env`などドットから始まる項目は
送りません。それ以外の不正名、symlink、hardlink、特殊ファイル、衝突、上限超過は拒否します。
読み取り中はフォルダ・ファイルの実体を照合し、HostのフォルダをEnvへマウントしません。
これは設定とスクリプトを渡す仕組みで、巨大リポジトリの転送手段ではありません。
選んだ通常ファイルは未信頼のビルドEnvへ渡るため、そこにも認証情報を含めないでください。

入力の収集はLinux／WSLに対応し、それ以外のネイティブクライアントでは未対応です。
Windowsでは通常のWSL／Host入口からLinuxのコマンドを使います。Windowsファイルの読み取りと
導入済みWindows→WSL経路の受入は、Linuxの部品テストとは別に記録します。

## 結果と失敗時の対応

`--json`はBaseの名前・revision、状態、残ったビルドEnvの名前を返します。依存ツール準備、
Packer準備、整形、初期化、検証、構築の失敗には段階を表示します。`--output`を加えると、
失敗した段階の非公開の標準出力・標準エラーも各16 KiBまで表示します。JSONには省略フラグが付きます。
人向け表示では引用表現にし、Envが出した端末制御文字をそのまま実行させません。
出力にはスクリプトの内容やスクリプトが出した秘密情報が含まれ得ます。controllerのエラー・ログへは
渡しません。成功した段階の出力は保持しません。

中断・公開前の失敗は、所有者を照合した共通の一時Env削除へ進みます。削除の成否が不明なら
所有を保持し、復旧が必要な状態を返します。公開の成否が不明なら、ビルドEnvとIncus側の証拠を
保持して確認を求めます。公開後にEnv削除が失敗してもBaseは残します。再試行は新規ビルドです。
一時SSH鍵・Packer・入力ファイルは`/run/hacocoon/packer`に置き、公開前の既存の初期化処理で
削除します。利用者のスクリプトが別の場所へ書いた秘密情報を検出・除去する仕組みではありません。

既存の`haco base build base.json`では、`name`・任意の`from`・長さを制限した`run`を使う
単純なshell定義も指定できます。HCLをこのJSONへ変換する必要はありません。非公開の制御要求では構築方式を一つだけ指定でき、
どちらも同じライフサイクル・lease・Incusのカタログを使います。
[ADR 0089](../adr/0089-guest-packer-provisioning.ja.md)を参照してください。
