# ADR 0025: 保存方針を Environment の作成識別子に結び付ける

状態: accepted。識別子と Git 要求への結び付けは implemented。通常の Git 保存承認は partial。

## 決定

再利用できる Environment 名を承認 identity としません。canonical な作成で Workspace
lease を予約する前に、ランダムな 128-bit の `env-...` instance ID を生成します。
provider 作成前に lease に永続化し、runtime 記録・ready commit で維持します。
lifecycle transition は識別子の差し替えを拒否します。

catalog は、渡された ready Environment の正確な snapshot に対して識別子を返します。
catalog lock 内で metadata と active lease を確認します。識別子のない既存 aggregate
に限り、migration と明示した処理でランダム ID を一度だけ付与・保存します。
不在・古い snapshot・不整合は拒否し、provider resource は再作成・変更しません。

Capability 要求と監査は、表示名とは別に `environment_instance` を持てます。
Environment 単位の保存方針は同じ instance のみ照合し、古い名前だけの保存方針を
識別済み要求へ継承しません。明示的な全 env 方針では instance 条件を外します。
管理者ルールの既存の名前範囲は維持し、instance を明示して制約することもできます。

実運用の Git broker は trusted catalog から ID を取得し、準備済み操作の実行直前にも
照合します。一般 client の要求 payload から ID を指定できません。承認 payload では
trusted な ID を表示用に保持します。他 provider も env 単位の保存を提供する前に、
trusted な送信元 binding から識別子を取得する必要があります。

## 却下した方式と範囲

- 現在の lease Owner は名前ラベルであり、作成ごとのランダムな ID ではありません。
- 名前・provider reference・時刻は再利用され得ます。
- ID を必須 CLI 引数にすると利用者の負担が増え、申告値を証拠と誤認します。
- 不明な既存 identity を古い名前だけの保存許可から推測しません。

ID は credential や provider 所有権 token ではなく、それだけで操作を許可しません。
Git commit/ref の照合、Policy、監査、canonical cleanup は引き続き必要です。
再利用する Git 許可範囲・通常の保存選択 UI・network の identity 統合は未完了です。

identity 解決では保存済み lease を直接確認します。一般の legacy reader が補完した不在 lease を、ID 付与の証拠として採用しません。
