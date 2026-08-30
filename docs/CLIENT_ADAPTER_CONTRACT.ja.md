# 再利用可能な Client Adapter Contract

Hacocoon は `github.com/SLktEx/Hacocoon/pkg/clientadapter` を通じて、client-neutral な adapter API を公開します。

このcontractは、VS Code固有の挙動へ依存せずにEnvironmentを作成/再利用し接続したいIDE、browser/code-server client、CLI tool、JetBrains adapter、将来のclient向けです。

これは **client integration boundary** であり、新しいUIでもauthorization bypassでもありません。Policy/Capability approvalはtrusted authority pathに残り、interaction eventは `pkg/interaction` からread-onlyに観測します。

## Public operation

| Operation | 役割 |
| --- | --- |
| `NewLocal` | local Hacocoon Hostへadapterを開く |
| `Ensure` | Environment/Workspace/access-modeが完全一致すれば再利用し、それ以外は新規作成 |
| `Status` | client-safeなEnvironment stateを取得 |
| `Connections` | Hacocoon/runtime stateから現在のclient connectionをreconcile |
| `PrepareSSH` | clientが渡す **public key** のみをinstallし、loopback-only SSHを作成 |
| `Forward` | loopback-only TCP forwardingを作成 |
| `Revoke` | managed SSH/forward connectionを1つrevoke |
| `Delete` | EnvironmentとHacocoon lifecycle stateを削除 |
| `InteractionBatch` | minimized/resumableな `pkg/interaction` eventを読む |

adapterへ返すEnvironment内Workspace pathは常に次です。

```text
/workspace
```

Host側source pathはlocal lifecycle/reuse判定用に `source_workspace` として別途返します。HacocoonがこのHost pathをremote serviceへ自動送信することはありません。

## Ownership model

### Hacocoonが持つもの

- Environment identity / lifecycle
- Workspace lease enforcement
- Incus/provider connection setup / cleanup
- loopback-only proxy enforcement
- Environmentへinstallするmanaged SSH **public-key** marker
- reconnect可能なconnection metadata
- trusted Policy/Capability approval / execution
- read-only interaction event source

### clientが持つもの

- SSH private key
- IDE/project configuration
- client process / launch behavior
- UI、notification、Browser Notification permission
- cross-session dedupが必要な場合のinteraction cursor/event ID persist

`pkg/clientadapter` はprivate keyを受け取りません。`PrepareSSH` が受け取るのはpublic-key textだけです。対応するprivate keyはclient自身がloopback endpointへ接続するときに直接使います。

## Fail-closed reuse

`Ensure` が既存Environmentを再利用できるのは、次の両方が完全一致する場合だけです。

1. canonical Host Workspace path
2. requested read-only/read-write access mode

異なるWorkspaceや異なるauthorityを持つEnvironmentをsilentに使い回しません。adapterは `ErrAlreadyExists` を返します。

create成功後のverificationが失敗した場合は新規Environmentをcleanupします。cleanupできたか証明できない場合は `ErrRecoveryRequired` を返します。

## Connection security

underlying providerはmanaged proxyをloopback-onlyに制限しています。`pkg/clientadapter` でもprojection時に再検証し、返却/reconcileされたconnectionのHostがloopbackでなければrejectします。

SSHではさらに次を要求します。

- connection kindが `ssh`
- target portが `22`
- validなloopback Host / host port

TCP forwardingではrequested target portとの一致を検証します。新規connectionがcontract違反ならadapterがrevokeし、revokeを証明できなければrecovery-requiredです。

これによりprovider driftでlocal-only connectionがLAN/WAN listenerへ広がってもclient adapterが誤って受け入れません。

## Reconnect / process restart

client processはHacocoon connectionの正本ではありません。restart後は次を呼べます。

1. `Status(environment)`
2. `Connections(environment)`
3. `InteractionBatch(lastOffset, ...)`

Incus-backed connection reconciliationはmanaged proxy metadataをruntimeから再構成するため、reconnectするclientはin-memoryなVS Code sessionなしでもcurrent endpointを発見できます。そのconnectionを再利用するか、明示的にrevokeできます。

## VS Codeを使わないgeneric proof

通常の `haco` CLIがすでに同じgeneric client boundaryを使います。VS Code extensionやVS Code protocolは不要です。

```sh
haco create --workspace "$PWD" demo
haco ssh demo --public-key "$HOME/.ssh/id_ed25519.pub" --host-port 2222
ssh -i "$HOME/.ssh/id_ed25519" -p 2222 root@127.0.0.1
```

client shellや別adapter processをrestartした後も確認できます。

```sh
haco status demo --json
haco connections demo --json
```

client connectionだけをrevokeする場合:

```sh
haco unforward demo ssh-2222
```

Environment lifecycleを終える場合:

```sh
haco delete demo
```

private keyを使うのは通常の `ssh` clientであり、Hacocoonではありません。

## code-server / 他IDE

code-server、JetBrains remote tooling、その他IDEはEnvironment内の通常software + client-owned launch/connection adapterとして扱えます。Hacocoon Coreへ `code-server` / `jetbrains` / `vscode` conditionalを追加する必要はありません。

web workloadではclientがworkload portへのloopback forwardingを準備できます。browser exposure、authentication、URL handling、UIはclient責務です。

## Interaction event

`InteractionBatch` はclient-neutral notification用のpublic `pkg/interaction` contractを返します。eventを読むだけではcapabilityの承認・実行は起きません。

minimization、resume cursor、Browser Notification mappingは [`INTERACTION_EVENTS.ja.md`](INTERACTION_EVENTS.ja.md) を参照してください。

## Public compatibility boundary

`pkg/clientadapter` のexported signatureはpackage-owned DTOとpublic error sentinelだけを使い、`internal/core` typeを公開しません。provider/runtimeやIDE固有detailはadapter boundaryの内側に残します。

pre-1.0のためbreaking changeはまだあり得ますが、client固有branchingはHacocoon Coreではなくclient adapter側へ置きます。
