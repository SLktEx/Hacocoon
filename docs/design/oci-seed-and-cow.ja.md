# v0.17 — OCI Seed Builder & Btrfs/COW Optimization

[English](oci-seed-and-cow.md) | **日本語**

Status: **repositoryのbuild/publish、operations-hardening、credential-free managed-Environment harvest sliceを実装済み / partial。v0.15 recommendationとv0.16 deletion policyは実装済みprerequisiteです。real-host、authenticated/private-registry combination、physical COW acceptanceが残っています。**

v0.17はphysical OCI Seed pipelineを担当します。trusted Host-side image acquisition/cache、offline Seed construction、immutable publication、revision pinning、storage-driver COW benefit、保守的なlifecycle maintenanceを一つのfeature gateとして扱います。

Local Registryは必須ではありません。

この実装は、このfeatureがv0.18と呼ばれていた時点からlandし始めました。現在の正本ではSeed Builder/COWをv0.17へ付け替えます。historical commit/PRには旧番号が残る場合があります。

## Goal

common OCI imageをimmutable Incus-derived Seedへpreloadし、normal Incus/storage-driver clone semanticsでunchanged filesystem blockを再利用しつつ、各Environmentのwritable containerd stateは独立させます。

```text
upstream registry
      |
 trusted Host acquisition/cache
      v
Host seed cache
      |
 OCI export/stream
      v
Offline Seed Builder
(no general network; no NIC)
      |
containerd import/unpack
      |
clean stop
      v
immutable Incus Seed
      |
Incus clone / Btrfs COW
      v
independent Environments
```

## 実装済みfirst repository slice

- `haco plugin oci seed build [--base <base>] [--json]`
- `haco plugin oci seed current [--base <base>] [--json]`
- Tooling Base / current Seed manifestのpersistとprocess-safe build lock
- build前のimmutable parent Base resolution
- dedicated `hacocoon-seed` namespaceを使うtrusted Host-side OCI acquisition
- profileなし・NICなしのoffline Seed Builder
- live containerd state directoryをcopyせず、supported nerdctl/containerd interfaceでimport
- publish前の全selected immutable digest verification
- immutable Incus image publish前のservice clean stop
- publicationとmanifest persist成功後だけcurrent-Seed pointerをadvance
- recorded parent Base revisionがcurrent immutable parentと一致する時だけSeedを使うexact-parent resolution
- Host Docker/containerd socketをforwardせず、Tooling Base内でcontainerd + nerdctlとgenuine Docker CLI/Engine compatibilityを提供

## 実装済みoperations-hardening slice

- `haco plugin oci seed pin <reference@sha256:...> [--base <base>] [--json]`
- `haco plugin oci seed unpin <reference@sha256:...> [--base <base>] [--json]`
- `haco plugin oci seed pins [--base <base>] [--json]`
- `haco plugin oci seed gc [--json]`
- `haco plugin oci seed recover [--json]`
- `haco plugin oci image reenable <reference@sha256:...> [--json]`
- Base単位のexplicit immutable pinをpersistし、automatic recommendationとmerge
- deletion tombstoneはrecommendationと既存pinより優先し、exact immutable identityを明示reenableするまで復活させない
- Seed publish後にもdeletion stateを再確認し、長いbuild中にdeleteがraceしてもcurrent pointerを進めない
- process-safe Seed build lockを保持したまま、次のbuild前にHacocoon-ownedの中断builderをreconcile
- old Tooling/Seed image GCはHacocoon Incus projectとHacocoon-owned aliasに限定
- current manifest revision/alias、instanceの `volatile.base_image`、Incus `used_by`、external aliasがあるimageはretain
- malformedなIncus/provider inventoryはdestructive delete前にfail closed
- GCはsupported Incus image lifecycleだけを使い、Incus-owned Btrfs subvolumeを直接操作しない

## 実装済みcredential-free Environment harvest slice

exact immutable OCI identityがrunning中のHacocoon-managed Environmentに既に存在する場合、registry credentialをcopyせず、そのlocal contentをSeed acquisitionへ再利用できます。

- 新規managed Incus Environmentへ `user.hacocoon.kind=environment` markerを付与
- harvest sourceはそのexact markerを持つrunning instanceだけ
- harvest対象はexact `reference@sha256:...` Seed pullだけ
- Environment内でexact identityを `nerdctl save` し、randomな `/tmp` archiveを作成
- trusted Hostは `incus file pull` でそのOCI archiveだけをcopyし、guest側archiveをすぐ削除
- archiveはHostのdedicated `hacocoon-seed` namespaceへloadし、exact immutable identityをverify
- markerのない既存/legacy Environmentはharvest対象にしない
- safe harvestが成立しない場合は従来のtrusted Host `nerdctl pull` へfallbackし、Host-owned registry credential pathを維持

registry login file、credential-helper output、workspace data、任意のEnvironment file、live `/var/lib/containerd` はcopyしません。転送するのは既にlocalにあるimage contentから作ったtemporary OCI archiveだけです。

## Mandatory isolation

storage savingのために複数Environmentで一つのwritable `/var/lib/containerd` を共有してはいけません。各Environmentはindependentにmutable/deletable/recoverableである必要があります。

## Inputs

- v0.11のimmutable parent Base revision
- v0.15 recommendation/auto-promotion + explicit operator pin
- v0.16 deletion tombstone/override
- 利用可能ならmarked managed Environmentに既に存在するexact immutable OCI content
- registry authenticationがなお必要な場合のtrusted Host-side upstream credential

mutable OCI tagはinput convenienceであり、Seed manifestとexplicit pinはimmutable digestをpersistします。

## Build lifecycle

1. process-safe Seed build lockを取得し、providerがmaintenance対応なら中断builderをreconcile
2. Baseをimmutable revisionへresolve
3. OCI recommendationとBase単位のexplicit pinからeffective Seed image setを確定
4. deletion tombstoneにblockedされたimmutable identityをreject
5. 各exact immutable OCI identityをtrusted Host Seed cacheへacquire。eligibleなmarked managed Environmentからcredential-free harvestを先に試し、必要ならtrusted Host registry acquisitionへfallback
6. trusted Host cacheからtemporary Seed BuilderへOCI export/stream
7. pinned Tooling Baseからgeneral networkなし・NICなしでbuilder作成
8. supported containerd/nerdctl interfaceでimport/unpack
9. requested digestをverify
10. containerd/Docker compatibility serviceをclean stop
11. builder stop
12. immutable Incus Seed revisionをpublish
13. selected immutable identityのdeletion stateを再確認
14. Base / Tooling / Seed revisionとOCI digest manifestをpersist
15. publication/validation成功後のみcurrent-Seed pointerをmove
16. publication/state persistenceやmaintenance cleanupが曖昧ならrecovery-required

## Pin / deletion / re-enableの優先順位

explicit pinは、一つのlogical Baseについて将来のSeedへexact immutable OCI identityを含めるoperator指定です。explicit deletionを上書きしません。

v0.16 deletion tombstoneはautomatic recommendationと既存pinの両方より優先します。tombstoned identityは `haco plugin oci image reenable <reference@sha256:...>` でexact identityを明示reenableするまで再選択できません。mutable tagが別digestへ動いた場合に誤って別identityをreenableしないため、reenableはexact identityだけを受け付けます。

## Recovery / GC

`haco plugin oci seed recover` はexact Hacocoon temporary Seed/Tooling builderをreconcileしてから、`seed gc` と同じ保守的なimage retention判定を実行します。configured backendが対応する場合、`seed build` も新しいbuild開始前にinterrupted-builder recoveryを呼びます。

Hacocoon-ownedかつunusedであることを証明できないimageはdeleteしません。current Seed/Tooling revision、protected alias、instance base-image fingerprint、Incus `used_by`、external aliasのいずれかがあればretainします。malformed inventoryはfail closed errorです。

## Plugin boundary

Seed observation/deletion/build/current/pin/maintenanceは `haco plugin oci ...` 配下です。v0.17のphysical builder/publisherもCoreへOCI/containerd/nerdctl/Incus/Btrfs vocabularyを持ち込まず、OCI/provider adapter boundaryに置きます。Incus固有のharvest mechanicsはOCI plugin/CoreではなくIncus provider/runner boundaryに置きます。

## Btrfs/COW boundary

HacocoonはIncus/storage-driver cloningを利用し、CoreやSeed GCからIncus-owned Btrfs subvolumeを直接操作しません。Btrfsではunchanged blockのCOW sharingを期待できますが、non-COW backendでは同等のphysical savingをclaimしません。

## Security requirements

shared writable containerd root禁止、coding Environment/Seed BuilderへのHost containerd/Docker socket禁止、Incus/Hacocoon control socket禁止、Builderのarbitrary upstream network禁止、registry acquisitionはtrusted Host側、credential-free harvestはexact immutable identityかつ明示marker付きrunning managed Environmentだけ、転送はtemporary OCI archiveだけ、credential file/helper output/workspace/arbitrary file/live containerd stateのcopy禁止、reusable credentialをSeedへ埋め込まない、option-like OCI reference拒否、immutable sha256 digest必須、deletion tombstoneをpin/recommendationより優先、partial/deletion-raced buildをcurrentにしない、cleanup ambiguityはrecovery-required、ownership/dependency evidenceが曖昧なold Seed GCはretainを優先します。

## 残るacceptance / follow-up

repository sliceだけではv0.17 completeではありません。残件は次です。

- real supported-host Incus + containerd + nerdctl acceptance
- Tooling Base pathのreal Docker Engine compatibility acceptance
- Host-owned credentialを使うauthenticated/private-registry combinationを、Seed Builder/coding Environmentへのcredential leakなしでvalidate
- physical Btrfs COW/block-sharing measurement
- publication/restart/cleanup/harvest/storage behaviorに対するbroader real-host failure-injection coverage
- Incus/Btrfs-backed trusted acquisition/cacheがcache → builder → Seedのblock reuseを実測上改善するか評価し、測定で有効な場合だけoptional採用を検討

## 他milestoneとの関係

- v0.15: OCI pluginでSeed候補をrecommend/select
- v0.16: OCI pluginでfuture Seedからtombstone/delete
- Optional Local Registryはprerequisiteではなくmilestoneも予約しない
- v0.17: actual immutable Seed build/publish/COW lifecycleとrepository-side maintenance/acquisition semanticsを担当
- v0.18 Docker Compatibilityはrepository integration実装済み。CLI/Engine compatibilityはoptionalかつCore外のまま扱う
