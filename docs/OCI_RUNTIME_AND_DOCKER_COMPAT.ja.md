# OCI Runtime と Docker Compatibility

[English](OCI_RUNTIME_AND_DOCKER_COMPAT.md) | **日本語**

Status: **v0.17 packaging foundationは実装済み。complete plugin integrationとreal-host acceptanceはpending。**

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

## v0.17 plugin boundary

Docker/containerd/nerdctl固有の処理は `modules/plugin/oci` のplugin/adapter境界に置きます。現在あるsystemd socket/service packagingはfoundationであり、v0.17 completeではありません。

- Host Docker socketをEnvironmentへmountしない。
- Host containerd / Incus / Hacocoon control socketを渡さない。
- TCP Docker API listenerをdefaultで開かない。
- `HACO_PLUGIN_OCI=docker` はplugin driver選択であり、Host Docker daemon authorityを与えない。
- OCI操作は `haco plugin oci ...`、Base identityは `haco base ...`。

## Storage

Dockerとnerdctlが同一containerdのcontent-addressed blobを共有できる場合はありますが、snapshot / unpacked filesystem / writable layer / build cacheまで完全dedupされる保証はありません。

Environment間の容量削減はv0.18 OCI Seed Builder & Btrfs/COWで扱います。trusted Host acquisition/cacheからoffline immutable Seedを作り、通常のIncus/storage-driver cloneで複製し、各Environmentはprivate writable stateを持ちます。writable `/var/lib/containerd` を共有しません。

## Registry

Local OCI Registryはdeferredなoptional infrastructureで、roadmap milestoneを予約しません。network policyとcredentialが許せばdirect upstream pullが通常経路として使え、Seed constructionの必須条件でもありません。

See [`18_v0.18_OCI_SEED_AND_COW.ja.md`](18_v0.18_OCI_SEED_AND_COW.ja.md) and [`OPTIONAL_LOCAL_OCI_REGISTRY.ja.md`](OPTIONAL_LOCAL_OCI_REGISTRY.ja.md).
