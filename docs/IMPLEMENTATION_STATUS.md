# Implementation status

| Area | Existing? | Target | Status / action |
|---|---:|---:|---|
| Core Session model | new | 0.1 | implemented |
| Runtime contract | new | 0.1 | implemented |
| Storage contract | new | 0.1 | implemented |
| Core fake Runtime/Storage tests | new | 0.1 | implemented |
| runtime.incus CLI adapter | new | 0.1 | initial implementation |
| storage.btrfs | new | 0.1 | initial implementation; local BlockStore seam is private to this module |
| block.local-raw | new | 0.1 | initial implementation |
| block.local-qcow2 | new | 0.1 | initial implementation |
| host init/doctor | new | 0.1 | initial implementation |
| base image systemd/containerd/nerdctl | new | 0.1 | host integration pending |
| storage grow | new | 0.1 | adapter implementation present; integration pending |
| shrink plan / safe ordering | new | 0.1 | implemented with session quiescence and ordering tests |
| crash reconciliation | new | 0.1 | Core lifecycle reconciliation present; storage step recovery needs integration tests |
| v0.1 WSL E2E acceptance | new | 0.1 | pending supported WSL host |
| repositories/workspace | no | 0.2 | deferred |
| Security + Git | no | 0.3 | deferred |
| GitHub/AWS/registry capabilities | no | 0.4 | deferred |
| WSLg/IntelliJ | no | 0.5 | deferred |
| Web/Interaction | no | 0.6 | deferred |
| Remote/EC2/EBS | no | 0.7 | deferred |
