# OCI Runtime と Docker Compatibility

[English](OCI_RUNTIME_AND_DOCKER_COMPAT.md) | **日本語**

Status: **v0.17 packaging foundationは実装済み。complete plugin integrationとreal-host acceptanceはpending。**

milestone contractは [`17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.ja.md`](17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.ja.md)、Environment間のSeed/COW容量最適化は [`19_v0.19_OCI_SEED_AND_COW.ja.md`](19_v0.19_OCI_SEED_AND_COW.ja.md) を参照してください。

## 方針

Hacocoonの標準OCI runtimeは次です。

```text
containerd  （常駐runtime/content service）
    ^
    |
nerdctl     （通常CLI）
```

Docker互換はoptionalな追加interfaceです。

```text
Docker CLI / Docker API client
          |
          | unix:///var/run/docker.sock
          v
hacocoon-docker.socket
          |
          | socket activation
          v
       dockerd
          |
          | supportedなら既存containerd
          v
     containerd
```

Docker ecosystem互換のため本物のDocker CLIを使えますが、`dockerd`をHacocoonの標準・常駐runtimeにはしません。

## v0.17 plugin boundary

Docker/containerd/nerdctl固有のlifecycleはHacocoon Coreではなくplugin/adapter境界に置きます。

必須方針:

1. `containerd`を標準の常駐OCI serviceにする。
2. 通常CLIは`nerdctl`。
3. Hacocoon製wrapperではなく本物の`docker` CLIを使う。
4. Docker Engine APIが必要でない限り`dockerd`を停止状態にする。
5. supported環境ではEnvironment-local socket activation/on-demand startupを使う。
6. Docker互換のためだけに2個目のHacocoon-managed containerdを起動しない。
7. Host Docker socketをEnvironmentへmountしない。
8. Docker互換を理由にGitHub/cloud/registry/Host authorityを追加しない。

現在repositoryにはdesignとHacocoon専用systemd socket/service packaging foundationがあります。これは**v0.17のpartial implementation**であり、complete plugin実装とは扱いません。

## Socket activation

`hacocoon-docker.socket`がEnvironment-local `/run/docker.sock`をlistenし、Docker Engine API request時に`hacocoon-docker.service`を起動します。

service自身を通常boot targetとして常駐enableしません。socket activationは必要時の**start** mechanismであり、client切断後のidle shutdownを自動保証するものではありません。

Hacocoon socketをenableする前にvendor `docker.socket`等が同じpathをlistenしていないことを確認します。

## Security boundary

Docker daemonは**そのEnvironment内部では強いauthority**を持ちます。そのEnvironmentのDocker socketへaccessできるuserはDocker-managed workloadに対して実質root-equivalentです。

必須rule:

- `/run/docker.sock`はEnvironment-local。
- Host `/var/run/docker.sock`をbind mountしない。
- Host/containerd/Incus/Hacocoon control socketをpassthroughしない。
- 標準ではDocker APIをTCP listenしない。
- socket/group membershipは明示的にする。
- Docker互換からreusable Host credentialを渡さない。
- 外側のsecurity boundaryはIncus Environmentのまま。

## containerd namespaceと容量

Dockerとnerdctlが別containerd namespaceを使っても、同じdaemonのcontent-addressed blobを共有できる場合があります。ただしnamespace metadata、snapshot、unpacked filesystem、writable layer、build cacheなどは追加容量を使い得ます。

したがって「shared containerd content」は説明できますが、完全なimage-store dedupを保証してはいけません。

## v0.19 Environment間の容量削減

Environment間の容量削減はDocker互換ではなく **v0.19 OCI Seed Builder & Btrfs/COW** の責務です。

```text
immutable Seed filesystem
        |
   Incus/storage clone
        +--------+--------+
        |        |        |
 independent   independent
 containerd    containerd
 state A       state B
```

1つのwritable `/var/lib/containerd`を複数Environmentで共有してはいけません。

Btrfs poolではSeed由来の未変更blockを通常のCOW semanticsで共有できる可能性があります。Hacocoon CoreからIncus管理下のBtrfs subvolume pathを直接操作しません。

## OCI pluginとの関係

Base image lifecycleとOCI/container image lifecycleはCLIでも分離します。

```text
haco base ...                    Environment starting point
haco plugin oci ...              OCI/containerd/nerdctl operation
```

v0.15 recommendationはoptional OCI plugin経由でusageをsampleします。Docker Engine互換を完全統合した後はDocker/moby namespaceのimage usageも扱い、同じimmutable digestを二重countしない設計が必要です。

## Completion criteria

v0.17をcompleteとする前にsupported hostで最低限次を確認します。

```text
boot
  containerd: active
  dockerd: inactive
  hacocoon-docker.socket: active

first Docker API request
  -> dockerd becomes active
  -> docker info succeeds
  -> ordinary nerdctl still works
```

現在はpackaging foundationまでです。full plugin lifecycle、Base/Seed integration、real-host validationはfollow-upです。

> **Dockerはoptional compatibility plugin。Hacocoonの標準OCI runtimeはcontainerd + nerdctl。**
