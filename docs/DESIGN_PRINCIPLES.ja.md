# Hacocoon 設計原則

[English](DESIGN_PRINCIPLES.md) | **日本語**

Status: authoritative cross-cutting design principles.

この文書は、Hacocoonの実装やEnvironment backendが増えても維持する設計上の原則を定義します。将来予定している機能がすべて実装済みであることを示す文書ではありません。

## 1. Hacocoonは特定runtimeではなくEnvironmentを扱う

Hacocoonは、隔離された開発Environmentを作成・操作するための仕組みです。Incus wrapperではありません。

Incus system containerは、Linux互換性、systemd、起動速度、storage効率、運用の単純さのバランスが良いため最初のbackendとして採用します。ただし、Incusを恒久的なCore依存にはしません。

将来、必要性が確認できれば、Environment backendとして次のような実装を追加できます。

- Incus container
- Incus VM
- microVM
- Kubernetesなどscheduler-backedなEnvironment
- remote / SSH-backed host
- その他のlocal / remote isolation technology

CoreはEnvironment lifecycleと必要なcapabilityを表現し、backend固有の仕組みはEnvironment境界の内側に閉じ込めます。

## 2. Environmentの中はuntrusted、境界の外はtrusted

Environment内で動くcommand、developer tool、build system、dependency、coding agentは、host authorityに対してuntrustedとして扱います。

Hacocoonのhost/control planeは、その境界を強制するtrusted側です。privileged credential、runtime control、policy decision、host-side capabilityは、明示的に仲介されない限りEnvironmentへ渡しません。

目的はagentが自分のEnvironmentを書き換えることを止めることではありません。Environment内の通常権限が、気付かないうちにhost authorityへ昇格することを防ぐのが目的です。

## 3. AgentにはEnvironment内で自由に動いてもらう

Hacocoonは、強く制限されたapplication sandboxよりも、実際の開発機として使える高い自由度を優先します。

Backendが安全に境界を維持できるなら、agentにEnvironment内の`root`を与えても構いません。たとえば次の操作を許容します。

- packageのinstall
- systemd serviceの起動・変更
- 任意コードのcompile
- Environment filesystemの変更
- 選択したBaseやoptional pluginが提供するcontainer/developer toolingの利用
- 設定されたresource limit内でのCPU、memory、process、disk消費

Environment内の`root`とhost authorityは同じではありません。Backendはこの区別を維持しなければなりません。

## 4. 境界をまたぐauthorityを最小化する

開発上の便利さのために、hostのambient authorityをEnvironmentへ露出してはいけません。

Defaultでは、次のようなものをEnvironmentへ渡しません。

- host HOME
- `~/.ssh`、`~/.aws`、cloud credential、GitHub token
- Incus、Docker、containerdなどのhost control socket
- Hacocoon control state
- 任意のhost filesystem path
- 無制限のprivileged deviceやruntime configuration

Hostや外部serviceのauthorityを必要とする操作は、明示的なPolicy / Approval / Capability境界を通し、必要最小限のauthorityだけを使います。

## 5. Defaultのsecurity targetは実用的なcontainmentであり、VM同等を名乗らない

Hacocoonは、自分が実際に提供できるsecurity boundaryを明確にします。

Incus system-container backendでは、host kernel、Incus daemon、trustedなHacocoon host processはtrusted computing baseです。Linux kernel exploit、Incus/container escape、trusted host control plane自体のcompromiseに対する防御はHacocoonの保証対象ではありません。

これは意図したtrade-offです。Default backendでは、別kernelによる分離よりも、速い起動、低memory overhead、安価なclone、高いLinux互換性を優先できます。

より強い分離が必要な場合は、VMやmicroVMのようなbackendを追加・選択できます。その場合でもHacocoon Coreの意味は変えません。

## 6. Isolationの強さはbackendの責務

Coreは「すべてのEnvironmentがhost kernelを共有する」「すべてのEnvironmentがVMである」といった仮定を持ってはいけません。

Backendは、自分が実際に提供できるcapabilityと保証を表現します。要求された保証を満たせない場合、Hacocoonはすべてのbackendが同等であるふりをせず、操作を拒否します。

そのため、backend選択はpolicyや利用者の目的に応じて変えられます。通常開発では軽量containerを使い、より危険なworkloadではVM/microVM backendを使う、といった選択が可能です。

Core全体にbackend名による`if`分岐を増やしません。複数の実装が実際に存在して必要性が確認できた時点で、capability-orientedな安定境界へ一般化します。

## 7. Workspaceは作業データであり、agentから守るvaultではない

WritableなWorkspaceは、意図的にagentから書き込み可能です。Read-write accessを持つEnvironmentでは、agentがWorkspace内のfileを変更・削除できます。

Hacocoonのcontainment目標は、明示的に選択したWorkspaceと許可されたcapabilityを越えたblast radiusを小さくすることです。Workspaceそのものをagentから保護することは別の要件です。必要ならread-only access、version control、snapshot、上位のreview/recovery workflowを利用します。

Hostは、Workspace mountを無関係なhost dataへのaccessへ勝手に拡張してはいけません。

## 8. 外部authorityはbrokerし、狭く、監査可能にする

可能な限りcredentialはhost側に保持します。

次の形を優先します。

```text
untrusted Environment
       |
       | request
       v
Policy / Approval / Capability
       |
       | narrow authorized operation
       v
host or external service
```

Reusable credentialをEnvironmentへcopyする方法は避けます。

Capability requestは、privileged executionの直前にstale approval、target変更、confused-deputyを検出できるだけのidentity/stateへbindします。

## 9. Trust boundaryではfail closed

Security-sensitiveな前提を検証できない場合、Hacocoonはprivileged operationを拒否します。

例:

- policyを評価できない
- 必要なapprovalが存在しない、またはstale
- runtime/network/profile configurationがdriftしている
- requested resource limitを強制できない
- approval後にrepositoryやremote identityが変わった
- cleanupに失敗し、安全な状態を証明できない

便利機能のfailureはgraceful degradationして構いません。Trust-boundaryのfailureを、黙ってより広いauthorityへ変えてはいけません。

## 10. 軽くて使い捨てやすいこと自体が機能

Hacocoonは、隔離Environmentを特別に危険な作業だけでなく普段から使えるほど安くすることを目指します。

高速なcreate、低いidle cost、copy-on-write storage、再利用可能なimmutable Base/Seed、deterministic cleanupはarchitecture上の目標です。Isolationが安ければ、developerやagentは普段から隔離を利用できます。

Security mechanismは境界を守るべきですが、Environment内でagentが普通のdeveloperのように動くことまで不必要に妨げるべきではありません。

## 11. Core / Standard / Pluginを分ける

「多くの利用者が使う」ことと「Coreである」ことは同じではありません。

### Core

Coreが所有するのは、Hacocoonの意味やsecurity contractそのものを決める安定した契約と制御です。

- genericなEnvironment lifecycle / Execution
- Policy / Approval / Capabilityの意味
- Environment境界を越えるrequest modelとcontroller contract
- provider/backendがそのdecisionを実行するためのcontract
- 実装が変わっても一貫していなければならないstate / audit semantics

Coreは「何を制御するか」を定義し、「どの具体技術で制御するか」は定義しません。

### Standard

Standardは、通常のHacocoon配布物として公式に提供し、多くの利用者がそのまま使うことを想定するdefault implementationです。ただし、Hacocoonの意味そのものではなく、Core contractを満たす交換可能な実装です。

たとえば次のようなものをStandardに置けます。

- 現在のIncus Environment backend
- 将来実装するproject-maintainedなdefault egress proxy / enforcer
- 通常配布するdefault notification / interaction adapter

Standardはdefaultで有効でも構いませんが、Coreがその実装技術へ恒久的に依存してはいけません。

### Plugin

Pluginは、なくても一般的なHacocoonとして成立するoptional integration、代替実装、特定workload向け機能です。

GitHub、containerd、nerdctl、Docker、OCI registry、cloud CLI、特定IDE、特殊なenterprise proxyなどは、Hacocoonが対応しているという理由だけでCore requirementにはしません。

実用上の判定基準は次です。

> そのcomponentを外しても、通常配布のHacocoonは一般用途として成立するか？

成立するなら通常はPluginです。一般利用で1つの実装がほぼ必要だが差し替え可能であるべきならStandardです。削除や変更によってHacocoonのproduct semantics / security contractそのものが変わる契約はCoreです。

nerdctl / Docker / OCI toolingはPluginです。現在のIncus backendはStandardです。

## 12. Egress制御の契約はCore、default実装はStandard

Environmentの中ではagentに広い自由を与えますが、Environmentの外へ出る通信は人間が設定したPolicyで制御できることをHacocoonのproduct semanticsに含めます。

概念的には次の形です。

```text
untrusted Environment
       |
       | EgressRequest(destination, protocol, port, metadata)
       v
Core Policy / Approval / EgressController
       |
       | allow / deny / require-approval
       v
EgressProvider contract
       |
       v
Standard or alternative enforcement implementation
```

`EgressRequest`、Policy decision、Approvalとのbinding、`EgressController` / `EgressProvider`の契約はCoreです。

一方で、HTTP CONNECT、SOCKS、DNS-aware proxy、nftables、Incus ACL操作、Kubernetes NetworkPolicyなどの具体的なenforcement mechanismはCoreではありません。

通常のHacocoon配布物で利用するproject-maintainedなdefault proxy/enforcerはStandardに置きます。特殊なenterprise proxyや別方式のenforcerはPluginまたはprovider-specific adapterにできます。

現在のv0.13 Managed Sandbox Networkはdefault-deny substrateまでであり、この設計原則を記載したことだけでdomain-aware allow/approval enforcementが実装済みになったとは扱いません。

Gitのように一般network destinationだけではauthorityを十分に表現できない操作は、org / repository / branch/ref / operationなどを含むspecialized Capabilityとして扱えます。特殊化してもPolicy / ApprovalのCore semanticsは共有します。

## 13. Portabilityを設計上の制約にする

Environmentは、異なるclientや異なるbackendから概念的に同じように利用できる状態を保ちます。

VS Code、Incus、特定container runtime、特定cloud providerをEnvironmentの定義そのものに含めません。Client-specificなlaunch、backend-specificなlifecycle、workload-specificなtoolingはCore domain modelの外に置きます。

## Security promise summary

| Layer | Hacocoonの前提 |
|---|---|
| Environment内のAgent / command | untrusted。Environment内では強い権限を持ってよい |
| Workspace | 明示的に選んだdataだけをmount。writable modeでは破壊的変更も可能 |
| Host credential / control socket | Environmentへambientに露出しない |
| Privileged external operation | 明示的なpolicy/capability boundaryで仲介する |
| Outbound network | Coreのpolicy/approval semanticsでauthorizationし、具体的なenforcementはStandard/provider/plugin実装へ委ねる |
| Environment isolation | 選択したbackendと、そのbackendが文書化した保証が提供する |
| Host kernel / trusted runtime daemon / Hacocoon host process | trusted computing base |
| Kernel exploit / container escape defense | container backendでは保証しない。必要ならより強いbackendを使う |

Hacocoonのdefaultな方向性を一文で表すと、次の通りです。

> 安価なEnvironmentの中ではagentに広い自由を与え、外へ出るauthorityは人間のPolicyで制御し、具体的な実装方式は交換可能なStandard / Provider / Pluginへ委ねる。
