# v0.16 — OCI Image Deletion

Status: **first deletion sliceは`main`に実装済み。immutable Seed rebuild/GCはv0.17。**

v0.16はoptional OCI plugin配下で、trusted Host Seed cacheと明示指定時のmanaged Environmentに対するOCI image deletion semanticsを提供します。

## CLI

```text
haco plugin oci image delete <reference[@sha256:...]> [--all-environments] [--json]
```

pre-1.0で廃止した `haco image ...` はaliasとして残しません。

defaultではHost Seed cacheとfuture Seed selectionだけに作用し、existing Environmentは書き換えません。

`--all-environments` を付けた場合だけ、managed Environment全体から同じimmutable identityの削除を試みます。

## Immutable deletion identity

削除identityは `reference + immutable digest` です。

mutable referenceから複数digestが観測されている場合はfail closedし、明示的な `reference@sha256:...` を要求します。削除直前にもtagをrevalidateし、tagが新digestへ移動していれば古いtargetとして削除しません。

## Host Seed cache

current first sliceはHostのdedicated nerdctl namespace/cache `hacocoon-seed` を対象にします。

1. target identityをvalidate
2. matching local referenceがあれば通常の`nerdctl rmi`で削除
3. trusted deletion tombstoneをpersist
4. 同一identityをv0.15 recommendation/auto promotionから除外
5. immutable Seed replacementとold-Seed GCはv0.17へ委譲

published immutable Seedをin-placeで変更しません。

## All-Environment deletion

`--all-environments` はdestructive change前に全Environmentをpreflightします。provider-neutral execution pathを使い、`nerdctl rmi --force` は使いません。

containerが参照中のimageは安全側に失敗できます。partial completionはrecovery-requiredとしてsurfacedし、retry時のalready-absent imageはno-opです。

## Tombstone

TombstoneはSeed-selection overrideでありruntime/network denylistではありません。

- telemetryはimageを観測してよい
- recommendation/auto promotionは同一identityを再追加しない
- Environmentは通常のauthority範囲でpull/use可能
- future Seedへ戻すにはexplicit operator overrideが必要

## Security requirements

mutable-tag ambiguityはfail closed、moved tagをstale identityとして削除しない、Host credentialをEnvironmentへcopyしない、Environment credentialをharvestしない、all-Environment deletionはexplicit、`--force`禁止、partial destructive workはrecovery-requiredとして扱います。
