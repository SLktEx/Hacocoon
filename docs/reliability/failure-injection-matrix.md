# Cross-cutting failure-injection matrix

This document is the executable-work map for #381. The goal is not to make every failure look successful. After an interrupted multi-step operation, the next invocation must converge to exactly one of these outcomes:

1. the operation is complete and authoritative state agrees with reality;
2. the previous safe state remains authoritative and retry completes safely;
3. ownership is ambiguous, so Hacocoon retains conservative ownership and fails closed with actionable recovery diagnostics.

Timing-based sleeps are not canonical fault injection. Prefer semantic before/after boundaries and compare authoritative state with the owned external resources that boundary can affect.

## Environment create

| Semantic boundary | Fast deterministic coverage | Real-substrate coverage | Required recovery result |
| --- | --- | --- | --- |
| before durable Workspace/Environment reservation | `recovery_failpoints_test.go` | user journey | no state/runtime; retry succeeds |
| after durable reservation, before provider side effects | `recovery_failpoints_test.go` | selected real-Incus case planned | reservation retained; no second runtime is created; fail closed until ownership can be resolved |
| provider instance created but later provider reconciliation fails | existing Incus cleanup tests; provider-ref retention work pending | real-Incus case planned | exact runtime identity retained when known; cleanup or recovery-required |
| before runtime ownership persistence | `recovery_failpoints_test.go` | selected real-Incus case planned | created runtime cleaned up; reservation finalized; retry succeeds |
| after runtime ownership persistence | `recovery_failpoints_test.go` | selected real-Incus case planned | bounded cleanup; retry succeeds or recovery-required retains exact ref |
| Incus storage pool ensure/use | Incus storage runner tests; common failpoint adapter pending | Incus-owned Btrfs E2E | conflicting or unavailable pool state cannot become Ready silently |
| network/profile/device reconciliation | Incus unit/component tests exist; semantic failpoint expansion pending | real-Incus Core E2E | incomplete security attachment cannot become Ready |
| instance start / readiness verification | Incus tests cover cleanup and RW probe; semantic failpoint expansion pending | real-Incus Core E2E | created != Ready; cleanup or retained recovery ownership |
| before Ready Environment + active lease commit | `recovery_failpoints_test.go` | selected real-Incus case planned | runtime cleaned up; retry succeeds |
| after Ready commit but caller loses response | `recovery_failpoints_test.go` | selected real-Incus case planned | no zombie aggregate; cleanup/retry converges deterministically |

## Environment delete

| Semantic boundary | Fast deterministic coverage | Real-substrate coverage | Required recovery result |
| --- | --- | --- | --- |
| before runtime delete | `recovery_failpoints_test.go` | selected real-Incus case planned | authoritative state remains; retry succeeds |
| after runtime delete but caller loses response | `recovery_failpoints_test.go` | selected real-Incus case planned | state retains ownership until retry confirms absence and finalizes |
| connection/forward teardown | pending common connection failpoint seam | user journey / real-Incus | retry-safe; unrelated forwards untouched |
| Workspace detach / lease release | lifecycle aggregate prevents independent lease release; broader failpoint case pending | real-Incus | RW lease retained until runtime absence is proven |
| shared Incus storage remains available | provider/storage tests pending | Incus-owned Btrfs E2E | Environment deletion never guesses at or destroys the shared pool lifecycle |
| before authoritative state finalization | `recovery_failpoints_test.go` | selected real-Incus case planned | runtime already absent; retry finalizes atomically |
| after authoritative state finalization but caller loses response | `recovery_failpoints_test.go` | selected real-Incus case planned | retry is idempotent success |

## `haco host ensure`

| Semantic boundary | Fast deterministic coverage | Real-substrate coverage | Required recovery result |
| --- | --- | --- | --- |
| trusted-host ownership/reconciliation | pending | existing trusted-host/installer journeys | never adopt unrelated `haco-host` authority |
| Incus-owned storage pool ensure | Incus storage tests; common harness pending | Incus-owned Btrfs E2E | retry uses only the supported pool identity and fails on unresolved conflict |
| host instance create/start | pending semantic failpoints | installer + real Incus | partial instance is repaired or retained as recovery-required |
| same-release binary provisioning | pending | installer journeys | old valid host remains usable until replacement accepted where applicable |
| controller endpoint/control channel | pending | installer/user journey | restart/retry restores exact authority boundary |
| final usable-state validation | pending | installer/user journey | command completion alone never means host Ready |

## Controller / process restart

| Interruption | Deterministic layer | Real-substrate layer | Required recovery result |
| --- | --- | --- | --- |
| controller restart before durable create transition | pending process harness | selected user journey | client receives failure/reconnect; no partial ownership loss |
| controller restart after runtime ownership transition | pending process harness | selected user journey | next invocation sees durable ownership and recovers safely |
| client process termination | pending process harness | selected user journey | server-side durable state remains authoritative |
| trusted `haco-host` restart | pending | selected installer/host journey | no privilege/ownership widening |
| Incus restart where CI-safe | pending | scheduled/manual real-host matrix | retries converge without guessed cleanup |

## Storage / host failures

| Failure | Lower faithful layer | Real-substrate layer | Required recovery result |
| --- | --- | --- | --- |
| Incus storage create/show failure | Incus runner tests; common recovery harness pending | Incus-owned Btrfs E2E | exact Incus error is surfaced; Environment/host is not published Ready |
| requested pool policy rejected by Incus | Incus storage tests | Incus-owned Btrfs E2E | fail closed without switching to an unrelated Host pool |
| ENOSPC/quota-style exhaustion | pending | scheduled/manual | previous safe state remains or recovery-required |
| read-only/unavailable Incus storage | pending | selected real-host | no successful Ready publication |
| Incus restart with pool unavailable | pending | selected real-host | retry follows Incus-reported pool reality instead of guessing Host block state |
| cleanup refusal with unexpected Incus resources | existing fail-closed cleanup guards; cross-cutting assertion pending | selected real-host | conservative retention + actionable diagnostics |

## Network / runtime failures

| Failure | Lower faithful layer | Real-substrate layer | Required recovery result |
| --- | --- | --- | --- |
| Incus command/API failure | Incus fake-runner tests | real-Incus Core E2E | structured failure; owned resources retained/cleaned deterministically |
| instance start timeout | Incus cleanup tests | selected real-Incus fault case planned | bounded cleanup or recovery-required |
| device/profile reconciliation failure | Incus tests | selected real-Incus fault case planned | Environment never Ready with incomplete security materialization |
| managed network unavailable/drifted | ownership guard tests | installer/network E2E | unowned/mismarked resources never adopted/deleted |
| dropped controller/client connection mid-operation | pending process harness | selected user journey | durable transition determines recovery, not connection state |

## Zombie-state assertions

Every failure case should assert the relevant subset of:

- authoritative Environment metadata;
- Workspace lease state, owner, access mode, and runtime reference;
- Incus project/instance/device inventory;
- Environment network ownership marker and guard state;
- Incus-owned Btrfs pool identity and configuration;
- connection/forward state;
- trusted-host ownership markers;
- user/root ownership of lock/state files;
- temporary runtime/build files.

A conservative retained resource is acceptable when ownership is ambiguous. A resource that is silently adopted, silently forgotten, or deleted based only on guessed ownership is not.

## CI layering

Required PR gates should remain bounded:

```text
fast deterministic recovery tests
  -> representative real-Incus/storage fault cases
  -> authoritative installed user-journey acceptance
```

Broader combinatorial storage/restart/fault permutations may run scheduled or manually, but deterministic regressions discovered there must be promoted to the lowest faithful required layer.
