# OCI Runtime と Docker Compatibility

[English](OCI_RUNTIME_AND_DOCKER_COMPAT.md) | **日本語**

Status: **packaging foundation は実装済み。Base/Seedへの焼き込みとreal-host acceptanceはpending。**

この資料は、Docker EngineをHacocoonの標準runtimeにせず、Docker前提のdeveloper toolingとの互換性を持たせる方法を定義します。

## 方針

Hacocoonの標準OCI runtimeは次です。

```text
containerd  （常駐runtime/content service）
    ^
    |
nerdctl     （通常CLI）
```

Docker互換は追加interfaceとして扱います。

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
          | --containerd=/run/containerd/containerd.sock
          v
     同じ containerd
```

Docker ecosystemとの互換性のため、Hacocoonのdevelopment Base/Seedには本物のDocker CLIを入れられます。`dockerd`もEngine互換が必要なBase/Seedでは入れますが、標準の常駐runtimeにはしません。

通常のHacocoonの資料やautomationは`nerdctl`を優先します。Docker Engineは、Docker Engine APIや`/var/run/docker.sock`を本当に要求するsoftware向けの互換経路です。

## Runtime rule

1. `containerd`を常駐OCI runtime serviceにする
2. 通常のcontainer CLIは`nerdctl`
3. `docker`はHacocoon製wrapperではなく本物のDocker CLIを使う
4. `dockerd`は通常停止状態にする
5. `/run/docker.sock`へのaccessでsystemdが`hacocoon-docker.service`を起動する
6. 起動した`dockerd`は既存の`/run/containerd/containerd.sock`へ接続する
7. Docker互換のためだけに2個目のprivate/managed containerdを起動しない
8. Docker Engine APIは標準ではEnvironment内のUnix socketにだけ公開する
9. HostのDocker socketをEnvironmentへmountしない

repositoryには`packaging/systemd/`以下にHacocoon専用unitを置きます。packageが所有する`docker.service` / `docker.socket`を上書きせず、Base作成時にvendor側のauto-startをdisableできるよう、別名にしています。

## Socket activation

`hacocoon-docker.socket`が`/run/docker.sock`をlistenし、最初のaccessで`hacocoon-docker.service`を起動します。

service自身には`multi-user.target`から起動される`[Install]`設定を持たせません。enableするのはsocketだけです。これにより通常bootでDocker Engineが2個目の常駐control planeになるのを避けます。

socket activationは**必要時に起動する仕組み**であって、idle時に自動停止する仕組みではありません。将来idle shutdown policyを追加することはできますが、client切断後にsystemdが自動で`dockerd`を止めるとは扱いません。

Hacocoon socketをenableする前に、Base/Seed provisioningはvendorの`docker.socket`など別processが`/run/docker.sock`をlistenしていないことを確認しなければなりません。

## containerd namespace と容量

Docker Engineは通常、containerd上で独自namespace（一般的には`moby`）を使います。容量削減のためだけにHacocoon/nerdctlのimage metadataまで同じnamespaceへ混ぜる必要はありません。

同じcontainerd daemonを使えば、高位のimage/container metadataをnamespaceで分離したままでも、content-addressedなOCI blobはcontainerdのcontent storeで共有できます。

ただし**全byteが必ずdedupされるという意味ではありません**。namespace固有metadata、snapshot/unpack済みfilesystem、writable layer、build cache、Docker固有stateなどは追加容量を使う場合があります。そのためHacocoonは「同じcontainerd contentを共有する」と表現し、「Dockerとnerdctlでimage容量が完全に0重複になる」とは保証しません。

Environment間の容量削減はv0.13A Seed/COW設計を使います。immutable Seed filesystemをIncus/storage driverのclone semanticsで共有し、複数Environmentから同じwritable `/var/lib/containerd`を共有してはいけません。

## Base / Seedへの組み込み

Docker互換はEnvironment起動時に毎回package installするのではなく、immutable development Base/Seedへ焼き込みます。

この機能を有効にするBase/Seed buildは、次を満たす必要があります。

1. supportedなstandalone `containerd` serviceと`nerdctl`をinstallする
2. 本物のDocker CLIをinstallする
3. Engine互換を提供するBase/Seedだけ`dockerd`もinstallする
4. non-root Docker API accessが必要ならEnvironment内の`docker` groupを作る
5. Hacocoonのsocket/service unitをinstallする
6. vendorがauto-startする`docker.service` / `docker.socket`をdisableする
7. `hacocoon-docker.service`ではなく`hacocoon-docker.socket`だけをenableする
8. immutable Base/Seed publish前に`dockerd`が停止していることを確認する
9. 起動したdaemonが`/run/containerd/containerd.sock`を使うことを確認する
10. registry credentialやHost control socketをimageへ取り込まない

現在のrepositoryには、これらのpackage/unitをofficial Hacocoon imageへ自動で焼き込むv0.11 custom Base builder / v0.13A Seed publisherがまだありません。そのため今回追加するunitは**packaging foundation**であり、現在のvanilla Ubuntu Baseから作った全Environmentですぐ`docker`が使えるというimplementation claimではありません。

## Security boundary

Docker daemonは**そのEnvironment内部では強いauthority**を持ちます。Docker socketへaccessできるuserは、そのEnvironmentのDocker daemonと管理対象containerを実質的に制御できます。

必須rule:

- `/run/docker.sock`はEnvironment-local
- Hostの`/var/run/docker.sock`をbind mountしない
- Host/containerd/Incus/Hacocoon control socketをDocker互換経路へ渡さない
- 標準ではDocker APIをTCP listenしない
- socketは`0660`、group membershipは明示的にする
- Environment内`docker` group membershipは、そのEnvironment内ではroot-equivalent authorityとして扱う
- Docker互換を理由にGitHub/cloud/registry/Host credentialを付与しない
- 外側のsecurity boundaryはIncus Environmentのまま

## Telemetryとの関係

OCI Seed usage telemetryは現在、通常の`nerdctl images`をsampleします。Docker Engine互換をBaseへ焼き込んだ後は、Docker/`moby` namespaceで使われたimageもsampleするか、より低位のcontainerd viewからusageを収集する必要があります。

同じimmutable digestが両方の経路に見えても、Seed recommendationでは重複countしません。Docker互換を使っただけで同じOCI contentの推薦weightが2倍になってはいけません。

## Acceptance

repository-levelのpackaging foundationでは次を確認対象にします。

- `/run/docker.sock`を`0660`でlistenするsocket unit
- `dockerd -H fd://`とexternal `/run/containerd/containerd.sock`を使うservice unit
- `hacocoon-docker.service`をboot時に直接enableするtargetを持たない
- shared contentとtotal-storage dedupを区別してdocumentする
- Host Docker socket passthroughを禁止する

Base/Seed buildができた後のreal acceptanceは次です。

```text
boot
  containerd: active
  dockerd: inactive
  hacocoon-docker.socket: active

最初のDocker API request
  -> dockerd becomes active
  -> docker info succeeds
  -> ordinary nerdctl still works
```

supported Incus imageでこのlifecycleを検証できるまではreal-host acceptance pendingです。

> **Dockerは互換interface。Hacocoonのruntimeはcontainerd + nerdctl。**
