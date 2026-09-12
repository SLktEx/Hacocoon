# OCI Seed Builder & Btrfs/COW Optimization

> 旧CLIの任意連携です。以下のコマンドはPhysical Hostの移行用 `hacoq` で使います。現行の通常操作は[CLI参照](../reference/cli.ja.md)と[移行情報](../reference/cli-migration.md)を参照してください。

[English](oci-seed-and-cow.md) | **日本語**

Status: **リポジトリのbuild/publish、operations-hardening、credential-free managed-Environment 回収 sliceを実装済み / 部分実装。v0.15 推奨とv0.16 deletion 方針は実装済みprerequisiteです。real-host、authenticated/private-registry combination、physical COW 検証が残っています。**

v0.17はphysical OCI Seed pipelineを担当します。信頼された Host 側イメージ acquisition/cache、offline Seed construction、不変の公開、revision pinning、storage-driver COW benefit、保守的なライフサイクル maintenanceを一つのfeature gateとして扱います。

Local Registryは必須ではありません。

2026-09-05の方針: Seed撤去は **未実装の計画** であり、削除済みではない。新しいSeed依存は追加しない。以降は残存実装と過去の受入範囲の説明として扱う。

`internal/composition` はOCI Plugin有効時のみSeed store/resolverを構成する。`modules/runtime/incus/base.go` は親Baseを独立解決してから任意の現在の Seed revisionを選ぶ。まずこの任意解決を分離して既存の固定Base revisionとデータを保持し、次にSeed コマンド・ビルダー・recommendation/harvest・関連test/docsを撤去する。Base選択・変更と任意Plugin契約は維持する。独立Workspace repo cloneは別のストレージ契約であり、Seed pipelineの後継にはしない。

## Goal

common OCI イメージを不変の Incus-derived Seedへpreloadし、通常の Incus/storage-driver clone 意味でunchanged ファイルシステム blockを再利用しつつ、各Environmentのwritable containerd 状態は独立させます。

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

- `hacoq plugin oci seed build [--base <base>] [--json]`
- `hacoq plugin oci seed current [--base <base>] [--json]`
- Tooling Base / 現在の Seed manifestのpersistとprocess-safe ビルド lock
- ビルド前の不変の parent Base resolution
- dedicated `hacocoon-seed` 名前空間を使う信頼された Host 側 OCI 取得
- プロファイルなし・NICなしのoffline Seed Builder
- live containerd 状態ディレクトリをコピーせず、対応している nerdctl/containerd interfaceでimport
- publish前の全選択した不変のダイジェスト確認
- 不変の Incus イメージ publish前のサービス clean 停止
- 公開とmanifest persist成功後だけcurrent-Seed pointerをadvance
- recorded parent Base revisionが現在の不変の parentと一致する時だけSeedを使うexact-parent resolution
- Host Docker/containerd ソケットをforwardせず、Tooling Base内でcontainerd + nerdctlとgenuine Docker CLI/Engine compatibilityを提供

## 実装済みoperations-hardening slice

- `hacoq plugin oci seed pin <reference@sha256:...> [--base <base>] [--json]`
- `hacoq plugin oci seed unpin <reference@sha256:...> [--base <base>] [--json]`
- `hacoq plugin oci seed pins [--base <base>] [--json]`
- `hacoq plugin oci seed gc [--json]`
- `hacoq plugin oci seed recover [--json]`
- `hacoq plugin oci image reenable <reference@sha256:...> [--json]`
- Base単位の明示的な不変の固定をpersistし、自動推奨とmerge
- deletion 削除指定は推奨と既存固定より優先し、正確な不変の識別を明示reenableするまで復活させない
- Seed publish後にもdeletion 状態を再確認し、長いビルド中に削除がraceしても現在の pointerを進めない
- process-safe Seed ビルド lockを保持したまま、次のビルド前にHacocoon-ownedの中断ビルダーを照合・調整
- old Tooling/Seed イメージ GCはHacocoon Incus projectとHacocoon-owned aliasに限定
- 現在の manifest revision/alias、instanceの `volatile.base_image`、Incus `used_by`、external aliasがあるイメージはretain
- 不正ななIncus/provider inventoryはdestructive 削除前に安全側で拒否
- GCは対応している Incus イメージライフサイクルだけを使い、Incus-owned Btrfs subvolumeを直接操作しない

## 実装済みcredential-free Environment harvest slice

正確な不変の OCI 識別がrunning中のHacocoon-managed Environmentに既に存在する場合、registry 認証情報をコピーせず、そのlocal 内容をSeed 取得へ再利用できます。

- 新規管理対象の Incus Environmentへ `user.hacocoon.kind=environment` 識別情報を付与
- 回収元データはその正確な識別情報を持つrunning instanceだけ
- 回収対象は正確な `reference@sha256:...` Seed pullだけ
- Environment内で正確な識別を `nerdctl save` し、randomな `/tmp` アーカイブを作成
- 信頼された Hostは `incus file pull` でそのOCI アーカイブだけをコピーし、guest側アーカイブをすぐ削除
- アーカイブはHostのdedicated `hacocoon-seed` 名前空間へloadし、正確な不変の識別をverify
- 識別情報のない既存/legacy Environmentは回収対象にしない
- 安全な回収が成立しない場合は従来の信頼された Host `nerdctl pull` へ代替経路し、Host 所有 registry 認証情報パスを維持

registry login ファイル、credential-helper 出力、workspace データ、任意のEnvironment ファイル、live `/var/lib/containerd` はコピーしません。転送するのは既にlocalにあるイメージ内容から作ったtemporary OCI アーカイブだけです。

## Mandatory isolation

ストレージ savingのために複数Environmentで一つのwritable `/var/lib/containerd` を共有してはいけません。各Environmentは独立したにmutable/deletable/recoverableである必要があります。

## Inputs

- v0.11の不変の parent Base revision
- v0.15 recommendation/auto-promotion + 明示的な運用者固定
- v0.16 deletion tombstone/override
- 利用可能ならmarked 管理対象の Environmentに既に存在する正確な不変の OCI 内容
- registry authenticationがなお必要な場合の信頼された Host 側上流認証情報

変更可能な OCI tagは入力 convenienceであり、Seed manifestと明示的な固定は不変のダイジェストをpersistします。

## Build lifecycle

1. process-safe Seed ビルド lockを取得し、プロバイダーがmaintenance対応なら中断ビルダーを照合・調整
2. Baseを不変の revisionへresolve
3. OCI 推奨とBase単位の明示的な固定からeffective Seed イメージ setを確定
4. deletion 削除指定にblockedされた不変の識別をreject
5. 各正確な不変の OCI 識別を信頼された Host Seed キャッシュへacquire。eligibleなmarked 管理対象の Environmentからcredential-free 回収を先に試し、必要なら信頼された Host registry 取得へ代替経路
6. 信頼された Host キャッシュからtemporary Seed BuilderへOCI export/stream
7. 固定済み Tooling Baseからgeneral ネットワークなし・NICなしでビルダー作成
8. 対応している containerd/nerdctl interfaceでimport/unpack
9. requested ダイジェストをverify
10. containerd/Docker compatibility サービスをclean 停止
11. ビルダー停止
12. 不変の Incus Seed revisionをpublish
13. 選択した不変の識別のdeletion 状態を再確認
14. Base / Tooling / Seed revisionとOCI ダイジェスト manifestをpersist
15. publication/validation成功後のみcurrent-Seed pointerをmove
16. publication/state persistenceやmaintenance 後始末が曖昧ならrecovery-required

## Pin / deletion / re-enableの優先順位

明示的な固定は、一つのlogical Baseについて将来のSeedへ正確な不変の OCI 識別を含める運用者指定です。明示的な deletionを上書きしません。

v0.16 deletion 削除指定は自動推奨と既存固定の両方より優先します。tombstoned 識別は `hacoq plugin oci image reenable <reference@sha256:...>` で正確な識別を明示reenableするまで再選択できません。変更可能な tagが別ダイジェストへ動いた場合に誤って別識別をreenableしないため、reenableは正確な識別だけを受け付けます。

## Recovery / GC

`hacoq plugin oci seed recover` は正確な Hacocoon temporary Seed/Tooling ビルダーを照合・調整してから、`seed gc` と同じ保守的なイメージ保持判定を実行します。configured バックエンドが対応する場合、`seed build` も新しいビルド開始前にinterrupted-builder 復旧を呼びます。

Hacocoon-ownedかつunusedであることを証明できないイメージは削除しません。現在の Seed/Tooling revision、protected alias、instance base-image fingerprint、Incus `used_by`、external aliasのいずれかがあればretainします。不正な inventoryは安全側で拒否 errorです。

## Plugin boundary

Seed observation/deletion/build/current/pin/maintenanceは `hacoq plugin oci ...` 配下です。v0.17のphysical builder/publisherもCoreへOCI/containerd/nerdctl/Incus/Btrfs vocabularyを持ち込まず、OCI/provider アダプター境界に置きます。Incus固有の回収 mechanicsはOCI plugin/CoreではなくIncus provider/runner 境界に置きます。

## Btrfs/COW boundary

HacocoonはIncus/storage-driver cloningを利用し、CoreやSeed GCからIncus-owned Btrfs subvolumeを直接操作しません。Btrfsではunchanged blockのCOW sharingを期待できますが、non-COW バックエンドでは同等のphysical savingをclaimしません。

## Security requirements

shared writable containerd root禁止、コーディング Environment/Seed BuilderへのHost containerd/Docker ソケット禁止、Incus/Hacocoon control ソケット禁止、Builderの任意の上流ネットワーク禁止、registry 取得は信頼された Host側、credential-free 回収は正確な不変の識別かつ明示識別情報付きrunning 管理対象の Environmentだけ、転送はtemporary OCI アーカイブだけ、認証情報 file/helper output/workspace/arbitrary file/live containerd 状態のコピー禁止、再利用可能な認証情報をSeedへ埋め込まない、option-like OCI reference拒否、不変の sha256 ダイジェスト必須、deletion 削除指定をpin/recommendationより優先、partial/deletion-raced ビルドを現在のにしない、後始末 ambiguityはrecovery-required、ownership/dependency evidenceが曖昧なold Seed GCはretainを優先します。

## 残るacceptance / follow-up

リポジトリ sliceだけではv0.17 completeではありません。残件は次です。

- real supported-host Incus + containerd + nerdctl 検証
- Tooling Base パスのreal Docker Engine compatibility 検証
- Host 所有認証情報を使うauthenticated/private-registry combinationを、Seed Builder/coding Environmentへの認証情報 leakなしでvalidate
- physical Btrfs COW/block-sharing 測定
- publication/restart/cleanup/harvest/storage behaviorに対するbroader real-host failure-injection 検証範囲
- Incus/Btrfs-backed 信頼された acquisition/cacheがキャッシュ → ビルダー → Seedのblock reuseを実測上改善するか評価し、測定で有効な場合だけ任意採用を検討

Host の Basic 認証によるレジストリ取得は、独立した[検証で成功](../status/seed-private-registry-acceptance.ja.md)しています。Seed／COW 全体や、すべての非公開レジストリの組合せを保証するものではありません。
