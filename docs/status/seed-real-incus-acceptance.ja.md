# Real Incus Seed acceptance

状態: 実際の Incus / containerd / nerdctl と Docker socket activation を使い、使い捨ての GitHub-hosted Ubuntu 26.04 runner 上で end-to-end acceptance 済み。

## Repository implementation status

OCI Seed は通常の Hacocoon Environment から独立した optional path として受け入れられている。`HACO_PLUGIN_OCI=nerdctl` を有効にした場合だけ Seed の解決・build 経路が有効になり、OCI plugin を有効にしないインストールでは nerdctl、Seed 用 containerd state、trusted tooling-builder network、Docker compatibility service は必須ではない。

`seed-real-incus-acceptance` workflow は production CLI と Incus provider の実経路を通す。acceptance 済み run では次を確認した。

- `haco/ubuntu-26.04` が immutable な parent Base revision に解決される。
- trusted Host が選択した OCI image を取得し、exact な `reference@sha256:...` identity を pin する。
- tooling acquisition は通常の default-deny sandbox network と分離された短命な trusted builder network を使う。
- immutable OCI content を load する最終 Seed Builder は no-NIC で動く。
- Seed revision は immutable に publish され、current pointer は publish 成功後にだけ進む。
- publish 後に一時 tooling / Seed builder が残らない。
- 2 個の Seed-derived Environment が registry access なしで import 済み OCI identity を inspect / execute できる。
- `/var/lib/containerd` の writable state は 2 Environment 間で独立している。
- Docker-compatible API は Base/Seed 由来の instance-local `/run/docker.sock` で提供され、`dockerd` は必要時に socket activation され、Host の Docker socket は Environment に mount されない。
- real Host state に対して `seed pin`、`pins`、`unpin`、`gc`、`recover`、exact image deletion、exact `image reenable` が完走する。
- maintenance operation が current の immutable Seed revision を意図せず変更しない。

acceptance harness は `test/e2e/seed_real_incus.sh`。通常の unit / PR CI はこの real-host 結果を暗黙に主張せず、harness は `HACO_E2E_INCUS_SEED=1` を要求する。専用 workflow は `workflow_dispatch` で手動実行できる。

## Accepted host and versions

基準となる成功 run: GitHub Actions `seed-real-incus-acceptance` run #46。

| Component | Accepted version |
|---|---|
| Host OS | Ubuntu 26.04 LTS |
| Host kernel | `7.0.0-1012-azure` |
| Incus client/server | `7.0.1` / `7.0.1` |
| Host containerd | `2.3.3` |
| nerdctl | `2.3.5` |
| 各 Seed-derived Environment 内の Docker Engine API | `29.1.3` |
| 基準 acceptance setup の Incus storage | Btrfs loop-backed pool |

OCI fixture は `docker.io/library/busybox:1.36` で、mutable tag ではなく exact SHA-256 digest を pin して実行した。

## Host-dependent limitations

以下は host/runtime 側の制約であり、repository implementation の未解決項目ではない。

- Hacocoon sandbox NIC は Incus bridge IP filtering を有効のまま使う。Host は bridge netfilter hook を提供する必要があり、GitHub-hosted runner では acceptance が `br_netfilter` を明示的に load し、`/proc/sys/net/bridge/bridge-nf-call-iptables` と `bridge-nf-call-ip6tables` を検証する。
- GitHub-hosted image には Docker など他の netfilter 利用者がいる。それらの forwarding policy が NAT-enabled Incus bridge を塞ぐことがあるため、acceptance では global policy を緩めず、短命な `haco-t-*` tooling bridge だけに forwarding 例外を限定する。
- Public package acquisition は trusted tooling Base preparation 中だけ許可する。publish される tooling Base からは NIC を外し、Seed Builder 自体も no-NIC/offline を検証する。
- acceptance は Ubuntu 26.04 と、project の real Incus CI substrate で使う verified Incus 7.0 LTS package を対象にしている。他 distribution / kernel / Incus series / firewall manager は別途 supported-host validation が必要。
- Docker compatibility は optional tooling build が生成する Base/Seed profile を必要とする。通常の non-Seed Environment に Docker や nested OCI authority が暗黙に追加されることはない。

## Failure and recovery boundary

production Seed implementation は各 mutating phase で fail closed する。一時 builder と tooling bridge は deterministic cleanup を持ち、cleanup failure は recovery-required state として表面化し、immutable publish 成功前に current Seed pointer を進めず、`seed recover` / `seed gc` は保持された immutable manifest を基準に動く。real acceptance でも 2 個の live Seed-derived Environment を作成した後に recovery と garbage collection を実行する。

Physical Btrfs COW savings measurement と専用の real-host failure injection は別 follow-up の acceptance work として残る。この Seed runtime acceptance の完了は、それらまで完了したという意味ではない。
