# v0.12 Sandbox Resource Limits

**Status:** first implementation slice 実装済み  
**Compatibility:** pre-1.0。interface、default、provider mapping は互換性なく変更される可能性があります。  
**Implementation:** provider-neutral ResourceBudget、strict CLI parsing、Incus creation-time enforcement/read-back verification、persistence/status、unsupported provider の fail-closed behavior を実装済み。real supported-Incus acceptance は pending。

## 目的

v0.12 では Hacocoon Environment に明示的な ResourceBudget を持たせます。

coding agent や developer tool は Environment 内では広く自由に動けますが、CPU、memory、process/PID count、root filesystem 容量は host/operator が選んだ上限の中でのみ消費できます。

```text
VS Code / coding agent / tool
            |
            v
      Environment
   sandbox 内では自由
 ResourceBudget の範囲内
            |
     ---- boundary ----
            |
        Hacocoon
  Policy / Capability / Audit
            |
   GitHub / AWS / Host
```

Resource limit は Capability ではありません。

```text
Capability
  -> Environment の外へどの authority を使えるか

ResourceBudget
  -> Environment の中でどれだけ resource を使えるか
```

## 実装済み first slice

persistent Environment と ephemeral `haco run` の両方で creation-time budget を指定できます。

```bash
haco create \
  --cpu 4 \
  --memory 8GiB \
  --pids 1024 \
  --root-size 40GiB \
  --workspace . dev

haco run --cpu 2 --memory 4GiB --workspace . -- go test ./...
```

各dimensionは finite value または `unlimited` を受けます。byte size は `B` / `KiB` / `MiB` / `GiB` / `TiB` の明示的なbinary unitを使います。

`8G`、`8GB`、符号付き値、fraction、zero finite value、overflow、trailing garbage、practical upper bound超過は provider 実行前にrejectします。

## ResourceBudget model

Core/public architecture は provider-neutral な概念だけを扱います。

```text
ResourceBudget
  CPU
  MemoryBytes
  PIDs
  RootBytes
```

各resourceは `finite` または `unlimited` です。

未指定dimensionを provider の暗黙defaultには任せません。first sliceでは Hacocoon が explicit `unlimited` effective value に解決し、そのeffective budgetをEnvironment metadataへ保存します。

`haco status` / `haco status --json` からpersisted budgetを確認できます。

## Incus enforcement

Incus providerではfinite limitを **Environment start前** に設定し、read-backして要求値と一致することを確認します。

```text
resolve Workspace
  -> acquire WorkspaceLease
  -> validate ResourceBudget
  -> v0.11 Baseをimmutable revisionへpin
  -> stopped Incus Environmentをcreate
  -> CPU/memory/PID/root limitをapply
  -> read-back verify
  -> Workspace attach
  -> start Environment
  -> metadata persist
```

applyに失敗した場合やread-backが一致しない場合は、requested constrained Environmentの作成成功として返しません。既存のcleanup/recovery semanticsに従います。

Incus native keyやcommandはadapter detailです。Core/public architectureのcompatibility contractにはしません。

real supported-Incus acceptanceでは、CPU / memory / PID / root-sizeが実際のworkloadを制限することと、normal Environment accessからlimitを引き上げられないことを別途確認します。

## Provider boundary

resource enforcementはEnvironment provider adapterが担当します。

```text
Workspace
   |
EnvironmentSpec + ResourceBudget
   |
Environment provider
   +-- runtime.incus
   +-- runtime.ec2 (experimental)
```

requested finite limitをselected providerがenforceできない場合はsilent ignoreせず **fail closed** します。

experimental EC2 providerは現時点で同等のfinite ResourceBudget enforcementをclaimしません。そのためHacocoonはfinite budgetをwrapped providerのcreateより前に拒否し、制限できないEnvironmentをAWS上に作ってから気づく形を避けます。

EC2自体も従来どおりexperimental / disabled by defaultです。

## Default と precedence

first sliceではdirect CLI inputとomissionをdeterministicに解決します。

未指定dimensionの意味はexplicit `unlimited`です。

将来project/user/global defaultを追加する場合は例えば次のprecedenceを取れます。

```text
CLI override
    > project configuration
    > user/global default
    > Hacocoon default budget
```

ただしprovider-ownedな曖昧defaultを再導入しません。

## Persistence

Environment作成時に確定したeffective ResourceBudgetをpersistします。defaultを後で変更しても既存Environmentのbudgetをsilent rewriteしません。

finite requestではproviderが返すcreation metadataも要求budgetと照合します。別budgetをmaterializeして成功扱いすることは許しません。

## Runtime mutation

first sliceはcreation-time budgetを実装し、running Environmentの任意live resizeはscope外です。

## Workspace storageとの区別

`/workspace` はhost-owned Workspaceのmountであり、Environment root filesystemとは別物です。

```text
Host Workspace
    |
    +-- /workspace

Environment root filesystem
    |
    +-- OS
    +-- packages
    +-- caches
    +-- logs
```

`--root-size` はEnvironment root filesystemのbudgetであり、arbitrary host Workspaceのquotaを意味しません。

## Parallel Agent

v0.12は **v0.9 agent-session binding** とv0.10 Agent Host adapterに合成します。

```text
Agent session A -> Environment A -> ResourceBudget A
Agent session B -> Environment B -> ResourceBudget B
Agent session C -> Environment C -> ResourceBudget C
```

per-Environment limitは一つのagentの暴走を抑えますが、host全体のaggregate capacity schedulingまでは扱いません。

## Capabilityとの関係

CPU/memory/PID/root-size selectionをv0.4 Capability serviceには流しません。

```text
Environment configuration
  -> sandboxの形とresource ceiling

Capability policy
  -> sandbox boundaryを越えるprivileged operation
```

Environment内のcoding agentには、自分のhost-enforced ceilingをHacocoon/Incus control-plane経由で引き上げるauthorityを渡しません。

## Baseとの関係

v0.12はv0.11 Basesと合成します。

```text
immutable Base revision
          |
          v
Environment + ResourceBudget
```

custom Baseはguest filesystem/runtime contentsを決めますが、image metadataからhost-selected resource limitを引き上げたり無効化したりできません。

## Failure / recovery

requested finite budgetが正しく適用されたと証明できない場合、successful constrained creationとして返しません。

invalid input、unsupported provider、provider rejection、apply/read-back mismatch、Workspace attach/start failure、persistence failure、createとstate commitの間のcrashは通常のEnvironment recovery問題として扱います。

ownershipが怪しいpotentially-running Environmentを黙って忘れるより、recovery-requiredとして明示的に残す方を優先します。

## Repository validation

first sliceでは次をrepository CIで検証します。

- ResourceBudget normalization / bounds / malformed mode-value combination;
- strict CLI parse;
- finite unsupported providerがinner provider side effect前にfail closedすること;
- Incus finite limitがstart前にapply/read-backされること;
- verify mismatch時にstartせずcleanupへ進むこと;
- fake-Incus CLI E2Eでcreate/run/status/resource-native commandを確認;
- Go version matrix、`go vet`、race detector、docs consistency、release packaging。

これらはreal provider acceptanceの代替ではありません。

## Non-goals

first sliceのscope外:

- AI scheduler / model budget;
- task/DAG orchestration;
- aggregate workstation/cluster scheduling;
- automatic overcommit;
- live autoscaling / live resize;
- Kubernetes-style scheduler semantics;
- host Workspace quota management;
- arbitrary Incus configuration passthrough;
- coding agent自身がlimitを上げるCapability。

## 一文でいうと

> **v0.12 は各 Hacocoon Environment にhost-selected ResourceBudgetを持たせ、requested finite ceilingをsandbox利用開始前に証明できなければfail closedする。**
