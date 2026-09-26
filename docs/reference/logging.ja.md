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

## プロキシ上流の診断

HTTP／CONNECT の境界が、上流失敗の `component=proxy`、`operation=egress_connect`
の ERROR を一度だけ記録します。フィールドは `environment_id`、`target_host`
（認可済みの正規化したホスト名）、`target_port`、`protocol`、`reason` です。
理由は `dns_lookup_failed`、`dns_empty_result`、`address_loopback`、
`address_disallowed`、`dial_failed`、`upstream_request_failed`、`canceled`、
`timeout` に限定します。包まれた失敗理由よりキャンセル・期限超過を優先します。
固定した集合内で次のアドレスへの接続に成功した場合、途中の失敗を操作の ERROR として
記録しません。DNS・接続処理の生のエラー、解決した IP、完全な URL、ヘッダー、本文は
どのログレベルにも含めません。

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

Windows転送の子プロセス失敗は、`operation=windows_tunnel_companion`、固定の
`stage`（pipe/start/prepare/wait）、`reason`（other/wait_delay/exit/signaled/timeout/canceled/pipe_closed）、
`context_state`（active/timeout/canceled）と`duration_ms`で一度だけ記録します。
生のエラー、実行ファイルパス、引数、子プロセスの出力は記録しません。
観測の追加で終了値・中断・子プロセスの停止期限は変えません。

共有runnerは診断に必要な場合だけ、実行ファイルと安全化したargv、
分類した構成要素、時間、終了値をDEBUGへ記録します。取得したstdout/stderrは自動記録しません。
秘密を含み得る引数は省略・秘匿し、生のコマンド行を重ねて記録しません。

provider/Host層はエラーと任意のDEBUG詳細を返し、操作の所有層がERRORを一度記録し、
CLIは返されたエラーを表示します。再試行・代替処理で扱ったエラーは自動的にERRORではありません。
動作が意味上変わる代替処理はWARNが適切です。不正なバックエンド文言がエラー値に入る場合もあり、
そもそも秘密情報を含むエラーを組み立てないことを優先します。

Env・Incusのライフサイクル、イメージ取得・Baseビルド、network/storage初期化、
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

native通知描画の失敗は数値`exit_code`と`duration_ms`も記録します。終了値-1は、子の起動前失敗など、
移植可能な終了コードが得られない場合です。contextのキャンセル／期限切れを子出力より優先します。
単一ERRORの所有者は同じreview境界のままで、生のprocessエラー・stdout・stderrはlogへ出しません。

描画失敗では、最大512バイトから確認した最後の固定段階を`native_progress`へ記録します。
値はruntime／input／decode／winrt／xml／create／identity／show／history／completeです。
未取得、途中の行、上限超過、その他の出力はunobservedになります。この観測は表示・実行・
承認の成功を示さず、期限切れやキャンセルの結果も変更しません。


COM起動の失敗は固定の`activation_stage`（initialize/register/create/dispatch）と数値`activation_error`（符号付きHRESULT）を記録します。読み取り専用の起動での期限・中止をCOM応答と非公開peer終了後も区別し、生のnativeエラーや要求内容は記録しません。

通知準備の失敗は既存のsetup境界で固定のサービス操作理由（notification_enable_state_failed、notification_activity_failed、notification_disable_failed、notification_reload_failed、notification_failure_state_failed、notification_reset_failed、notification_enable_failed、notification_restart_failed）を保持する。非公開helperの終了値50〜57は通知refreshだけで解釈し、生の出力は転送しない。中止を優先する。

Git照合は共通の同期済みCapability監査を使う。固定git-push-started/confirmed/observed記録へ要求・Env作成・取得元所有権・登録済みremote・ref・旧新OIDを保存し、照合は読み取り要求を別に記録する。これは監査上の事実であり、認証情報・subprocessの生出力・Git設定全体をログへ出さない。

キャッシュ管理は領域別の結果と診断を分けます。失敗を報告する境界ではcomponentを`cache`とし、固定した操作名と分類済み失敗コードだけを記録します。設定文書、パス、基盤の応答、生のエラーはログへ出しません。

キャッシュ保守も共通の `component=cache` 境界を使い、固定操作名 `cache.history` / `cache.clear` と固定 `failure_code` を記録する。確認revision・providerの保存場所・生のエラーはログに出さない。

名前付き収集の復旧は既存キャッシュ失敗境界と固定 operation=cache.recover を使う。所有記録の内容やproviderの生のエラーはログへ出さない。

Windowsの容量回収の準備・起動失敗は、既存の補助プログラムのエラー境界で、
固定の `phase`、`stage` と数値の `native_error` を記録する。失敗したコマンドの
標準出力には許可された段階と番号だけを返し、生のエラー文を含めない。

導入済みWindowsの容量回収CIは、WSL起動プロセスの`origins`と、起動元が終了した後も
残るWSL hostプロセスの`host_origins`を上限付きで記録する。同じ固定した親分類を使い、
親の欠落・ID再利用の疑いは未確認とする。生の名前・パス・PID・引数は含めない。
Windows全体の時系列観測であり、停止や圧縮を許可する証拠にはしない。

観測間に終了する短命な起動元は、別の上限付きWindowsプロセス起動通知でも記録する。
`reclamation_windows_start`は固定`state`・`kind`・親の`chain`・経過`duration_ms`だけを出し、
イベントのUTC生成時刻より新しい親はID再利用として拒否する。700秒・128イベントを上限とし、
未観測・切り詰めを明示する。生のWMI項目やエラーは破棄し、WSLへの接続・workerの結果変更・
workerの終了は行わない。終了するのは自身の観測プロセスだけとする。
