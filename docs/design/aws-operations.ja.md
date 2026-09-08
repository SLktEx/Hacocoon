# 承認を経由する AWS 操作

[English](aws-operations.md) | 日本語

Status: **D3 の部分実装**。trusted Host からの S3 一覧取得とストリームによるファイル取得を実装しています。
guest 専用の要求経路、アカウント名の設定、実 AWS・デスクトップ受入は
planned です。deferred の EC2 runtime を再導入する変更ではありません。

## 普段の使い方

Host の任意の AWS 連携を準備した後、trusted Host で実行します。

```sh
haco aws s3 ls s3://example-bucket/project/
```

Environment が一つなら自動で選択します。複数ある場合は `--env dev` を指定します。
`--profile` は `default`、`--region` は Host のその profile 設定が既定です。
option は S3 URL より前に置きます。結果はキーとサイズの JSON 配列です。
Unicode は端末で安全に表示するため escape しますが、JSON を解釈したキーは変わりません。
大きすぎる一覧や途中までの取得は成功扱いにせず、prefix を絞るよう案内します。

承認が必要なら既存の通知・詳細画面、または別の trusted terminal の
`haco approve` を使います。環境限定／全環境の許可・拒否・毎回確認は、
同じ `haco config`、監査、request ID を使います。毎回確認の保存と今回の判断は別です。
画面がなくても自動許可しません。AWS 認証前の Environment 作成 identity を保持し、
capability service が再確認します。Environment に credential は渡しません。

## 任意の Host 準備と方針

Host の `/usr/local/bin/aws` に AWS CLI v2、`/usr/bin/python3` から import できる
botocore が必要です。通常の Hacocoon setup や Core の必須依存にはしません。
[AWS 公式インストール手順](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html)
と Host distribution の `python3-botocore` package を使います。
home が `/root` の trusted Host ユーザーで、たとえば
`aws configure sso --profile default` と `aws sso login --profile default`
によって認証します。SSO region に加え、通常の AWS region も profile に設定します。
ログインと個別操作の許可は別です。

既存 Policy は自動変更しません。default-deny なら対象操作の require-approval rule が
必要です。[英語版の設定例](aws-operations.md#optional-host-preparation)を、
対象 bucket・prefix・region に合わせて `haco config` に追加できます。
例の identity wildcard は毎回 review する入口で、保存される判断は実 identity に限定されます。
明示的な require-approval rule は保存済み allow より強いため、そのままなら毎回確認します。
保存した方針を使う意図なら、その明示 rule を削除・限定するか、
既存の require-approval default を利用します。
[Policy](policy-and-capability-foundation.md) と
[config](../reference/configuration.md) の優先順位を変更しません。

## 実行境界

Host 内で AWS CLI の credential resolver を使い、SSO・credential-process 等を解決します。
credential は Host プロセスのメモリ内に留め、一度解決した同じ credential で
STS identity を確認して S3 を実行します。承認待ちの後も account／principal を再確認します。
Physical Host controller へ返すのは account ID・principal ARN・region・表示名・操作結果です。
account 名は unavailable または実 ID に紐付いた Host 管理者のラベルを表示し、
profile 名から推測しません。AWS 自体が証明した名前としては扱いません。

review には env・account・principal・profile・region・bucket ARN・prefix・API action と
IAM action を表示します。ListObjectsV2 の IAM action は
[s3:ListBucket](https://docs.aws.amazon.com/AmazonS3/latest/API/API_ListObjectsV2.html) です。
初期対象は commercial partition の、実行 account が所有する通常 bucket です。
各 page に ExpectedBucketOwner を設定します。directory bucket、access-point alias、
別 partition、cross-account bucket は未対応です。任意の AWS コマンド実行や credential 発行はしません。

送信前に SDK の署名 scope と HTTPS 接続先を照合し、region redirect と暗黙の HeadBucket を拒否します。
URL が同じでも署名 region が変わる場合を含みます。pagination は bucket・owner・prefix を固定し、
同一 token の反復や上限超過を失敗にします。Host 内の実行には process-group の期限を付けます。
credential、SDK の生のエラー、subprocess 出力を監査ログに残しません。

Hacocoon の拒否は未実行です。AWS AccessDenied は AWS が拒否した実行失敗として区別し、
その他の AWS 障害も空の成功一覧にしません。
[ADR 0034](../adr/0034-aws-operation-authentication.md) を参照してください。

## 検証範囲

repository 検証では入力・応答の検証、共通 Policy 保存と撤回、controller review receipt、
Host 所有権、合成 credential と HTTP transport の置換による実 SDK の署名・redirect を扱います。
`HACO_AWS_TEST_PYTHON=/path/to/venv/bin/python bash tools/ci-local.sh aws` で SDK 検証を実行できます。
GHA の aws-plugin job でも専用 SDK 環境を用意します。

実 AWS、インストール済み Host の認証、SSO 更新、native／VS Code からの実 AWS 要求の判断は
未実行です。SDK の置換テストや既存 Git／network の確認から成功を推測しません。

専用 WSL Hacocoon-Review-6771f2f で現行 Host adapter・transient unit・未設定時の拒否が
動作しました。所有確認済み Host に AWS CLI・botocore・AWS config がないため、実 AWS は
前提不足で SKIP です。AWS 操作やログインは試行していません。認証済み AWS の受入とは別です。

## オブジェクトを取得する

trusted Host の Linux／WSL client で実行します。

```sh
haco aws s3 cp s3://example-bucket/project/config.json ./config.json
```

env／profile／region option は一覧と同じで、URL より前に置きます。
保存先は client が動くマシンのローカルファイルです。取得には別の Policy scope が必要です。
API action は `GetObject`、IAM action は `s3:GetObject`、resource は完全な object ARN、
`prefix` の代わりに `key` attribute、description は
`Download current object at execution` です。その他の identity attribute は変わりません。
一覧の許可を取得の許可へ広げません。実行時の current version を一回で取得し、
archive restore・upload・range による再構成・SSE-C key 入力は自動で行いません。

64 KiB frame で転送し、オブジェクト全体を一件の JSON へ詰め込む上限は設けません。
ContentLength、最終サイズ・SHA-256、transport の終了、実行成功、監査完了が一致して
初めて保存先へ反映します。hash は転送の整合性を検証するもので、ユーザー指定の
object version を保証するものではありません。バイナリと空ファイルを扱えます。
dot path segment を含む key は、別 object への正規化を避けるため拒否します。

保存先の隣の private directory に仮置きし、確認後に既存 regular file を atomic に
置き換えます。symlink 等は拒否し、結果は client ユーザー専用の権限で保存します。
親・仮置き directory の rename でも公開先を変えません。公開前の失敗・キャンセルでは
以前のファイルを保持します。cleanup 対象の identity が変わった場合は失敗として報告し、
不明な内容を再帰削除しません。private 権限を保証できない filesystem は fail-closed とし、
native Windows filesystem の受入は別に残します。

[ADR 0035](../adr/0035-streamed-aws-downloads.md) を参照してください。
実 controller wire の 20 MiB 転送と、通常 review・保存方針・撤回を repository 検証しています。
実 SDK の HTTP transport を置換した 11 テストは一覧と取得を扱います。
専用 Host に AWS CLI・botocore・AWS config がないため認証済み S3 取得は SKIP です。
これらのテストから実 AWS の成功を推測しません。

現在の所有確認付き Host streaming adapter は、専用 WSL でも AWS 通信なしで
20 MiB の転送に成功しました。保守対象の local CI、SDK 11 テスト、文書検査は成功です。
認証済み S3 の検証を代替するものではありません。


## 承認画面のアカウント名

任意で、既存の trusted Host の /root/.aws/config に表示名と対応する ID を設定できます。

```ini
[profile development]
region = ap-northeast-1
haco_account_id = 123456789012
haco_account_name = Development
```

既定 profile は [default] を使い、既存の認証設定は保持します。
実行時は従来の --profile development を使います。名前は Host 管理者の表示ラベルで、
AWS から取得した正式名称ではありません。追加の IAM 照会権限は不要です。
実際の STS アカウント ID と principal は引き続き表示・照合します。

名前と ID は対で設定します。STS の ID と異なる場合は S3 操作前に拒否します。
実行時も設定を読み直すため、承認待ち中に名前が変わったら新しい要求が必要です。
名前も保存範囲に含まれ、変更後に古い名前の保存許可は一致しません。
account_name = unavailable に一致する明示ルールは、名前を追加するときに
通常の haco config から調整します。許可を自動で広げることはありません。

config は Host ユーザー所有の regular file、group/other 書込不可、1 MiB 以下が条件で、
symlink は拒否します。不正・重複 INI、制御／書式文字、256 UTF-8 bytes を超える名前は
拒否します。両フィールドがない profile は unavailable を表示します。
Environment や repo からラベルを指定することはできません。

focused race と SDK/config の 15 テストが成功し、profile 分離・ID 不一致・名前変更・
危険な config の拒否を確認しました。認証済み AWS とデスクトップ上の表示確認は、
Host の AWS 前提不足により引き続き未検証です。

## guest 要求の境界

Standard listener に AWS 専用の origin-form 入口 /_haco/operations/aws を追加しました。
受け付けるのは list/get・URL・profile・region のみで、Environment ID や管理・承認決定
メソッドは受け付けません。runtime の送信元証拠と保存済みの正確な Environment 作成 ID で
呼出元を特定し、再作成を認証前に拒否して、同じ ID を通常の承認・実行まで保持します。
転送ヘッダーから送信元を選ぶことはありません。

これは server 側の実装です。guest の通常 haco client と installed guest 検証は未完了です。
最終 receipt の検証前に取得データを保存完了として扱ってはいけません。
[ADR 0036](../adr/0036-guest-aws-source-identity.md)を参照してください。
