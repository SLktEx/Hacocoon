# Hacocoon 日本語 Architecture Guide

この資料は、Hacocoon の architecture と v0.1〜v0.7 の流れを日本語でまとめた読み物です。

> [!NOTE]
> これは理解しやすさを優先した補助資料です。Exact contract や security-sensitive な判断では、対応する英語版 authoritative document を正本として参照してください。

## 一言でいうと

**Hacocoon は Secure Workspace Runtime です。**

人間、IDE、Shell、Coding Agent、外部 Orchestrator から Workspace を受け取り、隔離された Environment の中で処理を実行し、Host や外部サービスに対する特権操作を明示的な Policy / Capability boundary で制御します。

Hacocoon 自身は「全部入り開発プラットフォーム」にはしません。

```text
Client / IDE / Agent / Orchestrator
                |
             Workspace
                |
                v
        +------------------+
        |     Hacocoon     |
        |                  |
        | Environment      |
        | Execution        |
        | Policy           |
        | Approval         |
        | Capability       |
        | Audit            |
        +--------+---------+
                 |
         Provider / Adapter
                 |
       Incus / EC2 / GitHub / AWS ...
```

## Hacocoon が所有するもの

Hacocoon が責任を持つのは、主に次です。

- Workspace を Environment に安全に結びつけること。
- Environment lifecycle。
- Command execution / interactive access。
- Workspace の read/write ownership safety。
- Host authority を跨ぐ Policy / Capability / Approval。
- Privileged operation の audit。
- Client / Orchestrator が利用できる安定した概念境界。

## Hacocoon が所有しないもの

次は Hacocoon Core の責任にしません。

- VS Code など IDE の UX。
- Git branch strategy。
- Git worktree orchestration。
- Codex / Claude 等の model selection。
- Agent task DAG。
- Retry strategy。
- Model / token budget。
- Development review queue。
- Provider 固有の Cloud / Storage mechanics。

外部 Orchestrator が worktree を作るなら、その path を Hacocoon に渡せばよい、という考え方です。

```text
Orchestrator
  -> worktree / directory
  -> Workspace path
  -> Hacocoon
  -> isolated Environment
```

## Core を小さくする理由

Hacocoon は今後も Scrap & Build しやすい構造を重視します。

Core に Incus、AWS、GitHub、Btrfs、EC2、VS Code の事情を直接入れてしまうと、一つを交換するたびに全体を壊すことになります。

そのため Core の語彙は小さく保ちます。

```text
Workspace
WorkspaceLease
Environment
Execution
CapabilityRequest
PolicyDecision
ApprovalRequest
```

Concrete technology は Adapter / Provider 側へ押し出します。

重要なのは「何でも共通 Interface にする」ことではありません。

**本当に境界が必要になったところだけ分ける**方針です。Second implementation や testability が seam を必要とするまでは、speculative abstraction を増やしません。

## Workspace

Workspace は Hacocoon Core から見ると opaque な作業対象です。

それが次のどれであるかを Core は気にしません。

- 普通の directory
- Git repository
- Git worktree
- 外部 Orchestrator が作った workspace

重要なのは「Environment に渡す対象として何であるか」です。

## WorkspaceLease

同じ Workspace を複数 Environment から同時に書き換えると壊れやすいため、Hacocoon は WorkspaceLease を使います。

大きくは次の2種類です。

- RO: read-only
- RW: read-write

RW lease が存在する Workspace に、別の RW Environment を無警戒に重ねないようにします。

Stale lease や process crash 後の recovery も、この ownership safety の一部です。

## Environment

Environment は Workspace を実際に動かす隔離された実行場所です。

標準は local Incus provider です。

```text
Workspace
   |
   v
Environment
   |
   +-- runtime.incus   <- default
   |
   +-- runtime.ec2     <- experimental
```

Core は `if ec2` や `if remote` を増やすのではなく、同じ高位 Environment contract の後ろに Provider を置きます。

## Execution

Agent だから特別な Core lifecycle を作る、という設計にはしません。

Hacocoon から見ると Codex も Claude も普通の command execution です。

```bash
haco run --workspace ./repo -- codex
haco run --workspace ./repo -- claude
haco run --workspace ./repo -- go test ./...
```

Agent-specific orchestration は上位レイヤーが所有します。

## Security boundary

Hacocoon では Environment 内の workload を Host authority に対して untrusted と考えます。

「自分しか使わない」「localhost」「VM/container の中」「internal API」だから安全、とは考えません。

特に Host 側の次のような authority は Environment に便利だからという理由で渡しません。

- Host HOME
- `~/.ssh`
- `~/.aws`
- Broad GitHub token
- Incus control socket
- Hacocoon internal state

外部サービスへの特権操作は、可能な限り Host 側の narrow Capability として broker します。

## Policy / Capability / Approval

v0.4 以降の重要な考え方です。

```text
CapabilityRequest
       |
       v
PolicyEvaluator
       |
       +-- allow
       +-- deny
       +-- require-approval
                         |
                         v
                    Human approval
                         |
                         v
                 Capability provider
                         |
                         v
                       Audit
```

Policy は fail closed が基本です。

曖昧な入力や unknown field を都合よく allow にしません。

Human-in-the-loop も「Agent の仕事を人間がレビューする仕組み」と「Security authority を人間が承認する仕組み」を分けます。

```text
Development approval -> Human / GitHub / Orchestrator
Security approval    -> Hacocoon Policy / Capability
```

## Git / GitHub

Git worktree ownership と GitHub authority は別問題です。

Workspace がどこから来たかに関係なく、Protected な GitHub operation は Capability boundary を通せます。

現在の brokered push は Host credential を Environment にそのまま export せず、repo/ref/source SHA 等の authority を Host 側で確認して処理します。

## Client / VS Code

VS Code は重要な client ですが、Hacocoon Core の一部ではありません。

基本は標準 protocol を利用します。

- SSH
- Remote-SSH
- local port forwarding
- terminal
- code-server

VS Code Extension がなくても成立する path を優先し、Extension は必要になった場合の optional adapter として扱います。

## EC2

v0.7 で EC2 Environment provider が入りましたが、これは **experimental** です。

Default では無効です。

現在は両方が必要です。

```bash
export HACO_RUNTIME_PROVIDER=runtime.ec2
export HACO_EXPERIMENTAL_EC2=1
```

AWS credential が存在するだけでは有効化されません。

Remote path では S3 staging と SSM を利用し、Host の parent credential を EC2 workspace にコピーしません。

Real AWS acceptance は別途必要です。

## EBS

EBS は in-place shrink できないため、「容量を減らす」を Core の generic shrink operation として扱いません。

必要な場合は adapter-owned replacement transaction として扱います。

概念的には次のようになります。

```text
source volume
   |
create smaller target
   |
migrate data
   |
verify
   |
switch over
   |
keep source for recovery
```

Source volume は自動削除しません。

Failure point によっては `recovery-required` として止め、危険な自動 rollback をしない設計です。

## v0.1〜v0.7 の流れ

### v0.1 — Secure Workspace Runtime MVP

最小の縦切りを作る段階です。

```text
host directory
  -> Incus Environment
  -> exec / shell
  -> delete
```

Hacocoon の最小価値をまず成立させます。

### v0.2 — Workspace Abstraction & Lease

Workspace の由来と Environment への貸し出しを分離し、RO/RW lease と ownership safety を導入します。

Git worktree は Core concept ではなく Workspace を作る一手段として扱います。

### v0.3 — Client & Interactive Access

人間が Environment を使いやすくします。

Status、SSH、loopback port forwarding、VS Code Remote-SSH 等を整えます。

IDE 固有 UX を Core に入れないのが重要です。

### v0.4 — Policy & Capability Foundation

Ambient credential を減らし、privileged authority を明示的な Capability boundary にします。

Allow / deny / require-approval、Human security approval、Audit を導入します。

### v0.5 — Git / GitHub Capability

GitHub への privileged operation を broker し、Environment に broad parent credential を渡さない形を作ります。

### v0.6 — Agent & Orchestrator Integration

`haco run`、machine-readable output、security event export 等を整え、外部 Agent / Orchestrator から使いやすくします。

ただし Hacocoon 自体は Agent scheduler にはなりません。

### v0.7 — Remote / Cloud Runtime & External Capabilities

Local Incus と同じ高位 Environment model を remote/cloud に拡張します。

EC2 provider、AWS capability、EBS replacement flow などが入りました。

EC2 は experimental / disabled by default のままです。

## Breaking Change を許す理由

Hacocoon はまだ pre-1.0 です。

現在は compatibility を最優先するより、architecture を小さく・安全に・交換しやすくすることを優先します。

そのため必要なら次を行います。

- rename
- delete
- replace
- state format change
- CLI redesign
- provider boundary redesign
- capability schema change

ただし「Breaking Change してよい」と「雑に壊してよい」は同じではありません。

次は守ります。

- security boundary を弱めない。
- silent data loss を許さない。
- destructive operation の target identity を曖昧にしない。
- partial failure / retry / recovery を考える。
- material operator impact を document する。
- safe migration path がある場合は明示する。

## 開発時の判断基準

迷ったら次を優先します。

1. Core を小さくする。
2. Responsibility boundary を明確にする。
3. Concrete provider detail を Core に漏らさない。
4. Premature abstraction を作らない。
5. Host authority を Environment に渡さない。
6. Security-sensitive operation は fail closed。
7. Cleanup / retry / partial failure を正常系と同じくらい重要に扱う。
8. Compatibility のために危険な設計を残さない。
9. ただし data loss と unsafe migration は避ける。
10. 実装の存在と real-provider acceptance を混同しない。

## 次に読む資料

日本語:

- [`../README.ja.md`](../README.ja.md)
- [`README.ja.md`](README.ja.md)
- [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md)

英語の正本:

- [`00_REBASELINE_AND_ROADMAP.md`](00_REBASELINE_AND_ROADMAP.md)
- [`00C_TERMINOLOGY_AND_BOUNDARIES.md`](00C_TERMINOLOGY_AND_BOUNDARIES.md)
- [`00B_SECURITY_ARCHITECTURE.md`](00B_SECURITY_ARCHITECTURE.md)
- [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md)
- [`01_v0.1_SECURE_WORKSPACE_RUNTIME.md`](01_v0.1_SECURE_WORKSPACE_RUNTIME.md) 〜 [`07_v0.7_REMOTE_AND_CLOUD_RUNTIME.md`](07_v0.7_REMOTE_AND_CLOUD_RUNTIME.md)
