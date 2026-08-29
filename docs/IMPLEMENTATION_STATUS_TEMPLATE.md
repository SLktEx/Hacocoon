# Hacocoon Implementation Status Template

Fill this from the actual repository before changing architecture.

| Area | Existing? | Current quality/tests | Target version | Action |
|---|---|---|---|---|
| Core Session lifecycle | ? | ? | 0.1 | preserve/simplify |
| Incus lifecycle | ? | ? | 0.1 | extract to runtime.incus |
| Btrfs storage | ? | ? | 0.1 | extract behind Storage |
| local image/loop backend | ? | ? | 0.1 | extract behind Block backend |
| storage grow | ? | ? | 0.1 | stabilize |
| storage shrink/compact | ? | ? | 0.1 | implement safe inner-to-outer workflow |
| systemd base | ? | ? | 0.1 | stabilize |
| containerd/nerdctl | ? | ? | 0.1 | stabilize |
| repo cache/workspace | ? | ? | 0.2 | preserve/defer |
| VS Code/SSH/ports | ? | ? | 0.2 | preserve/defer |
| Agent lifecycle | ? | ? | 0.2 | preserve/defer |
| push approval | ? | ? | 0.3 | refactor into shared Security |
| Git broker/provider | ? | ? | 0.3 | preserve/refactor |
| GitHub/gh | ? | ? | 0.4 | defer |
| AWS capability | ? | ? | 0.4 | defer |
| Registry/network profile | ? | ? | 0.4 | defer |
| WSLg/IntelliJ | ? | ? | 0.5 | defer |
| Web UI/notification | ? | ? | 0.6 | defer |
| code-server | ? | ? | 0.6 | defer |
| remote Linux/Gateway | ? | ? | 0.7 | defer |
| EC2 runtime | ? | ? | 0.7 | defer |
| EC2/EBS lifecycle | ? | ? | 0.7 | defer; package boundary intentionally unresolved until v0.7 design gate |
