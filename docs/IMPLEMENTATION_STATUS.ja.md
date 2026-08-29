# 実装状況

[English](IMPLEMENTATION_STATUS.md) | **日本語**

Status date: 2026-08-29 — v0.8 Client Adapters & VS Code Integration implementation pass 後。

このファイルは **現在のコードの事実**を説明するための日本語版です。理想 architecture や互換性保証ではありません。

Hacocoon はまだ **pre-1.0** です。実装済みであることは interface 固定、本番 support、real-provider/client acceptance 済みを意味しません。

| 領域 | 現在の repository reality | Release | 検証状況 |
|---|---|---:|---|
| Secure Workspace Runtime | `haco create --workspace` / `exec` / `shell` / `delete` | v0.1 | unit / process integration pass。real Incus acceptance は host-dependent |
| Workspace / Lease | canonical Workspace identity、RO/RW lease、RW conflict prevention、stale recovery、process serialization | v0.1-v0.2 | unit / persistence / concurrency / integration pass |
| Incus provider | local default Environment provider | v0.1+ | unit / process pass。real host acceptance pending |
| Client access | status、loopback forward、connection管理、SSH prepare/revoke、public-key hardening | v0.3 | unit / process integration pass。real Incus SSH acceptance pending |
| Policy / Capability | fail-closed allow/deny/require-approval、human security approval、JSONL audit | v0.4 | unit / integration / CLI E2E pass |
| Git / GitHub | Host-side brokered push。broad host credential を Environment に export しない | v0.5 | unit / adversarial / real-git integration / CLI E2E pass |
| Agent / Orchestrator | `haco run`、machine JSON、security event export。DAG/model selection は外部 responsibility | v0.6 | unit / race / integration / CLI E2E pass |
| Environment routing | provider-neutral router | v0.7 | unit pass |
| EC2 provider | S3 staging + SSM、experimental / disabled by default | v0.7 | fake-AWS path pass。real AWS acceptance pending |
| AWS capability | narrow host-side `aws.api` read capability | v0.7 | fake-AWS CLI/integration pass。real AWS pending |
| EBS replacement | adapter-owned replacement/migration。in-place shrink / automatic source deletion なし | v0.7 | fake-AWS integration pass |
| VS Code Client Adapter | separate `haco-vscode` binary。Environment create/reuse -> existing SSH path -> adapter-owned SSH config -> standard Remote-SSH `/workspace` | v0.8 | helper unit coverage added。real VS Code + Incus acceptance pending |
| Windows / WSL bridge | WSL 実行時に Windows user profile を解決し、desktop Client 側 `.ssh` を対象にする | v0.8 | implementation exists。real Windows/WSL acceptance pending |
| Windows / WSL bootstrap | `scripts/bootstrap-windows.ps1` が既存 WSL を破壊せず WSL 2 を install/select し、`scripts/bootstrap-wsl.sh` へ Linux dependency setup を委譲し、Hacocoon 本体は既存 `scripts/install.sh` で install。`incus-admin` は explicit opt-in | v0.8 | PowerShell / shell syntax は CI 対象。real Windows install/reboot/WSL/Incus acceptance pending |
| Client Adapter boundary | VS Code / Daintree / JetBrains 等の client-specific behavior を Core に入れない | v0.8 | architecture + separate binary boundary |
| CI | Go tests、vet、race、docs consistency、bootstrap syntax、release packaging、host-independent E2E | cross-cutting | v0.8 PR CI pass が merge gate。real provider/client acceptance は別 |

## v0.8 で増えたもの

通常の VS Code 利用では次を狙います。

```bash
haco-vscode open .
```

内部では概念的に:

```text
Workspace
  -> Environment create/reuse
  -> loopback-only SSH prepare
  -> Hacocoon-owned SSH host entry
  -> code --remote ssh-remote+<alias> /workspace
```

終了して Environment を削除する場合:

```bash
haco-vscode delete .
```

Private SSH key は Client 側に残し、Hacocoon の既存 SSH path には public key のみを渡します。

## Windows / WSL bootstrap

Windows host の初期 setup 用に次を追加しています。

```powershell
.\scripts\bootstrap-windows.ps1
```

Fresh PC では WSL 2 distribution の install まで行い、Windows reboot や Linux user の初回作成が必要ならそこで一度止まります。既存 WSL がある場合は user-owned state として扱い、unregister / reset / delete / WSL 1 conversion は自動では行いません。

WSL 内では base dependency と Incus を準備し、Hacocoon binary 自体は既存 `scripts/install.sh` に委譲します。

Incus administrator 権限は root 相当なので自動付与しません。明示的に許可する場合だけ:

```powershell
.\scripts\bootstrap-windows.ps1 -GrantIncusAdmin
```

を使います。詳細は [`WINDOWS_WSL_BOOTSTRAP.ja.md`](WINDOWS_WSL_BOOTSTRAP.ja.md) を参照してください。

## AI の扱い

v0.8 は Hacocoon 独自の AI UI を追加しません。

VS Code 上の Copilot / Codex / Claude 等の既存 UI / extension を使い、Agent は Incus Environment 内で permissive に動かせます。

```text
Agent
  -> isolated Environment      <- broad local freedom
  -> Hacocoon trust boundary
  -> Policy / Capability / Audit
  -> GitHub / AWS / Host
```

Environment 内の freedom と Host authority は分離したままです。

## Windows + WSL

Hacocoon/Incus が WSL、VS Code が Windows desktop の場合、VS Code Remote-SSH は Windows Client 側の SSH configuration を利用します。

`haco-vscode` はこの差を adapter 側で吸収します。ただし real Windows + WSL + Incus + VS Code Remote-SSH の end-to-end acceptance は対応環境で別途必要です。

## Orchestrator

Daintree 等との統合は引き続き Hacocoon の上位です。

```text
Daintree
  -> task / worktree / agent orchestration
  -> Workspace
  -> Hacocoon Environment
```

Hacocoon 自体は worktree manager、Agent scheduler、model router にはなりません。

## EC2 の扱い

v0.7 EC2 provider は引き続き **experimental / disabled by default** です。

```bash
export HACO_RUNTIME_PROVIDER=runtime.ec2
export HACO_EXPERIMENTAL_EC2=1
```

両方を明示しない限り有効化されません。Real AWS / EC2 / SSM / EBS acceptance は pending です。

## Acceptance の区別

次が pass しても real-provider/client acceptance の代替にはなりません。

- unit test
- process-boundary integration
- fake-provider E2E
- race
- vet
- build
- script syntax
- repository CI

Real Incus、Windows/WSL install、Windows/WSL + VS Code Remote-SSH、AWS/EC2/SSM/EBS はそれぞれ対応環境で確認します。

## Compatibility status

v0.1〜v0.8 のどの実装も、現在の concrete interface が変更されないという約束ではありません。

Breaking Change により CLI / helper binary / state / provider / capability / client-adapter configuration / host bootstrap behavior 等は変更可能です。

Compatibility のために unsafe authority boundary、曖昧な ownership、silent data loss、不要な complexity を残しません。ただし material change は explicit・tested・documented にします。

## Release / tag

この implementation status だけを見て release/tag readiness を判断しません。Specification の acceptance requirement と、その時点の stability level を別途確認します。
