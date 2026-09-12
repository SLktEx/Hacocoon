# 承認を経由する AWS 操作

[English](aws-operations.md) | 日本語

状態: **部分実装（D3の部分実装）**。S3一覧・ストリーム取得、IDに結び付いたアカウント表示名、ゲスト専用経路と通常CLIを実装済みです。導入済みゲストの拒否経路は確認済みですが、認証を伴う実AWS・デスクトップ確認は前提不足で未実施です。延期中のEC2 実行基盤とは別機能です。

## 普段の使い方

Host の任意の AWS 連携を準備した後、信頼された Host で実行します。

```sh
haco aws s3 ls s3://example-bucket/project/
```

Environment が一つなら自動で選択します。複数ある場合は `--env dev` を指定します。
`--profile` は `default`、`--region` は Host のそのプロファイル設定が既定です。
オプションは S3 URL より前に置きます。結果はキーとサイズの JSON 配列です。
Unicode は端末で安全に表示するため escape しますが、JSON を解釈したキーは変わりません。
大きすぎる一覧や途中までの取得は成功扱いにせず、prefix を絞るよう案内します。

承認が必要なら既存の通知・詳細画面、または別の信頼された terminal の
`haco approve` を使います。環境限定／全環境の許可・拒否・毎回確認は、
同じ `haco config`、監査、要求 ID を使います。毎回確認の保存と今回の判断は別です。
画面がなくても自動許可しません。AWS 認証前の Environment 作成識別を保持し、
capability サービスが再確認します。Environment に認証情報は渡しません。

## 任意の Host 準備と方針

Host の `/usr/local/bin/aws` に AWS CLI v2、`/usr/bin/python3` から import できる
botocore が必要です。通常の Hacocoon setup や Core の必須依存にはしません。
[AWS 公式インストール手順](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html)
と Host distribution の `python3-botocore` パッケージを使います。
home が `/root` の信頼された Host ユーザーで、たとえば
`aws configure sso --profile default` と `aws sso login --profile default`
によって認証します。SSO region に加え、通常の AWS region もプロファイルに設定します。
ログインと個別操作の許可は別です。

既存 Policy は自動変更しません。既定拒否なら対象操作の require-approval ルールが
必要です。[英語版の設定例](aws-operations.md#optional-host-preparation)を、
対象 bucket・prefix・region に合わせて `haco config` に追加できます。
例の識別 wildcard は毎回確認する入口で、保存される判断は実識別に限定されます。
明示的な require-approval ルールは保存済み allow より強いため、そのままなら毎回確認します。
保存した方針を使う意図なら、その明示ルールを削除・限定するか、
既存の require-approval 既定を利用します。
[Policy](policy-and-capability-foundation.md) と
[設定](../reference/configuration.md) の優先順位を変更しません。

## 実行境界

Host 内で AWS CLI の認証情報 resolver を使い、SSO・credential-process 等を解決します。
認証情報は Host プロセスのメモリ内に留め、一度解決した同じ認証情報で
STS 識別を確認して S3 を実行します。承認待ちの後も account／principal を再確認します。
Physical Host コントローラーへ返すのは account ID・principal ARN・region・表示名・操作結果です。
account 名は unavailable または実 ID に紐付いた Host 管理者のラベルを表示し、
プロファイル名から推測しません。AWS 自体が証明した名前としては扱いません。

確認には env・account・principal・プロファイル・region・bucket ARN・prefix・API action と
IAM action を表示します。ListObjectsV2 の IAM action は
[s3:ListBucket](https://docs.aws.amazon.com/AmazonS3/latest/API/API_ListObjectsV2.html) です。
初期対象は commercial partition の、実行 account が所有する通常 bucket です。
各 page に ExpectedBucketOwner を設定します。ディレクトリ bucket、access-point alias、
別 partition、cross-account bucket は未対応です。任意の AWS コマンド実行や認証情報発行はしません。

送信前に SDK の署名範囲と HTTPS 接続先を照合し、region redirect と暗黙の HeadBucket を拒否します。
URL が同じでも署名 region が変わる場合を含みます。pagination は bucket・所有者・prefix を固定し、
同一 token の反復や上限超過を失敗にします。Host 内の実行には process-group の期限を付けます。
認証情報、SDK の生のエラー、subprocess 出力を監査ログに残しません。

Hacocoon の拒否は未実行です。AWS AccessDenied は AWS が拒否した実行失敗として区別し、
その他の AWS 障害も空の成功一覧にしません。
[ADR 0034](../adr/0034-aws-operation-authentication.md) を参照してください。

## 検証範囲

リポジトリ試験は入力・応答、保存ポリシーと失効、承認・監査、Host所有、実SDKの
署名・redirectを模擬資格情報と置換HTTPで確認します。
`HACO_AWS_TEST_PYTHON=/path/to/venv/bin/python bash tools/ci-local.sh aws`
がSDK試験の入口です。GHAのaws-plugin jobも専用環境を用意します。
導入済みゲストの拒否と20 MiBの模擬転送は確認済みです。
[検証証拠](../status/acceptance-evidence.ja.md#development)に範囲とスキップを保持します。
認証済みS3、SSO更新、実デスクトップの判断はこれらの試験から推測しません。

## オブジェクトを取得する

信頼された Host の Linux／WSL クライアントで実行します。

```sh
haco aws s3 cp s3://example-bucket/project/config.json ./config.json
```

env／プロファイル／region オプションは一覧と同じで、URL より前に置きます。
保存先はクライアントが動くマシンのローカルファイルです。取得には別の Policy 範囲が必要です。
API action は `GetObject`、IAM action は `s3:GetObject`、resource は完全な object ARN、
`prefix` の代わりに `key` attribute、description は
`Download current object at execution` です。その他の識別 attribute は変わりません。
一覧の許可を取得の許可へ広げません。実行時の現在の version を一回で取得し、
アーカイブ restore・upload・range による再構成・SSE-C key 入力は自動で行いません。

64 KiB frame で転送し、オブジェクト全体を一件の JSON へ詰め込む上限は設けません。
ContentLength、最終サイズ・SHA-256、通信の終了、実行成功、監査完了が一致して
初めて保存先へ反映します。hash は転送の整合性を検証するもので、ユーザー指定の
object version を保証するものではありません。バイナリと空ファイルを扱えます。
dot パス segment を含む key は、別 object への正規化を避けるため拒否します。

保存先の隣の非公開ディレクトリに仮置きし、確認後に既存通常のファイルを不可分なに
置き換えます。symlink 等は拒否し、結果はクライアントユーザー専用の権限で保存します。
親・仮置きディレクトリの rename でも公開先を変えません。公開前の失敗・キャンセルでは
以前のファイルを保持します。後始末対象の識別が変わった場合は失敗として報告し、
不明な内容を再帰削除しません。非公開権限を保証できないファイルシステムは安全側で拒否するとし、
native Windows ファイルシステムの受入は別に残します。

[ADR 0035](../adr/0035-streamed-aws-downloads.md)が取得と確定の境界を定義します。

## 承認画面のアカウント名

任意で、既存の信頼された Host の /root/.aws/config に表示名と対応する ID を設定できます。

```ini
[profile development]
region = ap-northeast-1
haco_account_id = 123456789012
haco_account_name = Development
```

既定プロファイルは [既定] を使い、既存の認証設定は保持します。
実行時は従来の --profile development を使います。名前は Host 管理者の表示ラベルで、
AWS から取得した正式名称ではありません。追加の IAM 照会権限は不要です。
実際の STS アカウント ID と principal は引き続き表示・照合します。

名前と ID は対で設定します。STS の ID と異なる場合は S3 操作前に拒否します。
実行時も設定を読み直すため、承認待ち中に名前が変わったら新しい要求が必要です。
名前も保存範囲に含まれ、変更後に古い名前の保存許可は一致しません。
account_name = unavailable に一致する明示ルールは、名前を追加するときに
通常の `haco config` から調整します。許可を自動で広げることはありません。

設定は Host ユーザー所有の通常のファイル、group/other 書込不可、1 MiB 以下が条件で、
symlink は拒否します。不正・重複 INI、制御／書式文字、256 UTF-8 バイト列を超える名前は
拒否します。両フィールドがないプロファイルは unavailable を表示します。
Environment や repo からラベルを指定することはできません。

## guest 要求の境界

Standard listener に AWS 専用の origin-form 入口 /_haco/operations/aws を追加しました。
受け付けるのは list/get・URL・プロファイル・region のみで、Environment ID や管理・承認決定
メソッドは受け付けません。実行基盤の送信元証拠と保存済みの正確な Environment 作成 ID で
呼出元を特定し、再作成を認証前に拒否して、同じ ID を通常の承認・実行まで保持します。
転送ヘッダーから送信元を選ぶことはありません。

ゲストの通常haco クライアントは接続済みです。導入済みの拒否確認と、認証を伴う成功確認は別です。
最終処理記録の検証前に取得データを保存完了として扱ってはいけません。
[ADR 0036](../adr/0036-guest-aws-source-identity.md)を参照してください。


## Environment 内で AWS を使う

Standard の Environment 作成・起動時に、検証済み guest companion から通常の haco 入口を
用意します。Environment 内でも同じコマンドを使います。

```sh
haco aws s3 ls s3://example-bucket/project/
haco aws s3 cp s3://example-bucket/project/config.json ./config.json
```

呼出元 Environment は自動選択し、内部では --env を拒否します。プロファイルと region は
従来どおり信頼された Host 側の指定です。承認は信頼された Host／デスクトップから行います。
guest に認証情報や管理ソケットは渡しません。

管理された guest 実行ファイルは固定の隔離付き入口を自動選択します。この実行パスは
クライアントの接続先選択だけで、送信元の証明ではありません。server は実行基盤と保存状態で
引き続き本人性を確認します。HTTP proxy 環境変数や redirect は接続先を変更できません。
既存の無関係な /usr/local/bin/haco は上書きせず拒否します。

取得は非公開な仮置きと検証後の不可分な保存を使い、途中失敗では既存ファイルを保持します。
最終処理記録後の EOF、frame 上限、サイズ・hash、実行・監査の成功を確認します。
本文受信後は短い読み取り期限を解除し、人の承認待ちをそこで切断しません。
操作全体の 15 分制限は維持します。

実ソケット・通常の承認queue・保存・失効・監査を模擬送信元とAWSで検証します。導入済みゲストの拒否成功も、S3成功やSSO更新の証明ではありません。詳細は上記の検証証拠を参照してください。
