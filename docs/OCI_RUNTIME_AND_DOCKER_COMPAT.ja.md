# Optional OCI Plugin と Docker Compatibility

[English](OCI_RUNTIME_AND_DOCKER_COMPAT.md) | **日本語**

Status: **OCI plugin境界とdriver選択は実装済み。Base/Seedへの焼き込みとreal-host acceptanceはpending。**

この資料は、Hacocoon Coreとは独立した**任意のdeveloper-tooling plugin**としてOCI/Docker周りを定義します。

## Coreの方針

Hacocoon Coreには標準OCI runtimeを持たせません。

Coreは次を必須要件にしません。

- `containerd`
- `nerdctl`
- Docker CLI
- Docker Engine / `dockerd`
- OCI registry
- OCI image telemetry / Seed promotion

これらが一切入っていない環境でも、Environmentの作成・隔離・接続・実行・削除、policy/approval、eventなどのCore機能は成立しなければなりません。

OCI機能は`modules/plugin/oci`に置き、明示的に有効化します。

```text
HACO_PLUGIN_OCI=nerdctl
HACO_PLUGIN_OCI=docker
```

`HACO_PLUGIN_OCI`が未設定ならOCI pluginはcompositionされず、Coreはcontainer CLIの存在確認すらしません。

plugin側のCLIは次に置きます。

```text
haco plugin oci status
haco plugin oci seed sample
haco plugin oci seed recommend
```

`haco image`はHacocoon EnvironmentのBaseを扱うCore commandなので残します。OCI workload imageの管理commandではありません。

## Hacocoonが用意するOCI profile

OCI pluginを使う場合、Hacocoon projectが主に想定するprofileは次です。

```text
containerd  （Environment内のruntime/content service）
    ^
    |
nerdctl     （このprofileの通常CLI）
```

ただしこれは**plugin/profileの選択**であって、Hacocoon Coreの不変条件ではありません。

利用者のBase/Seedは別のOCI stackを選んでもよく、OCI stack自体を持たなくても構いません。

Docker ecosystemとの互換性が欲しいprofileでは、本物のDocker CLIも追加できます。Docker Engine APIまで必要なsoftware向けには、さらにoptionalなsocket activation経路を使えます。

```text
Docker CLI / Docker API client
          |
          | unix:///run/docker.sock
          v
hacocoon-docker.socket
          |
          | socket activation
          v
       dockerd
          |
          | --containerd=/run/containerd/containerd.sock
          v
     same containerd
```

これらのunitはCoreの`packaging/`ではなく、`modules/plugin/oci/packaging/systemd/`に置きます。

## Plugin driver選択

現在の`HACO_PLUGIN_OCI`は、OCI Seed telemetryでimage inventoryを取るCLI driverを選びます。

- `nerdctl`: Environment内で`nerdctl images ...`を実行
- `docker`: Environment内で本物の`docker images --digests ...`を実行

driverを選んでも、Hacocoon Coreがそのbinaryをinstallするわけではありません。必要なtoolはBase/Seedまたはoperatorが提供します。

## Docker compatibility rule

OCI pluginのDocker Engine compatibility profileを使う場合だけ、次を適用します。

1. そのprofileでは`containerd`を常駐OCI serviceとして使う
2. `docker`はHacocoon製wrapperではなく本物のDocker CLIを使う
3. `dockerd`は通常停止状態にする
4. `/run/docker.sock`へのaccessで`hacocoon-docker.service`を起動する
5. 起動した`dockerd`は`/run/containerd/containerd.sock`へ接続する
6. Docker互換のためだけに2個目のprivate containerdを起動しない
7. Docker APIはEnvironment-local Unix socketだけに公開するのを標準とする
8. HostのDocker socketをEnvironmentへmountしない

`hacocoon-docker.service`自身はboot targetから直接enableしません。enableするのはsocketだけです。

socket activationは必要時にdaemonを起動する仕組みであり、client切断後に自動停止する仕組みではありません。

## containerd namespace と容量

project-maintained profileでDocker Engineとnerdctlが同じcontainerd daemonを使う場合、Dockerは`moby`など別namespaceを使って構いません。

高位のmetadataを分離したままでも、content-addressedなOCI blobはcontainerd content storeで共有できます。

ただし全byteの完全dedupを保証するものではありません。namespace固有metadata、unpack済みsnapshot、writable layer、build cache、Docker固有stateなどは追加容量を使います。

Environment間の容量削減はSeed/COW側の責務です。複数Environmentで1つのwritable `/var/lib/containerd`を共有してはいけません。

## Base / Seedへの組み込み

OCI対応Base/Seedが、どのplugin profileを提供するかを決めます。Environment起動時にCoreが毎回package installする設計にはしません。

`nerdctl` profileでは例えば次を焼き込めます。

- standalone `containerd`
- `nerdctl`
- 必要なら本物のDocker CLI

Docker Engine compatibility profileではさらに次を追加できます。

- `dockerd`
- 必要ならEnvironment-local `docker` group
- `modules/plugin/oci/packaging/systemd/hacocoon-docker.socket`
- `modules/plugin/oci/packaging/systemd/hacocoon-docker.service`

provisioningではvendorのDocker auto-startと競合しないようにし、`hacocoon-docker.socket`だけをenableします。immutable Base/Seed publish前に`dockerd`が停止していること、registry credentialやHost control socketをimageへ取り込んでいないことも確認します。

## Security boundary

Docker daemonは**そのEnvironment内部では強いauthority**を持ちます。Docker socketへaccessできるuserは、そのEnvironment内ではroot-equivalentとして扱います。

必須rule:

- `/run/docker.sock`はEnvironment-local
- Hostの`/var/run/docker.sock`をbind mountしない
- Host側のcontainerd / Incus / Hacocoon control socketを渡さない
- 標準ではDocker APIをTCP listenしない
- socketは`0660`、group membershipは明示的にする
- OCI pluginを有効にしてもGitHub/cloud/registry/Host credentialを自動付与しない
- 外側のsecurity boundaryはHacocoon Environmentのまま

## Telemetry / Seed recommendation

OCI Seed usage telemetryはplugin側の責務です。選択されたdriverがworkload image usageをsampleし、CoreのEnvironment stateとは別のplugin stateへ保存します。

同じimmutable digestが複数経路で見えても、Seed recommendationで二重countしてはいけません。

## Acceptance

Core acceptanceでは、まず次を確認します。

```text
HACO_PLUGIN_OCI unset
nerdctl absent
Docker absent
containerd absent

-> OCI pluginなしでもCore compositionが成立する
```

plugin acceptanceではdriverごとに確認します。

```text
HACO_PLUGIN_OCI=nerdctl
  -> haco plugin oci status が nerdctl を返す
  -> Seed sample は nerdctl だけを使う

HACO_PLUGIN_OCI=docker
  -> haco plugin oci status が docker を返す
  -> Seed sample は Docker CLI だけを使う
```

Docker Engine compatibility profileのreal acceptanceは、Base/Seed build path完成後に次を確認します。

```text
boot
  containerd: active
  dockerd: inactive
  hacocoon-docker.socket: active

最初のDocker API request
  -> dockerd becomes active
  -> docker info succeeds
```

> **OCI toolingはoptional。`containerd + nerdctl`はHacocoonが用意するplugin profileであって、Core要件ではありません。**
