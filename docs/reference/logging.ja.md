# ログ

[English](logging.md) | 日本語

HacocoonはCore、プロバイダー、ネットワーク、ストレージ、プラグイン、CI/E2Eを通して障害を追跡する
構造化ログを使います。資格情報や信頼境界を弱めず、失敗した操作と責任範囲を特定するためのものです。

## 原則とレベル

意味のある操作・状態遷移を記録し、実装の逐語的な実況は避けます。
検索する値は文章に埋め込まず構造化項目へ置きます。
通常の障害はDEBUGなしでも判断できるようにし、DEBUGにも同じ秘密情報の制約を適用します。
ログの成否で本来の操作結果を変えてはいけません。

標準の`log/slog`を使い、既定レベルは`INFO`です。

| レベル | 用途 |
|---|---|
| DEBUG | 機密情報を除いたHostコマンド、再試行、内部状態などの詳細診断 |
| INFO | Environment作成・実行・削除の開始と完了など |
| WARN | 要求を継続できるが動作が劣化した場合や代替処理 |
| ERROR | 要求を正常完了できない場合 |

失敗は通常、その操作を報告する境界でERRORを一度記録します。
下位層はエラーを返すか付加情報で包み、必要な詳細をDEBUGへ出します。

## 実行ファイルの設定

```bash
HACO_LOG_LEVEL=debug haco doctor
HACO_LOG_FORMAT=json HACO_LOG_LEVEL=debug haco env create --workspace /work demo
```

`haco`、`haco-vscode`、`haco-wsl`、`haco-agent-host`、`haco-notify`は共通設定を使います。
形式は`text`（既定）または`json`です。ログはstderr、コマンド結果はstdoutへ出します。
設定はそのプロセスに適用され、クライアントの環境変数で稼働中コントローラーのログ設定が変わるわけではありません。

## 安定したfield名

同じ意味にパッケージ固有の別名を増やさず、既存項目を使います。

| Field | 意味 |
|---|---|
| `component` | `core`、`incus`、`network`、`storage`、`git`、`oci`、`proxy`、`host`など |
| `operation` | `create_environment`などの操作名 |
| `environment_id` | HacocoonのEnv ID |
| `runtime_ref` | 安全で診断に役立つプロバイダー側実行基盤参照 |
| `backend` | 対象の区別に必要なprovider/backend |
| `duration_ms` | 操作の実経過時間 |
| `attempt` | 再試行・試行番号 |
| `request_id` | 要求・Capabilityの対応を追うID |
| `error` | 失敗報告の所有層で機密情報を除いたエラー |
| `exit_code` | 子プロセス・Envコマンドの終了値 |
| `target_host` / `target_port` | 正規化した通信先。完全なURL・パス・queryは出さない |

任意object、全ファイルシステム構造、無制限のプロバイダー出力より、安定したIDを優先します。

## loggerの所有と引渡し

実行ファイルの入口がプロセスのroot loggerを設定します。
内部パッケージごとに無関係なglobal loggerを作りません。
操作の属性は`context.Context`で引き渡し、下位層はそこからloggerを作り、
自身の`component`などを追加します。domain契約へlogger依存を入れずに追跡できます。

## 秘密情報

全レベルで、password・passphrase、access/refresh/bearer/approval/session token、
Git資格情報・helper出力、SSH秘密鍵、API key、cookie、
`Authorization`/`Proxy-Authorization`、proxy資格情報、
資格情報付きURL、秘密を含む環境変数・設定値を記録してはいけません。

HTTP header全体、プロセス環境全体、任意設定object、要求・応答body、
子プロセスの生stdout/stderrを便宜的に記録しません。
共有handlerの既知パターン秘匿は多層防御であり、任意の機密objectを渡す許可ではありません。
呼出し元も安全な項目だけを選び、不明な値は省きます。

## Hostコマンド・エラー・時間

共有runnerは診断に必要な場合だけ、実行ファイルと安全化したargv、
分類した構成要素、時間、終了値をDEBUGへ記録します。取得したstdout/stderrは自動記録しません。
秘密を含み得る引数は省略・秘匿し、生のコマンド行を重ねて記録しません。

provider/Host層はエラーと任意のDEBUG詳細を返し、操作の所有層がERRORを一度記録し、
CLIは返されたエラーを表示します。再試行・代替処理で扱ったエラーは自動的にERRORではありません。
動作が意味上変わる代替処理はWARNが適切です。不正なバックエンド文言がエラー値に入る場合もあり、
そもそも秘密情報を含むエラーを組み立てないことを優先します。

Env・Incusのライフサイクル、イメージ取得・Seed作成、network/storage初期化、
Git fetch/push、後始末・復旧など、遅延が診断に役立つ操作で`duration_ms`を記録します。
小さなメモリ内処理へ大量の時間ログは追加しません。

## CIと変更時の確認

CIではrunner準備、Incus基盤、プロバイダー統合、Core ライフサイクル、network/proxy/DNS、
storage/pluginの失敗を区別できるようにします。試験は人間向け文章形式へ依存せず、
必要な個別診断artifactも保持します。CIのDEBUGでも秘密情報の規則は変えません。

ログ追加時は、運用上の必要性、レベル、構造化可能な値、重複ERROR、
秘密・任意出力の混入、項目の安定性を確認します。
新しい秘匿ルール、項目契約、形式、失敗報告境界には対象を絞った回帰試験を追加します。

## 監査の追加field

`environment_instance`は再利用可能な表示名と別に、正規のEnv作成を識別するランダムな公開IDです。
資格情報やプロバイダー所有tokenではなく、ポリシーと実行の対応を追うために保持します。

`policy-saved`の`saved_scope`は今回の厳密な属性と分け、検証済みのポリシー上の権限と
プロバイダーが明示したwildcardだけを含みます。資格情報、pack、内部を解釈しないパラメーターは含みません。

設定変更は変更前の`configuration-change-requested`と永続化後の`configuration-changed`を
`policy.configuration`へ記録します。操作IDと`previous_revision`/`revision`のhashだけを含み、
ルール全体・resource値・editor内容は出しません。完了監査失敗時は成功処理記録を返しません。

プロジェクト設定失敗の診断は許可リスト内の`stage`/`error_code`と数値`exit_code`だけです。
lookup/recipe/start/execute/scriptを区別し、未知値は`unknown`/`internal`にします。
バックエンドの生エラー、recipe本文、プロセス出力を診断項目へコピーしません。

## 日常操作の診断

Host setupは固定`stage`、`state`、`reason`と`request_id`、`duration_ms`を記録します。単一のERRORはcontrollerが所有し、個別工程はINFOの観測です。進捗はstderrへ出し、peer由来の任意フィールドを拒否します。journalの保持はsystemd-journaldが担います。helper終了値42はnative WSL binfmt不一致の固定コードであり、stderrの文字列解析には依存しません。[setup診断](../design/trusted-host.ja.md#setupの進捗と失敗診断)を参照してください。

Hostのユーザー設定のstdout/stderrは上限付き非公開結果として保存し、明示的な
`haco setup --script-result`だけで表示します。生の出力や結果objectを構造化log、
progress stage、auditに記録しません。

Windows review実行ファイルはnative review利用不能時の単一`notification_review` ERRORを
所有します。`stage`はregistration／session_plan／ownership／activation／clear／peer_start／
review／events／unknown、`reason`はunavailable／timeout／canceledに限定します。
検証済み表示結果では`native_stage`（runtime／xml／create／identity／show／history）と数値の
`native_error`（HRESULT）も記録できます。生エラー、controller応答、XML、page token、path、
子プロセス出力は含めません。導入済みprobeの失敗時も、これらの固定分類と終了値・期待文一致の
真偽値だけを表示します。
