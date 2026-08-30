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
| Environment lifecycle | create、inspect/status、exec、interactive shell、delete、exact ownership、collision refusal | partial unit-level implementation |
| `systemd` / sudo / root | Host root authorityを渡さずPID 1 systemdとEnvironment-local root behavior | partial; real runtime untested |
| Workspace leases | canonical Workspace identity、RO/RW semantics、conflicting lease refusal、`/workspace` behavior | untested / incomplete |
| Whole-Environment copy | fresh trust identityを保ちながらdurable machine/root/runtime stateをCOW copy | unimplemented; #322 |
| Resource budgets | CPU、memory、PID、root-storage semanticsを同等にenforceまたはreject | partial; PID parity gap currently known |
| Client status/access | 現行status、SSH、loopback TCP/forwarding、prepare/revoke behavior | untested / incomplete |
| `haco run` / ephemeral execution | 同じlifecycle、cleanup、lock/recovery behavior | untested |
| Base lifecycle | list/inspect/select/create-from-Base semantics | untested / incomplete |
| Policy / Approval / Capability | 同じfail-closed decision、approval semantics、stale-state handling、audit | existing trusted implementation retained; Kube interaction untested |
| Git push Broker | Environmentへreusable write credentialを渡さず、repo/remote/ref/SHA bindingとstale refusalを維持 | trusted Broker retained; end-to-end parity untested |
| Git fetch | trusted Host authorityとprivate repository behavior | untested |
| Network isolation | equivalent default isolation、DNS behavior、drift detection、bypass無し | partial manifest only; real CNI behavior untested |
| Domain-aware egress | accessを広げず同じauthorization/destination protection semantics | untested / redesign required |
| OCI / nested runtime | Environment-local container runtime behaviorとisolation | untested; current composition rejects OCI plugin |
| Docker compatibility | 現行Docker status/prepare behavior | untested |
| Seed / image behavior | 同じBase/Seed semantics、credential separation、immutable identity、recovery | untested |
| Btrfs/COW storage | 観測可能なcapacity、compression intent、COW、recovery propertyの同等性 | untested / storage design open |
| Trusted `haco-host` | 同じtrusted logical Host behaviorとEnvironmentからの分離 | 実験中はIncus版をretain; all-Kube化のparityは現在の必須条件ではない |
| Notifications / interaction events | 同じclient-visible event semanticsとclient側にapproval authorityを持たせないこと | existing implementation retained; integration untested |
| Structured logging | 同じoperation field、redaction、trust-boundary behavior | untested |
| Failure recovery | interrupted create/delete/run、ownership drift、stale state、cleanup-required semantics | partial unit coverage only |
| Ubuntu 26.04 | project target上でreal substrateが動くこと | blocked/untested; #323 |

Testが増えるほどmatrixを厳格化します。関連するreal/repository acceptanceが十分になるまで`parity proven`とは書きません。

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
