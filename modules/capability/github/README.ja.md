# GitHub Git capability plugin

このパッケージは `hacoq plugin git fetch` / `hacoq plugin git push` が利用する、オプションの GitHub 向け Git capability を実装します。

これは Core ドメインの一部ではなく adapter/plugin です。Core が持つのは汎用の capability 契約と方針判定だけです。Git remote の解析、repository/branch の権限境界、承認後の変更検知、最終的な brokered `git fetch` / `git push` など Git/GitHub 固有の処理はこのパッケージが担当します。

このプラグインは Sandbox に GitHub 認証情報を渡しません。Host 側の brokered Git は global/system Git 設定を無効化したまま、`github.com` の HTTPS 認証情報プロバイダーとして `gh auth git-credential` だけを明示的に設定します。そのため普段 Host で `gh auth login` / `gh auth setup-git` を利用している場合も、PAT や認証情報 helper 設定全体を Sandbox やリポジトリに公開せず非公開リポジトリを扱えます。

GitHub Actions などの headless Host では、代わりに Hacocoon Host プロセスへ `HACO_GITHUB_TOKEN` を渡せます。isolated Git runner が受け付ける token 入力はこの明示的な Hacocoon 用認証情報だけで、信頼された brokered Git プロセス内では `gh auth git-credential` が利用できるよう `GH_TOKEN` に変換します。暗黙に継承するな `GH_TOKEN` / `GITHUB_TOKEN` は従来どおり破棄します。token は Environment、Hacocoon 状態、方針要求、監査 record にはコピーしません。

`fetch` は capability サービスが GitHub リポジトリ、remote、リポジトリ識別を評価した後にだけ実行されます。実行時は repository-controlled `remote.<name>.fetch` を使わず、検証済みの GitHub URL と固定 refspec を使って `refs/remotes/<remote>/*` だけを更新します。tag と submodule は自動取得しません。

CLI:

```bash
hacoq plugin git fetch <environment>
hacoq plugin git fetch <environment> --remote upstream
```

`default: deny` の方針では fetch を明示的に許可してください。たとえば `acme/demo` の `origin` を取得する場合:

```json
{
  "capability": "github.git",
  "action": "fetch",
  "resource": "github://acme/demo/fetch/origin",
  "environment": "demo",
  "attributes": {
    "organization": "acme",
    "repository": "demo",
    "repository_identity": "*",
    "remote": "origin"
  },
  "decision": "allow"
}
```

`push` は従来どおり GitHub リポジトリ、対象 ref、元データ commit、force-push の意味を capability サービスが評価した後にだけ実行します。HTTPS remote では fetch と同じ Host の `gh` 認証情報プロバイダーを使い、SSH remote の場合は従来どおり Host の既定 key / `SSH_AUTH_SOCK` を利用できます。

現時点の Hacocoon のプラグインは通常の Go パッケージ境界と静的構成を使います。dynamic shared-object/plugin loader を導入するものではありません。CLI でも `hacoq plugin` 名前空間を使うことで extension 境界を明示しつつ、security-sensitive 権限は Host 側の Capability 実装に残します。
