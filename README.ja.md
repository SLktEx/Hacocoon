<div align="center">

<img src="docs/assets/readme/hacocoon-logo.webp" alt="Hacocoon — Secure Workspace Runtime" width="520">

# Hacocoon

**読み方: はこーん**

### AI は中で自由に。Host の権限は外で守る。

**人間・開発ツール・Coding Agent向けのOSS Secure Workspace Runtime。**

[English](README.md) · [ドキュメント](docs/README.ja.md) · [セキュリティ](docs/security/security-architecture.md) · [実装状況](docs/IMPLEMENTATION_STATUS.ja.md) · [ロードマップ](docs/status/architecture-and-roadmap.md)

[![CI](https://github.com/SLktEx/Hacocoon/actions/workflows/test.yml/badge.svg)](https://github.com/SLktEx/Hacocoon/actions/workflows/test.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

</div>

<p align="center"><img src="docs/assets/readme/hacocoon-hero.webp" alt="Hacocoon の Secure Workspace Runtime 全体像。AI は中で自由に、Host authority は外で守る" width="100%"></p>

HacocoonはWorkspaceを隔離された実行境界の中に置き、特権authorityをtrusted Host側に残します。Coding AgentはEnvironment内でpackage install、source edit、build、test、debug、破壊的変更まで自由に行えますが、それだけを理由にHost credential、Incus管理authority、無制限の外部アクセス、自分自身のresource limitを引き上げる権限までは得ません。

> [!WARNING]
> **Hacocoonはまだpre-1.0で、Breaking Changeは今後も発生します。**
>
> 現在のEnvironment backendはIncusです。Provider seamはgenericなまま維持し、以前のconcrete EC2/AWS/EBS implementationはdeferredです。現在のrepository realityとreal-host acceptance gapは [実装状況](docs/IMPLEMENTATION_STATUS.ja.md) を参照してください。

## なぜHacocoon?

```text
VS Code / Shell / Coding Agent / Orchestrator
                     |
                  Workspace
                     |
          +----------v------------+
          |       Hacocoon        |
          | isolated Environment  |
          | resource budgets      |
          | workspace leases      |
          | policy / approvals    |
          | capabilities / audit  |
          +----------+------------+
                     |
            Environment provider
                     |
              Incus (current)
```

Hacocoonは **Environment内での自由** と **Host / external serviceを触るauthority** を分離します。

## Quick start

<p align="center"><img src="docs/assets/readme/hacocoon-quickstart.webp" alt="Hacocoon の VS Code Quick Start" width="100%"></p>

```bash
git clone https://github.com/SLktEx/Hacocoon.git
cd Hacocoon

go build -o ./bin/haco ./cmd/haco
go build -o ./bin/haco-vscode ./cmd/haco-vscode
go build -o ./bin/haco-agent-host ./cmd/haco-agent-host
go build -o ./bin/haco-notify ./cmd/haco-notify

./bin/haco doctor
./bin/haco run --workspace "$PWD" -- go test ./...
```

Named Environmentを残す場合:

```bash
./bin/haco create --workspace "$PWD" dev
./bin/haco exec dev -- go test ./...
./bin/haco shell dev
./bin/haco status dev
./bin/haco delete dev
```

VS Codeでは:

```bash
./bin/haco-vscode open .
```

VS Codeは最初のconvenience clientであり、Core dependencyではありません。Windows + WSL hostの準備は [Windows / WSL bootstrap](docs/WINDOWS_WSL_BOOTSTRAP.ja.md) を参照してください。

## Core / Standard / Plugin

| Layer | 役割 | 例 |
|---|---|---|
| **Core** | 安定したproduct semantics / security boundary | Environment、Workspace lease、Policy、Capability、interaction contract |
| **Standard** | 通常配布で使うproject-maintainedな交換可能default | 現在のIncus backend、hostname-aware egress enforcement |
| **Plugin** | optional / specialized integration | Git helper、nerdctl / Docker / OCI tooling |

詳細は [Adapter and extension architecture](docs/design/plugin-architecture.md) と [設計原則](docs/DESIGN_PRINCIPLES.ja.md) を参照してください。

## Baseとoptional OCI tooling

```bash
haco base list
haco base inspect haco/ubuntu-26.04
haco create --base haco/ubuntu-26.04 --workspace "$PWD" dev

HACO_PLUGIN_OCI=nerdctl haco plugin oci seed sample
HACO_PLUGIN_OCI=nerdctl haco plugin oci seed recommend
HACO_PLUGIN_OCI=nerdctl haco plugin oci seed build
HACO_PLUGIN_OCI=nerdctl haco plugin oci seed current
HACO_PLUGIN_OCI=docker  haco plugin oci docker status dev
HACO_PLUGIN_OCI=docker  haco plugin oci docker prepare dev
```

Coreはcontainerd、nerdctl、Docker、local Registryを必須にしません。OCI Seed Builder/COWはv0.17のpartial checkpoint、Docker Compatibilityはv0.18、Domain-aware egressはv0.19、Hacocoon管理Btrfs rootfs storageはv0.20、managed Btrfs `compress=zstd:3` はv0.21です。その後、v0.22 Interaction Notification Clients、v0.23 Real Incus E2E Acceptance、v0.24 Structured Logging、v0.25 Managed Btrfs Host Privilege Brokerまで進んでいます。現在のmilestone位置は **v0.25** です。pre-1.0のminor versionはproductだけでなくoperator experience、observability、acceptanceの意味ある進捗にも使う軽量なcheckpointです。Local OCI Registryはdeferred/unversionedなoptional infrastructureです。

## Reusable client

`pkg/clientadapter` はexact Environment ensure/reuse、status、`/workspace` discovery、loopback SSH/TCP connection、revoke/delete、interaction batchのclient-neutral contractを提供します。SSH private keyとIDE configはclient自身が保持し、Hacocoonが受け取るのはpublic-key materialだけです。

v0.22のnotification clientは同じread-only interaction streamをbrowser、native OS、optional VS Code notificationへ接続しますが、event観測をapproval pathにはしません。

詳細は [Reusable client adapter contract](docs/CLIENT_ADAPTER_CONTRACT.ja.md) と [Interaction events](docs/INTERACTION_EVENTS.ja.md) を参照してください。

## Security model

Trusted HostがHacocoon state、Policy、Credential、Resource ceiling、privileged Capability executionを所有します。Coding AgentがEnvironment内でcode edit / build / test / debugをするためだけに、HacocoonやIncusのmanagement authorityは必要ありません。

Security-sensitiveな変更の前に [Security architecture](docs/security/security-architecture.md) と [adversarial audit guide](.github/security/ADVERSARIAL_AUDIT.md) を読んでください。

## ドキュメント

- [ドキュメント一覧](docs/README.ja.md)
- [Documentation style guide](docs/DOCUMENTATION_STYLE_GUIDE.md)
- [実装状況](docs/IMPLEMENTATION_STATUS.ja.md)
- [Architecture / Roadmap](docs/status/architecture-and-roadmap.md)
- [Versioning / Release status](docs/status/versioning-and-release-status.ja.md)
- [用語と境界](docs/reference/terminology-and-boundaries.md)

通常のdocumentation pathはsemantic nameを使い、feature addressへrelease/milestone番号を含めません。ADR sequence numberだけはidentityとして例外です。

## 開発

Hacocoonのprimary supported Host baselineは **Ubuntu 26.04+** です。GitHub-hosted Linux CIも **`ubuntu-26.04`** に明示固定し、`ubuntu-latest` や古いUbuntu世代ではなく同じbaselineを直接検証します。v0.23ではreal Incus substrate + Core lifecycle acceptanceを独立milestoneにし、v0.25ではordinary-user managed-Btrfs privilege pathをreal Incusで検証します。

```bash
go test ./...
go test -race ./...
go vet ./...
python tools/check_docs.py
bash tools/ci-local.sh
```

## License

Hacocoonは [Apache License 2.0](LICENSE) で公開します。
