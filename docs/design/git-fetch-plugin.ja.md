# v0.14 — Git Fetch Plugin

Status: **`main` に実装済み。**

v0.14 は、reusableなhost credentialをcoding Environmentへ渡さずにGitHub fetchを行うbrokered plugin機能です。

## CLI

```text
haco plugin git fetch <environment> [--remote <remote>]
```

## Authority boundary

- FetchはCoreのEnvironment lifecycleではなくGit/GitHub capability pluginとして扱う
- HTTPS認証はhost側の `gh auth git-credential` を使用
- credentialはtrusted Hostに残しEnvironmentへcopyしない
- privileged broker pathではglobal/system Git configを無効化
- repository-controlled credential helper、URL rewrite、transport hook、unsafe HTTP configを拒否
- repository-controlled `remote.<name>.fetch` を信用せず、validated GitHub remote + fixed branch refspecを使用
- tags/submodulesを暗黙にfetchしない

## v0.5との関係

v0.5はGit/GitHub capability boundaryとbrokered pushを導入しました。v0.14はそのpluginにcredential-safe fetchを追加する独立機能であり、GitをHacocoon Coreへ取り込みません。

## Acceptance

CLI parsing、trusted credential-helper injection、hostile Git configurationはrepository testでcoverageがあります。real private repository/provider combinationは別途acceptance対象です。
