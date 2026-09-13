# ホスト名に基づく外向き通信の認可

状態: **実装済み**。導入済み Windows で、プロキシ経由の許可・拒否と直接通信の拒否を確認しています。

許可対象は Environment が要求したホスト名です。一度解決した IP アドレスだけを許可すると、共有 CDN、DNS の変更、IP 直接指定によって別の接続先へ権限が広がるため、その方式は採用しません。

## 境界

要求と認可の契約は Core、HTTP／HTTPS の制御プロキシは Standard、Incus のネットワーク構成と送信元識別は Incus アダプターが担当します。

```text
Environment
  -> 専用ブリッジと Host 通信保護
  -> 固定プロキシ 169.254.254.1:18080
  -> 接続元から Environment を照合
  -> network.egress/connect
  -> Policy・承認・監査
  -> Host の DNS 解決と公開アドレスの固定
  -> 許可した上流
```

許可は一つの Environment、正規化したホスト名、プロトコル、ポート、一回の接続試行に限定します。プロキシは承認トークンを発行せず、承認を IP 許可リストとして保存しません。

終了時はクライアント側と CONNECT 上流の両方を同期的に閉じます。停止後に完了した接続は、最初の上流書き込み前に拒否します。非同期のキャンセル通知だけを停止完了とはみなしません。ClientHello 待ち、先頭データの書き込み、確立済みトンネル、停止後の上流登録を回帰試験で扱います。

## 認可と通信の検査

- `internal/core` が `EgressRequest`／`EgressGrant`、`internal/egress` がホスト名の正規化と `network.egress/connect` の仲介を担当します。
- IP アドレスの直接指定は Policy 評価前に拒否します。
- `modules/standard/egressproxy` が明示的な HTTP／HTTPS プロキシを実装します。HTTP の絶対 URI と `Host` は同じホスト名・ポートを指す必要があります。
- 認可後に Host 側で DNS 解決し、その接続にアドレス集合を固定します。接続時に名前を再解決しません。
- 私設、ループバック、リンクローカル、CGNAT、ベンチマーク用、文書用、マルチキャストなどのアドレスを拒否します。公開・私設が混在した応答は全体を拒否します。
- HTTPS CONNECT の文字列だけを証拠にしません。上流へ TLS データを送る前に上限付きの ClientHello を解析し、SNI が許可した CONNECT ホスト名と一致することを要求します。
- プロバイダーや監査の失敗は、既存 Capability サービスを通して安全側で拒否します。

## Incus の通信境界

現在の Environment は所有権を確認した専用ブリッジを使います。NAT 無効、DHCP 有効、ブリッジ DNS 無効を要求し、Host の inet ルールと送信元保護処理でプロキシ経由に制限します。信頼された `haco-host` の NAT ブリッジは別の基盤経路です。プロキシ環境変数はこの下位の保護を弱めません。[ネットワーク構成](managed-sandbox-network.ja.md)を参照してください。

Environment の自己申告名は信頼せず、Incus の状態とコントローラーの永続記録から接続元を照合します。固定接続先だけで待ち受け、不在・不明確・管理対象外の識別を拒否します。再起動を越えて接続許可を保持しません。

保存された実行基盤の参照にはプロバイダーの経路も含みます。Environment のルーターを使って復号し、設定した接続元プロバイダーとその内部参照の両方を照合します。別プロバイダーの同じ内部参照に権限を与えません。

## Policy例

特定の HTTPS ホスト名を許可する Policy の例です。通常の `haco config` では既存の `revision` と他のルールを保持して、このルールを追加します。

```json
{
  "default": "deny",
  "rules": [
    {
      "capability": "network.egress",
      "action": "connect",
      "resource": "api.example.com",
      "environment": "env-a",
      "attributes": {"protocol": "https", "port": "443"},
      "decision": "allow",
      "reason": "approved development API"
    }
  ]
}
```

接続ごとの承認には `require-approval` を使います。Environment、ホスト名、プロトコル、ポートは監査する権限範囲に残します。

## 製品既定の package repository 許可

Environment 全体は引き続き default-deny ですが、公式 Base 契約で使う Ubuntu package
repository だけは、製品所有の限定的な egress baseline として次を許可します。

- `archive.ubuntu.com` の HTTP 80 / HTTPS 443
- `security.ubuntu.com` の HTTP 80 / HTTPS 443
- `ports.ubuntu.com` の HTTP 80 / HTTPS 443

これは `apt` プロセスへの特権ではなく、正確な `network.egress/connect` 接続先への許可です。
guest の `/etc/apt/sources*` を読んで一覧を増やさないため、PPA、third-party repository、
任意 mirror を追加しても通信権限は増えません。公式 Ubuntu Base から build した custom Base が
同じ標準 repository を保持している場合にも、この固定 baseline は利用できます。

一致する管理者ルールや保存済み判断を先に評価します。そのため既存の
`deny > require-approval > allow` が維持され、管理者は baseline の接続先でも明示的に拒否または
承認必須へできます。明示ルールがない場合だけ package baseline を評価し、その後に Policy の
default を使います。baseline は製品コードであり revision-bound な管理者設定ではないため、
`haco config` と `policy.json` には直列化しません。[ADR 0063](../adr/0063-default-package-repository-egress.ja.md)
を参照してください。

## 起動経路

インストールしたサービスは `haco-controller --standard-egress` を実行します。Incus 側の保護を検証してから、既存の Policy・監査・永続的な送信元照合を使う Standard プロキシを起動します。引数なしのコントローラーは独立した通信試験用に残りますが、通常のインストーラーは Standard を有効にします。`hacoq egress serve` は旧機能です。

コントローラーとプロキシの終了は連動し、CONNECT を含む全接続を閉じます。ヘッダー上限は16 KiB、読取期限は10秒、保持接続上限は256です。通信失敗は固定の構造化メッセージで記録し、任意の panic 出力を含めません。

デーモンは継承した標準入力を読みません。Policy がない場合も、上記の固定 package baseline 以外は
拒否します。承認が必要な要求は上限付きの待機列に置き、信頼された Host の `haco approve` で
確認します。保存と実行には Policy・監査・識別の確認を適用します。管理者 Policy の編集には
`haco config` を使い、package baseline の接続先をさらに制限する場合は一致する `deny` または
`require-approval` を追加します。[承認待ち](pending-approval-review.ja.md)と
[ADR 0028](../adr/0028-pending-approval-sessions.ja.md)を参照してください。

Git push は別の権限操作です。Host の再利用可能な Git 認証情報を Environment に渡して有効化しません。

## 検証範囲

Windows の導入手順が成功した後、同じ導入済みコントローラーで検証します。まず管理者 Policy を
一切作成する前に、通常ユーザーの管理 API client が使い捨て Environment を作成し、静かな
`apt-get update` と標準 package の再インストールを実行します。その後、一時的な APT source に
`example.com` を追加し、その host が Standard proxy で引き続き 403 になることを確認します。
これにより、固定 package repository は既定で利用でき、guest の source list 編集では baseline を
拡張できないことの両方を検証します。

別の packet 検証では読み取り専用 Workspace／Environment を作成し、固定 HTTPS 検査を実行して
正規の経路で削除します。第二のコントローラー、旧 CLI、製品設定の上書き、NAT・ファイアウォール・
マウント修復を使いません。検証用 Policy は対象 Environment の `github.com:443` だけを許可します。
既存 Policy は上書きせず、後始末は変更されていない自分の検証用設定だけを対象とします。

packet 検査は証明書確認付き HTTPS の成功、未許可ホスト名の403、Host から到達できる公開先への直接
TCP 拒否、管理ソケットの非公開を確認します。この検証は、別途検証する製品 CLI や設定 UI の証拠を
兼ねません。リポジトリ内では許可・拒否・承認、IP 直接指定、共有 IP、別ホスト名、混在 DNS、
SNI 不一致、旧ネットワーク移行、不正な DNS／ACL、送信元照合を検査します。baseline の回帰試験では
package host / protocol / port の正確な集合と、明示的な制限が baseline より優先することも固定します。
実際の Incus・nftables・dnsmasq の条件は[検証証拠](../status/acceptance-evidence.ja.md)で区別します。

## 通信元観測の責任者

implemented: Incus adapter は native runtime reference だけを返す。
未使用だった Environment 名の直接導出 helper は削除し、永続 state と照合する
resolver を本番の Environment identity 解決経路として維持する。
失敗・キャンセル・途中で切れた Incus 出力は、もっともらしい名前が 1 件あっても
通信元の証明にしない。正規化・Policy/Approval・Standard の具体的な接続処理は
既存の責任者が担当する。HTTP/HTTPS 対応と公開コマンドは変更しない。

## 通信制御実装の構成

実装済み: Standard proxy の構成／routing、HTTP 転送、CONNECT、authority 解析、
TLS ClientHello 解析、固定アドレスへの接続を責務別に整理しました。HTTP／CONNECT は
要求と完全一致する grant の受領と許可後の DNS 解決を共有し、Core の Policy／Approval を
制御実装へ複製しません。grant の対象が異なる場合は DNS より前に拒否します。
名前解決と接続の前後でキャンセルを確認し、遅れて返った接続は上流への書込み前に閉じます。
既存 HTTP 応答、hostname 正規化、SNI 検証、1回の試行に限定した grant を維持します。
[既存 ADR](../adr/0007-controller-owned-standard-egress.ja.md#通信開始前の共通検証)を参照してください。
