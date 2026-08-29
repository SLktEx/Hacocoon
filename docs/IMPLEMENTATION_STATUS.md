# Implementation status

| Area | Existing? | Target | Status / action |
|---|---:|---:|---|
| Core Session model | new | 0.1 | implemented |
| Runtime contract | new | 0.1 | implemented |
| Storage contract | new | 0.1 | implemented |
| Core fake Runtime/Storage tests | new | 0.1 | implemented |
| runtime.incus CLI adapter | new | 0.1 | initial implementation; unit + fake-CLI process-boundary integration tests pass; opt-in real-host E2E added |
| storage.btrfs | new | 0.1 | initial implementation; local BlockStore seam is private to this module |
| block.local-raw | new | 0.1 | initial implementation |
| block.local-qcow2 | new | 0.1 | initial implementation |
| host init/doctor | new | 0.1 | runtime/storage probe + Incus project/pool/base-image prepare implemented; opt-in supported-host E2E exists, execution pending |
| base image systemd/containerd/nerdctl | new | 0.1 | build/publish choreography and provision script process-flow tests pass; real Incus/WSL nested-container E2E written, execution pending |
| storage grow | new | 0.1 | adapter implementation present; real-host E2E covers grow after Session stop, execution pending |
| shrink plan / safe ordering | new | 0.1 | implemented with session quiescence and ordering tests |
| crash reconciliation | new | 0.1 | Core lifecycle reconciliation present; storage step recovery needs integration tests |
| v0.1 WSL E2E acceptance | new | 0.1 | `HACO_E2E_INCUS=1 go test ./modules/runtime/incus -run TestRealIncusLifecycleE2E -v`; pending supported WSL host |
| repositories/workspace | no | 0.2 | deferred |
| Security + Git | no | 0.3 | deferred |
| GitHub/AWS/registry capabilities | no | 0.4 | deferred |
| WSLg/IntelliJ | no | 0.5 | deferred |
| Web/Interaction | no | 0.6 | deferred |
| Remote/EC2/EBS | no | 0.7 | deferred |
