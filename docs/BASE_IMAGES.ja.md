# Base Image Architecture — 日本語ガイド

Status: **v0.11 first slice は実装済み。この文書には今後の Base lifecycle 構想も含みます。** 現在の最小 contract は [`11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md`](11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md)、実装事実は [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md) を参照してください。

## 何をする機能か

Incus の alias / remote / fingerprint を Hacocoon Core の公開概念にせず、再利用できる Environment の starting point を **Base** として扱います。

```text
logical Base
   -> provider-owned source
   -> create 時に一度だけ resolve
   -> immutable Base revision
   -> Environment
```

既存 Environment は logical Base が後で動いても変わりません。

## 現在の実装

Core から見える identity は次です。

```text
BaseName
BaseRevision
BaseRef{Name, Revision}
```

現在の CLI:

```text
haco image list [--json]
haco image inspect <base> [--json]
haco create --base <base> --workspace <path> <environment>
```

Environment 作成時に logical Base を一度だけ解決し、immutable `BaseRef` を Environment metadata に保存します。

Incus adapter は configured source を `incus image info` で解決し、返された full fingerprint を検証して、その pinned fingerprint から `incus init` します。mutable alias は作成済み Environment の最終 identity にはなりません。

## Official / Custom Base

現在の Incus adapter には次の logical official Base があります。

```text
haco/ubuntu-26.04
haco/ubuntu-24.04
```

Host/operator の custom mapping は現在 `HACO_INCUS_BASES_JSON` で追加できます。

```json
{"my-dev":"images:my-moving-alias"}
```

`haco/` namespace は予約で、custom mapping から上書きできません。

この environment variable は Incus adapter の pre-1.0 configuration detail であり、固定 public schema ではありません。

## 一番大事なルール

```text
my-dev -> revision A -> Environment 1
my-dev -> revision B -> Environment 2
Environment 1 は revision A のまま
```

logical Base の変更は新規 Environment にだけ反映します。既存 Environment の Base を途中で差し替える操作はありません。

## Custom Base を信用しない

Base が決めてよいのは guest 側 filesystem/runtime contents までです。Base の選択だけで次の authority を増やしてはいけません。

- host filesystem mount
- Incus device
- privileged container
- Linux capability
- host network authority
- GitHub / AWS / cloud credentials
- SSH private key
- registry credential
- Hacocoon / Incus control-plane authority

```text
Base = guest contents
Policy / Capability = authority
```

現在の mapping parser は malformed logical name、control character、CLI option のような leading `-` source、reserved namespace override、過大設定、malformed fingerprint を reject します。

## Project Setup との分離

```text
Base
  common OS/runtime/tooling
      |
      v
Project Setup
  repo-specific setup
      |
      v
Environment
```

Project Setup の具体 schema はまだ固定しません。

## Build / Import は今後

Custom Base build/import は **first slice では未実装**です。

追加するときは build/import input を hostile data/code として扱い、archive traversal、unsafe symlink、malformed metadata、resource exhaustion、partial cleanup、credential capture を明示的に防ぎます。

推奨する将来形:

```text
Host
  |
  +-- Hacocoon / Incus authority
  |
  +-- isolated builder Environment
          |
          +-- build/import
          +-- immutable image
          +-- Base revision registration
```

Host credential は暗黙に注入しません。

## History / Rollback / Delete / GC も今後

これらも **first slice では未実装**です。

logical name の remove と physical revision deletion は分離します。

```text
logical name remove
    -> 新規選択不可
    -> referenced revision は保持
    -> 安全を証明できる未参照 revision だけ将来 GC
```

running/recoverable Environment が参照する revision は物理削除してはいけません。安全を証明できなければ、Environment を壊すより storage を残す方を選びます。

将来は `create vs update`、`create vs remove`、`gc vs create`、`build vs build` の race を意図的に扱う必要があります。

## Selection precedence

今の first slice は explicit `--base` と Hacocoon default を実装しています。project/user default を将来追加する場合は次の deterministic precedence を維持します。

```text
CLI --base
    > project configuration
    > user/global default
    > Hacocoon default
```

## Provider boundary

Incus は最初の/default provider ですが、Core は Incus alias / remote / fingerprint / publish/import mechanics を要求しません。

将来別 provider が増えても、同じ Base vocabulary を別の immutable starting-point mechanism に mapping できます。

## Acceptance

repository CI では list、inspect、explicit selection、alias -> fingerprint resolution、pinned init、persisted revision identity を unit/adversarial test と fake-Incus E2E で確認します。

real Incus image remote / custom image acceptance は host-dependent です。build/import/history/rollback/delete/GC は API 自体がまだないため今後の acceptance です。

> **Base は guest contents を選ぶ機能で、immutable revision が再現性の anchor です。Host authority を与える機能ではありません。**
