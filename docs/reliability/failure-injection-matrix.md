# Cross-cutting failure-injection matrix

This is the executable-work map for [#381](https://github.com/SLktEx/Hacocoon/issues/381),
reconciled against main `e85ebab58885e6117307a698000225ad93b1547e`.
After an interrupted multi-step operation, the next invocation must converge to
exactly one of these outcomes:

1. the operation is complete and authoritative state agrees with reality;
2. the previous safe state remains authoritative and retry completes safely;
3. ownership is ambiguous, so Hacocoon retains conservative ownership and fails
   closed with actionable recovery diagnostics.

Use existing canonical receipt boundaries and test-only semantic failpoints,
not source-line injection, timing sleeps or production interruption switches.
This map separates returned errors with fake providers, production JSON catalog
reopening, fresh test processes, real sockets/Incus and shipped/installed product
journeys. A source link identifies coverage, not proof that a native test ran.
The [current-main race job](https://github.com/SLktEx/Hacocoon/actions/runs/38064303571/job/114248719450)
covers repository tests; the bounded native job links below ran at main
`75a7132cdc233d45d1cfcc89256cc36ce763a47b`. Neither completes the remaining matrix.
[Acceptance evidence](../status/acceptance-evidence.md) owns detailed native
results, unresolved failures and skips. The reviewed records for
[socket cleanup #761](https://github.com/SLktEx/Hacocoon/pull/761),
[bare signals #762](https://github.com/SLktEx/Hacocoon/pull/762) and
[transport lifetime #763](https://github.com/SLktEx/Hacocoon/pull/763) retain their
newer bounded test results, failures and limits.

## Environment create

[Returned-error tests](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/workspace/recovery_failpoints_test.go) use the
production JSON store and a fake runtime:
`TestEnvironmentCreateDurableBoundaryFailuresConvergeOnRetry` and
`TestEnvironmentCreateLostReservationResponseFailsClosed`.
[Fresh-process coverage](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/workspace/recovery_process_linux_test.go) is
`TestEnvironmentLifecycleProcessExitRecovery`; its provider is a file-backed
fixture, not Incus. See [fixture fidelity](#lifecycle-process-termination-fixture).

| Semantic boundary | Existing coverage | Remaining fixture / native observation | Required recovery result |
| --- | --- | --- | --- |
| before durable Workspace/Environment reservation | returned error + fresh process | representative real-Incus lifecycle interruption | no state/runtime; retry succeeds |
| after durable reservation, before provider side effects | lost reservation response + fresh process | real-Incus reservation/lease comparison | retain reservation; do not create a second runtime or guess ownership |
| before runtime ownership persistence | returned error + fresh process | real-Incus init/receipt interruption | returned error permits exact cleanup; process exit retains unreceipted reservation and refuses guessed cleanup |
| after runtime ownership persistence | returned error + fresh process | real-Incus receipt/configuration interruption | returned error permits bounded cleanup; process exit retains exact ref and RW lease for explicit deletion |
| provider configuration after init | `TestCreationReceiptPrecedesConfigurationAndProtectsFailedCleanup` in [receipt tests](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/workspace/creation_receipt_test.go); fake-runner `TestSandboxReceiptFailureLeavesCleanupToLifecycleOwner` in [Incus receipt tests](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/adapters/incus/sandbox_test.go) | selected device/network/start/Ready failures on real Incus; see storage/network rows | receipt precedes fallible configuration; lifecycle owns cleanup; created is not Ready |
| before Ready Environment + active lease commit | returned error + fresh process | real-Incus publication interruption | returned error cleans up; process exit retains acquiring ownership for explicit deletion |
| after Ready commit but caller loses response | returned error + fresh process | real-Incus lost acknowledgement | returned error cleans up; process exit preserves Ready aggregate, refuses duplicate creation and permits exact deletion |

The [lifecycle ownership contract](../adr/0002-environment-lifecycle-ownership.md)
already requires the early provider receipt; this is not a missing implementation.
Representative native cases must also observe affected Workspace/Store reservations
and exact runtime/device identities, rather than treating normal lifecycle success
as fault-injection acceptance.

## Environment delete

`TestEnvironmentDeleteDurableBoundaryFailuresConvergeOnRetry` in the
[returned-error fixture](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/workspace/recovery_failpoints_test.go) and the
[fresh-process fixture](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/workspace/recovery_process_linux_test.go) cover
before/after runtime deletion and before/after authoritative finalization.

| Semantic boundary | Existing coverage | Remaining fixture / native observation | Required recovery result |
| --- | --- | --- | --- |
| before runtime delete | returned error + fresh process | real-Incus interruption before deletion | authoritative ownership remains; retry succeeds |
| after runtime delete but caller loses response | returned error + fresh process | real-Incus absence and network-cleanup observation | retain ownership until positive complete deletion; retry finalizes |
| absent runtime with unfinished provider cleanup / lease release | JSON/fake-runtime `TestDeleteRetainsAggregateWhenAbsentRuntimeStillNeedsCleanup` in [cleanup outcomes](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/workspace/cleanup_outcome_test.go) | native source-guard cleanup failure and retry | retain exact aggregate/lease until provider cleanup is complete |
| Environment-owned Workspace cleanup after finalization | JSON/fake-provider `TestEnvironmentDeleteRetriesOwnedDataAfterRuntimeRemoval` in [owned-data lifetime](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/workspace/creation_lifetime_test.go) | native owned-data cleanup failure and retry | Env/lease are finalized; retain the separate exact `OwnedWorkspaceCleanup` receipt, retry only owned data and do not repeat runtime deletion |
| SSH/forward teardown | scoped connection regressions [below](#controller--process-restart) | interrupted Environment-delete fixture with active native connections | repeated teardown preserves unrelated grants/forwards and cannot erase concurrent setup |
| shared Incus storage | attachment/absence guards [below](#storage--host-failures) | deletion with unavailable pool / unexpected consumers | never adopt a replacement or destroy the shared pool lifecycle |
| before authoritative finalization | returned error + fresh process | native runtime-absent / state-write failure | ownership remains retryable; finalize atomically |
| after finalization but caller loses response | returned error + fresh process | native final acknowledgement loss | retry is idempotent success |

## `haco setup`

The [trusted Host contract](../design/trusted-host.md) owns reconciliation and
[controller transport](../design/controller-client-transport.md#host-setup-observation)
owns setup observation. Existing fake-runner and in-process coverage is bounded:

| Semantic boundary | Representative tests / fixture | Remaining native observation | Required recovery result |
| --- | --- | --- | --- |
| ownership, create/start and controller endpoint | [host tests](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/adapters/incus/host_test.go): `TestEnsureTrustedHostStartsStoppedOwnedInstance`, `TestEnsureTrustedHostRefusesUnownedNameCollision`, `TestEnsureTrustedHostRejectsUnexpectedControlProxy`, `TestEnsureTrustedHostRecoversConcurrentCreateOfOwnedInstance` | interrupted create/start/endpoint reconciliation | reuse only exact owned Host; refuse collisions and authority drift |
| same-release client provisioning | [setup tests](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/adapters/incus/host_setup_test.go): `TestHostSetupReusesOwnedHostAndRecoversPartialClientInstall`; [product-client tests](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/adapters/incus/host_product_client_test.go): `TestProvisionTrustedHostProductClientPublishesOnlyVerifiedStaging` | native interrupted push/verification/publication and explicit retry | preserve ownership/data; reuse verified clients; staging publication coverage does not generalize to every companion |
| concurrent or lost-client setup | [setup API tests](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/controller/api/setup_test.go): `TestSetupRejectsConcurrentCallsAndAllowsExplicitRetry`, `TestSetupLostClientDoesNotAllowOverlappingMutation`; [progress tests](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/controller/api/setup_progress_test.go): `TestSetupProgressDisconnectKeepsExclusion` | actual client termination during setup | no overlapping mutation or automatic replay; exclusion lasts until service completion |
| transport ends while setup continues | actual `RegisterSetup` over `net.Pipe`: [TestSetupTransportLifetimeKeepsExclusionUntilServiceReturns](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/controller/api/setup_transport_lifetime_test.go) | independent controller/Host interruption with durable partial state | preserve original parent cancellation/timeout and busy exclusion; EOF is not success |
| storage ensure and usable-state validation | storage rows below; [installer stage-order tests](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/tools/test_install_network.py) | selected native setup interruption, then ordinary-user state/access check | setup completion alone does not prove Host readiness |

The ungated restart block in [the native journey](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/test/e2e/incus.sh#L226-L245)
stops the owned Host, observes STOPPED, invokes ordinary setup, then checks RUNNING,
client mode and controller round trips. Its [Core job](https://github.com/SLktEx/Hacocoon/actions/runs/38060429590/job/114237414155)
reached the final PASS. This existing restart is narrower than independent Host
and controller restarts around a durable lifecycle transition.

## Controller / process restart

| Interruption / ownership boundary | Existing coverage and fidelity | Remaining observation |
| --- | --- | --- |
| controller lifecycle before/after durable transitions | fresh lifecycle test processes below; [TestControllerRestartPreservesManagementPolicy](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/controller/startup_linux_test.go) restarts the controller in-process | kill/restart the actual controller during selected operations; retain ordinary-user state and exact durable ownership |
| bare controller SIGINT / SIGTERM | [controller_signals.py](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/test/e2e/controller_signals.py) runs the shipped bare executable; [command job](https://github.com/SLktEx/Hacocoon/actions/runs/38060429597/job/114237582533) logs both PASS | active-operation interruption; graceful idle shutdown is not crash recovery |
| Unix endpoint cleanup | [cleanup tests](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/controller/transport/unix_cleanup_test.go): native-socket `TestListenUnixCleanupOwnership`, `TestListenUnixPermissionFailureCleanup`, `TestListenUnixConcurrentClose`; separate file-backed `TestUnlinkListenerCleanupOwnership` / replacement / repeated-close fixtures | interruption during real lifecycle work; trusted-parent precondition and non-atomic pathname operations still apply |
| accepted control transports / cancellation watcher | in-memory [shutdown tests](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/controller/transport/server_shutdown_test.go): `TestServeClosesAcceptedTransportsBeforeReturning`, `TestServeCancellationWatcherEndsWithServe`; [ownership tests](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/controller/transport/server_shutdown_ownership_test.go): `TestServeWaitsForAcceptedTransportClose`, `TestServeSharesCapacityUntilHandlerActuallyReturns` | reusable component lifetime is covered; no post-process-exit FD/provider-leak or arbitrary-handler completion claim |
| connection / forward cleanup | [TestDisconnectClosesAllStreamsForOnlyTheRequestedGrant](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/client/stream_test.go), [TestUnforwardRevokesSSHCredentialAndTransport](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/client/service_test.go), [TestDeletedEnvironmentCleanupProtectsConcurrentSetup](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/client/ssh/cleanup_test.go), [TestServeTCPWaitsForCancellationCloseCompletion](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/network/client_close_test.go) are component tests | native disconnect/termination during Environment deletion; check only owned transports close and unrelated grants survive |
| client, trusted Host and Incus independent restart | bounded setup/native Host restart above | selected client process death, Host/controller independent restart and CI-safe daemon restart at durable boundaries; no privilege widening or guessed cleanup |

Bare signal tests check readiness, zero exit, endpoint removal and retained private
fixture data. Native Unix-socket tests exercise actual socket identity; their
file-backed counterparts alone do not. Accepted-transport cleanup in #763 closes
only that Serve invocation's transports and joins its watcher; handlers retain
shared slots until they return. None of these substitutes for active-operation
process recovery.

## Lifecycle process termination fixture

[recovery_process_linux_test.go](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/workspace/recovery_process_linux_test.go)
reuses the test-only semantic boundaries from the returned-error fixture. A child
exits with checked code 86 at six create and four delete points. Cleanup/unlock
defers do not run. A fresh child opens the production JSON catalog and invokes
ordinary lifecycle methods; no sleep chooses the interruption point.

A separate file-backed provider inventory and operation log survive both processes.
Refs include the fresh Environment instance ID. Assertions compare exact lease,
Workspace identity, Ready metadata and inventory; check RW exclusion, refusal to
adopt/delete unreceipted resources, exact-owner cleanup, idempotent deletion and
safe recreation; and preserve an unrelated provider resource and Workspace file.

This proves fresh-process recovery for this Linux lifecycle/store fixture. It
runs neither the installed controller/client nor haco-host/Incus, proves no
power-loss durability, and supplies no OCI-copy/Base/snapshot/registry interruption
evidence. Those need their own representative fixtures at existing boundaries.

## Storage / host failures

These are **Incus-owned** pools. `LoopPool` test names do not restore the retired
Host raw/loop/mount lifecycle or storage helper.

| Failure | Existing lower-layer coverage | Remaining native observation / required result |
| --- | --- | --- |
| create/get/set/read-back failure or policy drift | fake runners: [TestEnsureDefaultIncusStoragePoolSurfacesCreateStderr](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/composition/storage_test.go); [TestEnsureBtrfsLoopPoolSurfacesMountOptionReconcileFailure and TestEnsureBtrfsLoopPoolFailsClosedOnUnverifiedPolicy](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/adapters/incus/storage_loop_policy_test.go) cover failed initial read, failed read-back and stale policy | selected real-Incus failures with exact pool identity/configuration; surface failure and never publish Ready on unverified policy |
| unavailable attachment / missing pool | fake runner [TestStorageAttachmentUnavailableDoesNotCreateReplacement](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/adapters/incus/storage_attachment_test.go) | native missing/unavailable pool; refuse replacement/adoption and preserve owned state |
| full/quota-exhausted or read-only storage; daemon loss | no representative native fault acceptance claimed by ordinary Btrfs success | isolated real-host fixture; previous safe state or recovery-required, bounded diagnostics and retry without guessing Host block state |
| ambiguous inventory / cleanup refusal | fake-runner [observation tests](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/adapters/incus/observation_test.go): `TestInstanceAbsenceRequiresCompleteExactInventory`, `TestCanceledObservationCannotProveAbsence`, `TestDeleteCannotUseTruncatedInventoryAsAbsence` | real observation failure / unexpected consumers; conservative retention until complete exact absence, not attempted cleanup |

See [storage evidence](../status/acceptance-evidence.md#storage) for scoped native
success. It is not acceptance of every storage failure above.

## Network / runtime failures

The [Environment lease/start contract](../design/workspace-abstraction-and-lease.md)
and [egress contract](../design/egress-authorization.md) own safety requirements.

| Failure | Existing lower-layer coverage | Remaining native observation / required result |
| --- | --- | --- |
| network owner/NIC/isolation/source-guard/autostart drift | fake-runner [TestResumeValidatesNetworkBeforeStartingAndPreservesRuntime](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/adapters/incus/start_test.go) | selected real bridge/NIC/security drift before/after start; no weakened isolation, runtime replacement or false Ready |
| failed configuration / writable-Workspace readiness | fake-runner [TestSandboxProviderMarksManagedEnvironmentBeforeStart](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/adapters/incus/sandbox_managed_identity_test.go), plus creation-receipt tests above | selected native device/security/probe failures; durable receipt and bounded exact cleanup or recovery-required |
| Incus API failure, disconnect or start timeout | fake-runner receipt, observation and cleanup cases above are narrower than a real timeout | isolated native API/daemon-loss/start-timeout fixture; compare affected runtime/device/network and lease state |
| controller/client connection dropped mid-operation | transport/setup/connection components above | real process interruption with an active operation; durable transition, not connection loss, determines recovery |

## OCI acquisition

The current acquisition path is ordinary Host containerd/nerdctl from
[host_tooling.py](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/adapters/incus/host_tooling.py), governed by
[standard Host tools](../design/trusted-host.md#standard-host-tools). There is no
current Hacocoon pull/save-export/auth-file coordinator or maintained authenticated
registry fault fixture to treat as already covered.

| Boundary | Existing coverage | Remaining fixture / observation |
| --- | --- | --- |
| Host runtime pull/inspect/use/restart | [TestRealIncusHostToolingE2E](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/adapters/incus/host_tooling_e2e_test.go) passed in the [Host-tooling job](https://github.com/SLktEx/Hacocoon/actions/runs/38060429590/job/114237414056) | selected Host/containerd stop or acquisition/export interruption; preserve valid prior data and temporary-material ownership |
| managed inspection/deletion and copy exclusion | fake-runner [TestHostImageExecutionRequiresExactUnprivilegedSource and TestHostImageExecutionUsesCopyOperationLock](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/adapters/incus/host_oci_images_test.go); service fixture [TestManagedImageDeletionGuardsAndRuntimeIdentity](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/storage/oci/managed_images_test.go) | failed/truncated native observation must not authorize guessed deletion or bypass pending-copy exclusion |
| private registry, expired authentication and partial archives | [installed OCI evidence](../status/acceptance-evidence.md#installed-oci-cli) explicitly leaves authenticated acquisition/credential cleanup incomplete | prepared, authorized registry/runtime fixture; record failure or unavailable-fixture SKIP honestly and verify no credential leakage into Env data, copies or diagnostics |

Normal Host-runtime pull/export/authentication remains relevant to #381. The
[retired Seed registry job](../status/acceptance-evidence.md#main-seed-retirement)
is not current acceptance and must not be revived to fill this row.

## OCI copy

The [copy contract](../design/persistent-oci-store.md#retrying-a-positively-completed-copy)
distinguishes unknown completion from a positively completed durable receipt.

| Boundary | Existing coverage and fidelity | Remaining native observation / required result |
| --- | --- | --- |
| pause/copy/resume, socket activation/autostart and inverse operations | fake runners: [TestHostAreaCopyPausesExactOwnerAndRestoresOnlyAfterCompletion and TestHostEntryCannotRestartAnInterruptedCopy](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/adapters/incus/host_oci_copy_test.go); [TestCompletedHostCopyRecoveryFailsClosedAndRestoresExactGuard](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/adapters/incus/host_oci_recover_test.go) | actual process/provider interruption while completion is unknown; retain exact source/target/journal and keep Host writers fenced |
| source reservation, publication and lost acknowledgement | production JSON with fake provider: [TestCopyReservesExactSourceUntilVerifiedCommit](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/storage/resource/copy_test.go); [TestCompletedCopyRecoversAfterReopeningStateWithoutRecopying and TestCopyRecoveryResolvesCommitUncertaintyFromDurableIdentity](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/storage/resource/recover_test.go) | fresh-process interruption at selected copy/receipt/commit boundaries; do not infer completion from transport loss or recopy a confirmed target |
| positively completed copy with failed Host resume | [TestRealIncusCompletedHostCopyRecoveryE2E](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/adapters/incus/persistent_copy_e2e_test.go) passed in the [recovery job](https://github.com/SLktEx/Hacocoon/actions/runs/38060429590/job/114237414328) | real Incus/catalog writes, injected first Host-start failure via runner, then catalog reopen **in the same process**; actual controller death and unknown completion remain separate |

The [installed OCI journey](../status/acceptance-evidence.md#installed-oci-cli)
adds successful CLI copy/deletion/recreation, not interruption acceptance. Never
restart an unknown-completion writer, attach guest-populated data back to Host,
or clear a guard merely because cleanup was attempted.

## Base / snapshot / import

Use current [Base publication](../design/base-images-and-custom-environments.md#security-and-failure),
[snapshot receipts](../design/environment-snapshots.md#capture-and-ownership-boundary)
and [transfer cleanup](../design/environment-transfer.md#registration-and-failed-cleanup)
boundaries. New snapshots do not regain a historical BaseAsset retention dependency.

| Boundary | Existing deterministic coverage | Remaining fixture / native observation |
| --- | --- | --- |
| Base build/archive import/publication | fake lifecycle/runner: [TestBuildPreservesBoundariesAndFailureEvidence](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/base/build/service_test.go), [TestBaseArchiveImportSharesBuilderCleanupAndPublication](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/base/build/import_linux_test.go), [TestPublishBaseRecordsOwnershipBeforeVerificationAndPreservesImages](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/adapters/incus/base_build_test.go) | selected native interruption/publication uncertainty/cleanup failure; preserve builder and native evidence on ambiguity, and published image on builder-cleanup failure |
| snapshot plan/reserve/create/receipt/verify/commit | production JSON + fake runtime: [TestSnapshotCaptureOrdersDurableReceiptsAndRetainsEveryFailure, TestSnapshotCaptureCancellationAndAmbiguousCleanup and TestSnapshotCaptureKeepsReservationWhenRecoveryWriteFails](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/workspace/snapshot_capture_test.go) | selected real capture interruption and recovery-write/cleanup failure; retain exact receipts/source reservations and preserve source state |
| restore / bundle import / fresh data ownership | service/provider fixtures: [TestRestorePublicWorkflowAndFailureOwnership](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/snapshot/restore/service_test.go), [TestBundleImportOwnsDataAndFailureCleanup](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/env/transfer/import_linux_test.go); JSON + fake backend: [TestSnapshotRestorePersistenceFailureCleansOnlyItsOwnedCopy](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/storage/resource/restore_test.go), [TestResourceImportUsesCanonicalFreshOwnershipAndRetainsFailure](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/storage/resource/import_test.go) | selected native process interruption, partial archive, lost response and ambiguous cleanup; fresh authority, retained uncertain ownership and no rollback of a published Env on start failure |
| native volume/rootfs import boundary | fake runner/API: [TestVolumeImportNativeBoundaryRefusesAmbiguityAndRetainsFailures](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/adapters/incus/transfer_import_volume_linux_test.go), [TestNativeRootfsImportOwnershipAndCleanup](https://github.com/SLktEx/Hacocoon/blob/e85ebab58885e6117307a698000225ad93b1547e/internal/adapters/incus/transfer_import_image_linux_test.go) | real native failures at these ownership boundaries; method names containing “Native” alone do not prove real-provider execution |

Existing native logs show [Base build / owned-volume import](https://github.com/SLktEx/Hacocoon/actions/runs/38060429590/job/114237414190),
[snapshot aggregate, including shipped-controller import](https://github.com/SLktEx/Hacocoon/actions/runs/38060429590/job/114237414253)
and [Host-area/persistent copy and rootfs/volume transfer](https://github.com/SLktEx/Hacocoon/actions/runs/38060429590/job/114237414288)
PASS. [Transfer evidence](../status/acceptance-evidence.md#transfer) retains failures
and skips. Successful transfers do not establish every native interruption or
arbitrary live-application consistency, nor a new automatic crash-replay protocol.

## Zombie-state assertions

For each selected boundary compare the affected subset of:

- authoritative Environment metadata;
- Workspace/Store lease or reservation, owner, access mode and runtime reference;
- exact Incus project/instance/device inventory;
- Environment network ownership marker and guard state;
- Incus-owned Btrfs pool identity and configuration;
- connection/forward state;
- trusted Host ownership markers and copy writer guards;
- user/root ownership of lock/state files;
- temporary runtime/build/archive material and durable receipts.

Do not require unrelated fields to change or compare every field in every test.
Conservative retention is acceptable when ownership is ambiguous. Silent adoption,
silent loss of ownership or deletion based on guessed ownership is not.

## CI layering

Required PR gates remain bounded:

```text
fast deterministic recovery tests
  -> representative real-Incus/storage fault cases
  -> authoritative installed user-journey acceptance
```

Reuse [existing CI contracts](ci-contracts.md) and canonical failpoint boundaries;
a new common harness is not a prerequisite for every row. Promote deterministic
regressions from broader scheduled/manual cases to the lowest faithful required
layer. Record exact commit/run, sanitized diagnostics and PASS/FAIL/SKIP for the
selected native subset. Missing credentials or infrastructure are unaccepted
cases, never passing recovery evidence. Mapping the work does not complete #381's
remaining fixture, native interruption or authenticated-registry criteria.
