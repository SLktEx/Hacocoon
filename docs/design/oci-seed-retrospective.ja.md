# OCI Seedの記録とIncus native OCIへの移行

[English](oci-seed-retrospective.md) | **日本語**

Status: **historical design record / migration decision**  
Decision date: **2026-08-31**

この文書は、Hacocoonで実装してきたOCI Seed機構を「不要になった古い実装」として消してしまわないための記録です。

OCI Seedは、`containerd + nerdctl` を各Environment内部のworkload runtimeとして使う設計において、OCI imageの重複保存を避け、Btrfs/COWの恩恵をEnvironment cloneへ持ち込むために必要でした。Seed Builder、immutable Seed revision、pin/deletion policy、credential-free harvest、recovery/GCなどは、その制約の中で安全に共有・再利用を成立させるための仕組みでした。

実装の詳細は [OCI Seed Builder & Btrfs/COW Optimization](oci-seed-and-cow.ja.md) をhistorical implementation recordとして残します。この文書は、その設計がなぜ必要だったか、なぜIncus native OCIへ移行するのか、何を捨てて何を残すのかを説明します。

## 1. Seedが解決していた問題

旧構成では、Hacocoon Environmentの内部に`containerd`と`nerdctl`があり、OCI workloadもそのEnvironment内部で実行していました。

```text
Physical Host
└─ Incus
   └─ Environment
      ├─ workspace
      ├─ systemd
      └─ containerd / nerdctl
         ├─ app
         ├─ db
         └─ redis
```

この構成では、同じOCI imageを複数Environmentで使うと、各Environmentが独立したwritable containerd stateを持つため、image contentもEnvironmentごとに保持されやすくなります。

単純に`/var/lib/containerd`を共有すれば容量は減りますが、それではEnvironmentの独立性、破棄可能性、復旧可能性、安全な隔離が壊れます。そこで共有するのはwritable stateではなく、**immutableな事前構築済みEnvironment rootfsのblock**とし、Incus/Btrfs cloneのCOWで再利用する方式がOCI Seedでした。

```text
upstream OCI registry
        |
        v
trusted acquisition/cache
        |
        v
OCI export / stream
        |
        v
Offline Seed Builder
        |
 containerd import/unpack
        |
        v
immutable Incus Seed
        |
        v
Incus clone / Btrfs COW
        |
   independent Environments
```

Seedは単なるimage cacheではなく、次の問題をまとめて解いていました。

- common OCI imageを複数Environmentで物理的に再利用する
- writable containerd rootをEnvironment間で共有しない
- mutable tagではなくimmutable digestを記録する
- Seed build中のdelete raceやpartial publicationをcurrentへ昇格させない
- credentialをSeed Builderやcoding Environmentへ埋め込まない
- 古いSeedを安全側に倒してGCする
- Btrfs/COWの恩恵をIncusのsupported lifecycle経由で得る

## 2. なぜSeedが複雑になったか

Seed自身が悪かったわけではありません。複雑さの主因は、**Incusが管理するEnvironmentの内側に、別のOCI runtime stateを持つ**という二重構造でした。

```text
Incus lifecycle / storage / network
            |
            v
Environment filesystem
            |
            v
containerd lifecycle / image store / snapshotter / network
```

Hacocoonはこの境界を越えて、OCI acquisition、export、import、digest verification、builder lifecycle、publication、pin、delete、recovery、GCを接着する必要がありました。

つまりSeedは、`containerd/nerdctl`をworkload runtimeに選んだ結果生まれたstorage-sharing problemを、安全に解決するための合理的な仕組みでした。しかしIncus自身がOCI application containerをnativeに管理できるなら、この二重runtime構造を維持する必要が薄くなります。

## 3. 2026-08-31の新しいruntime方針

Hacocoonは、**Incus daemonを1つだけ使う**構成を基本とします。nested Incusは前提にしません。

```text
Physical Host
└─ Incus daemon
   ├─ Hacocoon Environment
   ├─ app      (OCI application container)
   ├─ db       (OCI application container)
   └─ redis    (OCI application container)
```

EnvironmentとOCI workloadは同じIncus daemon配下のinstanceとして管理します。必要に応じて同一Hacocoon project / managed networkへ所属させ、instance間通信、port exposure、storage、lifecycleをIncus側へ寄せます。

`haco-host`はIncusへ直接接続しません。Incus socketも渡しません。

```text
haco-host
   |
   | Hacocoon control request
   v
Host controller / broker
   |
   | authorized Incus operation
   v
Incus daemon
```

これにより、OCI workloadの`run/start/stop/exec/network/storage`を、Environment内部の`nerdctl`ではなくIncus instance lifecycleとして扱えるようにします。

## 4. Private registry / ECRの認証境界

Registry認証情報の取得主体は`haco-host`側です。Physical HostへAWSの長期credentialやSSO sessionそのものを配置することを前提にしません。

ECRの場合、概念フローは次です。

```text
haco-host
   |
   | AWS credential / SSO / credential_process
   v
ECR authorization token取得
   |
   | temporary registry credential
   v
Hacocoon control socket
   |
   v
Host controller / broker
   |
   | request authorization
   | credential held only for the pull operation
   v
Incus OCI pull
   |
   v
ECR
```

重要な境界:

- `haco-host`はIncus socketを持たない
- Physical Host controllerはAWS access keyやSSO sessionを恒久保存しない
- `haco-host`から渡すのは、可能な限りregistry用途に限定されたtemporary credentialとする
- temporary credentialはpull完了後に破棄し、diskへ永続化しない
- Incus操作はcontroller/brokerのpolicyを必ず通す

Incusのcredential-helper integrationへ接続する場合も、この境界を崩さず、Hacocoon-managed ephemeral credential provider/helperを介する設計を優先します。

## 5. SeedからIncus native OCIへの責務マッピング

```text
旧OCI Seed                         Incus native OCI方針
────────────────────────────────────────────────────────────
Environment内containerd runtime  -> Incus OCI application instance
Environmentごとのimage store    -> Incus daemon側の共有OCI/image管理
Seed Builder                     -> 原則不要
OCI export/import                -> 原則不要
immutable Seed revision          -> Incus image/cache + immutable OCI identity
Seed cloneによるCOW              -> Incus native storage/image lifecycle
credential-free harvest          -> 中央runtime/cache化により原則不要
seed current/pin                 -> Hacocoon policyでdigest identityを保持
seed deletion tombstone          -> image selection policyとして必要なら継承
seed recover                     -> Incus operation + Hacocoon transaction recoveryへ統合
seed GC                          -> Incus lifecycleを尊重したHacocoon policyへ統合
```

この移行で狙うのは「Seed実装を別名で作り直す」ことではなく、**Seedが必要になった原因そのものを消す**ことです。

## 6. 捨ててはいけないSeedの設計原則

Seed implementationを廃止・縮小しても、以下はHacocoonの設計原則として残します。

### 6.1 Immutable identity

mutable tagは入力として受けてもよいが、再現性やpolicy判断が必要な箇所では`sha256` digest等のimmutable identityへresolveして扱う。

### 6.2 Writable runtime stateを共有しない

容量削減を理由に、独立したEnvironment / workload間で一つのwritable runtime stateを共有しない。各instanceは独立にmutable、deletable、recoverableであること。

### 6.3 Credentialをimage/storageへ埋め込まない

registry credential、credential-helper output、AWS session、login fileをimageやsnapshotへ混入させない。必要なcredentialはoperation中だけ存在させる。

### 6.4 Supported Incus lifecycleを使う

Btrfsを使う場合でも、Hacocoon CoreがIncus-owned subvolumeを直接操作してCOW/GCを実装しない。Incusのsupported image/instance/storage lifecycleへ委譲する。

### 6.5 Destructive GCはfail closed

ownership、dependency、alias、usageが曖昧なimageやinstanceは削除しない。容量最適化より安全なretainを優先する。

### 6.6 Partial operationをcurrentにしない

pull、publication、metadata persistence、policy updateの途中失敗を「成功済み」として扱わない。必要なら明示的なrecovery-required stateへ移行する。

## 7. nerdctl/containerdの位置付け

Incus native OCIをdefault workload runtimeとする方向でも、`nerdctl/containerd`を直ちに世界から完全削除するとは限りません。

Docker/Compose compatibility、BuildKitを含むbuild workflow、特殊なcontainerd toolingが必要な場合には、**compatibility/build backendとして限定利用**できます。

重要なのは、Hacocoonのdefault runtime、image sharing、network、storage lifecycleの中心を`containerd inside Environment`へ戻さないことです。

```text
Hacocoon default runtime
        |
        v
      Incus
     /     \
system     OCI application
container  container

optional compatibility/build path
        |
        v
containerd / nerdctl / BuildKit
```

## 8. Migration acceptance

Seedを本当にretireする前に、少なくとも次をreal-hostで確認します。

- Incus OCI application containerがHacocoonの必要なrun/exec/stop/delete semanticsを満たす
- 複数OCI instanceとEnvironmentをmanaged networkで安全に接続できる
- host/environmentへのport exposureが必要な形で実装できる
- ECR/private registry pullを`haco-host`由来credentialで安全に実行できる
- credentialがHost disk、Incus image、instance rootfsへ残らない
- OCI image/cacheの重複量とBtrfs上のphysical usageがSeed方式より悪化しない、または単純化の価値に見合う
- failure/restart時にpartial pullやorphan resourceを安全にreconcileできる
- Environment間の権限境界をIncus project / controller policyで維持できる

これらが満たされるまでは、既存Seed implementation/docを削除してはいけません。

## 9. Historical preservation rule

`oci-seed-and-cow(.ja).md`は、Seed implementationを削除した後も**historical architecture documentとして残す**ことを推奨します。

将来「なぜこんな複雑なSeedがあったのか」と疑問になったときの答えは次です。

> Hacocoonが各Environment内部に独立したcontainerd runtimeを持ちながら、OCI imageの物理重複を減らし、credential isolationとEnvironment independenceを壊さずBtrfs/COWを利用するためだった。

そしてIncus native OCIへの移行理由は次です。

> 同じIncus daemonがEnvironmentとOCI workloadの両方をnativeに管理できるなら、二重runtime間をHacocoon独自Seed pipelineで橋渡しするより、image/runtime/network/storage lifecycleをIncusへ統合する方が単純で一貫しているため。

Seedは失敗した設計ではありません。**当時のruntime境界に対する解決策であり、その境界自体を取り除けるようになったため役目を終える候補になった**、という位置付けを残します。
