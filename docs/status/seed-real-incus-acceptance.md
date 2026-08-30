# Real Incus Seed acceptance

Status: accepted end to end on a disposable GitHub-hosted Ubuntu 26.04 runner with real Incus, containerd, nerdctl, and Docker socket activation.

## Repository implementation status

The optional OCI Seed path is accepted independently from ordinary Hacocoon Environment use. Enabling `HACO_PLUGIN_OCI=nerdctl` activates Seed resolution/build behavior; installations that do not enable the OCI plugin do not require nerdctl, containerd Seed state, the trusted tooling-builder network, or Docker compatibility services.

The `seed-real-incus-acceptance` workflow exercises the production CLI and Incus provider path. The accepted run proves all of the following:

- `haco/ubuntu-26.04` resolves to an immutable parent Base revision;
- the trusted Host acquires a selected OCI image and pins its exact `reference@sha256:...` identity;
- tooling acquisition uses a short-lived trusted builder network that is separate from the normal default-deny sandbox network;
- the final Seed Builder runs with no NIC while immutable OCI content is loaded;
- a Seed revision is published immutably and the current pointer advances only after successful publication;
- temporary tooling/Seed builders are removed after publication;
- two Seed-derived Environments can inspect and execute the imported OCI identity without registry access;
- writable `/var/lib/containerd` state is independent between the two Environments;
- the Docker-compatible API is provided by the Base/Seed through an instance-local `/run/docker.sock`, with `dockerd` socket-activated on demand and no Host Docker socket mounted into the Environment;
- `seed pin`, `pins`, `unpin`, `gc`, `recover`, exact image deletion, and exact `image reenable` complete against the real Host state;
- maintenance operations do not mutate the current immutable Seed revision unexpectedly.

The acceptance harness is `test/e2e/seed_real_incus.sh`. Normal unit/PR CI does not claim this real-host result; the harness requires `HACO_E2E_INCUS_SEED=1` and the dedicated workflow is manually runnable with `workflow_dispatch`.

## Accepted host and versions

Reference successful run: GitHub Actions `seed-real-incus-acceptance` run #46.

| Component | Accepted version |
|---|---|
| Host OS | Ubuntu 26.04 LTS |
| Host kernel | `7.0.0-1012-azure` |
| Incus client/server | `7.0.1` / `7.0.1` |
| Host containerd | `2.3.3` |
| nerdctl | `2.3.5` |
| Docker Engine API inside each Seed-derived Environment | `29.1.3` |
| Incus storage used by the reference acceptance setup | Btrfs loop-backed pool |

The OCI fixture was `docker.io/library/busybox:1.36` pinned and executed by its exact SHA-256 digest rather than by a mutable tag.

## Host-dependent limitations

These are host/runtime constraints, not unresolved repository implementation gaps:

- Hacocoon's sandbox NIC keeps Incus bridge IP filtering enabled. The Host must provide bridge netfilter hooks; on the GitHub-hosted runner the acceptance explicitly loads `br_netfilter` and verifies `/proc/sys/net/bridge/bridge-nf-call-iptables` and `bridge-nf-call-ip6tables`.
- The GitHub-hosted image includes other netfilter users, notably Docker. Their forwarding policy can block an otherwise NAT-enabled Incus bridge, so the acceptance adds forwarding exceptions scoped only to the short-lived `haco-t-*` tooling bridge instead of weakening the global sandbox policy.
- Public package acquisition is allowed only during trusted tooling Base preparation. The published tooling Base has that NIC removed, and the Seed Builder itself is verified no-NIC/offline.
- The acceptance uses Ubuntu 26.04 with the verified Incus 7.0 LTS packages used by the project's real Incus CI substrate. Other distributions, kernels, Incus series, and firewall managers need their own supported-host validation.
- Docker compatibility requires the Base/Seed profile produced by the optional tooling build. Ordinary non-Seed Environments do not gain Docker or nested OCI authority implicitly.

## Failure and recovery boundary

The production Seed implementation fails closed around each mutating phase: temporary builders and the tooling bridge use deterministic cleanup, cleanup failures are surfaced as recovery-required state, the current Seed pointer is not advanced before immutable publication succeeds, and `seed recover`/`seed gc` operate on retained immutable manifests. The real acceptance also executes recovery and garbage-collection commands after two live Seed-derived Environments have been created.

Physical Btrfs COW savings measurement and dedicated real-host failure injection remain separate follow-up acceptance work; completing this Seed runtime acceptance does not claim those checks are complete.
