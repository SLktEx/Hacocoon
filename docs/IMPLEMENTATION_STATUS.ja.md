# 実装状況

[English](IMPLEMENTATION_STATUS.md) | **日本語**

Status date: 2026-08-29 — v0.8 Client Adapters & VS Code Integration implementation pass 後、v0.9 Base Images & Custom Environments roadmap decision 反映後。

このファイルは **現在のコードの事実**を説明するための日本語版です。理想 architecture や互換性保証ではありません。

Hacocoon はまだ **pre-1.0** です。実装済みであることは interface 固定、本番 support、real-provider/client acceptance 済みを意味しません。

また、v0.9 の specification が存在することは、v0.9 のコードが実装済みであることを意味しません。現在の implementation progression は v0.8 までです。

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
| Windows / WSL bootstrap | `scripts/bootstrap-windows.ps1` は default/既存の普段使い WSL を選ばず、`Hacocoon` という dedicated WSL 2 instance を create/reuse。Linux dependency setup は `scripts/bootstrap-wsl.sh`、Hacocoon install は既存 `scripts/install.sh` に委譲。無関係な WSL は触らず、`incus-admin` は explicit opt-in | v0.8 | PowerShell / shell syntax は CI 対象。real Windows install/reboot/dedicated WSL/Incus acceptance pending |
| Client Adapter boundary | VS Code / Daintree / JetBrains 等の client-specific behavior を Core に入れない | v0.8 | architecture + separate binary boundary |
| Base Images & Custom Environments | logical Base、immutable revision、Incus fingerprint pinning の adapter boundary、custom image の trust boundary、safe deletion/reference semantics を v0.9 contract として定義 | v0.9 | **design only / implementation pending**。`haco image` / `haco create --base` はまだ実装済みと扱わない |
| CI | Go tests、vet、race、docs consistency、bootstrap syntax、release packaging、host-independent E2E | cross-cutting | implementation PR の CI pass が merge gate。real provider/client acceptance は別 |

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

## v0.9 で作るもの

次の explicit roadmap gate は **Base Images & Custom Environments** です。

```text
logical Base name
  -> immutable Base revision
  -> Incus fingerprint (adapter内部)
  -> newly created Environment
```

大事な contract は次です。

```text
my-dev -> revision A -> Environment 1
my-dev -> revision B -> Environment 2
Environment 1 は revision A のまま
```

つまり logical Base の更新は **新しく作る Environment だけ**に反映します。既存 Environment の Base を途中で差し替えません。

Custom Base は untrusted input として扱い、image metadata だけで Host mount、device、privileged mode、Linux capability、credential、network authority、GitHub/AWS authority 等を増やすことを許可しません。

予定している interaction は例えば次です。

```text
haco image list
haco image inspect <base>
haco create --base <base> --workspace <path> <environment>
```

ただしこれはまだ **planned CLI** であり、実装済みでも固定済みでもありません。

正本は [`09_v0.9_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md`](09_v0.9_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md)、詳細は [`BASE_IMAGES.ja.md`](BASE_IMAGES.ja.md) / [`BASE_IMAGES.md`](BASE_IMAGES.md) を参照してください。

## Windows / WSL bootstrap

Windows host の初期 setup 用:

```powershell
.\scripts\bootstrap-windows.ps1
```

これは普段使いの Ubuntu / Debian を流用せず、標準では次の dedicated instance を作ります。

```text
Instance: Hacocoon
Base: Ubuntu-26.04
```

概念的には:

```powershell
wsl --install Ubuntu-26.04 --name Hacocoon --no-launch
```

です。既に `Hacocoon` が存在する場合だけその instance を再利用し、default WSL や最初に見つかった既存 distribution へ fallback しません。

Fresh PC では Windows reboot や Linux user の初回作成が必要なら一度停止し、`wsl -d Hacocoon` で初期設定後に bootstrap を再実行します。

既存の無関係な WSL は user-owned state として扱い、unregister / reset / delete / WSL 1 conversion / default変更は自動で行いません。

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

Hacocoon/Incus は dedicated `Hacocoon` WSL、VS Code は Windows desktop に置きます。VS Code Remote-SSH は Windows Client 側の SSH configuration を利用し、`haco-vscode` が adapter 側でこの差を吸収します。

Real Windows + dedicated WSL + Incus + VS Code Remote-SSH の end-to-end acceptance は対応環境で別途必要です。

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

Real Incus、Base/image lifecycle、Windows dedicated WSL install、Windows/WSL + VS Code Remote-SSH、AWS/EC2/SSM/EBS はそれぞれ対応環境で確認します。

## Compatibility status

v0.1〜v0.9 のどの design / implementation も、現在の concrete interface が変更されないという約束ではありません。

Breaking Change により CLI / helper binary / state / provider / Base/image lifecycle / capability / client-adapter configuration / host bootstrap behavior 等は変更可能です。

Compatibility のために unsafe authority boundary、曖昧な ownership、silent data loss、不要な complexity を残しません。ただし material change は explicit・tested・documented にします。

## Release / tag

この implementation status だけを見て release/tag readiness を判断しません。Specification の acceptance requirement と、その時点の stability level を別途確認します。
