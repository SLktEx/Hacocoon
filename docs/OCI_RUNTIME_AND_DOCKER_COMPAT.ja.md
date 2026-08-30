# OCI Runtime と Docker Compatibility

[English](OCI_RUNTIME_AND_DOCKER_COMPAT.md) | **日本語**

Status: **v0.18 repository integrationはroadmap順より先に実装済み。real-host acceptanceは環境依存で別途。**

OCI/container toolingはHacocoon Coreのruntime必須要件ではなく、optionalなplugin/adapter concernです。

## Maintained plugin profile

project-maintained OCI plugin profileではcontainerd + nerdctlを使えます。ただしこれはCore invariantではありません。`HACO_PLUGIN_OCI`未設定ならCoreはcontainerd / nerdctl / Docker CLI / Docker Engineを要求・probeしません。

Docker互換はoptionalです。

```text
Docker CLI / API client
        |
Environment-local docker.sock
        |
socket-activated dockerd
        |
existing containerd where supported
```

genuine Docker CLIを使い、Engine APIが必要な時だけdockerdをon-demandで起動します。Docker互換のためだけに別のHacocoon-managed containerdを立ち上げません。

## v0.18 plugin boundary

Docker/containerd/nerdctl固有の処理は `modules/plugin/oci` のplugin/adapter境界に置きます。repositoryにはsystemd socket/service packagingに加えてEnvironment-local lifecycleのinspection/preparationが実装済みです。

```text
HACO_PLUGIN_OCI=docker haco plugin oci docker status <environment>
HACO_PLUGIN_OCI=docker haco plugin oci docker prepare <environment>
```

`status` はDockerを起動しません。`prepare` はBase/Seed側にgenuine Docker CLI、dockerd、containerd、systemd、docker group、Hacocoon-pinned unitがあることを要求します。unitを検証してからsocket activationを有効化し、package installはせず、既にactiveなvendor Docker daemon/socketを勝手に停止しません。

このcodeはDocker Compatibilityがv0.17と呼ばれていた時点でlandしました。現在の正本ではv0.18へ付け替えますが、runtime behaviorをrollbackしません。

- Host Docker socketをEnvironmentへmountしない
- Host containerd / Incus / Hacocoon control socketを渡さない
- TCP Docker API listenerをdefaultで開かない
- `HACO_PLUGIN_OCI=docker` はplugin driver選択であり、Host Docker daemon authorityを与えない
- `hacocoon-docker.service` がinactiveでも、`/run/docker.sock` 利用時にon-demand起動する設計なので正常
- OCI操作は `haco plugin oci ...`、Base identityは `haco base ...`

## Storage

Dockerとnerdctlが同一containerdのcontent-addressed blobを共有できる場合はありますが、snapshot / unpacked filesystem / writable layer / build cacheまで完全dedupされる保証はありません。

Environment間の容量削減はv0.17 OCI Seed Builder & Btrfs/COWで扱います。trusted Host acquisition/cacheからoffline immutable Seedを作り、通常のIncus/storage-driver cloneで複製し、各Environmentはprivate writable stateを持ちます。writable `/var/lib/containerd` を共有しません。

## Registry

Local OCI Registryはdeferredなoptional infrastructureで、roadmap milestoneを予約しません。network policyとcredentialが許せばdirect upstream pullが通常経路として使え、Seed constructionの必須条件でもありません。

See [`design/oci-seed-and-cow.ja.md`](design/oci-seed-and-cow.ja.md), [`design/docker-compatibility-plugin.ja.md`](design/docker-compatibility-plugin.ja.md), and [`OPTIONAL_LOCAL_OCI_REGISTRY.ja.md`](OPTIONAL_LOCAL_OCI_REGISTRY.ja.md).
