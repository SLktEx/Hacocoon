# Kubernetes Environment runtime

[English](kubernetes-runtime.md) | **日本語**

Status: **`main-kube` 上のpartial / experimental実装。parity evaluation専用であり、このbranchを`main`へmergeすることは目的にしません。**

## Summary

`main-kube` 実験では、Hacocoonのtrusted security boundaryをKubernetes workloadへ移さずに、KubernetesをEnvironment providerとして追加します。通常EnvironmentはSysbox-backed Podとして動かせますが、`haco-host`、Policy / Approval / Capability、Git push Broker、credential、audit state、現在のrecovery pathはtrusted Host側の責務として残します。

この実験の目的は、Kubernetesで現在のHacocoonの全feature/security behaviorを再現できるか、再現できる場合にHacocoon-owned implementation / operationがIncus baselineより実測で単純になるかを確認することだけです。migration、adoption、merge recommendationは行いません。評価contractは [`kubernetes-parity-experiment.ja.md`](kubernetes-parity-experiment.ja.md) を参照してください。

## Goals

- CoreへKubernetes専用conditionを増やさず、既存のprovider-neutral Environment routing seamを使う。
- Sysbox RuntimeClass経由で`systemd`をPID 1として動かす。
- Environment-local rootを許しつつ、Kubernetes control-plane authorityとreusable Host credentialを渡さない。
- Environment作成時からKubernetes networkをdefault denyにする。
- namespace ownership不明、resource guarantee不足、cleanup不完全をfail closedにする。
- Git pushなどのprivileged external operationでは既存Hacocoon Broker / Approval pathをauthoritativeな境界として維持する。
- Environment runtime parityの評価中も`haco-host`をtrusted infrastructureとして残す。
- Kubernetesが本当にHacocoon-owned complexityを消しているのか、別のglue codeやmandatory platform dependencyへ移しただけなのかを測定する。

## 現在のsliceで対象外

現在のrepository sliceは次をclaimしません。

- `main-kube`を`main`へmergeする意図;
- migration / replacement decision;
- real Kubernetes + Sysbox host acceptance;
- 安全なmulti-node Workspace placement;
- whole-Environment snapshot / clone support;
- Kubernetes-native Base / Seed support;
- Kubernetes Environment provider上のOCI plugin compatibility;
- Kubernetes Environment向けSSH / local-port client adapter;
- production-ready outbound network policy;
- Kubernetes PVC snapshotだけでHacocoonのwhole-Environment copy requirementを満たすこと。

これらはparity gapまたは未解決のexperiment questionです。Kubernetes側の表現が違うからという理由でHacocoon requirementを弱めません。

## Ownership / trust boundary

意図する分離は次です。

```text
Physical Host / WSL                         TRUSTED
  |- Hacocoon controller
  |- Policy / Approval / Capability
  |- Git / external-operation Broker
  |- credentials / audit state
  |- Incus recovery / current haco-host lifecycle
  `- haco-host                              TRUSTED

Kubernetes cluster
  `- haco-<environment> namespace
       `- Sysbox Environment Pod            UNTRUSTED
            |- systemd
            |- Environment-local root
            `- /workspace
```

Kubernetes Environmentへ次を渡してはいけません。

- defaultのKubernetes ServiceAccount token;
- Hacocoon controller / provider-control socket;
- reusable GitHub / Host credential;
- `haco-host` filesystem / control authority;
- host PID / IPC / network namespace;
- privileged-container authority。

そのため現在のPod manifestは`automountServiceAccountToken: false`、`hostUsers: false`、`hostNetwork: false`、`hostPID: false`、`hostIPC: false`、`privileged: false`を設定します。

`hostUsers: false`はKubernetes Pod user namespaceを要求します。選択したSysbox RuntimeClassはHacocoonが必要とするsystem-container behaviorを提供できる必要があります。この保証を提供できないclusterは、このexperimentではruntime parity gapです。

## Identity / ownership

各Environmentは現在、次のnamespaceを1つ所有します。

```text
haco-<environment-name>
```

mutation前にHacocoonは次のlabelをすべて書き込み、再検証します。

```text
app.kubernetes.io/managed-by=hacocoon
hacocoon.dev/role=environment
hacocoon.dev/environment=<environment-name>
```

同名namespaceがあってもidentityが完全一致しなければincompatible stateです。Hacocoonはそれをadopt、exec、deleteしません。

name validationはcluster mutationより先に実行します。provider refが`haco-` prefixを持つだけでは信用せず、namespace ownershipをauthorityとします。

## Network model

現在の実験ではnamespace全体へingress / egress両方の明示的default-deny `NetworkPolicy`を作成します。KubernetesはselectするNetworkPolicyが無いPodのtrafficを許可するため、policy不在をsecure defaultとは扱いません。

実験では、選択CNIが現在のHacocoon contractと同等behaviorをenforceできる範囲で、通常packet isolation、DNS、approved direct egressをcluster networking layerへ任せても構いません。これはHacocoon-owned proxy/network plumbingをproduct semanticsを変えず消せるかの検証です。

ただしprivileged-operation Brokerは消しません。Git pushなどprotected authorityが必要な操作はHacocoon Policy / Approval / Capabilityへのstructured requestのままとし、credentialはtrusted sideだけで使います。

現在のsliceにはbroad egress allow policyを意図的に入れていません。将来experimental direct outbound accessを有効化する前に必要CNI capabilityを明示し、そのcapabilityが無い・driftしている場合はfail closedにする必要があります。同等のdomain-aware semanticsを再現できない場合はparity failureです。

## Workspace transport

最初のlocal experimentでは明示的に選ばれたWorkspaceを`hostPath`でPodへmountします。

これは**parity-completeなKubernetes storage/security contractではありません**。writable `hostPath`はそのhost pathへのauthorityをPodへ与え、Kubernetes Pod Security Baseline/Restrictedとも互換ではありません。またmulti-node clusterではnode placement依存を作ります。

したがって現在の`hostPath`はlocal proof pathとしてのみ扱います。Workspace parityをclaimする前に、次のどちらかが必要です。

- explicit lease / blast-radius semanticsを同等に持つstorage mechanismの背後へ移す。
- local single-node placement modelを証明・enforceし、残る`hostPath` authorityがIncus baselineとbehaviorally equivalentであることを示す。

Host HOME、credential store、Kubernetes config、Hacocoon state、runtime socketを便利だからという理由でmountしてはいけません。

## Whole-Environment copy parity gate

Whole-Environment copyはHacocoonのrequired propertyであり、optional optimizationではありません。

目標semanticは概念的に次です。

```text
haco clone source target
```

`target`は通常root filesystem pathの変更やEnvironment-local runtime dataを含むsource Environmentのdurable machine stateから開始します。一方でHacocoon identityは新規であり、credential、approval、capability lease、trusted control authorityはcopyしません。

relevant stateがOCI writable root filesystemやSysbox runtime-local dataに残る構成では、Kubernetes PVC cloneや`VolumeSnapshot`だけでは不十分です。Hacocoon contractとして十分atomicに必要Environment stateをcaptureできるstorage layout / clone operationを実証するまで、whole-Environment copyはexplicitなparity failureです。

現在のinvestigation方向はdurable Environment root stateをsnapshot-capableなCOW storage boundaryへ置き、trust stateをその外へ置くことです。具体的なCSI/Btrfs実装はこのbranchでは未決定です。GitHub issue #322で追跡します。

## Resource guarantee

最初の実験ではCPU、memory、root-storage limitをKubernetes container limitsへprojectionします。

portable Pod resource APIでは現在Hacocoonがmodelするper-Environment PID guaranteeと同等のものを提供できないため、finite PID budgetはcluster mutation前にrejectします。node-wide kubelet settingを同等保証として扱ってはいけません。同等mechanismが無い間はfinite PID budgetをparity gapとして記録します。

## Failure / retry / cleanup

Createはfail-closed sequenceで進めます。

1. Environment name、Workspace path、requested resource、provider configをvalidateする。
2. target namespaceをinspectする。
3. exact Hacocoon ownershipを証明できないcollisionを拒否する。
4. owned namespaceを作る。
5. default-deny networkingとSysbox Podを作る。
6. Readyを待つ。
7. `systemd`がPID 1であることをverifyする。
8. RW requestならWorkspaceがwritableであることをverifyする。

namespace作成後に失敗した場合、caller cancellationからdetachしたbounded contextでcleanupします。namespace削除直前にownershipを再検証します。cleanup完了を証明できなければ成功扱いせずrecovery-requiredを返します。

exec、shell、inspect、deleteもoperation前にexact namespace ownershipを再検証します。

## Broker / Git push invariant

Kubernetesはprivileged external actionに対するHacocoon authorization modelにはなりません。

Git pushでは特に次を維持します。

- Environmentへreusable write credentialを渡さない。
- EnvironmentがKubernetes RBACを使って自分をauthorizeできない。
- requestはHacocoon Policy / Approval / Capabilityを通る。
- approvalはGit Broker contractが必要とする具体的repository / remote / ref / commit / operation stateへboundされたままにする。
- staleまたはmismatchしたstateはfail closedにする。
- trusted Brokerがprivileged operationを構築・実行し、audit evidenceを記録する。

push credentialをEnvironmentへ露出しないと成立しないKubernetes実装はparity failureです。

## `haco-host`

Environment runtime experimentでは既存`haco-host`実装をIncus上でそのまま残します。

これは意図的です。この実験では「untrusted Environment runtimeをKubernetesでfull parityかつ少ないHacocoon-owned complexityで再現できるか」と、「trusted `haco-host` infrastructureをどこで動かすか」を分離します。既存Environment-facing behaviorを再現するために必要にならない限り、all-Kubernetes `haco-host`はこの実験の必須条件ではありません。

## Current configuration

experimental providerは次で選択します。

```text
HACO_RUNTIME_PROVIDER=runtime.kubernetes
HACO_KUBERNETES_IMAGE=<systemd-capable-image>
HACO_KUBERNETES_EXPERIMENTAL_HOSTPATH=1
```

Workspace transportがwritable `hostPath`を使っている間、`HACO_KUBERNETES_EXPERIMENTAL_HOSTPATH=1`は意図的に必須です。指定しない場合はfail closedにして、この実験transportをdefault supported behaviorのように見せません。

optional experiment setting:

```text
HACO_KUBERNETES_RUNTIME_CLASS=sysbox-runc
HACO_KUBECTL=kubectl
```

Kubernetes Environment providerは、まだこのbackendで同じintegrationを再現していないため現在`HACO_PLUGIN_OCI` compositionをrejectします。このreject自体がexplicitなfeature-parity gapであり、accepted product differenceではありません。

## Parity gates

現在のHacocoon baselineと同じtarget上で次を実証するまでfull parityとは書きません。

1. `systemd`、Environment-local root、nested container-runtime workloadを含むreal Kubernetes + selected system-container RuntimeClass lifecycle。
2. ServiceAccount token、Host credential、Hacocoon control socket、`haco-host` authorityがEnvironmentへ露出していないこと。
3. 現行isolation / authorization contractと同等のingress/egress behaviorとfail-closed drift behavior。
4. credentialがEnvironment外に残り、changed stateへapprovalをreuseできないことを証明するbrokered Git push regression coverage。
5. fresh trust identityと同等COW semanticsを伴うwhole-Environment snapshot / clone。
6. accidental multi-node authority changeを持たないWorkspace identity、RO/RW lease、mount、conflict semantics。
7. current client access、Base、OCI/Docker、Git fetch、interaction、logging、resource、run/recovery、その他user-visible/runtime featureを再現するかexplicit parity gapとして記録すること。
8. cleanup、crash、retry、node restart、drift failure injection。
9. large-file pathを含むIncus baselineとのcomplexity / performance実測比較。

これらを通っても**`main`へmergeする意味にはなりません**。結果は [`kubernetes-parity-experiment.ja.md`](kubernetes-parity-experiment.ja.md) で定義したfact classificationのどれかとして記録するだけです。
