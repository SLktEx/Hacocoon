# GitHub Git capability plugin

このパッケージは `haco plugin git fetch` / `haco plugin git push` が利用する、オプションの GitHub 向け Git capability を実装します。

これは Core ドメインの一部ではなく adapter/plugin です。Core が持つのは汎用の capability 契約と policy 判定だけです。Git remote の解析、repository/branch の権限境界、承認後の変更検知、最終的な brokered `git fetch` / `git push` など Git/GitHub 固有の処理はこのパッケージが担当します。

この plugin は Sandbox に GitHub credential を渡しません。Host 側の brokered Git は global/system Git config を無効化したまま、`github.com` の HTTPS credential provider として `gh auth git-credential` だけを明示的に設定します。そのため普段 Host で `gh auth login` / `gh auth setup-git` を利用している場合も、PAT や credential helper 設定全体を Sandbox や repository に公開せず private repository を扱えます。

`fetch` は capability service が GitHub repository、remote、repository identity を評価した後にだけ実行されます。実行時は repository-controlled `remote.<name>.fetch` を使わず、検証済みの GitHub URL と固定 refspec を使って `refs/remotes/<remote>/*` だけを更新します。tag と submodule は自動取得しません。

CLI:

```bash
haco plugin git fetch <environment>
haco plugin git fetch <environment> --remote upstream
```

`default: deny` の policy では fetch を明示的に許可してください。たとえば `acme/demo` の `origin` を取得する場合:

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

`push` は従来どおり GitHub repository、target ref、source commit、force-push の意味を capability service が評価した後にだけ実行します。HTTPS remote では fetch と同じ Host の `gh` credential provider を使い、SSH remote の場合は従来どおり Host の default key / `SSH_AUTH_SOCK` を利用できます。

現時点の Hacocoon の plugin は通常の Go package 境界と静的 composition を使います。dynamic shared-object/plugin loader を導入するものではありません。CLI でも `haco plugin` namespace を使うことで extension boundary を明示しつつ、security-sensitive authority は Host 側の Capability 実装に残します。
