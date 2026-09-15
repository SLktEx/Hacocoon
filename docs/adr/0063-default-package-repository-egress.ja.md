# ADR 0063: 公式パッケージリポジトリを限定的な外向き通信の基準許可にする

Status: accepted
Date: 2026-09-13

## Context

通常の Environment は外向き通信をデフォルト拒否します。この安全条件は維持しますが、
公式 Ubuntu 開発 Base で `apt update` や通常のパッケージ導入を行うだけでも、製品 Base
が最初から選んでいるリポジトリへ接続するための Policy 編集が必要でした。SSH の初回利用で
`openssh-server` を実行時に導入した経路でも同じ問題が表面化しました。

`apt` という実行ファイル自体へ権限を与えるのは広すぎます。一方、guest の
`/etc/apt/sources*` から許可先を導出すると、信頼しない workload がそのファイルを書き換えて
通信権限を増やせます。製品の既定値を各管理者の `policy.json` に書き込む方式も、製品側の
権限と revision で保護された管理者設定を混在させ、upgrade 時にユーザー所有 Policy の変更を
必要にします。

## Decision

Hacocoon は、公式 Base の契約で使う次の Ubuntu パッケージリポジトリだけを
製品所有の baseline allow とします。

- `archive.ubuntu.com` の HTTP 80 / HTTPS 443
- `security.ubuntu.com` の HTTP 80 / HTTPS 443
- `ports.ubuntu.com` の HTTP 80 / HTTPS 443

これらも通常の `network.egress/connect` grant です。Standard proxy、信頼された Host 側の
DNS 解決、公開アドレス検査、接続先固定を通ります。プロセス名、URL path、guest の
package manager 設定を権限の根拠にはしません。

評価順は次のとおりです。

1. 管理者ルールと保存済み判断を ADR 0023 の `deny > require-approval > allow` で評価
2. 上記の限定的な製品 baseline
3. Policy の default。未指定時は従来どおり `deny`

そのため管理者は、製品 baseline を変更せずに、対象ホスト・protocol・port・Environment に
一致する明示的な `deny` または `require-approval` でさらに制限できます。`haco config` は
revision で保護された管理者 Policy の表示・編集だけを担当し、製品 baseline は
`policy.json` に直列化しません。

baseline は選択した Base 名ではなく接続先に対して固定します。公式 Base から build / 派生
した custom Base が同じ Ubuntu 標準リポジトリを保持することがあり、固定された接続先だけを
許可しても別 Base を使ったことで権限は広がりません。third-party repository は一切追加されず、
実行時に APT source を書き換えても baseline は変化しません。

## Rejected alternatives

- `apt` が行う通信を無条件に信頼すると、source list の変更だけで任意ホストへ到達できます。
- guest の APT source を読み取り Policy を自動拡張すると、信頼しない workload が通信権限を
  自分で増やせます。
- baseline を `policy.json` へ注入すると、製品既定値と管理者所有の revision-bound 設定が
  混ざり、upgrade が複雑になります。
- 現在の公式 Base 名だけに許可を結び付けると、同じ Ubuntu 標準リポジトリを保持する build / 派生
  Base が壊れる一方、接続先の隔離は強くなりません。
- Ubuntu domain 全体や wildcard port の許可は package transport の契約を超えます。

## Consequences

新規導入した Environment では、通常の Ubuntu package update / install を Policy 編集なしで
実行できます。third-party repository、任意の web host、直接 TCP 通信は従来の deny / approval
経路のままです。明示的な管理者制限は baseline より優先します。acceptance では標準 package の
成功と、guest が third-party APT source を追加しても拒否されたままであることの両方を確認します。

Issue #603 は別責務として公式 Base に OpenSSH を事前導入し、SSH setup 自体を package network
から独立させられます。この ADR は一般的な package repository の外向き通信だけを定義します。
