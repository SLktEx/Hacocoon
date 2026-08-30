# Kubernetes parity experiment

[English](kubernetes-parity-experiment.md) | **日本語**

Status: experimental branch-only evaluation.

## Scope

`main-kube` は捨ててもよい実験branchです。**`main` へmergeすることを目的にしません**。migration / replacement判断もこの実験の目的ではありません。

検証するのは次の2点だけです。

1. Kubernetesベース実装で、現在のHacocoonのbehaviorとsecurity invariantを完全なfeature parityとして再現できるか。
2. 完全なfeature parityを作れる場合、Hacocoonが所有する実装・運用が現在のIncusベース実装より実測で単純になるか。

Kubernetesらしい、流行している、説明しやすい、という理由では成功扱いしません。required behaviorを弱めず再現し、単純化を測定できた場合だけ評価します。

## Evaluation rule

`main` のIncus実装をbehavioral baselineとします。

Kubernetes側のbehaviorが違う場合は次のいずれかにします。

- 足りないbehaviorを実装する。
- explicitなparity gapとして記録する。
- そのfeatureではKubernetes approachがnon-viableだと記録する。

Kubernetesへ合わせるためにHacocoon側のrequirementを変えません。

Security invariantもfeatureです。user-visible commandだけ同じでも、credentialを露出する、approval bindingを弱める、Host authorityを広げる、drift時のfail-closedを失う、isolation semanticsを変える場合はfeature parityではありません。

## Feature-parity matrix

このtableはwork-in-progress checklistです。`untested`はparity claimがまだ無いことを意味します。

| Area | 再現するbaseline behavior | `main-kube` status |
|---|---|---|
| Environment lifecycle | create、inspect/status、exec、interactive shell、delete、exact ownership、collision refusal | create/exec/deleteとownershipはrepository integrationでcovered、inspect/shellはunit-covered、real runtimeはuntested |
| `systemd` / sudo / root | Host root authorityを渡さずPID 1 systemdとEnvironment-local root behavior | manifest + PID 1 verification実装済み、real runtimeはuntested |
| Workspace leases | canonical Workspace identity、RO/RW semantics、conflicting lease refusal、`/workspace` behavior | Core/state parityをreal provider routing経由でcovered。RW/RO拒否とRO/RO共有をcluster mutation前にtest済み。real mount semanticsはuntested |
| Whole-Environment copy | fresh trust identityを保ちながらdurable machine/root/runtime stateをCOW copy | unimplemented; #322 |
| Resource budgets | CPU、memory、PID、root-storage semanticsを同等にenforceまたはreject | finite CPU/memory/rootはCore/provider routingを保持。finite per-Environment PIDはexplicit parity gap |
| Client status/access | 現行status、SSH、loopback TCP/forwarding、prepare/revoke behavior | durableなloopback-only detached `kubectl port-forward` state、list/reconcile/remove、SSH prepare/revokeをrepository test済み。real Kubernetes、Environment delete時cleanup、crash/reboot recoveryは未証明 |
| `haco run` / ephemeral execution | 同じlifecycle、cleanup、lock/recovery behavior | Kubernetes provider経由でcreate/exec/delete cleanupとdurable marker削除をrepository integration test済み。crash/restart recoveryのreal-host acceptanceはuntested |
| Base lifecycle | list/inspect/select/create-from-Base semantics | untested / incomplete。現在のKubernetes providerはexplicit Base selectionをreject |
| Policy / Approval / Capability | 同じfail-closed decision、approval semantics、stale-state handling、audit | existing trusted implementationを変更せずretain。通常のKubernetes Environment push pathが同じcapability serviceを通ることをrepository integrationで確認 |
| Git push Broker | Environmentへreusable write credentialを渡さず、repo/remote/ref/SHA bindingとstale refusalを維持 | Kubernetes-backed Environmentでexact SHA/ref binding、stale remote拒否、ambient GitHub/askpass state除外をrepository integrationで確認。real authenticated GitHub pushはuntested |
| Git fetch | trusted Host authorityとprivate repository behavior | untested |
| Network isolation | equivalent default isolation、DNS behavior、drift detection、bypass無し | ingress/egress default-deny manifestとprovider-routed source identityをunit-covered。real CNI enforcement、DNS、SNAT behaviorはuntested |
| Domain-aware egress | accessを広げず同じauthorization/destination protection semantics | Policy/Approval/Brokerは再利用可能でsource identityもprovider-neutral化。proxy/listener transportは未解決。NetworkPolicyだけでは現行のhostname approval + DNS pinning + SNI validationと同義ではない |
| OCI / nested runtime | Environment-local container runtime behaviorとisolation | untested。current compositionはOCI pluginをreject |
| Docker compatibility | 現行Docker status/prepare behavior | untested |
| Seed / image behavior | 同じBase/Seed semantics、credential separation、immutable identity、recovery | untested |
| Btrfs/COW storage | 観測可能なcapacity、compression intent、COW、recovery propertyの同等性 | untested / storage design open |
| Trusted `haco-host` | 同じtrusted logical Host behaviorとEnvironmentからの分離 | 実験中はIncus版をretain。all-Kube formのparityは現在の必須条件ではない |
| Notifications / interaction events | 同じclient-visible event semanticsとclient側にapproval authorityを持たせないこと | existing implementation retained。integrationはuntested |
| Structured logging | 同じoperation field、redaction、trust-boundary behavior | test済みcreate/exec/delete pathではexisting Core loggingを再利用。provider-specific coverageはincomplete |
| Failure recovery | interrupted create/delete/run、ownership drift、stale state、cleanup-required semantics | provider cleanup/ownership fail-closed unit coverage + normal `haco run` cleanup integration。client-forward PID reuseはfail closed。crash/node-restart failure injectionはuntested |
| Ubuntu 26.04 | project target上でreal substrateが動くこと | blocked/untested; #323 |

Testが増えるほどmatrixを厳格化します。関連するreal/repository acceptanceが十分になるまで`parity proven`とは書きません。

## First findings

Architecture diagram上では似て見えたcomplexityが、実装するといくつかの種類に分かれてきました。

### Core behaviorはかなり安く持ってこれる

Workspace lease ownership、RO/RW conflict rule、Environment metadata、ephemeral `haco run` lifecycle、cleanup marker、Policy / Approval / Capability、brokered Git authority、audit state、client-neutral interaction semanticsはIncus featureではありません。provider seamより上にあるため、Kubernetes-specific codeをほぼ増やさず再利用できます。

Repository integrationでは、fake Core runtimeだけでなくKubernetes providerを実際にrouterへ接続した状態でWorkspace lease conflictとnormal ephemeral-run cleanupを通しています。conflicting Workspace requestはKubernetesに触る前に拒否されます。

Security-sensitiveなGit push pathも同じです。Kubernetes-backed Environmentを通常のWorkspace service経由でpersistし、そのまま変更していないtrusted Git Brokerへ渡すintegration testを追加しました。Brokerはapproved source SHAをapproved refへexactにpushし、policy evaluation後にGitHub remote identityが変わればstaleで拒否し、ambient `GH_TOKEN`、`GITHUB_TOKEN`、caller-controlled `GIT_ASKPASS`を引き継がないことを確認しています。real authenticated-GitHub acceptanceの代わりではありませんが、runtime swap自体のためにBroker contractを弱める必要がないことは確認できました。

### Network plumbingは減らせそうやけど、security Brokerは自動では消えない

Kubernetes `NetworkPolicy`はIncus-specific bridge/ACL isolation machineryのかなりの部分を置き換えられる可能性があります。`main-kube`ではexplicit default-deny policyを作り、egress source identityもIncus runtime直結ではなくEnvironment router経由へ移しました。

ただし現行domain-aware egressはstatic packet allowlistより強いfeatureです。Hacocoon Policy / Approval、Host-side DNS resolution/public-address validation、per-connection DNS pinning、HTTPS CONNECT/SNI validation、auditを組み合わせています。Standard `NetworkPolicy`だけでは同じsemanticsになりません。

そのためparity experimentでは次を分けて評価します。

- **Incus-specific network transport/proxy plumbing** — Kubernetes/CNIで消せる可能性がある部分。
- **Hacocoon authorization/enforcement proxy semantics** — 同じbehaviorをより少ないmachineryで再現する代替が無い限り必要な部分。

Outbound accessを広げてproxyを消すのはsimplificationではなくparity failureです。

### Client loopback accessは再現できたが、現時点ではHacocoon側machineryが増えた

Persistent/reconcilableなclient-connection contractを再現するrepository-level candidateを実装しました。foreground `kubectl port-forward`を同等扱いせず、trusted sideでloopback-only port-forwardをdetachし、`HACO_ROOT`配下へprivate durable recordを保存します。recordにはrandom process token、PID、Linux `/proc` start-time identity、Environment ref、exact portを持たせます。

新しいProvider instanceからconnectionを再発見してlist/revokeでき、PID reuseやprocess identity mismatchを誤ってkillせずfail closedにします。port-forward subprocessへ渡すenvironmentも最小化し、`KUBECONFIG`等のKubernetes authority inputはtrusted sideに残しつつ、ambient GitHub token、Git askpass state、その他の不要なprocess credentialは継承しません。SSHも同じdurable forwarding pathを使い、Environment内ではmarker-scoped public keyだけを管理します。

これでclient contractがKubernetesでは原理的に不可能というわけではないことは確認できました。ただし**単純化の証拠ではありません**。このsliceの前はKubernetes provider implementationが568 physical linesでしたが、durable client forwardingとprocess/state reconciliationを足した後は1,226 physical linesになりました。Incus provider baselineは3,382 linesでまだ大きいものの、Kubernetes側にはwhole-Environment clone、Base/Seed、OCI/storage semantics、real network/runtime acceptanceなど大物が残っています。したがってこの差はwinner判定ではなく、今後追跡するevidenceです。

Client accessにもまだgapがあります。real `kubectl port-forward` behavior、supervisorをtrusted `haco-host` boundaryへ確実に置くこと、Environment delete時のexplicit cleanup、restart/reboot recoveryは未証明です。これらのexact parityに追加daemon/reconciliation stateが必要なら、そのcomplexityもKubernetes結果へそのまま加算します。

### Runtime/storageが最大のparity risk

Whole-Environment COW clone、immutable Base identity、nested runtime state、Btrfs/storage behavior、Ubuntu 26.04 system-container compatibilityはまだopenです。ここはIncusが単なるorchestrationではなくhigh-level machine/container semanticsを提供している部分です。

## Simplicity comparison

まずfeature parityを評価します。同じbehaviorを作れてから単純さを比較します。

Incus baselineとKubernetes experimentを同じdimensionで測定します。

### Hacocoon-owned code

runtime、networking、storage integration、bootstrap、reconciliation、recovery専用に必要なcodeを比較します。

- Go LOC;
- shell/PowerShell/Python LOC;
- provider-specific test;
- E2E harness code。

Generated file、vendored dependency、Hacocoonが所有しないupstream Kubernetes manifestを「複雑さが消えた」として除外しません。external operational dependencyとして別に記録します。

### Hacocoon-owned mechanisms

Hacocoonが理解・作成・reconcile・recoverするcustom mechanismを数えます。

- daemon / helper;
- proxy / forwarding layer;
- bridge / ACL / custom network resource;
- storage pool / loop device / mount / snapshot glue;
- privileged helper;
- lock / persistent reconciliation state;
- custom ownership marker / drift check。

### External operational machinery

Hacocoonから消えたのではなく外部へ移ったcomplexityも記録します。

- Kubernetes distribution / control plane;
- CNI;
- CSI / local storage driver;
- Sysbox等のsystem-container RuntimeClass;
- container runtime;
- privileged node component;
- Ubuntu 26.04 / WSL compatibility constraint。

Hacocoon LOCが減ってもmandatory platform stackが大きく増える場合、total systemが自動的にsimpleとは判断しません。Hacocoon complexityとtotal operational complexityを両方出します。

### Performance / UX

同じoperationが存在する場合は少なくとも次を比較します。

- Environment cold start;
- repeated start/reuse（該当する場合）;
- exec latency;
- interactive shell latency;
- network throughput/latency;
- large-file I/O。可能なら100GB級も含む;
- whole-Environment copy time / physical space amplification;
- cleanup time;
- CI/E2E runtime。

結果が「Kubeの方が単純だが遅い」「速いが複雑」「parity不能」でもそのまま記録します。

## Security parity

次はnon-negotiableなparity requirementです。

- Environmentへreusable Host/GitHub write credentialを渡さない。
- Kubernetes上で動くというだけでEnvironmentへKubernetes control-plane authorityを渡さない。
- raw provider/control socketをEnvironmentへ露出しない。
- privileged external operationではPolicy / Approval / Capabilityをauthoritativeに維持する。
- approved Git stateがexecution前に差し替えられた場合はstaleとして拒否する。
- provider ownership ambiguityはfail closed。
- network-policy driftやunsupported enforcementをsecure successとして扱わない。
- cleanup uncertaintyはleftoverをsilent adoptせずrecovery-requiredとして表面化する。
- Environment clone/snapshotでtrust stateをcopyしない。

## Experiment result

最終結果はmerge proposalではなく、事実比較としてまとめます。

次のどれかを使います。

- **full parity + materially simpler**;
- **full parity + roughly equal complexity**;
- **full parity + more complex**;
- **partial parity**（missing behaviorを明示）;
- **not viable on the target substrate**。

Simplicity判定はarchitecture preferenceではなくcode/mechanism/dependency差の実測を添えます。

## Related documents

- [`kubernetes-runtime.ja.md`](kubernetes-runtime.ja.md) — 現在のexperimental provider mechanics / security boundary
- [`../IMPLEMENTATION_STATUS.ja.md`](../IMPLEMENTATION_STATUS.ja.md) — current repository baseline
- GitHub issue #322 — whole-Environment clone parity
- GitHub issue #323 — Ubuntu 26.04 real runtime / broad feature parity measurement
