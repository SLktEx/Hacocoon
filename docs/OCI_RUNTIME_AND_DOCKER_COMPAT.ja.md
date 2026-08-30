# OCI Runtime と Docker Compatibility

[English](OCI_RUNTIME_AND_DOCKER_COMPAT.md) | **日本語**

Status: **v0.17 packaging foundation実装済み。complete plugin lifecycle、Base/Seed bake-in、real-host acceptanceはpending。**

versioned contractは [`17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.ja.md`](17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.ja.md) です。

## 方針

標準OCI runtime:

```text
containerd  （常駐runtime/content service）
    ^
    |
nerdctl     （通常CLI）
```

Docker compatibilityはoptionalです。

```text
Docker CLI / API client
        |
Environment-local /run/docker.sock
        v
hacocoon-docker.socket
        |
 socket activation
        v
      dockerd
        |
 existing containerd
```

本物のDocker CLIはecosystem compatibility用に利用できます。`dockerd`はEngine APIが必要な場合だけ使い、Hacocoon標準の常駐runtimeにはしません。

## Plugin boundary

Docker/nerdctl固有のpackage provisioning、socket activation、daemon lifecycle、namespace処理、compatibility validationはCoreではなくoptional plugin/adapter側の責務です。

Coreはprovider-neutralなEnvironment lifecycle/executionを維持します。

## Runtime rules

1. `containerd`をlong-lived OCI runtime/content serviceにする
2. normal container CLIは`nerdctl`
3. `docker`はHacocoon wrapperではなくgenuine Docker CLI
4. `dockerd`はdefault停止
5. `/run/docker.sock` accessでHacocoon socket/serviceからon-demand起動可能
6. supported integrationではexisting `/run/containerd/containerd.sock` を利用
7. compatibilityだけのために2個目のHacocoon-managed containerdを作らない
8. Docker APIはEnvironment-local Unix socketだけをdefaultにする
9. Host Docker/containerd socketをEnvironmentへmountしない

`packaging/systemd/` のunitはv0.17 **foundation**であり、full plugin completionの証明ではありません。

## containerd namespace / storage

Dockerが`moby`等の別namespaceを使っても、同じcontainerd daemonのcontent-addressed blobを共有できる場合があります。ただしsnapshot、writable layer、build cache等まで完全dedupされる保証ではありません。

Environment間の容量削減はv0.19が正本です。immutable SeedをIncus/storage-driver cloneで利用し、複数Environmentで同じwritable `/var/lib/containerd` を共有しません。

## Base / Seed integration

complete integrationでは、containerd+nerdctl、genuine Docker CLI、必要時のみdockerd、Hacocoon socket/service unitをimmutable Base/Seedへprovisionします。vendor Docker auto-startと衝突させず、publish前にdockerd停止とcredential/control-socket非混入を確認します。

現在はcomplete Base build / v0.19 Seed publisherがまだないため、official imageへの自動bake-inは未完成です。

## Security boundary

Docker daemonはそのEnvironment内部では強いauthorityを持ちます。

- Docker socketはEnvironment-local
- Host Docker/containerd/Incus/Hacocoon control socket passthrough禁止
- Docker APIのTCP公開はdefault禁止
- group/socket authorityはexplicit
- Docker compatibilityを理由にGitHub/cloud/registry/Host credentialを与えない
- 外側のsecurity boundaryはIncus Environment

## Telemetry interaction

v0.15 telemetryは現在`nerdctl images`を中心にsampleします。Docker/moby namespaceをfull integrationする場合は同じimmutable OCI contentをdouble-countしない形でusage collectionを拡張します。

v0.16 deletion/tombstoneはCLI経路に依存せずexact immutable identityで安全性を維持します。

## Acceptance

repositoryではsystemd unit/static verificationまで。complete v0.17 acceptanceではsupported Environmentで次を確認します。

```text
boot
  containerd: active
  dockerd: inactive
  hacocoon-docker.socket: active

first Docker API request
  -> dockerd starts on demand
  -> docker info succeeds
  -> nerdctl still works
```

> **Dockerはoptional compatibility plugin。standard runtime directionはcontainerd + nerdctl。**
