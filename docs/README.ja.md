# Hacocoon ドキュメント

[English](README.md) | **日本語**

Hacocoon は **pre-1.0** です。ドキュメントでは、混同しやすい次の3つを明確に分けます。

- **architecture intent** — Hacocoon が何を所有し、どこに境界を置くか
- **repository reality** — 現在の `main` に実際に何が入っているか
- **acceptance** — real Incus / Windows / AWS 環境で何が確認済みか

> [!TIP]
> **「いま `main` で何が使える？」だけ知りたい場合は [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md) から読んでください。**

## まずここから

| 知りたいこと | 読む資料 |
|---|---|
| Hacocoon 全体像 | [`../README.ja.md`](../README.ja.md) |
| 現在の実装状況 | [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md) |
| architecture / roadmap | [`00_REBASELINE_AND_ROADMAP.md`](00_REBASELINE_AND_ROADMAP.md) |
| version番号 | [`00D_VERSIONING_AND_RELEASE_STATUS.ja.md`](00D_VERSIONING_AND_RELEASE_STATUS.ja.md) |
| security boundary | [`00B_SECURITY_ARCHITECTURE.md`](00B_SECURITY_ARCHITECTURE.md) |
| 用語・責務境界 | [`00C_TERMINOLOGY_AND_BOUNDARIES.md`](00C_TERMINOLOGY_AND_BOUNDARIES.md) |
| VS Code / client接続 | [`CLIENT_ACCESS.md`](CLIENT_ACCESS.md), [`08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md`](08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md) |
| agentごとのsandbox | [`09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md`](09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md) |
| VS Code Agent Host | [`10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.ja.md`](10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.ja.md) |
| Base Image | [`BASE_IMAGES.ja.md`](BASE_IMAGES.ja.md) |
| Resource Limit | [`12_v0.12_SANDBOX_RESOURCE_LIMITS.ja.md`](12_v0.12_SANDBOX_RESOURCE_LIMITS.ja.md) |
| OCI Seed / 利用統計 / COW | [`13A_v0.13_OCI_SEED_AND_COW.ja.md`](13A_v0.13_OCI_SEED_AND_COW.ja.md), [`13B_v0.13_SEED_AUTO_PROMOTION.ja.md`](13B_v0.13_SEED_AUTO_PROMOTION.ja.md), [`13C_v0.13_OCI_IMAGE_DELETION.ja.md`](13C_v0.13_OCI_IMAGE_DELETION.ja.md) |
| optional Local OCI Registry | [`13_v0.13_LOCAL_OCI_REGISTRY.md`](13_v0.13_LOCAL_OCI_REGISTRY.md) |

## 正本の使い分け

資料が食い違って見える場合は、**質問ごとに正本を選びます**。

1. **現在のcode reality:** `IMPLEMENTATION_STATUS.md`
2. **milestone番号・status:** `00D_VERSIONING_AND_RELEASE_STATUS.md`
3. **product boundary / roadmap:** `00_REBASELINE_AND_ROADMAP.md`
4. **canonical terminology:** `00C_TERMINOLOGY_AND_BOUNDARIES.md`
5. **security rule:** `00B_SECURITY_ARCHITECTURE.md`
6. **feature contract:** 各versioned specification
7. **detail / operation:** `CLIENT_ACCESS.md`, `REMOTE_CLOUD_PROVISIONING.md`, `BASE_IMAGES.md` など
8. **plugin / adapter guidance:** `00A_PLUGIN_ARCHITECTURE.md`
9. **implementation workflow:** `90_CODEX_IMPLEMENTATION_HANDOFF.md`
10. **historical / non-normative:** `91_IMPLEMENTATION_REFERENCE_NOTES.md`, `adr/`

READMEやindexは入口です。**現在の実装事実を上書きする正本ではありません。**

## 現在のmilestone

v0.1〜v0.12 の既存gateに加え、v0.13 OCIでは利用統計・recommendation・上位10%自動選択・OCI image削除/tombstoneのsliceが実装済みです。Seed build/publish、Environmentからのimage harvesting、real Btrfs COW acceptance、optional Registryは未完です。

## Specification と Implementation は別

versioned specification が存在することは、その機能が全部 `main` に実装済みという意味ではありません。

- v0.7 EC2: experimental実装済み。real AWS acceptance pending
- v0.8 `haco-vscode`: 実装済み。real Windows/WSL + Incus + VS Code acceptance pending
- v0.9: persisted per-session Environment broker 実装済み。real Agent Host/AHP routingはhost-dependent
- v0.10 `haco-agent-host`: 実装済み。real Agent Host acceptanceはhost-dependent
- v0.11 Base selection/pinning: 実装済み。custom build/import/history/GCはfollow-up
- v0.12 ResourceBudget: Incus adapterで実装済み。real workload enforcementはhost-dependent
- v0.13 OCI: telemetry/recommend/deleteは実装済み、Seed build/publishはplanned

## Base Image

```text
haco base list
haco base inspect <base>
haco create --base <base> --workspace <path> <environment>
```

`haco base` はHacocoon/Incus Environmentのstarting point専用です。OCI/container imageはこのnamespaceに混ぜません。

## v0.13 OCI plugin

OCI/containerd/nerdctl固有の操作はoptional plugin namespaceにまとめます。

```text
haco plugin oci seed sample
haco plugin oci seed recommend
haco plugin oci image delete <reference> [--all-environments]
```

曖昧だったpre-1.0の `haco image ...` はcompatibility aliasとして残しません。

## Breaking Change

Hacocoonはpre-1.0で、security / ownership boundaryを明確にするためのBreaking Changeを許容します。

accidental compatibilityを守るためにsecurity boundaryを弱めるより、明示的に互換性を壊して正す方を優先します。

## ドキュメント更新ルール

1. その事実を所有するauthoritative documentを先に更新する
2. code realityが変わったら `IMPLEMENTATION_STATUS.md` を更新する
3. numbering/statusが変わったら `00D_VERSIONING_AND_RELEASE_STATUS.md` を更新する
4. English authoritative doc → Japanese companion の順で揃える
5. implementation claim と real-host acceptance claim を混ぜない
6. experimental/default-off boundaryを明記する
7. `python tools/check_docs.py` を実行する
