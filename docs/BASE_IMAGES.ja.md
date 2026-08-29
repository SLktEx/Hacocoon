# Base Image Architecture — 日本語ガイド

Status: **v0.11 Base Images & Custom Environments の詳細設計 companion。v0.11 の最小 contract は roadmap commitment、ここに書かれた追加アイデアのすべてが初回実装必須という意味ではありません。**

英語版 [`11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md`](11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md) が v0.11 の authoritative minimum contract、[`BASE_IMAGES.md`](BASE_IMAGES.md) が詳細設計の英語版です。全体アーキテクチャと用語については `00_REBASELINE_AND_ROADMAP.md`、`00C_TERMINOLOGY_AND_BOUNDARIES.md`、`00B_SECURITY_ARCHITECTURE.md` が優先されます。

## 何をしたいか

Incus で Environment を作るとき、毎回固定の image だけを使うのではなく、Hacocoon 標準の starting point やユーザーが用意した custom image を選べるようにします。

ただし、Incus の alias / remote / fingerprint を Hacocoon Core の公開概念にはしません。Hacocoon から見えるのは **Base** です。

```text
Official Base
    |
    v
User Base（任意）
    |
    v
Project Setup
    |
    v
Environment
```

- **Official Base**: Hacocoon が推奨・管理する標準 starting point。
- **User Base**: ユーザーが再利用する custom starting point。
- **Project Setup**: Workspace / repository 固有のセットアップ。
- **Environment**: 1つの immutable Base revision から作られる隔離実行環境。

## 一番大事なルール

論理名と実体を分けます。

```text
my-dev
  |
  +-- revision A
  +-- revision B <- current
```

ユーザーは通常 `my-dev` を指定します。

```text
haco create --base my-dev --workspace . dev-1
```

Environment 作成時に Hacocoon が `my-dev` を immutable revision に一度だけ解決し、その revision を Environment metadata に記録します。

Incus backend では、その revision が最終的に Incus image fingerprint に対応します。Incus alias は変更可能なので、作成済み Environment の最終 identity として使いません。

## Base を更新したとき

既存 Environment は変えません。

```text
Environment A -> revision A

my-dev を revision B に更新

Environment A -> revision A のまま
Environment B -> revision B
```

Base の変更は **次に作る Environment から反映**します。起動済み・既存 Environment の root starting point を途中で差し替える操作は最初の scope に入れません。

## Project Setup との分離

Base には複数 Environment / project で共有したい OS・runtime・共通 tooling を置きます。Repository 固有の依存関係やセットアップは Project Setup に寄せます。

```text
Base
  common tools/runtime
      |
      v
Project Setup
  repo-specific setup
      |
      v
Environment
```

## 想定 CLI

以下は v0.11 の interaction model ですが、**現時点では未実装で、pre-1.0 の間は command/config の形を変更できます。**

```text
haco image list
haco image inspect <name>
haco image build <name>
haco image remove <name>
haco create --base <name> --workspace <path> <environment>
```

最初の v0.11 implementation gate では `list` / `inspect` / `create --base` と immutable revision pinning を優先し、`build` / `remove` / GC 等は reference/lifecycle semantics が安全になってから追加します。

Base の選択優先順位は次の形を想定します。

```text
CLI --base
    > project configuration
    > user/global default
    > Hacocoon default
```

## Custom Base を信用しない

Custom image は悪意がある前提で扱います。Base が決めてよいのは guest 側の filesystem / runtime contents までです。

Base を変更・選択しただけで、次の権限が増えてはいけません。

- host filesystem mount
- Incus device
- privileged container
- Linux capability
- host network access
- GitHub / AWS / cloud credentials
- SSH private key
- registry credential
- Hacocoon control authority

```text
Base = guest contents
Policy = authority
```

という分離を維持します。

## Build / Import の安全性

Custom Base の build step は任意コード実行になり得るため、host 上で直接実行しません。

```text
Host
  |
  +-- isolated builder Environment
          |
          +-- build steps
          +-- image creation
          +-- immutable Base revision registration
```

ローカル image archive の import を実装する場合も、archive traversal、symlink、異常な metadata、巨大入力、partial import などを敵対的入力として扱います。

Import は「信用する」という意味ではありません。

## Credentials

GitHub token、SSH private key、AWS credential などの reusable secret を Base に焼き込まない設計にします。

将来「既存 Environment を snapshot して Base にする」機能を考える場合も、runtime secret や cache、shell history 等を一緒に保存する危険があるため、単純 snapshot は最初の scope に含めません。

## 削除と GC

論理名の削除と、実体 revision の物理削除は分けます。

```text
my-dev を remove
    |
    v
新規選択できなくなる
    |
    v
既存 Environment が参照する revision は残す
    |
    v
未参照 revision だけ GC 候補
```

参照関係が怪しい場合は、Environment を壊すより disk space が残る方を選びます。

## 並行実行

複数 agent / client が同時に操作する前提にします。

```text
build vs build
remove vs create
gc vs create
update vs create
```

Environment 作成では Base の immutable revision を確保・記録してから、その revision が GC 可能になるような設計にします。

## Incus との境界

Core 側は概念的に次のような型を持てます。

```go
type BaseName string
type BaseRevision string
```

Incus adapter の中だけで fingerprint に変換します。Core に `IncusImageAlias` のような Incus 固有概念を持ち込まないことが重要です。

## 現在の roadmap との関係

- **v0.9**: 実装済み Per-Agent Sandbox broker。
- **v0.10**: PR #111 の VS Code Remote Agent Host Adapter integration。
- **v0.11**: この Base Images & Custom Environments design gate。
- **v0.12**: Resource Limits。Base metadata から host-selected resource limit を上げることはできない。

v0.11 Base 実装は通常の Environment lifecycle を使い、v0.9/v0.10 の ownership/security boundary を迂回しません。

## 最初の v0.11 implementation slice

最初は欲張らず、次までを狙います。

1. Hacocoon Base 名を既存 Incus image に解決する。
2. `haco create --base <name>` を追加する。
3. 解決済み immutable revision / fingerprint の関係を保存する。
4. list / inspect を最低限提供する。
5. default Base の解決順序を固定する。
6. pinning、削除安全性、競合の test を追加する。

その後に custom build/import、history、rollback、GC を追加します。

## まとめ

> Hacocoon Environment は 1 つの immutable Base revision から作成される。論理 Base を更新しても既存 Environment は変更しない。

> Base が定義するのは guest 側の starting contents であり、host 権限・credential・device・mount・external authority ではない。
