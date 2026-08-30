# Optional OCI Plugin と Docker Compatibility

[English](OCI_RUNTIME_AND_DOCKER_COMPAT.md) | **日本語**

Status: **optional pluginのpackaging foundationは実装済み。Base/Seedへの焼き込みとreal-host acceptanceはpending。**

この資料はproject-maintained OCI plugin profileを定義する。**Hacocoon CoreのOCI runtime dependencyを定義する資料ではない。**

## Core rule

Hacocoon CoreはWorkspaceとisolated Environmentを管理するためにcontainer CLI/runtimeを要求しない。`HACO_PLUGIN_OCI`未設定なら、nerdctl、Docker CLI、dockerd、Host OCI cache、Local Registryを要求してはいけない。

OCI developer toolingが必要なoperatorだけprofileを明示選択する。

```text
HACO_PLUGIN_OCI=nerdctl
HACO_PLUGIN_OCI=docker
```

## Project-maintained nerdctl profile

container toolingを使いたいdeployment向けの軽量profileは次の形を推奨する。

```text
containerd  (Environment-local service/content store)
    ^
    |
nerdctl     (ordinary CLI)
```

これはplugin profileであり、Hacocoon Core runtime contractではない。

## Docker compatibility profile

一部developer toolingはgenuine Docker CLIやDocker Engine API semanticsを要求する。optional Docker profileでは次の互換経路を提供できる。

```text
Docker CLI / Docker API client
          |
          | unix:///var/run/docker.sock
          v
plugin-owned hacocoon-docker.socket
          |
          | socket activation
          v
       dockerd
          |
          | --containerd=/run/containerd/containerd.sock
          v
 Environment-local containerd
```

Rule:

1. Hacocoon wrapperではなくgenuine Docker CLIを使う
2. `dockerd`はoptionalで、Docker API accessが必要になるまで通常inactive
3. Docker互換のためだけに2個目のprivate containerdを起動しない
4. Docker APIはdefaultでEnvironment-local Unix socketのみ
5. Host Docker socketをHacocoon Environmentへmountしない
6. Docker/nerdctl credentialをCore credentialとして扱わない

unitは次に置く。

```text
modules/plugin/oci/packaging/systemd/
```

vendor `docker.service` / `docker.socket`を上書きせずHacocoon固有unit名を使う。

## Socket activation

`hacocoon-docker.socket`が`/run/docker.sock`をlistenし、最初のrequestで`hacocoon-docker.service`をactivateする。socket activation自体はidleになったdockerdを自動停止しないので、idle shutdownはfuture policyでありcurrent claimではない。

plugin socketをenableする前に、同じpathを別serviceがlistenしていないことをprovisioningで確認する。

## containerd namespaceと容量

Docker Engineは独自containerd namespace、nerdctlは別namespaceを使える。同じcontainerd content serviceを使えばcontent-addressed blobを共有でき、高位metadataまで同じnamespaceへ混ぜる必要はない。

ただしtotal storageが完全に0重複になるとは保証しない。namespace metadata、unpacked snapshot、writable layer、build cache、Docker固有stateは追加容量を使う。

Environment間の容量最適化はplanned v0.19 Seed/COWで扱う。1つのwritable `/var/lib/containerd`を複数Environmentで共有しない。

## Base / Seed integration

選択したOCI toolingはEnvironment startごとのpackage installではなくimmutable development Base/Seedへ焼き込む方向。ただしbuild/publish pipelineはまだ完成していない。

Docker-compatible Base/Seedは必要に応じて:

- optional profileとしてsupported containerd/nerdctlをinstall
- genuine Docker CLIをinstall
- Engine互換が必要な場合だけdockerdをinstall
- plugin-owned socket/service unitをinstall
- conflicting vendor auto-startをdisable
- always-on dockerd serviceではなくplugin socketだけenable
- immutable publish前にdockerdを停止
- registry credentialやHost control socketをimageへ焼き込まない

## Security boundary

Docker socket accessは**そのEnvironment内ではroot-equivalent authority**。外側のsecurity boundaryはHacocoon Environmentのまま。

- Host `/var/run/docker.sock`をbind mountしない
- Host Incus/Hacocoon/containerd control socketを互換経路へ露出しない
- Docker APIをdefaultでTCP listenしない
- Docker互換を理由にGitHub/cloud/registry/Host credentialを付与しない

## Telemetry

v0.15 OCI usage telemetryはoptional plugin機能。選択driver（`nerdctl`または`docker`）をinventoryし、複数CLIから同じOCI digestが見えるだけでSeed recommendation weightを二重countしない。

## Acceptance

repository/local CIではplugin-owned systemd packagingをverifyする。real acceptanceではsupported Base/Incus host上で次を確認する。

```text
boot: configured profile services。dockerdはinactive
最初のDocker API request: plugin socketがdockerdをactivate
docker info succeeds
selected OCI CLIも通常利用可能
```

> **Dockerとnerdctlはoptional OCI plugin profile。Hacocoon Coreはuniversal container runtimeを選ばない。**
