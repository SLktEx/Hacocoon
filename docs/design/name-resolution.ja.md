# Environment の名前解決

日本語 | [English](name-resolution.md)

状態: **部分実装**。Policy に従う名前解決、コントローラーの HTTP 経路、UDP／TCP 中継、作成・再開時の自動設定を実装しています。導入済み Windows の通常の名前解決と既定拒否、設定を繰り返した場合のサービス起動は確認済みです。VPN・設定変更・再起動による反映は未確認です。

## 通常の利用方法

Environment の作成・再開時に自動設定します。ネームサーバーの指定や追加コマンドは不要で、通常の `getaddrinfo`／`getent` を利用できます。判断理由は [ADR 0021](../adr/0021-policy-bound-name-resolution.ja.md)に記載します。

Environment と正規化したホスト名に対する `network.resolve`／`lookup` の Policy 判断が必要です。名前解決の許可は HTTP、HTTPS、私設ネットワークへの接続許可ではありません。既存の許可・拒否・承認と監査を使い、許可ルールや設定 UI を勝手に追加しません。拒否した名前を基盤のリゾルバーへ渡しません。

## リゾルバーの所有者

Standard は Physical Host のリゾルバーを使います。対応する Windows／WSL では Windows DNS トンネリングを上流とするため、WSL の resolv.conf 自動生成を有効に保ちます。公開 DNS サーバーへの代替経路は設けません。[Microsoft の説明](https://learn.microsoft.com/en-us/windows/wsl/troubleshooting#networking-considerations-with-dns-tunneling)を参照してください。

信頼しないゲスト側の中継はループバックのポート53と既存の固定プロキシ接続先を使います。Host の認証情報や管理権限は渡しません。保護された接続元の識別から所有権を確認します。上限付きの IN A／AAAA 要求だけを扱い、応答をキャッシュしません。信頼された Host 自体の基盤 DNS は別の検証対象です。

## 変更反映と残る検証

Windows の DNS 変更、VPN 接続・切断、WSL 再起動は、同じ公開名・利用可能な VPN 内の名前を Windows、Physical Host、信頼された Host、Environment で比較して確認します。OS・アプリのキャッシュと、キャッシュしない中継の動作を区別します。リポジトリ内の試験だけでは各 VPN の反映時刻を証明できません。

Policy・監査拒否、送信元ヘッダー偽装、接続許可を伴わない私設アドレス、不正メッセージ、UDP／TCP、キャンセルを検査します。実際の名前解決の一致と既定拒否は `c05528a` の Windows 実行34132173483で成功しました。VPN／NRPT と OS 再起動の検証条件がなければ SKIP とします。接続の許可・拒否は別に確認し、プロキシ内部の名前解決だけをゲストの成功としません。

## 繰り返す設定とサービス起動

サービス設定を変更する前にゲストの systemd 管理機構を最大30秒待ちます。再読み込み自体の失敗は再試行しません。検証済みの補助バイナリ・ユニットが変わった場合は再起動し、同じなら systemd 起動を使います。稼働中のサービスを継続し、停止中なら起動します。

準備待ち、再読み込み、有効化、稼働・リゾルバー設定の確認、所有権、接続元識別、Policy の検査は維持します。導入済み環境での連続設定は `226991b` で成功しました。以前の準備待ちや起動回数制限の失敗は[検証証拠](../status/acceptance-evidence.ja.md#development)に残します。この成功を VPN／NRPT や再起動時の DNS 反映の証明とは扱いません。
