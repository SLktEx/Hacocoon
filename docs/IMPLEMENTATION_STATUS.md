# Implementation Status

## WSL delivery update — 2026-09-06

This candidate branch implements the following WSL slice; M0–M1 remains **partial**. Refreshed main was `e8974ef`, including #441/#442/#453/#456 and the #458/#459 image cache. A merged PR or historical green run does not accept this candidate.

- **Implemented storage:** Incus-owned Btrfs distribution and mount policy are the only path. Residual external `driver`/`source` attachments and ambiguous inspections fail closed. Rootfs/Workspace retention during policy reconciliation has a real-Incus CI contract; broader physical COW/compaction acceptance remains separate.
- **Implemented trusted networking:** the adapter owns `haco-host0`, validates ownership and configuration, creates fresh hosts without inherited profiles and preserves root disk/UUID during the bounded current-host NIC transition. The installer verifies trusted-host DNS, default route and HTTPS. Docker forwarding is limited to the owned bridge and established replies. See [ADR 0005](adr/0005-trusted-host-network-ownership.md).
- **Implemented installer:** managed `hacocoon` is the default, password input is unnecessary, and `-InteractiveUserSetup` is opt-in. Current reruns preserve account identity and password state, and create no sudo policy. PS5.1 native arguments, cache hash/promotion/reuse and read-only account-lookup retries have regressions. A failed native lookup does not establish account absence; the underlying intermittent WSL failure observed on `63fdf24` remains unconfirmed. See [bootstrap](WINDOWS_WSL_BOOTSTRAP.md) and [ADR 0004](adr/0004-wsl-installer-authority.md).
- **Partial product CLI:** new `haco` provides help/version, `setup`, `doctor`, and the controller-backed WSL login alias. It never delegates to `hacoq`. Startup waits for the existing controller; interactive process exit does not wait for stdin closure. Installer bootstrap now uses `haco setup` through the controller. The older lifecycle/Base/SSH surface remains migration work. The #456 controller adapter remains reusable.
- **Implemented controller diagnostics:** `haco doctor [--json]` checks runtime, configured Btrfs storage, owned running host/network and fixed trusted connectivity through a bounded read-only API. It neither repairs nor creates guest-local state. Failed/skipped checks now carry selected failure summaries and bounded next actions; fixed exit markers distinguish DNS/route/HTTPS failure without copying guest output. Missing or invalid actions and malformed responses cannot succeed. The cause/action projection passed the `663e2cd` hosted gates and local current-install checks below. The later two-minute controller readiness budget remains pending packaged acceptance. Release client/controller build identities must match in packaged acceptance. See [diagnostic contract](design/controller-client-transport.md#host-diagnostics).
- **Observed packaged acceptance — `663e2cd`:** the unmodified cached BAT applied the current package with exit 0. Subsequent ordinary WSL entry, both clients' five-check doctor reports, full build identities and the retained file/UUID/account/sudo-policy/Btrfs baseline passed. Direct cold doctor first exited 9 without a report. After a read-only listing confirmed stop, another cold invocation reached its 30-second controller readiness limit while Incus was still activating and the ordered controller was inactive; a later ordinary entry passed. Kernel logs also recorded WSL systemd's 10-second boot timeout. The CLI now allows two minutes for controller startup without retrying diagnostic checks. That later budget is not yet packaged-accepted; the separate exit-9 cause remains unconfirmed.
- **Repository validation — `b71f88e`:** maintained `ci-local.sh test` passed the full shuffled Go suite, vet, two JavaScript syntax checks and five notification tests. Focused race tests, installer stage/network/package checks, docs and workflow policy passed. Validation used Ubuntu Go 1.27.1 and the existing Windows Node runtime through a test-toolchain wrapper. GoReleaser and package checksums passed. These validation tools did not supply product environment overrides or installed-resource repairs.
- **Pending real-host acceptance:** Windows OS reboot, networking across firewall reload/startup orders, allowed proxy versus denied direct Environment traffic, SSH and Environment/Workspace work retention. Broader M1 layer diagnostics and reboot continuation remain partial. Firewall observations varied between Docker FORWARD DROP, ACCEPT without DOCKER-USER, and on `2d79778` ACCEPT with DOCKER-USER and scoped trusted-bridge rules. No acceptance-time repair was applied; startup order and the cold connectivity failure remain unverified. Isolated Linux packet gates do not establish live Windows Docker or installed Environment proxy acceptance.
- **Planned Seed retirement:** Seed code remains; [its Base/optional OCI dependencies](design/oci-seed-and-cow.md) are documented. Base selection and optional Plugins remain.
- **Commit-bound hosted acceptance — `663e2cd`:** [Windows exact BAT/restart/rerun/cold doctor](https://github.com/SLktEx/Hacocoon/actions/runs/34001263337), [Ubuntu installer](https://github.com/SLktEx/Hacocoon/actions/runs/34001266478), and [all four Incus jobs](https://github.com/SLktEx/Hacocoon/actions/runs/34001265234) passed. Windows fresh creation means an absent Hacocoon distribution on the runner's current-WSL substrate, not acceptance of disabled Windows features or an OS reboot. The two earlier harness fixes retain their negative regressions. Later product changes require their own acceptance.
- **Implemented controller-owned setup:** the installer uses `haco setup`; the legacy bootstrap path and guest hacoq provisioner are removed. Tests cover fixed validated companion sources, partial client provisioning, retained ownership, concurrent/lost-client requests, safe failures and installer stage ordering. Packaged and hosted results above cover this replacement. See [ADR 0006](adr/0006-controller-owned-host-setup.md).
- **Next concrete implementation:** accept the longer cold-start wait, then attach the existing Standard proxy lifecycle to the installed Physical Host controller. Complete remaining M1 layer/live-storage diagnostics and reboot continuation, keeping Environment direct traffic denied.

The historical checkpoint tables below retain their original milestone context.

Status date: 2026-08-31, after cloud deferral, the Base/OCI CLI split, Docker compatibility lifecycle integration, the OCI Seed Builder repository slices including credential-free managed-Environment harvest, the client-neutral interaction-event contract, the reusable client-adapter contract, domain-aware Standard egress enforcement, Incus-owned Btrfs rootfs pool integration, default Incus-managed Btrfs `zstd:3` compression, browser/native/VS Code notification clients, phased real-Incus CI acceptance on Ubuntu 26.04, the shared structured logging foundation, ordinary-user real-Incus storage acceptance, and the persistent trusted `haco-host` / default WSL entry slice.

This file reports **current code reality**, not desired architecture. Hacocoon is pre-1.0; implementation does not imply API stability, production support, or real-host acceptance beyond explicitly named acceptance checks.

The current milestone position is **v0.26**. Milestones are lightweight development checkpoints: v0.17 still has acceptance work, but that partial status does not block later implemented checkpoints such as v0.18-v0.26.

| Area | Current repository reality | Milestone |
|---|---|---:|
| Secure Workspace Runtime / Workspace leases | Incus-backed Environment lifecycle, canonical Workspace identity, RO/RW leases and recovery are implemented | v0.1-v0.2 |
| Client access | status, loopback forwarding and SSH preparation/revocation are implemented | v0.3 |
| Policy / Capability | fail-closed policy, approval and audit are implemented | v0.4 |
| Git / GitHub push | privileged push is brokered on the trusted Host without exporting reusable Host credentials | v0.5 |
| Agent / orchestrator integration | `haco run`, machine output and external events are implemented; orchestration remains outside Core | v0.6 |
| Client-neutral interaction events | public `pkg/interaction` projects capability audit records into minimized stable event types with deterministic IDs, resumable cursors, bounded batches, recovery/attention flags, and public corruption errors; observation never authorizes or executes a capability | v0.6 / cross-cutting |
| Environment routing | the provider-neutral routing seam remains implemented; **cloud implementation is currently deferred** and concrete EC2/AWS/EBS code is absent from the active tree | v0.7 |
| Reusable client adapter contract | public `pkg/clientadapter` exposes client-owned DTOs for exact Environment ensure/reuse, status, loopback SSH/TCP connections, revoke/delete, `/workspace` discovery, and `pkg/interaction` batches; ordinary `haco ssh` is the non-VS-Code proof path | v0.8 / cross-cutting |
| VS Code / Agent Host | `haco-vscode`, per-agent binding and `haco-agent-host` foundations are implemented | v0.8-v0.10 |
| Base lifecycle | provider-neutral Base identity and `haco base list` / `haco base inspect` / `create --base` are implemented | v0.11 |
| Resource budgets | CPU, memory, PID and root-storage budgets are modeled and Incus finite limits are enforced or rejected | v0.12 |
| Managed sandbox network | managed `haco-sandbox0`, proxy-only ACL transport guard and `haco-sandbox` profile are created/verified; bridge DNS is disabled while DHCP remains; drift fails closed | v0.13 / cross-cutting |
| Git fetch plugin | `haco plugin git fetch <environment>` uses trusted Host Git/GitHub authority including `gh auth git-credential` for HTTPS private repositories | v0.14 |
| OCI usage telemetry / Seed recommendation | `haco plugin oci seed sample` records image identities and `recommend` ranks immutable identities; deterministic top 10% eligible recommendations become `auto_promote=true` | v0.15 implemented |
| OCI image deletion | `haco plugin oci image delete <reference[@digest]>` records deletion tombstones; exact immutable identities can be explicitly re-enabled without silently undoing broader deletion state | v0.16 implemented |
| OCI Seed Builder / Btrfs COW | `haco plugin oci seed build` / `current`, per-Base pinning, conservative GC/recovery, trusted Host acquisition, credential-free exact-image harvest from explicitly marked running managed Environments, offline no-NIC build, immutable publication/current pointer, exact-parent resolution, and pre-build interrupted-builder recovery are implemented; real-host/authenticated-registry/COW acceptance remains pending | v0.17 partial |
| Docker compatibility | `haco plugin oci docker status/prepare` validates a Base-provided genuine Docker profile, verifies pinned systemd units, refuses active vendor-daemon takeover, and enables Environment-local socket activation without making Docker a Core requirement | v0.18 implemented |
| Domain-aware egress authorization | Core `network.egress/connect` authority, Standard HTTP/HTTPS proxy, Host-side DNS pinning, private-address rejection, CONNECT/SNI validation, trusted Incus source-IP mapping and `haco egress serve` are implemented; real supported-Incus acceptance remains host-dependent | v0.19 implemented |
| Managed Btrfs rootfs storage | local composition lazily asks Incus to create the `haco-local-default` loop-backed Btrfs pool with `size=128GiB` and routes Hacocoon-owned Base/Tooling/Seed/Environment/trusted-host rootfs paths to that pool | v0.20 implemented |
| Managed Btrfs transparent compression | default Incus pool creation requests `compress=zstd:3`; `compress-force` and `autodefrag` are not desired defaults, while filesystem mount lifecycle remains Incus-owned | v0.21 implemented |
| Interaction notification clients | `haco-notify` provides loopback interaction delivery for browser and native OS notification flows, and the optional VS Code notification extension consumes the same client-neutral interaction stream; replay/dedup behavior is covered by tests | v0.22 implemented |
| Real Incus E2E acceptance | GitHub-hosted Ubuntu 26.04 first verifies standalone real Incus system-container behavior and then runs Hacocoon Core lifecycle E2E on a fresh runner; the phased gate covers systemd/exec, networking, hotplug, storage/snapshot behavior, diagnostics, and guarded cleanup | v0.23 implemented |
| Structured logging | shared `log/slog` foundation, INFO-default text/JSON output, Environment lifecycle operation fields, sanitized DEBUG Host-command tracing, egress authorization tracing, and defense-in-depth secret redaction are implemented across maintained executables | v0.24 implemented |
| Incus-owned Btrfs storage acceptance | the actual ordinary-user `haco` binary is exercised against real Incus; acceptance verifies lazy pool creation, sparse Incus backing image, loop attachment, Btrfs mount, zstd policy, writable Workspace flow, pool reuse, and guarded cleanup | v0.25 implemented |
| Trusted `haco-host` / default WSL entry | local Incus runtime can ensure and shell into a persistent trusted logical `haco-host`; exact ownership markers and reserved-name collision refusal protect the boundary, managed storage is used, WSL interactive entry targets `haco-host` by default while Physical Host root remains an explicit recovery path, and raw Incus control is not exposed into the trusted Host | v0.26 implemented |
| OCI plugin boundary | containerd/nerdctl/Docker-dependent behavior lives under optional `modules/plugin/oci`; `HACO_PLUGIN_OCI=nerdctl|docker` opts in, and Core remains valid when unset | cross-cutting |
| Optional Local OCI Registry | Registry/proxy is optional and not required for ordinary direct upstream pulls or Seed construction | unversioned optional / deferred |

## Domain-aware egress boundary

Ordinary HTTP/HTTPS egress is enforced through the Standard proxy rather than by DNS-to-IP ACL approximation. The Incus NIC remains default deny and allows only TCP to the managed bridge gateway on the Standard proxy port. The bridge keeps DHCP but disables its dnsmasq DNS listener with `raw.dnsmasq=port=0`; unmanaged DNS or ACL configuration fails closed.

The managed profile provides HTTP(S) proxy discovery to Hacocoon Environments. The proxy derives Environment identity from trusted Incus source-IP state, routes each hostname/port/protocol request through the existing Policy / Approval / Capability / audit path, resolves DNS only on the Host after authorization, pins the public answer set per connection, and validates HTTPS CONNECT against ClientHello SNI before forwarding TLS bytes. `haco egress serve` is the foreground trusted-Host launch path so the current stdio approval provider remains usable. See [`EGRESS_AUTHORIZATION.md`](EGRESS_AUTHORIZATION.md).

## Notification clients

v0.22 turns the client-neutral interaction stream into user-visible notification adapters without moving approval authority into the client. `haco-notify` exposes the loopback bridge used by the browser and native notification paths, while `clients/vscode-notify` provides the optional VS Code consumer. Cursor persistence, replay, deduplication, corruption handling, browser behavior, and VS Code behavior are covered by repository tests. Reading or displaying a notification remains observational only and never approves or executes a Capability. See [`INTERACTION_EVENTS.md`](INTERACTION_EVENTS.md).

## Real Incus E2E acceptance

v0.23 records a support-confidence checkpoint rather than a new Core API. GitHub Actions on Ubuntu 26.04 proves the Incus substrate independently before exercising Hacocoon Core. The phased standalone stage verifies real system containers, systemd/exec behavior, networking, device hotplug, storage/snapshot behavior, diagnostics and exact cleanup; the dependent Core stage then runs the Hacocoon lifecycle against real Incus on a fresh runner. This distinguishes an Incus substrate failure from a Hacocoon regression and prevents fake-only E2E from being treated as sufficient acceptance.

## Structured logging

v0.24 makes structured logging a named milestone. Maintained executables configure one shared `log/slog` root from `HACO_LOG_LEVEL` and `HACO_LOG_FORMAT`; INFO/text is the default and JSON is available without changing stdout command results. Environment create/exec/shell/delete operations carry stable `operation`, `environment_id`, duration, result/error fields through context. The trusted Host runner adds sanitized DEBUG command metadata and classifies Incus/network/storage/Git/OCI commands without automatically logging subprocess stdout or stderr. Network egress authorization adds normalized target/protocol and request correlation at DEBUG.

The shared handler redacts known password/token/API-key, authorization/cookie, credential-bearing URL, and secret-assignment patterns as defense in depth, including at DEBUG. Call sites still must omit arbitrary headers, environments, configuration objects, private keys, request bodies, and untrusted output. ERROR ownership remains at the operation/reporting boundary; lower Host/provider layers use DEBUG diagnostics rather than duplicating ERROR. See [`reference/logging.md`](reference/logging.md).

## Trusted `haco-host` / WSL entry

v0.26 introduces a persistent trusted logical Host on the local Incus path. `haco host ensure` creates or reconciles `haco-host`, and `haco host shell` enters it after ensure. Hacocoon marks exact ownership, rejects name collisions with non-owned instances, uses the managed storage path, and keeps the raw Incus control socket outside `haco-host`. The WSL login shim makes normal interactive distro entry target `haco-host`; explicit Physical Host root entry remains the recovery escape hatch.

Real Incus acceptance covers trusted-host creation, ownership, idempotent ensure, stopped-state recovery, managed-storage behavior, and control-socket non-exposure. Real Windows/WSL interactive-login acceptance is still host-dependent. The current slice establishes lifecycle/default-entry behavior; broader Git/OCI/credential/control-channel migration remains follow-up work. See [`design/trusted-host.md`](design/trusted-host.md) and [`WINDOWS_WSL_BOOTSTRAP.md`](WINDOWS_WSL_BOOTSTRAP.md).

## Client adapter boundary

`pkg/clientadapter` is the reusable adapter-facing contract for VS Code-independent clients. Exported signatures use package-owned DTOs and public error sentinels rather than `internal/core` types. The adapter can ensure/reuse an Environment only when the canonical Host Workspace and requested access mode match exactly, exposes the in-guest Workspace as `/workspace`, reconciles connection metadata, and composes the public `pkg/interaction` event contract.

SSH preparation accepts public-key material only. Clients retain their private keys and IDE configuration. Returned/reconciled SSH and TCP connections are revalidated as loopback-only; incompatible provider output is rejected and newly-created invalid connections are revoked or surfaced as recovery-required when cleanup cannot be proven. The existing `haco create` + `haco ssh` + ordinary `ssh` flow is the non-VS-Code proof. See [`CLIENT_ADAPTER_CONTRACT.md`](CLIENT_ADAPTER_CONTRACT.md).

## Client interaction boundary

`pkg/interaction` is the reusable client-facing event contract. It reads the existing trusted capability audit stream and exposes only stable, presentation-safe fields: schema/event/request identity, UTC time, event kind, Environment/capability/action labels, attention/recovery flags, a closed failure code, and the next resume cursor.

Raw capability resources, authority attributes, opaque parameters, provider output, approval tokens, credentials, and free-form audit reasons are not part of the client schema. Browser, VS Code, code-server, JetBrains, and future adapters may independently observe/deduplicate these events; reading an event has no side effect and never substitutes for the trusted Policy/Capability approval or execution boundary. See [`INTERACTION_EVENTS.md`](INTERACTION_EVENTS.md).

## Core/plugin boundary

With `HACO_PLUGIN_OCI` unset, Hacocoon Core must not require or probe for containerd, nerdctl, Docker CLI, Docker Engine, or a local OCI Registry. Base identity remains a Core/provider-neutral concept under `haco base ...`; OCI workload tooling lives under `haco plugin oci ...`.

The project-maintained OCI plugin profile may use containerd + nerdctl, and the Docker driver may provide genuine Docker compatibility. Neither choice defines a mandatory Hacocoon Core runtime.

## OCI Seed / storage

v0.17 has repository slices for build/publish, operations hardening, and credential-free managed-Environment harvest. The implemented path is trusted Host acquisition/cache -> offline no-NIC Seed Builder -> immutable Seed revision/current pointer -> exact-parent resolution -> normal Incus/storage-driver clone. One writable `/var/lib/containerd` must never be shared across Environments.

Explicit per-Base pins are persisted as immutable OCI identities. Deletion tombstones override recommendations and existing pins until the exact immutable identity is explicitly re-enabled. `haco plugin oci seed recover` reconciles exact Hacocoon temporary builders and then performs conservative GC; `haco plugin oci seed build` invokes recovery before a new build while holding the Seed build lock. GC does not manipulate Incus-owned Btrfs internals and retains current, in-use, instance-base, or externally aliased images. Deletion state is re-checked after publication so an operator deletion racing a long build cannot silently become current.

For an exact immutable identity already present in an explicitly marked running Hacocoon-managed Environment, Seed acquisition can copy a temporary `nerdctl save` OCI archive into the trusted Host cache and then delete the guest archive. It does not copy registry credentials, credential-helper output, workspace data, arbitrary Environment files, or live containerd state. Legacy/unmarked Environments are not inspected; failed or unavailable harvest falls back to the existing trusted Host pull path.

v0.20 makes local rootfs storage explicit without giving Hacocoon its own block lifecycle. Before an Environment, Tooling Base builder, Seed builder, or trusted host needs root storage, local composition lazily ensures `haco-local-default` through Incus. Incus creates and owns the sparse backing file, loop device, Btrfs filesystem, and mount; Base, Tooling, Seed, trusted-host, and Environment rootfs volumes, snapshots, and clones share that Incus-owned Btrfs pool while Host Workspaces remain bind-mounted outside it.

v0.21 makes transparent compression the default creation policy: Hacocoon asks Incus for `btrfs.mount_options=compress=zstd:3`, does not request `compress-force`, and keeps `autodefrag` out of the desired default. Incus remains the mount lifecycle owner, and Hacocoon does not rewrite existing extents merely to recompress them.

v0.25 is the real ordinary-user acceptance checkpoint for that storage path. Dedicated GitHub-hosted Ubuntu 26.04 CI runs the actual `haco` binary against real Incus, verifies lazy `haco-local-default` creation, confirms the Incus-owned sparse backing image and loop attachment, checks the live Btrfs mount and zstd policy, exercises writable `haco create` / `exec` / `delete` / `run`, proves pool reuse, and verifies guarded cleanup. See [`design/btrfs-storage-layout.md`](design/btrfs-storage-layout.md).

Local Registry is not a prerequisite and has no reserved milestone. Remaining storage acceptance includes authenticated/private-registry combinations using Host-owned credentials without leakage, physical Btrfs compression ratio and CPU-cost measurement, COW/compaction behavior, broader real-host failure injection, Windows/WSL behavior, and supported-host verification beyond the automated GitHub-hosted Ubuntu 26.04 CLI lifecycle.

## Docker compatibility

v0.18 is implemented at the repository gate. This code landed while the feature was temporarily numbered v0.17 and is reclassified without rollback. `HACO_PLUGIN_OCI=docker` exposes `haco plugin oci docker status <environment>` and `prepare <environment>`. `prepare` does not install packages or mount Host sockets: it requires the selected Base/Seed to provide Docker CLI, dockerd, containerd, systemd, the docker group, and the Hacocoon-pinned socket/service units. It fails closed on unit drift or an already-active vendor Docker daemon instead of silently taking it over.

Real Incus/systemd acceptance remains host-dependent and is tracked separately from repository implementation status.

## Cloud status

v0.7 retains the provider-neutral Environment routing seam because that architecture remains useful. The former concrete EC2/AWS/EBS implementation was intentionally removed while the local/provider contracts are still moving. **Cloud implementation is currently deferred** and must not be described as active or accepted.

## Acceptance gaps

Repository tests do not substitute for all real-host acceptance. v0.23 proves a phased real-Incus substrate plus Core lifecycle on GitHub-hosted Ubuntu 26.04, v0.25 additionally proves ordinary-user Incus-owned Btrfs CLI behavior, and v0.26 proves trusted-host lifecycle/control-socket isolation on real Incus. Real Incus networking/resource behavior beyond those paths—including proxy-only bridge ACL/dnsmasq behavior—Windows/WSL + VS Code and interactive `haco-host` entry, private-registry credentials, Docker compatibility, physical Btrfs compression/COW/compaction behavior, broader storage failure injection, desktop notification delivery, and future cloud adapters remain environment-dependent. Partial acceptance in an earlier milestone does not prevent later minor checkpoints from advancing.
