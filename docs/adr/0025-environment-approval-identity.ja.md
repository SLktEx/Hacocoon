# ADR 0025: 保存方針を Environment の作成識別子に結び付ける

状態: accepted。識別子と通常 Git CLI の保存は実装済み。外部受け入れは部分実装。

## 決定

再利用できる Environment 名を承認識別としません。正規の作成で Workspace
lease を予約する前に、ランダムな 128-bit の `env-...` instance ID を生成します。
プロバイダー作成前に lease に永続化し、実行基盤記録・ready commit で維持します。
ライフサイクル transition は識別子の差し替えを拒否します。

catalog は、渡された ready Environment の正確なスナップショットに対して識別子を返します。
catalog lock 内でメタデータと稼働中 lease を確認します。識別子のない既存 aggregate
に限り、移行と明示した処理でランダム ID を一度だけ付与・保存します。
不在・古いスナップショット・不整合は拒否し、プロバイダー resource は再作成・変更しません。

Capability 要求と監査は、表示名とは別に `environment_instance` を持てます。
Environment 単位の保存方針は同じ instance のみ照合し、古い名前だけの保存方針を
識別済み要求へ継承しません。明示的な全 env 方針では instance 条件を外します。
管理者ルールの既存の名前範囲は維持し、instance を明示して制約することもできます。

実運用の Git broker は信頼された catalog から ID を取得し、準備済み操作の実行直前にも
照合します。一般クライアントの要求 payload から ID を指定できません。承認 payload では
信頼されたな ID を表示用に保持します。実運用の Capability サービスも、名前付き要求の ID を Policy 評価前に catalog から取得し、プロバイダー実行直前に同じスナップショットを再確認します。env 単位の保存には有効な作成 ID が必須です。識別できない要求では今回だけの回答と明示的な全 env 方針のみを提示します。

## 却下した方式と範囲

- 現在の lease Owner は名前ラベルであり、作成ごとのランダムな ID ではありません。
- 名前・プロバイダー reference・時刻は再利用され得ます。
- ID を必須 CLI 引数にすると利用者の負担が増え、申告値を証拠と誤認します。
- 不明な既存識別を古い名前だけの保存許可から推測しません。

ID は認証情報やプロバイダー所有権 token ではなく、それだけで操作を許可しません。
Git commit/ref の照合、Policy、監査、正規の後始末は引き続き必要です。
再利用する Git 許可範囲と通常 CLI の保存は [ADR 0026](0026-reusable-git-approval-scope.ja.md) で実装しました。通知と実 network/provider の受け入れ確認は未完了です。

識別解決では保存済み lease を直接確認します。一般の旧実装 reader が補完した不在 lease を、ID 付与の証拠として採用しません。
