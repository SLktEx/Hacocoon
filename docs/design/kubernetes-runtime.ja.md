# Kubernetes Environment runtime

[English](kubernetes-runtime.md) | **日本語**

Status: **`main-kube` 上のpartial / experimental実装。Incus pathのsupported replacementではありません。**

## Summary

`main-kube` 実験では、Hacocoonのtrusted security boundaryをKubernetes workloadへ移さずに、KubernetesをEnvironment providerとして追加します。通常EnvironmentはSysbox-backed Podとして動かせますが、`haco-host`、Policy / Approval / Capability、Git push Broker、credential、audit state、現在のrecovery pathはtrusted Host側の責務として残します。

この実験の目的は、Hacocoonのbrokered-authority modelを弱めず、whole-Environment copy semanticsを失わずに、KubernetesへEnvironment lifecycleとnetwork plumbingの多くを任せられるかを検証することです。

## Goals

- CoreへKubernetes専用conditionを増やさず、既存のprovider-neutral Environment routing seamを使う。
- Sysbox RuntimeClass経由で`systemd`をPID 1として動かす。
- Environment-local rootを許しつつ、Kubernetes control-plane authorityとreusable Host credentialを渡さない。
- Environment作成時からKubernetes networkをdefault denyにする。
- namespace ownership不明、resource guarantee不足、cleanup不完全をfail closedにする。
- Git pushなどのprivileged external operationでは既存Hacocoon Broker / Approval pathをauthoritativeな境界として維持する。
- Environment backendの評価中も`haco-host`をtrusted infrastructureとして残す。

## 現在のsliceで対象外

現在のrepository sliceは次をclaimしません。

- real Kubernetes + Sysbox host acceptance;
- 安全なmulti-node Workspace placement;
- whole-Environment snapshot / clone support;
- Kubernetes-native Base / Seed support;
- Kubernetes Environment provider上のOCI plugin compatibility;
- Kubernetes Environment向けSSH / local-port client adapter;
- production-ready outbound network policy;
- Kubernetes PVC snapshotだけでHacocoonのwhole-Environment copy requirementを満たすこと。

これらはacceptanceまたはdesign gateです。現在のsecurity modelを暗黙に弱めてよいfollow-upではありません。

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

`hostUsers: false`はKubernetes Pod user namespaceを要求します。選択したSysbox RuntimeClassはHacocoonが必要とするsystem-container behaviorを提供できる必要があります。この保証を提供できないclusterはHacocoon Kubernetes backendとしてacceptしません。

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

長期的なKubernetes方向では、通常packet isolation、DNS、選択CNIが必要policyをenforceできる範囲のapproved direct egressはcluster networking layerへ任せます。CNIと同じ仕事をするためだけにHacocoonのmandatory byte-forwarding proxyを残しません。

ただしprivileged-operation Brokerは消しません。Git pushなどprotected authorityが必要な操作はHacocoon Policy / Approval / Capabilityへのstructured requestのままとし、credentialはtrusted sideだけで使います。

現在のsliceにはbroad egress allow policyを意図的に入れていません。将来direct outbound accessを有効化する前に必要CNI capabilityを明示し、そのcapabilityが無い・driftしている場合はfail closedにする必要があります。

## Workspace transport

最初のlocal experimentでは明示的に選ばれたWorkspaceを`hostPath`でPodへmountします。

これは**最終的なKubernetes storage/security contractではありません**。writable `hostPath`はそのhost pathへのauthorityをPodへ与え、Kubernetes Pod Security Baseline/Restrictedとも互換ではありません。またmulti-node clusterではnode placement依存を作ります。

したがって現在の`hostPath`はlocal proof pathとしてのみ扱います。Kubernetes backendをsupportedとみなす前に、Workspace transportは次のどちらかへ進める必要があります。

- explicit lease / blast-radius semanticsを同等に持つstorage mechanismの背後へ移す。
- local single-node placement modelを証明・enforceし、残る`hostPath` authorityを意図的かつ限定されたものとして受け入れる。

Host HOME、credential store、Kubernetes config、Hacocoon state、runtime socketを便利だからという理由でmountしてはいけません。

## Whole-Environment copy gate

Whole-Environment copyはHacocoonのrequired propertyであり、optional optimizationではありません。

目標semanticは概念的に次です。

```text
haco clone source target
```

`target`は通常root filesystem pathの変更やEnvironment-local runtime dataを含むsource Environmentのdurable machine stateから開始します。一方でHacocoon identityは新規であり、credential、approval、capability lease、trusted control authorityはcopyしません。

relevant stateがOCI writable root filesystemやSysbox runtime-local dataに残る構成では、Kubernetes PVC cloneや`VolumeSnapshot`だけでは不十分です。そのため、必要なEnvironment stateをHacocoon contractとして十分atomicにcaptureできるstorage layout / clone operationを実証するまで、Kubernetes backendは**Incus backendを置き換える候補としてacceptしません**。

望ましい方向は、durable Environment root stateをsnapshot-capableなCOW storage boundaryへ置き、trust stateをその外へ置くことです。具体的なCSI/Btrfs実装はこのbranchでは未決定です。

## Resource guarantee

最初の実験ではCPU、memory、root-storage limitをKubernetes container limitsへprojectionします。

portable Pod resource APIでは現在Hacocoonがmodelするper-Environment PID guaranteeと同等のものを提供できないため、finite PID budgetはcluster mutation前にrejectします。node-wide kubelet settingを同等保証として扱ってはいけません。

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

push credentialをEnvironmentへ露出しないと成立しないKubernetes実装はこのdesignと互換ではありません。

## `haco-host`

experimental providerは`haco-host`を置き換えません。

`main-kube`では既存local Incus trusted-host lifecycleをnormal trusted logical Host / recovery pathとして残します。これによりEnvironment runtimeの検証と、将来`haco-host`自体をどこで動かすかという判断を分離できます。

## Current configuration

experimental providerは次で選択します。

```text
HACO_RUNTIME_PROVIDER=runtime.kubernetes
HACO_KUBERNETES_IMAGE=<systemd-capable-image>
```

optional experiment setting:

```text
HACO_KUBERNETES_RUNTIME_CLASS=sysbox-runc
HACO_KUBECTL=kubectl
```

Kubernetes Environment providerは、まだこのbackendでintegrationをverifyしていないため現在`HACO_PLUGIN_OCI` compositionをrejectします。

## Replacementまでのacceptance gate

次をintended supported hostで実証するまで、KubernetesをIncus replacementと記述しません。

1. `systemd`、Environment-local root、nested container-runtime workloadを含むreal Kubernetes + Sysbox lifecycle。
2. ServiceAccount token、Host credential、Hacocoon control socket、`haco-host` authorityがEnvironmentへ露出していないこと。
3. ingress / egress isolationと選択CNIのfail-closed policy behavior。
4. credentialがEnvironment外に残り、changed stateへapprovalをreuseできないことを証明するbrokered Git push regression coverage。
5. fresh trust identityを伴うwhole-Environment snapshot / clone。
6. accidental multi-node `hostPath` authority / placement bugを持たないWorkspace semantics。
7. cleanup、crash、retry、node restart、drift failure injection。
8. 通常networkやlarge-file pathへ不要なHacocoon forwarding proxyを戻さずperformance regressionが無いことのmeasurement。
