# 実装状況

[English](IMPLEMENTATION_STATUS.md) | **日本語**

Status date: 2026-08-29 — v0.7 Remote / Cloud Runtime implementation pass 後。

このファイルは **現在のコードの事実**を説明するための日本語版です。

理想 architecture や互換性保証を示すものではありません。`docs/01_...`〜`docs/07_...` は各 roadmap stage の versioned design reference です。

Hacocoon はまだ **pre-1.0** です。`main` にある機能が実装済みであることは、その CLI / API / state / config surface が固定済み、本番利用を保証済み、すべての real-provider acceptance test が pass 済み、という意味ではありません。

Repository には rebaseline 前の historical code も残っています。Historical package が存在するだけで current public architecture を定義するわけではありません。

| 領域 | 現在の repository reality | Release | 検証状況 |
|---|---|---:|---|
| Secure Workspace Runtime | `haco create --workspace` / `haco exec` / `haco shell` / `haco delete` の public Environment path を実装済み | v0.1 | unit / process-boundary integration pass。supported host での real Incus acceptance は未実施 |
| Workspace model | `Workspace`、`Environment`、`ExecutionResult`、canonical external-path Workspace identity、persisted Workspace lease を実装済み | v0.1-v0.2 | unit / persistence / concurrency / process-boundary test pass |
| Workspace lease safety | RO/RW lease、RW conflict prevention、stale-lease recovery、process serialization を実装済み | v0.2 | unit / concurrency / integration test pass |
| Incus Environment provider | concrete local Incus Environment implementation が default runtime | v0.1+ | unit / process test pass。real Incus host acceptance は現在の sandbox では未実施 |
| Client access | status、local-only port forwarding、connection list/remove、SSH prepare/revoke、public-key hardening を実装済み | v0.3 | unit / process integration pass。real Incus SSH acceptance は host dependency |
| Policy / Capability | fail-closed PolicyEvaluator、allow/deny/require-approval、human security approval、request correlation、JSONL audit を実装済み | v0.4 | unit / process integration / actual CLI capability E2E pass |
| Git / GitHub capability | normalized repo/ref authority、exact source SHA、Policy/Approval、force-with-lease を使った host-side brokered GitHub push を実装。host credential を Environment に export しない | v0.5 | unit / adversarial test / real-git integration / actual CLI E2E pass |
| Agent / orchestrator integration | `haco run`、stable machine JSON output、external security event export を実装。Orchestration/DAG/model selection は Hacocoon に入れない | v0.6 | unit / race / process integration / actual CLI E2E pass |
| Environment routing | provider-neutral Environment router を実装。pre-v0.7 bare runtime ref は Incus ref として backward-compatible | v0.7 | router unit test pass |
| EC2 Environment provider | S3 staging + SSM driven の experimental EC2 Environment provider を実装。default disabled で provider selection と explicit Hacocoon opt-in の両方が必要 | v0.7 | unit / fake-`aws` process integration / actual `haco` fake-AWS E2E pass。real AWS acceptance は未実施 |
| Experimental EC2 gate | `HACO_RUNTIME_PROVIDER=runtime.ec2` だけでは有効化されず、`HACO_EXPERIMENTAL_EC2=1` も必要。disabled path は AWS activity より前に fail | v0.7 | actual binary E2E で disabled path の fake-AWS call が 0 件であることを確認 |
| AWS capability | narrow host-side `aws.api` read capability を既存 Policy/Approval/Audit path と generic capability CLI 経由で実装 | v0.7 | unit / process integration / fake-AWS CLI E2E pass。real AWS acceptance は未実施 |
| EBS replacement | shrink-like operation 向け adapter-owned replacement/migration flow を実装。in-place EBS shrink なし、source-volume 自動削除なし | v0.7 | unit / fake-AWS process integration で preflight / migration / verification / cleanup / recovery-required transition を確認 |
| Btrfs / raw / QCOW2 historical storage | historical local storage implementation が repository に残存 | historical / provider detail | current Core Environment model の一部ではなく compatibility commitment でもない |
| CI | Go version matrix、`go vet`、race detector、docs consistency、host-independent E2E を有効化 | cross-cutting | maintained CI path で検証 |

## 現在の実装到達点

`main` の実装進行は現在 v0.7 まで到達しています。

```text
Workspace
  -> Environment lifecycle
  -> local Incus by default
  -> Workspace leases and client access
  -> Policy / Approval / Capability boundary
  -> Git/GitHub broker
  -> machine/orchestrator access
  -> experimental remote EC2 provider and AWS capability
```

## EC2 の扱い

v0.7 EC2 provider は **experimental かつ disabled by default** です。

実装が repository に入ったことは、EC2 が通常の supported backend になったことを意味しません。

現在の sandbox では real AWS / EC2 / SSM / EBS acceptance は実行されていません。したがって pass 済みと表現してはいけません。

EC2 を有効化するには現在次の両方が必要です。

```bash
export HACO_RUNTIME_PROVIDER=runtime.ec2
export HACO_EXPERIMENTAL_EC2=1
```

## Incus の扱い

Real Incus acceptance path は存在しますが、supported Incus host 上で実際に実行する必要があります。

Unit test、process-boundary integration、fake-provider E2E、race、vet、build、repository CI は real host/provider acceptance の代替ではありません。

## Compatibility status

v0.1〜v0.7 のどの実装 row も「現在の具体的な interface が今後変更されない」という約束ではありません。

明示的な stable compatibility milestone が宣言されるまでは、Breaking Change により次が変更・置換される可能性があります。

- CLI command / flag / output
- persisted state / migration
- provider interface / configuration
- Capability / Policy schema
- experimental runtime behavior

Compatibility を守るために、unsafe authority boundary、曖昧な ownership、silent data loss、不要な architecture complexity を残すことはしません。

Material な Breaking Change は、それでも explicit・tested・documented であるべきです。

## Release / tag について

この implementation status だけを見て release/tag readiness を判断してはいけません。

各 release specification の acceptance requirement と、その時点で意図する stability level の両方を確認してください。
