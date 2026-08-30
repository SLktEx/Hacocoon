# Optional OCI Plugin と Docker Compatibility

[English](OCI_RUNTIME_AND_DOCKER_COMPAT.md) | **日本語**

Status: **plugin境界/driver compositionは実装済み。v0.17 Docker Engine/Base統合とreal-host acceptanceはpending。**

Docker milestoneは [`17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.ja.md`](17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.ja.md)、Environment間のSeed/COWは [`19_v0.19_OCI_SEED_AND_COW.ja.md`](19_v0.19_OCI_SEED_AND_COW.ja.md) を参照してください。

## Coreの方針

Hacocoon Coreにはcanonical OCI runtimeを持たせません。Coreは`containerd`、`nerdctl`、Docker CLI、Docker Engineを必須にしません。

OCI toolingが一切ないinstallationでもEnvironment lifecycle、隔離、実行、connection、policy/approval、eventは成立しなければなりません。

OCI/container固有機能は`modules/plugin/oci`に置き、明示的にenableします。

```text
HACO_PLUGIN_OCI=nerdctl
HACO_PLUGIN_OCI=docker
```

`HACO_PLUGIN_OCI`未設定ならOCI pluginはcompositionされず、Coreはcontainer toolingをprobe・要求しません。

## Project-maintained OCI profile

OCI workflowを使う利用者向けにHacocoon projectが主に想定するprofileは:

```text
containerd  （Environment-local runtime/content service）
    ^
    |
nerdctl     （このprofileの通常CLI）
```

です。これは**profile選択**でありCore invariantではありません。別のBase/Seedは別のOCI stack、またはOCI toolingなしを選べます。

Docker互換が必要なprofileでは追加で:

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
          | existing Environment-local containerd where supported
          v
     containerd
```

という経路を使えます。

## Plugin driver選択

`HACO_PLUGIN_OCI`はoptional pluginのOCI inventory driverを選びます。

- `nerdctl`: managed Environment内で`nerdctl images ...`
- `docker`: 本物のDocker CLIでinventory

driver選択はbinary install、registry credential付与、任意のHost Docker daemon authorityを意味しません。

Base lifecycleとOCI workload-image lifecycleは分離します。

```text
haco base ...                    Environment starting point
haco plugin oci ...              optional OCI/container operation
```

## v0.15 / v0.16 plugin ownership

OCI Seed telemetry/recommendationとOCI deletion/tombstone stateは`modules/plugin/oci`所有です。

- v0.15: future Seed候補となるimmutable OCI identityを選ぶ
- v0.16: explicit deletion/tombstoneを扱う
- v0.19: physical immutable Seed build/publish/GCとCOWを扱う

plugin無効時はoptional OCI commandが使えないだけで、Core Environment lifecycleはvalidなままです。

## v0.17 Docker compatibility rule

Docker Engine compatibilityを提供するBase/Seedでは:

1. Hacocoon製imitate CLIではなく本物のDocker CLIを使う
2. Engine APIが必要になるまで`dockerd`を停止
3. `/run/docker.sock`はEnvironment-local
4. supportedな場合はsocket activation/on-demand startup
5. compatibilityのためだけに2個目のHacocoon-managed containerdを起動しない
6. Host Docker socketをEnvironmentへmountしない
7. Docker互換からGitHub/cloud/registry/Host authorityを得られない

plugin-owned unitは次に置きます。

```text
modules/plugin/oci/packaging/systemd/
```

serviceを通常boot targetとしてenableせず、socketでon-demand startします。socket activationはidle shutdownを意味しません。

## Security boundary

Docker daemonはそのEnvironment内部では強いauthorityを持ちます。

- `/run/docker.sock`はEnvironment-local
- Host Docker/containerd/Incus/Hacocoon control socket passthrough禁止
- 標準ではTCP Docker API listenなし
- socket/group membershipは明示
- OCI plugin enablementでreusable Host credentialを付与しない
- 外側のsecurity boundaryはHacocoon Environment

## Storage

project-maintained profileでは、Dockerとnerdctlが異なるcontainerd namespaceを使いつつcontent-addressed blobを共有できる場合があります。ただしmetadata、snapshot、unpack済みfilesystem、writable layer、build cacheまで完全dedupされる保証はありません。

Environment間の容量削減はv0.19の責務です。一つのwritable `/var/lib/containerd`を複数Environmentで共有してはいけません。

## Acceptance

Core acceptanceにはplugin-disabled caseを含めます。

```text
HACO_PLUGIN_OCI unset
containerd absent
nerdctl absent
Docker absent

-> Core Environment lifecycleが成立
```

plugin側では明示driver選択を検証し、full v0.17ではsupported Base/Seed上のEnvironment-local on-demand Docker Engine lifecycleを追加検証します。

> **OCI toolingはoptional。`containerd + nerdctl`はproject-maintained profileであってHacocoon Core要件ではありません。**
