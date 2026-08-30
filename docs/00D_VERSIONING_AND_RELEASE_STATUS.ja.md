# バージョン番号とリリース状況

[English](00D_VERSIONING_AND_RELEASE_STATUS.md) | **日本語**

> **milestone番号の日本語案内 · 2026-08-30更新**

Hacocoon は **pre-1.0** です。milestone番号はproduct/implementationの進行順を表すためのもので、compatibility guarantee、release tag、production supportの証明ではありません。

**番号の正本は英語版 [`00D_VERSIONING_AND_RELEASE_STATUS.md`](00D_VERSIONING_AND_RELEASE_STATUS.md)**、現在の実装事実は [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md) です。

## 番号付けの方針

1. 実装済みmilestoneは、変更コストが低い間はなるべく連続させる
2. design-only gateのせいで、既に実装済みの独立機能を後ろの番号へ追いやらない
3. security/hardeningだけでは通常product versionを消費しない
4. planned specificationが次の番号を予約しても、codeが入るまでは **planned / not implemented** と明記する
5. release tagとroadmap milestone番号は別物
6. repository implementationの正本は `IMPLEMENTATION_STATUS.md`

## 現在の番号

**凡例:** ✅ 実装済み · 🧪 experimental/historical · 🚧 planned

| Version | Gate | `main` の状況 |
|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | ✅ 実装済み |
| v0.2 | Workspace Abstraction & Lease | ✅ 実装済み |
| v0.3 | Client & Interactive Access | ✅ 実装済み |
| v0.4 | Policy & Capability Foundation | ✅ 実装済み |
| v0.5 | Git / GitHub Capability | ✅ 実装済み |
| v0.6 | Agent & Orchestrator Integration | ✅ 実装済み |
| v0.7 | Remote / Cloud Runtime & External Capabilities | 🧪 provider routing seamは維持。以前のEC2/AWS/EBS実装はdeferred |
| v0.8 | Client Adapters & VS Code Integration | ✅ 実装済み。real client acceptance pending |
| v0.9 | Per-Agent Sandbox & Agent Host Integration | ✅ broker foundation 実装済み |
| v0.10 | VS Code Remote Agent Host Adapter | ✅ PR #137で実装済み。real host acceptanceはpending |
| v0.11 | Base Images & Custom Environments | ✅ first slice 実装済み。build/import/history/GCはfollow-up |
| v0.12 | Sandbox Resource Limits | ✅ first slice 実装済み。real workload enforcementはhost-dependent |
| v0.13 | Local OCI Registry | 🚧 planned。`main` には未実装 |
| v0.13A | OCI Seed & Btrfs/COW Optimization | 🚧 planned second slice。`main` には未実装 |

**実装済みmilestoneは v0.1〜v0.12 まで連続**しています。v0.13は次のplanned milestoneであって、current implementationではありません。

v0.7で導入したprovider-neutral routing seam自体は残っているため、後続milestoneの番号は変更しません。EC2/AWS/EBSのconcrete implementationを一旦外すことと、milestone番号を消すことは分けて扱います。

## Implemented と Planned

```text
implemented on main
v0.1 ───────────────────────────── v0.12
                                      |
                                      v
                                next planned
                                   v0.13
                                      |
                                      v
                              planned second slice
                                  v0.13A
```

`13_v0.13_LOCAL_OCI_REGISTRY.md` や `13A_v0.13_OCI_SEED_AND_COW.md` が存在しても、その機能が実装済みという意味ではありません。

## 番号変更の履歴

2026-08-30の整理で、design-onlyだったBase関連を既に実装済みだったper-agent workより前に置く一時的な番号割り当てを解消しました。

```text
v0.9   Per-Agent Sandbox & Agent Host Integration    implemented
v0.10  VS Code Remote Agent Host Adapter             implemented
v0.11  Base Images & Custom Environments             implemented first slice
v0.12  Sandbox Resource Limits                       implemented first slice
v0.13  Local OCI Registry                            planned
```

古いcommit message、closed PR、candidate branch、過去のplanning textに旧番号が残っていても、それはhistorical recordです。

## Acceptance watch list

- **v0.7:** cloud implementationは現在deferred。以前のEC2 providerはexperimental/default-offで、real AWS acceptance pendingだった。cloud adapterを復活させた時点でacceptanceを改めて定義する
- **v0.8:** real Windows/WSL + Incus + VS Code Remote-SSH
- **v0.9/v0.10:** real VS Code Agent Host/AHP routing、Incus SSH
- **v0.11:** real Incus image source/custom Base。build/import/history/rollback/GCはfirst slice外
- **v0.12:** real Incus CPU/memory/PID/root-storage enforcement
- **v0.13/v0.13A:** plannedのみ。specificationの存在だけではimplementation/acceptance開始を意味しない

## 一文でいうと

> **番号はこのファイル、実装事実は `IMPLEMENTATION_STATUS.md`、設計意図はroadmap/specificationを見る。**
